package semanticconventions

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/gav4you/npmgo/internal/jsnum"
	"github.com/gav4you/npmgo/internal/jsvalue"
)

// EncExport renders an export as parity/oracle_opentelemetry.cjs does: a string as JSON, a number as String(x), a
// template function as fn(f(key)...) over the sample keys, a namespace object as {key:value,...} in key order.
func EncExport(t *testing.T, v any, sampleKeys []string) string {
	switch x := v.(type) {
	case string:
		return jsvalue.JSONQuote(x)
	case int:
		return jsnum.String(float64(x))
	case float64:
		return jsnum.String(x)
	case func(string) string:
		parts := make([]string, len(sampleKeys))
		for i, k := range sampleKeys {
			parts[i] = jsvalue.JSONQuote(x(k))
		}
		return "fn(" + strings.Join(parts, ",") + ")"
	case *ConstMap[string]:
		parts := []string{}
		for _, k := range x.Keys() {
			val, _ := x.Get(k)
			parts = append(parts, jsvalue.JSONQuote(k)+":"+jsvalue.JSONQuote(val))
		}
		return "{" + strings.Join(parts, ",") + "}"
	case *ConstMap[float64]:
		parts := []string{}
		for _, k := range x.Keys() {
			val, _ := x.Get(k)
			parts = append(parts, jsvalue.JSONQuote(k)+":"+jsnum.String(val))
		}
		return "{" + strings.Join(parts, ",") + "}"
	}
	t.Fatalf("unexpected export type %T", v)
	return ""
}

// LoadExpected reads the semantic-conventions part of the recording and the sample keys.
func LoadExpected(t *testing.T, path, casesPath string) (main, incubating [][2]string, sampleKeys []string) {
	var e struct {
		Semconv struct{ Main, Incubating [][2]string }
	}
	var c struct {
		Semconv struct{ SampleKeys []string }
	}
	for _, f := range []struct {
		path string
		v    any
	}{{path, &e}, {casesPath, &c}} {
		b, err := os.ReadFile(f.path)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(b, f.v); err != nil {
			t.Fatal(err)
		}
	}
	return e.Semconv.Main, e.Semconv.Incubating, c.Semconv.SampleKeys
}

// TestParity diffs every export of require('@opentelemetry/semantic-conventions') as recorded by
// parity/oracle_opentelemetry.cjs against its Go translation.
func TestParity(t *testing.T) {
	main, _, keys := LoadExpected(t, "../../../parity/opentelemetry/expected.json", "../../../parity/opentelemetry/cases.json")
	if len(main) != len(jsExports) {
		t.Errorf("the module has %d exports, the Go table %d", len(main), len(jsExports))
	}
	for _, e := range main {
		v, ok := jsExports[e[0]]
		if !ok {
			t.Errorf("%s: not translated", e[0])
			continue
		}
		if got := EncExport(t, v, keys); got != e[1] {
			t.Errorf("%s:\n--- js %s\n+++ go %s", e[0], e[1], got)
		}
	}
	t.Logf("%d exports", len(main))
}

// TestConstMap checks the namespace objects' accessors.
func TestConstMap(t *testing.T) {
	if v, ok := SemanticAttributes.Get("HTTP_METHOD"); !ok || v != SEMATTRS_HTTP_METHOD {
		t.Errorf("SemanticAttributes.HTTP_METHOD = %q, %v", v, ok)
	}
	if _, ok := SemanticAttributes.Get("NOPE"); ok {
		t.Error("unknown key found")
	}
	if v, ok := RpcGrpcStatusCodeValues.Get("UNAUTHENTICATED"); !ok || v != 16 {
		t.Errorf("RpcGrpcStatusCodeValues.UNAUTHENTICATED = %v, %v", v, ok)
	}
	if n := HttpFlavorValues.Len(); n != 5 {
		t.Errorf("HttpFlavorValues has %d keys", n)
	}
	if ATTR_HTTP_REQUEST_HEADER("x") != "http.request.header.x" {
		t.Error("ATTR_HTTP_REQUEST_HEADER")
	}
}
