"""Record broker-side uplink arrivals under each start-spread, then plot.

Reuses the fleet created by prep_abp.py. For each spread it starts the sim,
subscribes to the gateway-bridge uplink topic, records arrival times for a
short window, then stops. Writes mqtt_spread_activity.json and renders the
figure via mqtt_replot.plot().

  python3 mqtt_record_plot.py
  SPREADS=0,1000 WINDOW=20 OUT_JSON=/tmp/a.json python3 mqtt_record_plot.py
"""
import json
import sys
import threading
import time
from pathlib import Path

import paho.mqtt.client as mqtt

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE.parents[1]))
import mqtt_replot
from common import env, sim_start, sim_stop

TOPIC   = "eu868/gateway/+/event/up"
SPREADS = [int(x) for x in env("SPREADS", "0,100,1000,10000").split(",")]
WINDOW  = int(env("WINDOW", "45"))     # record seconds per spread
DEAD    = int(env("DEAD", "10"))       # quiet gap between spreads
OUT_JSON = Path(env("OUT_JSON", HERE / "mqtt_spread_activity.json"))
OUT_PNG  = Path(env("OUT_PNG", HERE / "mqtt_spread_activity.png"))


def main():
    hits, lock = {}, threading.Lock()
    state = {"spread": None, "t0": None}

    def on_message(_c, _u, _msg):
        with lock:
            if state["spread"] is not None:
                hits[state["spread"]].append((time.time() - state["t0"]) * 1000.0)

    c = mqtt.Client()
    c.on_message = on_message
    c.connect(env("MQTT_HOST", "localhost"), int(env("MQTT_PORT", 1883)), 60)
    c.subscribe(TOPIC, qos=0)
    c.loop_start()
    try:
        for spread in SPREADS:
            with lock:
                hits[spread] = []
                state.update(spread=spread, t0=time.time())
            sim_start(spread_ms=spread)
            print(f"spread={spread}ms recording {WINDOW}s...", flush=True)
            time.sleep(WINDOW)
            sim_stop()
            with lock:
                n = len(hits[spread])
                state["spread"] = None
            print(f"  received {n}")
            time.sleep(DEAD)
    finally:
        c.loop_stop()
        c.disconnect()

    OUT_JSON.write_text(json.dumps({str(k): v for k, v in hits.items()}))
    print(f"wrote {OUT_JSON}")
    mqtt_replot.plot(OUT_JSON, OUT_PNG)


if __name__ == "__main__":
    main()
