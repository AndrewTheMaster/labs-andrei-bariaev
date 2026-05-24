package ir

import "sort"

type posting struct {
	DocID uint32
	Poss  []uint32
}

type Doc struct {
	ID     uint32
	Tokens []string
	NTok   int // число токенов (для edge/BM25, если Tokens не хранятся)
}

// InvIndex хранит позиционные постинговые списки (DocID монотонно растёт с порядком Add).
type InvIndex struct {
	Docs     []Doc
	postings map[string][]posting
	// allIDsBuf ленивая последовательность [0 .. NumDocs-1] для дополнений NOT.
	allIDsBuf []uint32
	// posArena — единый буфер позиций; срезы в posting.Poss ссылаются сюда (без копии на терм).
	posArena []uint32
	// tokScratch переиспользуется в Add: map очищается по длине, слайсы — [:0].
	tokScratch map[string][]uint32
}

func NewIndex() *InvIndex {
	return &InvIndex{
		postings:   make(map[string][]posting),
		tokScratch: make(map[string][]uint32),
	}
}

func sortUint32(s []uint32) {
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
}

// Add добавляет документ как последовательность токенов.
func (ix *InvIndex) Add(tokens []string) uint32 {
	id := uint32(len(ix.Docs))
	ix.Docs = append(ix.Docs, Doc{ID: id, Tokens: tokens, NTok: len(tokens)})

	for k := range ix.tokScratch {
		ix.tokScratch[k] = ix.tokScratch[k][:0]
	}
	for pos, tok := range tokens {
		ix.tokScratch[tok] = append(ix.tokScratch[tok], uint32(pos))
	}
	for tok, poss := range ix.tokScratch {
		if len(poss) == 0 {
			continue
		}
		sortUint32(poss)
		start := len(ix.posArena)
		ix.posArena = append(ix.posArena, poss...)
		ix.postings[tok] = append(ix.postings[tok], posting{
			DocID: id,
			Poss:  ix.posArena[start : start+len(poss)],
		})
	}
	return id
}

func (ix *InvIndex) docLen(id uint32) int {
	if int(id) >= len(ix.Docs) {
		return 0
	}
	d := ix.Docs[id]
	if d.NTok > 0 {
		return d.NTok
	}
	return len(d.Tokens)
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
