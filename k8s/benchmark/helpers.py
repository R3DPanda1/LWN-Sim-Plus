#!/usr/bin/env python3
# Shared helpers for all benchmark scripts.
import os
import re
import subprocess

import requests

SIM_URL = os.environ.get("SIM_URL", "http://localhost:8002/api")
METRICS_URL = os.environ.get("METRICS_URL", "http://localhost:8003/metrics")
CS_URL = os.environ.get("CS_URL", "http://localhost:8090")
NAMESPACE = os.environ.get("NAMESPACE", "lorawan")


def chirpstack_api_key(key_name="bench"):
    r = subprocess.run(
        ["kubectl", "exec", "-n", NAMESPACE, "deployment/chirpstack", "--",
         "chirpstack", "-c", "/etc/chirpstack", "create-api-key", "--name", key_name],
        capture_output=True, text=True, timeout=30,
    )
    for line in (r.stdout + r.stderr).splitlines():
        if line.startswith("token:"):
            return line.split(":", 1)[1].strip()
    raise RuntimeError("could not create ChirpStack API key")


def cs_request(method, path, api_key, **kwargs):
    headers = kwargs.pop("headers", {})
    headers["Grpc-Metadata-Authorization"] = f"Bearer {api_key}"
    headers.setdefault("Content-Type", "application/json")
    return requests.request(method, f"{CS_URL}{path}", headers=headers, timeout=30, **kwargs)


def scrape_metrics(url=None):
    try:
        text = requests.get(url or METRICS_URL, timeout=5).text
    except Exception:
        return {}
    out = {}
    for line in text.splitlines():
        if line.startswith("#"):
            continue
        m = re.match(r"^([a-zA-Z_:][a-zA-Z0-9_:{}=\".,\-+e]*)\s+([\d.eE+\-]+|NaN|\+?Inf|-Inf)$", line)
        if m:
            try:
                out[m.group(1)] = float(m.group(2))
            except ValueError:
                out[m.group(1)] = 0.0
    return out


def scrape_single_metric(name, url=None):
    try:
        r = requests.get(url or METRICS_URL, timeout=5)
    except Exception:
        return 0.0
    for line in r.text.splitlines():
        if line.startswith("#"):
            continue
        m = re.match(rf"^{re.escape(name)}\s+([\d.eE+\-]+)", line)
        if m:
            return float(m.group(1))
    return 0.0


def clean_chirpstack(api_key, app_id):
    total = 0
    while True:
        r = cs_request("GET", f"/api/devices?limit=1000&applicationId={app_id}", api_key).json()
        devs = r.get("result", [])
        if not devs:
            break
        for d in devs:
            cs_request("DELETE", f"/api/devices/{d['devEui']}", api_key)
        total += len(devs)
    return total


def clear_simulator_devices():
    try:
        requests.get(f"{SIM_URL}/stop", timeout=10)
    except Exception:
        pass
    try:
        requests.post(f"{SIM_URL}/del-all-devices", timeout=600)
    except Exception:
        pass


def observe_seconds(count):
    if count <= 100:
        return 60
    if count <= 2000:
        return 120
    return 180


def create_devices_from_template(template_id, count, name_prefix):
    remaining, created, batch = count, 0, 0
    while remaining > 0:
        n = min(remaining, 5000)
        batch += 1
        r = requests.post(
            f"{SIM_URL}/create-devices-from-template",
            json={
                "templateId": template_id, "count": n,
                "namePrefix": f"{name_prefix}-b{batch}",
                "baseLat": 48.2077, "baseLng": 16.375,
                "baseAlt": 0, "spreadMeters": 500,
            },
            timeout=600,
        ).json()
        created += r.get("created", 0)
        remaining -= n
    return created
