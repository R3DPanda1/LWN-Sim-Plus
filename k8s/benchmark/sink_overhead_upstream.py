"""Upstream sink-overhead: CPU% and peak RAM versus fleet size.

The upstream simulator has no bulk endpoint and no Prometheus metrics, so the
fork builds the ABP fleet, its simdata is mirrored into the upstream container,
and CPU/RAM are read from the upstream cgroup. Pairs with sink_overhead_fork.py.
Needs the upstream image: docker compose --profile upstream build lwn-upstream.

  python3 sink_overhead_upstream.py
  SINK_TIERS=50 REPS=2 python3 sink_overhead_upstream.py     # quick check
"""
import csv
import statistics
import subprocess
import sys
import time
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
from common import (env, BENCH_ROOT, cs_profile_id, sim_configure_template,
                    sim_create, sim_del_all, sim_start, sim_stop, sim_status_ok,
                    sim_container, container_cpu_seconds, container_peak_ram_mb,
                    reset_peak_ram)

TIERS = [int(x) for x in env("SINK_TIERS", "500,1000,5000,10000").split(",")]
REPS  = int(env("REPS", "3"))
CSV_PATH = HERE / "sink_overhead.csv"
UPSTREAM_URL = env("UPSTREAM_URL", "http://localhost:8012/api")
SIM_DATA = BENCH_ROOT / "simdata"
UP_DATA  = BENCH_ROOT / "upstream-data"


def sh(cmd, check=True):
    print(f"  $ {cmd}", flush=True)
    r = subprocess.run(cmd, shell=True, capture_output=True, text=True)
    if check and r.returncode != 0:
        raise SystemExit(r.stderr.strip() or f"failed: {cmd}")


def start_upstream():
    sh(f"rm -rf {UP_DATA} && mkdir -p {UP_DATA} && cp -a {SIM_DATA}/. {UP_DATA}/")
    sh(f"cd {BENCH_ROOT} && docker compose --profile upstream up -d lwn-upstream")
    for _ in range(30):
        if sim_status_ok(url=UPSTREAM_URL):
            return
        time.sleep(2)
    raise SystemExit("upstream did not come up")


def measure(n, cont):
    reset_peak_ram(cont)
    cpu0 = container_cpu_seconds(cont)
    t0 = time.time()
    sim_start(url=UPSTREAM_URL)
    time.sleep(80 if n <= 1000 else 90 if n <= 5000 else 120)
    sim_stop(url=UPSTREAM_URL)
    wall = time.time() - t0
    cpu_pct = 100.0 * (container_cpu_seconds(cont) - cpu0) / wall if wall else 0.0
    return cpu_pct, container_peak_ram_mb(cont)


def upsert(rows):
    existing = {}
    if CSV_PATH.exists():
        for r in csv.DictReader(CSV_PATH.open()):
            existing[(r["sim"], r["count"])] = r
    for r in rows:
        existing[(r["sim"], str(r["count"]))] = {
            "sim": r["sim"], "count": str(r["count"]),
            "cpu_pct": f"{r['cpu_pct']:.2f}", "peak_ram_mb": f"{r['peak_ram_mb']:.0f}"}
    with CSV_PATH.open("w", newline="") as f:
        w = csv.DictWriter(f, fieldnames=["sim", "count", "cpu_pct", "peak_ram_mb"])
        w.writeheader()
        for row in sorted(existing.values(), key=lambda r: (r["sim"], int(r["count"]))):
            w.writerow(row)


def main():
    profile_id = cs_profile_id(env("ABP_PROFILE_NAME", "AM319 ABP"))
    rows = []
    for n in TIERS:
        print(f"\n--- upstream N={n} ({REPS} reps, drop first) ---", flush=True)
        sim_del_all()
        sim_configure_template(profile_id, "abp", 60, integration=False)
        sim_create(n, "Sink")
        sh(f"cd {BENCH_ROOT} && docker compose stop lwn-simulator")
        start_upstream()
        cont = sim_container("lwn-upstream")

        cpu_vals, ram_vals = [], []
        for i in range(REPS):
            cpu_pct, ram_mb = measure(n, cont)
            print(f"  rep {i+1}: cpu={cpu_pct:.2f}% ram={ram_mb:.0f}MB"
                  f"{' (warmup)' if i == 0 else ''}", flush=True)
            if i > 0:
                cpu_vals.append(cpu_pct)
                ram_vals.append(ram_mb)

        sh(f"cd {BENCH_ROOT} && docker compose --profile upstream rm -sf lwn-upstream", check=False)
        sh(f"cd {BENCH_ROOT} && docker compose start lwn-simulator")
        for _ in range(30):
            if sim_status_ok():
                break
            time.sleep(2)

        rows.append({"sim": "upstream", "count": n,
                     "cpu_pct": statistics.fmean(cpu_vals) if cpu_vals else 0.0,
                     "peak_ram_mb": statistics.fmean(ram_vals) if ram_vals else 0.0})
        print(f"  => cpu={rows[-1]['cpu_pct']:.2f}% ram={rows[-1]['peak_ram_mb']:.0f}MB", flush=True)
    upsert(rows)
    print(f"\nwrote {CSV_PATH}")


if __name__ == "__main__":
    main()
