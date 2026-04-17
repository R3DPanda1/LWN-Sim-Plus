#!/usr/bin/env python3
# OTAA benchmark for the upstream simulator.
# Devices created in both sim and ChirpStack manually (no integration).
import base64
import csv
import os
import subprocess
import time
from datetime import datetime

import requests

from helpers import chirpstack_api_key, clean_chirpstack, cs_request

SIM_URL = os.environ.get("SIM_URL", "http://localhost:8002/api")
BRIDGE_HOST = os.environ.get("BRIDGE_HOST", "chirpstack-gateway-bridge")
BRIDGE_PORT = os.environ.get("BRIDGE_PORT", "1700")
NAMESPACE = os.environ.get("NAMESPACE", "lorawan")

TIERS = [int(x) for x in os.environ.get("TIERS", "100,500,1000,2000,5000,10000").split(",")]
# 15min interval keeps data uplinks out of the join window
SEND_INTERVAL = int(os.environ.get("SEND_INTERVAL", "900"))
TIMEOUT_SMALL = int(os.environ.get("TIMEOUT_SMALL", "600"))
TIMEOUT_LARGE = int(os.environ.get("TIMEOUT_LARGE", "1200"))
LARGE_TIER = 1000
POLL_INTERVAL = 10

AM319_HEX = (
    "0367be0004684605000006cb03077dc201087d4600"
    "0973ac260a7d02000b7d0c000c7d0c00"
)
AM319_B64 = base64.b64encode(bytes.fromhex(AM319_HEX)).decode()


def resolve_ids(api_key):
    tenant = cs_request("GET", "/api/tenants?limit=1", api_key).json()["result"][0]["id"]
    apps = cs_request("GET", f"/api/applications?limit=100&tenantId={tenant}", api_key).json()
    app_id = next(a["id"] for a in apps.get("result", []) if a["name"] == "Benchmark")
    profiles = cs_request("GET", f"/api/device-profiles?limit=100&tenantId={tenant}", api_key).json()
    otaa_id = next(p["id"] for p in profiles.get("result", []) if p["name"] == "AM319 OTAA")
    return tenant, app_id, otaa_id


def restart_simulator_pod():
    subprocess.run(
        ["kubectl", "rollout", "restart", "-n", NAMESPACE, "deployment/simulator-upstream"],
        capture_output=True, timeout=30,
    )
    subprocess.run(
        ["kubectl", "rollout", "status", "-n", NAMESPACE, "deployment/simulator-upstream",
         "--timeout=120s"],
        capture_output=True, timeout=150,
    )
    for _ in range(30):
        try:
            requests.get(f"{SIM_URL}/status", timeout=3)
            return
        except Exception:
            time.sleep(1)


def stop_and_clear():
    try:
        requests.get(f"{SIM_URL}/stop", timeout=30)
    except Exception:
        pass
    # No bulk delete in upstream
    try:
        r = requests.get(f"{SIM_URL}/devices", timeout=30, allow_redirects=True)
        devs = r.json() or []
        for d in devs:
            try:
                requests.post(f"{SIM_URL}/del-device",
                              json={"id": d.get("id")}, timeout=10,
                              allow_redirects=True)
            except Exception:
                pass
    except Exception:
        pass


def bootstrap():
    requests.post(
        f"{SIM_URL}/bridge/save",
        json={"ip": BRIDGE_HOST, "port": BRIDGE_PORT},
        timeout=10, allow_redirects=True,
    )
    requests.post(
        f"{SIM_URL}/add-gateway",
        json={"info": {
            "macAddress": "a0b47dc3ca87bcba", "keepAlive": 30, "active": True,
            "typeGateway": False, "name": "UpstreamGW",
            "location": {"latitude": 48.2077, "longitude": 16.375, "altitude": 0},
            "ip": "", "port": "",
        }},
        timeout=10, allow_redirects=True,
    )


def make_sim_device(i):
    dev_eui = f"{i + 1:016x}"
    app_key = f"{i + 1:032x}"
    return {"info": {
        "name": f"otaa-{i+1}",
        "devEUI": dev_eui, "devAddr": "00000000",
        "nwkSKey": "00" * 16, "appSKey": "00" * 16, "appKey": app_key,
        "status": {
            "active": True, "mtype": "UnConfirmedDataUp",
            "payload": AM319_B64, "base64": True,
            "infoUplink": {"FPort": 85, "DataRate": 5, "ADR": {}, "FOpts": []},
            "fcntDown": 0,
        },
        "configuration": {
            "region": 1, "sendInterval": SEND_INTERVAL, "ackTimeout": 2,
            "range": 5000, "supportedOtaa": True, "supportedADR": True,
            "supportedFragment": False, "supportedClassB": False,
            "supportedClassC": False, "dataRate": 5, "rx1DROffset": 0,
            "nbRetransmission": 1,
        },
        "location": {"latitude": 48.2077, "longitude": 16.375, "altitude": 0},
        "rxs": [
            {"delay": 1000, "durationOpen": 3000, "dataRate": 5,
             "channel": {"active": True, "enableUplink": True,
                         "freqUplink": 868100000, "freqDownlink": 868100000,
                         "minDR": 0, "maxDR": 5}},
            {"delay": 2000, "durationOpen": 3000, "dataRate": 0,
             "channel": {"active": True, "enableUplink": False,
                         "freqUplink": 0, "freqDownlink": 869525000,
                         "minDR": 0, "maxDR": 0}},
        ],
    }}


def create_devices(count, api_key, app_id, otaa_profile_id):
    t0 = time.time()
    sim_fail, cs_fail = 0, 0
    for i in range(count):
        dev_eui = f"{i + 1:016x}"
        app_key = f"{i + 1:032x}"
        try:
            r = requests.post(f"{SIM_URL}/add-device", json=make_sim_device(i),
                              timeout=30, allow_redirects=True)
            if '"code":0' not in r.text:
                sim_fail += 1
                if sim_fail <= 3:
                    print(f"  sim add-device {i} failed: {r.text[:140]}")
        except Exception as e:
            sim_fail += 1
            if sim_fail <= 3:
                print(f"  sim exception {i}: {e}")
        try:
            cs_request("POST", "/api/devices", api_key, json={"device": {
                "applicationId": app_id, "description": "",
                "devEui": dev_eui, "deviceProfileId": otaa_profile_id,
                "isDisabled": False, "name": f"otaa-{i+1}", "skipFcntCheck": True,
            }})
            cs_request("POST", f"/api/devices/{dev_eui}/keys", api_key, json={
                "deviceKeys": {"devEui": dev_eui, "nwkKey": app_key},
            })
        except Exception as e:
            cs_fail += 1
            if cs_fail <= 3:
                print(f"  cs exception {i}: {e}")
        if (i + 1) % 500 == 0:
            print(f"  created {i+1}/{count} ({time.time()-t0:.1f}s)")
    elapsed = time.time() - t0
    print(f"  provisioned {count} in {elapsed:.1f}s (sim_fail={sim_fail}, cs_fail={cs_fail})")
    return elapsed, sim_fail, cs_fail


def count_activated(api_key, app_id, count):
    activated = 0
    offset = 0
    while offset < count:
        r = cs_request("GET",
                       f"/api/devices?limit=100&offset={offset}&applicationId={app_id}",
                       api_key).json()
        devs = r.get("result", [])
        if not devs:
            break
        for d in devs:
            try:
                ar = cs_request("GET", f"/api/devices/{d['devEui']}/activation", api_key)
                act = ar.json().get("deviceActivation", {})
                if act.get("devAddr") and act["devAddr"] != "00000000":
                    activated += 1
            except Exception:
                pass
        offset += len(devs)
    return activated


def run_tier(count, api_key, app_id, otaa_profile_id, ts_writer):
    timeout = TIMEOUT_SMALL if count <= LARGE_TIER else TIMEOUT_LARGE
    print(f"\n=== Tier {count} devices (timeout {timeout}s) ===")

    stop_and_clear()
    clean_chirpstack(api_key, app_id)
    bootstrap()
    time.sleep(2)

    create_sec, sim_fail, cs_fail = create_devices(count, api_key, app_id, otaa_profile_id)
    target = count - sim_fail
    if target <= 0:
        return None

    requests.get(f"{SIM_URL}/start", timeout=60)
    t_start = time.time()
    last_joins, stall = 0, 0

    while True:
        elapsed = time.time() - t_start
        if elapsed >= timeout:
            print(f"  TIMEOUT after {elapsed:.0f}s")
            break
        time.sleep(POLL_INTERVAL)
        elapsed = time.time() - t_start
        joins = count_activated(api_key, app_id, count)
        pct = joins / target * 100 if target else 0
        ts_writer.writerow([count, f"{elapsed:.0f}", joins, f"{pct:.1f}"])
        print(f"  [{elapsed:4.0f}s] joins={joins}/{target} ({pct:.1f}%)")
        if joins >= target:
            print(f"  ALL JOINED in {elapsed:.1f}s")
            break
        # Give up if no progress for 10 consecutive polls
        if joins == last_joins:
            stall += 1
            if stall >= 10:
                print(f"  Stalled, giving up")
                break
        else:
            stall = 0
        last_joins = joins

    try:
        requests.get(f"{SIM_URL}/stop", timeout=30)
    except Exception:
        pass

    final_elapsed = time.time() - t_start
    final_joins = count_activated(api_key, app_id, count)
    final_pct = final_joins / target * 100 if target else 0
    print(f"  Result: {final_joins}/{target} ({final_pct:.1f}%) in {final_elapsed:.1f}s")

    return {
        "count": count, "target": target, "joins": final_joins,
        "pct": final_pct, "elapsed": final_elapsed,
        "create_sec": create_sec, "sim_fail": sim_fail, "cs_fail": cs_fail,
    }


def main():
    stamp = datetime.now().strftime("%Y%m%d_%H%M%S")
    rp = f"upstream_otaa_results_{stamp}.csv"
    tp = f"upstream_otaa_timeseries_{stamp}.csv"
    print(f"Upstream OTAA join benchmark")
    print(f"  tiers={TIERS}  send_interval={SEND_INTERVAL}s")

    api_key = chirpstack_api_key("upstream-otaa-bench")
    _, app_id, otaa_id = resolve_ids(api_key)
    print(f"  app={app_id[:8]}  otaa_profile={otaa_id[:8]}")

    with open(rp, "w", newline="") as rf, open(tp, "w", newline="") as tf:
        rw = csv.writer(rf)
        tw = csv.writer(tf)
        rw.writerow(["device_count", "target", "joins_completed", "completion_pct",
                      "elapsed_sec", "create_sec", "sim_fail", "cs_fail"])
        tw.writerow(["device_count", "elapsed_sec", "joins", "pct"])

        for count in TIERS:
            try:
                result = run_tier(count, api_key, app_id, otaa_id, tw)
                rf.flush(); tf.flush()
                if result:
                    rw.writerow([result["count"], result["target"], result["joins"],
                                 f"{result['pct']:.1f}", f"{result['elapsed']:.1f}",
                                 f"{result['create_sec']:.1f}",
                                 result["sim_fail"], result["cs_fail"]])
                    if result["pct"] < 10 and count >= 500:
                        print(f"  Only {result['pct']:.0f}% joined, stopping early")
                        break
            except Exception as e:
                print(f"  tier {count} aborted: {e}")

    print(f"\nDone.  {rp}")


if __name__ == "__main__":
    main()
