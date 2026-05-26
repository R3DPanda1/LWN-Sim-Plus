# Benchmark stack

Self-contained environment to deploy LWN-Sim-Plus with ChirpStack and reproduce
the thesis benchmarks, on WSL2 or native Linux. Two deployment paths are
supported — Docker Compose (here) and Kubernetes via k3d (`../k8s`). Both expose
the same localhost ports and are seeded by the same `../k8s/seed.py`.

The benchmark drivers live in `../k8s/benchmark/`; this folder is the stack they
run against.

## Requirements

- Docker and Docker Compose (Docker Desktop on WSL2 works).
- Python 3 with `pip install requests paho-mqtt matplotlib`.
- For the k3d path: `kubectl` and `k3d`.
- For the fork-vs-upstream benchmarks: the upstream repo cloned next to this one:
  `git clone https://github.com/UniCT-ARSLab/LWN-Simulator.git ../../LWN-Simulator`.

## Quick start (Docker Compose)

```bash
cd benchmark
docker compose up -d                 # ChirpStack + bridge + mosquitto + simulator
python3 ../k8s/seed.py               # app, profiles, gateway, integration; writes bench_config.json
```

`seed.py` is idempotent and writes `bench_config.json` here, which the drivers
read for the ChirpStack API key and URLs. Then run any benchmark (see below).

### Optional services

ThingsBoard (needed only for the provisioning `cs+tb` mode) must have its schema
installed once, then started:

```bash
docker compose --profile thingsboard up -d tb-postgres
docker compose run --rm -e INSTALL_TB=true -e LOAD_DEMO=true thingsboard
docker compose --profile thingsboard up -d thingsboard
python3 ../k8s/seed.py               # re-run: mints a TB key and wires the sim's TB integration
```

Upstream simulator (for `otaa_correctness` and `sink_overhead` comparison):

```bash
docker compose --profile upstream build lwn-upstream
```

## Running the benchmarks

From `../k8s/benchmark`. Each driver defaults to the thesis scale; the env
overrides shown give a quick under-100-device sanity run. `OUT_CSV` / `OUT_JSON`
redirect output so the preserved thesis data is not overwritten.

```bash
cd ../k8s/benchmark

# Provisioning: bulk create/delete wall time, 3 integration modes
COUNTS=1000,5000,10000 python3 provisioning_benchmark.py          # thesis
COUNTS=50 MODES=bare,cs OUT_CSV=/tmp/p.csv python3 provisioning_benchmark.py   # quick

# OTAA correctness: fork vs upstream (upstream profile + image required)
python3 otaa_correctness.py                                       # thesis (N=10 / 2000)
PASS=fork FORK_N=20 OUT_CSV=/tmp/o.csv python3 otaa_correctness.py # quick, fork only

# ABP emission-ceiling sweep (CPU/RAM); plot reads the summary it writes
python3 v2/axis_a_sweep.py && python3 results/abp_scaling/plot_abp_scaling.py
ABP_TIERS=50 REPS=2 OUT_JSON=/tmp/a.json python3 v2/axis_a_sweep.py   # quick

# Sink overhead (fork vs upstream), bridge-only
python3 sink_overhead_fork.py && python3 sink_overhead_upstream.py
SINK_TIERS=50 REPS=2 python3 sink_overhead_fork.py                # quick

# Start-spread burst shape (broker-side); needs the MQTT port (compose only)
python3 results/spread_final/prep_abp.py
python3 results/spread_final/mqtt_record_plot.py

# OTAA join success vs start-spread at scale
python3 results/spread_final/otaa_spread_quick.py                 # thesis (8000, 7 min)
COUNT=20 SPREADS=0,1000 INTERVAL=30 SAMPLE_SEC=75 \
  python3 results/spread_final/otaa_spread_quick.py               # quick
```

## Kubernetes (k3d)

```bash
cd ../k8s
make up        # create k3d cluster, build + import the image, deploy
make seed      # seed once pods are ready
```

The REST-driven benchmarks (`provisioning_benchmark.py`, `otaa_correctness.py`
fork pass, `otaa_spread_quick.py`) run unchanged against the k3d ports. The
cgroup-based ones (`axis_a_sweep.py`, `sink_overhead_*`) read container stats
via `docker exec` and are Docker-Compose only.

## Notes

- Ports: simulator 8002, metrics 8003, ChirpStack REST 8090, UI 8080, MQTT 1883.
  Compose and k3d both bind these, so stop one before starting the other.
- Always start from a clean slate between scale runs (`docker compose down -v`
  then `up -d` and re-seed) — accumulated ChirpStack state skews results.
- The host UDP receive buffer must be raised for large fleets (see `host-tune.sh`;
  the k8s path applies it via `../k8s/host-tuning.yaml`).
- `.env` is optional; copy `.env.example` to override URLs or set a ThingsBoard key.
