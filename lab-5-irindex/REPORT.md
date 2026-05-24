# Лабораторная работа №5 — Обратный индекс, булевы запросы, mmap, сжатие, TF/IDF(BM25)

**Дисциплина:** Структуры и алгоритмы в базах данных и распределённых системах  
**Тема:** Инвертированный индекс с позициями; операторы **AND / OR / NOT**, **ADJ**, **NEAR**, границы документа (**«edge»**); хранение с mmap и сжатием; ранжирование **BM25** поверх булева фильтра.

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

- **1) Координатный индекс + булевы операции + ADJ/NEAR**: [`internal/ir/index.go`](internal/ir/index.go), [`internal/ir/eval.go`](internal/ir/eval.go), [`internal/ir/ast.go`](internal/ir/ast.go).  
  `AND` — `intersectSortedSkip` ([`eval.go`](internal/ir/eval.go) **26–74**).
- **2) Сложные запросы**: [`internal/ir/parse.go`](internal/ir/parse.go) (`NOT > AND > OR`, `NEAR`/`ADJ`/`FIRST`/`LAST`).
- **3) Дисковый индекс + mmap**: [`storage.go`](internal/ir/storage.go) `SaveCompressed`, `OpenMMapIndex` (**148–259**).
- **4) Сжатие**: delta + varint, [`encodePostings`](internal/ir/storage.go) **45–118**.
- **5) BM25**: [`bm25.go`](internal/ir/bm25.go), [`collect.go`](internal/ir/collect.go).
- **6) Бенчмарки**: [`benchmark_test.go`](internal/ir/benchmark_test.go), `Makefile`, `metrics/`.

| Требование (замечания) | Где в коде | Как проверено |
|:-----------------------|:-----------|:--------------|
| Меньше аллокаций при сборке (общий буфер позиций) | `posArena`, `tokScratch` — [`index.go`](internal/ir/index.go) **40–63** | табл. 4.2, §6.4 |
| Переиспользование буферов на запросах (без `map` на AND/OR/NOT) | `EvalCtx` — [`eval_ctx.go`](internal/ir/eval_ctx.go) **4–84** | табл. 4.2–4.3, §6.2 |
| mmap дискового индекса | `OpenMMapIndex` — [`storage.go`](internal/ir/storage.go) **148–174** | `TestCompressedMMapRoundtrip` |
| Одинаковые интервалы корпуса | `BENCH_CORPUS=400,2000` — все бенчи | табл. 3.1 |
| Операторы по отдельности | `BenchmarkOp` — [`benchmark_test.go`](internal/ir/benchmark_test.go) **96–135** | табл. 4.3 |
| Размер индекса до/после сжатия | `MeasureIndexSizes` — [`stats.go`](internal/ir/stats.go) **44–64** | **табл. 4.1** |
| Построение индекса | `BenchmarkBuildIndex` | табл. 4.2 |

---

## 2. Реализация и язык запросов

### Структура кода

| Файл | Назначение |
|:-----|:-----------|
| [`internal/ir/index.go`](internal/ir/index.go) | `InvIndex`, `Add`, `posArena` (**51–60**), `tokScratch` |
| [`internal/ir/ast.go`](internal/ir/ast.go) | AST: `Term`, `Not`, `And`, `Or`, `Near`, `Adj`, границы |
| [`internal/ir/parse.go`](internal/ir/parse.go) | парсер, `Parse` (**339–357**) |
| [`internal/ir/eval.go`](internal/ir/eval.go) | `intersectSortedSkip`, ADJ/NEAR/MSM, `Eval` (**149–152**) |
| [`internal/ir/eval_ctx.go`](internal/ir/eval_ctx.go) | `EvalCtx`, `intersectSortedSkipInto` |
| [`internal/ir/scan.go`](internal/ir/scan.go) | `SlowEval` (**4–15**) |
| [`internal/ir/bm25.go`](internal/ir/bm25.go) | BM25 |
| [`internal/ir/search.go`](internal/ir/search.go) | `SearchBoolEval`, `SearchBM25` |
| [`internal/ir/storage.go`](internal/ir/storage.go) | `SaveCompressed`, `OpenMMapIndex`, mmap |
| [`internal/ir/stats.go`](internal/ir/stats.go) | `EstimateIndexBytes`, `MeasureIndexSizes` |

---

## 3. Методика бенчмарков

```bash
make test
make collect plot
make profile
```

### 3.1 Единые интервалы и сценарии

| Параметр | Значение |
|:---------|:---------|
| Корпус | синтетика `fillCorpus`, ~96 символов/док |
| **`BENCH_CORPUS`** | **`400,2000`** — одни и те же N во всех бенчах |
| Build | `BenchmarkBuildIndex/corpN` |
| Смешанный запрос | `BenchmarkQueryEvalMixed/idx_N`, `…/scan_N` |
| ADJ / NEAR (составные) | `BenchmarkQueryAdjNear/idx_adj_N`, `idx_near_N`, scan |
| **Каждый оператор отдельно** | `BenchmarkOp/<OP>/idx\|scan/corpN` |

| Оператор | Запрос в `BenchmarkOp` |
|:---------|:----------------------|
| AND | `alpha AND beta` |
| OR | `alpha OR gamma` |
| NOT | `NOT delta` |
| ADJ | `ADJ(alpha, beta)` |
| NEAR | `NEAR(3, alpha, gamma)` |
| EDGE | `FIRST(alpha) AND NOT EDGE_END(delta)` |
| MSM | `MSM(40, gamma, omega, alpha)` |

Сырые логи: [`metrics/raw/benchmarks.txt`](metrics/raw/benchmarks.txt), снимок первой версии: [`benchmarks_before_refactor.csv`](metrics/raw/benchmarks_before_refactor.csv).

---

## 4. Результаты и графики

### 4.1 Размер индекса: до сжатия (RAM) и после (файл `.irx`)

[`stats.go`](internal/ir/stats.go) **20–64**, [`TestIndexSizesOnSynthetic`](internal/ir/ir_test.go) **234–245**, синтетика `fillCorpus`.

| N | термов | RAM, КБ | файл `.irx`, КБ | сжатие |
|--:|-------:|--------:|----------------:|-------:|
| 400 | 16 | **9 950** | **17** | **580×** |
| 2000 | 16 | **239 554** | **85** | **2833×** |

**1 КБ = 1024 байт** (`raw_bytes` / `compressed_bytes` в тесте, делим на 1024). RAM — `EstimateIndexBytes` (тексты в `Docs` + постинги); файл — только сжатые постинги (delta+varint).

### 4.2 Сравнение с первой версией (`benchmarks_before_refactor.csv` → текущий прогон)

| Сценарий | N | метрика | было | стало | Δ |
|:---------|--:|:--------|-----:|------:|--:|
| **BuildIndex** | 400 | B/op | 1 364 405 | 1 102 132 | **−19%** |
| **BuildIndex** | 2000 | B/op | 7 237 768 | 6 059 898 | **−16%** |
| **BuildIndex** | 2000 | ns/op | 17.5M | 10.4M | **−40%** |
| QueryEvalMixed | 2000 | ns/op (idx) | 1.52M | 0.99M | **−35%** |
| QueryEvalMixed | 2000 | B/op (idx) | 436 303 | 485 463 | +11%¹ |
| QueryAdjNear | 2000 | ns/op (idx_adj) | 26 754 | 47 207 | +77%² |
| QueryAdjNear | 400 | ns/op (idx_adj) | 7 308 | 6 691 | **−8%** |

¹ Смешанный запрос с MSM; доминирует `msmInDoc`, не булева часть.  
² Составной запрос `ADJ(…) AND NOT EDGE_END(…)`, не чистый ADJ — см. табл. 4.3.

**Построение индекса (цель ~3 с):** на синтетике N=2000 одна итерация `BenchmarkBuildIndex` ≈ **10.4 ms** (табл. 4.4); проверка «3 с на полном корпусе» — отдельный прогон на большом дампе, не входит в этот отчёт.

### 4.3 Операторы по отдельности — `BenchmarkOp`, N = 2000 (idx)

`go test -bench='BenchmarkOp/.*/idx/corp2000' -benchmem`, `BENCH_CORPUS=2000`.

| OP | ns/op | B/op | allocs/op |
|:---|------:|-----:|----------:|
| AND | 15 283 | 19 064 | 12 |
| OR | 19 268 | 27 256 | 13 |
| NOT | 8 891 | 15 736 | 11 |
| **ADJ** | **8 597** | **1 016** | **7** |
| **NEAR** | **9 146** | **4 088** | **9** |
| EDGE | 28 080 | 44 832 | 40 |
| MSM | 618 993 | 272 000 | 570 |

**Чистый ADJ** (табл. 4.3): **1 016 B/op** vs старый `idx_adj` (**11 760 B/op**, составной запрос, табл. 4.2) — **≈11×**.

### 4.4 Агрегат `metrics/raw/benchmarks.csv` (`BENCH_CORPUS=400,2000`)

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

#### Рисунок 4.1 — построение индекса

![Build index](./metrics/plots/build_index_ns.png)

#### Рисунок 4.2 — запрос: индекс vs полный скан

![Query idx vs scan](./metrics/plots/query_idx_vs_scan.png)

---

## 5. Тесты и эталон SlowEval

[`internal/ir/ir_test.go`](internal/ir/ir_test.go): `Eval` vs `SlowEval` (**129–176**), mmap roundtrip (**199–232**), размеры (**234–245**).

Покрытие: **86.8%** statements (`go test ./... -coverprofile=coverage.out`).

---

## 6. Профилирование CPU и памяти

`make profile`, корпус **2000**: `BenchmarkQueryEvalMixed/idx_2000`, `BenchmarkBuildIndex/corp2000`.

### 6.1–6.2 Запрос Eval — flamegraph

![Flame CPU query](./metrics/plots/flamegraph_cpu_query_idx.png)

![Flame mem query](./metrics/plots/flamegraph_mem_query_idx.png)

| Показатель | Первая версия (смешанный idx) | Текущая |
|:-----------|:----------------------------|:--------|
| Доля `msmInDoc` (alloc) | ≈52% | ≈74% |
| `setToSortedIDs` / `postingsDocSet` в топе | **да** | **нет** |
| `intersectSortedSkipInto` / `EvalCtx` | нет | **≈5% + ≈31% cum** |
| Суммарный alloc_space (сэмпл бенча) | ≈812 MB | ≈651 MB |

### 6.3–6.4 Построение индекса — flamegraph

![Flame CPU build](./metrics/plots/flamegraph_cpu_build_index.png)

![Flame mem build](./metrics/plots/flamegraph_mem_build_index.png)

| Показатель | Первая версия | Текущая |
|:-----------|:-------------|:--------|
| `(*InvIndex).Add` cum (build) | ≈67% | **≈59%** |
| `Tokenize` flat | ≈26% | **≈32%** |

HTML: [`flamegraph_mem_query_idx.html`](metrics/plots/flamegraph_mem_query_idx.html), [`flamegraph_cpu_build_index.html`](metrics/plots/flamegraph_cpu_build_index.html).

---

## 7. Вывод

Координатный индекс, булевы операторы, ADJ/NEAR/edge, BM25, диск + mmap + сжатие — по ТЗ. Измеримые улучшения на синтетике: **табл. 4.1** (сжатие до **2833×**), **табл. 4.2** (build **−16…−40%**, mixed idx **−35% ns**), **табл. 4.3** (отдельные операторы), **§6** (профиль без `setToSortedIDs`/`postingsDocSet`). MSM и составные ADJ/NEAR-запросы остаются тяжёлыми — видно в табл. 4.3–4.4 и flamegraph.
