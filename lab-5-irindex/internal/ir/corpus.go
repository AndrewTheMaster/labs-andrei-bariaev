package ir

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"strings"
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

// BuildIndexFromWikiXML строит индекс потоковым xml.Decoder (encoding/xml, без самописного разбора тегов).
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

	for {
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
		if opt.MaxDocs > 0 && st.PagesIndexed >= opt.MaxDocs {
			break
		}
		text := strings.TrimSpace(page.Text)
		if text == "" {
			continue
		}
		ix.Add(Tokenize(text))
		st.PagesIndexed++
	}
	return ix, st, nil
}

type wikiPageXML struct {
	Title string `xml:"title"`
	Text  string `xml:"revision>text"`
}

// ResolveCorpusPath: CORPUS_XML / WIKI_XML / data/ruwiki-latest-pages-articles.xml.
func ResolveCorpusPath() string {
	for _, k := range []string{"CORPUS_XML", "WIKI_XML"} {
		if p := strings.TrimSpace(os.Getenv(k)); p != "" {
			return p
		}
	}
	return DefaultWikiPath
}
