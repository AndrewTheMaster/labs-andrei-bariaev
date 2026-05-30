package ir

import "fmt"

// SearchBoolMMap — булев поиск по mmap-индексу.
func SearchBoolMMap(mi *MMapIndex, query string) (MatchSet, Node, error) {
	n, err := Parse(query)
	if err != nil {
		return nil, nil, err
	}
	ctx := NewEvalCtx(mi)
	return ctx.Eval(n), n, nil
}

// SearchBoolMMapWarnMSM как SearchBoolMMap; MSM без текстов документов на диске не поддерживается.
func SearchBoolMMapWarnMSM(mi *MMapIndex, query string) (MatchSet, Node, error) {
	n, err := Parse(query)
	if err != nil {
		return nil, nil, err
	}
	if containsMSM(n) {
		return nil, n, fmt.Errorf("MSM требует in-memory индекс (тексты документов не хранятся в .irx)")
	}
	ctx := NewEvalCtx(mi)
	return ctx.Eval(n), n, nil
}

// SearchBM25MMap — булев фильтр + BM25 по mmap-индексу.
func SearchBM25MMap(mi *MMapIndex, query string, k1, b float64) ([]Scored, Node, error) {
	n, err := Parse(query)
	if err != nil {
		return nil, nil, err
	}
	if containsMSM(n) {
		return nil, n, fmt.Errorf("MSM требует in-memory индекс (тексты документов не хранятся в .irx)")
	}
	ctx := NewEvalCtx(mi)
	ds := ctx.Eval(n)
	return BM25Index(mi, ds, PositiveTerms(n), k1, b), n, nil
}

func containsMSM(n Node) bool {
	if n == nil {
		return false
	}
	switch t := n.(type) {
	case *MSM:
		return true
	case *Not:
		return containsMSM(t.Child)
	case *And:
		for _, c := range t.Children {
			if containsMSM(c) {
				return true
			}
		}
	case *Or:
		return containsMSM(t.Left) || containsMSM(t.Right)
	case *Near:
		return false
	case *Adj:
		return false
	default:
		return false
	}
	return false
}
