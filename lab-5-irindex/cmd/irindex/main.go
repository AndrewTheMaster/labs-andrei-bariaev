// Command irindex строит сжатый индекс и печатает размеры до/после сжатия.
//
// Usage:
//
//	irindex -xml data/ruwiki-latest-pages-articles.xml [-maxdocs 0] [-out index.irx]
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"siaod-hw5-irindex/internal/ir"
)

func main() {
	xmlPath := flag.String("xml", ir.ResolveCorpusPath(), "path to unpacked ruwiki XML")
	maxDocs := flag.Int("maxdocs", 0, "limit documents (0 = all)")
	outPath := flag.String("out", "data/index.irx", "compressed index output")
	flag.Parse()

	if _, err := os.Stat(*xmlPath); err != nil {
		log.Fatalf("XML not found (%s): %v — распакуйте ruwiki-latest-pages-articles.xml.bz2 в data/", *xmlPath, err)
	}

	t0 := time.Now()
	ix, st, err := ir.BuildIndexFromWikiXML(*xmlPath, ir.CorpusOpts{MaxDocs: *maxDocs})
	if err != nil {
		log.Fatal(err)
	}
	elapsed := time.Since(t0)

	if err := os.MkdirAll(filepath.Dir(*outPath), 0o755); err != nil && !os.IsExist(err) {
		log.Fatal(err)
	}
	report, err := ir.MeasureIndexSizes(ix, *outPath)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("=== index build ===")
	fmt.Printf("pages seen:     %d\n", st.PagesSeen)
	fmt.Printf("pages indexed:  %d\n", st.PagesIndexed)
	fmt.Printf("build time:     %s\n", elapsed.Round(time.Millisecond))
	fmt.Println("=== sizes ===")
	fmt.Printf("docs:           %d\n", report.Docs)
	fmt.Printf("terms:          %d\n", report.Terms)
	fmt.Printf("postings lists: %d\n", report.PostingsLists)
	fmt.Printf("raw RAM est:    %.0f KB\n", float64(report.RawBytes)/1024)
	fmt.Printf("compressed:     %.0f KB (%s)\n", float64(report.CompressedBytes)/1024, report.CompressedPath)
	if report.CompressedBytes > 0 {
		fmt.Printf("ratio raw/comp: %.2fx\n", report.Ratio)
	}
}
