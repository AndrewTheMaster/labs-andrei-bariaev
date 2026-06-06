package ir

import "testing"

func TestP4DeltaRoundtrip(t *testing.T) {
	vals := []uint32{0, 1, 2, 127, 128, 1000, 50000, 3, 3, 3}
	for i := 0; i < 200; i++ {
		vals = append(vals, uint32(i*7%1000))
	}
	enc := encodePForDelta(vals)
	got, err := decodePForDelta(enc, len(vals))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(vals) {
		t.Fatalf("len %d vs %d", len(got), len(vals))
	}
	for i := range vals {
		if got[i] != vals[i] {
			t.Fatalf("at %d: got %d want %d", i, got[i], vals[i])
		}
	}
}

func TestOptimalStreamRoundtrip(t *testing.T) {
	for _, vals := range [][]uint32{
		{0, 1, 2, 3},
		{0, 100, 200, 300},
		{1, 1, 1, 1, 1},
	} {
		c, p := encodeOptimalStream(vals)
		got, err := decodeStream(c, p, len(vals))
		if err != nil {
			t.Fatal(err)
		}
		for i := range vals {
			if got[i] != vals[i] {
				t.Fatalf("vals=%v at %d got %d", vals, i, got[i])
			}
		}
	}
}

func TestBitpackStreamRoundtrip(t *testing.T) {
	for _, vals := range [][]uint32{
		{0, 1, 2, 3},
		{0, 100, 200, 300},
		{1, 1, 1, 1, 1},
	} {
		c, p := encodeBitpackStream(vals)
		if c != streamBitpack {
			t.Fatalf("codec=%d want bitpack", c)
		}
		got, err := decodeBitpackStream(p, len(vals))
		if err != nil {
			t.Fatal(err)
		}
		for i := range vals {
			if got[i] != vals[i] {
				t.Fatalf("vals=%v at %d got %d", vals, i, got[i])
			}
		}
	}
}

func TestPostingsP4Roundtrip(t *testing.T) {
	ps := []posting{
		{DocID: 0, Poss: []uint32{0, 5, 10}},
		{DocID: 12, Poss: []uint32{1, 2}},
		{DocID: 100, Poss: []uint32{50}},
	}
	raw := encodePostings(ps)
	got, err := decodePostings(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(ps) {
		t.Fatalf("len mismatch")
	}
	for i := range ps {
		if got[i].DocID != ps[i].DocID || len(got[i].Poss) != len(ps[i].Poss) {
			t.Fatalf("posting %d: got %+v want %+v", i, got[i], ps[i])
		}
	}
}
