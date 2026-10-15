package helperstringparser

import (
	"testing"

	"github.com/gav4you/npmgo/internal/jsvalue"
)

// A nil handler is a missing method, as in JavaScript.
func TestMissingHandler(t *testing.T) {
	defer func() {
		te, ok := recover().(*jsvalue.TypeError)
		if !ok || te.Message != "errors.unterminated is not a function" {
			t.Fatalf("got %v", te)
		}
	}()
	ReadStringContents("template", "abc", 0, 0, 1, ErrorHandlers{})
}

// ReadInt's len is undefined (nil or jsvalue.Undefined{}) or a number of any Go numeric type.
func TestReadIntLength(t *testing.T) {
	for _, length := range []any{nil, jsvalue.Undefined{}} {
		if r := ReadInt("123", 0, 0, 1, 10, length, false, false, ErrorHandlers{}, false); r.N != 123.0 || r.Pos != 3 {
			t.Errorf("len %v: %v", length, r)
		}
	}
	for _, length := range []any{2, 2.0, int64(2)} {
		if r := ReadInt("123", 0, 0, 1, 10, length, false, false, ErrorHandlers{}, false); r.N != 12.0 || r.Pos != 2 {
			t.Errorf("len %v: %v", length, r)
		}
	}
	if r := ReadInt("12", 0, 0, 1, 10, 3, false, false, ErrorHandlers{}, false); r.N != nil || r.Pos != 2 {
		t.Errorf("short input: %v", r)
	}
	if r := ReadCodePoint("{1F600}", 0, 0, 1, true, ErrorHandlers{}); r.Code != 128512.0 || r.Pos != 7 {
		t.Errorf("code point: %v", r)
	}
}
