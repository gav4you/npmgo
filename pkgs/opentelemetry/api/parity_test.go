package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode"

	"github.com/gav4you/npmgo/internal/jsvalue"
	"github.com/gav4you/npmgo/internal/parityspec"
)

const (
	casesPath    = "../../../parity/opentelemetry/cases.json"
	expectedPath = "../../../parity/opentelemetry/expected.json"
	tid          = "4bf92f3577b34da6a3ce929d0e0e4736"
	sid          = "00f067aa0ba902b7"
)

// --- the recording machinery (the Go twin of parity/oracle_opentelemetry.cjs's helpers)

var sink *[]string

func rec(line string) {
	if sink != nil {
		*sink = append(*sink, line)
	}
}

var stackRe = regexp.MustCompile(`\n {4}at [\s\S]*$`)

func enc(v any) string { return parityspec.Enc(v, nil) }

func thrown(r any) string {
	switch e := r.(type) {
	case *jsvalue.TypeError:
		return "throws TypeError: " + e.Message
	case error:
		return "throws Error: " + e.Error()
	}
	return "throws " + enc(r)
}

type recorder struct{ label string }

func (r recorder) log(level, message string, args []any) {
	parts := []string{enc(stackRe.ReplaceAllString(message, ""))}
	for _, a := range args {
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

type stackManager struct{ stack []Context }

func newStackManager() *stackManager { return &stackManager{stack: []Context{ROOT_CONTEXT}} }

func (m *stackManager) Active() Context { return m.stack[len(m.stack)-1] }
func (m *stackManager) With(ctx Context, fn func(args ...any) any, args ...any) any {
	m.stack = append(m.stack, ctx)
	defer func() { m.stack = m.stack[:len(m.stack)-1] }()
	return fn(args...)
}
func (m *stackManager) Bind(_ Context, target any) any { return target }
func (m *stackManager) Enable() ContextManager         { return m }
func (m *stackManager) Disable() ContextManager {
	m.stack = []Context{ROOT_CONTEXT}
	return m
}

func level(l DiagLogLevel) *DiagLogLevel { return &l }

// reset is the oracle's reset, the registry record deleted first.
func reset() {
	sink = nil
	globalMu.Lock()
	global = nil
	globalMu.Unlock()
	Trace.Disable()
	Ctx.Disable()
	Propagation.Disable()
	Metrics.Disable()
	Diag.SetLogger(diagRecorder, DiagLoggerOptions{LogLevel: level(DiagLogLevel_ALL), SuppressOverrideMessage: true})
}

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

func encSC(sc *SpanContext, null string) string {
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

func encEntry(e *BaggageEntry) string {
	if e == nil {
		return "undefined"
	}
	s := enc(e.Value)
	if e.Metadata != nil {
		s += ";" + enc(e.Metadata.ToString())
	}
	return s
}

func encEntries(entries []BaggageEntryPair) string {
	parts := []string{}
	for _, p := range entries {
		e := p.Entry
		parts = append(parts, enc(p.Key)+"="+encEntry(&e))
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func encBaggage(b Baggage) string {
	if b == nil {
		return "undefined"
	}
	return encEntries(b.GetAllEntries())
}

// encVersion renders an optional version or schema: "" stands for undefined.
func encVersion(v string) string {
	if v == "" {
		return "undefined"
	}
	return enc(v)
}

func encTracerOptions(o *TracerOptions) string {
	if o == nil {
		return "undefined"
	}
	return `{"schemaUrl":` + enc(o.SchemaUrl) + "}"
}

func build(raw json.RawMessage) any { return parityspec.BuildJS(raw, nil) }

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

func validSC() SpanContext { return SpanContext{TraceId: tid, SpanId: sid, TraceFlags: 1} }

// --- test providers, tracers, propagators

type testTracer struct{ label, provider string }

func (t *testTracer) StartSpan(name string, _ *SpanOptions, _ Context) Span {
	rec(t.provider + " tracer startSpan " + enc(name))
	return wrapSpanContext(SpanContext{TraceId: strings.Repeat("3", 32), SpanId: strings.Repeat("4", 16), TraceFlags: 1})
}

func (t *testTracer) StartActiveSpan(name string, args ...any) any {
	rec(fmt.Sprintf("%s tracer startActiveSpan %s with %d more arguments", t.provider, enc(name), len(args)))
	return "delegated"
}

type testTracerProvider struct{ name string }

func (p *testTracerProvider) GetTracer(n, v string, o *TracerOptions) Tracer {
	rec(fmt.Sprintf("%s.getTracer(%s, %s, %s)", p.name, enc(n), encVersion(v), encTracerOptions(o)))
	if p.name == "empty" {
		return nil
	}
	return &testTracer{label: p.name + " tracer", provider: p.name}
}

type labelledMeter struct {
	Meter
	label string
}

type testMeterProvider struct{ name, label string }

func (p *testMeterProvider) GetMeter(n, v string, o *MeterOptions) Meter {
	opt := "undefined"
	if o != nil {
		opt = `{"schemaUrl":` + enc(o.SchemaUrl) + "}"
	}
	rec(fmt.Sprintf("%s.getMeter(%s, %s, %s)", p.name, enc(n), encVersion(v), opt))
	return &labelledMeter{label: p.label}
}

type fieldsPropagator struct{ name string }

func (fieldsPropagator) Inject(Context, any, TextMapSetter)                  {}
func (fieldsPropagator) Extract(ctx Context, _ any, _ TextMapGetter) Context { return ctx }
func (p fieldsPropagator) Fields() []string                                  { return []string{p.name + "-field"} }

func describeProvider(p TracerProvider) string {
	pp, ok := p.(*ProxyTracerProvider)
	if !ok {
		return "other"
	}
	switch d := pp.GetDelegate().(type) {
	case *testTracerProvider:
		return "proxy delegate=" + d.name
	case noopTracerProvider:
		return "proxy delegate=NoopTracerProvider"
	}
	return "proxy delegate=?"
}

func describeTracer(t Tracer, withOptions bool) string {
	switch x := t.(type) {
	case *ProxyTracer:
		if withOptions {
			return fmt.Sprintf("proxy(%s, %s, %s)", enc(x.Name), encVersion(x.Version), encTracerOptions(x.Options))
		}
		return fmt.Sprintf("proxy(%s, %s)", enc(x.Name), encVersion(x.Version))
	case *testTracer:
		return x.label
	}
	return reflect.TypeOf(t).String()
}

// TestParity diffs @opentelemetry/api 1.9.0 as recorded by parity/oracle_opentelemetry.cjs against the port.
func TestParity(t *testing.T) {
	var cases struct {
		API map[string][]json.RawMessage `json:"api"`
	}
	var expected struct {
		API map[string]json.RawMessage `json:"api"`
	}
	loadJSON(t, casesPath, &cases)
	loadJSON(t, expectedPath, &expected)
	checks := map[string]int{}
	compare := func(name string, inputs []json.RawMessage, got [][]string) {
		var want [][]string
		if err := json.Unmarshal(expected.API[name], &want); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(want) != len(got) {
			t.Errorf("%s: %d recorded cases, %d run", name, len(want), len(got))
			return
		}
		for i := range want {
			checks[name] += len(want[i])
			input := ""
			if i < len(inputs) {
				var b bytes.Buffer
				json.Compact(&b, inputs[i])
				input = b.String()
			}
			if strings.Join(want[i], "\n") == strings.Join(got[i], "\n") {
				continue
			}
			for j := 0; j < len(want[i]) || j < len(got[i]); j++ {
				w, g := "<none>", "<none>"
				if j < len(want[i]) {
					w = want[i][j]
				}
				if j < len(got[i]) {
					g = got[i][j]
				}
				if w != g {
					t.Errorf("%s #%d %s, line %d:\n--- js %s\n+++ go %s", name, i, input, j, w, g)
					break
				}
			}
		}
	}
	run := func(name string, fn func(raw json.RawMessage)) {
		inputs := cases.API[name]
		got := make([][]string, len(inputs))
		for i, raw := range inputs {
			got[i] = runCase(func() { fn(raw) })
		}
		compare(name, inputs, got)
	}
	decode := func(raw json.RawMessage, v any) {
		if err := json.Unmarshal(raw, v); err != nil {
			panic(err)
		}
	}

	run("context", func(raw json.RawMessage) {
		var ops [][]json.RawMessage
		decode(raw, &ops)
		ctx := ROOT_CONTEXT
		for _, op := range ops {
			var name, key string
			decode(op[0], &name)
			decode(op[1], &key)
			k := CreateContextKey(key)
			switch name {
			case "set":
				prev := ctx
				v := build(op[2])
				ctx = ctx.SetValue(k, v)
				rec(fmt.Sprintf("set %s %s -> %s; before -> %s", key, enc(v), sameOrNew(ctx == prev), enc(prev.GetValue(k))))
			case "get":
				rec("get " + key + " -> " + enc(ctx.GetValue(k)))
			case "delete":
				prev := ctx
				ctx = ctx.DeleteValue(k)
				rec("delete " + key + " -> " + sameOrNew(ctx == prev))
			case "rootGet":
				rec("root get " + key + " -> " + enc(ROOT_CONTEXT.GetValue(k)))
			}
		}
	})

	run("keys", func(raw json.RawMessage) {
		var pair [2]string
		decode(raw, &pair)
		ka, kb := CreateContextKey(pair[0]), CreateContextKey(pair[1])
		rec(fmt.Sprintf("symbol %s same=%t", ka.String(), ka == kb))
	})

	rawRecorder := recorder{"raw"}
	run("registry", func(raw json.RawMessage) {
		var ops [][]json.RawMessage
		decode(raw, &ops)
		for _, op := range ops {
			var name, a string
			var b json.RawMessage
			decode(op[0], &name)
			if len(op) > 1 {
				decode(op[1], &a)
			}
			if len(op) > 2 {
				b = op[2]
			}
			switch name {
			case "setTracer":
				rec("setGlobalTracerProvider -> " + strconv.FormatBool(Trace.SetGlobalTracerProvider(&testTracerProvider{a})))
			case "getTracerProvider":
				rec("getTracerProvider -> " + describeProvider(Trace.GetTracerProvider()))
			case "getTracer":
				var v string
				decode(b, &v)
				rec("getTracer -> " + describeTracer(Trace.GetTracer(a, v), false))
			case "disableTrace":
				Trace.Disable()
				rec("trace disabled")
			case "setPropagator":
				rec("setGlobalPropagator -> " + strconv.FormatBool(Propagation.SetGlobalPropagator(fieldsPropagator{a})))
			case "propagationFields":
				rec("fields -> " + enc(Propagation.Fields()))
			case "disablePropagation":
				Propagation.Disable()
				rec("propagation disabled")
			case "setMeter":
				rec("setGlobalMeterProvider -> " + strconv.FormatBool(Metrics.SetGlobalMeterProvider(&testMeterProvider{a, a + " meter"})))
			case "getMeter":
				var v string
				decode(b, &v)
				m := Metrics.GetMeter(a, v, nil)
				if m == CreateNoopMeter() {
					rec("getMeter -> noop meter")
				} else {
					rec("getMeter -> " + m.(*labelledMeter).label)
				}
			case "disableMetrics":
				Metrics.Disable()
				rec("metrics disabled")
			case "setManager":
				rec("setGlobalContextManager -> " + strconv.FormatBool(Ctx.SetGlobalContextManager(newStackManager())))
			case "active":
				if Ctx.Active() == ROOT_CONTEXT {
					rec("active -> root")
				} else {
					rec("active -> other")
				}
			case "disableContext":
				Ctx.Disable()
				rec("context disabled")
			case "tamper":
				var withDiag bool
				decode(b, &withDiag)
				r := &globalRecord{version: a, entries: map[string]any{}}
				suffix := ""
				if withDiag {
					r.entries["diag"] = DiagLogger(rawRecorder)
					suffix = " with a diag logger"
				}
				globalMu.Lock()
				global = r
				globalMu.Unlock()
				rec("registry replaced: version " + enc(a) + suffix)
			case "diagError":
				Diag.Error("direct error")
				rec("diag.error called")
			default:
				panic("bad op " + name)
			}
		}
	})

	run("semver", func(raw json.RawMessage) {
		var pair [2]string
		decode(raw, &pair)
		check := makeCompatibilityCheck(pair[0])
		rec(fmt.Sprintf("%s %s -> %t %t", enc(pair[0]), enc(pair[1]), check(pair[1]), check(pair[1])))
	})

	run("diag", func(raw json.RawMessage) {
		var spec struct {
			Setup     [][]json.RawMessage
			Logs      bool
			Component []string
			Console   string
		}
		decode(raw, &spec)
		L := recorder{"L"}
		savedConsole := Console
		defer func() { Console = savedConsole }()
		if spec.Console != "" {
			methods := map[string]bool{}
			switch spec.Console {
			case "full":
				methods = map[string]bool{"error": true, "warn": true, "info": true, "debug": true, "trace": true, "log": true}
			case "noDebug":
				methods = map[string]bool{"error": true, "warn": true, "info": true, "trace": true, "log": true}
			case "logOnly":
				methods = map[string]bool{"log": true}
			}
			method := func(name string) func(...any) {
				if !methods[name] {
					return nil
				}
				return func(args ...any) {
					parts := make([]string, len(args))
					for i, a := range args {
						parts[i] = enc(a)
					}
					rec("console." + name + "(" + strings.Join(parts, ", ") + ")")
				}
			}
			Console = &ConsoleMethods{Error: method("error"), Warn: method("warn"), Info: method("info"), Debug: method("debug"),
				Trace: method("trace"), Log: method("log")}
		}
		levels := map[string]DiagLogLevel{"NONE": DiagLogLevel_NONE, "ERROR": DiagLogLevel_ERROR, "WARN": DiagLogLevel_WARN,
			"INFO": DiagLogLevel_INFO, "DEBUG": DiagLogLevel_DEBUG, "VERBOSE": DiagLogLevel_VERBOSE, "ALL": DiagLogLevel_ALL}
		for _, step := range spec.Setup {
			var op, kind string
			decode(step[0], &op)
			if op == "disable" {
				Diag.Disable()
				rec("disable")
				continue
			}
			decode(step[1], &kind)
			var logger DiagLogger = L
			var args []any
			switch kind {
			case "level":
				var l string
				decode(step[2], &l)
				args = []any{levels[l]}
			case "number":
				args = []any{jsvalue.Float(build(step[2]))}
			case "options":
				var o struct {
					LogLevel                *string
					SuppressOverrideMessage *bool
				}
				decode(step[2], &o)
				opts := DiagLoggerOptions{}
				if o.LogLevel != nil {
					opts.LogLevel = level(levels[*o.LogLevel])
				}
				if o.SuppressOverrideMessage != nil {
					opts.SuppressOverrideMessage = *o.SuppressOverrideMessage
				}
				args = []any{opts}
			case "default":
			case "null":
				args = []any{nil}
			case "self":
				logger = Diag
			case "nullLogger":
				var l string
				decode(step[2], &l)
				logger, args = nil, []any{levels[l]}
			case "console":
				var l string
				decode(step[2], &l)
				logger, args = NewDiagConsoleLogger(), []any{levels[l]}
			}
			func() {
				defer func() {
					if r := recover(); r != nil {
						rec("setLogger(" + kind + ") " + thrown(r))
					}
				}()
				rec("setLogger(" + kind + ") -> " + strconv.FormatBool(Diag.SetLogger(logger, args...)))
			}()
		}
		obj := jsvalue.NewObject("a", 1.0)
		if spec.Logs {
			Diag.Verbose("verbose message", 1.0, obj)
			Diag.Debug("debug message", 1.0, obj)
			Diag.Info("info message", 1.0, obj)
			Diag.Warn("warn message", 1.0, obj)
			Diag.Error("error message", 1.0, obj)
		}
		for _, ns := range spec.Component {
			cl := Diag.CreateComponentLogger(ComponentLoggerOptions{Namespace: ns})
			cl.Verbose("verbose from component", 2.0)
			cl.Debug("debug from component", 2.0)
			cl.Info("info from component", 2.0)
			cl.Warn("warn from component", 2.0)
			cl.Error("error from component", 2.0)
		}
	})

	run("baggage", func(raw json.RawMessage) {
		var ops [][]json.RawMessage
		decode(raw, &ops)
		var b, prev Baggage
		entry := func(e []string) BaggageEntry {
			be := BaggageEntry{Value: e[1]}
			if len(e) > 2 {
				be.Metadata = BaggageEntryMetadataFromString(e[2])
			}
			return be
		}
		for _, op := range ops {
			var name string
			decode(op[0], &name)
			str := func(i int) string {
				var s string
				decode(op[i], &s)
				return s
			}
			switch name {
			case "create":
				var es [][]string
				decode(op[1], &es)
				var pairs []BaggageEntryPair
				for _, e := range es {
					pairs = append(pairs, BaggageEntryPair{Key: e[0], Entry: entry(e)})
				}
				b = Propagation.CreateBaggage(pairs...)
				rec("created")
			case "all":
				rec("all -> " + encEntries(b.GetAllEntries()))
			case "get":
				rec("get " + str(1) + " -> " + encEntry(b.GetEntry(str(1))))
			case "mutateEntry":
				b.GetEntry(str(1)).Value = "mutated"
				rec("mutated a copy")
			case "set":
				prev = b
				b = b.SetEntry(str(1), BaggageEntry{Value: str(2)})
				rec("set -> " + sameOrNew(b == prev))
			case "prev":
				rec("previous all -> " + encEntries(prev.GetAllEntries()))
			case "remove":
				prev = b
				b = b.RemoveEntry(str(1))
				rec("remove -> " + sameOrNew(b == prev))
			case "removeMany":
				var keys []string
				decode(op[1], &keys)
				prev = b
				b = b.RemoveEntries(keys...)
				rec("removeMany -> " + sameOrNew(b == prev))
			case "clear":
				prev = b
				b = b.Clear()
				rec("clear -> " + sameOrNew(b == prev))
			case "meta":
				m := BaggageEntryMetadataFromString(build(op[1]))
				rec("meta -> " + enc(m.ToString()) + " branded")
			case "context":
				bb := Propagation.CreateBaggage(BaggageEntryPair{Key: "a", Entry: BaggageEntry{Value: "1"}})
				ctx := Propagation.SetBaggage(ROOT_CONTEXT, bb)
				if Propagation.GetBaggage(ctx) == bb {
					rec("get -> the baggage")
				} else {
					rec("get -> other")
				}
				rec("root -> " + encBaggage(Propagation.GetBaggage(ROOT_CONTEXT)))
				ctx = Propagation.DeleteBaggage(ctx)
				rec("deleted -> " + encBaggage(Propagation.GetBaggage(ctx)))
				rec("active -> " + encBaggage(Propagation.GetActiveBaggage()))
				Ctx.SetGlobalContextManager(newStackManager())
				Ctx.With(Propagation.SetBaggage(ROOT_CONTEXT, bb), func(...any) any {
					rec("active inside -> " + encBaggage(Propagation.GetActiveBaggage()))
					return nil
				})
				for _, falsy := range []any{0.0, "", false, nil} {
					fctx := ROOT_CONTEXT.SetValue(CreateContextKey("OpenTelemetry Baggage Key"), falsy)
					rec("falsy " + enc(falsy) + " -> " + encBaggage(Propagation.GetBaggage(fctx)))
				}
			default:
				panic("bad op " + name)
			}
		}
	})

	run("propagation", func(raw json.RawMessage) {
		var spec struct {
			Carrier json.RawMessage
			Global  bool
			Set     [][2]string
		}
		decode(raw, &spec)
		carrier := build(spec.Carrier)
		g, s := DefaultTextMapGetter, DefaultTextMapSetter
		rec("keys -> " + enc(g.Keys(carrier)))
		for _, k := range []string{"a", "b", "2", "missing"} {
			rec("get " + k + " -> " + enc(g.Get(carrier, k)))
		}
		s.Set(carrier, "new", "v")
		s.Set(carrier, "a", "replaced")
		for _, kv := range spec.Set {
			s.Set(carrier, kv[0], kv[1])
		}
		rec("after set -> " + enc(carrier))
		key := CreateContextKey("extracted")
		if spec.Global {
			rec("setGlobalPropagator -> " + strconv.FormatBool(Propagation.SetGlobalPropagator(recordingPropagator{key})))
		}
		rec("fields -> " + enc(Propagation.Fields()))
		ctx := Propagation.Extract(ROOT_CONTEXT, carrier, nil)
		if ctx == ROOT_CONTEXT {
			rec("extract -> same")
		} else {
			rec("extract -> value " + enc(ctx.GetValue(key)))
		}
		Propagation.Inject(ROOT_CONTEXT, carrier, nil)
		rec("after inject -> " + enc(carrier))
	})

	run("traceIds", func(raw json.RawMessage) {
		var id string
		decode(raw, &id)
		rec(fmt.Sprintf("traceId %t spanId %t", IsValidTraceId(id), IsValidSpanId(id)))
	})

	run("trace", func(raw json.RawMessage) {
		var spec []string
		decode(raw, &spec)
		tracePrograms[spec[0]]()
	})

	traceStates := []json.RawMessage{}
	for _, raw := range cases.API["traceState"] {
		var spec struct{ Raw json.RawMessage }
		decode(raw, &spec)
		if bytes.HasPrefix(bytes.TrimSpace(spec.Raw), []byte(`"`)) || bytes.Contains(spec.Raw, []byte(`"undef"`)) {
			traceStates = append(traceStates, raw)
		}
	}
	{
		got := make([][]string, len(traceStates))
		for i, raw := range traceStates {
			raw := raw
			got[i] = runCase(func() { runTraceStateOps(raw) })
		}
		compare("traceState", traceStates, got)
	}

	run("metrics", func(raw json.RawMessage) {
		var spec []string
		decode(raw, &spec)
		metricsPrograms[spec[0]]()
	})

	checkStatic(t, expected.API["static"], checks)

	total := 0
	for _, n := range checks {
		total += n
	}
	t.Logf("%d trace lines and static checks: %v", total, checks)
}

func sameOrNew(same bool) string {
	if same {
		return "same"
	}
	return "new"
}

type recordingPropagator struct{ key jsvalue.Symbol }

func (recordingPropagator) Inject(_ Context, carrier any, setter TextMapSetter) {
	rec("propagator inject")
	setter.Set(carrier, "injected", "1")
}

func (p recordingPropagator) Extract(ctx Context, carrier any, getter TextMapGetter) Context {
	rec("propagator extract " + enc(getter.Get(carrier, "traceparent")))
	return ctx.SetValue(p.key, "yes")
}

func (recordingPropagator) Fields() []string { return []string{"f1", "f2"} }

func runTraceStateOps(raw json.RawMessage) {
	var spec struct {
		Raw json.RawMessage
		Ops [][]string
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		panic(err)
	}
	r, _ := build(spec.Raw).(string)
	ts := CreateTraceState(r)
	for _, op := range spec.Ops {
		switch op[0] {
		case "serialize":
			rec("serialize -> " + enc(ts.Serialize()))
		case "get":
			v, ok := ts.Get(op[1])
			got := "undefined"
			if ok {
				got = enc(v)
			}
			rec("get " + op[1] + " -> " + got)
		case "set":
			n := ts.Set(op[1], op[2])
			rec("set " + op[1] + " " + enc(op[2]) + " -> " + sameOrNew(n == ts))
			ts = n
		case "unset":
			n := ts.Unset(op[1])
			rec("unset " + op[1] + " -> " + sameOrNew(n == ts))
			ts = n
		}
	}
}

var invalidSC = SpanContext{TraceId: strings.Repeat("0", 32), SpanId: sid, TraceFlags: 1}

func spanOf(s Span) string {
	sc := s.SpanContext()
	return encSC(&sc, "undefined")
}

var tracePrograms = map[string]func(){
	"startSpan root": func() {
		s := Trace.GetTracer("t", "").StartSpan("x", &SpanOptions{Root: true}, Trace.SetSpanContext(ROOT_CONTEXT, validSC()))
		rec(fmt.Sprintf("span %s recording=%t", spanOf(s), s.IsRecording()))
	},
	"startSpan valid parent": func() {
		t := Trace.GetTracer("t", "")
		rec("span " + spanOf(t.StartSpan("x", &SpanOptions{}, Trace.SetSpanContext(ROOT_CONTEXT, validSC()))))
		sc := validSC()
		sc.TraceState = CreateTraceState("a=1")
		rec("span " + spanOf(t.StartSpan("x", nil, Trace.SetSpanContext(ROOT_CONTEXT, sc))))
	},
	"startSpan invalid parent": func() {
		t := Trace.GetTracer("t", "")
		rec("span " + spanOf(t.StartSpan("x", &SpanOptions{}, Trace.SetSpanContext(ROOT_CONTEXT, invalidSC))))
		rec("span " + spanOf(t.StartSpan("x", &SpanOptions{}, ROOT_CONTEXT)))
	},
	"startSpan no context": func() {
		t := Trace.GetTracer("t", "")
		rec("span " + spanOf(t.StartSpan("x", nil, nil)))
		Ctx.SetGlobalContextManager(newStackManager())
		Ctx.With(Trace.SetSpanContext(ROOT_CONTEXT, validSC()), func(...any) any {
			rec("span " + spanOf(t.StartSpan("x", nil, nil)))
			return nil
		})
	},
	"startActiveSpan arities": func() {
		t := Trace.GetTracer("t", "")
		parent := Trace.SetSpanContext(ROOT_CONTEXT, validSC())
		fn := func(label string) func(Span) any {
			return func(span Span) any {
				rec(label + " fn " + spanOf(span))
				return label + " result"
			}
		}
		rec("no fn -> " + enc(t.StartActiveSpan("a")))
		rec("fn -> " + enc(t.StartActiveSpan("a", fn("2 args"))))
		rec("options -> " + enc(t.StartActiveSpan("a", &SpanOptions{Root: true}, fn("3 args"))))
		rec("context -> " + enc(t.StartActiveSpan("a", jsvalue.Undefined{}, parent, fn("4 args"))))
		rec("undefined context -> " + enc(t.StartActiveSpan("a", &SpanOptions{}, jsvalue.Undefined{}, fn("4 args, no context"))))
	},
	"proxy before delegate": func() {
		pp := NewProxyTracerProvider()
		t := pp.GetTracer("n", "v", &TracerOptions{SchemaUrl: "s"})
		rec("tracer " + describeTracer(t, true))
		rec("delegate NoopTracerProvider")
		if _, ok := pp.GetDelegate().(noopTracerProvider); !ok {
			panic("delegate is not the no-op provider")
		}
		rec("span " + spanOf(t.StartSpan("x", nil, nil)))
		pp.SetDelegate(&testTracerProvider{"late"})
		rec("span " + spanOf(t.StartSpan("y", nil, nil)))
		rec("span again " + spanOf(t.StartSpan("z", nil, nil)))
		rec("active " + enc(t.StartActiveSpan("w", func(Span) any { return "unused" })))
		rec("new tracer " + describeTracer(pp.GetTracer("m", "", nil), true))
		rec("delegate " + pp.GetDelegate().(*testTracerProvider).name)
	},
	"proxy after delegate": func() {
		pp := NewProxyTracerProvider()
		pp.SetDelegate(&testTracerProvider{"early"})
		rec("tracer " + describeTracer(pp.GetTracer("n", "v", nil), true))
		rec("delegate tracer " + describeTracer(pp.GetDelegateTracer("d", "e", &TracerOptions{SchemaUrl: "f"}), true))
	},
	"proxy delegate returns nothing": func() {
		pp := NewProxyTracerProvider()
		pp.SetDelegate(&testTracerProvider{"empty"})
		t := pp.GetTracer("n", "", nil)
		rec("tracer " + describeTracer(t, true))
		rec("span " + spanOf(t.StartSpan("x", nil, nil)))
		rec("span " + spanOf(t.StartSpan("y", nil, nil)))
	},
	"wrap and helpers": func() {
		sc := validSC()
		span := Trace.WrapSpanContext(sc)
		rec(fmt.Sprintf("wrapped %s recording=%t", spanOf(span), span.IsRecording()))
		ctx := Trace.SetSpan(ROOT_CONTEXT, span)
		rec("getSpan same -> " + strconv.FormatBool(Trace.GetSpan(ctx) == span))
		rec("getSpanContext -> " + encSC(Trace.GetSpanContext(ctx), "undefined"))
		ctx = Trace.DeleteSpan(ctx)
		after := "undefined"
		if Trace.GetSpan(ctx) != nil {
			after = "a span"
		}
		rec("after delete -> " + after + " " + encSC(Trace.GetSpanContext(ctx), "undefined"))
		active := "undefined"
		if Trace.GetActiveSpan() != nil {
			active = "a span"
		}
		rec("active span -> " + active)
		zeroSpan := sc
		zeroSpan.SpanId = strings.Repeat("0", 16)
		rec(fmt.Sprintf("valid -> %t %t %t", Trace.IsSpanContextValid(sc), IsSpanContextValid(invalidSC), IsSpanContextValid(zeroSpan)))
		ctx2 := Trace.SetSpanContext(ROOT_CONTEXT, sc)
		rec(fmt.Sprintf("setSpanContext -> %s recording=%t", encSC(Trace.GetSpanContext(ctx2), "undefined"), Trace.GetSpan(ctx2).IsRecording()))
		for _, falsy := range []any{0.0, "", false, nil} {
			fctx := ROOT_CONTEXT.SetValue(CreateContextKey("OpenTelemetry Context Key SPAN"), falsy)
			got := "undefined"
			if Trace.GetSpan(fctx) != nil {
				got = "a span"
			}
			rec("falsy " + enc(falsy) + " -> " + got)
		}
		inv := INVALID_SPAN_CONTEXT
		rec("invalid constant " + encSC(&inv, "undefined") + " " + enc(INVALID_SPANID) + " " + enc(INVALID_TRACEID))
	},
	"span methods": func() {
		span := Trace.WrapSpanContext(validSC())
		same := func(v Span) string {
			if v == span {
				return "this"
			}
			return "other"
		}
		rec("setAttribute " + same(span.SetAttribute("a", 1.0)))
		rec("setAttributes " + same(span.SetAttributes(jsvalue.NewObject("a", 1.0))))
		rec("addEvent " + same(span.AddEvent("e", jsvalue.NewObject("a", 1.0), 5.0)))
		rec("addLink " + same(span.AddLink(Link{Context: validSC()})))
		rec("addLinks " + same(span.AddLinks(nil)))
		rec("setStatus " + same(span.SetStatus(SpanStatus{Code: SpanStatusCode_ERROR, Message: "m"})))
		rec("updateName " + same(span.UpdateName("n")))
		span.End(5.0)
		rec("end undefined")
		span.RecordException(fmt.Errorf("x"))
		rec("recordException undefined")
		rec("isRecording " + strconv.FormatBool(span.IsRecording()))
		rec("noop span " + spanOf(Trace.GetTracer("t", "").StartSpan("x", nil, nil)))
	},
	"startActiveSpan with manager": func() {
		Ctx.SetGlobalContextManager(newStackManager())
		t := Trace.GetTracer("t", "")
		parent := Trace.SetSpanContext(ROOT_CONTEXT, validSC())
		r := t.StartActiveSpan("a", &SpanOptions{}, parent, func(span Span) any {
			rec("active span is the span -> " + strconv.FormatBool(Trace.GetActiveSpan() == span))
			rec("active span context " + encSC(Trace.GetSpanContext(Ctx.Active()), "undefined"))
			return "done"
		})
		active := "undefined"
		if Trace.GetActiveSpan() != nil {
			active = "a span"
		}
		rec("result " + enc(r) + " active after -> " + active)
	},
	"startActiveSpan bad fn": func() {
		t := Trace.GetTracer("t", "")
		for _, args := range [][]any{{&SpanOptions{}, jsvalue.Undefined{}, jsvalue.Undefined{}}, {5.0}, {&SpanOptions{}, nil}} {
			func() {
				defer func() {
					if r := recover(); r != nil {
						rec(fmt.Sprintf("args %d %s", len(args)+1, thrown(r)))
					}
				}()
				rec(fmt.Sprintf("args %d -> %s", len(args)+1, enc(t.StartActiveSpan("a", args...))))
			}()
		}
	},
	"global tracer": func() {
		rec("register -> " + strconv.FormatBool(Trace.SetGlobalTracerProvider(&testTracerProvider{"global"})))
		if _, ok := Trace.GetTracerProvider().(*ProxyTracerProvider); ok {
			rec("provider proxy")
		} else {
			rec("provider other")
		}
		rec("tracer " + describeTracer(Trace.GetTracer("n", "v"), true))
		Trace.Disable()
		rec("tracer after disable " + describeTracer(Trace.GetTracer("n", "v"), true))
		rec("register again -> " + strconv.FormatBool(Trace.SetGlobalTracerProvider(&testTracerProvider{"second"})))
		rec("tracer " + describeTracer(Trace.GetTracer("n", ""), true))
	},
}

// jsClassName is the JavaScript class name of a no-op instrument (the Go type name, capitalised).
func jsClassName(v any) string {
	n := reflect.TypeOf(v).Elem().Name()
	r := []rune(n)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

var metricsPrograms = map[string]func(){
	"noop": func() {
		m := CreateNoopMeter()
		rec("same meter -> " + strconv.FormatBool(m == CreateNoopMeter()))
		kinds := []struct {
			name string
			make func(name string, o *MetricOptions) any
		}{
			{"createCounter", func(n string, o *MetricOptions) any { return m.CreateCounter(n, o) }},
			{"createUpDownCounter", func(n string, o *MetricOptions) any { return m.CreateUpDownCounter(n, o) }},
			{"createGauge", func(n string, o *MetricOptions) any { return m.CreateGauge(n, o) }},
			{"createHistogram", func(n string, o *MetricOptions) any { return m.CreateHistogram(n, o) }},
			{"createObservableCounter", func(n string, o *MetricOptions) any { return m.CreateObservableCounter(n, o) }},
			{"createObservableGauge", func(n string, o *MetricOptions) any { return m.CreateObservableGauge(n, o) }},
			{"createObservableUpDownCounter", func(n string, o *MetricOptions) any { return m.CreateObservableUpDownCounter(n, o) }},
		}
		for _, k := range kinds {
			a, b := k.make("a", nil), k.make("b", &MetricOptions{Description: "d"})
			rec(fmt.Sprintf("%s -> shared=%t %s", k.name, a == b, jsClassName(a)))
		}
		rec("counter vs up-down -> " + strconv.FormatBool(any(m.CreateCounter("a", nil)) == any(m.CreateUpDownCounter("a", nil))))
		m.CreateCounter("a", nil).Add(1, jsvalue.NewObject("k", "v"), nil)
		rec("add -> undefined")
		m.CreateHistogram("a", nil).Record(1, nil, nil)
		rec("record -> undefined")
		m.CreateObservableGauge("a", nil).AddCallback(func(ObservableResult) any { return nil })
		rec("addCallback -> undefined")
		m.AddBatchObservableCallback(func(BatchObservableResult) any { return nil }, nil)
		rec("addBatchObservableCallback -> undefined")
		rec(fmt.Sprintf("provider meter -> %t %t", Metrics.GetMeterProvider().GetMeter("x", "", nil) == m, Metrics.GetMeter("y", "v", nil) == m))
	},
	"global": func() {
		provider := &testMeterProvider{"provider", "provided"}
		rec("register -> " + strconv.FormatBool(Metrics.SetGlobalMeterProvider(provider)))
		rec("provider is it -> " + strconv.FormatBool(Metrics.GetMeterProvider() == MeterProvider(provider)))
		rec("meter -> " + Metrics.GetMeter("n", "v", &MeterOptions{SchemaUrl: "s"}).(*labelledMeter).label)
		rec("register again -> " + strconv.FormatBool(Metrics.SetGlobalMeterProvider(provider)))
		Metrics.Disable()
		if Metrics.GetMeter("n", "", nil) == CreateNoopMeter() {
			rec("meter after disable -> noop")
		} else {
			rec("meter after disable -> other")
		}
	},
}

// enumEntries is Object.entries of a TypeScript numeric enum: the reverse mappings under their array-index keys,
// ascending, then the members in order.
func enumEntries(names []string, values []float64) string {
	type rev struct {
		idx  uint64
		name string
	}
	var revs []rev
	for i, n := range names {
		if k, ok := jsvalue.ArrayIndex(parityspec.EncNumber(values[i])); ok {
			revs = append(revs, rev{k, n})
		}
	}
	for i := 1; i < len(revs); i++ {
		for j := i; j > 0 && revs[j].idx < revs[j-1].idx; j-- {
			revs[j], revs[j-1] = revs[j-1], revs[j]
		}
	}
	var out [][2]any
	for _, r := range revs {
		out = append(out, [2]any{strconv.FormatUint(r.idx, 10), r.name})
	}
	for i, n := range names {
		out = append(out, [2]any{n, values[i]})
	}
	return enc(out)
}

// jsExports maps every export of require('@opentelemetry/api') to its Go translation.
var jsExports = map[string]string{
	"baggageEntryMetadataFromString": "BaggageEntryMetadataFromString", "createContextKey": "CreateContextKey",
	"ROOT_CONTEXT": "ROOT_CONTEXT", "DiagConsoleLogger": "DiagConsoleLogger", "DiagLogLevel": "DiagLogLevel",
	"createNoopMeter": "CreateNoopMeter", "ValueType": "ValueType", "defaultTextMapGetter": "DefaultTextMapGetter",
	"defaultTextMapSetter": "DefaultTextMapSetter", "ProxyTracer": "ProxyTracer", "ProxyTracerProvider": "ProxyTracerProvider",
	"SamplingDecision": "SamplingDecision", "SpanKind": "SpanKind", "SpanStatusCode": "SpanStatusCode", "TraceFlags": "TraceFlags",
	"createTraceState": "CreateTraceState", "isSpanContextValid": "IsSpanContextValid", "isValidTraceId": "IsValidTraceId",
	"isValidSpanId": "IsValidSpanId", "INVALID_SPANID": "INVALID_SPANID", "INVALID_TRACEID": "INVALID_TRACEID",
	"INVALID_SPAN_CONTEXT": "INVALID_SPAN_CONTEXT", "context": "Ctx", "diag": "Diag", "metrics": "Metrics",
	"propagation": "Propagation", "trace": "Trace",
	"default": "N/A: the ES-module default export ({context, diag, metrics, propagation, trace}); Go has the five variables",
}

func checkStatic(t *testing.T, raw json.RawMessage, checks map[string]int) {
	var s struct {
		Enums         map[string]string
		Invalid       []string
		Exports       []string
		DefaultExport []string
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatal(err)
	}
	inv := INVALID_SPAN_CONTEXT
	got := map[string][2]string{
		"SpanKind": {s.Enums["SpanKind"], enumEntries([]string{"INTERNAL", "SERVER", "CLIENT", "PRODUCER", "CONSUMER"},
			[]float64{float64(SpanKind_INTERNAL), float64(SpanKind_SERVER), float64(SpanKind_CLIENT), float64(SpanKind_PRODUCER), float64(SpanKind_CONSUMER)})},
		"SpanStatusCode": {s.Enums["SpanStatusCode"], enumEntries([]string{"UNSET", "OK", "ERROR"},
			[]float64{float64(SpanStatusCode_UNSET), float64(SpanStatusCode_OK), float64(SpanStatusCode_ERROR)})},
		"TraceFlags": {s.Enums["TraceFlags"], enumEntries([]string{"NONE", "SAMPLED"}, []float64{float64(TraceFlags_NONE), float64(TraceFlags_SAMPLED)})},
		"SamplingDecision": {s.Enums["SamplingDecision"], enumEntries([]string{"NOT_RECORD", "RECORD", "RECORD_AND_SAMPLED"},
			[]float64{float64(SamplingDecision_NOT_RECORD), float64(SamplingDecision_RECORD), float64(SamplingDecision_RECORD_AND_SAMPLED)})},
		"DiagLogLevel": {s.Enums["DiagLogLevel"], enumEntries([]string{"NONE", "ERROR", "WARN", "INFO", "DEBUG", "VERBOSE", "ALL"},
			[]float64{float64(DiagLogLevel_NONE), float64(DiagLogLevel_ERROR), float64(DiagLogLevel_WARN), float64(DiagLogLevel_INFO),
				float64(DiagLogLevel_DEBUG), float64(DiagLogLevel_VERBOSE), float64(DiagLogLevel_ALL)})},
		"ValueType": {s.Enums["ValueType"], enumEntries([]string{"INT", "DOUBLE"}, []float64{float64(ValueType_INT), float64(ValueType_DOUBLE)})},
		"invalid":   {enc(s.Invalid), enc([]string{INVALID_SPANID, INVALID_TRACEID, encSC(&inv, "undefined")})},
		"default":   {enc(s.DefaultExport), enc([]string{"context", "diag", "metrics", "propagation", "trace"})},
	}
	for k, v := range got {
		checks["static"]++
		if v[0] != v[1] {
			t.Errorf("static %s:\n--- js %s\n+++ go %s", k, v[0], v[1])
		}
	}
	for _, name := range s.Exports {
		checks["exports"]++
		if _, ok := jsExports[name]; !ok {
			t.Errorf("export %s is not accounted for", name)
		}
	}
	if len(s.Exports) != len(jsExports) {
		t.Errorf("the module has %d exports, the table %d", len(s.Exports), len(jsExports))
	}
}
