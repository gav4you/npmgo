package helpervalidatoridentifier

import (
	"encoding/json"
	"math"
	"os"
	"strconv"
	"testing"
)

const (
	casesPath    = "../../../parity/babel-types/cases.json"
	expectedPath = "../../../parity/babel-types/expected.json"
)

// TestParity diffs @babel/helper-validator-identifier 7.28.5, as parity/oracle_babel_types.cjs records it, against the
// port: isIdentifierStart and isIdentifierChar on every code point (as ranges) and on numbers that are not code
// points, isIdentifierName on strings, and the keyword predicates on words.
func TestParity(t *testing.T) {
	var cases struct {
		Identifier struct {
			Special []json.RawMessage
			Names   []string
			Words   []string
		}
	}
	var expected struct {
		Identifier struct {
			Start   [][2]int
			Char    [][2]int
			Special []string
			Names   string
			Words   []string
		}
	}
	load := func(path string, v any) {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, v); err != nil {
			t.Fatal(err)
		}
	}
	load(casesPath, &cases)
	load(expectedPath, &expected)
	checks := 0

	ranges := func(pred func(float64) bool) [][2]int {
		var out [][2]int
		start := -1
		for c := 0; c <= 0x110000; c++ {
			yes := c <= 0x10ffff && pred(float64(c))
			if yes && start < 0 {
				start = c
			}
			if !yes && start >= 0 {
				out = append(out, [2]int{start, c - 1})
				start = -1
			}
		}
		return out
	}
	for _, x := range []struct {
		name string
		pred func(float64) bool
		want [][2]int
	}{{"isIdentifierStart", IsIdentifierStart, expected.Identifier.Start}, {"isIdentifierChar", IsIdentifierChar, expected.Identifier.Char}} {
		got := ranges(x.pred)
		checks += 0x110000
		if len(got) != len(x.want) {
			t.Errorf("%s: %d ranges, want %d", x.name, len(got), len(x.want))
		}
		for i := 0; i < len(got) && i < len(x.want); i++ {
			if got[i] != x.want[i] {
				t.Errorf("%s: range %d is %v, want %v", x.name, i, got[i], x.want[i])
				break
			}
		}
	}

	for i, raw := range cases.Identifier.Special {
		v := number(t, raw)
		got := bit(IsIdentifierStart(v)) + bit(IsIdentifierChar(v))
		checks++
		if got != expected.Identifier.Special[i] {
			t.Errorf("special %v: %s, want %s", v, got, expected.Identifier.Special[i])
		}
	}
	got := ""
	for _, n := range cases.Identifier.Names {
		got += bit(IsIdentifierName(n))
		checks++
	}
	if got != expected.Identifier.Names {
		t.Errorf("isIdentifierName: %s\nwant %s", got, expected.Identifier.Names)
	}
	for i, w := range cases.Identifier.Words {
		got := bit(IsKeyword(w)) + bit(IsReservedWord(w, false)) + bit(IsReservedWord(w, true)) +
			bit(IsStrictReservedWord(w, false)) + bit(IsStrictReservedWord(w, true)) + bit(IsStrictBindOnlyReservedWord(w)) +
			bit(IsStrictBindReservedWord(w, false)) + bit(IsStrictBindReservedWord(w, true))
		checks += 8
		if got != expected.Identifier.Words[i] {
			t.Errorf("word %q: %s, want %s", w, got, expected.Identifier.Words[i])
		}
	}
	t.Logf("%d checks", checks)
}

func bit(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// number reads a number of the value-spec language: a JSON number or {"$":"num","v":...}.
func number(t *testing.T, raw json.RawMessage) float64 {
	var f float64
	if json.Unmarshal(raw, &f) == nil {
		return f
	}
	var spec struct{ V string }
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatal(err)
	}
	switch spec.V {
	case "NaN":
		return math.NaN()
	case "Infinity":
		return math.Inf(1)
	case "-Infinity":
		return math.Inf(-1)
	case "-0":
		return math.Copysign(0, -1)
	}
	v, err := strconv.ParseFloat(spec.V, 64)
	if err != nil {
		t.Fatal(err)
	}
	return v
}
