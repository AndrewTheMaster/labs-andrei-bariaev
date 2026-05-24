package ir

import "testing"

func TestTokenizeCyrillic(t *testing.T) {
	got := Tokenize("Россия — город Москва")
	want := []string{"россия", "город", "москва"}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("i=%d got %q want %q (all %v)", i, got[i], want[i], got)
		}
	}
}
