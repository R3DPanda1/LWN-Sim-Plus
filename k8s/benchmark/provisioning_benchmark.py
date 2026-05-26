#!/usr/bin/env python3
"""Provisioning benchmark — final benchmark.

Creates then deletes 5000 ABP AM319 devices via simulator REST in three
integration modes: bare (none), cs (ChirpStack only), cs+tb (ChirpStack
plus ThingsBoard). Measures total wall time for create and delete.

Runs on the Hetzner benchmark host; all services reachable on localhost
via compose publish or the hetzner SSH tunnel.

Outputs results/provisioning/provisioning.csv with one row per
(mode, operation).
"""
import csv
import json
import os
import time
from pathlib import Path

import requests

SIM    = os.environ.get("SIM",   "http://localhost:8002/api")
CS     = os.environ.get("CS",    "http://localhost:8090")
TB     = os.environ.get("TB",    "http://localhost:9090")
COUNTS = [int(x) for x in os.environ.get("COUNTS", "1000,5000,10000").split(",")]
MODES  = [m.strip() for m in os.environ.get("MODES", "bare,cs,cs+tb").split(",") if m.strip()]

HERE = Path(__file__).resolve().parent
OUT_DIR = HERE / "results" / "provisioning"
OUT_DIR.mkdir(parents=True, exist_ok=True)
OUT_CSV = Path(os.environ.get("OUT_CSV", OUT_DIR / "provisioning.csv"))

# bench_config.json (written by seed.py) carries the ChirpStack API key and IDs.
BENCH_ROOT = Path(os.environ.get("BENCH_ROOT", HERE.parent.parent / "benchmark"))
_bcfg = BENCH_ROOT / "bench_config.json"
BENCH_CONFIG = json.load(open(_bcfg)) if _bcfg.exists() else {}
TB_API_KEY = os.environ.get("TB_API_KEY") or BENCH_CONFIG.get("tb_api_key", "")
TB_INTG_ID = BENCH_CONFIG.get("sim_tb_integration_id") or 1


def die(msg):
    raise SystemExit(f"FATAL: {msg}")


def get_template(tid=1):
    r = requests.get(f"{SIM}/template/{tid}", timeout=10)
    r.raise_for_status()
    j = r.json()
    return j.get("template", j)


def update_template(tmpl):
    r = requests.post(f"{SIM}/update-template", json=tmpl, timeout=30)
    if r.status_code != 200:
        die(f"update-template failed: {r.status_code} {r.text}")


def get_tb_default_profile_id():
    tb_key = TB_API_KEY
    if not tb_key:
        return ""
    r = requests.get(
        f"{TB}/api/deviceProfiles?pageSize=100&page=0",
        headers={"X-Authorization": f"ApiKey {tb_key}"},
        timeout=10)
    if r.status_code != 200:
        return ""
    for p in r.json().get("data", []):
        if p.get("default"):
            return p["id"]["id"]
    return ""


TB_PROFILE_ID = get_tb_default_profile_id()


def set_template_mode(mode):
    """mode in {'bare','cs','cs+tb'}."""
    t = get_template(1)
    t["activationMode"]      = "abp"
    t["sendInterval"]        = 300
    t["deviceProfileId"]     = BENCH_CONFIG.get("cs_abp_profile_id", t.get("deviceProfileId", ""))
    t["integrationEnabled"]  = mode in ("cs", "cs+tb")
    t["integrationId"]       = 0
    t["tbIntegrationEnabled"]= mode == "cs+tb"
    t["tbIntegrationId"]     = TB_INTG_ID
    t["tbDeviceProfileId"]   = TB_PROFILE_ID if mode == "cs+tb" else ""
    update_template(t)


def cs_device_count():
    key = BENCH_CONFIG.get("cs_api_key", "")
    app = BENCH_CONFIG.get("cs_app_id", "")
    if not key or not app:
        return -1
    r = requests.get(
        f"{CS}/api/devices?applicationId={app}&limit=1",
        headers={"Grpc-Metadata-Authorization": f"Bearer {key}"},
        timeout=10)
    if r.status_code != 200:
        return -1
    return int(r.json().get("totalCount", -1))


def tb_device_count():
    tb_key = TB_API_KEY
    if not tb_key:
        return -1
    r = requests.get(
        f"{TB}/api/tenant/devices?pageSize=1&page=0",
        headers={"X-Authorization": f"ApiKey {tb_key}"},
        timeout=10)
    if r.status_code != 200:
        return -1
    return int(r.json().get("totalElements", -1))


def create_devices(n, mode):
    body = {
        "templateId":   1,
        "count":        n,
        "namePrefix":   f"Prov-{mode.replace('+','-')}",
        "baseLat":      48.207736,
        "baseLng":      16.374783,
        "baseAlt":      0,
        "spreadMeters": 100,
    }
    t0 = time.monotonic()
    r = requests.post(f"{SIM}/create-devices-from-template",
                      json=body, timeout=3600)
    t1 = time.monotonic()
    if r.status_code != 200:
        die(f"create failed: {r.status_code} {r.text[:400]}")
    created = r.json().get("created", 0)
    if created != n:
        die(f"create returned {created} (expected {n})")
    return t1 - t0


def delete_all():
    t0 = time.monotonic()
    r = requests.post(f"{SIM}/del-all-devices", timeout=3600)
    t1 = time.monotonic()
    if r.status_code != 200:
        die(f"del-all-devices failed: {r.status_code} {r.text[:400]}")
    return t1 - t0


def sim_device_count():
    r = requests.get(f"{SIM}/devices?page=1&limit=1", timeout=10)
    r.raise_for_status()
    j = r.json()
    if j is None:
        return 0
    if isinstance(j, list):
        return len(j)
    return int(j.get("total", j.get("totalCount", 0)))


def run_cell(mode, n):
    print(f"\n=== mode={mode} N={n} ===", flush=True)
    set_template_mode(mode)

    if sim_device_count() > 0:
        print("  prior devices in sim; deleting...", flush=True)
        delete_all()

    print(f"  creating {n} devices...", flush=True)
    create_sec = create_devices(n, mode)
    sim_n = sim_device_count()
    cs_n  = cs_device_count()
    tb_n  = tb_device_count()
    print(f"  created in {create_sec:.2f}s  sim={sim_n} cs={cs_n} tb={tb_n}",
          flush=True)

    print(f"  deleting {n} devices...", flush=True)
    delete_sec = delete_all()
    sim_after  = sim_device_count()
    cs_after   = cs_device_count()
    tb_after   = tb_device_count()
    print(f"  deleted in {delete_sec:.2f}s  sim={sim_after} cs={cs_after} tb={tb_after}",
          flush=True)

    return {
        "mode":          mode,
        "count":         n,
        "create_sec":    round(create_sec, 3),
        "delete_sec":    round(delete_sec, 3),
        "sim_after_create": sim_n,
        "cs_after_create":  cs_n,
        "tb_after_create":  tb_n,
        "sim_after_delete": sim_after,
        "cs_after_delete":  cs_after,
        "tb_after_delete":  tb_after,
    }


def main():
    rows = []
    for n in COUNTS:
        for mode in MODES:
            rows.append(run_cell(mode, n))

    fields = list(rows[0].keys())
    with open(OUT_CSV, "w", newline="") as f:
        w = csv.DictWriter(f, fieldnames=fields)
        w.writeheader()
        w.writerows(rows)
    print(f"\nwrote {OUT_CSV}")
    print("\nsummary:")
    print(f"  {'mode':7s} {'N':>6s} {'create_s':>10s} {'delete_s':>10s}")
    for r in rows:
        print(f"  {r['mode']:7s} {r['count']:>6d} {r['create_sec']:>10.2f} {r['delete_sec']:>10.2f}")


if __name__ == "__main__":
    main()
