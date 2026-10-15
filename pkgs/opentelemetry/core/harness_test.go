package core

// The recording machinery of the parity test: the Go twin of parity/oracle_opentelemetry.cjs's helpers.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/gav4you/npmgo/internal/jsvalue"
	"github.com/gav4you/npmgo/internal/parityspec"
	"github.com/gav4you/npmgo/pkgs/opentelemetry/api"
)

// sink is the trace of the running case.
var sink *[]string

func rec(line string) {
	if sink != nil {
		*sink = append(*sink, line)
	}
}

var stackRe = regexp.MustCompile(`\n {4}at [\s\S]*$`)

// special renders core's own values.
func special(v any) (string, bool) {
	switch x := v.(type) {
	case *TimeoutError:
		return "TimeoutError(Error: " + x.Message + ")", true
	case builtinFunction:
		return "fn", true
	}
	return "", false
}

func enc(v any) string { return parityspec.Enc(v, special) }

// thrown renders a panic as the oracle renders a thrown value.
func thrown(r any) string {
	switch e := r.(type) {
	case *jsvalue.TypeError:
		return "throws TypeError: " + e.Message
	case *jsvalue.RangeError:
		return "throws RangeError: " + e.Message
	case *jsvalue.URIError:
		return "throws URIError: " + e.Message
	case *TimeoutError:
		return "throws Error: " + e.Message
	case error:
		return "throws Error: " + e.Error()
	}
	return "throws " + enc(r)
}

// recorder is a DiagLogger recording each call.
type recorder struct{ label string }

func (r recorder) log(level, message string, args []any) {
	parts := []string{enc(stackRe.ReplaceAllString(message, ""))}
	for _, a := range args {
		if s, ok := a.(string); ok {
			a = stackRe.ReplaceAllString(s, "")
		}
		parts = append(parts, enc(a))
	}
	rec(r.label + "." + level + "(" + strings.Join(parts, ", ") + ")")
}

func (r recorder) Error(m string, a ...any)   { r.log("error", m, a) }
func (r recorder) Warn(m string, a ...any)    { r.log("warn", m, a) }
func (r recorder) Info(m string, a ...any)    { r.log("info", m, a) }
func (r recorder) Debug(m string, a ...any)   { r.log("debug", m, a) }
func (r recorder) Verbose(m string, a ...any) { r.log("verbose", m, a) }

var diagRecorder = recorder{"diag"}

type recGetter struct{}

func (recGetter) Get(carrier any, key string) any {
	v := api.DefaultTextMapGetter.Get(carrier, key)
	rec("get " + key + " -> " + enc(v))
	return v
}

func (recGetter) Keys(carrier any) []string {
	k := api.DefaultTextMapGetter.Keys(carrier)
	rec("keys -> " + enc(k))
	return k
}

type recSetter struct{}

func (recSetter) Set(carrier any, key, value string) {
	rec("set " + key + " = " + enc(value))
	api.DefaultTextMapSetter.Set(carrier, key, value)
}

// stackManager is the oracle's StackContextManager.
type stackManager struct{ stack []api.Context }

func newStackManager() *stackManager { return &stackManager{stack: []api.Context{api.ROOT_CONTEXT}} }

func (m *stackManager) Active() api.Context { return m.stack[len(m.stack)-1] }

func (m *stackManager) With(ctx api.Context, fn func(args ...any) any, args ...any) any {
	m.stack = append(m.stack, ctx)
	defer func() { m.stack = m.stack[:len(m.stack)-1] }()
	return fn(args...)
}

func (m *stackManager) Bind(_ api.Context, target any) any { return target }
func (m *stackManager) Enable() api.ContextManager         { return m }
func (m *stackManager) Disable() api.ContextManager {
	m.stack = []api.Context{api.ROOT_CONTEXT}
	return m
}

func level(l api.DiagLogLevel) *api.DiagLogLevel { return &l }

// reset is the oracle's reset: the globals disabled, the diag recorder registered at ALL, the default error handler.
func reset() {
	sink = nil
	api.Trace.Disable()
	api.Ctx.Disable()
	api.Propagation.Disable()
	api.Metrics.Disable()
	api.Diag.SetLogger(diagRecorder, api.DiagLoggerOptions{LogLevel: level(api.DiagLogLevel_ALL), SuppressOverrideMessage: true})
	SetGlobalErrorHandler(LoggingErrorHandler())
}

// runCase runs fn on a reset state and returns its trace; a panic is recorded as the oracle records an exception.
func runCase(fn func()) (trace []string) {
	reset()
	trace = []string{}
	sink = &trace
	defer func() {
		if r := recover(); r != nil {
			rec("case " + thrown(r))
		}
		sink = nil
	}()
	fn()
	return trace
}

// encSC is the oracle's encSC; null is how a missing span context prints.
func encSC(sc *api.SpanContext, null string) string {
	if sc == nil {
		return null
	}
	state := "none"
	if sc.TraceState != nil {
		state = enc(sc.TraceState.Serialize())
	}
	return fmt.Sprintf("sc(%s,%s,%s,remote=%t,state=%s)", enc(sc.TraceId), enc(sc.SpanId),
		parityspec.EncNumber(float64(sc.TraceFlags)), sc.IsRemote, state)
}

func encEntry(e *api.BaggageEntry) string {
	if e == nil {
		return "undefined"
	}
	s := enc(e.Value)
	if e.Metadata != nil {
		s += ";" + enc(e.Metadata.ToString())
	}
	return s
}

func encBaggage(b api.Baggage) string {
	if b == nil {
		return "undefined"
	}
	parts := []string{}
	for _, p := range b.GetAllEntries() {
		e := p.Entry
		parts = append(parts, enc(p.Key)+"="+encEntry(&e))
	}
	return "[" + strings.Join(parts, ",") + "]"
}

// scSpec, ctxSpec: the oracle's buildContext input.
type scSpec struct {
	TraceId, SpanId string
	TraceFlags      json.RawMessage
	TraceState      *string
}

type ctxSpec struct {
	Sc         *scSpec
	Baggage    [][]string
	Suppressed bool
}

func buildContext(c ctxSpec) api.Context {
	ctx := api.ROOT_CONTEXT
	if c.Sc != nil {
		sc := api.SpanContext{TraceId: c.Sc.TraceId, SpanId: c.Sc.SpanId}
		if c.Sc.TraceFlags != nil {
			sc.TraceFlags = api.TraceFlags(jsvalue.Float(build(c.Sc.TraceFlags)))
		}
		if c.Sc.TraceState != nil {
			sc.TraceState = NewTraceState(*c.Sc.TraceState)
		}
		ctx = api.Trace.SetSpanContext(ctx, sc)
	}
	if c.Baggage != nil {
		var pairs []api.BaggageEntryPair
		for _, e := range c.Baggage {
			pairs = append(pairs, api.BaggageEntryPair{Key: e[0], Entry: entry(e)})
		}
		ctx = api.Propagation.SetBaggage(ctx, api.Propagation.CreateBaggage(pairs...))
	}
	if c.Suppressed {
		ctx = SuppressTracing(ctx)
	}
	return ctx
}

// entry builds a baggage entry from [key, value, metadata?].
func entry(e []string) api.BaggageEntry {
	be := api.BaggageEntry{Value: e[1]}
	if len(e) > 2 {
		be.Metadata = api.BaggageEntryMetadataFromString(e[2])
	}
	return be
}

var noFieldsKey = api.CreateContextKey("noFields")

func encContext(ctx api.Context) string {
	if ctx == api.ROOT_CONTEXT {
		return "same context"
	}
	return "span " + encSC(api.Trace.GetSpanContext(ctx), "undefined") + " baggage " + encBaggage(api.Propagation.GetBaggage(ctx)) +
		" noFields=" + enc(ctx.GetValue(noFieldsKey))
}

func build(raw json.RawMessage) any { return parityspec.BuildJS(raw, nil) }

// orderedObject decodes a JSON object of strings into a *jsvalue.Object, keeping the key order.
func orderedObject(raw json.RawMessage) *jsvalue.Object {
	dec := json.NewDecoder(bytes.NewReader(raw))
	o := &jsvalue.Object{}
	if _, err := dec.Token(); err != nil {
		panic(err)
	}
	for dec.More() {
		k, _ := dec.Token()
		var v string
		if err := dec.Decode(&v); err != nil {
			panic(err)
		}
		o.Set(k.(string), v)
	}
	return o
}

func loadJSON(t *testing.T, path string, v any) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatal(path, err)
	}
}

// checks counts the trace lines compared.
type checks map[string]int

// compare diffs the traces of one scenario, line by line.
func (c checks) compare(t *testing.T, scenario string, inputs []json.RawMessage, want, got [][]string) {
	t.Helper()
	if len(want) != len(got) {
		t.Errorf("%s: %d recorded cases, %d run", scenario, len(want), len(got))
		return
	}
	for i := range want {
		c[scenario] += len(want[i])
		input := ""
		if inputs != nil && i < len(inputs) {
			input = string(inputs[i])
			if len(input) > 200 {
				input = input[:200] + "…"
			}
		}
		n := len(want[i])
		if len(got[i]) > n {
			n = len(got[i])
		}
		for j := 0; j < n; j++ {
			w, g := "<none>", "<none>"
			if j < len(want[i]) {
				w = want[i][j]
			}
			if j < len(got[i]) {
				g = got[i][j]
			}
			if w != g {
				t.Errorf("%s #%d %s, line %d:\n--- js %s\n+++ go %s", scenario, i, input, j, clip(w), clip(g))
				break
			}
		}
	}
}

func clip(s string) string {
	if len(s) > 600 {
		return s[:600] + "…"
	}
	return s
}

var errBadOp = errors.New("bad op")
