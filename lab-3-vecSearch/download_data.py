#!/usr/bin/env python3
"""Скачивание SIFT1M base (1M×128) в формате .fvecs.

Канонический URL каталога Corpus TexMex мог устареть (404). Поэтому последовательность:
1) прямой .fvecs с зеркал (Hugging Face и т.п.);
2) архив sift.tar.gz с сервера Irisa (как в ann-benchmarks) и извлечение sift/sift_base.fvecs."""
from __future__ import annotations

import argparse
import shutil
import sys
import tempfile
import urllib.request
import tarfile
from pathlib import Path

import requests
from tqdm import tqdm

# Прямые ссылки на уже готовый sift_base.fvecs (первый рабочий выигрывает).
SIFT_BASE_MIRROR_FVECS = [
    "https://huggingface.co/datasets/qbo-odp/sift1m/resolve/main/sift_base.fvecs",
    # Бывший TexMex (на случай восстановления или редиректа)
    "http://corpus-texmex.irisa.fr/vectors/sift/sift_base.fvecs",
    "https://corpus-texmex.irisa.fr/vectors/sift/sift_base.fvecs",
]

# Полный архив с Irisa FTP (официально раздаётся как .tar.gz, внутри sift/sift_base.fvecs).
SIFT_FALLBACK_TAR_GZ = [
    "http://ftp.irisa.fr/local/texmex/corpus/sift.tar.gz",
    "ftp://ftp.irisa.fr/local/texmex/corpus/sift.tar.gz",
]

TAR_MEMBER = "sift/sift_base.fvecs"
DEFAULT_OUT = Path(__file__).resolve().parent / "data" / "sift_base.fvecs"

HTTP_HEADERS = {
    "User-Agent": "Mozilla/5.0 (compatible; lab-3-vecSearch/1.0; +https://github.com/itmo-siaod)",
}


def download_stream(url: str, dest: Path, chunk: int = 1 << 20) -> None:
    dest.parent.mkdir(parents=True, exist_ok=True)
    with requests.get(
        url, stream=True, timeout=120, headers=HTTP_HEADERS, allow_redirects=True
    ) as r:
        r.raise_for_status()
        total = int(r.headers.get("content-length", 0))
        tmp = dest.with_suffix(dest.suffix + ".part")
        with open(tmp, "wb") as f, tqdm(
            total=total, unit="B", unit_scale=True, desc=dest.name
        ) as pbar:
            for c in r.iter_content(chunk_size=chunk):
                if c:
                    f.write(c)
                    pbar.update(len(c))
        tmp.replace(dest)
    print(f"Сохранено: {dest}", file=sys.stderr)


def extract_member_from_tar_gz_archive(archive_path: Path, member: str, dest: Path) -> None:
    with tarfile.open(archive_path, "r:*") as tar:
        m = tar.getmember(member)
        f = tar.extractfile(m)
        if f is None:
            raise OSError(f"не удалось прочитать {member} из {archive_path}")
        data = f.read()
    dest.parent.mkdir(parents=True, exist_ok=True)
    tmp = dest.with_suffix(dest.suffix + ".part")
    tmp.write_bytes(data)
    tmp.replace(dest)
    print(f"Извлечено {member} → {dest}", file=sys.stderr)


def _materialize_tar_gz_locally(url: str) -> Path | None:
    """Сохранить tarball во временный файл; None при ошибке."""
    tmp = tempfile.NamedTemporaryFile(suffix="_sift.tar.gz", delete=False)
    tmp.close()
    tpath = Path(tmp.name)
    print(f"Пробую архив: {url}", file=sys.stderr)
    try:
        if url.startswith("ftp://"):
            try:
                with urllib.request.urlopen(url, timeout=420) as r, open(tpath, "wb") as f:
                    shutil.copyfileobj(r, f, length=1 << 20)
            except OSError as e:
                print(f"  FTP: {e}", file=sys.stderr)
                tpath.unlink(missing_ok=True)
                return None
            return tpath
        try:
            with requests.get(
                url, timeout=420, headers=HTTP_HEADERS, stream=True
            ) as r:
                r.raise_for_status()
                total = int(r.headers.get("content-length", 0))
                with open(tpath, "wb") as f, tqdm(
                    total=total, unit="B", unit_scale=True, desc="sift.tar.gz"
                ) as pbar:
                    for chunk in r.iter_content(chunk_size=1 << 20):
                        if chunk:
                            f.write(chunk)
                            pbar.update(len(chunk))
        except requests.RequestException as e:
            print(f"  HTTP: {e}", file=sys.stderr)
            tpath.unlink(missing_ok=True)
            return None
        return tpath
    except OSError:
        tpath.unlink(missing_ok=True)
        raise


def download_via_tgz_fallback(dest: Path) -> bool:
    for url in SIFT_FALLBACK_TAR_GZ:
        tpath = _materialize_tar_gz_locally(url)
        if tpath is None:
            continue
        try:
            extract_member_from_tar_gz_archive(tpath, TAR_MEMBER, dest)
            return True
        except (tarfile.TarError, KeyError, OSError) as e:
            print(f"  ошибка распаковки: {e}", file=sys.stderr)
        finally:
            tpath.unlink(missing_ok=True)
    return False


def ensure_sift_base(dest: Path, *, urls: list[str] | None) -> None:
    if dest.exists() and dest.stat().st_size > 0:
        print(f"Уже есть: {dest}", file=sys.stderr)
        return

    candidates = urls if urls else list(SIFT_BASE_MIRROR_FVECS)
    errors: list[str] = []
    for u in candidates:
        print(f"Пробую: {u}", file=sys.stderr)
        try:
            download_stream(u, dest)
            return
        except requests.RequestException as e:
            errors.append(f"{u}: {e}")
            dest.unlink(missing_ok=True)

    print("Прямые .fvecs недоступны, пробую .tar.gz (Irisa)…", file=sys.stderr)
    if download_via_tgz_fallback(dest):
        return

    msg = (
        "Не удалось загрузить SIFT base ни с одного зеркала:\n"
        + "\n".join(errors)
        + "\n\nУкажите файл вручную: python download_data.py --url '<ваш-url>' "
        "--out data/sift_base.fvecs"
    )
    raise SystemExit(msg)


def main() -> None:
    p = argparse.ArgumentParser()
    p.add_argument("--out", type=Path, default=DEFAULT_OUT)
    p.add_argument(
        "--url",
        action="append",
        dest="urls",
        default=None,
        help="Своя ссылка на .fvecs (можно указать несколько раз; иначе встроенные зеркала).",
    )
    args = p.parse_args()

    urls = args.urls if args.urls else None
    ensure_sift_base(args.out, urls=urls)


if __name__ == "__main__":
    main()
