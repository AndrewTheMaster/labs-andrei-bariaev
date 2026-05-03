# Lab 3 по ТЗ: SIFT (*.fvecs), ≥10 000 случайных запросов из корпуса, k=100, полная решётка.
# Отладочный синтетический режим без скачивания данных — make bench-quick

.PHONY: bench bench-quick plots profile artifacts all check-data

METRICS ?= metrics
LAB_ROOT := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))
PYTHON := $(LAB_ROOT)/.venv/bin/python

TZ_DATA ?= $(LAB_ROOT)/data/sift_base.fvecs
BENCH_SYNTH ?= 100000

# Графики: подпись с перечнем условий ТЗ
PLOT_TITLE ?= SIFT corpus; random nq=10000 queries from corpus; k=100; full grids (no --quick)

# Профиль: те же PROFILE_NQ и k, что в ТЗ-бенче; корпус — префикс SIFT первых PROFILE_MAX_VECTORS строк (0 = весь файл)
export PROFILE_DATA := $(TZ_DATA)
PROFILE_NQ ?= 10000
PROFILE_MAX_VECTORS ?= 200000
PROFILE_K ?= 100
PROFILE_SEED ?= 42
# 1 = не требовать sift_base перед profile (Gaussian fallback, не для отчётного ТЗ)
PROFILE_ALLOW_SYNTHETIC ?= 0
export PROFILE_NQ PROFILE_MAX_VECTORS PROFILE_K PROFILE_SEED PROFILE_ALLOW_SYNTHETIC

bench-quick:
	mkdir -p $(METRICS)/raw $(METRICS)/plots $(METRICS)/profiles
	$(PYTHON) run_benchmark.py --quick --synthetic $(BENCH_SYNTH) --nq 2000

check-data:
	@test -f "$(TZ_DATA)" || ( \
		echo "Ожидается файл корпуса: $(TZ_DATA)"; \
		echo "Загрузка: cd $(LAB_ROOT) && $(PYTHON) download_data.py"; \
		exit 1; )

bench: check-data
	mkdir -p $(METRICS)/raw $(METRICS)/plots $(METRICS)/profiles
	$(PYTHON) run_benchmark.py --data "$(TZ_DATA)" --nq 10000 --k 100 --max-vectors 0

plots:
	$(PYTHON) scripts/export_plots.py --title-suffix "$(PLOT_TITLE)"

profile:
	mkdir -p $(METRICS)/profiles $(METRICS)/plots $(METRICS)/raw
	@if [ "$(PROFILE_ALLOW_SYNTHETIC)" != "1" ]; then $(MAKE) check-data TZ_DATA="$(TZ_DATA)"; fi
	@echo "● PROFILE_ALLOW_SYNTHETIC=$(PROFILE_ALLOW_SYNTHETIC) PROFILE_DATA=$(PROFILE_DATA)"
	@echo "● PROFILE_NQ=$(PROFILE_NQ) PROFILE_MAX_VECTORS=$(PROFILE_MAX_VECTORS) PROFILE_K=$(PROFILE_K)"
	@echo "● полный хот-пас cProfile → vec_search.prof"
	$(PYTHON) -m cProfile -o $(METRICS)/profiles/vec_search.prof scripts/profile_hotpath.py
	@echo "● по фазам (HNSW/IVFPQ) + profile_phase_wall.tsv + profile_walltime_phases.png"
	$(PYTHON) scripts/profile_phases.py
	@echo "● топ cumulative / tottime + сетка + pie"
	$(PYTHON) scripts/pstats_top_png.py --prof $(METRICS)/profiles/vec_search.prof \
		--out $(METRICS)/plots/cpu_profile_python_top.png --metric cum
	$(PYTHON) scripts/pstats_top_png.py --prof $(METRICS)/profiles/vec_search.prof \
		--out $(METRICS)/plots/cpu_profile_python_tot.png --metric tot
	$(PYTHON) scripts/pstats_multi.py --grid-out $(METRICS)/plots/cpu_profile_phases_grid.png \
		--pie-out $(METRICS)/plots/cpu_profile_top_pie.png

# CI/ноутбук без SIFT: PROFILE_ALLOW_SYNTHETIC=1 make profile-smoke (малый nq задаётся в env)
.PHONY: profile-smoke
profile-smoke:
	mkdir -p $(METRICS)/profiles $(METRICS)/plots $(METRICS)/raw
	PROFILE_ALLOW_SYNTHETIC=1 PROFILE_NQ=800 PROFILE_MAX_VECTORS=50000 PROFILE_K=50 \
	  $(PYTHON) -m cProfile -o $(METRICS)/profiles/vec_search.prof scripts/profile_hotpath.py
	PROFILE_ALLOW_SYNTHETIC=1 PROFILE_NQ=800 PROFILE_MAX_VECTORS=50000 PROFILE_K=50 $(PYTHON) scripts/profile_phases.py
	$(PYTHON) scripts/pstats_top_png.py --prof $(METRICS)/profiles/vec_search.prof \
		--out $(METRICS)/plots/cpu_profile_python_top.png --metric cum
	$(PYTHON) scripts/pstats_top_png.py --prof $(METRICS)/profiles/vec_search.prof \
		--out $(METRICS)/plots/cpu_profile_python_tot.png --metric tot
	$(PYTHON) scripts/pstats_multi.py --grid-out $(METRICS)/plots/cpu_profile_phases_grid.png \
		--pie-out $(METRICS)/plots/cpu_profile_top_pie.png

artifacts: bench plots profile

all: artifacts
