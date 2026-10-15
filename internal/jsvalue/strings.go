package jsvalue

import (
	"strings"
	"sync"
	"unicode/utf8"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// URIError is a JavaScript URIError (encodeURIComponent and
// decodeURIComponent throw one, with V8's message "URI malformed").
type URIError struct{ Message string }

func (e *URIError) Error() string { return e.Message }

// Len16 is a string's JavaScript length: its UTF-16 code units. A byte that
// is not part of valid UTF-8 counts as one unit (the U+FFFD it decodes to).
func Len16(s string) int {
	n := 0
	for _, r := range s {
		n++
		if r > 0xFFFF {
			n++
		}
	}
	return n
}

// uriUnreserved is encodeURIComponent's unescaped set: A-Z a-z 0-9 and
// - _ . ! ~ * ' ( ).
func uriUnreserved(c byte) bool {
	switch {
	case 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z', '0' <= c && c <= '9':
		return true
	}
	return strings.IndexByte("-_.!~*'()", c) >= 0
}

// EncodeURIComponent is the global encodeURIComponent: every character
// outside the unreserved set becomes the %XX escapes of its UTF-8 bytes.
// JavaScript throws a URIError for a string that is not well-formed UTF-16
// (a lone surrogate); a Go string that is not valid UTF-8, the Go form of a
// string no JavaScript string can be, gets the same URIError.
func EncodeURIComponent(s string) (string, error) {
	if !utf8.ValidString(s) {
		return "", &URIError{"URI malformed"}
	}
	const hex = "0123456789ABCDEF"
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if uriUnreserved(c) {
			b.WriteByte(c)
			continue
		}
		b.WriteByte('%')
		b.WriteByte(hex[c>>4])
		b.WriteByte(hex[c&15])
	}
	return b.String(), nil
}

func unhex(c byte) (byte, bool) {
	switch {
	case '0' <= c && c <= '9':
		return c - '0', true
	case 'a' <= c && c <= 'f':
		return c - 'a' + 10, true
	case 'A' <= c && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}

// DecodeURIComponent is the global decodeURIComponent (the spec's Decode with
// an empty reserved set): each run of %XX escapes must spell one UTF-8
// encoded code point (no overlong form, surrogate or value above U+10FFFF),
// or it returns a URIError ("URI malformed") where JavaScript throws one.
// Other characters are copied.
func DecodeURIComponent(s string) (string, error) {
	if strings.IndexByte(s, '%') < 0 {
		return s, nil
	}
	malformed := &URIError{"URI malformed"}
	// octet reads the escape %XX at k.
	octet := func(k int) (byte, bool) {
		if k+2 >= len(s) || s[k] != '%' {
			return 0, false
		}
		hi, ok1 := unhex(s[k+1])
		lo, ok2 := unhex(s[k+2])
		return hi<<4 | lo, ok1 && ok2
	}
	var b strings.Builder
	for k := 0; k < len(s); k++ {
		if s[k] != '%' {
			b.WriteByte(s[k])
			continue
		}
		first, ok := octet(k)
		if !ok {
			return "", malformed
		}
		k += 2
		if first < 0x80 {
			b.WriteByte(first)
			continue
		}
		// n, the number of leading 1 bits, is the length of the sequence.
		n := 0
		for first<<n&0x80 != 0 {
			n++
		}
		if n == 1 || n > 4 {
			return "", malformed
		}
		seq := []byte{first}
		for j := 1; j < n; j++ {
			next, ok := octet(k + 1)
			if !ok || next&0xC0 != 0x80 {
				return "", malformed
			}
			k += 3
			seq = append(seq, next)
		}
		// Go's decoder rejects overlong forms, surrogates and values above
		// U+10FFFF, as the spec requires: it then reads one byte. (It also
		// returns RuneError, with size 3, for the valid encoding of U+FFFD
		// itself, which decodes to U+FFFD.)
		if _, size := utf8.DecodeRune(seq); size != n {
			return "", malformed
		}
		b.Write(seq)
	}
	return b.String(), nil
}

// Casers are stateful, so each goroutine takes its own from a pool.
var (
	upperPool = sync.Pool{New: func() any { c := cases.Upper(language.Und); return &c }}
	lowerPool = sync.Pool{New: func() any { c := cases.Lower(language.Und); return &c }}
)

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

// ToUpperCase is String.prototype.toUpperCase: the full, locale-independent
// Unicode case mapping (ß becomes SS). Non-ASCII text goes through
// golang.org/x/text's tables (Unicode 15.0) after the mappings of
// casemap_supplement.go, which Node's ICU (Unicode 17.0) has and x/text
// lacks: the letters of the case pairs added since. x/text does not know
// those letters as cased, which only the final-sigma rule reads (see
// ToLowerCase).
func ToUpperCase(s string) string {
	if isASCII(s) {
		return strings.ToUpper(s)
	}
	s = supplement(s, upperSupplement)
	c := upperPool.Get().(*cases.Caser)
	defer upperPool.Put(c)
	return c.String(s)
}

// ToLowerCase is String.prototype.toLowerCase: the full, locale-independent
// Unicode case mapping (İ becomes i̇, a final Σ becomes ς), from
// golang.org/x/text for non-ASCII text (Unicode 15.0), after the mappings of
// casemap_supplement.go. A Σ whose only neighbouring cased letter is one of
// the supplemented letters becomes σ here (x/text does not count them as
// cased), where ICU's final-sigma rule gives ς.
func ToLowerCase(s string) string {
	if isASCII(s) {
		return strings.ToLower(s)
	}
	s = supplement(s, lowerSupplement)
	c := lowerPool.Get().(*cases.Caser)
	defer lowerPool.Put(c)
	return c.String(s)
}

// supplement applies the simple mappings of table to s. Nothing else changes:
// the bytes of the other characters, invalid UTF-8 included, are copied, and
// s itself is returned when no character of it is in the table.
func supplement(s string, table map[rune]rune) string {
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if _, ok := table[r]; !ok {
			i += size
			continue
		}
		var b strings.Builder
		b.Grow(len(s))
		b.WriteString(s[:i])
		for i < len(s) {
			r, size := utf8.DecodeRuneInString(s[i:])
			if m, ok := table[r]; ok {
				b.WriteRune(m)
			} else {
				b.WriteString(s[i : i+size])
			}
			i += size
		}
		return b.String()
	}
	return s
}
