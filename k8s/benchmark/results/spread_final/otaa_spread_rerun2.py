"""Re-run of the OTAA start-spread sweep for a single spread value.

Used to refresh one row of tab:spread-8k-otaa (the 1 s row) without repeating
the whole sweep. Same protocol as otaa_spread_quick.py.

  python3 otaa_spread_rerun2.py                 # spread = 1 s
  SPREADS=10000 python3 otaa_spread_rerun2.py   # a different value
"""
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[2]))
import os

os.environ.setdefault("SPREADS", "1000")
import otaa_spread_quick

if __name__ == "__main__":
    otaa_spread_quick.main()
