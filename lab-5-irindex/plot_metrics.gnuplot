csv_dir = raw_dir

reset
set terminal pngcairo size 960, 560 enhanced font "Arial,11"
set key outside right top
set grid
set border linewidth 1.2
set logscale x 2
set xlabel "Число документов в корпусе"

series(b,m) = sprintf('%s/series_%s_%s.tsv', csv_dir, b, m)

# ───────── Построение индекса ─────────
set output plot_dir . "/build_index_ns.png"
set title "Индексация: построение обратного индекса (Add-документы)"
set ylabel "ns/op (полный корпус за итерацию)"
unset logscale y
plot series("BenchmarkBuildIndex","build") using 1:2 with linespoints lw 2 pt 7 lc rgb "#377eb8" title "BuildIndex"

# ───────── Запрос: индекс vs полный скан ─────────
set output plot_dir . "/query_idx_vs_scan.png"
set title "Смешанный булев запрос (+ MSM + FIRST): inverted index vs brute SlowEval"
set ylabel "ns/op"
plot series("BenchmarkQueryEvalMixed","idx") using 1:2 with linespoints lw 2 pt 7 lc rgb "#4daf4a" title "Eval (инверсный индекс)", \
     series("BenchmarkQueryEvalMixed","scan") using 1:2 with linespoints lw 2 pt 9 lc rgb "#e41a1c" title "SlowEval (скан всех текстов)"

# ───────── Операторы (синт., idx) ─────────
set output plot_dir . "/ops_idx_ns.png"
set title "BenchmarkOp: отдельные операторы (inverted index, синтетика)"
set ylabel "ns/op"
plot series("BenchmarkOp","AND_idx") using 1:2 with linespoints lw 2 pt 7 title "AND", \
     series("BenchmarkOp","OR_idx") using 1:2 with linespoints lw 2 pt 7 title "OR", \
     series("BenchmarkOp","NOT_idx") using 1:2 with linespoints lw 2 pt 7 title "NOT", \
     series("BenchmarkOp","ADJ_idx") using 1:2 with linespoints lw 2 pt 7 title "ADJ", \
     series("BenchmarkOp","NEAR_idx") using 1:2 with linespoints lw 2 pt 7 title "NEAR"

# ───────── Ruwiki: AND idx vs scan (реальный корпус) ─────────
wiki_and_idx = csv_dir . "/series_wiki_AND_idx.tsv"
wiki_and_scan = csv_dir . "/series_wiki_AND_scan.tsv"
set output plot_dir . "/wiki_op_AND_idx_vs_scan.png"
set title "Ruwiki: «россия AND москва» — idx vs scan"
set ylabel "ns/op"
plot wiki_and_idx using 1:2 with linespoints lw 2 pt 7 lc rgb "#4daf4a" title "idx", \
     wiki_and_scan using 1:2 with linespoints lw 2 pt 9 lc rgb "#e41a1c" title "scan"

# ───────── Ruwiki: операторы idx (bar, N=20k) ─────────
load csv_dir . "/wiki_plot_queries.gnuplot"
wiki_ops = csv_dir . "/wiki_ops_idx_ms.tsv"
set output plot_dir . "/wiki_ops_idx_bar.png"
set terminal pngcairo size 1400, 760 enhanced font "Arial,9"
set title "Ruwiki BenchmarkOp (idx): ns/op, N=20000"
set xlabel "ns/op"
set ylabel ""
unset logscale x
unset logscale y
set datafile separator "\t"
set style fill solid 0.6 border lt -1
set boxwidth 0.65 relative
set ytics offset graph 0, 0 font ",8"
set lmargin 42
set rmargin 4
set tmargin 2
set bmargin 4
plot wiki_ops using 2:0:ytic(1) with boxes lc rgb "#377eb8" notitle
unset datafile separator

# ───────── Ruwiki: операторы idx — рост по корпусу ─────────
set output plot_dir . "/wiki_ops_idx_scale.png"
set terminal pngcairo size 1280, 640 enhanced font "Arial,10"
set title "Ruwiki BenchmarkOp (idx): масштабирование"
set xlabel "Число документов (ruwiki)"
set ylabel "µs/op"
set logscale x 2
unset logscale y
set key outside right top font ",9"
set margins 6, 14, 3, 3
plot csv_dir.'/series_wiki_ADJ_idx.tsv' using 1:($2/1e3) with linespoints lw 2 title sprintf("%s", wiki_q_ADJ), \
     csv_dir.'/series_wiki_NEAR_idx.tsv' using 1:($2/1e3) with linespoints lw 2 title sprintf("%s", wiki_q_NEAR), \
     csv_dir.'/series_wiki_AND_idx.tsv' using 1:($2/1e3) with linespoints lw 2 title sprintf("%s", wiki_q_AND), \
     csv_dir.'/series_wiki_MSM_idx.tsv' using 1:($2/1e3) with linespoints lw 2 title sprintf("%s", wiki_q_MSM), \
     csv_dir.'/series_wiki_OR_idx.tsv' using 1:($2/1e3) with linespoints lw 2 title sprintf("%s", wiki_q_OR), \
     csv_dir.'/series_wiki_NOT_idx.tsv' using 1:($2/1e3) with linespoints lw 2 title sprintf("%s", wiki_q_NOT), \
     csv_dir.'/series_wiki_Complex_idx.tsv' using 1:($2/1e3) with linespoints lw 2 title sprintf("%s", wiki_q_Complex)
