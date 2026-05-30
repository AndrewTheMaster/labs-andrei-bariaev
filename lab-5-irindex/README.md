# lab-5-irindex

Обратный позиционный индекс: булевы запросы (AND/OR/NOT, ADJ, NEAR, границы документа), BM25, сжатие на диске (**delta + bit-packing**), чтение через **mmap**, консольный стенд запросов.

## Требования

- Go 1.22+
- Распакованный дамп ruwiki (опционально для вики-бенчей и сборки индекса)

По умолчанию ищется XML:

1. `$CORPUS_XML` / `$WIKI_XML`
2. `data/ruwiki-latest-pages-articles.xml`
3. `../ruwiki-latest-pages-articles.xml` (корень репозитория SIAOD)

```bash
# если дамп лежит в корне репозитория:
ln -sf ../ruwiki-latest-pages-articles.xml data/ruwiki-latest-pages-articles.xml
# или
export WIKI_XML=../ruwiki-latest-pages-articles.xml
```

## Сборка

```bash
go build -o bin/irindex ./cmd/irindex
go build -o bin/irquery ./cmd/irquery
```

## Индекс с вики

Сборка идёт через `AddLean` (тексты статей в RAM не копируются). Прогресс: `WIKI_PROGRESS=5000` (каждые N документов в stderr).

```bash
# лимит для проверки (~25 с на 5k статей после оптимизации Add):
go run ./cmd/irindex -xml ../ruwiki-latest-pages-articles.xml -maxdocs 5000 -out data/index.irx

# все страницы (долго — весь XML):
go run ./cmd/irindex -xml ../ruwiki-latest-pages-articles.xml -maxdocs 0 -out data/index.irx

make build-index WIKI_XML=../ruwiki-latest-pages-articles.xml
```

Печатаются: время построения, RAM (КБ), размер `.irx` (КБ), коэффициент сжатия.

Формат файла: magic `IRIXV2BP`, постинги — **bit-packing** трёх потоков (doc Δ, tf, pos Δ).

## Стенд запросов (консоль)

По mmap-индексу на диске:

```bash
./bin/irquery -index data/index.irx
```

Одна команда:

```bash
./bin/irquery -index data/index.irx -q 'россия AND город'
./bin/irquery -index data/index.irx -q 'ADJ(россия, город)' -limit 50
./bin/irquery -index data/index.irx -q 'россия AND город' -rank -limit 20
```

Поддерживаются: `AND`, `OR`, `NOT`, `ADJ(a, b)`, `NEAR(k, a, b)`, `FIRST(term)`, `EDGE_END(term)`.  
**`-rank`** — BM25 после булева фильтра (`-k1`, `-b`). В REPL: `:rank on|off`.  
**MSM(...)** на дисковом индексе недоступен (в `.irx` нет текстов документов). Термы — UTF-8 (кириллица).

## Тесты и бенчмарки

```bash
make test

# синтетика (по умолчанию N=400,2000)
make collect plot

# вики: нужен распакованный XML
export WIKI_XML=../ruwiki-latest-pages-articles.xml
make bench-wiki BENCH_CORPUS=5000,10000,20000 BENCH_TIME=300ms \
  BENCH_FILTER='^Benchmark(QueryEvalMixed|Op)'
# результат: metrics/raw/benchmarks_wiki.txt
make collect plot

make profile
```

Переменные:

| Переменная | Значение по умолчанию |
|:-----------|:----------------------|
| `BENCH_CORPUS` | `400,2000` |
| `BENCH_FILTER` | `^Benchmark(BuildIndex\|Query\|Op)` |
| `WIKI_XML` | см. `ResolveCorpusPath()` |

## Структура

| Путь | Назначение |
|:-----|:-----------|
| `internal/ir/index.go` | RAM-индекс, `posArena` |
| `internal/ir/storage.go` | `SaveCompressed`, `OpenMMapIndex`, bit-packing |
| `internal/ir/bitpack.go` | упаковка потоков uint32 |
| `internal/ir/eval_ctx.go` | оценка запросов с переиспользованием буферов |
| `cmd/irindex` | построение `.irx` из XML |
| `cmd/irquery` | REPL / `-q` по mmap |
| `REPORT.md` | отчёт по лабораторной |
| `metrics/` | csv, gnuplot, pprof |

## Отчёт

См. [REPORT.md](REPORT.md). После прогона на вики обновите таблицы: `make collect`, размеры — `go run ./cmd/irindex …`.
