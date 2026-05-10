package ir

// Node — AST запроса.
type Node interface {
	ast()
}

type Term struct {
	Lex string
}

func (*Term) ast() {}

type Not struct {
	Child Node
}

func (*Not) ast() {}

type And struct {
	Children []Node
}

func (*And) ast() {}

type Or struct {
	Left, Right Node
}

func (*Or) ast() {}

type Near struct {
	K    int
	A, B string
}

func (*Near) ast() {}

// Adj — строгое соседство: B идет сразу после A (эквивалент directed NEAR/1).
type Adj struct {
	A, B string
}

func (*Adj) ast() {}

// MSM — все лексемы встречаются в окне длины ≤ W (по индексам токенов, включительно).
type MSM struct {
	W     int
	Terms []string
}

func (*MSM) ast() {}

// EdgeStart / EdgeEnd — токен на позиции 0 / последней позиции документа.
type EdgeStart struct {
	Lex string
}

func (*EdgeStart) ast() {}

type EdgeEnd struct {
	Lex string
}

func (*EdgeEnd) ast() {}
