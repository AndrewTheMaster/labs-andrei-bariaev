package ir

import (
	"os"
	"testing"
)

func logCompressionRow(t *testing.T, corpus string, n int, v1, v2, v3, v4 int64) {
	t.Helper()
	t.Logf("%s N=%d postings KB: V1=%d V2=%d V3_p4bp=%d V4_p4opt=%d",
		corpus, n, v1/1024, v2/1024, v3/1024, v4/1024)
}

func postPayload(ix *InvIndex, enc postingsEncoder) int64 {
	return postingsPayloadBytes(ix, enc)
}

func TestCompressionFileSizesSynthetic(t *testing.T) {
	for _, n := range []int{400, 2000} {
		ix := fillCorpus(n, 4242, defaultWords())
		logCompressionRow(t, "synthetic", n,
			postPayload(ix, encodePostingsVarint),
			postPayload(ix, encodePostingsBitpackAll),
			postPayload(ix, encodePostingsP4Bitpack),
			postPayload(ix, encodePostings),
		)
	}
}

func TestCompressionWikiScale(t *testing.T) {
	if os.Getenv("WIKI_SCALE_BENCH") == "" {
		t.Skip("set WIKI_SCALE_BENCH=1")
	}
	path := ResolveCorpusPath()
	if path == "" {
		t.Skip("no wiki xml")
	}
	if _, err := os.Stat(path); err != nil {
		t.Skip("wiki xml missing")
	}
	for _, max := range []int{20000, 50000, 100000} {
		ix, st, err := BuildIndexFromWikiXML(path, CorpusOpts{MaxDocs: max})
		if err != nil {
			t.Fatal(err)
		}
		v2 := postPayload(ix, encodePostingsBitpackAll)
		v3 := postPayload(ix, encodePostings)
		p4d, bpd, pw, bw, _ := AnalyzeDocCodecBreakdown(ix)
		t.Logf("N=%d terms=%d: post V2=%d KB V3=%d KB V3/V2=%.4f docP4=%d KB docBP=%d KB p4wins=%d bpwins=%d",
			st.PagesIndexed, len(ix.postings), v2/1024, v3/1024, float64(v3)/float64(v2),
			p4d/1024, bpd/1024, pw, bw)
	}
}

func TestCompressionFileSizesWiki(t *testing.T) {
	if os.Getenv("WIKI_COMPRESS_BENCH") == "" {
		t.Skip("set WIKI_COMPRESS_BENCH=1")
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
	logCompressionRow(t, "ruwiki", st.PagesIndexed,
		postPayload(ix, encodePostingsVarint),
		postPayload(ix, encodePostingsBitpackAll),
		postPayload(ix, encodePostingsP4Bitpack),
		postPayload(ix, encodePostings),
	)
	p4d, bpd, pw, bw, tie := AnalyzeDocCodecBreakdown(ix)
	t.Logf("doc Δ only: PForDelta=%d KB bitpack=%d KB (delta %+d KB); lists: p4 wins=%d bp wins=%d tie=%d",
		p4d/1024, bpd/1024, (p4d-bpd)/1024, pw, bw, tie)
}
