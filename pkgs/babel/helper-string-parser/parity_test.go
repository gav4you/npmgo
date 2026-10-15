package helperstringparser

import (
	"encoding/json"
	"math"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/gav4you/npmgo/internal/jsnum"
	"github.com/gav4you/npmgo/internal/jsvalue"
)

const (
	casesPath    = "../../../parity/babel-types/cases.json"
	expectedPath = "../../../parity/babel-types/expected.json"
)

// loneSurrogatesAsFFFD replaces each lone surrogate in an encoded result (JSON.stringify writes one as the escape
// \udXXX; Go strings cannot hold one, the port gives U+FFFD in its place) by U+FFFD, and reports whether it did. A
// pair of surrogates is never escaped by JSON.stringify, and an escaped backslash before "u" is not an escape.
func loneSurrogatesAsFFFD(s string) (string, bool) {
	var b strings.Builder
	changed := false
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			b.WriteByte(s[i])
			continue
		}
		if s[i+1] == 'u' && i+6 <= len(s) {
			if v, err := strconv.ParseUint(s[i+2:i+6], 16, 16); err == nil && v >= 0xd800 && v <= 0xdfff {
				b.WriteString("�")
				changed = true
				i += 5
				continue
			}
		}
		b.WriteString(s[i : i+2])
		i++
	}
	return b.String(), changed
}

// TestParity diffs @babel/helper-string-parser 7.27.1, as parity/oracle_babel_types.cjs records it, against the port:
// each case's calls to the errors object and its result or exception. A result that holds a lone surrogate is
// compared with U+FFFD in its place (Go strings cannot hold one; the README of the port documents it).
func TestParity(t *testing.T) {
	var cases struct {
		StringParser [][]json.RawMessage
	}
	var expected struct {
		StringParser [][]string
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
	mismatches, replaced := 0, 0
	for i, c := range cases.StringParser {
		got := runCase(t, c)
		want := append([]string(nil), expected.StringParser[i]...)
		for j, line := range want {
			if fixed, changed := loneSurrogatesAsFFFD(line); changed {
				replaced++
				want[j] = fixed
			}
		}
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			mismatches++
			if mismatches <= 20 {
				t.Errorf("case %d %s\nwant %q\n got %q", i, raw(c), want, got)
			}
		}
	}
	t.Logf("%d cases, %d mismatches, %d results with a lone surrogate compared as U+FFFD", len(cases.StringParser), mismatches, replaced)
}

func raw(c []json.RawMessage) string {
	parts := make([]string, len(c))
	for i, r := range c {
		parts[i] = string(r)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func runCase(t *testing.T, c []json.RawMessage) (trace []string) {
	var kind string
	json.Unmarshal(c[0], &kind)
	str := func(i int) string {
		var s string
		if err := json.Unmarshal(c[i], &s); err != nil {
			t.Fatal(err)
		}
		return s
	}
	num := func(i int) int {
		var n int
		if err := json.Unmarshal(c[i], &n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	flag := func(i int) bool {
		var b bool
		json.Unmarshal(c[i], &b)
		var n int
		if json.Unmarshal(c[i], &n) == nil {
			return n != 0
		}
		return b
	}
	throwing := false
	switch kind {
	case "c":
		throwing = flag(6)
	case "i":
		throwing = flag(9)
	}
	unterminatedCalls := 0
	record := func(name string, args ...int) {
		parts := make([]string, len(args))
		for i, a := range args {
			parts[i] = strconv.Itoa(a)
		}
		trace = append(trace, name+"("+strings.Join(parts, ",")+")")
	}
	handler := func(name string) func(int, int, int) {
		return func(a, b, c int) {
			record(name, a, b, c)
			if throwing {
				panic(&jsvalue.Error{Message: name})
			}
		}
	}
	errors := ErrorHandlers{
		Unterminated: func(a, b, c int) {
			record("unterminated", a, b, c)
			unterminatedCalls++
			if unterminatedCalls >= 2 {
				panic(&jsvalue.Error{Message: "unterminated again"})
			}
		},
		StrictNumericEscape:              handler("strictNumericEscape"),
		InvalidEscapeSequence:            handler("invalidEscapeSequence"),
		NumericSeparatorInEscapeSequence: handler("numericSeparatorInEscapeSequence"),
		UnexpectedNumericSeparator:       handler("unexpectedNumericSeparator"),
		InvalidCodePoint:                 handler("invalidCodePoint"),
		InvalidDigit: func(a, b, c, radix int) bool {
			record("invalidDigit", a, b, c, radix)
			if throwing {
				panic(&jsvalue.Error{Message: "invalidDigit"})
			}
			return kind == "i" && flag(8)
		},
	}
	defer func() {
		if r := recover(); r != nil {
			switch e := r.(type) {
			case *jsvalue.Error:
				trace = append(trace, "throws Error: "+e.Message)
			case *jsvalue.RangeError:
				trace = append(trace, "throws RangeError: "+e.Message)
			case *jsvalue.TypeError:
				trace = append(trace, "throws TypeError: "+e.Message)
			default:
				panic(r)
			}
		}
	}()
	switch kind {
	case "c":
		r := ReadStringContents(str(1), str(2), num(3), num(4), num(5), errors)
		loc := "null"
		if r.FirstInvalidLoc != nil {
			loc = `{"pos":` + strconv.Itoa(r.FirstInvalidLoc.Pos) + `,"lineStart":` + strconv.Itoa(r.FirstInvalidLoc.LineStart) +
				`,"curLine":` + strconv.Itoa(r.FirstInvalidLoc.CurLine) + "}"
		}
		trace = append(trace, `ret {"pos":`+strconv.Itoa(r.Pos)+`,"str":`+jsvalue.JSONQuote(r.Str)+`,"firstInvalidLoc":`+loc+
			`,"lineStart":`+strconv.Itoa(r.LineStart)+`,"curLine":`+strconv.Itoa(r.CurLine)+`,"containsInvalid":`+strconv.FormatBool(r.ContainsInvalid)+"}")
	case "i":
		var length any = jsvalue.Undefined{}
		var n float64
		if json.Unmarshal(c[4], &n) == nil {
			length = n
		}
		var allow any
		var allowStr string
		if json.Unmarshal(c[6], &allowStr) == nil {
			allow = allowStr
		} else {
			var b bool
			json.Unmarshal(c[6], &b)
			allow = b
		}
		r := ReadInt(str(1), num(2), 0, 1, num(3), length, flag(5), allow, errors, flag(7))
		trace = append(trace, `ret {"n":`+numOrNull(r.N)+`,"pos":`+strconv.Itoa(r.Pos)+"}")
	case "p":
		r := ReadCodePoint(str(1), num(2), 0, 1, flag(3), errors)
		trace = append(trace, `ret {"code":`+numOrNull(r.Code)+`,"pos":`+strconv.Itoa(r.Pos)+"}")
	}
	return trace
}

func numOrNull(v any) string {
	if v == nil {
		return "null"
	}
	f := v.(float64)
	if f == 0 && math.Signbit(f) {
		return "-0"
	}
	return jsnum.String(f)
}
