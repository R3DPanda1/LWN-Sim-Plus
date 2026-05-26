# OTAA Correctness Benchmark — final benchmark

Data behind the OTAA correctness subsection of the thesis. Two runs,
two rows. The measurement is qualitative: an asymmetry between upstream
(silent failure at tiny N) and fork (end-to-end success at thousands).
Date: 2026-04-23. Host: Hetzner CX23 (same stack as provisioning).

## What's measured

For each simulator, N OTAA AM319 devices are started with matching
credentials in ChirpStack. After a drain period, two counts are read
from ChirpStack:

| Column        | Meaning                                                        |
|---------------|----------------------------------------------------------------|
| `DevAddr`     | Count of devices for which CS has assigned a DevAddr (server-side sees the Join-Request and schedules a Join-Accept) |
| `lastSeenAt`  | Count of devices for which CS received a subsequent data frame (device-side actually decrypted the Join-Accept, entered activated state, and transmitted a normal uplink) |

`DevAddr = N, lastSeenAt = 0` is the signature of upstream's silent
OTAA failure: CS thinks everything worked, no device actually received
a usable Join-Accept. `DevAddr = N, lastSeenAt = N` is the signature of
fork's end-to-end success.

## Results

| Sim       | N      | DevAddr | lastSeenAt | observation wait (s) | spread (ms) |
|-----------|-------:|--------:|-----------:|---------------------:|------------:|
| upstream  |     10 |      10 |          0 |                   60 |           0 |
| fork      |  2 000 |   2 000 |      2 000 |                  120 |      10 000 |

Source: `otaa_correctness.csv`.

## Setup

To ensure the two simulators see the same fleet (same DevEUIs, same
AppKeys, same CS device records), both runs share the same device set:

1. The fork bulk-creates N OTAA devices through its own REST endpoint,
   which registers identical records on ChirpStack at the same time.
2. For the upstream run, the fork is then stopped and its
   `simdata/{devices.json, gateways.json, simulator.json}` are copied
   into the upstream container's data directory.
3. Upstream reads those files on start, so it emits Join-Requests for
   exactly the same DevEUIs/AppKeys that ChirpStack already has on file.

This removes every confounder except the simulator under test: same
gateway, same bridge address, same CS, same device credentials.

## Files

| File | Purpose |
|---|---|
| `otaa_correctness.csv` | two rows (upstream / fork) with DevAddr and lastSeenAt counts |
| `README.md` | this file |

Driver: `k8s/benchmark/otaa_correctness.py`.

## Reproduction

```bash
scp k8s/benchmark/otaa_correctness.py hetzner:/root/
ssh hetzner 'cd /root && UPSTREAM_N=10 FORK_N=2000 \
  UPSTREAM_WAIT_SEC=60 FORK_WAIT_SEC=120 \
  python3 otaa_correctness.py'
scp 'hetzner:/root/results/otaa_correctness/otaa_correctness.csv' \
    k8s/benchmark/results/otaa_correctness/
```

Requirements on the host:

1. Full docker-compose stack up (`lwn-benchmarks/docker-compose.yml`),
   fork `lwn-simulator` service running, upstream `lwn-upstream` service
   present (profiled, built on demand).
2. `seed.py` has been run, so `bench_config.json` carries the OTAA
   device profile UUID and CS application / API key.
