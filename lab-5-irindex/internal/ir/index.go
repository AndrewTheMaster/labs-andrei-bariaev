package ir

import "sort"

type posting struct {
	DocID uint32
	Poss  []uint32
}

type Doc struct {
	ID     uint32
	Tokens []string
}

// InvIndex хранит позиционные постинговые списки (DocID монотонно растёт с порядком Add).
type InvIndex struct {
	Docs     []Doc
	postings map[string][]posting
	// allIDsBuf ленивая последовательность [0 .. NumDocs-1] для дополнений NOT без аллокаций на запросе.
	allIDsBuf []uint32
}

func NewIndex() *InvIndex {
	return &InvIndex{postings: make(map[string][]posting)}
}

func sortUint32(s []uint32) {
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
}

// Add добавляет документ как последовательность токенов.
func (ix *InvIndex) Add(tokens []string) uint32 {
	id := uint32(len(ix.Docs))
	ix.Docs = append(ix.Docs, Doc{ID: id, Tokens: tokens})
	posByTok := make(map[string][]uint32)
	for pos, tok := range tokens {
		posByTok[tok] = append(posByTok[tok], uint32(pos))
	}
	for tok, poss := range posByTok {
		sortUint32(poss)
		ix.postings[tok] = append(ix.postings[tok], posting{DocID: id, Poss: append([]uint32(nil), poss...)})
	}
	return id
}

func (ix *InvIndex) NumDocs() int { return len(ix.Docs) }

func (ix *InvIndex) Postings(tok string) []posting {
	return ix.postings[tok]
}

func (ix *InvIndex) df(tok string) int {
	return len(ix.postings[tok])
}

// allDocIDs возвращает [0, 1, …, NumDocs−1]; буфер пересоздаётся только при изменении числа документов.
func (ix *InvIndex) allDocIDs() []uint32 {
	n := ix.NumDocs()
	if len(ix.allIDsBuf) == n {
		return ix.allIDsBuf
	}
	ix.allIDsBuf = make([]uint32, n)
	for i := range ix.allIDsBuf {
		ix.allIDsBuf[i] = uint32(i)
	}
	return ix.allIDsBuf
}
