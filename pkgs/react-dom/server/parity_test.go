package server

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/gav4you/npmgo/internal/jsvalue"
	"github.com/gav4you/npmgo/internal/reactspec"
	"github.com/gav4you/npmgo/pkgs/react"
)

type legacyResult struct {
	HTML   *string `json:"html"`
	Error  *string `json:"error"`
	HTML2  *string `json:"html2"`
	Error2 *string `json:"error2"`
}

type streamResult struct {
	HTML       *string  `json:"html"`
	Events     []string `json:"events"`
	Error      *string  `json:"error"`
	SetupError *string  `json:"setupError"`
}

type caseResult struct {
	Name   string        `json:"name"`
	String *legacyResult `json:"string"`
	Static *legacyResult `json:"static"`
	Stream *streamResult `json:"stream"`
}

// errMessage mirrors the oracle's msg(e): e.message for Errors, otherwise
// "THROWN " + String(e).
func errMessage(err any) string {
	if p, ok := err.(*react.Panic); ok {
		if e, ok := p.Value.(error); ok {
			return errMessage(e)
		}
		return "THROWN " + jsvalue.String(p.Value)
	}
	if e, ok := err.(error); ok {
		return e.Error()
	}
	return "THROWN " + jsvalue.String(err)
}

func sptr(s string) *string { return &s }

func runLegacy(c *reactspec.Case, render func(react.Node, ...*Options) (string, error)) *legacyResult {
	env := reactspec.Setup(c)
	var opts []*Options
	if c.Opts != nil {
		opts = []*Options{{IdentifierPrefix: c.Opts.IdentifierPrefix}}
	}
	out := &legacyResult{}
	html, err := renderCatching(func() (string, error) { return render(env.EvalTop(c.Tree), opts...) })
	if err != nil {
		out.Error = sptr(errMessage(err))
	} else {
		out.HTML = &html
	}
	if c.Twice {
		env.Resolve(1)
		html, err := renderCatching(func() (string, error) { return render(env.EvalTop(c.Tree), opts...) })
		if err != nil {
			out.Error2 = sptr(errMessage(err))
		} else {
			out.HTML2 = &html
		}
	}
	return out
}

// renderCatching turns a panic while building the tree into an error.
func renderCatching(f func() (string, error)) (html string, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = ToError(react.Thrown(p))
		}
	}()
	return f()
}

// recordingDestination records what the oracle's fake Writable records.
type recordingDestination struct {
	chunks []string
	events *[]string
}

func (d *recordingDestination) write(chunk string) bool { d.chunks = append(d.chunks, chunk); return true }
func (d *recordingDestination) beginWriting()           {}
func (d *recordingDestination) completeWriting()        {}
func (d *recordingDestination) flushBuffered()          {}
func (d *recordingDestination) end()                    { *d.events = append(*d.events, "end") }
func (d *recordingDestination) destroy(err any) {
	*d.events = append(*d.events, "destroy:"+errMessage(err))
}

func streamOptions(c *reactspec.Case, env *reactspec.Env, events *[]string) *StreamOptions {
	o := &StreamOptions{}
	if c.Stream == nil {
		return o
	}
	for k, raw := range c.Stream.Opts {
		if k == "onHeaders" {
			o.OnHeaders = func(h map[string]string) {
				obj := &react.Object{}
				if l, ok := h["Link"]; ok {
					obj.Set("Link", l)
				}
				j, _ := jsvalue.JSONStringify(obj)
				*events = append(*events, "headers:"+j)
			}
			continue
		}
		v := env.EvalTop(raw)
		switch k {
		case "identifierPrefix":
			o.IdentifierPrefix = jsvalue.ToString(v)
		case "namespaceURI":
			o.NamespaceURI = jsvalue.ToString(v)
		case "nonce":
			o.Nonce = v
		case "bootstrapScriptContent":
			o.BootstrapScriptContent = v
		case "bootstrapScripts":
			o.BootstrapScripts = jsvalue.Items(v)
		case "bootstrapModules":
			o.BootstrapModules = jsvalue.Items(v)
		case "importMap":
			o.ImportMap = v
		case "progressiveChunkSize":
			o.ProgressiveChunkSize = int(jsvalue.ToNumber(v))
		case "maxHeadersLength":
			o.MaxHeadersLength = int(jsvalue.ToNumber(v))
		case "formState":
			o.FormState = v
		default:
			panic("unknown stream option " + k)
		}
	}
	return o
}

func runStream(c *reactspec.Case) (res *streamResult) {
	env := reactspec.Setup(c)
	events := []string{}
	dest := &recordingDestination{events: &events}
	sc := c.Stream
	if sc == nil {
		sc = &reactspec.StreamSpec{}
	}
	pipeWhen := sc.Pipe
	if pipeWhen == "" {
		pipeWhen = "now"
	}
	var s *PipeableStream
	opts := streamOptions(c, env, &events)
	opts.OnShellReady = func() {
		events = append(events, "shellReady")
		if pipeWhen == "shell" {
			s.attach(dest)
		}
	}
	opts.OnShellError = func(err error) { events = append(events, "shellError:"+errMessage(err)) }
	opts.OnAllReady = func() {
		events = append(events, "allReady")
		if pipeWhen == "all" {
			s.attach(dest)
		}
	}
	opts.OnError = func(err error, _ ErrorInfo) string {
		events = append(events, "error:"+errMessage(err))
		if sc.Digest == "msg" {
			return "D:" + errMessage(err)
		}
		return ""
	}
	var tree any
	if _, err := renderCatching(func() (string, error) { tree = env.EvalTop(c.Tree); return "", nil }); err != nil {
		return &streamResult{SetupError: sptr(errMessage(err))}
	}
	s = RenderToPipeableStream(tree, opts)
	if pipeWhen == "now" {
		s.attach(dest)
	}
	s.r.loop.RunUntilIdle()
	last := env.MaxStep()
	if sc.Abort != nil && int(sc.Abort.Step) > last {
		last = int(sc.Abort.Step)
	}
	for step := 1; step <= last; step++ {
		if sc.Abort != nil && int(sc.Abort.Step) == step {
			var reason any = react.Undefined{}
			if sc.Abort.Reason != nil {
				reason = env.EvalTop(sc.Abort.Reason)
			}
			s.r.abort(reason)
		}
		env.Resolve(step)
		s.r.loop.RunUntilIdle()
	}
	html := strings.Join(dest.chunks, "")
	return &streamResult{HTML: &html, Events: events}
}

func load(t *testing.T, path string, v any) []byte {
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if v != nil {
		if err := json.Unmarshal(b, v); err != nil {
			t.Fatal(path, err)
		}
	}
	return b
}

func show(p *string) string {
	if p == nil {
		return "<none>"
	}
	return *p
}

// TestReactParity renders every case in parity/react/cases.json and diffs
// the HTML (and stream callback order) against the output of the original
// react@19.2.4 / react-dom@19.2.4 recorded by parity/oracle_react.cjs.
func TestReactParity(t *testing.T) {
	reactspec.RenderToString = func(node any) (string, error) { return RenderToString(node) }
	cases, err := reactspec.LoadCases(load(t, "../../../parity/react/cases.json", nil))
	if err != nil {
		t.Fatal(err)
	}
	var want struct {
		Node    string        `json:"node"`
		Results []*caseResult `json:"results"`
	}
	load(t, "../../../parity/react/expected.json", &want)
	if len(want.Results) != len(cases) {
		t.Fatalf("expected.json has %d results for %d cases; re-run the oracle", len(want.Results), len(cases))
	}
	checks, fails := 0, 0
	for i, c := range cases {
		w := want.Results[i]
		if w.Name != c.Name {
			t.Fatalf("case %d: expected.json has %q, cases.json %q", i, w.Name, c.Name)
		}
		cmpLegacy := func(mode string, got, exp *legacyResult) {
			checks++
			if !reflect.DeepEqual(got, exp) {
				fails++
				t.Errorf("%s [%s]\n--- node html=%s error=%s html2=%s error2=%s\n+++ go   html=%s error=%s html2=%s error2=%s",
					c.Name, mode, show(exp.HTML), show(exp.Error), show(exp.HTML2), show(exp.Error2),
					show(got.HTML), show(got.Error), show(got.HTML2), show(got.Error2))
			}
		}
		if c.HasMode("string") {
			cmpLegacy("string", runLegacy(c, RenderToString), w.String)
		}
		if c.HasMode("static") {
			cmpLegacy("static", runLegacy(c, RenderToStaticMarkup), w.Static)
		}
		if c.HasMode("stream") {
			checks++
			got := runStream(c)
			if !reflect.DeepEqual(got, w.Stream) {
				fails++
				t.Errorf("%s [stream]\n--- node html=%s\n         events=%q error=%s\n+++ go   html=%s\n         events=%q error=%s",
					c.Name, show(w.Stream.HTML), w.Stream.Events, show(w.Stream.Error),
					show(got.HTML), got.Events, show(got.Error))
			}
		}
	}
	t.Logf("node %s: %d cases, %d checks, %d mismatches", want.Node, len(cases), checks, fails)
}
