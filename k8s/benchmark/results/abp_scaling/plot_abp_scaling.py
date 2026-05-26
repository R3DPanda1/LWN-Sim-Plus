"""ABP emission scaling — AM319 payload, single core, bridge-only sink.

Reads the v2 axis_a (K=1) sweep summary and renders CPU and peak RAM versus
fleet size, with the zero-loss result annotated. Source data:
  k8s/benchmark/v2/results/bench_v2/axis_a/K1/summary.json
"""
import json
from pathlib import Path

import matplotlib.pyplot as plt

HERE = Path(__file__).resolve().parent
SRC = HERE / "../../v2/results/bench_v2/axis_a/K1/summary.json"
OUT = HERE / "abp_scaling.png"

CPU_C = "#1f77b4"
RAM_C = "#d62728"

rows = sorted(json.load(open(SRC)), key=lambda r: r["N"])
N = [r["N"] for r in rows]
cpu = [r["cpu_pct_mean"] for r in rows]
cpu_sd = [r["cpu_pct_sd"] for r in rows]
ram = [r["ram_peak_mean"] for r in rows]
ram_sd = [r["ram_peak_sd"] for r in rows]
loss = max(r["loss_mean"] for r in rows)

fig, ax1 = plt.subplots(figsize=(8, 4.5))

ax1.errorbar(N, cpu, yerr=cpu_sd, color=CPU_C, marker="o", markersize=7,
             linewidth=2.0, capsize=3, label="CPU")
ax1.set_xlabel("Devices")
ax1.set_ylabel("CPU (% of one core)", color=CPU_C)
ax1.tick_params(axis="y", labelcolor=CPU_C)
ax1.set_ylim(0, max(cpu) * 1.25)
ax1.grid(True, alpha=0.3)

ax2 = ax1.twinx()
ax2.errorbar(N, ram, yerr=ram_sd, color=RAM_C, marker="s", markersize=7,
             linewidth=2.0, capsize=3, label="Peak RAM")
ax2.set_ylabel("Peak RAM (MB)", color=RAM_C)
ax2.tick_params(axis="y", labelcolor=RAM_C)
ax2.set_ylim(0, max(ram) * 1.25)

ax1.set_xticks(N)
ax1.set_xticklabels([f"{n//1000}k" for n in N])
ax1.set_xlim(0, max(N) * 1.05)

ax1.annotate(f"UDP loss = {loss:.1f}% at every tier",
             xy=(0.03, 0.94), xycoords="axes fraction",
             fontsize=10, fontweight="bold",
             bbox=dict(boxstyle="round,pad=0.3", fc="#eef7ee", ec="#2ca02c"))

fig.tight_layout()
fig.savefig(OUT, dpi=150, bbox_inches="tight")
print(f"wrote {OUT}")
