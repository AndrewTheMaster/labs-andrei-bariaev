#!/usr/bin/env python3
"""Графики по нескольким .prof: сетка фаз + pie по полному сценарию."""
from __future__ import annotations

import argparse
from pathlib import Path

import matplotlib.pyplot as plt
import matplotlib.ticker as mtick
import numpy as np
import pstats

from pstats_util import StatRow, rows_from_prof


def barh_top(rows: list[StatRow], title: str, ax, *, metric: str) -> None:
    vals = np.array([(r.cumtime if metric == "cum" else r.tottime) for r in rows], dtype=float)
    labels = [r.label for r in rows]
    vals = vals[::-1]
    labels = labels[::-1]
    ax.barh(np.arange(len(labels)), vals, color="#4daf4a", edgecolor="white", linewidth=0.4)
    ax.set_yticks(np.arange(len(labels)), labels, fontsize=6.2)
    ax.set_xlabel("секунды")
    ax.set_title(title, fontsize=9.5)
    ax.grid(True, axis="x", alpha=0.35)
    ax.xaxis.set_major_formatter(mtick.FormatStrFormatter("%.3g"))


def phase_grid(paths: dict[str, Path], out: Path, *, top: int, metric: str) -> None:
    fig, axes = plt.subplots(2, 2, figsize=(11.8, 8.8))
    ax_flat = axes.ravel()
    for i, (title, pp) in enumerate(paths.items()):
        if not pp.is_file():
            ax_flat[i].text(0.5, 0.5, f"нет профиля\n{pp}", ha="center", va="center")
            ax_flat[i].set_axis_off()
            continue
        rows = rows_from_prof(pp, top=top)
        barh_top(rows, title, ax_flat[i], metric=metric)
    fig.suptitle(
        "cProfile по фазам (обёртки Python вокруг нативного Faiss доминируют в cumulative)",
        fontsize=11,
        y=0.995,
    )
    fig.tight_layout()
    out.parent.mkdir(parents=True, exist_ok=True)
    fig.savefig(out, dpi=150)
    plt.close(fig)


def pie_top(path: Path, out: Path, *, top: int) -> None:
    st = pstats.Stats(str(path))
    st.strip_dirs()
    total = getattr(st, "total_tt", None)
    rows = rows_from_prof(path, top=top)

    tops = rows[:top]
    sizes = [r.cumtime for r in tops]
    labels = [r.short for r in tops]
    sum_top = sum(sizes)
    if total is not None and total > sum_top:
        rest = total - sum_top
        if rest > max(sum_top * 1e-6, 1e-9):
            sizes.append(rest)
            labels.append(f"остальное (~{rest:.3g} с)")

    fig, ax = plt.subplots(figsize=(7.8, 6.8))
    ax.pie(np.array(sizes, dtype=float), labels=labels, autopct="%1.1f%%", textprops={"fontsize": 6.8})
    ax.set_title("Доля cumulative time (полный сценарий, cProfile)")
    fig.tight_layout()
    out.parent.mkdir(parents=True, exist_ok=True)
    fig.savefig(out, dpi=150)
    plt.close(fig)


def main() -> None:
    here = Path(__file__).resolve().parents[1]
    plots = here / "metrics" / "plots"
    prof = here / "metrics" / "profiles"

    ap = argparse.ArgumentParser()
    ap.add_argument("--grid-out", type=Path, default=plots / "cpu_profile_phases_grid.png")
    ap.add_argument("--pie-out", type=Path, default=plots / "cpu_profile_top_pie.png")
    ap.add_argument("--top", type=int, default=14)
    ap.add_argument("--metric", choices=("cum", "tot"), default="cum")
    args = ap.parse_args()

    paths = {
        "HNSW: add()": prof / "vec_hnsw_add.prof",
        "HNSW: search()": prof / "vec_hnsw_search.prof",
        "IVFPQ: train+add()": prof / "vec_ivf_train_add.prof",
        "IVFPQ: search()": prof / "vec_ivf_search.prof",
    }

    phase_grid(paths, args.grid_out, top=args.top, metric=args.metric)
    pie_full = prof / "vec_search.prof"
    if pie_full.is_file():
        pie_top(pie_full, args.pie_out, top=min(11, args.top))

    print(f"wrote {args.grid_out}")
    if pie_full.is_file():
        print(f"wrote {args.pie_out}")


if __name__ == "__main__":
    main()
