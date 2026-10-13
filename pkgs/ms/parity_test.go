package ms

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

type expected struct {
	ParseIn []string   `json:"parseIn"`
	Parse   []*float64 `json:"parse"`
	Short   []string   `json:"short"`
	Long    []string   `json:"long"`
	Err     []string   `json:"err"`
}

type cases struct {
	Num []float64 `json:"num"`
}

func load(t *testing.T, path string, v any) {
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatal(err)
	}
}

// TestParity diffs every case against outputs recorded from the original npm ms@2.1.3.
func TestParity(t *testing.T) {
	var e expected
	var c cases
	load(t, "../../parity/ms/expected.json", &e)
	load(t, "../../parity/ms/cases.json", &c)

	for i, in := range e.ParseIn {
		got, ok := Parse(in)
		want := e.Parse[i]
		gs, ws := "undefined", "undefined"
		if ok {
			gs = fmt.Sprint(got)
		}
		if want != nil {
			ws = fmt.Sprint(*want)
		}
		if gs != ws {
			t.Errorf("parse(%q): -js %s +go %s", in, ws, gs)
		}
	}
	for i, n := range c.Num {
		if got, _ := Format(n, false); got != e.Short[i] {
			t.Errorf("ms(%v): -js %q +go %q", n, e.Short[i], got)
		}
		if got, _ := Format(n, true); got != e.Long[i] {
			t.Errorf("ms(%v,{long}): -js %q +go %q", n, e.Long[i], got)
		}
	}
	// JS JSON.stringify(NaN) and (Infinity) both yield "null".
	for _, msg := range e.Err {
		if _, err := Format(nan(), false); err == nil || err.Error() != msg {
			t.Errorf("error: -js %q +go %v", msg, err)
		}
	}
}

func nan() float64 { var z float64; return z / z }
