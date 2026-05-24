package ir

import (
	"path/filepath"
	"testing"
)

func TestBuildIndexFromWikiXMLSample(t *testing.T) {
	path := filepath.Join("testdata", "sample_wiki.xml")
	ix, st, err := BuildIndexFromWikiXML(path, CorpusOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if st.PagesIndexed != 3 {
		t.Fatalf("pages indexed: got %d want 3", st.PagesIndexed)
	}
	if ix.NumDocs() != 3 {
		t.Fatalf("docs: got %d want 3", ix.NumDocs())
	}
	if ix.df("alpha") != 3 {
		t.Fatalf("df(alpha)=%d want 3", ix.df("alpha"))
	}
}
