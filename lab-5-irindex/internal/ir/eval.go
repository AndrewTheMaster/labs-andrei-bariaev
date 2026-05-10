package ir

import "slices"

// MatchSet — отсортированный по возрастанию массив уникальных ID документов.
type MatchSet []uint32

func postingDocIDs(ix *InvIndex, term string) []uint32 {
	ps := ix.Postings(term)
	if len(ps) == 0 {
		return nil
	}
	out := make([]uint32, len(ps))
	for i := range ps {
		out[i] = ps[i].DocID
	}
	return out
}

// intersectSortedSkip: пересечение отсортированных docID с виртуальными skip-прыжками.
// Шаг берется как floor(sqrt(n)); дополнительных структур в памяти не создается.
func intersectSortedSkip(a, b []uint32) []uint32 {
	if len(a) == 0 || len(b) == 0 {
		return nil
	}
	out := make([]uint32, 0, min(len(a), len(b)))
	stepA := 1
	for stepA*stepA < len(a) {
		stepA++
	}
	stepB := 1
	for stepB*stepB < len(b) {
		stepB++
	}
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		ai, bj := a[i], b[j]
		if ai == bj {
			out = append(out, ai)
			i++
			j++
			continue
		}
		if ai < bj {
			next := i + stepA
			if next < len(a) && a[next] <= bj {
				for next < len(a) && a[next] <= bj {
					i = next
					next += stepA
				}
			} else {
				i++
			}
			continue
		}
		next := j + stepB
		if next < len(b) && b[next] <= ai {
			for next < len(b) && b[next] <= ai {
				j = next
				next += stepB
			}
		} else {
			j++
		}
	}
	return out
}

// unionSorted — объединение двух возрастающих уникальных списков без map.
func unionSorted(a, b []uint32) []uint32 {
	if len(a) == 0 {
		return slices.Clone(b)
	}
	if len(b) == 0 {
		return slices.Clone(a)
	}
	out := make([]uint32, 0, len(a)+len(b))
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		ai, bj := a[i], b[j]
		if ai < bj {
			out = append(out, ai)
			i++
		} else if bj < ai {
			out = append(out, bj)
			j++
		} else {
			out = append(out, ai)
			i++
			j++
		}
	}
	out = append(out, a[i:]...)
	out = append(out, b[j:]...)
	return out
}

// subtractSorted возвращает a \\ b для возрастающих уникальных срезов.
func subtractSorted(a, minus []uint32) []uint32 {
	if len(minus) == 0 || len(a) == 0 {
		return slices.Clone(a)
	}
	out := make([]uint32, 0, len(a))
	i, j := 0, 0
	for i < len(a) && j < len(minus) {
		ai, bj := a[i], minus[j]
		if ai < bj {
			out = append(out, ai)
			i++
		} else if ai > bj {
			j++
		} else {
			i++
			j++
		}
	}
	out = append(out, a[i:]...)
	if len(out) == 0 {
		return nil
	}
	return out
}

// Eval выполняет булеву модель документов над индексом.
func Eval(ix *InvIndex, n Node) MatchSet {
	return eval(ix, n)
}

func eval(ix *InvIndex, n Node) []uint32 {
	switch t := n.(type) {
	case *Term:
		return postingDocIDs(ix, t.Lex)
	case *Not:
		return subtractSorted(ix.allDocIDs(), eval(ix, t.Child))
	case *And:
		if len(t.Children) == 0 {
			return nil
		}
		sorted := eval(ix, t.Children[0])
		for i := 1; i < len(t.Children); i++ {
			sorted = intersectSortedSkip(sorted, eval(ix, t.Children[i]))
			if len(sorted) == 0 {
				return nil
			}
		}
		return sorted
	case *Or:
		return unionSorted(eval(ix, t.Left), eval(ix, t.Right))
	case *Near:
		return evalNear(ix, t.K, t.A, t.B)
	case *Adj:
		return evalAdj(ix, t.A, t.B)
	case *MSM:
		return evalMSM(ix, t.W, t.Terms)
	case *EdgeStart:
		return edgeStart(ix, t.Lex)
	case *EdgeEnd:
		return edgeEnd(ix, t.Lex)
	default:
		return nil
	}
}

func evalNear(ix *InvIndex, k int, a, b string) []uint32 {
	if k < 0 {
		k = 0
	}
	da := ix.Postings(a)
	db := ix.Postings(b)
	var out []uint32

	i, j := 0, 0
	for i < len(da) && j < len(db) {
		switch {
		case da[i].DocID < db[j].DocID:
			i++
		case da[i].DocID > db[j].DocID:
			j++
		default:
			doc := da[i].DocID
			if positionsNear(da[i].Poss, db[j].Poss, k) {
				out = append(out, doc)
			}
			i++
			j++
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func evalAdj(ix *InvIndex, a, b string) []uint32 {
	da := ix.Postings(a)
	db := ix.Postings(b)
	var out []uint32
	i, j := 0, 0
	for i < len(da) && j < len(db) {
		switch {
		case da[i].DocID < db[j].DocID:
			i++
		case da[i].DocID > db[j].DocID:
			j++
		default:
			doc := da[i].DocID
			if positionsAdj(da[i].Poss, db[j].Poss) {
				out = append(out, doc)
			}
			i++
			j++
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func edgeStart(ix *InvIndex, term string) []uint32 {
	ps := ix.Postings(term)
	var out []uint32
	for _, p := range ps {
		if len(p.Poss) > 0 && p.Poss[0] == 0 {
			out = append(out, p.DocID)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func edgeEnd(ix *InvIndex, term string) []uint32 {
	var out []uint32
	for _, p := range ix.Postings(term) {
		last := len(ix.Docs[p.DocID].Tokens) - 1
		if last < 0 {
			continue
		}
		lp := uint32(last)
		for _, pos := range p.Poss {
			if pos == lp {
				out = append(out, p.DocID)
				break
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func evalMSM(ix *InvIndex, w int, terms []string) []uint32 {
	if len(terms) == 0 {
		return slices.Clone(ix.allDocIDs())
	}
	ids := postingDocIDs(ix, terms[0])
	for i := 1; i < len(terms); i++ {
		ids = intersectSortedSkip(ids, postingDocIDs(ix, terms[i]))
		if len(ids) == 0 {
			return nil
		}
	}
	if w < 0 {
		w = 0
	}
	out := make([]uint32, 0, len(ids))
	for _, id := range ids {
		if msmInDoc(ix.Docs[id], terms, w) {
			out = append(out, id)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func positionsNear(pa, pb []uint32, k int) bool {
	i, j := 0, 0
	for i < len(pa) && j < len(pb) {
		xa, xb := pa[i], pb[j]
		diff := int(xa) - int(xb)
		if diff < 0 {
			diff = -diff
		}
		if diff <= k {
			return true
		}
		if xa < xb {
			i++
		} else {
			j++
		}
	}
	return false
}

func positionsAdj(pa, pb []uint32) bool {
	i, j := 0, 0
	for i < len(pa) && j < len(pb) {
		a, b := pa[i], pb[j]
		if b == a+1 {
			return true
		}
		if b <= a {
			j++
			continue
		}
		i++
	}
	return false
}

func msmInDoc(d Doc, terms []string, w int) bool {
	if len(d.Tokens) == 0 || len(terms) == 0 {
		return false
	}
	need := make(map[string]int)
	for _, t := range terms {
		need[t]++
	}
	type ev struct {
		pos uint32
		t   string
	}
	evs := make([]ev, 0, len(d.Tokens))
	for p, tok := range d.Tokens {
		if _, ok := need[tok]; ok {
			evs = append(evs, ev{uint32(p), tok})
		}
	}
	if len(evs) == 0 {
		return false
	}
	have := make(map[string]int, len(need))
	l := 0
	add := func(t string) { have[t]++ }
	sub := func(t string) { have[t]-- }
	satisfied := func() bool {
		for t, n := range need {
			if have[t] < n {
				return false
			}
		}
		return true
	}
	for r := range evs {
		add(evs[r].t)
		for satisfied() {
			if int(evs[r].pos-evs[l].pos) <= w {
				return true
			}
			sub(evs[l].t)
			l++
			if l > r {
				break
			}
		}
	}
	return false
}
