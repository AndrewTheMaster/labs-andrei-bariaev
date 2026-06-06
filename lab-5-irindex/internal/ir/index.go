package ir

import "sort"

type posting struct {
	DocID uint32
	Poss  []uint32
}

type Doc struct {
	ID     uint32
	Title  string // заголовок wiki (для irquery)
	Tokens []string
	NTok   int // число токенов (для edge/BM25/MSM, если Tokens не хранятся)
}

// InvIndex хранит позиционные постинговые списки (DocID монотонно растёт с порядком Add).
type InvIndex struct {
	Docs     []Doc
	postings map[string][]posting
	allIDsBuf []uint32
	posArena  []uint32
	tokScratch map[string][]uint32
	// scratchKeys — термы только текущего документа (не обходим весь словарь вики на каждый Add).
	scratchKeys []string
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

// Add добавляет документ и сохраняет токены в RAM (тесты, MSM).
func (ix *InvIndex) Add(tokens []string) uint32 {
	return ix.add(tokens, true, "")
}

// AddLean — постинги + NTok + title; тексты не копируются (вики).
func (ix *InvIndex) AddLean(tokens []string) uint32 {
	return ix.add(tokens, false, "")
}

// AddLeanTitle как AddLean с заголовком документа.
func (ix *InvIndex) AddLeanTitle(tokens []string, title string) uint32 {
	return ix.add(tokens, false, title)
}

func (ix *InvIndex) add(tokens []string, keepTokens bool, title string) uint32 {
	id := uint32(len(ix.Docs))
	if keepTokens {
		cp := append([]string(nil), tokens...)
		ix.Docs = append(ix.Docs, Doc{ID: id, Tokens: cp, NTok: len(tokens), Title: title})
	} else {
		ix.Docs = append(ix.Docs, Doc{ID: id, NTok: len(tokens), Title: title})
	}

	for _, k := range ix.scratchKeys {
		ix.tokScratch[k] = ix.tokScratch[k][:0]
	}
	ix.scratchKeys = ix.scratchKeys[:0]

	for pos, tok := range tokens {
		if len(ix.tokScratch[tok]) == 0 {
			ix.scratchKeys = append(ix.scratchKeys, tok)
		}
		ix.tokScratch[tok] = append(ix.tokScratch[tok], uint32(pos))
	}
	for _, tok := range ix.scratchKeys {
		poss := ix.tokScratch[tok]
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
