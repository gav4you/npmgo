package debug

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/gav4you/npmgo/internal/parityspec"
)

type logCase struct {
	NS         string            `json:"ns"`
	Args       []json.RawMessage `json:"args"`
	UseColors  *bool             `json:"useColors"`
	GlobalOpts map[string]any    `json:"globalOpts"`
}

type cases struct {
	Colors   []string  `json:"colors"`
	Patterns []string  `json:"patterns"`
	Names    []string  `json:"names"`
	Logs     []logCase `json:"logs"`
}

type mode struct {
	Colors    []int      `json:"colors"`
	Enabled   [][]bool   `json:"enabled"`
	Disable   []string   `json:"disable"`
	Logs      [][]string `json:"logs"`
	Extend    []string   `json:"extend"`
	Custom    []string   `json:"custom"`
	Override  bool       `json:"override"`
	Override2 bool       `json:"override2"`
}

func load(t *testing.T, path string, v any) {
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatal(path, err)
	}
}

// capture collects what the default sink (Log, debug's process.stderr.write)
// receives while fn runs, like the oracle's capture().
func capture(fn func()) []string {
	lines := []string{}
	saved := Log
	Log = func(line string) { lines = append(lines, line) }
	defer func() { Log = saved }()
	fn()
	return lines
}

// run reproduces parity/oracle_debug.cjs's run(env) against the Go port.
func run(c cases, env []string) mode {
	InspectOpts = LoadInspectOpts(env)
	var out mode
	for _, ns := range c.Colors {
		out.Colors = append(out.Colors, SelectColor(ns))
	}
	for _, p := range c.Patterns {
		Enable(p)
		row := []bool{}
		for _, n := range c.Names {
			row = append(row, Enabled(n))
		}
		out.Enabled = append(out.Enabled, row)
	}
	for _, p := range c.Patterns {
		Enable(p)
		out.Disable = append(out.Disable, Disable())
	}
	Enable("*")
	for _, k := range c.Logs {
		out.Logs = append(out.Logs, capture(func() {
			Now = func() time.Time { return time.UnixMilli(1700000000000) }
			defer func() { Now = time.Now }()
			d := New(k.NS)
			if k.UseColors != nil {
				d.UseColors = *k.UseColors
			}
			saved := InspectOpts
			if k.GlobalOpts != nil {
				merged := map[string]any{}
				for key, v := range saved {
					merged[key] = v
				}
				for key, v := range k.GlobalOpts {
					merged[key] = v
				}
				InspectOpts = merged
			}
			defer func() { InspectOpts = saved }()
			args := make([]any, len(k.Args))
			for i, a := range k.Args {
				args[i] = parityspec.Build(a)
			}
			d.Print(args...)
		}))
	}
	parent := New("parent")
	slash, empty := "/", ""
	out.Extend = []string{
		parent.Extend("child", nil).Namespace,
		parent.Extend("child", &slash).Namespace,
		parent.Extend("child", &empty).Namespace,
	}
	Formatters['h'] = func(d *Debugger, v any) string {
		return hex.EncodeToString([]byte(fmt.Sprint(v)))
	}
	d := New("custom")
	d.UseColors = false
	out.Custom = capture(func() { d.Print("hex %h and %s", "AB", "tail") })
	delete(Formatters, 'h')
	d2 := New("override")
	Enable("")
	d2.SetEnabled(true)
	out.Override = d2.Enabled()
	Enable("*")
	d2.SetEnabled(false)
	out.Override2 = d2.Enabled()
	return out
}

// TestParity diffs the Go port against outputs recorded from the original
// npm debug@4.4.3 by parity/oracle_debug.cjs, in its three env modes.
func TestParity(t *testing.T) {
	var c cases
	var want struct{ Plain, Colored, Env mode }
	load(t, "../../parity/debug/cases.json", &c)
	load(t, "../../parity/debug/expected.json", &want)

	for _, m := range []struct {
		name string
		env  []string
		want mode
	}{
		{"plain", []string{"DEBUG_COLORS=no", "DEBUG_HIDE_DATE=yes"}, want.Plain},
		{"colored", []string{"DEBUG_COLORS=true", "DEBUG_HIDE_DATE=yes"}, want.Colored},
		{"env", []string{"DEBUG_COLORS=off", "DEBUG_HIDE_DATE=on", "DEBUG_DEPTH=yes", "DEBUG_SHOW_HIDDEN=1",
			"DEBUG_COMPACT=null", "DEBUG_BREAK_LENGTH=abc", "DEBUG_MAX_ARRAY_LENGTH=2", "DEBUG_SORTED=enabled",
			"DEBUG_GETTERS=1", "DEBUG_NUMERIC_SEPARATOR=TRUE", "DEBUG_SHOW_PROXY=true", "DEBUG_MAX_STRING_LENGTH=3"}, want.Env},
	} {
		got := run(c, m.env)
		gv, wv := reflect.ValueOf(got), reflect.ValueOf(m.want)
		for i := 0; i < gv.NumField(); i++ {
			f := gv.Type().Field(i).Name
			if reflect.DeepEqual(gv.Field(i).Interface(), wv.Field(i).Interface()) {
				continue
			}
			switch f {
			case "Logs":
				for j := range m.want.Logs {
					if j >= len(got.Logs) || !reflect.DeepEqual(got.Logs[j], m.want.Logs[j]) {
						var g []string
						if j < len(got.Logs) {
							g = got.Logs[j]
						}
						t.Errorf("%s log #%d %q\n--- js  %q\n+++ go  %q", m.name, j, c.Logs[j].NS, m.want.Logs[j], g)
					}
				}
			case "Enabled", "Disable":
				for j := range c.Patterns {
					if !reflect.DeepEqual(gv.Field(i).Index(j).Interface(), wv.Field(i).Index(j).Interface()) {
						t.Errorf("%s %s pattern %q\n--- js  %v\n+++ go  %v", m.name, f, c.Patterns[j], wv.Field(i).Index(j).Interface(), gv.Field(i).Index(j).Interface())
					}
				}
			default:
				t.Errorf("%s %s\n--- js  %v\n+++ go  %v", m.name, f, wv.Field(i).Interface(), gv.Field(i).Interface())
			}
		}
	}
}
