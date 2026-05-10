package ir

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"sort"
	"syscall"
)

var fileMagic = [8]byte{'I', 'R', 'I', 'X', 'V', '1', 0, 0}

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

func putUvarint(buf *bytes.Buffer, v uint64) {
	var b [10]byte
	n := binary.PutUvarint(b[:], v)
	buf.Write(b[:n])
}

func encodePostings(ps []posting) []byte {
	var buf bytes.Buffer
	putUvarint(&buf, uint64(len(ps)))
	var prevDoc uint32
	for i, p := range ps {
		if i == 0 {
			putUvarint(&buf, uint64(p.DocID))
		} else {
			putUvarint(&buf, uint64(p.DocID-prevDoc))
		}
		prevDoc = p.DocID
		putUvarint(&buf, uint64(len(p.Poss)))
		var prevPos uint32
		for j, pos := range p.Poss {
			if j == 0 {
				putUvarint(&buf, uint64(pos))
			} else {
				putUvarint(&buf, uint64(pos-prevPos))
			}
			prevPos = pos
		}
	}
	return buf.Bytes()
}

func readUvarintAt(data []byte, i *int) (uint64, error) {
	v, n := binary.Uvarint(data[*i:])
	if n <= 0 {
		return 0, fmt.Errorf("bad varint at %d", *i)
	}
	*i += n
	return v, nil
}

func decodePostings(data []byte) ([]posting, error) {
	i := 0
	nPost, err := readUvarintAt(data, &i)
	if err != nil {
		return nil, err
	}
	out := make([]posting, 0, int(nPost))
	var prevDoc uint32
	for pidx := 0; pidx < int(nPost); pidx++ {
		delta, err := readUvarintAt(data, &i)
		if err != nil {
			return nil, err
		}
		doc := uint32(delta)
		if pidx > 0 {
			doc = prevDoc + uint32(delta)
		}
		prevDoc = doc
		tf, err := readUvarintAt(data, &i)
		if err != nil {
			return nil, err
		}
		poss := make([]uint32, int(tf))
		var prevPos uint32
		for j := 0; j < int(tf); j++ {
			d, err := readUvarintAt(data, &i)
			if err != nil {
				return nil, err
			}
			pos := uint32(d)
			if j > 0 {
				pos = prevPos + uint32(d)
			}
			prevPos = pos
			poss[j] = pos
		}
		out = append(out, posting{DocID: doc, Poss: poss})
	}
	return out, nil
}

// SaveCompressed сохраняет индекс в бинарном формате (delta+varint).
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
