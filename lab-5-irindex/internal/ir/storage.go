package ir

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"sort"
	"syscall"
)

// IRIX V2: delta + bit-packing (вместо varint).
var fileMagic = [8]byte{'I', 'R', 'I', 'X', 'V', '2', 'B', 'P'}

var errBitpackTrunc = errors.New("bitpack: truncated stream")

type termMeta struct {
	off uint64
	ln  uint32
}

// MMapIndex хранит индекс в mmap-буфере; постинги декодируются по терму по требованию.
type MMapIndex struct {
	data    []byte
	fd      *os.File
	docLens []uint32
	terms   map[string]termMeta
}

func writeU32(buf *bytes.Buffer, v uint32) {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], v)
	buf.Write(b[:])
}

func writeU16(buf *bytes.Buffer, v uint16) {
	var b [2]byte
	binary.LittleEndian.PutUint16(b[:], v)
	buf.Write(b[:])
}

// encodePostings: docID (первый абсолютный, остальные delta), tf, позиции (delta) — bit-packing по потокам.
func encodePostings(ps []posting) []byte {
	var buf bytes.Buffer
	writeU32(&buf, uint32(len(ps)))
	if len(ps) == 0 {
		return buf.Bytes()
	}

	docVals := make([]uint32, len(ps))
	tfVals := make([]uint32, len(ps))
	var posVals []uint32
	var prevDoc uint32
	for i, p := range ps {
		if i == 0 {
			docVals[i] = p.DocID
		} else {
			docVals[i] = p.DocID - prevDoc
		}
		prevDoc = p.DocID
		tfVals[i] = uint32(len(p.Poss))
		var prevPos uint32
		for j, pos := range p.Poss {
			if j == 0 {
				posVals = append(posVals, pos)
			} else {
				posVals = append(posVals, pos-prevPos)
			}
			prevPos = pos
		}
	}

	docBits, docPay := packUint32Stream(docVals)
	tfBits, tfPay := packUint32Stream(tfVals)
	posBits, posPay := packUint32Stream(posVals)

	buf.WriteByte(docBits)
	buf.WriteByte(tfBits)
	buf.WriteByte(posBits)
	buf.WriteByte(0)
	writeU32(&buf, uint32(len(docPay)))
	buf.Write(docPay)
	writeU32(&buf, uint32(len(tfPay)))
	buf.Write(tfPay)
	writeU32(&buf, uint32(len(posPay)))
	buf.Write(posPay)
	return buf.Bytes()
}

func decodePostings(data []byte) ([]posting, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("postings block too small")
	}
	i := 0
	nPost := int(binary.LittleEndian.Uint32(data[i : i+4]))
	i += 4
	if nPost == 0 {
		return nil, nil
	}
	if len(data) < i+4 {
		return nil, fmt.Errorf("postings header truncated")
	}
	docBits := data[i]
	tfBits := data[i+1]
	posBits := data[i+2]
	i += 4

	readChunk := func() ([]byte, error) {
		if len(data) < i+4 {
			return nil, fmt.Errorf("postings chunk len truncated")
		}
		ln := int(binary.LittleEndian.Uint32(data[i : i+4]))
		i += 4
		if len(data) < i+ln {
			return nil, fmt.Errorf("postings chunk truncated")
		}
		chunk := data[i : i+ln]
		i += ln
		return chunk, nil
	}

	docPay, err := readChunk()
	if err != nil {
		return nil, err
	}
	tfPay, err := readChunk()
	if err != nil {
		return nil, err
	}
	posPay, err := readChunk()
	if err != nil {
		return nil, err
	}

	docDeltas, err := unpackUint32Stream(docBits, docPay, nPost)
	if err != nil {
		return nil, err
	}
	tfVals, err := unpackUint32Stream(tfBits, tfPay, nPost)
	if err != nil {
		return nil, err
	}
	nPos := 0
	for _, tf := range tfVals {
		nPos += int(tf)
	}
	posDeltas, err := unpackUint32Stream(posBits, posPay, nPos)
	if err != nil {
		return nil, err
	}

	out := make([]posting, 0, nPost)
	posIdx := 0
	var prevDoc uint32
	for pidx := 0; pidx < nPost; pidx++ {
		doc := docDeltas[pidx]
		if pidx > 0 {
			doc = prevDoc + doc
		}
		prevDoc = doc
		tf := int(tfVals[pidx])
		poss := make([]uint32, tf)
		var prevPos uint32
		for j := 0; j < tf; j++ {
			pos := posDeltas[posIdx]
			if j > 0 {
				pos = prevPos + pos
			}
			prevPos = pos
			poss[j] = pos
			posIdx++
		}
		out = append(out, posting{DocID: doc, Poss: poss})
	}
	return out, nil
}

// SaveCompressed сохраняет индекс в бинарном формате (delta + bit-packing).
func SaveCompressed(ix *InvIndex, path string) error {
	var buf bytes.Buffer
	buf.Write(fileMagic[:])
	writeU32(&buf, uint32(len(ix.Docs)))
	writeU32(&buf, uint32(len(ix.postings)))
	for _, d := range ix.Docs {
		writeU32(&buf, uint32(len(d.Tokens)))
	}

	keys := make([]string, 0, len(ix.postings))
	for k := range ix.postings {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		raw := encodePostings(ix.postings[k])
		if len(k) > 0xffff {
			return fmt.Errorf("term too long")
		}
		writeU16(&buf, uint16(len(k)))
		buf.WriteString(k)
		writeU32(&buf, uint32(len(raw)))
		buf.Write(raw)
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

func OpenMMapIndex(path string) (*MMapIndex, error) {
	fd, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	st, err := fd.Stat()
	if err != nil {
		fd.Close()
		return nil, err
	}
	if st.Size() == 0 {
		fd.Close()
		return nil, fmt.Errorf("empty index file")
	}
	data, err := syscall.Mmap(int(fd.Fd()), 0, int(st.Size()), syscall.PROT_READ, syscall.MAP_SHARED)
	if err != nil {
		fd.Close()
		return nil, err
	}
	mi := &MMapIndex{data: data, fd: fd}
	if err := mi.parseHeader(); err != nil {
		_ = mi.Close()
		return nil, err
	}
	return mi, nil
}

func (mi *MMapIndex) parseHeader() error {
	if len(mi.data) < 16 {
		return fmt.Errorf("file too small")
	}
	if !bytes.Equal(mi.data[:8], fileMagic[:]) {
		return fmt.Errorf("bad file magic")
	}
	nDocs := int(binary.LittleEndian.Uint32(mi.data[8:12]))
	nTerms := int(binary.LittleEndian.Uint32(mi.data[12:16]))
	i := 16
	if len(mi.data) < i+4*nDocs {
		return fmt.Errorf("truncated doc lens")
	}
	mi.docLens = make([]uint32, nDocs)
	for d := 0; d < nDocs; d++ {
		mi.docLens[d] = binary.LittleEndian.Uint32(mi.data[i : i+4])
		i += 4
	}
	mi.terms = make(map[string]termMeta, nTerms)
	for t := 0; t < nTerms; t++ {
		if len(mi.data) < i+2 {
			return fmt.Errorf("truncated term len")
		}
		l := int(binary.LittleEndian.Uint16(mi.data[i : i+2]))
		i += 2
		if len(mi.data) < i+l+4 {
			return fmt.Errorf("truncated term payload")
		}
		term := string(mi.data[i : i+l])
		i += l
		bl := binary.LittleEndian.Uint32(mi.data[i : i+4])
		i += 4
		if len(mi.data) < i+int(bl) {
			return fmt.Errorf("truncated postings block")
		}
		mi.terms[term] = termMeta{off: uint64(i), ln: bl}
		i += int(bl)
	}
	return nil
}

func (mi *MMapIndex) Close() error {
	var e1, e2 error
	if mi.data != nil {
		e1 = syscall.Munmap(mi.data)
		mi.data = nil
	}
	if mi.fd != nil {
		e2 = mi.fd.Close()
		mi.fd = nil
	}
	if e1 != nil {
		return e1
	}
	return e2
}

func (mi *MMapIndex) NumDocs() int {
	return len(mi.docLens)
}

func (mi *MMapIndex) Terms() int {
	return len(mi.terms)
}

func (mi *MMapIndex) DocLen(id uint32) int {
	if int(id) >= len(mi.docLens) {
		return 0
	}
	return int(mi.docLens[id])
}

func (mi *MMapIndex) df(term string) int {
	ps, err := mi.Postings(term)
	if err != nil {
		return 0
	}
	return len(ps)
}

func (mi *MMapIndex) Postings(term string) ([]posting, error) {
	md, ok := mi.terms[term]
	if !ok {
		return nil, nil
	}
	start := int(md.off)
	end := start + int(md.ln)
	return decodePostings(mi.data[start:end])
}

func (mi *MMapIndex) LookupPostings(term string) ([]posting, error) {
	return mi.Postings(term)
}

// AllDocIDs — [0 .. NumDocs-1] для NOT на mmap-индексе.
func (mi *MMapIndex) AllDocIDs() []uint32 {
	n := mi.NumDocs()
	ids := make([]uint32, n)
	for i := range ids {
		ids[i] = uint32(i)
	}
	return ids
}
