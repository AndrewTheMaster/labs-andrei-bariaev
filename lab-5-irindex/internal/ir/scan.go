package ir

// SlowEval — эталон без индекса: прямой скан токенов документа (для верификации/бенч baseline).
func SlowEval(ix *InvIndex, n Node) MatchSet {
	out := make(MatchSet)
	for _, d := range ix.Docs {
		if slowEvalDoc(d, n) {
			out[d.ID] = struct{}{}
		}
	}
	return out
}

func slowEvalDoc(d Doc, n Node) bool {
	switch t := n.(type) {
	case *Term:
		for _, x := range d.Tokens {
			if x == t.Lex {
				return true
			}
		}
		return false
	case *Not:
		return !slowEvalDoc(d, t.Child)
	case *And:
		if len(t.Children) == 0 {
			return false
		}
		for _, ch := range t.Children {
			if !slowEvalDoc(d, ch) {
				return false
			}
		}
		return true
	case *Or:
		return slowEvalDoc(d, t.Left) || slowEvalDoc(d, t.Right)
	case *Near:
		return slowNearDoc(d, t.K, t.A, t.B)
	case *Adj:
		return slowAdjDoc(d, t.A, t.B)
	case *MSM:
		return msmInDoc(d, t.Terms, t.W)
	case *EdgeStart:
		return len(d.Tokens) > 0 && d.Tokens[0] == t.Lex
	case *EdgeEnd:
		nTok := len(d.Tokens)
		return nTok > 0 && d.Tokens[nTok-1] == t.Lex
	default:
		return false
	}
}

func slowNearDoc(d Doc, k int, a, b string) bool {
	if k < 0 {
		k = 0
	}
	var pa, pb []uint32
	for i, tok := range d.Tokens {
		switch tok {
		case a:
			pa = append(pa, uint32(i))
		case b:
			pb = append(pb, uint32(i))
		}
	}
	return positionsNear(pa, pb, k)
}

func slowAdjDoc(d Doc, a, b string) bool {
	for i := 0; i+1 < len(d.Tokens); i++ {
		if d.Tokens[i] == a && d.Tokens[i+1] == b {
			return true
		}
	}
	return false
}
