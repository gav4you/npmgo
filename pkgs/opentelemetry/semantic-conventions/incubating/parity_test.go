package incubating

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/gav4you/npmgo/internal/jsnum"
	"github.com/gav4you/npmgo/internal/jsvalue"
)

// encExport renders an export as parity/oracle_opentelemetry.cjs does (the incubating entry point has strings,
// numbers and template functions only).
func encExport(t *testing.T, v any, sampleKeys []string) string {
	switch x := v.(type) {
	case string:
		return jsvalue.JSONQuote(x)
	case int:
		return jsnum.String(float64(x))
	case func(string) string:
		parts := make([]string, len(sampleKeys))
		for i, k := range sampleKeys {
			parts[i] = jsvalue.JSONQuote(x(k))
		}
		return "fn(" + strings.Join(parts, ",") + ")"
	}
	t.Fatalf("unexpected export type %T", v)
	return ""
}

// TestParity diffs every export of require('@opentelemetry/semantic-conventions/incubating') as recorded by
// parity/oracle_opentelemetry.cjs against its Go translation.
func TestParity(t *testing.T) {
	var e struct {
		Semconv struct{ Incubating [][2]string }
	}
	var c struct {
		Semconv struct{ SampleKeys []string }
	}
	for _, f := range []struct {
		path string
		v    any
	}{{"../../../../parity/opentelemetry/expected.json", &e}, {"../../../../parity/opentelemetry/cases.json", &c}} {
		b, err := os.ReadFile(f.path)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(b, f.v); err != nil {
			t.Fatal(err)
		}
	}
	exports := e.Semconv.Incubating
	if len(exports) != len(jsExports) {
		t.Errorf("the module has %d exports, the Go table %d", len(exports), len(jsExports))
	}
	for _, ex := range exports {
		v, ok := jsExports[ex[0]]
		if !ok {
			t.Errorf("%s: not translated", ex[0])
			continue
		}
		if got := encExport(t, v, c.Semconv.SampleKeys); got != ex[1] {
			t.Errorf("%s:\n--- js %s\n+++ go %s", ex[0], ex[1], got)
		}
	}
	t.Logf("%d exports", len(exports))
}
