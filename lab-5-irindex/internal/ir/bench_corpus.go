package ir

import (
	"math/rand/v2"
	"os"
	"strconv"
	"strings"
	"testing"
)

type benchQueries struct {
	AND     string
	OR      string
	NOT     string
	ADJ     string
	NEAR    string
	EDGE    string
	MSM     string
	Complex string
}

func RuWikiBenchQueries() benchQueries {
	return benchQueries{
		AND:     `россия AND москва`,
		OR:      `россия OR москва`,
		NOT:     `NOT россия`,
		ADJ:     `ADJ(великая, отечественная)`,
		NEAR:    `NEAR(3, великая, отечественная)`,
		EDGE:    `FIRST(россия) AND NOT EDGE_END(город)`,
		MSM:     `MSM(40, россия, город, река)`,
		Complex: `(россия OR москва) AND история AND NOT футбол`,
	}
}

func defaultBenchQueries() benchQueries {
	path := ResolveCorpusPath()
	if path != "" {
		if _, err := os.Stat(path); err == nil {
			return RuWikiBenchQueries()
		}
	}
	return benchQueries{
		AND:     `alpha AND beta`,
		OR:      `alpha OR gamma`,
		NOT:     `NOT delta`,
		ADJ:     `ADJ(alpha, beta)`,
		NEAR:    `NEAR(3, alpha, gamma)`,
		EDGE:    `FIRST(alpha) AND NOT EDGE_END(delta)`,
		MSM:     `MSM(40, gamma, omega, alpha)`,
		Complex: `(alpha OR beta) AND gamma AND NOT delta`,
	}
}

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
		var buf strings.Builder
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

// buildCorpusN: wiki XML (CORPUS_XML / data/…) при наличии файла, иначе синтетика.
func buildCorpusN(tb testing.TB, n int) *InvIndex {
	tb.Helper()
	path := ResolveCorpusPath()
	if path != "" {
		if _, err := os.Stat(path); err == nil {
			ix, st, err := BuildIndexFromWikiXML(path, CorpusOpts{MaxDocs: n})
			if err != nil {
				tb.Fatalf("wiki build %s: %v", path, err)
			}
			if st.PagesIndexed < n {
				tb.Logf("wiki: запрошено %d, проиндексировано %d", n, st.PagesIndexed)
			}
			return ix
		}
	}
	return fillCorpus(n, 4242, defaultWords())
}
