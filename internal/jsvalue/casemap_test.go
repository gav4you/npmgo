package jsvalue

import (
	"encoding/json"
	"os"
	"testing"
	"unicode"
)

// TestCaseMappingParity checks ToUpperCase and ToLowerCase of every code point against Node's ICU, as recorded in
// parity/jsvalue/casemap.json by tools/gen_casemap.cjs (a code point that is not listed maps to itself).
func TestCaseMappingParity(t *testing.T) {
	raw, err := os.ReadFile("../../parity/jsvalue/casemap.json")
	if err != nil {
		t.Fatal(err)
	}
	var rec struct {
		Node, ICU, Unicode string
		Mappings           [][3]json.RawMessage
	}
	if err := json.Unmarshal(raw, &rec); err != nil {
		t.Fatal(err)
	}
	type mapping struct{ upper, lower string }
	want := map[rune]mapping{}
	for _, m := range rec.Mappings {
		var cp rune
		if err := json.Unmarshal(m[0], &cp); err != nil {
			t.Fatal(err)
		}
		forms := [2]string{}
		for i := range forms {
			if string(m[i+1]) != "null" {
				var out []rune
				if err := json.Unmarshal(m[i+1], &out); err != nil {
					t.Fatal(err)
				}
				forms[i] = string(out)
			}
		}
		want[cp] = mapping{forms[0], forms[1]}
	}
	bad := 0
	report := func(format string, args ...any) {
		bad++
		if bad <= 20 {
			t.Errorf(format, args...)
		}
	}
	for cp := rune(0); cp <= unicode.MaxRune; cp++ {
		if cp >= 0xD800 && cp <= 0xDFFF {
			continue
		}
		ch := string(cp)
		wu, wl := ch, ch
		if m, ok := want[cp]; ok {
			if m.upper != "" {
				wu = m.upper
			}
			if m.lower != "" {
				wl = m.lower
			}
		}
		if got := ToUpperCase(ch); got != wu {
			report("U+%04X.toUpperCase() = %U, Node %U", cp, []rune(got), []rune(wu))
		}
		if got := ToLowerCase(ch); got != wl {
			report("U+%04X.toLowerCase() = %U, Node %U", cp, []rune(got), []rune(wl))
		}
	}
	if bad > 20 {
		t.Errorf("%d code points differ in all", bad)
	}
	t.Logf("%d code points with a case mapping (Node %s, ICU %s, Unicode %s)", len(want), rec.Node, rec.ICU, rec.Unicode)
}
