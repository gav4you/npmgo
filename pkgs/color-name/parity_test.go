package colorname

import (
	"encoding/json"
	"os"
	"testing"
)

// TestParity diffs the table against the original color-name@1.1.4 as
// recorded by parity/oracle_chalk.cjs: every name, in key order, and its value.
func TestParity(t *testing.T) {
	b, err := os.ReadFile("../../parity/chalk/expected.json")
	if err != nil {
		t.Fatal(err)
	}
	var e struct {
		ColorName [][2]json.RawMessage `json:"colorName"`
	}
	if err := json.Unmarshal(b, &e); err != nil {
		t.Fatal(err)
	}
	if len(e.ColorName) != len(Names) || len(RGB) != len(Names) {
		t.Fatalf("js has %d names, go %d (%d in RGB)", len(e.ColorName), len(Names), len(RGB))
	}
	for i, entry := range e.ColorName {
		var name string
		var rgb [3]uint8
		if err := json.Unmarshal(entry[0], &name); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(entry[1], &rgb); err != nil {
			t.Fatal(err)
		}
		if Names[i] != name {
			t.Errorf("name #%d: -js %q +go %q", i, name, Names[i])
		}
		if RGB[name] != rgb {
			t.Errorf("%s: -js %v +go %v", name, rgb, RGB[name])
		}
	}
}
