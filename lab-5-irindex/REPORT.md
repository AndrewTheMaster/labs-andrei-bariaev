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

- **1) Координатный индекс + булевы операции + ADJ/NEAR**: реализовано в [`internal/ir/index.go`](internal/ir/index.go), [`internal/ir/eval.go`](internal/ir/eval.go), [`internal/ir/ast.go`](internal/ir/ast.go).  
  Для `AND` применяется пересечение отсортированных `docID` с виртуальными skip-прыжками (`intersectSortedSkip`), без выделения отдельной skip-структуры в памяти.
- **2) Сложные запросы**: реализован парсер выражений с приоритетами `NOT > AND > OR`, скобками и функциями `NEAR/ADJ/FIRST/LAST` в [`internal/ir/parse.go`](internal/ir/parse.go).
- **3) Дисковый индекс + mmap**: сериализация и mmap-загрузка в [`internal/ir/storage.go`](internal/ir/storage.go) (`SaveCompressed`, `OpenMMapIndex`).
- **4) Сжатие индекса**: `delta-encoding + varint bitpacking` для `docID` и позиций в [`internal/ir/storage.go`](internal/ir/storage.go).
- **5) Простейшее ранжирование TF/IDF(BM25)**: [`internal/ir/bm25.go`](internal/ir/bm25.go) + извлечение позитивных терминов [`internal/ir/collect.go`](internal/ir/collect.go).
- **6) Бенчмарки и профилирование**: [`internal/ir/benchmark_test.go`](internal/ir/benchmark_test.go), `Makefile`, `metrics/raw`, `metrics/profiles`, `metrics/plots`.

---

## 2. Реализация и язык запросов

### Структура кода

| Файл | Назначение |
|:-----|:-----------|
| [`internal/ir/index.go`](internal/ir/index.go) | `InvIndex`, `Doc`, постинги, `df`, добавление документов (`posArena`, переиспользуемый `tokScratch`) |
| [`internal/ir/ast.go`](internal/ir/ast.go) | AST: `Term`, `Not`, `And`, `Or`, `Near`, `Adj`, границы |
| [`internal/ir/parse.go`](internal/ir/parse.go) | парсер: `and` / `or` / `not`, скобки, `NEAR (…)`, `ADJ (…)`, **`FIRST`**/**`LAST`** и синонимы **`EDGE_START`/`EDGE_END`** |
| [`internal/ir/eval.go`](internal/ir/eval.go) | интерпретация над индексом (пересечения постингов с skip-прыжками, ADJ/NEAR/границы, MSM) |
| [`internal/ir/eval_ctx.go`](internal/ir/eval_ctx.go) | `EvalCtx` — булевы операции с переиспользованием буферов `docID` |
| [`internal/ir/scan.go`](internal/ir/scan.go) | **`SlowEval`** — полный проход текстов документа (эталон) |
| [`internal/ir/bm25.go`](internal/ir/bm25.go) | BM25; при отсутствии положительных терминов — сортировка по `DocID` |
| [`internal/ir/search.go`](internal/ir/search.go) | `SearchBoolEval`, `SearchBM25` |
| [`internal/ir/storage.go`](internal/ir/storage.go) | сериализация на диск (`SaveCompressed`), mmap-открытие (`OpenMMapIndex`), декодирование постингов по терму |

Булева часть запросов идёт через **`EvalCtx`**: пересечение/объединение/дополнение отсортированных списков `docID` в переиспользуемых срезах (`intersectSortedSkipInto` и т.д.), без промежуточных `map`. При **`Add`** позиции термов пишутся в общий **`posArena`**, а не копируются отдельным `[]uint32` на каждый терм.

## 3. Методика бенчмарков

Команды ([`Makefile`](Makefile)):

```bash
make test           # go test ./...
make collect plot   # metrics/raw/*.txt, csv, gnuplot PNG в metrics/plots/
make profile        # два сценария: Eval + построение индекса; *.prof, top, -png и flame graphs
```

- **`BENCH_CORPUS`** — список размеров синтетического корпуса (число документов), по умолчанию `400,2000`.
- Имена подбенчей согласованы с суффиксом `-GOMAXPROCS` в выводе `go test`: `BenchmarkBuildIndex/corpN`, `BenchmarkQueryEvalMixed/idx_N` и `…/scan_N`, чтобы `awk` в `collect` корректно строил `benchmarks.csv`.

Сценарии:

1. **`BenchmarkBuildIndex`** — полная индексация корпуса за одну итерацию (`ns/op`).
2. **`BenchmarkQueryEvalMixed`** — один и тот же тяжёлый запрос через **`EvalCtx`** (индекс) vs **`SlowEval`** (линейный скан текстов):

   `(alpha AND beta) OR MSM(40, gamma, omega) AND NOT FIRST(delta)`.

3. **`BenchmarkQueryAdjNear`** — сценарии строго по операторам ТЗ:
   - `ADJ(alpha,beta) AND NOT EDGE_END(delta)` (idx vs scan);
   - `NEAR(3,alpha,gamma) OR ADJ(gamma,omega)` (idx vs scan).

На машине отчёта: **`goos: linux`**, **`goarch: amd64`**, см. строку `cpu:` в [`metrics/raw/benchmarks.txt`](metrics/raw/benchmarks.txt).

---

## 4. Результаты и графики

### Таблица — агрегат `metrics/raw/benchmarks.csv` (прогон `BENCH_CORPUS=400,2000`)

| bench | режим | документов | iters | ns/op | B/op |
|:------|:------|-----------:|------:|------:|-----:|
| BenchmarkBuildIndex | build | 400 | 331 | 1 787 792 | 1 102 132 |
| BenchmarkBuildIndex | build | 2000 | 57 | 10 413 717 | 6 059 898 |
| BenchmarkQueryEvalMixed | idx | 400 | 3874 | 153 577 | 96 345 |
| BenchmarkQueryEvalMixed | scan | 400 | 2678 | 204 347 | 100 120 |
| BenchmarkQueryEvalMixed | idx | 2000 | 561 | 992 846 | 485 463 |
| BenchmarkQueryEvalMixed | scan | 2000 | 526 | 1 163 025 | 494 681 |
| BenchmarkQueryAdjNear | idx_adj | 400 | 85272 | 6 691 | 8 352 |
| BenchmarkQueryAdjNear | scan_adj | 400 | 24727 | 24 779 | 248 |
| BenchmarkQueryAdjNear | idx_near | 400 | 118335 | 8 183 | 4 152 |
| BenchmarkQueryAdjNear | scan_near | 400 | 9992 | 55 971 | 2 040 |
| BenchmarkQueryAdjNear | idx_adj | 2000 | 15512 | 47 207 | 42 656 |
| BenchmarkQueryAdjNear | scan_adj | 2000 | 2354 | 237 486 | 1 016 |
| BenchmarkQueryAdjNear | idx_near | 2000 | 17451 | 32 074 | 20 008 |
| BenchmarkQueryAdjNear | scan_near | 2000 | 849 | 627 293 | 7 544 |

**Интерпретация.** Построение индекса масштабируется с ростом корпуса; на N=2000 **`B/op` снизился** с **7.24M до 6.06M** (первая версия vs текущий прогон) за счёт `posArena` и `tokScratch`. На **смешанном** запросе с **MSM** на корпусе 2000 **`SlowEval` быстрее по `ns/op`**, как и раньше: короткие синтетические строки дешевле сканировать, чем гонять `msmInDoc` на индексе. По **`BenchmarkQueryAdjNear`** на N=400 idx для ADJ/NEAR **быстрее скана** по `ns/op`. Профили (§6) показывают смещение булевой части на `intersectSortedSkipInto` / `EvalCtx` вместо `setToSortedIDs` и `map`.

#### Рисунок 4.1 — построение индекса

![Build index](./metrics/plots/build_index_ns.png)

#### Рисунок 4.2 — запрос: индекс vs полный скан

![Query idx vs scan](./metrics/plots/query_idx_vs_scan.png)

---

## 5. Тесты и эталон SlowEval

Пакет [`internal/ir/ir_test.go`](internal/ir/ir_test.go):

- разбор и **`Eval`** для **NEAR**, **ADJ**, **NOT**, **`FIRST`** / **`EDGE_START`**;
- границы **`LAST`** / **`EDGE_END`**;
- roundtrip сжатого дискового индекса через `SaveCompressed` + `OpenMMapIndex`;
- проверки error-веток парсера и `mmap`-открытия (`empty`, `bad magic`, `truncated`);
- проверка `DocLen`/`df` для mmap-индекса;
- сложное комбинированное выражение `ADJ/NEAR/NOT` с `Eval` vs `SlowEval`;
- **`go test -short`**: длинные property-тесты «`SlowEval` vs `Eval`» на случайных корпусах пропускаются;
- упорядочивание **BM25** на фиксированном примере.

Текущее покрытие по `go test ./... -covermode=atomic -coverprofile=coverage.out`:

- **86.8% statements** по пакету `internal/ir` (это не полное покрытие, но покрыты ключевые пути `Eval`, парсер, BM25, сериализация/mmap и error-ветки открытия mmap-индекса).

---

## 6. Профилирование CPU и памяти

Команда `make profile` (см. [`Makefile`](Makefile)) снимает **две** ключевые нагрузки на корпус из **2000** документов:

1. **`BenchmarkQueryEvalMixed/idx_2000`** — смешанный булев запрос через индекс (`EvalCtx`).
2. **`BenchmarkBuildIndex/corp2000`** — полное построение индекса в цикле.

Для каждого случая сохраняются **CPU** и **heap** (`-memprofile`), текстовые `go tool pprof -top`, при наличии **graphviz** — **`go tool pprof [-alloc_space] -png`** в `metrics/plots/`. Скрипт [`scripts/gen_flamegraphs.sh`](scripts/gen_flamegraphs.sh) поднимает `go tool pprof -http`, забирает страницу **`/ui/flamegraph`** в HTML и (если доступен headless Chromium/Chrome) делает PNG — тот же приём, что в **lab-1 / lab-2**.

Сырые файлы: `metrics/profiles/*.prof`, топы [`cpu_query_idx_top.txt`](metrics/profiles/cpu_query_idx_top.txt), [`mem_query_idx_top.txt`](metrics/profiles/mem_query_idx_top.txt) (`-alloc_space`), [`cpu_build_index_top.txt`](metrics/profiles/cpu_build_index_top.txt), [`mem_build_index_top.txt`](metrics/profiles/mem_build_index_top.txt).

### 6.1 CPU — смешанный запрос Eval (corpus 2000)

**Рисунок 6.1 — Flame graph CPU запроса к индексу**

![Flame CPU query](./metrics/plots/flamegraph_cpu_query_idx.png)

| Компонента | Оценка из top | Интерпретация |
|:-----------|:--------------|:---------------|
| `evalMSM` / `msmInDoc` | по-прежнему заметная доля | скользящее окно MSM в смешанном запросе |
| `intersectSortedSkipInto`, `EvalCtx` | видны на булевой части | skip-пересечения без `setToSortedIDs` |
| GC + `mapassign` | слабее, чем в первой версии | меньше промежуточных `map` на AND/OR/NOT |

### 6.2 Память (`alloc_space`) — тот же запрос Eval

**Рисунок 6.2 — Flame graph памяти (alloc_space)**

![Flame mem query](./metrics/plots/flamegraph_mem_query_idx.png)

Общий объём сэмпла в топе профиля: **≈651 MB alloc_space** (агрегируются повторные итерации бенча).

| Функция | alloc_space | доля | Комментарий |
|:--------|------------:|-----:|:------------|
| `msmInDoc` | ≈484 MB | ≈74% | окно MSM в смешанном запросе |
| `intersectSortedSkipInto` | ≈34 MB | ≈5% | skip-пересечения в `EvalCtx` |
| `unionSortedInto`, `subtractSortedInto` | ≈25 MB каждый | ≈4% | OR/NOT без `map` |
| `setToSortedIDs`, `postingsDocSet` | нет в топе | — | в первой версии давали заметный «хвост» |

### 6.3 CPU — построение индекса BuildIndex/corp2000

**Рисунок 6.3 — Flame graph CPU построения**

![Flame CPU build](./metrics/plots/flamegraph_cpu_build_index.png)

В топе: **`fillCorpus` → `Tokenize`**, плюс `mapassign`/аллокатор на пути добавления документа в индекс. Это соответствует дорогому пути **`(*InvIndex).Add`** для каждого токена.

### 6.4 Память (`alloc_space`) — построение индекса

**Рисунок 6.4 — Flame graph памяти построения**

![Flame mem build](./metrics/plots/flamegraph_mem_build_index.png)

| Функция | alloc_space | доля | Комментарий |
|:--------|------------:|-----:|:------------|
| `(*InvIndex).Add` cum | основной вклад (~59% cum) | вся логика роста индексных структур; **`posArena`** |
| `Tokenize` | ≈32% flat | срезы токенов и строковые операции для каждого документа |
| `sortUint32` | ≈14% cum | упорядочивание позиций в scratch |
| `strings.Builder`, `reflect.Swapper` | заметно | сборка текстов синтетического документа + сортировка |

Интерактивные HTML тем же профилям: например [`flamegraph_mem_query_idx.html`](metrics/plots/flamegraph_mem_query_idx.html), [`flamegraph_cpu_build_index.html`](metrics/plots/flamegraph_cpu_build_index.html) (полный комплект генерирует `scripts/gen_flamegraphs.sh`).

---

## 7. Вывод

Реализованы координатный обратный индекс с позициями, парсер и вычислитель булевых запросов с **AND/OR/NOT**, **ADJ**, **NEAR** и границами документа (понятие *edge* в курсе: **`FIRST`** / **`EDGE_START`** у начала строки после токенизации, **`LAST`** / **`EDGE_END`** у конца), ранжирование **BM25** по положительным терминам. Добавлены дисковый формат с **delta+varint** и загрузка через **mmap**, а также бенчи/профили и графики на синтетическом корпусе.

Доработки по замечаниям: **`posArena`** и **`tokScratch`** при сборке, **`EvalCtx`** на запросах — меньше аллокаций на построении и на ADJ/NEAR; в flamegraph'ах булевая часть опирается на **`intersectSortedSkipInto`**, а не на `setToSortedIDs`/`map`. Смешанный запрос с **MSM** остаётся тяжёлым по CPU и памяти из‑за `msmInDoc`; на короткой синтетике для него по-прежнему выгоден линейный скан по `ns/op`.
