"""Shared helpers for the benchmark drivers.

Config comes from the benchmark/ deploy dir: bench_config.json (written by
seed.py) supplies the ChirpStack key and URLs, an optional .env overrides them,
and process env wins over both. ChirpStack IDs are looked up by name at runtime
so a reseeded stack keeps working.
"""
import os
import subprocess
import threading
import time
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path

import requests

HERE = Path(__file__).resolve().parent
# Deploy dir with the compose stack, bench_config.json, and optional .env.
BENCH_ROOT = Path(os.environ.get("BENCH_ROOT", HERE.parents[1] / "benchmark"))

_dotenv = {}
_envfile = BENCH_ROOT / ".env"
if _envfile.exists():
    for line in _envfile.read_text().splitlines():
        line = line.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        k, v = line.split("=", 1)
        v = v.strip()
        if len(v) >= 2 and v[0] == v[-1] and v[0] in "\"'":
            v = v[1:-1]
        _dotenv[k.strip()] = v

_bcfg = {}
if (BENCH_ROOT / "bench_config.json").exists():
    import json
    _raw = json.loads((BENCH_ROOT / "bench_config.json").read_text())
    _bcfg = {k: v for k, v in {
        "CS_URL": _raw.get("cs_url"), "CS_API_KEY": _raw.get("cs_api_key"),
        "SIM_URL": _raw.get("sim_url"), "METRICS_URL": _raw.get("metrics_url"),
    }.items() if v}


def env(key, default=None):
    return os.environ.get(key, _dotenv.get(key, _bcfg.get(key, default)))


CS_URL = env("CS_URL", "http://localhost:8090")
SIM_URL = env("SIM_URL", "http://localhost:8002/api")
METRICS_URL = env("METRICS_URL", "http://localhost:8003/metrics")
TB_URL = env("TB_URL", "http://localhost:9090")

CS_HDR = {
    "Grpc-Metadata-Authorization": f"Bearer {env('CS_API_KEY', '')}",
    "Content-Type": "application/json",
}
TB_HDR = {"X-Authorization": f"ApiKey {env('TB_API_KEY', '')}"}


# ---------- ChirpStack ----------

_cache = {}


def cs_tenant_id():
    if "tenant" not in _cache:
        r = requests.get(f"{CS_URL}/api/tenants", params={"limit": 100},
                         headers=CS_HDR, timeout=30)
        r.raise_for_status()
        _cache["tenant"] = r.json()["result"][0]["id"]
    return _cache["tenant"]


def cs_app_id():
    if "app" not in _cache:
        name = env("APP_NAME", "Benchmark")
        r = requests.get(f"{CS_URL}/api/applications",
                         params={"tenantId": cs_tenant_id(), "limit": 100},
                         headers=CS_HDR, timeout=30)
        r.raise_for_status()
        match = [a for a in r.json()["result"] if a["name"] == name]
        if not match:
            raise SystemExit(f"no ChirpStack application named {name!r}")
        _cache["app"] = match[0]["id"]
    return _cache["app"]


def cs_profile_id(name):
    key = f"profile:{name}"
    if key not in _cache:
        r = requests.get(f"{CS_URL}/api/device-profiles",
                         params={"tenantId": cs_tenant_id(), "limit": 100},
                         headers=CS_HDR, timeout=30)
        r.raise_for_status()
        match = [p for p in r.json()["result"] if p["name"] == name]
        if not match:
            raise SystemExit(f"no ChirpStack device profile named {name!r}")
        _cache[key] = match[0]["id"]
    return _cache[key]


def cs_device_count():
    r = requests.get(f"{CS_URL}/api/devices",
                     params={"applicationId": cs_app_id(), "limit": 1},
                     headers=CS_HDR, timeout=30)
    r.raise_for_status()
    return int(r.json().get("totalCount", 0))


def cs_list_euis():
    euis, offset = [], 0
    while True:
        r = requests.get(f"{CS_URL}/api/devices",
                         params={"applicationId": cs_app_id(),
                                 "limit": 100, "offset": offset},
                         headers=CS_HDR, timeout=60)
        r.raise_for_status()
        devs = r.json().get("result", [])
        if not devs:
            break
        euis.extend(d["devEui"] for d in devs)
        offset += len(devs)
    return euis


def cs_last_seen_count():
    seen, offset = 0, 0
    while True:
        r = requests.get(f"{CS_URL}/api/devices",
                         params={"applicationId": cs_app_id(),
                                 "limit": 100, "offset": offset},
                         headers=CS_HDR, timeout=60)
        r.raise_for_status()
        devs = r.json().get("result", [])
        if not devs:
            break
        seen += sum(1 for d in devs if d.get("lastSeenAt"))
        offset += len(devs)
    return seen


def cs_devaddr_count(euis):
    def activated(eui):
        r = requests.get(f"{CS_URL}/api/devices/{eui}/activation",
                         headers=CS_HDR, timeout=30)
        if r.status_code != 200:
            return 0
        act = r.json().get("deviceActivation") or {}
        return 1 if act.get("devAddr") else 0
    with ThreadPoolExecutor(max_workers=20) as ex:
        return sum(ex.map(activated, euis))


# ---------- ThingsBoard ----------

def tb_default_profile_id():
    if not env("TB_API_KEY"):
        return ""
    try:
        r = requests.get(f"{TB_URL}/api/deviceProfiles",
                         params={"pageSize": 100, "page": 0},
                         headers=TB_HDR, timeout=30)
    except Exception:
        return ""
    if r.status_code != 200:
        return ""
    for p in r.json().get("data", []):
        if p.get("default"):
            return p["id"]["id"]
    return ""


def tb_device_count():
    if not env("TB_API_KEY"):
        return -1
    try:
        r = requests.get(f"{TB_URL}/api/tenant/devices",
                         params={"pageSize": 1, "page": 0},
                         headers=TB_HDR, timeout=30)
    except Exception:
        return -1
    if r.status_code != 200:
        return -1
    return int(r.json().get("totalElements", -1))


# ---------- Simulator ----------

def sim_get_template(tid=1):
    t = requests.get(f"{SIM_URL}/template/{tid}", timeout=30).json()
    if isinstance(t, dict) and "template" in t:
        return t["template"]
    if isinstance(t, dict) and len(t) == 1 and "id" not in t:
        return t[next(iter(t))]
    return t


def sim_update_template(tmpl):
    r = requests.post(f"{SIM_URL}/update-template", json=tmpl, timeout=30)
    if r.status_code != 200:
        raise SystemExit(f"update-template failed: {r.status_code} {r.text}")


def sim_integration_id(itype):
    r = requests.get(f"{SIM_URL}/integrations", timeout=30).json()
    ints = r if isinstance(r, list) else r.get("integrations", r)
    for i in (ints or []):
        if i.get("type") == itype:
            return i.get("id")
    return 0


def sim_configure_template(profile_id, activation, interval,
                           integration=True, tb=False, tb_profile="",
                           range_m=5000, tid=1):
    t = sim_get_template(tid)
    t["activationMode"] = activation
    t["sendInterval"] = interval
    t["deviceProfileId"] = profile_id
    t["range"] = range_m
    t["integrationEnabled"] = integration
    t["integrationId"] = sim_integration_id("chirpstack") if integration else 0
    t["tbIntegrationEnabled"] = tb
    t["tbIntegrationId"] = sim_integration_id("thingsboard") if tb else 0
    t["tbDeviceProfileId"] = tb_profile
    sim_update_template(t)


def sim_create(count, prefix, spread_m=100,
               lat=48.207736, lng=16.374783, alt=0, tid=1):
    r = requests.post(f"{SIM_URL}/create-devices-from-template",
                      json={"templateId": tid, "count": count,
                            "namePrefix": prefix, "baseLat": lat,
                            "baseLng": lng, "baseAlt": alt,
                            "spreadMeters": spread_m},
                      timeout=3600)
    if r.status_code != 200:
        raise SystemExit(f"create failed: {r.status_code} {r.text[:400]}")
    return r.json().get("created", 0)


def sim_start(spread_ms=0, url=None):
    url = url or SIM_URL
    q = f"?jitter={spread_ms}" if spread_ms > 0 else ""
    requests.get(f"{url}/start{q}", timeout=120)


def sim_stop(url=None):
    url = url or SIM_URL
    try:
        requests.get(f"{url}/stop", timeout=60)
    except Exception:
        pass


def sim_status_ok(url=None):
    url = url or SIM_URL
    try:
        return requests.get(f"{url}/status", timeout=5).ok
    except Exception:
        return False


def sim_device_count():
    j = requests.get(f"{SIM_URL}/devices", params={"page": 1, "limit": 1},
                     timeout=30).json()
    if j is None:
        return 0
    if isinstance(j, list):
        return len(j)
    return int(j.get("total", j.get("totalCount", 0)))


def sim_del_all():
    r = requests.post(f"{SIM_URL}/del-all-devices", timeout=3600)
    if r.status_code != 200:
        raise SystemExit(f"del-all-devices failed: {r.status_code} {r.text}")
    return r.json().get("deleted", 0)


def sim_container(service="lwn-simulator"):
    """Resolve a compose service's container id (override the sim with SIM_CONTAINER)."""
    if service == "lwn-simulator" and os.environ.get("SIM_CONTAINER"):
        return os.environ["SIM_CONTAINER"]
    r = subprocess.run(["docker", "compose", "ps", "-q", service],
                       capture_output=True, text=True, cwd=str(BENCH_ROOT))
    cid = r.stdout.strip().splitlines()
    if not cid or not cid[0]:
        raise SystemExit(f"could not find the {service} container (is it up?)")
    return cid[0]


# ---------- Metrics ----------

def metric(name):
    try:
        text = requests.get(METRICS_URL, timeout=5).text
        for line in text.splitlines():
            if line.startswith(name + " "):
                return int(float(line.split()[1]))
    except Exception:
        return -1
    return -1


# ---------- container resource counters (cgroup v1 + v2, via docker exec) ----------

_PEAK_PATHS = ("/sys/fs/cgroup/memory.peak",
               "/sys/fs/cgroup/memory/memory.max_usage_in_bytes")
_peak_path = {}


def _exec(container, *cmd):
    return subprocess.run(["docker", "exec", container, *cmd],
                          capture_output=True, text=True)


def _cgroup_peak_file(container):
    if container not in _peak_path:
        for p in _PEAK_PATHS:
            if _exec(container, "test", "-f", p).returncode == 0:
                _peak_path[container] = p
                break
        else:
            raise RuntimeError(f"no cgroup peak-memory file in {container}")
    return _peak_path[container]


def container_peak_ram_mb(container):
    out = _exec(container, "cat", _cgroup_peak_file(container)).stdout.strip()
    return int(out) / (1024 * 1024)


def reset_peak_ram(container):
    p = _cgroup_peak_file(container)
    _exec(container, "sh", "-c", f"echo 0 > {p}")


def container_cpu_seconds(container):
    out = _exec(container, "cat", "/sys/fs/cgroup/cpu.stat").stdout
    for line in out.splitlines():
        if line.startswith("usage_usec "):
            return int(line.split()[1]) / 1e6
    for p in ("/sys/fs/cgroup/cpu/cpuacct.usage",
              "/sys/fs/cgroup/cpuacct/cpuacct.usage"):
        r = _exec(container, "cat", p)
        if r.returncode == 0 and r.stdout.strip().isdigit():
            return int(r.stdout.strip()) / 1e9
    raise RuntimeError(f"no cgroup cpu counter in {container}")


# ---------- MQTT ----------

class MqttCounter:
    """Counts CS uplink events per DevEUI plus raw gateway-bridge uplinks."""

    def __init__(self, app_id):
        import paho.mqtt.client as mqtt
        self.app_id = app_id
        self.lock = threading.Lock()
        self.cs_devices = set()
        self.cs_events = 0
        self.gw_events = 0
        self.client = mqtt.Client()
        self.client.on_connect = self._on_connect
        self.client.on_message = self._on_message

    def _on_connect(self, client, userdata, flags, rc):
        client.subscribe(f"application/{self.app_id}/device/+/event/up")
        client.subscribe("+/gateway/+/event/up")

    def _on_message(self, client, userdata, msg):
        parts = msg.topic.split("/")
        with self.lock:
            if parts[0] == "application" and len(parts) >= 6:
                self.cs_events += 1
                self.cs_devices.add(parts[3])
            elif len(parts) >= 4 and parts[1] == "gateway" and parts[3] == "event":
                self.gw_events += 1

    def start(self):
        self.client.connect(env("MQTT_HOST", "localhost"),
                            int(env("MQTT_PORT", 1883)), 60)
        self.client.loop_start()

    def stop(self):
        self.client.loop_stop()
        self.client.disconnect()

    def snapshot(self):
        with self.lock:
            return len(self.cs_devices), self.cs_events, self.gw_events

    def reset(self):
        with self.lock:
            self.cs_devices.clear()
            self.cs_events = 0
            self.gw_events = 0


# ---------- Plot style ----------

PALETTE = {
    "cpu": "#1f77b4",
    "ram": "#d62728",
    "bare": "#2ca02c",
    "cs": "#1f77b4",
    "cs+tb": "#d62728",
    "spread": {0: "#d62728", 100: "#ff7f0e", 1000: "#2ca02c", 10000: "#1f77b4"},
}


def use_paper_style():
    import matplotlib.pyplot as plt
    plt.rcParams.update({
        "font.size": 12,
        "axes.labelsize": 13,
        "axes.titlesize": 14,
        "xtick.labelsize": 11,
        "ytick.labelsize": 11,
        "legend.fontsize": 11,
    })
