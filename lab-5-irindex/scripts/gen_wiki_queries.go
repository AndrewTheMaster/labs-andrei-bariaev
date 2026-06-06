//go:build ignore

package main

import (
	"fmt"
	"os"

	"siaod-hw5-irindex/internal/ir"
)

func main() {
	q := ir.RuWikiBenchQueries()
	ops := []struct {
		name string
		q    string
	}{
		{"AND", q.AND},
		{"OR", q.OR},
		{"NOT", q.NOT},
		{"ADJ", q.ADJ},
		{"NEAR", q.NEAR},
		{"EDGE", q.EDGE},
		{"Complex", q.Complex},
		{"MSM", q.MSM},
	}
	for _, op := range ops {
		fmt.Fprintf(os.Stdout, "%s\t%s\n", op.name, op.q)
	}
}
