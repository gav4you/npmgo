package supportscolor

import (
	"bytes"
	"encoding/json"
	"fmt"
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

// envRow is one environment of parity/chalk/cases.json, with the oracle's
// defaults (a Linux TTY with no environment variables) for absent fields.
type envRow struct {
	Name      string
	Platform  *string
	Release   *string
	Argv      []string
	Env       map[string]string
	StdoutTTY *bool `json:"stdoutTTY"`
	StderrTTY *bool `json:"stderrTTY"`
}

func (r envRow) env() Env {
	e := Env{Argv: []string{"node", "/app/index.js"}, Env: r.Env, Platform: "linux", OSRelease: "5.15.0", StdoutTTY: true, StderrTTY: true}
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

// levelJSON renders a level as JSON.stringify renders translateLevel's result.
func levelJSON(l *Level) string {
	if l == nil {
		return "false"
	}
	return fmt.Sprintf(`{"level":%d,"hasBasic":%t,"has256":%t,"has16m":%t}`, l.Level, l.HasBasic, l.Has256, l.Has16m)
}

func compact(t *testing.T, raw json.RawMessage) string {
	var b bytes.Buffer
	if err := json.Compact(&b, raw); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

// TestParity diffs Load against the original supports-color@7.2.0 as
// recorded by parity/oracle_chalk.cjs, which loads the module in a fresh
// Node process per environment row with process.argv, process.env,
// process.platform, os.release() and tty.isatty() pinned: exports.stdout,
// exports.stderr and supportsColor() for no stream, a TTY, a non-TTY and a
// stream without isTTY.
func TestParity(t *testing.T) {
	var c struct{ Envs []envRow }
	var e struct {
		Envs []struct {
			Stdout, Stderr json.RawMessage
			Streams        []json.RawMessage
		}
	}
	load(t, "../../parity/chalk/cases.json", &c)
	load(t, "../../parity/chalk/expected.json", &e)
	if len(c.Envs) != len(e.Envs) {
		t.Fatalf("%d rows, %d recorded", len(c.Envs), len(e.Envs))
	}
	checks := 0
	for i, row := range c.Envs {
		m := Load(row.env())
		want := e.Envs[i]
		got := map[string]string{"stdout": levelJSON(m.Stdout), "stderr": levelJSON(m.Stderr)}
		for name, raw := range map[string]json.RawMessage{"stdout": want.Stdout, "stderr": want.Stderr} {
			checks++
			if w := compact(t, raw); got[name] != w {
				t.Errorf("%s: %s: -js %s +go %s", row.Name, name, w, got[name])
			}
		}
		for j, stream := range []*Stream{nil, {IsTTY: true}, {IsTTY: false}, {}} {
			checks++
			if w, g := compact(t, want.Streams[j]), levelJSON(m.SupportsColor(stream)); g != w {
				t.Errorf("%s: supportsColor(%v): -js %s +go %s", row.Name, stream, w, g)
			}
		}
	}
	t.Logf("%d environments, %d checks", len(c.Envs), checks)
}
