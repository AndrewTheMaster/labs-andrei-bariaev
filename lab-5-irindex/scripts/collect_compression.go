//go:build ignore

package main

import (
	"log"
	"os"

	"siaod-hw5-irindex/internal/ir"
)

func main() {
	out := "metrics/raw/compression_sizes.tsv"
	if len(os.Args) > 1 {
		out = os.Args[1]
	}
	if err := os.MkdirAll("metrics/raw", 0o755); err != nil {
		log.Fatal(err)
	}
	if err := ir.WriteCompressionMetrics(out); err != nil {
		log.Fatal(err)
	}
}
