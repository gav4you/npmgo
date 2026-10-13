package debug

import (
	"encoding/json"
	"encoding/hex"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/gav4you/npmgo/internal/nodeutil"
	"github.com/gav4you/npmgo/internal/parityspec"
)

type logCase struct {
	NS   string            `json:"ns"`
	Args []json.RawMessage `json:"args"`
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
		lines := []string{}
		Now = func() time.Time { return time.UnixMilli(1700000000000) }
		d := New(k.NS)
		d.Log = func(line string) { lines = append(lines, line) }
		args := make([]any, len(k.Args))
		for i, a := range k.Args {
			args[i] = parityspec.Build(a)
		}
		d.Print(args...)
		Now = time.Now
		out.Logs = append(out.Logs, lines)
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
	out.Custom = []string{}
	d := New("custom")
	d.UseColors = false
	d.Log = func(line string) { out.Custom = append(out.Custom, line) }
	saved := InspectOpts
	InspectOpts = map[string]any{"hideDate": true}
	d.Print("hex %h and %s", "AB", "tail")
	InspectOpts = saved
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
// npm debug@4.4.3 by parity/oracle_debug.cjs.
func TestParity(t *testing.T) {
	var c cases
	var want struct{ Plain, Colored mode }
	load(t, "../../parity/debug/cases.json", &c)
	load(t, "../../parity/debug/expected.json", &want)

	for name, m := range map[string]struct {
		env  []string
		want mode
	}{
		"plain":   {[]string{"DEBUG_COLORS=no", "DEBUG_HIDE_DATE=yes"}, want.Plain},
		"colored": {[]string{"DEBUG_COLORS=true"}, want.Colored},
	} {
		got := run(c, m.env)
		gv, wv := reflect.ValueOf(got), reflect.ValueOf(m.want)
		for i := 0; i < gv.NumField(); i++ {
			f := gv.Type().Field(i).Name
			if !reflect.DeepEqual(gv.Field(i).Interface(), wv.Field(i).Interface()) {
				if f == "Logs" {
					for j := range m.want.Logs {
						if j >= len(got.Logs) || !reflect.DeepEqual(got.Logs[j], m.want.Logs[j]) {
							var g []string
							if j < len(got.Logs) {
								g = got.Logs[j]
							}
							t.Errorf("%s log #%d %q\n--- js  %q\n+++ go  %q", name, j, c.Logs[j].NS, m.want.Logs[j], g)
						}
					}
					continue
				}
				t.Errorf("%s %s\n--- js  %v\n+++ go  %v", name, f, wv.Field(i).Interface(), gv.Field(i).Interface())
			}
		}
	}
	_ = nodeutil.Undefined{}
}
