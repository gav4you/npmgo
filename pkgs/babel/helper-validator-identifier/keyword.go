// Copyright (c) 2014-present Sebastian McKenzie and other contributors. MIT License (see LICENSE).
// Translated to Go by the npmgo contributors from @babel/helper-validator-identifier 7.28.5 (lib/keyword.js).

package helpervalidatoridentifier

// reservedWords is keyword.js's table of the same name.
var reservedWords = struct{ keyword, strict, strictBind []string }{
	keyword:    []string{"break", "case", "catch", "continue", "debugger", "default", "do", "else", "finally", "for", "function", "if", "return", "switch", "throw", "try", "var", "const", "while", "with", "new", "this", "super", "class", "extends", "export", "import", "null", "true", "false", "in", "instanceof", "typeof", "void", "delete"},
	strict:     []string{"implements", "interface", "let", "package", "private", "protected", "public", "static", "yield"},
	strictBind: []string{"eval", "arguments"},
}

func newSet(words []string) map[string]bool {
	s := make(map[string]bool, len(words))
	for _, w := range words {
		s[w] = true
	}
	return s
}

var (
	keywords                   = newSet(reservedWords.keyword)
	reservedWordsStrictSet     = newSet(reservedWords.strict)
	reservedWordsStrictBindSet = newSet(reservedWords.strictBind)
)

// IsReservedWord is isReservedWord(word, inModule): "enum", and "await" in a module.
func IsReservedWord(word string, inModule bool) bool {
	return inModule && word == "await" || word == "enum"
}

// IsStrictReservedWord is isStrictReservedWord(word, inModule): a reserved word or a strict-mode reserved word.
func IsStrictReservedWord(word string, inModule bool) bool {
	return IsReservedWord(word, inModule) || reservedWordsStrictSet[word]
}

// IsStrictBindOnlyReservedWord is isStrictBindOnlyReservedWord(word): "eval" or "arguments".
func IsStrictBindOnlyReservedWord(word string) bool {
	return reservedWordsStrictBindSet[word]
}

// IsStrictBindReservedWord is isStrictBindReservedWord(word, inModule): a strict reserved word or a word strict mode
// forbids as a binding name.
func IsStrictBindReservedWord(word string, inModule bool) bool {
	return IsStrictReservedWord(word, inModule) || IsStrictBindOnlyReservedWord(word)
}

// IsKeyword is isKeyword(word): an ECMAScript keyword (including null, true and false).
func IsKeyword(word string) bool {
	return keywords[word]
}
