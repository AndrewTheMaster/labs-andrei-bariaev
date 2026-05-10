package ir

import "sort"

// MatchSet — множество ID документов, удовлетворяющих запросу.
type MatchSet map[uint32]struct{}

func allDocs(ix *InvIndex) MatchSet {
	out := make(MatchSet, ix.NumDocs())
	for i := 0; i < ix.NumDocs(); i++ {
		out[uint32(i)] = struct{}{}
	}
	return out
}

func postingsDocSet(ix *InvIndex, term string) MatchSet {
	ps := ix.Postings(term)
	out := make(MatchSet, len(ps))
	for _, p := range ps {
		out[p.DocID] = struct{}{}
	}
	return out
}

func intersect(a, b MatchSet) MatchSet {
	if len(a) > len(b) {
		a, b = b, a
	}
	out := make(MatchSet)
	for id := range a {
		if _, ok := b[id]; ok {
			out[id] = struct{}{}
		}
	}
	return out
}

func setToSortedIDs(s MatchSet) []uint32 {
	out := make([]uint32, 0, len(s))
	for id := range s {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func sortedIDsToSet(ids []uint32) MatchSet {
	out := make(MatchSet, len(ids))
	for _, id := range ids {
		out[id] = struct{}{}
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

func union(a, b MatchSet) MatchSet {
	out := make(MatchSet, len(a)+len(b))
	for id := range a {
		out[id] = struct{}{}
	}
	for id := range b {
		out[id] = struct{}{}
	}
	return out
}

func subtract(a, b MatchSet) MatchSet {
	out := make(MatchSet)
	for id := range a {
		if _, ok := b[id]; !ok {
			out[id] = struct{}{}
		}
	}
	return out
}

// Eval выполняет булеву модель документов над индексом.
func Eval(ix *InvIndex, n Node) MatchSet {
	switch t := n.(type) {
	case *Term:
		return postingsDocSet(ix, t.Lex)
	case *Not:
		return subtract(allDocs(ix), Eval(ix, t.Child))
	case *And:
		if len(t.Children) == 0 {
			return MatchSet{}
		}
		ds := Eval(ix, t.Children[0])
		sorted := setToSortedIDs(ds)
		for i := 1; i < len(t.Children); i++ {
			right := setToSortedIDs(Eval(ix, t.Children[i]))
			sorted = intersectSortedSkip(sorted, right)
			if len(sorted) == 0 {
				return MatchSet{}
			}
		}
		return sortedIDsToSet(sorted)
	case *Or:
		return union(Eval(ix, t.Left), Eval(ix, t.Right))
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
		return MatchSet{}
	}
}

func evalNear(ix *InvIndex, k int, a, b string) MatchSet {
	if k < 0 {
		k = 0
	}
	da := ix.Postings(a)
	db := ix.Postings(b)
	out := MatchSet{}

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
				out[doc] = struct{}{}
			}
			i++
			j++
		}
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

func evalAdj(ix *InvIndex, a, b string) MatchSet {
	da := ix.Postings(a)
	db := ix.Postings(b)
	out := MatchSet{}
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
				out[doc] = struct{}{}
			}
			i++
			j++
		}
	}
	return out
}

func edgeStart(ix *InvIndex, term string) MatchSet {
	ps := ix.Postings(term)
	out := MatchSet{}
	for _, p := range ps {
		if len(p.Poss) > 0 && p.Poss[0] == 0 {
			out[p.DocID] = struct{}{}
		}
	}
	return out
}

func edgeEnd(ix *InvIndex, term string) MatchSet {
	out := MatchSet{}
	for _, p := range ix.Postings(term) {
		last := len(ix.Docs[p.DocID].Tokens) - 1
		if last < 0 {
			continue
		}
		lp := uint32(last)
		for _, pos := range p.Poss {
			if pos == lp {
				out[p.DocID] = struct{}{}
				break
			}
		}
	}
	return out
}

func evalMSM(ix *InvIndex, w int, terms []string) MatchSet {
	if len(terms) == 0 {
		return allDocs(ix)
	}
	ds := postingsDocSet(ix, terms[0])
	for i := 1; i < len(terms); i++ {
		ds = intersect(ds, postingsDocSet(ix, terms[i]))
		if len(ds) == 0 {
			return ds
		}
	}
	if w < 0 {
		w = 0
	}
	out := MatchSet{}
	for id := range ds {
		if msmInDoc(ix.Docs[id], terms, w) {
			out[id] = struct{}{}
		}
	}
	return out
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
