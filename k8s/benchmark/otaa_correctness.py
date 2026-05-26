"""OTAA join correctness, fork versus upstream.

Upstream fails past a few concurrent OTAA devices (every device gets a DevAddr
but none reach lastSeenAt); the fork scales to thousands. Both passes drive the
same ChirpStack instance with the same credentials.

The upstream pass reuses the fork's bulk endpoint to register the fleet, then
stops the fork, mirrors its simdata into the upstream container, and brings
upstream up against the same ChirpStack.

  python3 otaa_correctness.py            # both passes, paper scale
  PASS=fork FORK_N=20 python3 otaa_correctness.py     # quick fork-only check
"""
import csv
import subprocess
import time
from pathlib import Path

import common
from common import (env, BENCH_ROOT, cs_profile_id, cs_list_euis,
                    cs_devaddr_count, cs_last_seen_count, sim_del_all,
                    sim_configure_template, sim_create, sim_start, sim_stop,
                    sim_status_ok)

PASS         = env("PASS", "both")
UPSTREAM_N   = int(env("UPSTREAM_N", "10"))
UPSTREAM_WAIT= int(env("UPSTREAM_WAIT_SEC", "60"))
UPSTREAM_URL = env("UPSTREAM_URL", "http://localhost:8012/api")
FORK_N       = int(env("FORK_N", "2000"))
FORK_SPREAD  = int(env("FORK_SPREAD_MS", "10000"))
FORK_WAIT    = int(env("FORK_WAIT_SEC", "120"))
OTAA_PROFILE = env("OTAA_PROFILE_NAME", "AM319 OTAA")

HERE     = Path(__file__).resolve().parent
OUT_CSV  = Path(env("OUT_CSV", HERE / "results" / "otaa_correctness" / "otaa_correctness.csv"))
FIELDS   = ["sim", "n", "devaddr", "last_seen", "wait_sec", "spread_ms"]
SIM_DATA = BENCH_ROOT / "simdata"
UP_DATA  = BENCH_ROOT / "upstream-data"


def sh(cmd, check=True):
    print(f"  $ {cmd}", flush=True)
    r = subprocess.run(cmd, shell=True, capture_output=True, text=True)
    if check and r.returncode != 0:
        raise SystemExit(r.stderr.strip() or f"failed: {cmd}")
    return r


def upsert(row):
    rows = []
    if OUT_CSV.exists():
        rows = [r for r in csv.DictReader(open(OUT_CSV)) if r["sim"] != row["sim"]]
    rows.append({k: row[k] for k in FIELDS})
    OUT_CSV.parent.mkdir(parents=True, exist_ok=True)
    with open(OUT_CSV, "w", newline="") as f:
        w = csv.DictWriter(f, fieldnames=FIELDS)
        w.writeheader()
        w.writerows(rows)


def make_fleet(n):
    sim_del_all()
    sim_configure_template(cs_profile_id(OTAA_PROFILE), "otaa", 30)
    sim_create(n, "otaa-corr")


def sample():
    euis = cs_list_euis()
    return cs_devaddr_count(euis), cs_last_seen_count()


def fork_pass():
    print(f"=== fork OTAA N={FORK_N} spread={FORK_SPREAD}ms ===")
    make_fleet(FORK_N)
    sim_start(spread_ms=FORK_SPREAD)
    time.sleep(FORK_WAIT)
    devaddr, last_seen = sample()
    sim_stop()
    print(f"  DevAddr={devaddr}/{FORK_N}  lastSeenAt={last_seen}/{FORK_N}")
    upsert({"sim": "fork", "n": FORK_N, "devaddr": devaddr,
            "last_seen": last_seen, "wait_sec": FORK_WAIT, "spread_ms": FORK_SPREAD})


def upstream_pass():
    print(f"=== upstream OTAA N={UPSTREAM_N} ===")
    make_fleet(UPSTREAM_N)
    sh(f"cd {BENCH_ROOT} && docker compose stop lwn-simulator")
    sh(f"rm -rf {UP_DATA} && mkdir -p {UP_DATA} && cp -a {SIM_DATA}/. {UP_DATA}/")
    sh(f"cd {BENCH_ROOT} && docker compose --profile upstream up -d lwn-upstream")
    for _ in range(30):
        if sim_status_ok(url=UPSTREAM_URL):
            break
        time.sleep(2)
    sim_start(url=UPSTREAM_URL)
    time.sleep(UPSTREAM_WAIT)
    devaddr, last_seen = sample()
    print(f"  DevAddr={devaddr}/{UPSTREAM_N}  lastSeenAt={last_seen}/{UPSTREAM_N}")
    sim_stop(url=UPSTREAM_URL)
    sh(f"cd {BENCH_ROOT} && docker compose --profile upstream rm -sf lwn-upstream", check=False)
    sh(f"cd {BENCH_ROOT} && docker compose start lwn-simulator")
    for _ in range(30):
        if sim_status_ok():
            break
        time.sleep(2)
    upsert({"sim": "upstream", "n": UPSTREAM_N, "devaddr": devaddr,
            "last_seen": last_seen, "wait_sec": UPSTREAM_WAIT, "spread_ms": 0})


def main():
    if PASS in ("both", "upstream"):
        upstream_pass()
    if PASS in ("both", "fork"):
        fork_pass()
    print(f"\nwrote {OUT_CSV}")
    for r in csv.DictReader(open(OUT_CSV)):
        print(f"  {r['sim']:9s} N={r['n']:>5} DevAddr={r['devaddr']:>5} lastSeenAt={r['last_seen']:>5}")


if __name__ == "__main__":
    main()
