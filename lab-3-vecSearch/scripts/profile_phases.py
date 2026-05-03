#!/usr/bin/env python3
"""Пофазный cProfile: те же данные, что TZ-бенчи (profile_data.load_profile_xy)."""
from __future__ import annotations

import cProfile
import os
import time
from pathlib import Path

import faiss
import matplotlib.pyplot as plt
import numpy as np

from profile_data import load_profile_xy

HERE = Path(__file__).resolve().parents[1]
PROF_DIR = HERE / "metrics" / "profiles"
RAW_DIR = HERE / "metrics" / "raw"
PLOT_DIR = HERE / "metrics" / "plots"


def _dump(pr: cProfile.Profile, path: Path) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    pr.dump_stats(str(path))


def main() -> None:
    xb, xq, rng = load_profile_xy()
    n, d = xb.shape
    k_eff = min(int(os.environ.get("PROFILE_K", "100")), n - 1)
    ml = os.environ.get("PROFILE_NLIST")
    if ml and ml.strip():
        nlist = int(ml)
    else:
        nlist = min(1024, max(256, n // 160))
    m_dim = int(os.environ.get("PROFILE_IVFM", "32"))
    train_cap = min(n, int(os.environ.get("PROFILE_TRAIN_MULT", "256")) * nlist)

    wall: list[tuple[str, float]] = []

    h = faiss.IndexHNSWFlat(d, 16)
    h.hnsw.efConstruction = 100

    t0 = time.perf_counter()
    pr = cProfile.Profile()
    pr.enable()
    h.add(xb)
    pr.disable()
    wall.append(("HNSW add", time.perf_counter() - t0))
    _dump(pr, PROF_DIR / "vec_hnsw_add.prof")

    h.hnsw.efSearch = 64
    t0 = time.perf_counter()
    pr = cProfile.Profile()
    pr.enable()
    _, _ = h.search(xq, k_eff)
    pr.disable()
    wall.append(("HNSW search", time.perf_counter() - t0))
    _dump(pr, PROF_DIR / "vec_hnsw_search.prof")

    quantizer = faiss.IndexFlatL2(d)
    ivf = faiss.IndexIVFPQ(quantizer, d, nlist, m_dim, 8)
    xt = xb[rng.choice(n, size=train_cap, replace=False)]

    t0 = time.perf_counter()
    pr = cProfile.Profile()
    pr.enable()
    ivf.train(xt)
    ivf.add(xb)
    pr.disable()
    wall.append(("IVF PQ train+add", time.perf_counter() - t0))
    _dump(pr, PROF_DIR / "vec_ivf_train_add.prof")

    ivf.nprobe = 16
    t0 = time.perf_counter()
    pr = cProfile.Profile()
    pr.enable()
    _, _ = ivf.search(xq, k_eff)
    pr.disable()
    wall.append(("IVF PQ search", time.perf_counter() - t0))
    _dump(pr, PROF_DIR / "vec_ivf_search.prof")

    RAW_DIR.mkdir(parents=True, exist_ok=True)
    out_wall = RAW_DIR / "profile_phase_wall.tsv"
    with out_wall.open("w", encoding="utf-8") as fh:
        fh.write("phase\twall_sec\n")
        for name, sec in wall:
            fh.write(f"{name}\t{sec:.6g}\n")
    print(f"wrote {out_wall}")
    nx, nd = xb.shape
    with (RAW_DIR / "profile_manifest.txt").open("w", encoding="utf-8") as fm:
        fm.write(f"n={nx} dim={nd} n_queries={len(xq)} search_k={k_eff} PROFILE_MAX_VECTORS={os.environ.get('PROFILE_MAX_VECTORS','')}\n")

    PLOT_DIR.mkdir(parents=True, exist_ok=True)
    names_ph = [n for n, _ in wall]
    secs_ph = [s for _, s in wall]
    fig, ax = plt.subplots(figsize=(8.8, 4.6))
    y = np.arange(len(names_ph))
    ax.barh(y, secs_ph, color="#377eb8", edgecolor="white", linewidth=0.6)
    ax.set_yticks(y, names_ph, fontsize=10)
    ax.set_xlabel("wall time, сек")
    ax.set_title("Фазы профилирования (данные совпадают с ТЗ-бенчем: см. PROFILE_*)")
    ax.grid(True, axis="x", alpha=0.35)
    wall_png = PLOT_DIR / "profile_walltime_phases.png"
    fig.tight_layout()
    fig.savefig(wall_png, dpi=140)
    plt.close(fig)
    print(f"wrote {wall_png}")

    for name in (
        "vec_hnsw_add.prof",
        "vec_hnsw_search.prof",
        "vec_ivf_train_add.prof",
        "vec_ivf_search.prof",
    ):
        print("wrote", PROF_DIR / name)


if __name__ == "__main__":
    main()
