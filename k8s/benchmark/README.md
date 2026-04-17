# Benchmarks

Scripts for benchmarking the k8s deployment of LWN-Sim-Plus and the upstream LWN-Simulator.

| Script | What it measures |
|---|---|
| `otaa_benchmark.py` | Bulk OTAA join completion time (fork) |
| `scaling_benchmark.py` | ABP uplink throughput + CPU/memory up to 20k devices (fork, AM319 only) |
| `mixed_benchmark.py` | Same as scaling but split across AM319, MCF-LW13IO, SDM230 codecs |
| `upstream_benchmark.py` | CPU/memory of the unmodified upstream simulator |
| `upstream_otaa_benchmark.py` | OTAA join completion for the upstream (manual ChirpStack provisioning) |

Shared helpers live in `helpers.py`.

## Prerequisites

```sh
cd k8s
make up
make seed
```

NodePort services must be reachable on localhost (k3d forwards these automatically):
- Simulator API: `localhost:8002`, metrics: `localhost:8003`
- ChirpStack REST: `localhost:8090`

If not using k3d, port-forward manually:
```sh
kubectl -n lorawan port-forward svc/simulator 8002:8002 8003:8003 &
kubectl -n lorawan port-forward svc/chirpstack-rest-api 8090:8090 &
```

## Running

```sh
python3 otaa_benchmark.py
python3 scaling_benchmark.py
python3 mixed_benchmark.py
python3 upstream_benchmark.py
python3 upstream_otaa_benchmark.py
```

Each script writes two CSVs: `<name>_results_<timestamp>.csv` (summary per tier) and `<name>_timeseries_<timestamp>.csv` (per-sample data).

## Environment variables

All scripts use env vars for configuration. Common ones:

| Variable | Default | Used by |
|---|---|---|
| `TIERS` | varies | all |
| `SEND_INTERVAL` | varies | all |
| `SIM_URL` | `http://localhost:8002/api` | all |
| `METRICS_URL` | `http://localhost:8003/metrics` | all |
| `CS_URL` | `http://localhost:8090` | fork scripts |
| `NAMESPACE` | `lorawan` | fork scripts |
| `SAMPLE_INTERVAL` | `10` | scaling, mixed, upstream |
| `TIMEOUT_SMALL` / `TIMEOUT_LARGE` | `600` / `1200` | OTAA scripts |
| `BRIDGE_HOST` / `BRIDGE_PORT` | `chirpstack-gateway-bridge` / `1700` | upstream scripts |

See each script's top-level constants for exact defaults.
