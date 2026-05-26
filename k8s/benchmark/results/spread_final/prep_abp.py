"""Create the ABP fleet for the burst-shape figure (fig:spread-burst-shape).

Run once before mqtt_record_plot.py. Default 1000 AM319 ABP devices, 30 s
interval, integration off (the figure is broker-side only).

  python3 prep_abp.py
  COUNT=50 python3 prep_abp.py
"""
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[2]))
from common import env, cs_profile_id, sim_del_all, sim_configure_template, sim_create

COUNT = int(env("COUNT", "1000"))


def main():
    sim_del_all()
    profile = cs_profile_id(env("ABP_PROFILE_NAME", "AM319 ABP"))
    sim_configure_template(profile, "abp", 30, integration=False)
    print(f"created {sim_create(COUNT, 'burst')} ABP devices")


if __name__ == "__main__":
    main()
