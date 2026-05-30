package ir

import (
	"bytes"
	"encoding/binary"
	"os"
	"testing"
)

// encodePostingsVarint — прежний формат IRIXV1 (delta + uvarint), только для сравнения размеров.
func encodePostingsVarint(ps []posting) []byte {
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

func putUvarint(buf *bytes.Buffer, v uint64) {
	var b [10]byte
	n := binary.PutUvarint(b[:], v)
	buf.Write(b[:n])
}

func postingsPayloadBytes(ix *InvIndex, encode func([]posting) []byte) int64 {
	var n int64
	for _, ps := range ix.postings {
		n += int64(len(encode(ps)))
	}
	return n
}

func TestCompressionVarintVsBitpackSynthetic(t *testing.T) {
	for _, n := range []int{400, 2000} {
		ix := fillCorpus(n, 4242, defaultWords())
		v := postingsPayloadBytes(ix, encodePostingsVarint)
		b := postingsPayloadBytes(ix, encodePostings)
		t.Logf("synthetic N=%d: varint=%d B bitpack=%d B ratio=%.2fx", n, v, b, float64(v)/float64(b))
		if b >= v {
			t.Fatalf("N=%d: bitpack (%d) should be smaller than varint (%d)", n, b, v)
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
	b := postingsPayloadBytes(ix, encodePostings)
	t.Logf("ruwiki N=%d: varint=%d B bitpack=%d B ratio=%.2fx indexed=%d",
		st.PagesIndexed, v, b, float64(v)/float64(b), st.PagesIndexed)
}
