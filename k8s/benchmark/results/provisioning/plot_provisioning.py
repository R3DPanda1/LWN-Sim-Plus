"""Provisioning benchmark plot — final benchmark.

Renders a two-panel figure (create, delete) with one line per integration
mode across N = 1k, 5k, 10k. Log-log axes.
"""
import csv
from pathlib import Path

import matplotlib.pyplot as plt

HERE = Path(__file__).resolve().parent
OUT  = HERE / "provisioning.png"
CSV  = HERE / "provisioning.csv"

MODES = [
    ("bare",  "none",              "#2ca02c", "o"),
    ("cs",    "ChirpStack",        "#1f77b4", "s"),
    ("cs+tb", "ChirpStack + TB",   "#d62728", "^"),
]


def load():
    data = {m: {"n": [], "create": [], "delete": []} for m, _, _, _ in MODES}
    for row in csv.DictReader(open(CSV)):
        m = row["mode"]
        if m in data:
            data[m]["n"].append(int(row["count"]))
            data[m]["create"].append(float(row["create_sec"]))
            data[m]["delete"].append(float(row["delete_sec"]))
    return data


def panel(ax, data, key, title):
    for mode, label, color, marker in MODES:
        xs = data[mode]["n"]
        ys = data[mode][key]
        ax.plot(xs, ys, color=color, marker=marker, linewidth=2.0,
                markersize=7, label=label)
    ax.set_xscale("log")
    ax.set_yscale("log")
    ax.set_xticks([1000, 5000, 10000])
    ax.set_xticklabels(["1k", "5k", "10k"])
    ax.set_xlabel("device count")
    ax.set_ylabel("wall time (s)")
    ax.set_title(title)
    ax.grid(True, which="both", linestyle=":", alpha=0.4)


def main():
    data = load()
    plt.rcParams.update({
        "font.size": 11, "axes.labelsize": 12, "axes.titlesize": 13,
        "legend.fontsize": 10,
    })
    fig, (ax1, ax2) = plt.subplots(1, 2, figsize=(10, 4.3))
    panel(ax1, data, "create", "Create")
    panel(ax2, data, "delete", "Delete")
    ax1.legend(title="Integrations", loc="upper left", framealpha=0.9)
    fig.tight_layout()
    fig.savefig(OUT, dpi=150)
    print(f"wrote {OUT}")


if __name__ == "__main__":
    main()
