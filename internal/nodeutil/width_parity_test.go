package nodeutil_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	. "github.com/gav4you/npmgo/internal/nodeutil"
)

// TestWidthParity diffs GetStringWidth / IsZeroWidthCodePoint against Node's
// ICU-backed implementation (parity/oracle_width.cjs).
func TestWidthParity(t *testing.T) {
	var c struct {
		Strings    []string
		Codepoints []rune
	}
	var want struct {
		Width, WidthKeepControl []int
		ZeroWidth               []bool
	}
	for p, v := range map[string]any{"../../parity/width/cases.json": &c, "../../parity/width/expected.json": &want} {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(b, v); err != nil {
			t.Fatal(err)
		}
	}
	for i, s := range c.Strings {
		s = strings.ReplaceAll(s, "{ESC}", "\x1b")
		if got := GetStringWidth(s); got != want.Width[i] {
			t.Errorf("width(%q): -node %d +go %d", s, want.Width[i], got)
		}
		if got := GetStringWidth(s, false); got != want.WidthKeepControl[i] {
			t.Errorf("width(%q, false): -node %d +go %d", s, want.WidthKeepControl[i], got)
		}
	}
	for i, cp := range c.Codepoints {
		if got := IsZeroWidthCodePoint(cp); got != want.ZeroWidth[i] {
			t.Errorf("isZeroWidthCodePoint(%#x): -node %v +go %v", cp, want.ZeroWidth[i], got)
		}
	}
}
