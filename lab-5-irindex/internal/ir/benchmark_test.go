package ir

import (
	"fmt"
	"math/rand/v2"
	"os"
	"strconv"
	"strings"
	"testing"
)

func corpusSizes(tb testing.TB) []int {
	raw := strings.TrimSpace(os.Getenv("BENCH_CORPUS"))
	if raw == "" {
		return []int{400, 2000}
	}
	var out []int
	for _, p := range strings.Split(raw, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		n, err := strconv.Atoi(p)
		if err != nil || n <= 0 {
			tb.Fatalf("bad BENCH_CORPUS part %q", p)
		}
		out = append(out, n)
	}
	if len(out) == 0 {
		tb.Fatal("empty BENCH_CORPUS")
	}
	return out
}

type vocab []string

func defaultWords() vocab {
	w := strings.Fields("alpha beta gamma delta epsilon zeta omega cat dog mouse tiger lion quantum field iris node")
	out := make(vocab, len(w))
	copy(out, w)
	return out
}

func fillCorpus(nDocs int, seed uint64, words vocab) *InvIndex {
	ix := NewIndex()
	src := rand.NewPCG(seed, seed^909)
	for i := 0; i < nDocs; i++ {
		buf := strings.Builder{}
		for buf.Len() < 96 {
			if buf.Len() > 0 {
				buf.WriteByte(' ')
			}
			buf.WriteString(words[src.Uint64()%uint64(len(words))])
		}
		ix.Add(Tokenize(buf.String()))
	}
	return ix
}

func BenchmarkBuildIndex(b *testing.B) {
	words := defaultWords()
	for _, n := range corpusSizes(b) {
		// Имя подбенча без «docs_»: Go приклеивает -GOMAXPROCS к последнему сегменту.
		b.Run(fmt.Sprintf("corp%d", n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				fillCorpus(n, 777, words)
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
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = Eval(ix, ast)
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
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = Eval(ix, adjQ)
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
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = Eval(ix, nearQ)
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
