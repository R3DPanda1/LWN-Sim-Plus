"""Fork sink-overhead: per-tier CPU% and peak RAM, AM319 ABP single burst on a
bridge-only sink. Pairs with sink_overhead_upstream.py; the two rows make the
fork-versus-upstream comparison. Absolute numbers are host-specific.

  python3 sink_overhead_fork.py
  SINK_TIERS=50 REPS=2 python3 sink_overhead_fork.py     # quick check
"""
import csv
import statistics
import sys
import time
from pathlib import Path

import requests

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
from common import (env, cs_profile_id, sim_configure_template, sim_create,
                    sim_start, sim_stop, sim_del_all, container_peak_ram_mb,
                    reset_peak_ram, sim_container, METRICS_URL)

TIERS = [int(x) for x in env("SINK_TIERS", "500,1000,5000,10000").split(",")]
REPS  = int(env("REPS", "3"))
CSV_PATH = HERE / "sink_overhead.csv"


def prom(name):
    try:
        for line in requests.get(METRICS_URL, timeout=5).text.splitlines():
            if line.startswith(name + " "):
                return float(line.split()[1])
    except Exception:
        pass
    return 0.0


def burst(n, profile_id, container):
    sim_stop()
    sim_del_all()
    sim_configure_template(profile_id, "abp", 60, integration=False)
    sim_create(n, "Sink", spread_m=100)
    reset_peak_ram(container)
    cpu0 = prom("process_cpu_seconds_total")
    t0 = time.time()
    sim_start()
    time.sleep(80 if n <= 1000 else 90 if n <= 5000 else 120)
    sim_stop()
    wall = time.time() - t0
    cpu_pct = 100.0 * (prom("process_cpu_seconds_total") - cpu0) / wall if wall else 0.0
    return cpu_pct, container_peak_ram_mb(container)


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
    container = sim_container()
    rows = []
    for n in TIERS:
        print(f"\n--- fork N={n} ({REPS} reps, drop first) ---", flush=True)
        cpu_vals, ram_vals = [], []
        for i in range(REPS):
            cpu_pct, ram_mb = burst(n, profile_id, container)
            print(f"  rep {i+1}: cpu={cpu_pct:.2f}% ram={ram_mb:.0f}MB"
                  f"{' (warmup)' if i == 0 else ''}", flush=True)
            if i > 0:
                cpu_vals.append(cpu_pct)
                ram_vals.append(ram_mb)
        rows.append({"sim": "fork", "count": n,
                     "cpu_pct": statistics.fmean(cpu_vals) if cpu_vals else 0.0,
                     "peak_ram_mb": statistics.fmean(ram_vals) if ram_vals else 0.0})
        print(f"  => cpu={rows[-1]['cpu_pct']:.2f}% ram={rows[-1]['peak_ram_mb']:.0f}MB", flush=True)
    upsert(rows)
    print(f"\nwrote {CSV_PATH}")


if __name__ == "__main__":
    main()
