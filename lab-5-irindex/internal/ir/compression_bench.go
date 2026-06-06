package ir

import (
	"fmt"
	"os"
	"path/filepath"
)

// WriteCompressionMetrics — posting payload (КБ): V1 varint, V2 bitpack, V3 p4+bp, V4 p4+opt.
func WriteCompressionMetrics(outPath string) error {
	type row struct {
		corpus          string
		n               int
		v1, v2, v3, v4  int64
	}
	var rows []row
	for _, n := range []int{400, 2000} {
		ix := fillCorpus(n, 4242, defaultWords())
		rows = append(rows, row{"synthetic", n,
			postingsPayloadBytes(ix, encodePostingsVarint),
			postingsPayloadBytes(ix, encodePostingsBitpackAll),
			postingsPayloadBytes(ix, encodePostingsP4Bitpack),
			postingsPayloadBytes(ix, encodePostings),
		})
	}
	path := ResolveCorpusPath()
	if path != "" {
		if _, err := os.Stat(path); err == nil {
			ix, st, err := BuildIndexFromWikiXML(path, CorpusOpts{MaxDocs: 20000})
			if err != nil {
				return err
			}
			rows = append(rows, row{"ruwiki", st.PagesIndexed,
				postingsPayloadBytes(ix, encodePostingsVarint),
				postingsPayloadBytes(ix, encodePostingsBitpackAll),
				postingsPayloadBytes(ix, encodePostingsP4Bitpack),
				postingsPayloadBytes(ix, encodePostings),
			})
		}
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()
	fmt.Fprintln(f, "corpus\tn\tv1_varint_post_kb\tv2_bitpack_post_kb\tv3_p4_bitpack_post_kb\tv4_p4_opt_post_kb")
	for _, r := range rows {
		fmt.Fprintf(f, "%s\t%d\t%d\t%d\t%d\t%d\n",
			r.corpus, r.n, r.v1/1024, r.v2/1024, r.v3/1024, r.v4/1024)
	}
	return nil
}

func postingsPayloadBytes(ix *InvIndex, encode postingsEncoder) int64 {
	var n int64
	for _, ps := range ix.postings {
		n += int64(len(encode(ps)))
	}
	return n
}

func irxFileBytes(ix *InvIndex, encode postingsEncoder) (int64, error) {
	dir, err := os.MkdirTemp("", "irx-bench-*")
	if err != nil {
		return 0, err
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "idx.irx")
	if err := saveIndexFile(ix, path, encode); err != nil {
		return 0, err
	}
	st, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return st.Size(), nil
}
