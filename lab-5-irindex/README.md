# lab-5-irindex

Обратный позиционный индекс: булевы запросы (AND/OR/NOT, ADJ, NEAR, границы документа), BM25, сжатие на диске (**PForDelta + varint|bitpack**), чтение через **mmap**, консольный стенд **`irquery`**.

## Требования

- Go 1.22+
- gnuplot (графики `make plot`)
- Распакованный дамп ruwiki (вики-бенчи, сборка индекса)

По умолчанию ищется XML:

1. `$CORPUS_XML` / `$WIKI_XML`
2. `data/ruwiki-latest-pages-articles.xml`
3. `../ruwiki-latest-pages-articles.xml` (корень репозитория SIAOD)

```bash
ln -sf ../ruwiki-latest-pages-articles.xml data/ruwiki-latest-pages-articles.xml
# или
export WIKI_XML=../ruwiki-latest-pages-articles.xml
```

## Сборка

```bash
make build   # bin/irindex, bin/irquery
```

## Индекс с вики

Сборка через `AddLean` (тексты в RAM не копируются). Прогресс: `WIKI_PROGRESS=5000`.

```bash
go run ./cmd/irindex -xml ../ruwiki-latest-pages-articles.xml -maxdocs 20000 -out data/index.irx
make build-index WIKI_XML=../ruwiki-latest-pages-articles.xml
```

Формат **`IRIXV3PD`**: doc Δ — **PForDelta**; tf/pos Δ — **varint|bitpack (optimal, V4)**; заголовки wiki в `.irx`.

**Ruwiki N=20 000** (прогон 2026-06-06): построение **~47 с**, `.irx` **187 088 КБ**, RAM/файл **≈8.3×**.

## Стенд запросов

```bash
./bin/irquery -index data/index.irx -q 'россия AND москва' -limit 10
./bin/irquery -index data/index.irx -q 'ADJ(великая, отечественная)' -limit 5
./bin/irquery -index data/index.irx -q 'история AND NOT(россии AND китая)' -limit 10
./bin/irquery -index data/index.irx -q 'россия AND москва' -rank -limit 10
```

Вывод: **docID**, **заголовок**, **terms×tf**. **MSM(...)** только in-memory (в `.irx` нет текстов).

## Тесты и метрики

```bash
make test

# синтетика → metrics/raw/benchmarks.csv + plots
make collect plot

# ruwiki → metrics/raw/benchmarks_wiki.csv + wiki plots
make bench-wiki parse-wiki plot \
  WIKI_XML=../ruwiki-latest-pages-articles.xml \
  BENCH_CORPUS=5000,10000,20000 BENCH_TIME=300ms

# сравнение кодеков (posting payload, КБ)
make collect-compression WIKI_XML=../ruwiki-latest-pages-articles.xml

# pprof + flamegraph HTML/PNG
make profile
```

| Переменная | По умолчанию |
|:-----------|:-------------|
| `BENCH_CORPUS` | `400,2000` |
| `WIKI_BENCH_CORPUS` | `5000,10000,20000` |
| `BENCH_TIME` | `500ms` |

### Сжатие posting lists (КБ, `metrics/raw/compression_sizes.tsv`)

| Корпус | V1 varint | V2 bitpack | V3 P4+bitpack | **V4 P4+opt (боевой)** |
|:-------|----------:|-----------:|--------------:|-----------------------:|
| синт. N=2000 | 76 | 38 | 37 | **37** |
| ruwiki 20k | 120 522 | 161 326 | 173 422 | **153 645** |

V4 — формат `SaveCompressed`; V3 — отдельная колонка «P4 + только bitpack tf/pos».

### Бенчмарки синтетика (`benchmarks.csv`, N=2000)

| Сценарий | idx ns/op | scan ns/op |
|:---------|----------:|-----------:|
| BuildIndex | 10.8M | — |
| QueryEvalMixed | 769k | 1.15M |
| BenchmarkOp AND | 10.9k | 166k |
| BenchmarkOp ADJ | 7.6k | 125k |
| BenchmarkOp Complex | 36.2k | 345k |

### Бенчмарки ruwiki (`benchmarks_wiki.csv`, N=20 000, idx)

| Запрос | ns/op |
|:-------|------:|
| `ADJ(великая, отечественная)` | **7.0k** |
| `NEAR(3, великая, отечественная)` | **9.3k** |
| `россия AND москва` | **38.0k** |
| `россия OR москва` | 69.8k |
| `NOT россия` | 189k |
| `(россия OR москва) AND история AND NOT футбол` | 518k |

`россия AND москва`: idx **38 µs** vs scan **250 µs** (**≈6.6×**).

## Графики (`metrics/plots/`)

| Файл | Содержание |
|:-----|:-----------|
| `build_index_ns.png` | построение индекса (синт.) |
| `query_idx_vs_scan.png` | смешанный запрос idx vs scan |
| `ops_idx_ns.png` | операторы по отдельности (синт.) |
| `wiki_ops_idx_bar.png` | ruwiki: ns/op, полные подписи запросов |
| `wiki_ops_idx_scale.png` | ruwiki: масштабирование по N |
| `wiki_op_AND_idx_vs_scan.png` | ruwiki AND idx vs scan |
| `pprof_cpu_query_idx.png` | CPU профиль запроса |
| `pprof_mem_query_idx.png` | alloc профиль запроса |
| `flamegraph_cpu_query_idx.png` | flamegraph CPU (запрос) |
| `flamegraph_mem_query_idx.png` | flamegraph mem (запрос) |
| `flamegraph_cpu_build_index.png` | flamegraph CPU (сборка) |
| `flamegraph_mem_build_index.png` | flamegraph mem (сборка) |

Пересборка всех PNG:

```bash
make collect plot
make bench-wiki parse-wiki plot WIKI_XML=../ruwiki-latest-pages-articles.xml
make profile
```

## Структура

| Путь | Назначение |
|:-----|:-----------|
| `internal/ir/p4delta.go` | PForDelta (doc Δ) |
| `internal/ir/encode_stream.go` | bitpack / optimal varint|bitpack (tf/pos) |
| `internal/ir/storage.go` | `SaveCompressed`, `OpenMMapIndex`, `IRIXV3PD` |
| `cmd/irindex` | построение `.irx` |
| `cmd/irquery` | REPL / `-q` по mmap |
| `plot_metrics.gnuplot` | gnuplot-скрипт |
| `REPORT.md` | отчёт |
| `metrics/raw/` | csv, tsv, pprof |

Подробности — [REPORT.md](REPORT.md).
