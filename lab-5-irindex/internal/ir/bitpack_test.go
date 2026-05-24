package ir

import "testing"

func TestBitpackRoundtrip(t *testing.T) {
	vals := []uint32{0, 1, 3, 7, 100, 65535}
	w, pay := packUint32Stream(vals)
	got, err := unpackUint32Stream(w, pay, len(vals))
	if err != nil {
		t.Fatal(err)
	}
	for i := range vals {
		if got[i] != vals[i] {
			t.Fatalf("i=%d want %d got %d", i, vals[i], got[i])
		}
	}
}

func TestEncodePostingsBitpack(t *testing.T) {
	ps := []posting{
		{DocID: 1, Poss: []uint32{0, 2, 5}},
		{DocID: 4, Poss: []uint32{1}},
	}
	raw := encodePostings(ps)
	got, err := decodePostings(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(ps) {
		t.Fatalf("len %d want %d", len(got), len(ps))
	}
	for i := range ps {
		if got[i].DocID != ps[i].DocID {
			t.Fatalf("doc %d: %d vs %d", i, got[i].DocID, ps[i].DocID)
		}
		if len(got[i].Poss) != len(ps[i].Poss) {
			t.Fatalf("poss len %d", i)
		}
		for j := range ps[i].Poss {
			if got[i].Poss[j] != ps[i].Poss[j] {
				t.Fatalf("pos mismatch doc=%d j=%d", i, j)
			}
		}
	}
}
