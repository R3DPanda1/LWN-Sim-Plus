"""ABP emission-ceiling sweep: fleet size versus CPU and peak RAM on one core.

AM319 ABP, single burst against a bridge-only sink (integration off), CPU from
the Prometheus counter and peak RAM from the container cgroup. Writes the
per-tier summary that plot_abp_scaling.py renders into abp_scaling.png. The
simulator container is held to one core by the compose stack.

  python3 axis_a_sweep.py                                  # paper tiers, 5 reps
  ABP_TIERS=50 REPS=2 OUT_JSON=/tmp/s.json python3 axis_a_sweep.py   # quick check
"""
import json
import statistics
import sys
import time
from pathlib import Path

import requests

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE.parent))
from common import (env, cs_profile_id, sim_configure_template, sim_create,
                    sim_start, sim_stop, sim_del_all, container_peak_ram_mb,
                    reset_peak_ram, sim_container, METRICS_URL)

TIERS = [int(x) for x in env("ABP_TIERS", "1000,5000,10000,20000,40000").split(",")]
REPS  = int(env("REPS", "5"))
OUT   = Path(env("OUT_JSON", HERE / "results" / "bench_v2" / "axis_a" / "K1" / "summary.json"))


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
    sim_create(n, "Bench", spread_m=100)
    reset_peak_ram(container)

    cpu0, gw0 = prom("process_cpu_seconds_total"), prom("gateway_data_sent_total")
    t0 = time.time()
    sim_start()
    time.sleep(80 if n <= 10_000 else 90 if n <= 40_000 else 120)
    sim_stop()
    wall = time.time() - t0

    sent = int(prom("gateway_data_sent_total") - gw0)
    cpu_pct = 100.0 * (prom("process_cpu_seconds_total") - cpu0) / wall if wall else 0.0
    loss = 100.0 * max(0, n - sent) / n if n else 0.0
    return {"sent": sent, "cpu_pct": cpu_pct, "ram_mb": container_peak_ram_mb(container), "loss": loss}


def main():
    profile_id = cs_profile_id(env("ABP_PROFILE_NAME", "AM319 ABP"))
    container = sim_container()
    summary = []
    for n in TIERS:
        print(f"\n--- N={n} ({REPS} reps) ---", flush=True)
        reps = []
        for i in range(REPS):
            r = burst(n, profile_id, container)
            print(f"  rep {i+1}: sent={r['sent']} cpu={r['cpu_pct']:.2f}% "
                  f"ram={r['ram_mb']:.0f}MB loss={r['loss']:.2f}%", flush=True)
            reps.append(r)
        scored = reps[1:] if len(reps) > 1 else reps   # drop warm-up

        def agg(key):
            vals = [r[key] for r in scored]
            return statistics.fmean(vals), (statistics.pstdev(vals) if len(vals) > 1 else 0.0)

        cpu_m, cpu_sd = agg("cpu_pct")
        ram_m, ram_sd = agg("ram_mb")
        sent_m, _ = agg("sent")
        summary.append({"N": n, "cpu_pct_mean": cpu_m, "cpu_pct_sd": cpu_sd,
                        "ram_peak_mean": ram_m, "ram_peak_sd": ram_sd,
                        "loss_mean": statistics.fmean([r["loss"] for r in scored]),
                        "sent_mean": sent_m, "recv_mean": sent_m})
        print(f"  => cpu={cpu_m:.2f}±{cpu_sd:.2f}% ram={ram_m:.0f}±{ram_sd:.0f}MB", flush=True)

    OUT.parent.mkdir(parents=True, exist_ok=True)
    OUT.write_text(json.dumps(summary, indent=2))
    print(f"\nwrote {OUT}")


if __name__ == "__main__":
    main()
