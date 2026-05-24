package ir

// EvalCtx выполняет Eval с буферами docID (меньше аллокаций на AND/OR/NOT).
type EvalCtx struct {
	ix  PostingIndex
	ram *InvIndex
	tmp, work []uint32
	out       []uint32
}

func NewEvalCtx(ix PostingIndex) *EvalCtx {
	c := &EvalCtx{ix: ix}
	if inv, ok := ix.(*InvIndex); ok {
		c.ram = inv
	}
	return c
}

func (c *EvalCtx) Reset() {
	c.tmp = c.tmp[:0]
	c.work = c.work[:0]
	c.out = c.out[:0]
}

// Eval — булева оценка с переиспользованием буферов.
func (c *EvalCtx) Eval(n Node) MatchSet {
	return c.eval(n)
}

func (c *EvalCtx) eval(n Node) []uint32 {
	switch t := n.(type) {
	case *Term:
		return c.postingDocIDs(t.Lex)
	case *Not:
		all := append(c.work[:0], c.ix.AllDocIDs()...)
		child := append(c.tmp[:0], c.eval(t.Child)...)
		return subtractSortedInto(c.out[:0], all, child)
	case *And:
		return c.evalAnd(t.Children)
	case *Or:
		left := append(c.work[:0], c.eval(t.Left)...)
		right := append(c.tmp[:0], c.eval(t.Right)...)
		return unionSortedInto(c.out[:0], left, right)
	case *Near:
		return evalNear(c.ix, t.K, t.A, t.B)
	case *Adj:
		return evalAdj(c.ix, t.A, t.B)
	case *MSM:
		if c.ram == nil {
			return nil
		}
		return evalMSM(c.ram, t.W, t.Terms)
	case *EdgeStart:
		return edgeStart(c.ix, t.Lex)
	case *EdgeEnd:
		return edgeEnd(c.ix, t.Lex)
	default:
		return nil
	}
}

func (c *EvalCtx) evalAnd(children []Node) []uint32 {
	if len(children) == 0 {
		return nil
	}
	acc := append(c.out[:0], c.eval(children[0])...)
	for i := 1; i < len(children); i++ {
		right := append(c.tmp[:0], c.eval(children[i])...)
		if i%2 == 1 {
			acc = intersectSortedSkipInto(c.work[:0], acc, right)
		} else {
			acc = intersectSortedSkipInto(c.out[:0], acc, right)
		}
		if len(acc) == 0 {
			return nil
		}
	}
	return acc
}

func (c *EvalCtx) postingDocIDs(term string) []uint32 {
	ps, err := c.ix.LookupPostings(term)
	if err != nil || len(ps) == 0 {
		return nil
	}
	c.tmp = c.tmp[:0]
	for i := range ps {
		c.tmp = append(c.tmp, ps[i].DocID)
	}
	return c.tmp
}
