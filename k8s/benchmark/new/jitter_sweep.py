#!/usr/bin/env python3
"""Jitter sweep: find the mean start-jitter that lets N OTAA devices all
reach lastSeenAt on the target ChirpStack within the run window.

Deploy-agnostic: talks HTTP only. Works against docker-compose, k3d, or
a remote Hetzner box via an SSH tunnel (`ssh -fN hetzner`).

Cleanup between runs is done via /api/del-all-devices on the simulator,
which parallel-deprovisions every device from ChirpStack (10 workers).
No docker restarts, no Redis flush.

Env vars (all optional):
  SIM_URL        http://localhost:8002/api
  METRICS_URL    http://localhost:8003/metrics
  CS_URL         http://localhost:8080
  CS_API_KEY     (required for CS lastSeenAt polling)
  CS_APP_ID      (required)
  OTAA_PROFILE   (required; device-profile id with OTAA enabled)
  SIM_INTEG_ID   0
  COUNT          5000
  JITTERS        0,10000,30000,60000,120000     (ms, mean exponential)
  INTERVAL       120                             (seconds; sim sendInterval)
  MAX_MIN        12                              (run cap, early-stop at 100%)
  OUTDIR         ./results
"""
import csv
import json
import os
import sys
import time
from datetime import datetime

import requests

SIM = os.environ.get("SIM_URL", "http://localhost:8002/api")
METRICS = os.environ.get("METRICS_URL", "http://localhost:8003/metrics")
CS = os.environ.get("CS_URL", "http://localhost:8080")
KEY = os.environ.get("CS_API_KEY", "")
APP = os.environ.get("CS_APP_ID", "")
OTAA_PROFILE = os.environ.get("OTAA_PROFILE", "")
SIM_INTEG = int(os.environ.get("SIM_INTEG_ID", "0"))

COUNT = int(os.environ.get("COUNT", "5000"))
JITTERS = [int(x) for x in os.environ.get(
    "JITTERS", "0,10000,30000,60000,120000").split(",") if x.strip()]
INTERVAL = int(os.environ.get("INTERVAL", "120"))
MAX_MIN = int(os.environ.get("MAX_MIN", "12"))
OUTDIR = os.environ.get("OUTDIR", "./results")

H = {"Authorization": f"Bearer {KEY}", "Content-Type": "application/json"}


def metric(name):
    try:
        for ln in requests.get(METRICS, timeout=5).text.splitlines():
            if ln.startswith(name + " "):
                return int(float(ln.split()[1]))
    except Exception:
        return -1
    return -1


def cs_count():
    try:
        r = requests.get(f"{CS}/api/devices",
                         params={"limit": 1, "applicationId": APP},
                         headers=H, timeout=30)
        return int(r.json().get("totalCount", -1)) if r.ok else -1
    except Exception:
        return -1


def cs_last_seen():
    seen, offset = 0, 0
    while True:
        for attempt in range(4):
            try:
                r = requests.get(f"{CS}/api/devices",
                                 params={"limit": 500, "offset": offset,
                                         "applicationId": APP},
                                 headers=H, timeout=180)
                break
            except requests.exceptions.ReadTimeout:
                if attempt == 3:
                    return -1
                time.sleep(10)
        res = r.json().get("result", []) if r.ok else []
        if not res:
            break
        seen += sum(1 for d in res if d.get("lastSeenAt"))
        if len(res) < 500:
            break
        offset += 500
        if offset > COUNT * 3:
            break
    return seen


def sim_cleanup():
    try:
        requests.get(f"{SIM}/stop", timeout=60)
    except Exception:
        pass
    print("  [clean] POST /del-all-devices (parallel CS deprovision)...",
          flush=True)
    t0 = time.time()
    r = requests.post(f"{SIM}/del-all-devices", timeout=1200)
    dt = time.time() - t0
    body = r.json() if r.ok else {}
    print(f"  [clean] deleted {body.get('deleted', '?')} in {dt:.0f}s "
          f"(cs remaining={cs_count()})", flush=True)


def configure_template():
    t = requests.get(f"{SIM}/template/1", timeout=30).json()
    if isinstance(t, dict) and len(t) == 1 and "id" not in t:
        t = t[next(iter(t))]
    t["activationMode"] = "otaa"
    t["sendInterval"] = INTERVAL
    t["range"] = 5000
    t["integrationEnabled"] = True
    t["integrationId"] = SIM_INTEG
    t["deviceProfileId"] = OTAA_PROFILE
    r = requests.post(f"{SIM}/update-template", json=t, timeout=30)
    if r.status_code != 200:
        raise SystemExit(f"template update: {r.status_code} {r.text}")


def create_devices():
    BATCH = 10000
    t0 = time.time()
    remaining, i = COUNT, 0
    while remaining > 0:
        n = min(BATCH, remaining)
        r = requests.post(
            f"{SIM}/create-devices-from-template",
            json={"templateId": 1, "count": n,
                  "namePrefix": f"bench-b{i}",
                  "baseLat": 48.2077, "baseLng": 16.375,
                  "baseAlt": 0, "spreadMeters": 500},
            timeout=1200,
        )
        body = r.json()
        if "error" in body:
            raise SystemExit(f"batch {i} failed: {body}")
        print(f"  [create] batch {i}: {body.get('created')}", flush=True)
        remaining -= n
        i += 1
    print(f"  [create] total: {int(time.time() - t0)}s", flush=True)


def run_one(jitter_ms, stamp):
    csv_path = f"{OUTDIR}/jitter_{stamp}_n{COUNT}_j{jitter_ms}.csv"
    print(f"\n=== RUN n={COUNT} jitter={jitter_ms}ms out={csv_path} ===",
          flush=True)

    sim_cleanup()
    configure_template()
    create_devices()

    start_url = f"{SIM}/start"
    if jitter_ms > 0:
        start_url += f"?jitter={jitter_ms}"
    print(f"  [start] GET {start_url}", flush=True)

    final_seen = 0
    early_stop = False
    with open(csv_path, "w", newline="") as f:
        w = csv.writer(f)
        w.writerow(["jitter_ms", "elapsed_min", "elapsed_sec",
                    "joins", "gw_sent", "last_seen"])
        requests.get(start_url, timeout=120)
        t0 = time.time()

        def sample(label=""):
            el = time.time() - t0
            j = metric("lwnsim_otaa_joins_total")
            g = metric("gateway_data_sent_total")
            s = cs_last_seen()
            w.writerow([jitter_ms, f"{el/60:.2f}", f"{el:.0f}", j, g, s])
            f.flush()
            print(f"  t+{el/60:5.2f}m joins={j:5d} gw={g:7d} "
                  f"lastSeen={s:5d}/{COUNT} {label}", flush=True)
            return s

        final_seen = sample("start")
        for target_min in range(1, MAX_MIN + 1):
            target = target_min * 60
            while time.time() - t0 < target:
                time.sleep(2)
            final_seen = sample(f"t={target_min}min")
            if final_seen >= COUNT:
                print(f"  [early-stop] lastSeen=={COUNT} at "
                      f"t={target_min}min", flush=True)
                early_stop = True
                break

        print("  [stop] /stop", flush=True)
        try:
            requests.get(f"{SIM}/stop", timeout=300)
        except Exception as e:
            print(f"  stop err: {e}", flush=True)

    return csv_path, final_seen, early_stop


def main():
    for var, val in [("CS_API_KEY", KEY), ("CS_APP_ID", APP),
                     ("OTAA_PROFILE", OTAA_PROFILE)]:
        if not val:
            sys.exit(f"missing env var: {var}")
    os.makedirs(OUTDIR, exist_ok=True)

    stamp = datetime.now().strftime("%Y%m%d_%H%M%S")
    print(f"=== JITTER SWEEP n={COUNT} jitters={JITTERS}ms "
          f"interval={INTERVAL}s cap={MAX_MIN}min ===", flush=True)

    results = []
    for mu in JITTERS:
        p, seen, early = run_one(mu, stamp)
        pct = round(100.0 * seen / COUNT, 1)
        results.append({"jitter_ms": mu, "final_last_seen": seen,
                        "pct": pct, "csv": p, "early_stop": early})
        print(f"  [result] jitter={mu}ms lastSeen={seen}/{COUNT} "
              f"({pct}%)", flush=True)
        if seen >= COUNT:
            print(f"\n=== 100% at jitter={mu}ms. Sweep stops here. ===",
                  flush=True)
            break
        print("  [cooldown] 30s", flush=True)
        time.sleep(30)

    summary = f"{OUTDIR}/jitter_{stamp}_n{COUNT}_summary.csv"
    with open(summary, "w", newline="") as f:
        w = csv.writer(f)
        w.writerow(["jitter_ms", "final_last_seen", "pct",
                    "early_stop", "csv"])
        for r in results:
            w.writerow([r["jitter_ms"], r["final_last_seen"], r["pct"],
                        r["early_stop"], r["csv"]])

    print(f"\n=== DONE === summary: {summary}", flush=True)
    for r in results:
        print(f"  jitter={r['jitter_ms']:>6d}ms  "
              f"lastSeen={r['final_last_seen']:>5d}/{COUNT}  "
              f"{r['pct']:5.1f}%  early={r['early_stop']}", flush=True)


if __name__ == "__main__":
    main()
