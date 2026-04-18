#!/usr/bin/env python3
# OTAA end-to-end benchmark — measures time until all devices have a
# lastSeenAt timestamp in ChirpStack (i.e. joined AND delivered one uplink).
import csv
import os
import time
from datetime import datetime

import requests

from helpers import (
    SIM_URL, chirpstack_api_key, clean_chirpstack, clear_simulator_devices,
    create_devices_from_template, cs_request,
)

TIERS = [int(x) for x in os.environ.get("TIERS", "100,500,1000,2000,5000").split(",")]
SEND_INTERVAL = int(os.environ.get("SEND_INTERVAL", "60"))
TIMEOUT_SMALL = int(os.environ.get("TIMEOUT_SMALL", "600"))
TIMEOUT_LARGE = int(os.environ.get("TIMEOUT_LARGE", "1200"))
LARGE_TIER = 2000
POLL_INTERVAL = 5


def resolve_ids(api_key):
    tenant = cs_request("GET", "/api/tenants?limit=1", api_key).json()["result"][0]["id"]
    apps = cs_request("GET", f"/api/applications?limit=100&tenantId={tenant}", api_key).json()
    app_id = next(a["id"] for a in apps.get("result", []) if a["name"] == "Benchmark")
    profiles = cs_request("GET", f"/api/device-profiles?limit=100&tenantId={tenant}", api_key).json()
    otaa_id = next(p["id"] for p in profiles.get("result", []) if p["name"] == "AM319 OTAA")
    return tenant, app_id, otaa_id


def set_template(otaa_profile_id):
    tmpl = requests.get(f"{SIM_URL}/template/1", timeout=10).json()
    if isinstance(tmpl, dict) and len(tmpl) == 1 and "id" not in tmpl:
        tmpl = tmpl[next(iter(tmpl))]
    tmpl["activationMode"] = "otaa"
    tmpl["sendInterval"] = SEND_INTERVAL
    tmpl["range"] = 5000
    tmpl["deviceProfileId"] = otaa_profile_id
    r = requests.post(f"{SIM_URL}/update-template", json=tmpl, timeout=30)
    if r.status_code != 200:
        raise SystemExit(f"update-template failed: {r.status_code} {r.text}")


def count_last_seen(api_key, app_id, expected):
    seen = 0
    offset = 0
    while True:
        r = cs_request("GET",
                       f"/api/devices?limit=500&offset={offset}&applicationId={app_id}",
                       api_key).json()
        result = r.get("result", [])
        if not result:
            break
        seen += sum(1 for d in result if d.get("lastSeenAt"))
        if len(result) < 500:
            break
        offset += len(result)
        if offset > expected * 3:
            break
    return seen


def run_tier(count, api_key, app_id, otaa_profile_id, timeout, ts_writer):
    print(f"\n=== Tier {count} devices (timeout {timeout}s) ===")
    clear_simulator_devices()
    clean_chirpstack(api_key, app_id)
    set_template(otaa_profile_id)

    created = create_devices_from_template(1, count, "bench")
    if created == 0:
        print("  no devices created, skipping")
        return None
    print(f"  created {created}")

    requests.get(f"{SIM_URL}/start", timeout=60)
    t_start = time.time()
    complete_sec, max_seen = None, 0

    while time.time() - t_start < timeout:
        time.sleep(POLL_INTERVAL)
        elapsed = time.time() - t_start
        seen = count_last_seen(api_key, app_id, count)
        max_seen = max(max_seen, seen)
        ts_writer.writerow([count, f"{elapsed:.1f}", seen])
        pct = seen / count * 100 if count else 0
        print(f"  [{elapsed:5.0f}s] {seen}/{count} ({pct:.0f}%)")
        if complete_sec is None and seen >= count:
            complete_sec = elapsed
            break

    try:
        requests.get(f"{SIM_URL}/stop", timeout=60)
    except Exception:
        pass
    time.sleep(3)

    pct = max_seen / count * 100 if count else 0
    if complete_sec is None:
        complete_sec = -1
        print(f"  timeout: {max_seen}/{count} ({pct:.0f}%)")
    else:
        print(f"  all devices active in {complete_sec:.1f}s")
    return {"count": count, "complete_sec": complete_sec, "complete_pct": pct}


def main():
    stamp = datetime.now().strftime("%Y%m%d_%H%M%S")
    results_path = f"otaa_results_{stamp}.csv"
    ts_path = f"otaa_timeseries_{stamp}.csv"

    print("OTAA end-to-end benchmark (ChirpStack lastSeenAt)")
    print(f"  tiers={TIERS}  send_interval={SEND_INTERVAL}s  poll={POLL_INTERVAL}s")
    print(f"  results={results_path}")

    api_key = os.environ.get("CS_API_KEY") or chirpstack_api_key("otaa-bench")
    tenant, app_id, otaa_id = resolve_ids(api_key)
    print(f"  tenant={tenant[:8]} app={app_id[:8]} otaa_profile={otaa_id[:8]}")

    with open(results_path, "w", newline="") as rf, open(ts_path, "w", newline="") as tf:
        rw = csv.writer(rf)
        tw = csv.writer(tf)
        rw.writerow(["device_count", "complete_sec", "complete_pct"])
        tw.writerow(["device_count", "elapsed_sec", "last_seen"])

        for count in TIERS:
            timeout = TIMEOUT_SMALL if count <= LARGE_TIER else TIMEOUT_LARGE
            result = run_tier(count, api_key, app_id, otaa_id, timeout, tw)
            tf.flush()
            if result:
                rw.writerow([result["count"], f"{result['complete_sec']:.1f}",
                             f"{result['complete_pct']:.0f}"])
                rf.flush()

    print(f"\nDone.  {results_path}")


if __name__ == "__main__":
    main()
