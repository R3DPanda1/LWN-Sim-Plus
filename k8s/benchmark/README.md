# Benchmarks

Drivers for the thesis benchmarks. Each is deploy-agnostic: it reads the
ChirpStack key and URLs from `../../benchmark/bench_config.json` (written by
`../seed.py`) and resolves ChirpStack IDs by name at runtime, so it works
against Docker Compose, k3d, or a remote host over an SSH tunnel. Shared helpers
are in `common.py`. See `../../benchmark/README.md` for how to bring up the stack.

Cleanup between runs uses `POST /api/del-all-devices`, which deprovisions every
device from ChirpStack (and ThingsBoard) in parallel.

| Script | Measures | Thesis artifact |
|---|---|---|
| `provisioning_benchmark.py` | Bulk create/delete wall time at N ∈ {1k,5k,10k} in three modes (`bare`, `cs`, `cs+tb`) | Tab. provisioning-results, Fig. provisioning-sweep |
| `otaa_correctness.py` | OTAA join success, fork (N=2000) vs upstream (N=10) | Tab. otaa-results |
| `v2/axis_a_sweep.py` | ABP emission ceiling: CPU and peak RAM vs fleet size | Fig. abp-scaling (plot: `results/abp_scaling/plot_abp_scaling.py`) |
| `results/spread_final/otaa_spread_quick.py` | OTAA join success vs start-spread at N=8000 (`otaa_spread_rerun2.py` reruns one value) | Tab. spread-8k-otaa |
| `results/spread_final/prep_abp.py` + `mqtt_record_plot.py` + `mqtt_replot.py` | Broker-side burst shape under each start-spread | Fig. spread-burst-shape |
| `sink_overhead_fork.py` + `sink_overhead_upstream.py` | Per-tier CPU and peak RAM, fork vs upstream, bridge-only | Tab. sink-overhead |

## Configuration

After seeding, `bench_config.json` supplies everything. Override with process
env or a `../../benchmark/.env`:

| Variable | Default | Used by |
|---|---|---|
| `BENCH_ROOT` | `../../benchmark` | locating `bench_config.json`, the compose stack |
| `SIM_URL` / `CS_URL` / `METRICS_URL` | localhost 8002 / 8090 / 8003 | all |
| `UPSTREAM_URL` | `http://localhost:8012/api` | upstream passes |
| `*_PROFILE_NAME` | `AM319 ABP` / `AM319 OTAA` / … | profile lookup by name |
| `OUT_CSV` / `OUT_JSON` | the preserved data path | redirect test output |

## Running

Thesis scale is the default; the env overrides give an under-100-device check.

```bash
# Provisioning (cs+tb needs ThingsBoard running and seeded)
python3 provisioning_benchmark.py
COUNTS=50 MODES=bare,cs OUT_CSV=/tmp/p.csv python3 provisioning_benchmark.py

# OTAA correctness (upstream pass needs the upstream profile + image)
python3 otaa_correctness.py
PASS=fork FORK_N=20 OUT_CSV=/tmp/o.csv python3 otaa_correctness.py

# ABP emission ceiling, then render the figure
python3 v2/axis_a_sweep.py
python3 results/abp_scaling/plot_abp_scaling.py
ABP_TIERS=50 REPS=2 OUT_JSON=/tmp/a.json python3 v2/axis_a_sweep.py

# Sink overhead (bridge-only, cgroup reads — Docker Compose only)
python3 sink_overhead_fork.py
python3 sink_overhead_upstream.py

# Start-spread burst shape (needs the MQTT port — Docker Compose only)
python3 results/spread_final/prep_abp.py
python3 results/spread_final/mqtt_record_plot.py          # records + plots
python3 results/spread_final/mqtt_replot.py               # re-plot from saved JSON

# OTAA join success vs start-spread
python3 results/spread_final/otaa_spread_quick.py
COUNT=20 SPREADS=0,1000 INTERVAL=30 SAMPLE_SEC=75 \
  python3 results/spread_final/otaa_spread_quick.py
```

## Preserved data

`results/*/` and `v2/results/` hold the thesis data behind the tables and
figures; the plot scripts regenerate the PNGs from it without a full rerun. The
drivers default to writing back to these paths, so pass `OUT_CSV` / `OUT_JSON`
when smoke-testing to avoid overwriting them.
