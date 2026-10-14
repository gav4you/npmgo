package chalk

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/gav4you/npmgo/internal/jsvalue"
	ansistyles "github.com/gav4you/npmgo/pkgs/ansi-styles"
	supportscolor "github.com/gav4you/npmgo/pkgs/supports-color"
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

// errorText renders a thrown error as the oracle does: "throws <name>: <message>".
func errorText(e any) string {
	switch x := e.(type) {
	case *jsvalue.TypeError:
		return "throws TypeError: " + x.Message
	case *jsvalue.RangeError:
		return "throws RangeError: " + x.Message
	case error:
		return "throws Error: " + x.Error()
	}
	panic(e)
}

// attempt runs f and returns its string result, or the error it panicked
// with as errorText.
func attempt(f func() string) (s string) {
	defer func() {
		if r := recover(); r != nil {
			s = errorText(r)
		}
	}()
	return f()
}

// same compares a result with an oracle record: a JSON-encoded string
// (decoded, so a lone surrogate JavaScript records as \udXXX reads as
// U+FFFD, which Go produces in its place) or "throws …".
func same(t *testing.T, want, got string) bool {
	if strings.HasPrefix(want, "throws ") {
		return got == want
	}
	var s string
	if err := json.Unmarshal([]byte(want), &s); err != nil {
		t.Fatalf("bad record %q", want)
	}
	return got == s
}

// base is the module the test instances come from; every case gives its
// instance an explicit level, so the detected levels do not matter.
var base = Load(nil, nil)

func instance(level int) *Chalk { return base.Instance(Options{Level: level}) }

// num decodes {"num": "NaN"} and its kin.
func special(s string) float64 {
	return map[string]float64{"NaN": math.NaN(), "Infinity": math.Inf(1), "-Infinity": math.Inf(-1), "-0": math.Copysign(0, -1)}[s]
}

// val is the oracle's val: a call argument from its JSON spec.
func val(t *testing.T, raw json.RawMessage, c *Chalk) any {
	raw = bytes.TrimSpace(raw)
	switch raw[0] {
	case '[':
		var items []json.RawMessage
		json.Unmarshal(raw, &items)
		out := make([]any, len(items))
		for i, it := range items {
			out[i] = val(t, it, c)
		}
		return out
	case '{':
		var o struct {
			Num   *string
			Undef bool
			Chalk *struct {
				Chain []json.RawMessage
				Args  []json.RawMessage
			}
		}
		if err := json.Unmarshal(raw, &o); err != nil {
			t.Fatal(err)
		}
		switch {
		case o.Num != nil:
			return special(*o.Num)
		case o.Undef:
			return jsvalue.Undefined{}
		case o.Chalk != nil:
			return chain(t, c, o.Chalk.Chain).Sprint(vals(t, o.Chalk.Args, c)...)
		}
		t.Fatalf("bad value %s", raw)
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func vals(t *testing.T, raws []json.RawMessage, c *Chalk) []any {
	out := make([]any, len(raws))
	for i, r := range raws {
		out[i] = val(t, r, c)
	}
	return out
}

// applyModel calls a colour-model method by its JavaScript name.
func applyModel(c *Chalk, name string, args []any) *Chalk {
	f := func(i int) float64 { return args[i].(float64) }
	s := func() string { return args[0].(string) }
	switch name {
	case "rgb":
		return c.RGB(f(0), f(1), f(2))
	case "bgRgb":
		return c.BgRGB(f(0), f(1), f(2))
	case "hsl":
		return c.HSL(f(0), f(1), f(2))
	case "bgHsl":
		return c.BgHSL(f(0), f(1), f(2))
	case "hsv":
		return c.HSV(f(0), f(1), f(2))
	case "bgHsv":
		return c.BgHSV(f(0), f(1), f(2))
	case "hwb":
		return c.HWB(f(0), f(1), f(2))
	case "bgHwb":
		return c.BgHWB(f(0), f(1), f(2))
	case "hex":
		return c.Hex(s())
	case "bgHex":
		return c.BgHex(s())
	case "keyword":
		return c.Keyword(s())
	case "bgKeyword":
		return c.BgKeyword(s())
	case "ansi":
		return c.ANSI(f(0))
	case "bgAnsi":
		return c.BgANSI(f(0))
	case "ansi256":
		return c.ANSI256(f(0))
	case "bgAnsi256":
		return c.BgANSI256(f(0))
	}
	panic("not a model: " + name)
}

// chain is the oracle's chain: getters ([name]) and model calls ([name, ...args]).
func chain(t *testing.T, c *Chalk, items []json.RawMessage) *Chalk {
	cur := c
	for _, it := range items {
		var parts []json.RawMessage
		json.Unmarshal(it, &parts)
		var name string
		json.Unmarshal(parts[0], &name)
		if len(parts) > 1 {
			cur = applyModel(cur, name, vals(t, parts[1:], c))
		} else {
			cur = cur.styleByName(name)
		}
	}
	return cur
}

type envRow struct {
	Name      string
	Platform  *string
	Release   *string
	Argv      []string
	Env       map[string]string
	StdoutTTY *bool `json:"stdoutTTY"`
	StderrTTY *bool `json:"stderrTTY"`
}

// env applies the oracle's defaults: a Linux TTY with no environment.
func (r envRow) env() supportscolor.Env {
	e := supportscolor.Env{Argv: []string{"node", "/app/index.js"}, Env: r.Env, Platform: "linux", OSRelease: "5.15.0", StdoutTTY: true, StderrTTY: true}
	if r.Argv != nil {
		e.Argv = r.Argv
	}
	if e.Env == nil {
		e.Env = map[string]string{}
	}
	if r.Platform != nil {
		e.Platform = *r.Platform
	}
	if r.Release != nil {
		e.OSRelease = *r.Release
	}
	if r.StdoutTTY != nil {
		e.StdoutTTY = *r.StdoutTTY
	}
	if r.StderrTTY != nil {
		e.StderrTTY = *r.StderrTTY
	}
	return e
}

func levelJSON(l *supportscolor.Level) string {
	if l == nil {
		return "false"
	}
	return fmt.Sprintf(`{"level":%d,"hasBasic":%t,"has256":%t,"has16m":%t}`, l.Level, l.HasBasic, l.Has256, l.Has16m)
}

// TestParity diffs chalk against the original chalk@4.1.2 as recorded by
// parity/oracle_chalk.cjs.
func TestParity(t *testing.T) {
	type call struct {
		Level        int
		SetLevel     *int
		BuilderLevel *int
		Chain        []json.RawMessage
		Args         []json.RawMessage
	}
	type tpl struct {
		Level  int
		Tpl    []string
		Values []json.RawMessage
		Chain  []json.RawMessage
	}
	var c struct {
		Envs  []envRow
		Chalk struct {
			LevelOption []int
			StyleMatrix struct {
				Levels []int
				Args   []json.RawMessage
			}
			ModelMatrix struct {
				Levels []int
				Values map[string][][]json.RawMessage
				Args   []json.RawMessage
			}
			Calls      []call
			LevelSet   [][2]int
			Templates  []tpl
			ReplaceAll [][3]string
			Encase     [][3]string
		}
	}
	var e struct {
		Envs  []struct{ Chalk json.RawMessage }
		Chalk struct {
			LevelOption []string
			StyleNames  []string
			Styles      []string
			ModelOrder  []string
			Models      []string
			Calls       []string
			LevelSet    []int
			Templates   []string
			ReplaceAll  []string
			Encase      []string
		}
	}
	load(t, "../../parity/chalk/cases.json", &c)
	load(t, "../../parity/chalk/expected.json", &e)
	k, w := c.Chalk, e.Chalk
	checks := map[string]int{}
	check := func(kind, what, want, got string) {
		checks[kind]++
		if !same(t, want, got) {
			t.Errorf("%s %s:\n--- js  %s\n+++ go  %q", kind, what, want, got)
		}
	}

	// require('chalk') in each environment: level, supportsColor, stderr, new Instance().
	for i, row := range c.Envs {
		sc := supportscolor.Load(row.env())
		got := attempt(func() string {
			m := Load(sc.Stdout, sc.Stderr)
			return fmt.Sprintf(`{"level":%d,"supportsColor":%s,"stderrLevel":%d,"stderrSupportsColor":%s,"instanceLevel":%d}`,
				m.Level(), levelJSON(m.SupportsColor), m.Stderr.Level(), levelJSON(m.Stderr.SupportsColor), m.Instance().Level())
		})
		want := string(e.Envs[i].Chalk)
		var s string
		if json.Unmarshal(e.Envs[i].Chalk, &s) == nil {
			want = s // a thrown error
		} else {
			var b bytes.Buffer
			json.Compact(&b, e.Envs[i].Chalk)
			want = b.String()
		}
		checks["env"]++
		if got != want {
			t.Errorf("env %s: require('chalk'):\n--- js  %s\n+++ go  %s", row.Name, want, got)
		}
	}

	for i, level := range k.LevelOption {
		got := attempt(func() string { return fmt.Sprint(instance(level).Level()) })
		want := w.LevelOption[i]
		checks["level option"]++
		if want != got && !(strings.HasPrefix(want, "throws") && got == want) {
			t.Errorf("new Instance({level: %d}): -js %s +go %s", level, want, got)
		}
	}

	names := append(append([]string{}, ansistyles.Names...), "visible")
	if !reflect.DeepEqual(w.StyleNames, names) {
		t.Fatalf("style names: -js %v\n+go %v", w.StyleNames, names)
	}
	n := 0
	for _, level := range k.StyleMatrix.Levels {
		for _, name := range names {
			c := instance(level)
			got := attempt(func() string { return c.styleByName(name).Sprint(vals(t, k.StyleMatrix.Args, c)...) })
			check("style", fmt.Sprintf("level %d %s", level, name), w.Styles[n], got)
			n++
		}
	}

	n = 0
	for _, level := range k.ModelMatrix.Levels {
		for _, model := range w.ModelOrder {
			for _, v := range k.ModelMatrix.Values[model] {
				for _, name := range []string{model, "bg" + strings.ToUpper(model[:1]) + model[1:]} {
					c := instance(level)
					got := attempt(func() string {
						return applyModel(c, name, vals(t, v, c)).Sprint(vals(t, k.ModelMatrix.Args, c)...)
					})
					check("model", fmt.Sprintf("level %d %s%s", level, name, raw(v...)), w.Models[n], got)
					n++
				}
			}
		}
	}

	for i, cl := range k.Calls {
		got := attempt(func() string {
			c := instance(cl.Level)
			if cl.SetLevel != nil {
				c.SetLevel(*cl.SetLevel)
			}
			target := chain(t, c, cl.Chain)
			if cl.BuilderLevel != nil {
				target.SetLevel(*cl.BuilderLevel)
			}
			return target.Sprint(vals(t, cl.Args, c)...)
		})
		check("call", fmt.Sprintf("#%d level %d %s%s", i, cl.Level, raw(cl.Chain...), raw(cl.Args...)), w.Calls[i], got)
	}

	for i, ls := range k.LevelSet {
		c := instance(ls[0])
		c.Red().Bold().SetLevel(ls[1])
		checks["level set"]++
		if c.Level() != w.LevelSet[i] {
			t.Errorf("instance.red.bold.level = %d from %d: -js %d +go %d", ls[1], ls[0], w.LevelSet[i], c.Level())
		}
	}

	for i, tp := range k.Templates {
		got := attempt(func() string {
			c := instance(tp.Level)
			target := c
			if tp.Chain != nil {
				target = chain(t, c, tp.Chain)
			}
			s, err := target.Tag(tp.Tpl, vals(t, tp.Values, c)...)
			if err != nil {
				return errorText(err)
			}
			return s
		})
		check("template", fmt.Sprintf("#%d level %d %q", i, tp.Level, tp.Tpl), w.Templates[i], got)
	}

	for i, r := range k.ReplaceAll {
		checks["util"]++
		if got := stringReplaceAll(r[0], r[1], r[2]); got != w.ReplaceAll[i] {
			t.Errorf("stringReplaceAll(%q, %q, %q): -js %q +go %q", r[0], r[1], r[2], w.ReplaceAll[i], got)
		}
	}
	for i, r := range k.Encase {
		checks["util"]++
		got := stringEncaseCRLFWithFirstIndex(r[0], r[1], r[2], strings.IndexByte(r[0], '\n'))
		if got != w.Encase[i] {
			t.Errorf("stringEncaseCRLFWithFirstIndex(%q, %q, %q): -js %q +go %q", r[0], r[1], r[2], w.Encase[i], got)
		}
	}
	total := 0
	for _, v := range checks {
		total += v
	}
	t.Logf("%d checks: %v", total, checks)
}

func raw(items ...json.RawMessage) string {
	parts := make([]string, len(items))
	for i, it := range items {
		parts[i] = string(it)
	}
	return "(" + strings.Join(parts, ", ") + ")"
}
