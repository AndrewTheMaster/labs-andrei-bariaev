package ir

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"testing"
)

func buildCorpus(ix *InvIndex, lines []string) {
	for _, ln := range lines {
		ix.Add(Tokenize(ln))
	}
}

func TestNearAndNotEdge(t *testing.T) {
	ix := NewIndex()
	buildCorpus(ix, []string{
		"alpha beta gamma omega",
		"zeta eta beta tails",
		"only lonely",
		"trail marker omega",
	})
	q := `NEAR ( 2 , beta , omega ) AND NOT FIRST(only)`
	n, err := Parse(q)
	if err != nil {
		t.Fatal(err)
	}
	fast := Eval(ix, n)
	slow := SlowEval(ix, n)
	if fmt.Sprint(fast) != fmt.Sprint(slow) {
		t.Fatalf("index vs scan:\n%+v\n%+v", fast, slow)
	}
	if len(fast) != 1 || !contains(fast, 0) {
		t.Fatalf("expected exactly doc 0 hit, got %#v", fast)
	}
}

func contains(ms MatchSet, id uint32) bool {
	_, ok := ms[id]
	return ok
}

func TestMSMWINDOW(t *testing.T) {
	ix := NewIndex()
	buildCorpus(ix, []string{"a b c"})
	n, err := Parse(`MSM(3, a, b, c)`)
	if err != nil {
		t.Fatal(err)
	}
	ms := Eval(ix, n)
	if len(ms) != 1 || !contains(ms, 0) {
		t.Fatalf("msm want doc0 got %#v", ms)
	}
}

func TestEdgeStartEndAliasesMatchFirstLast(t *testing.T) {
	ix := NewIndex()
	buildCorpus(ix, []string{"hello planet", "solo hello"})
	fs, err := Parse(`FIRST(hello)`)
	if err != nil {
		t.Fatal(err)
	}
	es, err := Parse(`EDGE_START(hello)`)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(Eval(ix, fs)) != fmt.Sprint(Eval(ix, es)) {
		t.Fatalf("FIRST vs EDGE_START")
	}
	le, err := Parse(`LAST(planet)`)
	if err != nil {
		t.Fatal(err)
	}
	ee, err := Parse(`edge_end(planet)`)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(Eval(ix, le)) != fmt.Sprint(Eval(ix, ee)) {
		t.Fatalf("LAST vs edge_end")
	}
}

func TestFirstLastBoundary(t *testing.T) {
	ix := NewIndex()
	buildCorpus(ix, []string{"hello planet", "hello endtoken"})
	h, err := Parse(`FIRST(hello)`)
	if err != nil {
		t.Fatal(err)
	}
	if len(Eval(ix, h)) != 2 {
		t.Fatal("FIRST(hello) both docs")
	}
	h2, err := Parse(`LAST(planet)`)
	if err != nil {
		t.Fatal(err)
	}
	e := Eval(ix, h2)
	if len(e) != 1 || !contains(e, 0) {
		t.Fatalf("LAST planet doc0 %#v", e)
	}
}

func TestSlowVsIndexRandomQueries(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	const trials = 200
	r := rand.New(rand.NewPCG(1, 2))
	sw := func() []string {
		w := []string{"alpha", "beta", "gamma", "near", "edge", "term"}
		buf := strings.Builder{}
		nw := int(r.Uint64()%4) + 2
		for i := 0; i < nw; i++ {
			if i > 0 {
				buf.WriteByte(' ')
			}
			buf.WriteString(w[r.Uint64()%uint64(len(w))])
		}
		return []string{buf.String()}
	}
	for range trials {
		ix := NewIndex()
		for d := range 25 {
			_ = d
			ns := sw()
			buildCorpus(ix, ns)
		}
		queries := []string{
			`alpha AND beta`,
			`alpha OR gamma`,
			`NOT alpha`,
			`MSM(10, alpha, beta)`,
			`NEAR(5, alpha, beta)`,
			`FIRST(alpha)`,
			`LAST(beta)`,
			`(alpha OR beta) AND NOT gamma`,
		}
		for _, qq := range queries {
			ast, err := Parse(qq)
			if err != nil {
				continue
			}
			a := Eval(ix, ast)
			b := SlowEval(ix, ast)
			if fmt.Sprint(a) != fmt.Sprint(b) {
				t.Fatalf("mismatch query %q:\n%+v vs %+v", qq, a, b)
			}
		}
	}
}

func TestBM25Ordering(t *testing.T) {
	ix := NewIndex()
	buildCorpus(ix, []string{
		"cat cat mouse",
		"cat mouse",
		"mouse elephant",
	})
	res, err := SearchBM25(ix, `cat OR mouse`, 1.2, 0.75)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) < 2 {
		t.Fatal("bm25 expects multiple hits")
	}
	for i := 1; i < len(res); i++ {
		if res[i-1].Score < res[i].Score {
			t.Fatalf("BM25 не упорядочен по убыванию на %d: %v < %v", i, res[i-1].Score, res[i].Score)
		}
	}
}
