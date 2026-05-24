package ir

import (
	"bytes"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
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

func TestADJ(t *testing.T) {
	ix := NewIndex()
	buildCorpus(ix, []string{
		"alpha beta gamma",
		"beta alpha gamma",
		"alpha x beta",
	})
	n, err := Parse(`ADJ(alpha, beta)`)
	if err != nil {
		t.Fatal(err)
	}
	got := Eval(ix, n)
	slow := SlowEval(ix, n)
	if fmt.Sprint(got) != fmt.Sprint(slow) {
		t.Fatalf("ADJ index vs slow mismatch: %v vs %v", got, slow)
	}
	if len(got) != 1 || !contains(got, 0) {
		t.Fatalf("ADJ(alpha,beta) expected {0}, got %#v", got)
	}
}

func contains(ms MatchSet, id uint32) bool {
	_, ok := slices.BinarySearch(ms, id)
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

func TestCompressedMMapRoundtrip(t *testing.T) {
	ix := NewIndex()
	buildCorpus(ix, []string{
		"alpha beta gamma",
		"beta gamma gamma",
		"delta alpha",
	})
	tmp := filepath.Join(t.TempDir(), "index.irx")
	if err := SaveCompressed(ix, tmp); err != nil {
		t.Fatal(err)
	}
	mi, err := OpenMMapIndex(tmp)
	if err != nil {
		t.Fatal(err)
	}
	defer mi.Close()

	if mi.NumDocs() != ix.NumDocs() {
		t.Fatalf("num docs mismatch: %d vs %d", mi.NumDocs(), ix.NumDocs())
	}
	for _, term := range []string{"alpha", "beta", "gamma", "delta"} {
		got, err := mi.Postings(term)
		if err != nil {
			t.Fatal(err)
		}
		want := ix.Postings(term)
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Fatalf("postings mismatch term=%q: got=%v want=%v", term, got, want)
		}
	}
	if _, err := os.Stat(tmp); err != nil {
		t.Fatalf("saved index file missing: %v", err)
	}
}

func TestIndexSizesOnSynthetic(t *testing.T) {
	for _, n := range []int{400, 2000} {
		ix := fillCorpus(n, 4242, defaultWords())
		tmp := filepath.Join(t.TempDir(), "index.irx")
		r, err := MeasureIndexSizes(ix, tmp)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("N=%d docs=%d terms=%d postings=%d raw_bytes=%d compressed_bytes=%d ratio=%.2f",
			n, r.Docs, r.Terms, r.PostingsLists, r.RawBytes, r.CompressedBytes, r.Ratio)
	}
}

func TestParseErrors(t *testing.T) {
	bad := []string{
		`ADJ(alpha beta)`,
		`NEAR(2, alpha)`,
		`FIRST alpha`,
		`(alpha AND beta`,
		`alpha AND )`,
	}
	for _, q := range bad {
		if _, err := Parse(q); err == nil {
			t.Fatalf("expected parse error for query: %q", q)
		}
	}
}

func TestMMapOpenErrors(t *testing.T) {
	dir := t.TempDir()

	empty := filepath.Join(dir, "empty.irx")
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenMMapIndex(empty); err == nil {
		t.Fatal("expected error on empty index file")
	}

	badMagic := filepath.Join(dir, "badmagic.irx")
	if err := os.WriteFile(badMagic, []byte("not-an-index"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenMMapIndex(badMagic); err == nil {
		t.Fatal("expected error on bad magic")
	}

	// truncation after header/doc-len section
	var buf bytes.Buffer
	buf.Write([]byte{'I', 'R', 'I', 'X', 'V', '2', 'B', 'P'})
	buf.Write([]byte{1, 0, 0, 0}) // nDocs=1
	buf.Write([]byte{1, 0, 0, 0}) // nTerms=1
	// no doclen and no terms -> truncated
	trunc := filepath.Join(dir, "trunc.irx")
	if err := os.WriteFile(trunc, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenMMapIndex(trunc); err == nil {
		t.Fatal("expected error on truncated file")
	}
}

func TestMMapDocLenAndDF(t *testing.T) {
	ix := NewIndex()
	buildCorpus(ix, []string{
		"alpha beta gamma",
		"alpha beta",
		"omega",
	})
	tmp := filepath.Join(t.TempDir(), "index.irx")
	if err := SaveCompressed(ix, tmp); err != nil {
		t.Fatal(err)
	}
	mi, err := OpenMMapIndex(tmp)
	if err != nil {
		t.Fatal(err)
	}
	defer mi.Close()

	if mi.DocLen(0) != 3 || mi.DocLen(1) != 2 || mi.DocLen(2) != 1 {
		t.Fatalf("unexpected doc lens: %d %d %d", mi.DocLen(0), mi.DocLen(1), mi.DocLen(2))
	}
	if mi.DocLen(99) != 0 {
		t.Fatalf("out of range doc len must be 0")
	}
	if mi.df("alpha") != 2 {
		t.Fatalf("df(alpha) want 2 got %d", mi.df("alpha"))
	}
	if mi.df("missing") != 0 {
		t.Fatalf("df(missing) want 0 got %d", mi.df("missing"))
	}
}

func TestComplexExpressionFastSlow(t *testing.T) {
	ix := NewIndex()
	buildCorpus(ix, []string{
		"alpha beta gamma omega",
		"alpha x beta y gamma",
		"omega alpha beta",
		"zeta eta",
		"beta gamma alpha",
	})
	q := `(ADJ(alpha,beta) OR NEAR(2,alpha,gamma)) AND NOT LAST(eta)`
	ast, err := Parse(q)
	if err != nil {
		t.Fatal(err)
	}
	fast := Eval(ix, ast)
	slow := SlowEval(ix, ast)
	if fmt.Sprint(fast) != fmt.Sprint(slow) {
		t.Fatalf("complex query mismatch fast=%v slow=%v", fast, slow)
	}
}
