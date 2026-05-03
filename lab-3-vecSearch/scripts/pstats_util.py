"""Разбор дампов cProfile для Python 3.12+ и старых версий."""
from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path

import pstats


@dataclass(frozen=True)
class StatRow:
    label: str
    short: str
    tottime: float
    cumtime: float


def _unpack(raw: tuple) -> tuple[float, float, float]:
    """Возвращает (ncalls, tottime, cumtime)."""
    if len(raw) >= 5:
        _prim, nc, tt, ct, _callers = raw[:5]
        return float(nc), float(tt), float(ct)
    if len(raw) == 4:
        _cc, nc, tt, ct = raw
        return float(nc), float(tt), float(ct)
    raise ValueError(f"Неожиданная запись pstats {raw!r}")


def rows_from_prof(prof_path: Path, *, top: int = 22) -> list[StatRow]:
    stats = pstats.Stats(str(prof_path))
    stats.strip_dirs()
    pairs: list[StatRow] = []

    for key, tup in stats.stats.items():
        _nc, tt, ct = _unpack(tuple(tup))
        if isinstance(key, tuple) and len(key) >= 3:
            filename, lineno, funcname = key[0], key[1], key[2]
        elif isinstance(key, tuple) and len(key) == 2:
            filename, lineno, funcname = key[0], key[1], "?"
        else:
            filename, lineno, funcname = str(key), 0, str(key)
        short_nm = Path(str(filename)).name
        lbl = f"{funcname[:80]} ({short_nm}:{lineno})"
        short_nm2 = Path(str(filename)).name
        pairs.append(
            StatRow(
                label=lbl,
                short=f"{short_nm2}:{lineno} — {funcname[:48]}",
                tottime=tt,
                cumtime=ct,
            )
        )

    pairs.sort(key=lambda r: -r.cumtime)
    return pairs[:top]
