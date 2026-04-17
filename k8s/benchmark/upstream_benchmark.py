#!/usr/bin/env python3
# CPU/memory benchmark for the unmodified upstream LWN-Simulator.
# Creates devices one by one (no bulk endpoint). Does not use ChirpStack.
import base64
import csv
import os
import re
import time
from datetime import datetime

import requests

SIM_URL = os.environ.get("SIM_URL", "http://localhost:8002/api")
METRICS_URL = os.environ.get("METRICS_URL", "http://localhost:8003/metrics")
BRIDGE_HOST = os.environ.get("BRIDGE_HOST", "chirpstack-gateway-bridge")
BRIDGE_PORT = os.environ.get("BRIDGE_PORT", "1700")

TIERS = [int(x) for x in os.environ.get("TIERS", "100,500,1000,2000").split(",")]
SEND_INTERVAL = int(os.environ.get("SEND_INTERVAL", "60"))
SAMPLE_INTERVAL = int(os.environ.get("SAMPLE_INTERVAL", "10"))

AM319_HEX = (
    "0367be0004684605000006cb03077dc201087d4600"
    "0973ac260a7d02000b7d0c000c7d0c00"
)
AM319_B64 = base64.b64encode(bytes.fromhex(AM319_HEX)).decode()


def observe_seconds(count):
    if count <= 1000:
        return 180
    return 240


def scrape_metrics():
    """Simpler regex than helpers.py -- upstream doesn't emit label-bearing metrics."""
    try:
        text = requests.get(METRICS_URL, timeout=5).text
    except Exception:
        return {}
    out = {}
    for line in text.splitlines():
        if line.startswith("#"):
            continue
        m = re.match(r"^([a-zA-Z_:][a-zA-Z0-9_:]*)\s+([\d.eE+\-]+)$", line)
        if m:
            try:
                out[m.group(1)] = float(m.group(2))
            except ValueError:
                pass
    return out


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


def device_payload(index):
    dev_eui = f"{index + 1:016x}"
    dev_addr = f"{(0x01000000 + index) & 0xFFFFFFFF:08x}"
    key = f"{index + 1:032x}"
    return {"info": {
        "name": f"upstream-{index + 1}",
        "devEUI": dev_eui, "devAddr": dev_addr,
        "nwkSKey": key, "appSKey": key, "appKey": "00" * 16,
        "status": {
            "active": True,
            "mtype": "UnConfirmedDataUp",
            "payload": AM319_B64,
            "base64": True,
            "infoUplink": {"FPort": 85, "DataRate": 5, "ADR": {}, "FOpts": []},
            "fcntDown": 0,
        },
        "configuration": {
            "region": 1, "sendInterval": SEND_INTERVAL, "ackTimeout": 2,
            "range": 5000, "supportedOtaa": False, "supportedADR": True,
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


def stop_and_clear():
    try:
        requests.get(f"{SIM_URL}/stop", timeout=30)
    except Exception:
        pass
    # No bulk delete in upstream; iterate and delete each device
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


def create_devices(count):
    t0 = time.time()
    failed = 0
    for i in range(count):
        try:
            r = requests.post(f"{SIM_URL}/add-device", json=device_payload(i),
                              timeout=30, allow_redirects=True)
            if '"code":0' not in r.text:
                failed += 1
                if failed <= 3:
                    print(f"  add-device {i} failed: {r.status_code} {r.text[:140]}")
        except Exception as e:
            failed += 1
            if failed <= 3:
                print(f"  add-device {i} exception: {e}")
        if (i + 1) % 500 == 0:
            print(f"  created {i + 1}/{count} ({time.time() - t0:.1f}s, fail={failed})")
    return time.time() - t0, failed


def run_tier(count, ts_writer):
    window = observe_seconds(count)
    print(f"\n=== Tier {count} devices (observe {window}s) ===")
    stop_and_clear()
    bootstrap()

    create_sec, failed = create_devices(count)
    print(f"  created {count - failed}/{count} in {create_sec:.1f}s")

    m0 = scrape_metrics()
    cpu0 = m0.get("process_cpu_seconds_total", 0.0)

    requests.get(f"{SIM_URL}/start", timeout=60)
    t_start = time.time()
    last_t, last_cpu = t_start, cpu0
    cpu_samples, mem_samples = [], []

    while time.time() - t_start < window:
        time.sleep(SAMPLE_INTERVAL)
        elapsed = time.time() - t_start
        m = scrape_metrics()
        cpu_now = m.get("process_cpu_seconds_total", 0.0)
        mem_mb = m.get("process_resident_memory_bytes", 0.0) / (1024 * 1024)
        dt = time.time() - last_t
        cpu_pct = ((cpu_now - last_cpu) / dt * 100) if dt > 0 else 0.0
        last_t, last_cpu = time.time(), cpu_now
        cpu_samples.append(cpu_pct)
        mem_samples.append(mem_mb)
        ts_writer.writerow([count, f"{elapsed:.0f}", f"{cpu_pct:.1f}", f"{mem_mb:.1f}"])
        print(f"  [{elapsed:4.0f}s] cpu={cpu_pct:.1f}% mem={mem_mb:.0f}MB")

    stop_and_clear()

    return {
        "count": count,
        "observe_sec": window,
        "create_sec": create_sec,
        "create_failed": failed,
        "avg_cpu": sum(cpu_samples) / len(cpu_samples) if cpu_samples else 0.0,
        "max_cpu": max(cpu_samples) if cpu_samples else 0.0,
        "avg_mem": sum(mem_samples) / len(mem_samples) if mem_samples else 0.0,
        "max_mem": max(mem_samples) if mem_samples else 0.0,
    }


def main():
    stamp = datetime.now().strftime("%Y%m%d_%H%M%S")
    results_path = f"upstream_results_{stamp}.csv"
    ts_path = f"upstream_timeseries_{stamp}.csv"

    print("Upstream LWN-Simulator benchmark (static AM319 payload)")
    print(f"  tiers={TIERS}  send_interval={SEND_INTERVAL}s")
    print(f"  results={results_path}")

    with open(results_path, "w", newline="") as rf, open(ts_path, "w", newline="") as tf:
        rw = csv.writer(rf)
        tw = csv.writer(tf)
        rw.writerow([
            "device_count", "observe_sec", "create_sec", "create_failed",
            "avg_cpu_pct", "max_cpu_pct", "avg_mem_mb", "max_mem_mb",
        ])
        tw.writerow(["device_count", "elapsed_sec", "cpu_pct", "mem_mb"])

        for count in TIERS:
            try:
                r = run_tier(count, tw)
            except Exception as e:
                print(f"  tier {count} aborted: {e}")
                continue
            tf.flush()
            rw.writerow([
                r["count"], r["observe_sec"],
                f"{r['create_sec']:.1f}", r["create_failed"],
                f"{r['avg_cpu']:.1f}", f"{r['max_cpu']:.1f}",
                f"{r['avg_mem']:.1f}", f"{r['max_mem']:.1f}",
            ])
            rf.flush()

    print(f"\nDone.  {results_path}")


if __name__ == "__main__":
    main()
