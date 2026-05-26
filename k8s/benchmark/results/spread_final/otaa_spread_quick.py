"""OTAA join success versus start-spread at scale (tab:spread-8k-otaa).

For each spread T it creates a fresh OTAA fleet, starts the sim with that
spread, waits, and reads ChirpStack's lastSeenAt count. Send interval is 300 s,
so the single sample is taken at 7 minutes; do not poll during the run. The
table value is this first sample. Output is printed (captured to
otaa_spread_quick.log); no separate data file is kept.

  python3 otaa_spread_quick.py
  COUNT=30 SPREADS=0,1000 SAMPLE_SEC=90 python3 otaa_spread_quick.py   # quick check
"""
import sys
import time
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[2]))
from common import (env, cs_profile_id, cs_last_seen_count, sim_del_all,
                    sim_configure_template, sim_create, sim_start, sim_stop)

COUNT      = int(env("COUNT", "8000"))
SPREADS    = [int(x) for x in env("SPREADS", "0,100,1000,10000").split(",")]
INTERVAL   = int(env("INTERVAL", "300"))
SAMPLE_SEC = int(env("SAMPLE_SEC", "420"))


def run_spread(spread_ms):
    print(f"\n--- spread={spread_ms}ms, N={COUNT} ---", flush=True)
    sim_del_all()
    profile = cs_profile_id(env("OTAA_PROFILE_NAME", "AM319 OTAA"))
    sim_configure_template(profile, "otaa", INTERVAL)
    sim_create(COUNT, "otaa-spread")
    sim_start(spread_ms=spread_ms)
    print(f"  started, sampling at {SAMPLE_SEC}s", flush=True)
    time.sleep(SAMPLE_SEC)
    joined = cs_last_seen_count()
    sim_stop()
    print(f"  spread={spread_ms}ms: joined={joined}/{COUNT} ({joined / COUNT * 100:.2f}%)", flush=True)
    return joined


def main():
    results = [(s, run_spread(s)) for s in SPREADS]
    print(f"\n{'spread_ms':>10}  {'joined':>7}  {'pct':>7}")
    for s, j in results:
        print(f"{s:>10}  {j:>7}  {j / COUNT * 100:>6.2f}%")


if __name__ == "__main__":
    main()
