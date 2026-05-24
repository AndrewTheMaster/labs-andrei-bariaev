# Лабораторная работа №5 — Обратный индекс, булевы запросы, mmap, сжатие, TF/IDF(BM25)

**Дисциплина:** Структуры и алгоритмы в базах данных и распределённых системах  
**Тема:** Инвертированный индекс с позициями; операторы **AND / OR / NOT**, **ADJ**, **NEAR**, границы документа (**«edge»**); хранение с mmap и сжатием (**delta + bit-packing**); ранжирование **BM25**; консольный стенд запросов по mmap-индексу.

---

## Содержание

1. [Постановка](#1-постановка)
2. [Реализация и язык запросов](#2-реализация-и-язык-запросов)
3. [Методика бенчмарков](#3-методика-бенчмарков)
4. [Результаты и графики](#4-результаты-и-графики)
5. [Тесты и эталон SlowEval](#5-тесты-и-эталон-sloweval)
6. [Профилирование CPU и памяти](#6-профилирование-cpu-и-памяти)
7. [Вывод](#7-вывод)

---

## 1. Постановка

### Соответствие ТЗ (чеклист)

- **1) Координатный индекс + булевы операции + ADJ/NEAR**: [`index.go`](internal/ir/index.go), [`eval.go`](internal/ir/eval.go), [`ast.go`](internal/ir/ast.go). `AND` — `intersectSortedSkip` ([`eval.go`](internal/ir/eval.go)).
- **2) Сложные запросы**: [`parse.go`](internal/ir/parse.go) (`NOT > AND > OR`, `NEAR`/`ADJ`/`FIRST`/`LAST`).
- **3) Дисковый индекс + mmap**: [`storage.go`](internal/ir/storage.go) `SaveCompressed`, `OpenMMapIndex`.
- **4) Сжатие**: delta + **bit-packing** (`IRIXV2BP`), [`encodePostings`](internal/ir/storage.go), [`bitpack.go`](internal/ir/bitpack.go).
- **5) BM25**: [`bm25.go`](internal/ir/bm25.go), [`collect.go`](internal/ir/collect.go).
- **6) Бенчмарки**: [`benchmark_test.go`](internal/ir/benchmark_test.go), `Makefile`, `metrics/`.
- **7) Стенд запросов**: [`cmd/irquery`](cmd/irquery/main.go) — REPL / `-q` по `.irx`.

| Требование | Где в коде | Как проверено |
|:-----------|:-----------|:--------------|
| Буфер позиций при сборке | `posArena`, `scratchKeys` — [`index.go`](internal/ir/index.go) | табл. 4.2, §6.4 |
| Сборка вики без копии текстов | `AddLean` — [`index.go`](internal/ir/index.go) | табл. 4.1б, `irindex` |
| Буферы на запросах | `EvalCtx`, `PostingIndex` — [`eval_ctx.go`](internal/ir/eval_ctx.go) | табл. 4.3–4.4, §6.2 |
| mmap | `OpenMMapIndex` — [`storage.go`](internal/ir/storage.go) | `TestCompressedMMapRoundtrip`, `irquery` |
| Интервалы `BENCH_CORPUS` | `400,2000` синтетика | табл. 3.1, 4.4–4.5 |
| Операторы по отдельности | `BenchmarkOp` | табл. 4.5 |
| Размер до/после сжатия | `MeasureIndexSizes`, `irindex` | **табл. 4.1а–4.1б** |
| Построение на корпусе | `irindex -maxdocs 20000` | **табл. 4.1б** |

---

## 2. Реализация и язык запросов

### Структура кода

| Файл | Назначение |
|:-----|:-----------|
| [`internal/ir/index.go`](internal/ir/index.go) | `InvIndex`, `Add` / `AddLean`, `posArena`, `scratchKeys` |
| [`internal/ir/bitpack.go`](internal/ir/bitpack.go) | упаковка потоков uint32 |
| [`internal/ir/storage.go`](internal/ir/storage.go) | `SaveCompressed`, `OpenMMapIndex`, формат `IRIXV2BP` |
| [`internal/ir/corpus.go`](internal/ir/corpus.go) | `BuildIndexFromWikiXML`, UTF-8 `Tokenize` |
| [`internal/ir/tokenize.go`](internal/ir/tokenize.go) | токены (латиница + кириллица) |
| [`internal/ir/eval.go`](internal/ir/eval.go), [`eval_ctx.go`](internal/ir/eval_ctx.go) | оценка, `EvalIndex` / mmap |
| [`internal/ir/search_mmap.go`](internal/ir/search_mmap.go) | `SearchBoolMMap` |
| [`cmd/irindex`](cmd/irindex/main.go) | построение `.irx` из XML |
| [`cmd/irquery`](cmd/irquery/main.go) | консольный стенд запросов |

### Консольный стенд (`irquery`)

```bash
go build -o bin/irquery ./cmd/irquery
./bin/irquery -index data/index.irx
./bin/irquery -index data/index.irx -q 'россия AND город' -limit 20
```

Запросы выполняются по **mmap**-индексу (`SearchBoolMMap`). **MSM(...)** на `.irx` недоступен (тексты документов на диск не пишутся).

---

## 3. Методика бенчмарков

```bash
make test
make collect plot          # синтетика BENCH_CORPUS=400,2000
make profile

# индекс и размеры на ruwiki:
go run ./cmd/irindex -xml ../ruwiki-latest-pages-articles.xml -maxdocs 20000 -out data/index.irx
```

### 3.1 Корпуса

| Назначение | Корпус | N |
|:-----------|:-------|--:|
| `go test -bench`, табл. 4.3–4.5 | синтетика `fillCorpus`, ~96 символов/док, 16 термов | **400, 2000** |
| размер индекса, построение, `irquery` | **ruwiki** `ruwiki-latest-pages-articles.xml` | **20 000** статей |

`BENCH_CORPUS=400,2000` — одни и те же N во всех бенчах `BenchmarkBuildIndex|Query|Op`.

| Сценарий | Бенч / команда |
|:---------|:---------------|
| Build (синт.) | `BenchmarkBuildIndex/corpN` |
| Смешанный запрос | `BenchmarkQueryEvalMixed/idx_N`, `scan_N` |
| ADJ / NEAR | `BenchmarkQueryAdjNear/idx_adj_N`, `idx_near_N` |
| Каждый оператор | `BenchmarkOp/<OP>/idx/corpN` |

---

## 4. Результаты и графики

### 4.1а Синтетика — размер индекса (RAM vs `.irx`)

[`TestIndexSizesOnSynthetic`](internal/ir/ir_test.go), `fillCorpus`. **1 КБ = 1024 байт.**

| N | термов | RAM, КБ | файл `.irx`, КБ | сжатие |
|--:|-------:|--------:|----------------:|-------:|
| 400 | 16 | **9 950** | **17** | **580×** |
| 2000 | 16 | **239 554** | **85** | **2833×** |

На синтетике в RAM ещё лежат тексты в `Docs` (`Add`); коэффициент завышен за счёт маленького файла.

### 4.1б Ruwiki — построение и размеры (**N = 20 000**)

Команда: `go run ./cmd/irindex -xml ../ruwiki-latest-pages-articles.xml -maxdocs 20000 -out data/index.irx`  
Сборка: `AddLean` (без хранения тел статей), токенизация UTF-8.

| метрика | значение |
|:--------|--------:|
| страниц просмотрено | 20 002 |
| проиндексировано | **20 000** |
| **время построения** | **1 м 19 с** |
| термов в индексе | 1 655 705 |
| постинговых записей | 18 084 944 |
| RAM (оценка постингов), КБ | **1 547 332** |
| файл `data/index.irx`, КБ | **194 162** |
| сжатие RAM / файл | **≈8×** |
| peak RSS при сборке | ≈4,3 ГБ |

Полный дамп (все статьи) — отдельный прогон `-maxdocs 0`; для отчёта зафиксирован срез **20 000** статей, как в методичке по масштабу.

### 4.2 Сравнение с первой версией (синтетика, `benchmarks_before_refactor.csv`)

| Сценарий | N | метрика | было | стало | Δ |
|:---------|--:|:--------|-----:|------:|--:|
| **BuildIndex** | 400 | B/op | 1 364 405 | 1 102 132 | **−19%** |
| **BuildIndex** | 2000 | B/op | 7 237 768 | 6 059 898 | **−16%** |
| **BuildIndex** | 2000 | ns/op | 17.5M | 10.4M | **−40%** |
| QueryEvalMixed | 2000 | ns/op (idx) | 1.52M | 0.99M | **−35%** |
| QueryEvalMixed | 2000 | B/op (idx) | 436 303 | 485 463 | +11%¹ |
| QueryAdjNear | 2000 | ns/op (idx_adj) | 26 754 | 47 207 | +77%² |
| QueryAdjNear | 400 | ns/op (idx_adj) | 7 308 | 6 691 | **−8%** |

¹ Доминирует `msmInDoc`. ² Составной `ADJ(…) AND NOT EDGE_END(…)` — см. табл. 4.5.

### 4.3 Исправление сборки вики (почему «висело» 30+ мин)

| Проблема | Следствие | Исправление |
|:---------|:----------|:------------|
| `for k := range tokScratch` на каждый `Add` | O(документы × словарь), рост времени | `scratchKeys` — сброс только термов текущей статьи |
| `Docs.Tokens` для каждой статьи | гигабайты RAM | `AddLean` при загрузке XML |
| токенизация по байтам | битая кириллица | `utf8.DecodeRuneInString` |

После правок: **5 000** статей ≈ **25 с**, **20 000** ≈ **1 м 19 с** (табл. 4.1б).

### 4.4 Агрегат `metrics/raw/benchmarks.csv` (синтетика)

| bench | режим | N | ns/op | B/op |
|:------|:------|--:|------:|-----:|
| BenchmarkBuildIndex | build | 400 | 1 787 792 | 1 102 132 |
| BenchmarkBuildIndex | build | 2000 | 10 413 717 | 6 059 898 |
| BenchmarkQueryEvalMixed | idx | 400 | 153 577 | 96 345 |
| BenchmarkQueryEvalMixed | scan | 400 | 204 347 | 100 120 |
| BenchmarkQueryEvalMixed | idx | 2000 | 992 846 | 485 463 |
| BenchmarkQueryEvalMixed | scan | 2000 | 1 163 025 | 494 681 |
| BenchmarkQueryAdjNear | idx_adj | 400 | 6 691 | 8 352 |
| BenchmarkQueryAdjNear | idx_near | 400 | 8 183 | 4 152 |
| BenchmarkQueryAdjNear | idx_adj | 2000 | 47 207 | 42 656 |
| BenchmarkQueryAdjNear | idx_near | 2000 | 32 074 | 20 008 |

#### Рисунок 4.1 — построение индекса (синт.)

![Build index](./metrics/plots/build_index_ns.png)

#### Рисунок 4.2 — запрос: индекс vs полный скан (синт.)

![Query idx vs scan](./metrics/plots/query_idx_vs_scan.png)

### 4.5 Операторы по отдельности — `BenchmarkOp`, N = 2000 (синт., idx)

| OP | ns/op | B/op | allocs/op |
|:---|------:|-----:|----------:|
| AND | 15 283 | 19 064 | 12 |
| OR | 19 268 | 27 256 | 13 |
| NOT | 8 891 | 15 736 | 11 |
| **ADJ** | **8 597** | **1 016** | **7** |
| **NEAR** | **9 146** | **4 088** | **9** |
| EDGE | 28 080 | 44 832 | 40 |
| MSM | 618 993 | 272 000 | 570 |

Чистый **ADJ**: **1 016 B/op** vs составной `idx_adj` (**11 760 B/op**) — **≈11×**.

---

## 5. Тесты и эталон SlowEval

[`internal/ir/ir_test.go`](internal/ir/ir_test.go): `Eval` vs `SlowEval`, mmap roundtrip (`IRIXV2BP`), размеры, `TestBuildIndexFromWikiXMLSample`, `TestTokenizeCyrillic`.

```bash
go test ./... -coverprofile=coverage.out
```

---

## 6. Профилирование CPU и памяти

`make profile`, синтетика **N = 2000**.

### 6.1–6.2 Запрос Eval

![Flame CPU query](./metrics/plots/flamegraph_cpu_query_idx.png)

![Flame mem query](./metrics/plots/flamegraph_mem_query_idx.png)

| Показатель | Первая версия | Текущая |
|:-----------|:-------------|:--------|
| `setToSortedIDs` / `postingsDocSet` в топе | **да** | **нет** |
| `intersectSortedSkipInto` / `EvalCtx` | нет | **есть** |
| alloc_space (смешанный idx) | ≈812 MB | ≈651 MB |

### 6.3–6.4 Построение индекса

![Flame CPU build](./metrics/plots/flamegraph_cpu_build_index.png)

![Flame mem build](./metrics/plots/flamegraph_mem_build_index.png)

---

## 7. Вывод

Реализованы: координатный индекс, булевы операторы, ADJ/NEAR/edge, BM25, **bit-packing** + mmap (`IRIXV2BP`), консольный **`irquery`**.

| Корпус | Главные цифры |
|:-------|:--------------|
| синтетика N=2000 | сжатие **2833×**, build **−40% ns**, mixed idx **−35% ns** (табл. 4.2–4.5) |
| **ruwiki N=20 000** | построение **1 м 19 с**, `.irx` **194 162 КБ**, RAM постингов **1 547 332 КБ** (табл. 4.1б) |

Запросы к боевому индексу — через `irquery` по `data/index.irx`. MSM и тяжёлые составные запросы по-прежнему доминируют в профиле на синтетике.
