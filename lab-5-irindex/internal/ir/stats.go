package ir

import (
	"os"
	"unsafe"
)

// IndexSizeReport размеры индекса в памяти и на диске.
type IndexSizeReport struct {
	Docs          int
	Terms         int
	PostingsLists int
	RawBytes      int64
	CompressedPath string
	CompressedBytes int64
	Ratio         float64
}

// EstimateIndexBytes оценка RAM занятого инвертированным индексом (постинги + строки термов + doc meta).
func EstimateIndexBytes(ix *InvIndex) int64 {
	if ix == nil {
		return 0
	}
	var n int64
	n += int64(len(ix.Docs)) * int64(unsafe.Sizeof(Doc{}))
	for _, d := range ix.Docs {
		for _, t := range d.Tokens {
			n += int64(len(t))
		}
		n += int64(cap(d.Tokens)) * 8
	}
	for term, ps := range ix.postings {
		n += int64(len(term))
		for _, p := range ps {
			n += int64(unsafe.Sizeof(posting{})) + int64(len(p.Poss))*4
		}
		n += int64(cap(ps)) * int64(unsafe.Sizeof(posting{}))
	}
	n += int64(len(ix.postings)) * 16
	return n
}

// MeasureIndexSizes строит отчёт: raw в RAM и размер файла после SaveCompressed.
func MeasureIndexSizes(ix *InvIndex, compressedPath string) (IndexSizeReport, error) {
	r := IndexSizeReport{
		Docs:          ix.NumDocs(),
		Terms:         len(ix.postings),
		PostingsLists: countPostings(ix),
		RawBytes:      EstimateIndexBytes(ix),
		CompressedPath: compressedPath,
	}
	if err := SaveCompressed(ix, compressedPath); err != nil {
		return r, err
	}
	st, err := os.Stat(compressedPath)
	if err != nil {
		return r, err
	}
	r.CompressedBytes = st.Size()
	if r.RawBytes > 0 {
		r.Ratio = float64(r.RawBytes) / float64(r.CompressedBytes)
	}
	return r, nil
}

func countPostings(ix *InvIndex) int {
	c := 0
	for _, ps := range ix.postings {
		c += len(ps)
	}
	return c
}
