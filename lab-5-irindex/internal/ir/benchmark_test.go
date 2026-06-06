package ir

import (
	"fmt"
	"testing"
)

func BenchmarkBuildIndex(b *testing.B) {
	for _, n := range corpusSizes(b) {
		b.Run(fmt.Sprintf("corp%d", n), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = fillCorpus(n, 777, defaultWords())
			}
		})
	}
}

func BenchmarkQueryEvalMixed(b *testing.B) {
	words := defaultWords()
	query := `(alpha AND beta) OR MSM(40, gamma, omega) AND NOT FIRST(delta)`
	ast, err := Parse(query)
	if err != nil {
		b.Fatal(err)
	}
	for _, n := range corpusSizes(b) {
		b.Run(fmt.Sprintf("idx_%d", n), func(b *testing.B) {
			ix := fillCorpus(n, 4242, words)
			ctx := NewEvalCtx(ix)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ctx.Reset()
				_ = ctx.Eval(ast)
			}
		})
		b.Run(fmt.Sprintf("scan_%d", n), func(b *testing.B) {
			ix := fillCorpus(n, 4242, words)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = SlowEval(ix, ast)
			}
		})
	}
}

func BenchmarkQueryAdjNear(b *testing.B) {
	words := defaultWords()
	adjQ, err := Parse(`ADJ(alpha, beta) AND NOT EDGE_END(delta)`)
	if err != nil {
		b.Fatal(err)
	}
	nearQ, err := Parse(`NEAR(3, alpha, gamma) OR ADJ(gamma, omega)`)
	if err != nil {
		b.Fatal(err)
	}
	for _, n := range corpusSizes(b) {
		ix := fillCorpus(n, 5150, words)
		b.Run(fmt.Sprintf("idx_adj_%d", n), func(b *testing.B) {
			ctx := NewEvalCtx(ix)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ctx.Reset()
				_ = ctx.Eval(adjQ)
			}
		})
		b.Run(fmt.Sprintf("scan_adj_%d", n), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = SlowEval(ix, adjQ)
			}
		})
		b.Run(fmt.Sprintf("idx_near_%d", n), func(b *testing.B) {
			ctx := NewEvalCtx(ix)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ctx.Reset()
				_ = ctx.Eval(nearQ)
			}
		})
		b.Run(fmt.Sprintf("scan_near_%d", n), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = SlowEval(ix, nearQ)
			}
		})
	}
}

func BenchmarkOp(b *testing.B) {
	q := defaultBenchQueries()
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
	for _, n := range corpusSizes(b) {
		ix := buildCorpusN(b, n)
		for _, op := range ops {
			ast, err := Parse(op.q)
			if err != nil {
				b.Fatalf("%s: %v", op.name, err)
			}
			b.Run(fmt.Sprintf("%s/idx/corp%d", op.name, n), func(b *testing.B) {
				ctx := NewEvalCtx(ix)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					ctx.Reset()
					_ = ctx.Eval(ast)
				}
			})
			b.Run(fmt.Sprintf("%s/scan/corp%d", op.name, n), func(b *testing.B) {
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					_ = SlowEval(ix, ast)
				}
			})
		}
	}
}
