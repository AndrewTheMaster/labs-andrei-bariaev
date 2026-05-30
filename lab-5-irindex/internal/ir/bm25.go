package ir

import (
	"math"
	"sort"
)

type Scored struct {
	DocID uint32
	Score float64
}

func postingTF(ps []posting, docID uint32) float64 {
	for i := range ps {
		if ps[i].DocID == docID {
			return float64(len(ps[i].Poss))
		}
	}
	return 0
}

func avgDocLen(ix PostingIndex) float64 {
	n := ix.NumDocs()
	if n == 0 {
		return 0
	}
	sum := 0
	for i := 0; i < n; i++ {
		sum += ix.DocLen(uint32(i))
	}
	return float64(sum) / float64(n)
}

// BM25Index оценивает кандидатов по PostingIndex (RAM или mmap).
func BM25Index(ix PostingIndex, cand MatchSet, queryTerms []string, k1, b float64) []Scored {
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
	avgdl := avgDocLen(ix)
	if avgdl == 0 {
		avgdl = 1
	}

	type qStat struct {
		idf float64
		ps  []posting
	}
	stats := make([]qStat, len(queryTerms))
	for i, q := range queryTerms {
		ps, _ := ix.LookupPostings(q)
		df := float64(len(ps))
		stats[i] = qStat{
			idf: math.Log(1.0 + (N-df+0.5)/(df+0.5)),
			ps:  ps,
		}
	}

	out := make([]Scored, 0, len(cand))
	for _, id := range cand {
		Ld := float64(ix.DocLen(id))
		score := 0.0
		for _, st := range stats {
			tf := postingTF(st.ps, id)
			if tf == 0 {
				continue
			}
			score += st.idf * (tf * (k1 + 1)) / (tf + k1*(1.0-b+b*(Ld/avgdl)))
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

// BM25 оценивает кандидатов in-memory индекса.
func BM25(ix *InvIndex, cand MatchSet, queryTerms []string, k1, b float64) []Scored {
	return BM25Index(ix, cand, queryTerms, k1, b)
}
