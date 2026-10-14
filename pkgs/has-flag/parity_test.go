package hasflag

import (
	"encoding/json"
	"os"
	"testing"
)

func load(t *testing.T, path string, v any) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatal(path, err)
	}
}

// TestParity diffs HasFlag against the original has-flag@4.0.0 as recorded
// by parity/oracle_chalk.cjs.
func TestParity(t *testing.T) {
	var c struct{ HasFlag [][2]json.RawMessage }
	var e struct{ HasFlag []bool }
	load(t, "../../parity/chalk/cases.json", &c)
	load(t, "../../parity/chalk/expected.json", &e)
	if len(c.HasFlag) != len(e.HasFlag) {
		t.Fatalf("%d cases, %d recorded", len(c.HasFlag), len(e.HasFlag))
	}
	for i, k := range c.HasFlag {
		var flag string
		var argv []string
		if json.Unmarshal(k[0], &flag) != nil || json.Unmarshal(k[1], &argv) != nil {
			t.Fatalf("bad case %s", k)
		}
		if got := HasFlag(flag, argv); got != e.HasFlag[i] {
			t.Errorf("hasFlag(%q, %q): -js %v +go %v", flag, argv, e.HasFlag[i], got)
		}
	}
}
