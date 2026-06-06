package ir

import (
	"bytes"
	"encoding/binary"
	"os"
	"testing"
)

func putUvarintLegacy(buf *bytes.Buffer, v uint64) {
	var b [10]byte
	n := binary.PutUvarint(b[:], v)
	buf.Write(b[:n])
}

// encodePostingsVarint — IRIXV1 (delta + uvarint), для сравнения размеров.
func encodePostingsVarint(ps []posting) []byte {
	var buf bytes.Buffer
	putUvarintLegacy(&buf, uint64(len(ps)))
	var prevDoc uint32
	for i, p := range ps {
		if i == 0 {
			putUvarintLegacy(&buf, uint64(p.DocID))
		} else {
			putUvarintLegacy(&buf, uint64(p.DocID-prevDoc))
		}
		prevDoc = p.DocID
		putUvarintLegacy(&buf, uint64(len(p.Poss)))
		var prevPos uint32
		for j, pos := range p.Poss {
			if j == 0 {
				putUvarintLegacy(&buf, uint64(pos))
			} else {
				putUvarintLegacy(&buf, uint64(pos-prevPos))
			}
			prevPos = pos
		}
	}
	return buf.Bytes()
}

func encodePostingsBitpackLegacy(ps []posting) []byte {
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

func postingsPayloadBytes(ix *InvIndex, encode func([]posting) []byte) int64 {
	var n int64
	for _, ps := range ix.postings {
		n += int64(len(encode(ps)))
	}
	return n
}

func TestCompressionFormatsSynthetic(t *testing.T) {
	for _, n := range []int{400, 2000} {
		ix := fillCorpus(n, 4242, defaultWords())
		v := postingsPayloadBytes(ix, encodePostingsVarint)
		bp := postingsPayloadBytes(ix, encodePostingsBitpackLegacy)
		p4 := postingsPayloadBytes(ix, encodePostings)
		t.Logf("synthetic N=%d: varint=%d B bitpack=%d B p4+opt=%d B", n, v, bp, p4)
		if p4 >= v && n == 400 {
			t.Logf("note: p4+opt may be larger than varint on some corpora")
		}
	}
}

func TestCompressionVarintVsBitpackWiki(t *testing.T) {
	if os.Getenv("WIKI_COMPRESS_BENCH") == "" {
		t.Skip("set WIKI_COMPRESS_BENCH=1 to compare on ruwiki (slow)")
	}
	path := ResolveCorpusPath()
	if path == "" {
		t.Skip("no wiki xml")
	}
	if _, err := os.Stat(path); err != nil {
		t.Skip("wiki xml missing")
	}
	ix, st, err := BuildIndexFromWikiXML(path, CorpusOpts{MaxDocs: 20000})
	if err != nil {
		t.Fatal(err)
	}
	v := postingsPayloadBytes(ix, encodePostingsVarint)
	p4 := postingsPayloadBytes(ix, encodePostings)
	t.Logf("ruwiki N=%d: varint=%d B p4+opt=%d B ratio=%.2fx indexed=%d",
		st.PagesIndexed, v, p4, float64(v)/float64(p4), st.PagesIndexed)
}
