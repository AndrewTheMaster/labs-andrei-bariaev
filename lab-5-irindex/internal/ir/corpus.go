package ir

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// CorpusOpts параметры загрузки дампа Wikipedia (распакованный XML).
type CorpusOpts struct {
	MaxDocs int // 0 = все страницы
}

// CorpusStats результат сборки корпуса.
type CorpusStats struct {
	PagesSeen    int
	PagesIndexed int
	BuildSeconds float64
}

// DefaultWikiPath — ожидаемый путь после скачивания в репозиторий.
const DefaultWikiPath = "data/ruwiki-latest-pages-articles.xml"

// BuildIndexFromWikiXML строит индекс потоковым xml.Decoder (encoding/xml).
func BuildIndexFromWikiXML(path string, opt CorpusOpts) (*InvIndex, CorpusStats, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, CorpusStats{}, err
	}
	defer f.Close()

	ix := NewIndex()
	var st CorpusStats
	dec := xml.NewDecoder(f)
	dec.Strict = false

	t0 := time.Now()
	progressEvery := 5000
	if v := os.Getenv("WIKI_PROGRESS"); v != "" {
		if n, err := fmt.Sscanf(v, "%d", &progressEvery); err != nil || n != 1 || progressEvery <= 0 {
			progressEvery = 5000
		}
	}

	for {
		if opt.MaxDocs > 0 && st.PagesIndexed >= opt.MaxDocs {
			break
		}
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, st, err
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "page" {
			continue
		}
		st.PagesSeen++
		var page wikiPageXML
		if err := dec.DecodeElement(&page, &se); err != nil {
			return nil, st, fmt.Errorf("decode page %d: %w", st.PagesSeen, err)
		}
		text := strings.TrimSpace(page.Text)
		if text == "" {
			continue
		}
		ix.AddLean(Tokenize(text))
		st.PagesIndexed++
		if progressEvery > 0 && st.PagesIndexed%progressEvery == 0 {
			fmt.Fprintf(os.Stderr, "wiki: indexed %d docs (seen %d pages) %s\n",
				st.PagesIndexed, st.PagesSeen, time.Since(t0).Round(time.Millisecond))
		}
	}
	st.BuildSeconds = time.Since(t0).Seconds()
	return ix, st, nil
}

type wikiPageXML struct {
	Title string `xml:"title"`
	Text  string `xml:"revision>text"`
}

// ResolveCorpusPath: CORPUS_XML / WIKI_XML / data/… / ../ruwiki-latest-pages-articles.xml.
func ResolveCorpusPath() string {
	for _, k := range []string{"CORPUS_XML", "WIKI_XML"} {
		if p := strings.TrimSpace(os.Getenv(k)); p != "" {
			return p
		}
	}
	for _, p := range []string{
		DefaultWikiPath,
		"../ruwiki-latest-pages-articles.xml",
	} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return DefaultWikiPath
}
