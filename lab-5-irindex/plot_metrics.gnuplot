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
set title "Ruwiki: AND (россия AND город) — idx vs scan"
set ylabel "ns/op"
plot wiki_and_idx using 1:2 with linespoints lw 2 pt 7 lc rgb "#4daf4a" title "idx", \
     wiki_and_scan using 1:2 with linespoints lw 2 pt 9 lc rgb "#e41a1c" title "scan"
