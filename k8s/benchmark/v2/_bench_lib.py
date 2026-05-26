"""
Benchmark library for the Chapter 7 rerun.

Measurement philosophy (see BENCHMARK_PLAN.md section 3):
  - Sim CPU : Prometheus process_cpu_seconds_total counter delta / wall time
              -> method-independent, catches bursts, no 1 s polling gap
  - Sim RAM : cgroup memory.peak read at end of run
              -> true peak since cgroup creation, no polling
  - Counters over gauges everywhere. No docker stats for final numbers.

Intended to run on the Hetzner host next to docker. Paths assume the
lwn-benchmarks compose project.
"""

from __future__ import annotations

import json
import os
import re
import statistics
import subprocess
import threading
import time
from dataclasses import dataclass, field, asdict
from pathlib import Path
from typing import Callable

import paho.mqtt.client as mqtt
import requests


SIM_URL = "http://localhost:8002"
METRICS_URL = "http://localhost:8003/metrics"
MQTT_HOST, MQTT_PORT = "localhost", 1883
BRIDGE_TOPIC = "eu868/gateway/+/event/up"
SIM_CONTAINER = "lwn-benchmarks-lwn-simulator-1"
BRIDGE_CONTAINER = "lwn-benchmarks-chirpstack-gateway-bridge-1"
COMPOSE_FILE = "/root/lwn-benchmarks/docker-compose.yml"
DRAIN_STABLE_SEC = 30
DRAIN_CAP_SEC = 600
TEMPLATE_ID_DEFAULT = 1


# ---------------------------------------------------------------------------
# cgroup helpers (cgroups v2; system.slice/docker-<cid>.scope)
# ---------------------------------------------------------------------------

def container_id(name: str) -> str:
    out = subprocess.check_output(["docker", "inspect", "-f", "{{.Id}}", name], text=True).strip()
    if not out:
        raise RuntimeError(f"container {name} not found")
    return out


def cgroup_path(cid: str) -> Path:
    p = Path(f"/sys/fs/cgroup/system.slice/docker-{cid}.scope")
    if not p.is_dir():
        raise RuntimeError(f"cgroup path missing: {p}")
    return p


def read_cpu_usec(cid: str) -> int:
    with open(cgroup_path(cid) / "cpu.stat") as f:
        for line in f:
            if line.startswith("usage_usec "):
                return int(line.split()[1])
    raise RuntimeError("usage_usec not found in cpu.stat")


def read_memory_peak_mb(cid: str) -> float:
    with open(cgroup_path(cid) / "memory.peak") as f:
        return int(f.read().strip()) / (1024 * 1024)


def reset_memory_peak(cid: str) -> None:
    # kernels >= 6.7 support writing 0; otherwise container restart does it
    try:
        with open(cgroup_path(cid) / "memory.peak", "w") as f:
            f.write("0")
    except OSError:
        pass


# ---------------------------------------------------------------------------
# Prometheus metric helpers (counter-based CPU)
# ---------------------------------------------------------------------------

def read_prom(name: str) -> float | None:
    try:
        text = requests.get(METRICS_URL, timeout=5).text
    except Exception:
        return None
    m = re.search(rf"^{re.escape(name)}\s+([0-9eE+.\-]+)\s*$", text, re.M)
    return float(m.group(1)) if m else None


# ---------------------------------------------------------------------------
# bridge UDP RcvbufErrors (net stack saturation probe)
# ---------------------------------------------------------------------------

def bridge_rcvbuf_errors() -> int | None:
    try:
        out = subprocess.check_output(
            ["docker", "exec", BRIDGE_CONTAINER, "cat", "/proc/net/snmp"],
            text=True, timeout=5,
        )
    except Exception:
        return None
    header = None
    for line in out.splitlines():
        if line.startswith("Udp:"):
            parts = line.split()
            if header is None:
                header = parts
            else:
                values = parts
                d = dict(zip(header[1:], values[1:]))
                try:
                    return int(d.get("RcvbufErrors", "0"))
                except ValueError:
                    return None
    return None


# ---------------------------------------------------------------------------
# MQTT bridge-event counter (Axis A)
# ---------------------------------------------------------------------------

class MqttCounter:
    def __init__(self, topic: str = BRIDGE_TOPIC, client_id: str = "bench_v2"):
        self.n = 0
        self.last_change = 0.0
        self.lock = threading.Lock()
        try:
            self.cli = mqtt.Client(mqtt.CallbackAPIVersion.VERSION1, client_id=client_id)
        except AttributeError:
            self.cli = mqtt.Client(client_id=client_id)
        self.cli.on_message = self._on_msg
        self.cli.on_connect = lambda c, u, f, rc: c.subscribe(topic, qos=0)

    def _on_msg(self, c, u, msg):
        with self.lock:
            self.n += 1
            self.last_change = time.time()

    def start(self) -> None:
        self.cli.connect(MQTT_HOST, MQTT_PORT, 60)
        self.cli.loop_start()

    def stop(self) -> None:
        self.cli.loop_stop()
        self.cli.disconnect()

    def get(self) -> tuple[int, float]:
        with self.lock:
            return self.n, self.last_change


# ---------------------------------------------------------------------------
# simulator API
# ---------------------------------------------------------------------------

def sim_status() -> bool:
    try:
        r = requests.get(f"{SIM_URL}/api/status", timeout=5).text.strip()
        return r.lower() == "true"
    except Exception:
        return False


def sim_wait_ready(timeout: int = 60) -> None:
    t0 = time.time()
    while time.time() - t0 < timeout:
        try:
            r = requests.get(f"{SIM_URL}/api/status", timeout=3)
            if r.status_code == 200:
                return
        except Exception:
            pass
        time.sleep(1)
    raise RuntimeError("sim did not become ready")


def sim_del_all_devices() -> None:
    requests.post(f"{SIM_URL}/api/del-all-devices", timeout=60)


def sim_start(jitter_ms: int = 0) -> None:
    url = f"{SIM_URL}/api/start"
    if jitter_ms > 0:
        url += f"?jitter={jitter_ms}"
    requests.get(url, timeout=30)


def sim_stop(timeout: int = 60) -> None:
    try:
        requests.get(f"{SIM_URL}/api/stop", timeout=timeout)
    except requests.Timeout:
        pass


def sim_configure_template(interval: int, template_id: int = TEMPLATE_ID_DEFAULT) -> None:
    t = requests.get(f"{SIM_URL}/api/template/{template_id}", timeout=10).json()
    if "template" in t:
        t = t["template"]
    t["sendInterval"] = interval
    t["integrationEnabled"] = False
    t["tbIntegrationEnabled"] = False
    r = requests.post(f"{SIM_URL}/api/update-template", json=t, timeout=15)
    r.raise_for_status()


def sim_create_devices(count: int, template_id: int = TEMPLATE_ID_DEFAULT,
                       name_prefix: str = "Bench", base_lat: float = 48.207736,
                       base_lng: float = 16.374783, spread_m: int = 100) -> None:
    MAX = 10000
    remaining, idx = count, 0
    while remaining > 0:
        n = min(MAX, remaining)
        body = {"templateId": template_id, "count": n,
                "namePrefix": f"{name_prefix}{idx}",
                "baseLat": base_lat, "baseLng": base_lng, "baseAlt": 0,
                "spreadMeters": spread_m}
        r = requests.post(f"{SIM_URL}/api/create-devices-from-template",
                          json=body, timeout=180)
        r.raise_for_status()
        remaining -= n
        idx += 1


# ---------------------------------------------------------------------------
# clean-slate
# ---------------------------------------------------------------------------

def clean_slate(restart_sim: bool = True) -> None:
    """Stop sim if running, delete all devices, restart sim container, wait ready."""
    try:
        sim_stop(timeout=15)
    except Exception:
        pass
    try:
        sim_del_all_devices()
    except Exception:
        pass
    if restart_sim:
        subprocess.run(
            ["docker", "compose", "-f", COMPOSE_FILE, "restart", "lwn-simulator"],
            check=False, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
        )
        sim_wait_ready(timeout=90)
    time.sleep(2)


# ---------------------------------------------------------------------------
# single-run result
# ---------------------------------------------------------------------------

@dataclass
class RunResult:
    count: int
    interval: int
    jitter_ms: int
    burst_wait: int
    wall_sec: float
    sim_sent: int
    bridge_recv: int
    loss_pct: float
    sim_cpu_seconds: float              # from process_cpu_seconds_total
    sim_cpu_pct_of_core: float          # cpu_seconds / wall * 100
    sim_cpu_usec_cgroup: int            # from cgroup cpu.stat
    sim_cpu_pct_cgroup: float
    sim_ram_peak_mb: float              # cgroup memory.peak
    sim_ram_prom_mb: float              # process_resident_memory_bytes at end
    bridge_rcvbuf_errors_delta: int | None
    k_cores: int
    notes: str = ""


def run_burst(count: int, interval: int, burst_wait: int,
              jitter_ms: int = 0, k_cores: int = 1,
              csv_path: str | None = None, notes: str = "") -> RunResult:
    """One burst: create -> start -> wait burst_wait -> stop -> drain."""
    clean_slate(restart_sim=True)
    sim_configure_template(interval)

    cid = container_id(SIM_CONTAINER)
    reset_memory_peak(cid)

    sim_create_devices(count)

    ctr = MqttCounter()
    ctr.start()
    time.sleep(0.5)

    rcvbuf0 = bridge_rcvbuf_errors()
    cpu_s0 = read_prom("process_cpu_seconds_total") or 0.0
    cpu_us0 = read_cpu_usec(cid)
    sent0 = read_prom("gateway_data_sent_total") or 0.0

    t0 = time.time()
    sim_start(jitter_ms=jitter_ms)
    time.sleep(burst_wait)
    sim_stop()
    t_burst_end = time.time()

    # drain-until-stable
    t_drain0 = time.time()
    while True:
        _, last = ctr.get()
        if last and (time.time() - last) >= DRAIN_STABLE_SEC:
            break
        if time.time() - t_drain0 >= DRAIN_CAP_SEC:
            break
        time.sleep(5)

    cpu_s1 = read_prom("process_cpu_seconds_total") or cpu_s0
    cpu_us1 = read_cpu_usec(cid)
    sent1 = read_prom("gateway_data_sent_total") or sent0
    ram_peak_mb = read_memory_peak_mb(cid)
    ram_prom_mb = (read_prom("process_resident_memory_bytes") or 0.0) / (1024 * 1024)
    rcvbuf1 = bridge_rcvbuf_errors()

    ctr.stop()
    wall = time.time() - t0
    sim_sent = int(sent1 - sent0)
    bridge_recv, _ = ctr.get()
    loss_pct = 100.0 * (sim_sent - bridge_recv) / sim_sent if sim_sent else 0.0
    cpu_seconds = cpu_s1 - cpu_s0
    cpu_pct = 100.0 * cpu_seconds / wall if wall > 0 else 0.0
    cpu_us_delta = cpu_us1 - cpu_us0
    cpu_pct_cg = 100.0 * (cpu_us_delta / 1_000_000.0) / wall if wall > 0 else 0.0
    rcvbuf_delta = (rcvbuf1 - rcvbuf0) if (rcvbuf1 is not None and rcvbuf0 is not None) else None

    if csv_path:
        # trace samples aren't needed at this level; leave hook for ablation scripts
        pass

    return RunResult(
        count=count, interval=interval, jitter_ms=jitter_ms, burst_wait=burst_wait,
        wall_sec=wall, sim_sent=sim_sent, bridge_recv=bridge_recv, loss_pct=loss_pct,
        sim_cpu_seconds=cpu_seconds, sim_cpu_pct_of_core=cpu_pct,
        sim_cpu_usec_cgroup=cpu_us_delta, sim_cpu_pct_cgroup=cpu_pct_cg,
        sim_ram_peak_mb=ram_peak_mb, sim_ram_prom_mb=ram_prom_mb,
        bridge_rcvbuf_errors_delta=rcvbuf_delta,
        k_cores=k_cores, notes=notes,
    )


# ---------------------------------------------------------------------------
# replicate runner
# ---------------------------------------------------------------------------

@dataclass
class CellResult:
    count: int
    jitter_ms: int
    k_cores: int
    runs: list[dict] = field(default_factory=list)
    mean: dict = field(default_factory=dict)
    stddev: dict = field(default_factory=dict)

    def to_json(self) -> dict:
        return {"count": self.count, "jitter_ms": self.jitter_ms, "k_cores": self.k_cores,
                "runs": self.runs, "mean": self.mean, "stddev": self.stddev}


_NUMERIC_FIELDS = [
    "wall_sec", "sim_sent", "bridge_recv", "loss_pct",
    "sim_cpu_seconds", "sim_cpu_pct_of_core",
    "sim_cpu_pct_cgroup", "sim_ram_peak_mb", "sim_ram_prom_mb",
]


def aggregate(runs: list[RunResult]) -> tuple[dict, dict]:
    scored = runs[1:]  # drop first as warmup
    m, s = {}, {}
    if not scored:
        return m, s
    for f in _NUMERIC_FIELDS:
        vals = [getattr(r, f) for r in scored]
        m[f] = statistics.fmean(vals)
        s[f] = statistics.pstdev(vals) if len(vals) > 1 else 0.0
    return m, s


def run_cell(count: int, interval: int, burst_wait: int, jitter_ms: int,
             k_cores: int, replicates: int = 5,
             out_dir: str | None = None, test_id: str | None = None) -> CellResult:
    runs: list[RunResult] = []
    for i in range(replicates):
        print(f"  [{i+1}/{replicates}] N={count} K={k_cores} jitter={jitter_ms}ms ...", flush=True)
        r = run_burst(count, interval, burst_wait, jitter_ms=jitter_ms, k_cores=k_cores,
                      notes=f"replicate {i+1}{'  (warmup)' if i == 0 else ''}")
        print(f"      sent={r.sim_sent} recv={r.bridge_recv} loss={r.loss_pct:.2f}% "
              f"cpu_pct={r.sim_cpu_pct_of_core:.1f} ram_peak={r.sim_ram_peak_mb:.0f}MB "
              f"wall={r.wall_sec:.0f}s", flush=True)
        runs.append(r)
    m, s = aggregate(runs)
    cell = CellResult(count=count, jitter_ms=jitter_ms, k_cores=k_cores,
                      runs=[asdict(r) for r in runs], mean=m, stddev=s)
    if out_dir and test_id:
        out = Path(out_dir)
        out.mkdir(parents=True, exist_ok=True)
        (out / f"{test_id}.json").write_text(json.dumps(cell.to_json(), indent=2))
    return cell


# ---------------------------------------------------------------------------
# stddev-over-mean report (sanity gate)
# ---------------------------------------------------------------------------

def stddev_ratio(cell: CellResult, field_name: str) -> float:
    m = cell.mean.get(field_name, 0.0)
    s = cell.stddev.get(field_name, 0.0)
    return (s / m) if m else 0.0
