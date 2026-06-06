package ir

import (
	"fmt"
	"sort"
	"strings"
)

// HitLine — одна строка результата для irquery (проверяемый вывод).
type HitLine struct {
	DocID      uint32
	Title      string
	Matched    string // термы запроса, найденные в документе
	TotalTF    int
	Score      float64 // 0 если без BM25
	HasScore   bool
}

// FormatHitsForQuery строит строки вывода: title + какие термы попали и с каким tf.
func FormatHitsForQuery(ix PostingIndex, titles func(uint32) string, queryTerms []string, ids []uint32, scored []Scored) []HitLine {
	termSet := make(map[string]struct{}, len(queryTerms))
	for _, t := range queryTerms {
		termSet[strings.ToLower(t)] = struct{}{}
	}
	useScore := len(scored) > 0
	scoreOf := make(map[uint32]float64, len(scored))
	for _, s := range scored {
		scoreOf[s.DocID] = s.Score
	}
	if useScore {
		ids = make([]uint32, len(scored))
		for i, s := range scored {
			ids[i] = s.DocID
		}
	}

	out := make([]HitLine, 0, len(ids))
	for _, id := range ids {
		var parts []string
		totalTF := 0
		for term := range termSet {
			ps, _ := ix.LookupPostings(term)
			tf := 0
			for _, p := range ps {
				if p.DocID == id {
					tf = len(p.Poss)
					break
				}
			}
			if tf > 0 {
				parts = append(parts, fmt.Sprintf("%s×%d", term, tf))
				totalTF += tf
			}
		}
		sort.Strings(parts)
		title := titles(id)
		if title == "" {
			title = fmt.Sprintf("(doc %d)", id)
		}
		line := HitLine{
			DocID:   id,
			Title:   title,
			Matched: strings.Join(parts, ", "),
			TotalTF: totalTF,
		}
		if useScore {
			line.Score = scoreOf[id]
			line.HasScore = true
		}
		out = append(out, line)
	}
	return out
}

// SearchBoolMMapDetailed — булев поиск + строки для вывода.
func SearchBoolMMapDetailed(mi *MMapIndex, query string) ([]uint32, []HitLine, error) {
	n, err := Parse(query)
	if err != nil {
		return nil, nil, err
	}
	if containsMSM(n) {
		return nil, nil, fmt.Errorf("MSM требует in-memory индекс (тексты документов не хранятся в .irx)")
	}
	ctx := NewEvalCtx(mi)
	ids := ctx.Eval(n)
	terms := PositiveTerms(n)
	lines := FormatHitsForQuery(mi, mi.DocTitle, terms, ids, nil)
	return ids, lines, nil
}

// SearchBM25MMapDetailed — BM25 + строки для вывода.
func SearchBM25MMapDetailed(mi *MMapIndex, query string, k1, b float64) ([]Scored, []HitLine, error) {
	n, err := Parse(query)
	if err != nil {
		return nil, nil, err
	}
	if containsMSM(n) {
		return nil, nil, fmt.Errorf("MSM требует in-memory индекс")
	}
	ctx := NewEvalCtx(mi)
	ds := ctx.Eval(n)
	terms := PositiveTerms(n)
	scored := BM25Index(mi, ds, terms, k1, b)
	lines := FormatHitsForQuery(mi, mi.DocTitle, terms, nil, scored)
	return scored, lines, nil
}
