# Лабораторная работа №4 — Потокобезопасная хеш-таблица с закрытой адресацией

**Дисциплина:** Структуры и алгоритмы в базах данных и распределённых системах  
**Тема:** Striping + per-bucket RW-lock: сравнение с «одной глобальной RWMutex» вокруг обычной `map`

---

## Содержание

1. [Постановка и соответствие CHM](#1-постановка-и-соответствие-chm)
2. [Реализация](#2-реализация)
3. [Методика бенчмарков](#3-методика-бенчмарков)
4. [Результаты и графики](#4-результаты-и-графики)
5. [Concurrency-тесты («аналог jcstress»)](#5-concurrency-тесты-аналог-jcstress)
6. [Профилирование CPU и памяти](#6-профилирование-cpu-и-памяти)
7. [Вывод](#7-вывод)

---

## 1. Постановка и соответствие CHM

Требуется **закрытая адресация** (цепочки в бакетах) и API:

- **`Put` / `Get` / `Size` / `Clear` / `Merge` / `Range`**
- операции чтения (**`Get`/`Range`**) **«почти никогда не блокируют»** за исключением конфликтов с писателями того же сегмента;
- **happens-before**: завершённая запись видна последующим успешным чтениям тем же или другим горутинам (см. память-модель Go `sync`: release при `Unlock` / `RUnlock` и acquire при последующем `Lock` / `RLock` — аналог идеи *retrieval operations do not block …* и *happens-before ordering* в [документации `ConcurrentHashMap`](https://docs.oracle.com/en/java/javase/21/docs/api/java.base/java/util/concurrent/ConcurrentHashMap.html)).

**Baseline** — `Plain[K,V]`: одна `sync.RWMutex` + встроенная `map`; любой `Get` берёт глобальный `RLock`, поэтому **параллельные записи в другие ключи** всё равно блокируют **всех** читателей.

**Основная структура** — `Map[K,V]` в [`internal/concmap/map.go`](internal/concmap/map.go): `2^bucketBits` бакетов, каждый хранит указатель на односвязную цепочку и **свой** `sync.RWMutex`. Хеш — `uint64 & (n-1)`; для `string` ключей выбран быстрый путь без `reflect` в [`makeDefaultHashFunc`](internal/concmap/map.go).

**`Merge`**, как в JDK: если ключа не было — сохраняется `value` без вызова функции слияния; иначе **`merger(existing, incoming)`**.

**`Range`**: итерация бакет за бакетом под `RLock`, слабая согласованность (как weakly-consistent view у CHM): живых паник из-за параллельных модификаций нет, но «снимок всей таблицы» не гарантируется.

**`Size`** — атомарный счётчик корректируется на вставку нового ключа (`Put`/`Merge`); `Clear` обнуляет счётчик **под удержанием всех бакетных Lock** (см. порядок ниже в коде: сначала очистка цепочек + `size.Store(0)`, затем `Unlock`).

---

## 2. Реализация

| Файл | Назначение |
|:-----|:-----------|
| [`internal/concmap/map.go`](internal/concmap/map.go) | `Map`, `New`, `Put`, `Get`, `Merge`, `Clear`, `Size`, `Range`, `WithHasher` |
| [`internal/concmap/plain.go`](internal/concmap/plain.go) | эталонная «одна mutex + map» оболочка |
| [`internal/concmap/hasher.go`](internal/concmap/hasher.go) | reflect-хэш для общих `K` или кастомная функция |

---

## 3. Методика бенчмарков

Запуск через [`Makefile`](Makefile):

```bash
make collect plot   # gnuplot строит PNG latency в metrics/plots/
make profile        # CPU+heap prof, pprof -png (+ flame graph HTML/PNG см. §6)
make test-race      # -race + многократные concurrency тесты
```

Переменные:

- **`BENCH_KEYS`** — два размера предзаполненной таблицы (ключи `fmt.Sprintf("k_%d")`);
- каждый сценарий — `testing.B.RunParallel` (столько процессоров, сколько `GOMAXPROCS`).

Измерены четыре сценария:

1. **`BenchmarkParallelGetHit`** — успешное чтение существующих ключей под конкуренцией;
2. **`BenchmarkParallelPutOverwrite`** — перезапись фиксированного набора ключей (тяжёлое соперничество записи);
3. **`BenchmarkParallelMixedRW`** — цикл `Get` / `Put` / отдельные `mix_%d`-ключи для `Merge` в пропорции 1:1:1;
4. **`BenchmarkRangeFullTable`** — последовательный полный проход `Range`.

Сырые логи в `metrics/raw/benchmarks.txt`, агрегированная таблица — `metrics/raw/benchmarks.csv` (поле `*_per_op` уже нормализовано gnuplot-серии `series_*.tsv` генерируется в `collect`).

---

## 4. Результаты и графики

**Аппаратное окружение замеров данного отчёта:** Linux amd64 (Fedora), CPU см. строку `cpu:` бенча; Go toolchain соответствует `go.mod`.

### Таблица 4.1 — `ns/op` из `metrics/raw/benchmarks.csv` (prelude `BENCH_KEYS=4096,65536`)

| workload | impl | keys | ns/op |
|:---------|:-----|-----:|------:|
| ParallelGetHit | concmap | 4096 | 11.23 |
| ParallelGetHit | plain | 4096 | 43.78 |
| ParallelGetHit | concmap | 65536 | 25.88 |
| ParallelGetHit | plain | 65536 | 44.68 |
| ParallelPutOverwrite | concmap | 4096 | 14.43 |
| ParallelPutOverwrite | plain | 4096 | 148.10 |
| ParallelPutOverwrite | concmap | 65536 | 30.99 |
| ParallelPutOverwrite | plain | 65536 | 147.60 |
| ParallelMixedRW | concmap | 4096 | 24.30 |
| ParallelMixedRW | plain | 4096 | 58.97 |
| ParallelMixedRW | concmap | 65536 | 38.24 |
| ParallelMixedRW | plain | 65536 | 65.30 |
| RangeFullTable | concmap | 4096 | 32617 |
| RangeFullTable | plain | 4096 | 35282 |
| RangeFullTable | concmap | 65536 | 734148 |
| RangeFullTable | plain | 65536 | 779573 |

**Интерпретация:**

- При **конкуррентном чтении (`Get Hit`)** `concmap` в **3–4×** быстрее baseline: блокировки сегментированы, два ядра читают разные ключи почти без стыковки, тогда как `plain` упирается в глобальный `rwmutex`-шлюз (+ атомики встроенной `map` под капотом).
- При **перезаписи (`Put overwrite`)** выигрыш **~10×**: baseline сериализует даже столкновения ключей разных семейств, наш — только бакеты.
- **Mixed** сохраняет преимущество, но уже меньше: нагрузку размывают дорогие `Merge`/`Put` локально.
- **Range**: `concmap` выигрывает за счёт **короче держимых `RLock` на маленьком бакете** и отсутствия «одного большого захвата» на всю таблицу; при этом абсолютные `ns/op` огромные — обход тысяч/десятков тысяч цепочек.

#### Рисунок 4.1 — Parallel Get-hit (`metrics/plots/latency_parallel_get_hit.png`)

![Parallel get hit](./metrics/plots/latency_parallel_get_hit.png)

#### Рисунок 4.2 — Parallel Put overwrite (`latency_parallel_put_overwrite.png`)

![Parallel put overwrite](./metrics/plots/latency_parallel_put_overwrite.png)

#### Рисунок 4.3 — Parallel mixed RW (`latency_parallel_mixed_rw.png`)

![Parallel mixed RW](./metrics/plots/latency_parallel_mixed_rw.png)

#### Рисунок 4.4 — Range full (`latency_range_full_table.png`)

![Range full](./metrics/plots/latency_range_full_table.png)

---

## 5. Concurrency-тесты («аналог jcstress»)

Java-экосистемный **jcstress** в Go напрямую не дублируется, поэтому применены:

1. **`-race`** на всех модульных/стресс тестах (`make test-race`). Детектор гонок — основной официальный инструмент времени выполнения; он ловит нарушения отношений **happens-before** на памяти пользовательского уровня.
2. **`TestStressMergeAdditiveRace`** — стресс суммирования счётчиков через `Merge` + независимая последовательная модель суммирования ключей под `mutex` → сверка ожидаемых значений.
3. **`TestStressPlainVsConcNoPanic`** — комбинируется `Put`/`Get`/`Merge`/`Range`/`Clear` из десятков горутин, проверяя отсутствие блокировочных ошибок конструктора.

Дополнительно можно подключить **linearizability** (напр. checker уровня *porcupine*) — это выходит за минимально необходимый объём, но даёт jcstress-близкую модель упорядочивания операций.

---

## 6. Профилирование CPU и памяти

Замеры: параллельный `BenchmarkParallelGetHit/size_65536/{concmap|plain}`, CPU + heap профили через `-cpuprofile` / `-memprofile`.

```bash
make profile      # *.prof и top-тексты в metrics/profiles/; PNG + flame graph в metrics/plots/
```

Для каждого сценария генерируются:

- текстовые **`top`**: [`cpu_parallel_get_conc_top.txt`](metrics/profiles/cpu_parallel_get_conc_top.txt), [`mem_parallel_get_conc_mem_top.txt`](metrics/profiles/mem_parallel_get_conc_mem_top.txt) и симметрично для `plain`;
- **`go tool pprof -png`** — граф вызовов (нужен **graphviz**, `dot`);
- интерактивный **`/ui/flamegraph`** через `go tool pprof -http` + сохранённый HTML; при наличии headless Chromium/Chrome делается **скриншот PNG** тем же пайплайном, что в lab-2 (см. [`scripts/gen_flamegraphs.sh`](scripts/gen_flamegraphs.sh)).

### 6.1 CPU — параллельный Get

**Рисунок 6.1 — Flamegraph CPU, `concmap` (`flamegraph_cpu_parallel_get_conc.png`)**

![Flame CPU concmap](./metrics/plots/flamegraph_cpu_parallel_get_conc.png)

**Рисунок 6.2 — Flamegraph CPU, `plain` (`flamegraph_cpu_parallel_get_plain.png`)**

![Flame CPU plain](./metrics/plots/flamegraph_cpu_parallel_get_plain.png)

**Рисунок 6.3 — Call graph CPU (`go tool pprof -png`), `concmap`**

![pprof cpu conc](./metrics/plots/pprof_cpu_get_concmap.png)

**Рисунок 6.4 — Call graph CPU, `plain`**

![pprof cpu plain](./metrics/plots/pprof_cpu_get_plain.png)

| Компонента | flat (порядок) | Комментарий |
|:-----------|:---------------|:--------------|
| `concmap`: `memeqbody` | ~31% | сравнение ключей при обходе цепочки |
| `concmap`: `Map.Get` cum | ~96% | вся стоимость чтения в одном месте списка + короткий `RLock` |
| `plain`: `atomic.Int32.Add` | ~76% | счётчик `RLock`-ов делит линию с упором в общий замок |
| `plain`: `Plain.Get` cum | ~96% | встроенная `map` + глобальный `RWMutex` |

Интерактивный HTML flame graph: [`flamegraph_cpu_parallel_get_conc.html`](metrics/plots/flamegraph_cpu_parallel_get_conc.html), [`flamegraph_cpu_parallel_get_plain.html`](metrics/plots/flamegraph_cpu_parallel_get_plain.html).

### 6.2 Память — параллельный Get (`alloc_space`)

**Рисунок 6.5 — Flamegraph памяти, `concmap`**

![Flame mem conc](./metrics/plots/flamegraph_mem_parallel_get_conc.png)

**Рисунок 6.6 — Flamegraph памяти, `plain`**

![Flame mem plain](./metrics/plots/flamegraph_mem_parallel_get_plain.png)

**Рисунок 6.7 — Call graph heap (`go tool pprof -alloc_space -png`), `concmap`**

![pprof mem conc](./metrics/plots/pprof_mem_get_concmap.png)

**Рисунок 6.8 — Call graph heap, `plain`**

![pprof mem plain](./metrics/plots/pprof_mem_get_plain.png)

| Функция | concmap (~alloc_space) | plain (~alloc_space) | Интерпретация |
|:--------|----------------------:|---------------------:|:--------------|
| `Map.Put` / `Plain.Put` | ~10.5 MB (≈43%) | ~29.4 MB (≈71%) | бенч **предварительно заполняет** таблицу `Put`; у `Plain` дороже рост builtin-`map` |
| `makeKeys` | ~3.75 MB (~15%) | ~1.7 MB (~4%) | генерация строковых ключей для смеси операций (`fmt.Sprintf`-стиль нагрузки) |
| `runtime.allocm` | заметная доля | заметная доля | побочный эффект параллельного бенча и захвата heap-профиля |
| профилировщик / gzip | несколько процентов | несколько процентов | включён общий захват профилей; для «идеальных» долей брать узкий бенч и отдельный `-memprofile` только на `Get` |

**Вывод по памяти:** у **`Plain`** доминирует аллокация при прогреве (`Put`) на одной большой встроенной `map`; у **`concmap`** доля узловых вставок в цепочки ниже в этом смесительном профиле, но остаются накладные расходы `makeKeys` и рантайма. Для снижения шума в отчётных `alloc_space` разумно разнести **прогрев** и профилирование «чистого» `Get`, а для оптимизаций продакшена — пулировать буферы генерации ключей.

HTML: [`flamegraph_mem_parallel_get_conc.html`](metrics/plots/flamegraph_mem_parallel_get_conc.html), [`flamegraph_mem_parallel_get_plain.html`](metrics/plots/flamegraph_mem_parallel_get_plain.html).

---

## 7. Вывод

1. Сегментированная хеш-таблица с **закрытой адресацией и per-bucket `RWMutex`** даёт операциям чтения «локальность» блокировок: они не конкурируют с изменениями **других** бакетов, соблюдая при этом упорядочивание между завершёнными мутациями и последующими чтениями.
2. Baseline одиночной `sync.RWMutex` вокруг `map`, напротив, **блокирует всех при любой мутации**, что особенно проявляется в сценариях многопоточных `Put`/смешанных нагрузок.
3. Недостаток текущей реализации: **дорогая микро-хэш-проходка списков** под конкуррентными обновлениями (цепочка длиннее при плохом распределении); для production стоило бы добавить **динамический rehash**/контроль длины цепочек и **более дешёвый string hash**.
4. Concurrency качество верифицируется **`-race` + стохастический стресс**; для jcstress-подобного отчёта дополнительно оправдан внешний checker линеаризации.
