BENCH_KEYS   ?= 4096,65536
BENCH_TIME   ?= 600ms
BENCH_FILTER ?= ^Benchmark
METRICS_OUT  ?= metrics
RAW_DIR      := $(METRICS_OUT)/raw
PLOT_DIR     := $(METRICS_OUT)/plots
PROF_DIR     := $(METRICS_OUT)/profiles

.PHONY: test test-race bench collect metrics plot profile clean

test:
	go test ./...

test-race:
	go test ./internal/concmap -race -count=3

bench:
	BENCH_KEYS='$(BENCH_KEYS)' go test ./internal/concmap -run='^$$' -bench '$(BENCH_FILTER)' -benchmem -benchtime='$(BENCH_TIME)'

collect:
	mkdir -p $(RAW_DIR) $(PLOT_DIR)
	BENCH_KEYS='$(BENCH_KEYS)' go test ./internal/concmap -run='^$$' -bench '$(BENCH_FILTER)' -benchmem -benchtime='$(BENCH_TIME)' \
		> $(RAW_DIR)/benchmarks.txt
	awk 'BEGIN { OFS=","; print "bench,impl,size,iters,ns_per_op,bytes_per_op" } \
	/^Benchmark/ { \
	  name=$$1; \
	  split(name, a, "/"); \
	  if (length(a) < 3) next; \
	  bench=a[1]; \
	  sztok=a[2]; sub(/^size_/, "", sztok); \
	  implcpu=a[3]; \
	  split(implcpu, b, "-"); impl=b[1]; \
	  iters=$$2; \
	  ns=0; bpb=0; \
	  for (i=2; i<=NF; i++) { \
	    if ($$i == "ns/op") ns=$$(i-1); \
	    if ($$i == "B/op") bpb=$$(i-1); \
	  } \
	  print bench,impl,sztok,iters,ns,bpb; \
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
	BENCH_KEYS='65536' BENCH_BUCKET_BITS='12' go test ./internal/concmap -run='^$$' \
		-bench '^BenchmarkParallelGetHit/size_65536/concmap$$' -benchmem -benchtime='800ms' \
		-cpuprofile=$(PROF_DIR)/cpu_parallel_get_conc.prof \
		-memprofile=$(PROF_DIR)/mem_parallel_get_conc.prof \
		>/dev/null
	BENCH_KEYS='65536' BENCH_BUCKET_BITS='12' go test ./internal/concmap -run='^$$' \
		-bench '^BenchmarkParallelGetHit/size_65536/plain$$' -benchmem -benchtime='800ms' \
		-cpuprofile=$(PROF_DIR)/cpu_parallel_get_plain.prof \
		-memprofile=$(PROF_DIR)/mem_parallel_get_plain.prof \
		>/dev/null
	go tool pprof -top $(PROF_DIR)/cpu_parallel_get_conc.prof   >$(PROF_DIR)/cpu_parallel_get_conc_top.txt
	go tool pprof -top $(PROF_DIR)/cpu_parallel_get_plain.prof >$(PROF_DIR)/cpu_parallel_get_plain_top.txt
	go tool pprof -alloc_space -top $(PROF_DIR)/mem_parallel_get_conc.prof  >$(PROF_DIR)/mem_parallel_get_conc_mem_top.txt
	go tool pprof -alloc_space -top $(PROF_DIR)/mem_parallel_get_plain.prof >$(PROF_DIR)/mem_parallel_get_plain_mem_top.txt
	@if command -v dot >/dev/null 2>&1; then \
		go tool pprof -png $(PROF_DIR)/cpu_parallel_get_conc.prof  >$(PLOT_DIR)/pprof_cpu_get_concmap.png 2>/dev/null; \
		go tool pprof -png $(PROF_DIR)/cpu_parallel_get_plain.prof >$(PLOT_DIR)/pprof_cpu_get_plain.png 2>/dev/null; \
		go tool pprof -alloc_space -png $(PROF_DIR)/mem_parallel_get_conc.prof  >$(PLOT_DIR)/pprof_mem_get_concmap.png 2>/dev/null; \
		go tool pprof -alloc_space -png $(PROF_DIR)/mem_parallel_get_plain.prof >$(PLOT_DIR)/pprof_mem_get_plain.png 2>/dev/null; \
	else \
		echo "graphviz(dot) отсутствует — см. текстовые top и flamegraph *.html в $(PLOT_DIR)"; \
	fi
	bash scripts/gen_flamegraphs.sh $(PROF_DIR) $(PLOT_DIR)

clean:
	rm -rf $(METRICS_OUT)
