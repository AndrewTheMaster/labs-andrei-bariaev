BENCH_CORPUS ?= 400,2000
BENCH_TIME   ?= 500ms
BENCH_FILTER ?= ^Benchmark(BuildIndex|Query|Op)
METRICS_OUT  ?= metrics
RAW_DIR      := $(METRICS_OUT)/raw
PLOT_DIR     := $(METRICS_OUT)/plots
PROF_DIR     := $(METRICS_OUT)/profiles
WIKI_XML     ?= ../ruwiki-latest-pages-articles.xml

.PHONY: test bench collect metrics plot profile build-index build query clean

build:
	go build -o bin/irindex ./cmd/irindex
	go build -o bin/irquery ./cmd/irquery

query:
	@test -f data/index.irx || $(MAKE) build-index
	./bin/irquery -index data/index.irx

test:
	go test ./...

bench:
	BENCH_CORPUS='$(BENCH_CORPUS)' go test ./internal/ir -run='^$$' -bench '$(BENCH_FILTER)' -benchmem -benchtime='$(BENCH_TIME)'

# Полный прогон на ruwiki (после bunzip2): make bench-wiki BENCH_CORPUS=50000,100000 BENCH_TIME=2s
bench-wiki:
	CORPUS_XML='$(WIKI_XML)' BENCH_CORPUS='$(BENCH_CORPUS)' $(MAKE) bench

# Сборка индекса + размеры до/после сжатия (цель ~3s на полном дампе — проверить после скачивания)
build-index:
	go run ./cmd/irindex -xml '$(WIKI_XML)' -out data/index.irx

collect:
	mkdir -p $(RAW_DIR) $(PLOT_DIR)
	BENCH_CORPUS='$(BENCH_CORPUS)' go test ./internal/ir -run='^$$' -bench '$(BENCH_FILTER)' -benchmem -benchtime='$(BENCH_TIME)' \
		> $(RAW_DIR)/benchmarks.txt
	awk 'BEGIN { OFS=","; print "bench,mode,corpus,iters,ns_per_op,bytes_per_op" } \
	/^Benchmark/ { \
	  fname=$$1; \
	  sub(/-[0-9]+$$/, "", fname); \
	  n=split(fname, a, "/"); \
	  if (n < 2) next; \
	  bench=a[1]; tail=a[2]; \
	  mode=""; corpus=0; \
	  if (bench == "BenchmarkBuildIndex") { \
	    sub(/^corp/, "", tail); corpus=tail+0; mode="build"; \
	  } else if (bench == "BenchmarkQueryEvalMixed") { \
	    split(tail, u, "_"); mode=u[1]; corpus=u[2]+0; \
	  } else if (bench == "BenchmarkQueryAdjNear") { \
	    split(tail, u, "_"); \
	    if (length(u) < 3) next; \
	    mode=u[1] "_" u[2]; corpus=u[3]+0; \
	  } else if (bench == "BenchmarkOp") { \
	    if (n < 4) next; \
	    mode=a[2] "_" a[3]; sub(/^corp/, "", a[4]); corpus=a[4]+0; \
	  } else next; \
	  iters=$$2; ns=0; bpb=0; \
	  for (i=2; i<=NF; i++) { \
	    if ($$i == "ns/op") ns=$$(i-1); \
	    if ($$i == "B/op") bpb=$$(i-1); \
	  } \
	  print bench,mode,corpus,iters,ns,bpb; \
	}' $(RAW_DIR)/benchmarks.txt > $(RAW_DIR)/benchmarks.csv
	rm -f $(RAW_DIR)/series_*.tsv
	awk -F',' \
	  -v RAW="$(RAW_DIR)" \
	  'BEGIN{OFS="\t"} \
	   NR==1 || $$1=="bench" { next } \
	   { fname = RAW "/series_" $$1 "_" $$2 ".tsv"; print $$3+0, $$5+0 >> fname }' \
	  $(RAW_DIR)/benchmarks.csv
	for f in $(RAW_DIR)/series_*.tsv; do \
	  [ -f "$$f" ] && sort -n -o "$$f" "$$f" || true; \
	done

metrics: collect plot

plot:
	mkdir -p $(PLOT_DIR)
	gnuplot -e "raw_dir='$(RAW_DIR)'; plot_dir='$(PLOT_DIR)'" plot_metrics.gnuplot

profile:
	mkdir -p $(PROF_DIR) $(PLOT_DIR)
	BENCH_CORPUS='2000' go test ./internal/ir -run='^$$' \
		-bench '^BenchmarkQueryEvalMixed/idx_2000$$' -benchmem -benchtime='900ms' \
		-cpuprofile=$(PROF_DIR)/cpu_query_idx.prof \
		-memprofile=$(PROF_DIR)/mem_query_idx.prof >/dev/null
	BENCH_CORPUS='2000' go test ./internal/ir -run='^$$' \
		-bench '^BenchmarkBuildIndex/corp2000$$' -benchmem -benchtime='900ms' \
		-cpuprofile=$(PROF_DIR)/cpu_build_index.prof \
		-memprofile=$(PROF_DIR)/mem_build_index.prof >/dev/null
	go tool pprof -top $(PROF_DIR)/cpu_query_idx.prof >$(PROF_DIR)/cpu_query_idx_top.txt
	go tool pprof -alloc_space -top $(PROF_DIR)/mem_query_idx.prof >$(PROF_DIR)/mem_query_idx_top.txt
	go tool pprof -top $(PROF_DIR)/cpu_build_index.prof >$(PROF_DIR)/cpu_build_index_top.txt
	go tool pprof -alloc_space -top $(PROF_DIR)/mem_build_index.prof >$(PROF_DIR)/mem_build_index_top.txt
	@if command -v dot >/dev/null 2>&1; then \
		go tool pprof -png $(PROF_DIR)/cpu_query_idx.prof >$(PLOT_DIR)/pprof_cpu_query_idx.png 2>/dev/null; \
		go tool pprof -alloc_space -png $(PROF_DIR)/mem_query_idx.prof >$(PLOT_DIR)/pprof_mem_query_idx.png 2>/dev/null; \
		go tool pprof -png $(PROF_DIR)/cpu_build_index.prof >$(PLOT_DIR)/pprof_cpu_build_index.png 2>/dev/null; \
		go tool pprof -alloc_space -png $(PROF_DIR)/mem_build_index.prof >$(PLOT_DIR)/pprof_mem_build_index.png 2>/dev/null; \
		echo "dot: call-graph PNG сохранены в $(PLOT_DIR)"; \
	else \
		echo "нет graphviz (dot) — только top-тексты и flamegraph HTML при запуске скрипта"; \
	fi
	bash scripts/gen_flamegraphs.sh $(PROF_DIR) $(PLOT_DIR)

clean:
	rm -rf $(METRICS_OUT)
