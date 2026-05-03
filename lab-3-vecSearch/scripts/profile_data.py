"""
Единые векторные данные для cProfile-хотпаcов под ТЗ:
  — по умолчанию SIFT (*.fvecs), до PROFILE_MAX_VECTORS строк (≤ |корпус|);
  — PROFILE_NQ случайных различных запросов как в run_benchmark (replace=False).

Без локального sift_base и без PROFILE_ALLOW_SYNTHETIC=1 — ошибка с подсказкой download_data.py.
"""
from __future__ import annotations

import os
import sys
from pathlib import Path

import numpy as np

_HERE = Path(__file__).resolve().parents[1]
if str(_HERE) not in sys.path:
    sys.path.insert(0, str(_HERE))

from io_fvecs import read_fvecs  # noqa: E402


def load_profile_xy() -> tuple[np.ndarray, np.ndarray, np.random.Generator]:
    """xb (N,d), xq (PROFILE_NQ,d); seed PROFILE_SEED (default 42)."""
    data = Path(os.environ.get("PROFILE_DATA", _HERE / "data" / "sift_base.fvecs"))

    mv_raw = os.environ.get("PROFILE_MAX_VECTORS", "200000").strip()
    if mv_raw == "0":
        mv: int | None = None  # полный файл
    else:
        mv = max(10_001, int(mv_raw))

    nq = int(os.environ.get("PROFILE_NQ", "10000"))
    seed = int(os.environ.get("PROFILE_SEED", "42"))
    rng = np.random.default_rng(seed)

    if data.is_file():
        xb = read_fvecs(data, max_vectors=mv).astype(np.float32, copy=False)
    elif os.environ.get("PROFILE_ALLOW_SYNTHETIC", "").strip() in ("1", "true", "yes"):
        n = max(int(mv or 250_000), nq + 1)
        xb = rng.standard_normal((n, 128)).astype(np.float32)
        print(
            "[profile_data] PROFILE_ALLOW_SYNTHETIC: Gaussian N×128, не SIFT.",
            flush=True,
        )
    else:
        raise SystemExit(
            f"Не найден {data}. Скачайте базу:\n"
            f"  cd {_HERE} && .venv/bin/python download_data.py\n"
            "Или временно разрешите синтетику: PROFILE_ALLOW_SYNTHETIC=1\n"
            f"(PROFILE_DATA переопределит путь)."
        )

    n, d = xb.shape
    if nq > n:
        raise SystemExit(
            f"PROFILE_NQ={nq} превышает размер загруженного корпуса N={n} "
            "(уменьшите PROFILE_NQ или PROFILE_MAX_VECTORS=0 для всего файла)."
        )

    qi = rng.choice(n, size=nq, replace=False)
    xq = xb[qi].copy()
    return xb, xq, rng
