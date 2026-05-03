#!/usr/bin/env python3
"""Горизонтальные топ-N по cumulative или tottime (cProfile → matplotlib PNG)."""
from __future__ import annotations

import argparse
from pathlib import Path

import matplotlib.pyplot as plt
import matplotlib.ticker as mtick
import numpy as np

from pstats_util import rows_from_prof


def main() -> None:
    here = Path(__file__).resolve().parents[1]
    ap = argparse.ArgumentParser()
    ap.add_argument("--prof", type=Path, default=here / "metrics" / "profiles" / "vec_search.prof")
    ap.add_argument("--out", type=Path, default=here / "metrics" / "plots" / "cpu_profile_python_top.png")
    ap.add_argument("--top", type=int, default=22)
    ap.add_argument("--metric", choices=("cum", "tot"), default="cum", help="cum=cumulative time, tot=local tottime")
    args = ap.parse_args()

    rows = rows_from_prof(args.prof, top=args.top)
    metric_lbl = "cumulative time" if args.metric == "cum" else "tottime (без учёта вложенных)"
    vals = np.array([(r.cumtime if args.metric == "cum" else r.tottime) for r in rows], dtype=float)
    labels = [r.label for r in rows]

    vals = vals[::-1]
    labels = labels[::-1]

    fig, ax = plt.subplots(figsize=(10, 6.2))
    color = "#377eb8" if args.metric == "cum" else "#e6550d"
    ax.barh(np.arange(len(labels)), vals, color=color, edgecolor="white", linewidth=0.5)
    ax.set_yticks(np.arange(len(labels)), labels, fontsize=7.8)
    ax.set_xlabel(f"{metric_lbl} (с)")
    suf = "(cumulative)" if args.metric == "cum" else "(tottime)"
    ax.set_title(f"Топ функций по {suf}: Python-оболочка над Faiss/NumPy")
    ax.grid(True, axis="x", alpha=0.35)
    ax.xaxis.set_major_formatter(mtick.FormatStrFormatter("%.3g"))

    fig.tight_layout()
    args.out.parent.mkdir(parents=True, exist_ok=True)
    fig.savefig(args.out, dpi=150)
    plt.close(fig)


if __name__ == "__main__":
    main()
