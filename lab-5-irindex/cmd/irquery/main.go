package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"siaod-hw5-irindex/internal/ir"
)

func printHits(lines []ir.HitLine, limit int) {
	n := len(lines)
	show := n
	if show > limit {
		show = limit
	}
	for i := 0; i < show; i++ {
		h := lines[i]
		if h.HasScore {
			fmt.Printf("  [%d] doc=%d  score=%.4f  «%s»\n", i+1, h.DocID, h.Score, h.Title)
		} else {
			fmt.Printf("  [%d] doc=%d  «%s»\n", i+1, h.DocID, h.Title)
		}
		if h.Matched != "" {
			fmt.Printf("       terms: %s  (total tf=%d)\n", h.Matched, h.TotalTF)
		}
	}
	if n > show {
		fmt.Printf("  ... (+%d)\n", n-show)
	}
}

func main() {
	indexPath := flag.String("index", "data/index.irx", "path to compressed mmap index")
	query := flag.String("q", "", "single query (non-interactive)")
	limit := flag.Int("limit", 20, "max results to print")
	rank := flag.Bool("rank", false, "BM25 ranking (default: bool filter only)")
	k1 := flag.Float64("k1", 1.2, "BM25 k1")
	bParam := flag.Float64("b", 0.75, "BM25 b")
	flag.Parse()

	mi, err := ir.OpenMMapIndex(*indexPath)
	if err != nil {
		log.Fatalf("open index %s: %v (пересоберите: go run ./cmd/irindex …)", *indexPath, err)
	}
	defer mi.Close()

	fmt.Printf("index: %s  docs=%d  terms=%d (mmap IRIXV3PD)\n", *indexPath, mi.NumDocs(), mi.Terms())
	if *rank {
		fmt.Printf("mode: BM25 (k1=%.2f b=%.2f)\n", *k1, *bParam)
	}
	fmt.Println("язык: AND OR NOT(...) ADJ(...) NEAR(k,...) FIRST(...) — MSM только RAM")
	fmt.Println("команды: :q :quit  :help  :rank on|off")

	run := func(q string) error {
		q = strings.TrimSpace(q)
		if q == "" {
			return nil
		}
		if strings.HasPrefix(strings.ToLower(q), ":rank") {
			parts := strings.Fields(q)
			if len(parts) == 2 {
				switch strings.ToLower(parts[1]) {
				case "on", "1", "true":
					*rank = true
					fmt.Println("BM25 ranking: on")
				case "off", "0", "false":
					*rank = false
					fmt.Println("BM25 ranking: off")
				default:
					fmt.Println("usage: :rank on|off")
				}
			} else {
				fmt.Printf("BM25 ranking: %v\n", *rank)
			}
			return nil
		}
		t0 := time.Now()
		if *rank {
			scored, lines, err := ir.SearchBM25MMapDetailed(mi, q, *k1, *bParam)
			elapsed := time.Since(t0)
			if err != nil {
				return err
			}
			fmt.Printf("hits=%d  time=%s\n", len(scored), elapsed.Round(time.Microsecond))
			printHits(lines, *limit)
			return nil
		}
		ids, lines, err := ir.SearchBoolMMapDetailed(mi, q)
		elapsed := time.Since(t0)
		if err != nil {
			return err
		}
		fmt.Printf("hits=%d  time=%s\n", len(ids), elapsed.Round(time.Microsecond))
		printHits(lines, *limit)
		return nil
	}

	if *query != "" {
		if err := run(*query); err != nil {
			log.Fatal(err)
		}
		return
	}

	sc := bufio.NewScanner(os.Stdin)
	fmt.Print("> ")
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		switch strings.ToLower(line) {
		case "", ":help", "help":
			fmt.Println(`пример: история AND NOT(россии AND китая)
  ADJ(россия, город)  |  :rank on`)
		case ":q", ":quit", "quit", "exit":
			return
		default:
			if err := run(line); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
			}
		}
		fmt.Print("> ")
	}
	if err := sc.Err(); err != nil {
		log.Fatal(err)
	}
}
