# New benchmark scripts

Simple, reproducible benchmarks that talk HTTP only. No kubectl, no docker commands.

## `jitter_sweep.py`

Sweeps mean start-jitter at a fixed device count and records how many devices
reach `lastSeenAt` on ChirpStack within the run window.

### Per-run protocol

1. `GET /api/stop`
2. `POST /api/del-all-devices` — parallel-deprovisions every device from ChirpStack
3. Configure the OTAA template (sendInterval, integration, device profile)
4. Create `COUNT` devices from the template
5. `GET /api/start?jitter=<mu>`
6. Sample every 60s: joins, gateway packets sent, lastSeenAt
7. Early-stop when `lastSeenAt == COUNT`, otherwise stop after `MAX_MIN`
8. 30s cooldown, next jitter value

### Run

Set the four required env vars, then run:

```bash
export CS_API_KEY=...            # ChirpStack API key
export CS_APP_ID=...              # application UUID
export OTAA_PROFILE=...           # OTAA-enabled device-profile UUID
export SIM_INTEG_ID=0             # simulator integration id (seed.py creates 0)

python3 jitter_sweep.py
```

All other settings have sensible defaults. Override as needed:

```bash
COUNT=3000 JITTERS=0,5000,20000 INTERVAL=60 MAX_MIN=8 python3 jitter_sweep.py
```

### Env vars

| Var | Default | Meaning |
|---|---|---|
| `SIM_URL` | `http://localhost:8002/api` | Simulator API |
| `METRICS_URL` | `http://localhost:8003/metrics` | Prometheus endpoint |
| `CS_URL` | `http://localhost:8080` | ChirpStack REST |
| `CS_API_KEY` | *required* | ChirpStack global API key |
| `CS_APP_ID` | *required* | ChirpStack application UUID |
| `OTAA_PROFILE` | *required* | ChirpStack OTAA device profile UUID |
| `SIM_INTEG_ID` | `0` | Simulator integration id (see `seed.py`) |
| `COUNT` | `5000` | Number of devices |
| `JITTERS` | `0,10000,30000,60000,120000` | ms, mean exponential |
| `INTERVAL` | `120` | sendInterval (s) |
| `MAX_MIN` | `12` | Run cap; early-stop at 100% |
| `OUTDIR` | `./results` | Where CSVs are written |

### Running against a remote box (Hetzner)

Open an SSH tunnel so the remote ports map to localhost, then run the script
locally:

```bash
ssh -fN hetzner          # requires ~/.ssh/config alias 'hetzner'
python3 jitter_sweep.py
pkill -f 'ssh -fN hetzner'
```

### Output

Per run: `results/jitter_<stamp>_n<count>_j<mu>.csv` (timeseries).
Summary:  `results/jitter_<stamp>_n<count>_summary.csv` (one row per jitter).

Columns in the timeseries CSV:

| Column | Meaning |
|---|---|
| `jitter_ms` | Mean exponential start delay (ms) |
| `elapsed_min` | Minutes since `/start` |
| `elapsed_sec` | Seconds since `/start` |
| `joins` | `lwnsim_otaa_joins_total` counter |
| `gw_sent` | `gateway_data_sent_total` counter |
| `last_seen` | ChirpStack devices with `lastSeenAt != null` |

### Interpreting the result

- `last_seen == COUNT` at some jitter value → that jitter is sufficient for this N.
- `last_seen` plateau well below `COUNT` → ChirpStack is overloaded at this N regardless of jitter (Redis/Postgres bottleneck). Lower `COUNT` or add resources.
- The smallest jitter value that reaches 100% is the result to report.
