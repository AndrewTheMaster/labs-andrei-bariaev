// Command irquery — интерактивный стенд запросов по mmap-индексу (.irx).
//
// Usage:
//
//	irquery -index data/index.irx
//	irquery -index data/index.irx -q 'россия AND город'
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

func main() {
	indexPath := flag.String("index", "data/index.irx", "path to compressed mmap index")
	query := flag.String("q", "", "single query (non-interactive)")
	limit := flag.Int("limit", 20, "max doc IDs to print")
	flag.Parse()

	mi, err := ir.OpenMMapIndex(*indexPath)
	if err != nil {
		log.Fatalf("open index %s: %v", *indexPath, err)
	}
	defer mi.Close()

	fmt.Printf("index: %s  docs=%d  terms=%d (mmap)\n", *indexPath, mi.NumDocs(), mi.Terms())
	fmt.Println("язык: AND OR NOT ADJ(...) NEAR(k,...) FIRST(...) EDGE_END(...) MSM(...) — MSM только в RAM-индексе")
	fmt.Println("команды: :q :quit  :help")

	run := func(q string) error {
		q = strings.TrimSpace(q)
		if q == "" {
			return nil
		}
		t0 := time.Now()
		ids, _, err := ir.SearchBoolMMapWarnMSM(mi, q)
		elapsed := time.Since(t0)
		if err != nil {
			return err
		}
		n := len(ids)
		fmt.Printf("hits=%d  time=%s\n", n, elapsed.Round(time.Microsecond))
		show := n
		if show > *limit {
			show = *limit
		}
		if show > 0 {
			fmt.Print("docIDs:")
			for i := 0; i < show; i++ {
				fmt.Printf(" %d", ids[i])
			}
			if n > show {
				fmt.Printf(" ... (+%d)", n-show)
			}
			fmt.Println()
		}
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
			fmt.Println("пример: россия AND город | ADJ(россия, город) | NOT река")
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
