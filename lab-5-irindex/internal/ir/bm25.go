package ir

import (
	"math"
	"sort"
)

type Scored struct {
	DocID uint32
	Score float64
}

// BM25 оценивает кандидатов; queryTerms — наблюдаемые позитивные лексемы.
func BM25(ix *InvIndex, cand MatchSet, queryTerms []string, k1, b float64) []Scored {
	if len(queryTerms) == 0 {
		out := make([]Scored, 0, len(cand))
		for _, id := range cand {
			out = append(out, Scored{DocID: id})
		}
		return out
	}
	N := float64(ix.NumDocs())
	if N == 0 {
		return nil
	}
	avgdl := 0.0
	for _, d := range ix.Docs {
		avgdl += float64(len(d.Tokens))
	}
	avgdl /= N

	idf := func(term string) float64 {
		df := float64(ix.df(term))
		return math.Log(1.0 + (N-df+0.5)/(df+0.5))
	}

	tfDoc := func(d Doc, term string) float64 {
		c := 0
		for _, t := range d.Tokens {
			if t == term {
				c++
			}
		}
		return float64(c)
	}

	out := make([]Scored, 0, len(cand))
	for _, id := range cand {
		d := ix.Docs[id]
		Ld := float64(len(d.Tokens))
		score := 0.0
		for _, q := range queryTerms {
			tf := tfDoc(d, q)
			if tf == 0 {
				continue
			}
			idfq := idf(q)
			score += idfq * (tf * (k1 + 1)) / (tf + k1*(1.0-b+b*(Ld/avgdl)))
		}
		out = append(out, Scored{DocID: id, Score: score})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score == out[j].Score {
			return out[i].DocID < out[j].DocID
		}
		return out[i].Score > out[j].Score
	})
	return out
}
