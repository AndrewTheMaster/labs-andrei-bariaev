# Визуализация бенчмарков lab-4 (gnuplot 5+).
# Переменные: raw_dir, plot_dir — из Makefile: make plot

reset
csv_dir = raw_dir

conc_c  = "#2563EB"
plain_c = "#DC2626"
unsafe_c = "#059669"
grid_c  = "#E5E7EB"

set terminal pngcairo size 1100, 620 enhanced font "Helvetica,12" dashed noenhanced
set border linewidth 1.4 lc rgb "#374151"
set grid back lt 1 lc rgb grid_c
set key outside right top maxrows 4 spacing 1.2
set datafile commentschars "#"
set style fill solid 0.85 border -1

series(bench, impl) = sprintf("%s/series_%s_%s.tsv", csv_dir, bench, impl)
speedup(bench) = sprintf("%s/speedup_%s.tsv", csv_dir, bench)

set style line 1 lc rgb conc_c  lw 2.8 pt 7  ps 1.35
set style line 2 lc rgb plain_c lw 2.8 pt 9  ps 1.35
set style line 3 lc rgb unsafe_c lw 2.6 pt 5 ps 1.2
set style line 4 lc rgb "#7C3AED" lw 2.6 pt 11 ps 1.2
set style line 5 lc rgb "#D97706" lw 2.6 pt 13 ps 1.2
set style line 6 lc rgb "#0891B2" lw 2.6 pt 15 ps 1.2

# ── 1. Parallel Get ───────────────────────────────────────────────
set output plot_dir . "/latency_parallel_get_hit.png"
set title "Parallel Get-hit" font ",16" tc rgb "#111827"
set xlabel "Предзаполнение (уникальных ключей)" font ",12"
set ylabel "ns/op" font ",12"
set logscale x 2
set format x "10^{%L}"
set xtics nomirror
set ytics nomirror
set key box
plot \
  series("BenchmarkParallelGetHit","concmap") using 1:2:xtic(1) with linespoints ls 1 title "concmap", \
  series("BenchmarkParallelGetHit","plain")   using 1:2:xtic(1) with linespoints ls 2 title "plain (RWMutex+map)"

# ── 2. Parallel Put ───────────────────────────────────────────────
set output plot_dir . "/latency_parallel_put_overwrite.png"
set title "Parallel Put (overwrite)" font ",16" tc rgb "#111827"
set xlabel "Предзаполнение (уникальных ключей)" font ",12"
set ylabel "ns/op" font ",12"
set logscale x 2
set format x "10^{%L}"
set xtics nomirror
set ytics nomirror
set key box
plot \
  series("BenchmarkParallelPutOverwrite","concmap") using 1:2:xtic(1) with linespoints ls 1 title "concmap", \
  series("BenchmarkParallelPutOverwrite","plain")   using 1:2:xtic(1) with linespoints ls 2 title "plain"

# ── 3. Mixed RW ───────────────────────────────────────────────────
set output plot_dir . "/latency_parallel_mixed_rw.png"
set title "Parallel mixed RW (Get : Put : Merge ≈ 1 : 1 : 1)" font ",16" tc rgb "#111827"
set xlabel "Предзаполнение (уникальных ключей)" font ",12"
set ylabel "ns/op" font ",12"
set logscale x 2
set format x "10^{%L}"
set xtics nomirror
set ytics nomirror
set key box
plot \
  series("BenchmarkParallelMixedRW","concmap") using 1:2:xtic(1) with linespoints ls 1 title "concmap", \
  series("BenchmarkParallelMixedRW","plain")   using 1:2:xtic(1) with linespoints ls 2 title "plain"

# ── 4. Range (log Y) ──────────────────────────────────────────────
set output plot_dir . "/latency_range_full_table.png"
set title "Range — полный обход таблицы" font ",16" tc rgb "#111827"
set xlabel "Предзаполнение (уникальных ключей)" font ",12"
set ylabel "ns/op (log)" font ",12"
set logscale x 2
set format x "10^{%L}"
set xtics nomirror
set ytics nomirror
set key box
set logscale y 10
plot \
  series("BenchmarkRangeFullTable","concmap") using 1:2:xtic(1) with linespoints ls 1 title "concmap", \
  series("BenchmarkRangeFullTable","plain")   using 1:2:xtic(1) with linespoints ls 2 title "plain"

# ── 5. Speedup ────────────────────────────────────────────────────
set output plot_dir . "/speedup_parallel.png"
set title "Ускорение: plain / concmap (×)" font ",16" tc rgb "#111827"
set xlabel "Предзаполнение (уникальных ключей)" font ",12"
set ylabel "×" font ",12"
set logscale x 2
set format x "10^{%L}"
set xtics nomirror
set ytics nomirror
set key box
unset logscale y
plot \
  speedup("BenchmarkParallelGetHit")       using 1:2:xtic(1) with linespoints ls 4 title "Get-hit", \
  speedup("BenchmarkParallelPutOverwrite") using 1:2:xtic(1) with linespoints ls 5 title "Put overwrite", \
  speedup("BenchmarkParallelMixedRW")      using 1:2:xtic(1) with linespoints ls 6 title "Mixed RW"

# ── 6. Bars @ 64k ─────────────────────────────────────────────────
set output plot_dir . "/bars_parallel_64k.png"
set title "Параллельные сценарии @ 65 536 ключей" font ",16" tc rgb "#111827"
set ylabel "ns/op" font ",12"
set style histogram clustered gap 1 title offset 0,0 font ",11"
set boxwidth 0.72
set grid y
unset logscale
unset logscale y
set xtics rotate by -20
plot csv_dir . "/bars_65536.tsv" using 2:xtic(1) ti "concmap" lc rgb conc_c, \
     '' using 3 ti "plain" lc rgb plain_c

# ── 7. Sequential Get ─────────────────────────────────────────────
set output plot_dir . "/latency_sequential_get.png"
set title "Однопоточный Get-hit" font ",16" tc rgb "#111827"
set xlabel "Предзаполнение (уникальных ключей)" font ",12"
set ylabel "ns/op" font ",12"
set logscale x 2
set format x "10^{%L}"
set xtics nomirror
set ytics nomirror
set key box
plot \
  series("BenchmarkSequentialGetHit","unsafe")  using 1:2:xtic(1) with linespoints ls 3 title "unsafe (map)", \
  series("BenchmarkSequentialGetHit","plain")   using 1:2:xtic(1) with linespoints ls 2 title "plain", \
  series("BenchmarkSequentialGetHit","concmap") using 1:2:xtic(1) with linespoints ls 1 title "concmap"

# ── 8. Dashboard 2×2 ───────────────────────────────────────────────
set output plot_dir . "/dashboard_parallel.png"
set multiplot layout 2,2 rowsfirst title "Сводка: concmap vs plain" font ",17" tc rgb "#111827"
set tmargin 2.5
set bmargin 3.2
set logscale x 2
set format x "10^{%L}"
set xtics nomirror
set ytics nomirror
set key box

set title "Get-hit" font ",13"
set ylabel "ns/op"
plot series("BenchmarkParallelGetHit","concmap") u 1:2:xtic(1) w lp ls 1 t "concmap", \
     series("BenchmarkParallelGetHit","plain")   u 1:2 w lp ls 2 t "plain"

set title "Put overwrite" font ",13"
plot series("BenchmarkParallelPutOverwrite","concmap") u 1:2:xtic(1) w lp ls 1 t "concmap", \
     series("BenchmarkParallelPutOverwrite","plain")   u 1:2 w lp ls 2 t "plain"

set title "Mixed RW" font ",13"
plot series("BenchmarkParallelMixedRW","concmap") u 1:2:xtic(1) w lp ls 1 t "concmap", \
     series("BenchmarkParallelMixedRW","plain")   u 1:2 w lp ls 2 t "plain"

set title "Speedup (×)" font ",13"
unset logscale y
plot speedup("BenchmarkParallelGetHit") u 1:2 w lp ls 4 t "Get", \
     speedup("BenchmarkParallelPutOverwrite") u 1:2 w lp ls 5 t "Put", \
     speedup("BenchmarkParallelMixedRW") u 1:2 w lp ls 6 t "Mixed"

unset multiplot
