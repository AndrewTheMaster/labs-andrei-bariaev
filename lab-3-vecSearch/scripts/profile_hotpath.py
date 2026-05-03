#!/usr/bin/env python3
"""
Единый хот-пас под cProfile: HNSW (add→search), затем IVFPQ train+add→search.

Данные: scripts/profile_data.py — SIFT (*.fvecs), PROFILE_NQ≥10к по умолчанию.

Запуск: python -m cProfile -o metrics/profiles/vec_search.prof scripts/profile_hotpath.py
"""
from __future__ import annotations

import os

import faiss

from profile_data import load_profile_xy


def _ivf_lists(n: int) -> tuple[int, int, int]:
    """nlist, train_sz, ivf_train_sample cap — согласуем масштаб корпуса."""
    ml = os.environ.get("PROFILE_NLIST")
    if ml and ml.strip():
        nlist = int(ml)
    else:
        nlist = min(1024, max(256, n // 160))
    m_dim = int(os.environ.get("PROFILE_IVFM", "32"))
    mult = int(os.environ.get("PROFILE_TRAIN_MULT", "256"))
    train_cap = min(n, mult * nlist)
    return nlist, train_cap, m_dim


def main() -> None:
    xb, xq, rng = load_profile_xy()
    n, d = xb.shape
    nq = len(xq)
    k_eff = min(int(os.environ.get("PROFILE_K", "100")), n - 1)
    print(f"[profile_hotpath] N={n}, d={d}, n_queries={nq}, search_k={k_eff}", flush=True)

    h = faiss.IndexHNSWFlat(d, 16)
    h.hnsw.efConstruction = 100
    h.add(xb)
    h.hnsw.efSearch = 64
    _, _ = h.search(xq, k_eff)

    nlist, train_cap, m_dim = _ivf_lists(n)
    quantizer = faiss.IndexFlatL2(d)
    ivf = faiss.IndexIVFPQ(quantizer, d, nlist, m_dim, 8)
    train = train_cap
    xt = xb[rng.choice(n, size=train, replace=False)]
    ivf.train(xt)
    ivf.add(xb)
    ivf.nprobe = 16
    _, _ = ivf.search(xq, k_eff)


if __name__ == "__main__":
    main()
