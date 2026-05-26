"""Render the burst-shape figure (fig:spread-burst-shape) from recorded data.

Reads mqtt_spread_activity.json (written by mqtt_record_plot.py) and writes
mqtt_spread_activity.png. Also importable: plot(json_path, png_path).

  python3 mqtt_replot.py
"""
import json
import sys
from collections import Counter
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE.parents[1]))
from common import use_paper_style, PALETTE

BIN_MS = 10
SMOOTH_WIN = 20  # bins → 200 ms rolling mean
LABELS = {0: "spread = 0 ms", 100: "spread = 100 ms",
          1000: "spread = 1 s", 10000: "spread = 10 s"}


def _smooth(ys, w):
    if w <= 1:
        return ys
    out, half = [], w // 2
    for i in range(len(ys)):
        lo, hi = max(0, i - half), min(len(ys), i + half + 1)
        out.append(sum(ys[lo:hi]) / (hi - lo))
    return out


def plot(json_path, png_path):
    import matplotlib.pyplot as plt

    hits = {int(k): v for k, v in json.loads(Path(json_path).read_text()).items()}
    spreads = sorted(hits)
    use_paper_style()
    fig, ax = plt.subplots(figsize=(9, 5))
    colors = PALETTE["spread"]

    for s in reversed(spreads):              # 10 s at the back, 0 ms in front
        samples = hits.get(s, [])
        if not samples:
            continue
        shifted = [t - min(samples) for t in samples]
        bins = Counter(int(t // BIN_MS) for t in shifted)
        lo, hi = min(bins), max(bins)
        xs = [i * BIN_MS / 1000.0 for i in range(lo, hi + 1)]
        ys = [bins.get(i, 0) for i in range(lo, hi + 1)]
        ax.plot(xs, ys, color=colors.get(s, "#888"), linewidth=0.7, alpha=0.30)
        ax.plot(xs, _smooth(ys, SMOOTH_WIN), color=colors.get(s, "#888"),
                linewidth=2.0, alpha=0.9, label=LABELS.get(s, f"spread = {s} ms"))

    ax.set_xlim(0, 10)
    ax.set_yscale("log")
    ax.set_xlabel("Time since first uplink (s)")
    ax.set_ylabel(f"MQTT messages per {BIN_MS} ms")
    ax.set_title("Uplink burst shape by start spread")
    ax.legend(loc="upper right", frameon=True, framealpha=0.9)
    ax.grid(True, which="both", linestyle=":", alpha=0.4)
    fig.tight_layout()
    fig.savefig(png_path, dpi=150)
    print(f"wrote {png_path}")


if __name__ == "__main__":
    plot(HERE / "mqtt_spread_activity.json", HERE / "mqtt_spread_activity.png")
