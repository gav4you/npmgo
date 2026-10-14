package nodeutil_test

import (
	"encoding/json"
	"math"
	"os"
	"strings"
	"testing"

	. "github.com/gav4you/npmgo/internal/nodeutil"
	"github.com/gav4you/npmgo/internal/parityspec"
)

func optsFrom(raw json.RawMessage) (Options, bool) {
	o := DefaultOptions()
	if len(raw) == 0 {
		return o, false
	}
	var m map[string]any
	json.Unmarshal(raw, &m)
	// Numeric options may be the strings "Infinity" or "NaN", as in the oracle.
	num := func(v any) float64 {
		switch v {
		case "Infinity":
			return math.Inf(1)
		case "NaN":
			return math.NaN()
		}
		return v.(float64)
	}
	for k, v := range m {
		switch k {
		case "depth":
			o.Depth = num(v)
		case "compact":
			switch x := v.(type) {
			case bool:
				o.CompactTrue, o.CompactFalse = x, !x
			case float64:
				o.Compact = int(x)
			}
		case "breakLength":
			o.BreakLength = num(v)
		case "maxArrayLength":
			o.MaxArrayLength = num(v)
		case "maxStringLength":
			o.MaxStringLength = num(v)
		case "sorted":
			if v == "reverse" {
				o.Sorted = true
				o.SortFunc = func(a, b string) int { return strings.Compare(b, a) }
			} else {
				o.Sorted = v.(bool)
			}
		case "getters":
			switch g := v.(type) {
			case bool:
				o.Getters = g
			case string:
				o.GettersMode = g
			}
		case "showHidden":
			o.ShowHidden = v.(bool)
		case "colors":
			o.Colors = v.(bool)
		case "numericSeparator":
			o.NumericSeparator = v.(bool)
		case "showProxy":
			o.ShowProxy = v.(bool)
		case "customInspect":
			o.CustomInspect = v.(bool)
		}
	}
	return o, true
}

type kase struct {
	V    json.RawMessage `json:"v"`
	Args json.RawMessage `json:"args"`
	O    json.RawMessage `json:"o"`
}

// TestParity diffs Go output against Node's util.inspect / util.format output
// recorded by parity/oracle_nodeutil.cjs.
func TestParity(t *testing.T) {
	var cases struct{ Inspect, Format []kase }
	var want struct {
		Node    string
		Cwd     string
		Inspect []string
		Format  []string
	}
	for path, v := range map[string]any{"../../parity/nodeutil/cases.json": &cases, "../../parity/nodeutil/expected.json": &want} {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(b, v); err != nil {
			t.Fatal(path, err)
		}
	}
	parityspec.CWD = want.Cwd
	Cwd = func() (string, error) { return want.Cwd, nil }
	fails := 0
	check := func(kind string, i int, spec []byte, got, exp string) {
		if got != exp {
			fails++
			t.Errorf("%s #%d %s\n--- node\n%s\n+++ go\n%s", kind, i, spec, exp, got)
		}
	}
	for i, k := range cases.Inspect {
		o, _ := optsFrom(k.O)
		check("inspect", i, k.V, Inspect(parityspec.Build(k.V), o), want.Inspect[i])
	}
	for i, k := range cases.Format {
		args := parityspec.Build(k.Args).([]any)
		var got string
		if o, ok := optsFrom(k.O); ok {
			got = FormatWithOptions(o, args...)
		} else {
			got = Format(args...)
		}
		check("format", i, k.Args, got, want.Format[i])
	}
	t.Logf("node %s: %d inspect + %d format cases, %d mismatches",
		want.Node, len(cases.Inspect), len(cases.Format), fails)
}
