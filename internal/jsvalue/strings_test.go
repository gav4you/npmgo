package jsvalue

import "testing"

// Expected values were produced by V8 (node v25.9.0). In review the helpers
// also matched V8 on toUpperCase / toLowerCase of every code point except
// the 55 + 55 whose case mappings were added in Unicode 16 and 17 (x/text
// has Unicode 15.0), on 32 contextual strings, and on 3,000
// encodeURIComponent and 6,000 decodeURIComponent inputs.

func TestCaseMapping(t *testing.T) {
	for _, c := range []struct{ in, upper, lower string }{
		{"ß", "SS", "ß"},
		{"ŉ", "ʼN", "ŉ"},
		{"ﬁ", "FI", "ﬁ"},
		{"ı", "I", "ı"},
		{"İ", "İ", "i̇"},
		{"ǰ", "J̌", "ǰ"},
		{"ΐ", "Ϊ́", "ΐ"},
		{"ΑΣ", "ΑΣ", "ας"},
		{"ΑΣ Α", "ΑΣ Α", "ας α"},
		{"Σ", "Σ", "σ"},
		{"AΣb", "AΣB", "aσb"},
		{"ΑΣΑ", "ΑΣΑ", "ασα"},
		{"ẞ", "ẞ", "ß"},
		{"Straße", "STRASSE", "straße"},
		{"TRUE", "TRUE", "true"},
		{"ǅ", "Ǆ", "ǆ"},
		{"𐐀", "𐐀", "𐐨"},
	} {
		if got := ToUpperCase(c.in); got != c.upper {
			t.Errorf("%q.toUpperCase() = %q, want %q", c.in, got, c.upper)
		}
		if got := ToLowerCase(c.in); got != c.lower {
			t.Errorf("%q.toLowerCase() = %q, want %q", c.in, got, c.lower)
		}
	}
}

func TestURIComponent(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"a b;c=d,é€😀!~*'()", "a%20b%3Bc%3Dd%2C%C3%A9%E2%82%AC%F0%9F%98%80!~*'()"},
		{"", ""},
		{"%", "%25"},
		{"\x00\x7f", "%00%7F"},
		{"\U0010FFFF", "%F4%8F%BF%BF"},
	} {
		if got, err := EncodeURIComponent(c.in); err != nil || got != c.want {
			t.Errorf("encodeURIComponent(%q) = %q, %v; want %q", c.in, got, err, c.want)
		}
	}
	if _, err := EncodeURIComponent("a\xed\xa0\x80"); err == nil || err.Error() != "URI malformed" {
		t.Errorf("encodeURIComponent of invalid UTF-8: %v", err)
	}
	for _, c := range []struct{ in, want string }{
		{"%41%42", "AB"},
		{"%e2%82%ac", "€"},
		{"%F0%9F%98%80x", "😀x"},
		{"%25", "%"},
		{"é%20", "é "},
		{"no escapes", "no escapes"},
		{"%3b%3B", ";;"},
		{"%EF%BF%BD", "�"}, // the valid encoding of U+FFFD (utf8.DecodeRune reports it as RuneError)
		{"k%EF%BF%BDv", "k�v"},
	} {
		if got, err := DecodeURIComponent(c.in); err != nil || got != c.want {
			t.Errorf("decodeURIComponent(%q) = %q, %v; want %q", c.in, got, err, c.want)
		}
	}
	for _, in := range []string{"%", "%2", "%zz", "%C0%80", "%ED%A0%80", "%F4%90%80%80", "%E2%82", "%80", "%E2%82x", "%F8%80%80%80%80", "%E2%82%2C", "a%"} {
		if got, err := DecodeURIComponent(in); err == nil || err.Error() != "URI malformed" {
			t.Errorf("decodeURIComponent(%q) = %q, %v; want URIError", in, got, err)
		}
	}
}

func TestLen16(t *testing.T) {
	for _, c := range []struct {
		in   string
		want int
	}{{"", 0}, {"abc", 3}, {"é", 1}, {"€", 1}, {"😀", 2}, {"a😀b", 4}, {"\xff", 1}} {
		if got := Len16(c.in); got != c.want {
			t.Errorf("Len16(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}
