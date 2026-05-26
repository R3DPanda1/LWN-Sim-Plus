# Provisioning Benchmark — final benchmark

Data behind Section `sec:provisioning-optimization` of the thesis. Single
sweep, $N \in \{1000, 5000, 10000\}$ ABP AM319 devices created and deleted
under three integration modes (nine cells total). Hetzner CX23 (8 vCPU,
32 GB), simulator pinned `--cpuset-cpus=7`. Date: 2026-04-23.

## What's measured

Wall-clock time reported by the simulator REST endpoints
`POST /create-devices-from-template` and `POST /del-all-devices`. Because
both endpoints block until their integration-side work completes, the
numbers fold in the latency of the external systems the simulator has to
provision into.

| Mode    | Active integrations                     | Measures |
|---------|-----------------------------------------|----------|
| `bare`  | none                                     | simulator-internal persistence only |
| `cs`    | ChirpStack                               | `+ N` device creates + `N` activation calls into CS |
| `cs+tb` | ChirpStack + ThingsBoard                 | the above + TB device POST + TB access-token GET + CS variables PUT per device |

External device counts are verified after every create and every delete,
to confirm the operation actually happened end-to-end; those numbers are
in the CSV for audit but are not in the paper table.

## Results

| Mode    | N        | create (s) | delete (s) |
|---------|---------:|-----------:|-----------:|
| `bare`  |    1 000 |      0.07  |      0.00  |
| `bare`  |    5 000 |      0.23  |      0.01  |
| `bare`  |   10 000 |      0.39  |      0.01  |
| `cs`    |    1 000 |      2.39  |      0.45  |
| `cs`    |    5 000 |     15.63  |      1.98  |
| `cs`    |   10 000 |     35.75  |      3.61  |
| `cs+tb` |    1 000 |     10.00  |      3.16  |
| `cs+tb` |    5 000 |     41.63  |     14.39  |
| `cs+tb` |   10 000 |     93.87  |     25.81  |

Source: `provisioning.csv`. Plot: `provisioning.png` (also copied to
`paper/master/PICs/provisioning_sweep.png`). All three modes scale close
to linearly in $N$ on log-log axes.

## Upstream comparison

The upstream LWN-Simulator has neither a bulk-creation endpoint nor a
ChirpStack integration of any kind: devices must be added individually
through `POST /api/add-device`, and no network-server registration
happens at all. It rewrites `devices.json` on every single add, giving
$\mathcal{O}(n^2)$ disk traffic. For this reason the paper presents the
fork numbers as a simple table of the three fork modes; the upstream
number (single-threaded `add-device` loop, no integrations) is
qualitative context rather than a directly comparable measurement.

## Files

| File | Purpose |
|---|---|
| `provisioning.csv` | raw results (nine cells: three modes × three counts, create and delete, with external device counts) |
| `provisioning.png` | two-panel log-log plot, one line per mode |
| `plot_provisioning.py` | plot renderer |
| `README.md` | this file |

The driver lives at `k8s/benchmark/provisioning_benchmark.py`.

## Reproduction

```bash
scp k8s/benchmark/provisioning_benchmark.py hetzner:/root/
ssh hetzner 'cd /root && TB_API_KEY="tb_..." COUNTS=1000,5000,10000 \
  python3 provisioning_benchmark.py'
scp 'hetzner:/root/results/provisioning/provisioning.csv' \
    k8s/benchmark/results/provisioning/
python3 k8s/benchmark/results/provisioning/plot_provisioning.py
```

Requirements on the host:

1. Full docker-compose stack up (`lwn-benchmarks/docker-compose.yml`),
   including the `thingsboard` + `tb-postgres` services added for this
   benchmark.
2. `seed.py` has been run (creates CS application, device profiles,
   gateway, and the simulator's CS integration at ID 0).
3. A ThingsBoard tenant-admin API key (ThingsBoard >= 4.3 for tenant
   API keys) has been obtained via the TB UI at `http://localhost:9090`
   and exported as `TB_API_KEY`.
4. The simulator's ThingsBoard integration has been added at ID 1
   pointing at `http://thingsboard:9090` with that key.
