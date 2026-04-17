#!/usr/bin/env python3
# ABP scaling benchmark — uplink throughput + CPU/memory per tier.
import csv
import os
import time
from datetime import datetime

import requests

from helpers import (
    SIM_URL, chirpstack_api_key, clean_chirpstack, clear_simulator_devices,
    create_devices_from_template, cs_request, observe_seconds, scrape_metrics,
)

TIERS = [int(x) for x in os.environ.get(
    "TIERS", "1,10,100,500,1000,2000,5000,10000,15000,20000"
).split(",")]
SEND_INTERVAL = int(os.environ.get("SEND_INTERVAL", "10"))
SAMPLE_INTERVAL = int(os.environ.get("SAMPLE_INTERVAL", "10"))


def resolve_ids(api_key):
    tenant = cs_request("GET", "/api/tenants?limit=1", api_key).json()["result"][0]["id"]
    apps = cs_request("GET", f"/api/applications?limit=100&tenantId={tenant}", api_key).json()
    app_id = next(a["id"] for a in apps.get("result", []) if a["name"] == "Benchmark")
    profiles = cs_request("GET", f"/api/device-profiles?limit=100&tenantId={tenant}", api_key).json()
    abp_id = next(p["id"] for p in profiles.get("result", []) if p["name"] == "AM319 ABP")
    return app_id, abp_id


def set_template(abp_profile_id):
    tmpl = requests.get(f"{SIM_URL}/template/1", timeout=10).json()
    if isinstance(tmpl, dict) and len(tmpl) == 1 and "id" not in tmpl:
        tmpl = tmpl[next(iter(tmpl))]
    tmpl["activationMode"] = "abp"
    tmpl["sendInterval"] = SEND_INTERVAL
    tmpl["range"] = 5000
    tmpl["deviceProfileId"] = abp_profile_id
    requests.post(f"{SIM_URL}/update-template", json=tmpl, timeout=30)


def run_tier(count, api_key, app_id, abp_profile_id, ts_writer):
    window = observe_seconds(count)
    print(f"\n=== Tier {count} devices (observe {window}s) ===")

    clear_simulator_devices()
    clean_chirpstack(api_key, app_id)
    set_template(abp_profile_id)

    created = create_devices_from_template(1, count, "scale")
    if created == 0:
        print("  no devices created, skipping")
        return None
    print(f"  created {created}")

    m0 = scrape_metrics()
    up0 = m0.get("gateway_data_sent_total", 0.0)
    dn0 = m0.get("lwnsim_downlinks_total", 0.0)
    cpu0 = m0.get("process_cpu_seconds_total", 0.0)

    requests.get(f"{SIM_URL}/start", timeout=60)
    t_start = time.time()
    last_t, last_cpu = t_start, cpu0
    cpu_samples, mem_samples = [], []

    while time.time() - t_start < window:
        time.sleep(SAMPLE_INTERVAL)
        elapsed = time.time() - t_start
        m = scrape_metrics()
        up = m.get("gateway_data_sent_total", 0.0) - up0
        dn = m.get("lwnsim_downlinks_total", 0.0) - dn0
        cpu_now = m.get("process_cpu_seconds_total", 0.0)
        mem_mb = m.get("process_resident_memory_bytes", 0.0) / (1024 * 1024)
        dt = time.time() - last_t
        cpu_pct = ((cpu_now - last_cpu) / dt * 100) if dt > 0 else 0.0
        last_t, last_cpu = time.time(), cpu_now
        cpu_samples.append(cpu_pct)
        mem_samples.append(mem_mb)
        ts_writer.writerow([
            count, f"{elapsed:.0f}", f"{up:.0f}", f"{dn:.0f}",
            f"{cpu_pct:.1f}", f"{mem_mb:.1f}",
        ])
        rate = up / elapsed if elapsed > 0 else 0.0
        print(f"  [{elapsed:4.0f}s] uplinks={up:.0f} ({rate:.1f}/s) "
              f"cpu={cpu_pct:.1f}% mem={mem_mb:.0f}MB")

    total_elapsed = time.time() - t_start
    m_end = scrape_metrics()
    total_up = m_end.get("gateway_data_sent_total", 0.0) - up0
    total_dn = m_end.get("lwnsim_downlinks_total", 0.0) - dn0
    try:
        requests.get(f"{SIM_URL}/stop", timeout=60)
    except Exception:
        pass
    time.sleep(2)

    return {
        "count": count,
        "observe_sec": window,
        "uplinks": total_up,
        "uplinks_per_sec": total_up / total_elapsed if total_elapsed > 0 else 0.0,
        "downlinks": total_dn,
        "avg_cpu": sum(cpu_samples) / len(cpu_samples) if cpu_samples else 0.0,
        "max_cpu": max(cpu_samples) if cpu_samples else 0.0,
        "avg_mem": sum(mem_samples) / len(mem_samples) if mem_samples else 0.0,
        "max_mem": max(mem_samples) if mem_samples else 0.0,
    }


def main():
    stamp = datetime.now().strftime("%Y%m%d_%H%M%S")
    results_path = f"scaling_results_{stamp}.csv"
    ts_path = f"scaling_timeseries_{stamp}.csv"

    print(f"Scaling benchmark (ABP)")
    print(f"  tiers={TIERS}  send_interval={SEND_INTERVAL}s")
    print(f"  results={results_path}")

    api_key = chirpstack_api_key("scaling-bench")
    app_id, abp_id = resolve_ids(api_key)
    print(f"  app={app_id[:8]} abp_profile={abp_id[:8]}")

    with open(results_path, "w", newline="") as rf, open(ts_path, "w", newline="") as tf:
        rw = csv.writer(rf)
        tw = csv.writer(tf)
        rw.writerow([
            "device_count", "observe_sec",
            "total_uplinks", "uplinks_per_sec", "total_downlinks",
            "avg_cpu_pct", "max_cpu_pct", "avg_mem_mb", "max_mem_mb",
        ])
        tw.writerow([
            "device_count", "elapsed_sec",
            "uplinks", "downlinks", "cpu_pct", "mem_mb",
        ])

        for count in TIERS:
            result = run_tier(count, api_key, app_id, abp_id, tw)
            tf.flush()
            if result:
                rw.writerow([
                    result["count"], result["observe_sec"],
                    f"{result['uplinks']:.0f}", f"{result['uplinks_per_sec']:.2f}",
                    f"{result['downlinks']:.0f}",
                    f"{result['avg_cpu']:.1f}", f"{result['max_cpu']:.1f}",
                    f"{result['avg_mem']:.1f}", f"{result['max_mem']:.1f}",
                ])
                rf.flush()

    print(f"\nDone.  {results_path}")


if __name__ == "__main__":
    main()
