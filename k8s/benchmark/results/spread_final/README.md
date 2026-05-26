# Start-Spread Benchmark — Preserved Artifacts

Final data and scripts behind Section 4.x "Randomized Start Spread" of the
thesis. Run date: 2026-04-22. Host: Hetzner CX23 (AMD EPYC Milan, 8 vCPU,
32 GB). Simulator pinned with `--cpuset-cpus=7 --cpus=1.0`.

## What's in the paper

### Figure — `spread_burst_shape.png` (= `mqtt_spread_activity.png`)

Broker-side arrival rate per 10 ms, 200 ms centred rolling mean. N = 1000
ABP devices, interval 30 s, four spread values (0, 100 ms, 1 s, 10 s).
Captured by subscribing to `eu868/gateway/+/event/up` on mosquitto.

- Raw data: `mqtt_spread_activity.json`
- Capture script: `mqtt_record_plot.py` (runs locally, talks to sim+broker
  over SSH tunnel)
- Plotting only (from the JSON): `mqtt_replot.py`
- Prep (hard reset + create 1000 ABP devices on Hetzner): `prep_abp.py`

N = 1000 was chosen because the simulator's `SenderVirtual()` goroutine
tops out at ~1500–2000 PUSH_DATA packets/s per core. At N = 8000 the knob
shape is hidden by that ceiling; at N = 1000 the emit rate stays below it
and the spread's effect is visible.

### Table — OTAA join success at N = 8000, one cycle, seven minutes after start

| spread T  | lastSeenAt/total | %        |
|-----------|------------------|----------|
|    0 ms   |  1244 / 8000     |  15.55 % |
|  100 ms   |  5213 / 8000     |  65.16 % |
|    1 s    |  7429 / 8000     |  92.86 % |
|   10 s    |  8000 / 8000     | 100.00 % |

(Numbers are from successive runs; 1000 ms row came from `otaa_spread_rerun2.log`,
the others from `otaa_spread_quick.log`.)

- Sweep script: `otaa_spread_quick.py`
- Rerun of the conditions that 503'd during seed: `otaa_spread_rerun2.py`
- Logs: `otaa_spread_quick.log`, `otaa_spread_rerun2.log`

## Supporting data (not in paper)

`udp_spread.json` / `udp_record.py` / `udp_plot.py` — tcpdump capture on
the docker bridge interface between the simulator and the gateway-bridge
container (`udp and dst port 1700`). At N = 8000 the emit rate is flat
~1500 pkt/s regardless of spread, which is the evidence that the observed
plateau is the `SenderVirtual` ceiling, not a ChirpStack bottleneck.

## Reproduction quickstart

Against a host running the bench stack in `/root/lwn-benchmarks`:

```bash
# 1. Prep: 1000 ABP devices, seeded CS (idempotent)
scp prep_abp.py hetzner:/root/ && ssh hetzner 'python3 /root/prep_abp.py'

# 2. Open SSH tunnel for sim + mosquitto
ssh -fN hetzner            # forwards :8002 and :1883

# 3. Capture burst shape locally
python3 mqtt_record_plot.py     # -> /tmp/mqtt_spread_activity.{json,png}

# 4. OTAA table (runs ~35 min on the Hetzner host)
scp otaa_spread_quick.py hetzner:/root/
ssh hetzner 'python3 /root/otaa_spread_quick.py 2>&1 | tee /root/otaa_spread_quick.log'
```

## Provenance

| File | Source |
|---|---|
| `mqtt_*.{json,png}`, `udp_spread.json` | local `/tmp/` after capture |
| `mqtt_record_plot.py`, `mqtt_replot.py`, `udp_*.py` | local `/tmp/` |
| `otaa_spread_*.py`, `otaa_spread_*.log`, `prep_abp.py` | hetzner `/root/` |
