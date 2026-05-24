package ir

// PostingIndex — минимальный API для булевой оценки (RAM или mmap).
type PostingIndex interface {
	NumDocs() int
	LookupPostings(term string) ([]posting, error)
	DocLen(id uint32) int
	AllDocIDs() []uint32
}

func (ix *InvIndex) LookupPostings(term string) ([]posting, error) {
	return ix.Postings(term), nil
}

func (ix *InvIndex) DocLen(id uint32) int {
	return ix.docLen(id)
}

func (ix *InvIndex) AllDocIDs() []uint32 {
	return ix.allDocIDs()
}
