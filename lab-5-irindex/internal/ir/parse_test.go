package ir

import "testing"

func TestParseCyrillicTerms(t *testing.T) {
	n, err := Parse(`россия AND город`)
	if err != nil {
		t.Fatal(err)
	}
	and, ok := n.(*And)
	if !ok || len(and.Children) != 2 {
		t.Fatalf("expected AND with 2 children, got %T", n)
	}
	a, ok := and.Children[0].(*Term)
	if !ok || a.Lex != "россия" {
		t.Fatalf("term0: got %#v", and.Children[0])
	}
	b, ok := and.Children[1].(*Term)
	if !ok || b.Lex != "город" {
		t.Fatalf("term1: got %#v", and.Children[1])
	}
}
