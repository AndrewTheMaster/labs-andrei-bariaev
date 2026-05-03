package ir

import (
	"fmt"
	"strings"
	"unicode"
)

type tokenKind int

const (
	tkEOF tokenKind = iota
	tkIdent
	tkInt
	tkLP
	tkRP
	tkComma
	tkAND
	tkOR
	tkNOT
	tkNEAR
	tkMSM
	tkFIRST
	tkLAST // границы документа (edge): LAST(w), синоним edge_end(w); FIRST(w), синоним edge_start(w)
)

type token struct {
	kind tokenKind
	lit  string
	ival int
}

type lexer struct {
	s   string
	i   int
	err error
	tok token
}

func (l *lexer) next() {
	if l.err != nil {
		return
	}
	l.skipSpace()
	if l.i >= len(l.s) {
		l.tok = token{kind: tkEOF}
		return
	}
	ch := l.s[l.i]
	switch ch {
	case '(':
		l.i++
		l.tok = token{kind: tkLP}
		return
	case ')':
		l.i++
		l.tok = token{kind: tkRP}
		return
	case ',':
		l.i++
		l.tok = token{kind: tkComma}
		return
	}
	if unicode.IsDigit(rune(ch)) {
		j := l.i
		for j < len(l.s) && unicode.IsDigit(rune(l.s[j])) {
			j++
		}
		var v int
		for k := l.i; k < j; k++ {
			v = v*10 + int(l.s[k]-'0')
		}
		l.i = j
		l.tok = token{kind: tkInt, ival: v}
		return
	}
	if unicode.IsLetter(rune(ch)) || ch == '_' {
		j := l.i
		for j < len(l.s) && (unicode.IsLetter(rune(l.s[j])) || unicode.IsDigit(rune(l.s[j])) || l.s[j] == '_') {
			j++
		}
		lit := strings.ToLower(l.s[l.i:j])
		l.i = j
		switch lit {
		case "and":
			l.tok = token{kind: tkAND, lit: lit}
		case "or":
			l.tok = token{kind: tkOR, lit: lit}
		case "not":
			l.tok = token{kind: tkNOT, lit: lit}
		case "near":
			l.tok = token{kind: tkNEAR, lit: lit}
		case "msm":
			l.tok = token{kind: tkMSM, lit: lit}
		case "first", "edge_start":
			l.tok = token{kind: tkFIRST, lit: lit}
		case "last", "edge_end":
			l.tok = token{kind: tkLAST, lit: lit}
		default:
			l.tok = token{kind: tkIdent, lit: lit}
		}
		return
	}
	l.err = fmt.Errorf("unexpected char %q", ch)
	l.tok = token{kind: tkEOF}
}

func (l *lexer) skipSpace() {
	for l.i < len(l.s) && (l.s[l.i] == ' ' || l.s[l.i] == '\t' || l.s[l.i] == '\n' || l.s[l.i] == '\r') {
		l.i++
	}
}

type parser struct {
	lex *lexer
	err error
}

func (p *parser) failf(f string, args ...any) Node {
	if p.err == nil {
		p.err = fmt.Errorf(f, args...)
	}
	return &Term{Lex: "__error__"}
}

func (p *parser) expect(kind tokenKind, msg string) {
	if p.lex.tok.kind != kind {
		p.err = fmt.Errorf("%s: got %v want %v", msg, p.lex.tok.kind, kind)
	}
}

func (p *parser) parsePrimary() Node {
	if p.lex.err != nil {
		return &Term{Lex: "__error__"}
	}
	switch p.lex.tok.kind {
	case tkIdent:
		lit := p.lex.tok.lit
		p.lex.next()
		return &Term{Lex: lit}
	case tkFIRST:
		p.lex.next()
		if p.lex.tok.kind != tkLP {
			return p.failf("FIRST ждёт '('")
		}
		p.lex.next()
		if p.lex.tok.kind != tkIdent {
			return p.failf("FIRST ждёт идентификатор")
		}
		lit := p.lex.tok.lit
		p.lex.next()
		if p.lex.tok.kind != tkRP {
			return p.failf("FIRST ждёт ')'")
		}
		p.lex.next()
		return &EdgeStart{Lex: lit}
	case tkLAST:
		p.lex.next()
		if p.lex.tok.kind != tkLP {
			return p.failf("LAST ждёт '('")
		}
		p.lex.next()
		if p.lex.tok.kind != tkIdent {
			return p.failf("LAST ждёт идентификатор")
		}
		lit := p.lex.tok.lit
		p.lex.next()
		if p.lex.tok.kind != tkRP {
			return p.failf("LAST ждёт ')'")
		}
		p.lex.next()
		return &EdgeEnd{Lex: lit}
	case tkNEAR:
		p.lex.next()
		if p.lex.tok.kind != tkLP {
			return p.failf("NEAR ждёт '('")
		}
		p.lex.next()
		if p.lex.tok.kind != tkInt {
			return p.failf("NEAR ждёт целое окно k")
		}
		k := p.lex.tok.ival
		p.lex.next()
		if p.lex.tok.kind != tkComma {
			return p.failf("NEAR ждёт ',' после k")
		}
		p.lex.next()
		if p.lex.tok.kind != tkIdent {
			return p.failf("NEAR: первый термин")
		}
		a := p.lex.tok.lit
		p.lex.next()
		if p.lex.tok.kind != tkComma {
			return p.failf("NEAR ждёт ',' между терминами")
		}
		p.lex.next()
		if p.lex.tok.kind != tkIdent {
			return p.failf("NEAR: второй термин")
		}
		b := p.lex.tok.lit
		p.lex.next()
		if p.lex.tok.kind != tkRP {
			return p.failf("NEAR ждёт ')'")
		}
		p.lex.next()
		return &Near{K: k, A: a, B: b}
	case tkMSM:
		p.lex.next()
		if p.lex.tok.kind != tkLP {
			return p.failf("MSM ждёт '('")
		}
		p.lex.next()
		if p.lex.tok.kind != tkInt {
			return p.failf("MSM ждёт окно W")
		}
		W := p.lex.tok.ival
		p.lex.next()
		if p.lex.tok.kind != tkComma {
			return p.failf("MSM ждёт ',' после W")
		}
		p.lex.next()
		var terms []string
		if p.lex.tok.kind != tkIdent {
			return p.failf("MSM: минимум один термин")
		}
		terms = append(terms, p.lex.tok.lit)
		p.lex.next()
		for p.lex.tok.kind == tkComma {
			p.lex.next()
			if p.lex.tok.kind != tkIdent {
				return p.failf("MSM: ожидался термин после ','")
			}
			terms = append(terms, p.lex.tok.lit)
			p.lex.next()
		}
		if p.lex.tok.kind != tkRP {
			return p.failf("MSM ждёт ')'")
		}
		p.lex.next()
		return &MSM{W: W, Terms: terms}
	case tkLP:
		p.lex.next()
		n := p.parseOrExpr()
		if p.lex.tok.kind != tkRP {
			p.err = fmt.Errorf("ожидалась ')'")
			return n
		}
		p.lex.next()
		return n
	default:
		p.err = fmt.Errorf("неожиданное начало первичного выражения %v", p.lex.tok.kind)
		return &Term{Lex: "__error__"}
	}
}

func (p *parser) parseUnary() Node {
	if p.lex.err != nil {
		return &Term{Lex: "__error__"}
	}
	if p.lex.tok.kind == tkNOT {
		p.lex.next()
		child := p.parseUnary()
		return &Not{Child: child}
	}
	return p.parsePrimary()
}

func (p *parser) parseAndExpr() Node {
	if p.lex.err != nil {
		return &Term{Lex: "__error__"}
	}
	first := p.parseUnary()
	if p.err != nil {
		return first
	}
	var parts []Node
	parts = append(parts, first)
	for p.lex.tok.kind == tkAND {
		p.lex.next()
		parts = append(parts, p.parseUnary())
		if p.err != nil {
			return parts[0]
		}
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return &And{Children: parts}
}

func (p *parser) parseOrExpr() Node {
	if p.lex.err != nil {
		return &Term{Lex: "__error__"}
	}
	left := p.parseAndExpr()
	if p.err != nil {
		return left
	}
	for p.lex.tok.kind == tkOR {
		p.lex.next()
		right := p.parseAndExpr()
		if p.err != nil {
			return left
		}
		left = &Or{Left: left, Right: right}
	}
	return left
}

// Parse разбирает запрос; ключевые слова case-insensitive через лексер.
func Parse(q string) (Node, error) {
	l := &lexer{s: strings.TrimSpace(q)}
	l.next()
	if l.err != nil {
		return nil, l.err
	}
	p := &parser{lex: l}
	n := p.parseOrExpr()
	if l.err != nil {
		return nil, l.err
	}
	if p.err != nil {
		return nil, p.err
	}
	if l.tok.kind != tkEOF {
		return nil, fmt.Errorf("лишние токены после выражения")
	}
	return n, nil
}
