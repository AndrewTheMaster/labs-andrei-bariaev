package ir

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Tokenize — токены [буквы/цифры]+ в lower case (UTF-8, в т.ч. кириллица).
func Tokenize(text string) []string {
	var out []string
	i := 0
	for i < len(text) {
		r, size := utf8.DecodeRuneInString(text[i:])
		if !isTokRune(r) {
			i += size
			continue
		}
		j := i + size
		for j < len(text) {
			r2, sz := utf8.DecodeRuneInString(text[j:])
			if !isTokRune(r2) {
				break
			}
			j += sz
		}
		out = append(out, strings.ToLower(text[i:j]))
		i = j
	}
	return out
}

func isTokRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}
