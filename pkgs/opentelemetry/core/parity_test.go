package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gav4you/npmgo/internal/jsloop"
	"github.com/gav4you/npmgo/internal/jspromise"
	"github.com/gav4you/npmgo/internal/jsvalue"
	"github.com/gav4you/npmgo/internal/parityspec"
	"github.com/gav4you/npmgo/pkgs/opentelemetry/api"
)

const (
	casesPath    = "../../../parity/opentelemetry/cases.json"
	expectedPath = "../../../parity/opentelemetry/expected.json"
	tid          = "4bf92f3577b34da6a3ce929d0e0e4736"
	sid          = "00f067aa0ba902b7"
)

func validSC() api.SpanContext { return api.SpanContext{TraceId: tid, SpanId: sid, TraceFlags: 1} }

// TestParity diffs @opentelemetry/core 2.11.0 as recorded by parity/oracle_opentelemetry.cjs against the port.
func TestParity(t *testing.T) {
	var cases struct {
		Core map[string][]json.RawMessage
	}
	var expected struct {
		Core map[string]json.RawMessage
	}
	loadJSON(t, casesPath, &cases)
	loadJSON(t, expectedPath, &expected)
	want := func(name string) [][]string {
		var w [][]string
		if err := json.Unmarshal(expected.Core[name], &w); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return w
	}
	c := checks{}
	run := func(name string, fn func(raw json.RawMessage)) {
		inputs := cases.Core[name]
		got := make([][]string, len(inputs))
		for i, raw := range inputs {
			got[i] = runCase(func() { fn(raw) })
		}
		c.compare(t, name, inputs, want(name), got)
	}
	decode := func(raw json.RawMessage, v any) {
		if err := json.Unmarshal(raw, v); err != nil {
			panic(err)
		}
	}

	run("traceparent", func(raw json.RawMessage) {
		var s string
		decode(raw, &s)
		rec(encSC(ParseTraceParent(s), "null"))
	})

	run("extract", func(raw json.RawMessage) {
		var spec map[string]json.RawMessage
		decode(raw, &spec)
		carrier := &jsvalue.Object{}
		for _, k := range []string{"traceparent", "tracestate"} {
			if v, ok := spec[k]; ok {
				carrier.Set(k, build(v))
			}
		}
		ctx := NewW3CTraceContextPropagator().Extract(api.ROOT_CONTEXT, carrier, recGetter{})
		if ctx == api.ROOT_CONTEXT {
			rec("same context")
		} else {
			rec(fmt.Sprintf("span %s recording=%t", encSC(api.Trace.GetSpanContext(ctx), "undefined"), api.Trace.GetSpan(ctx).IsRecording()))
		}
	})

	run("inject", func(raw json.RawMessage) {
		var spec struct {
			Sc         *scSpec
			Suppressed bool
			NoSpan     bool
		}
		decode(raw, &spec)
		cs := ctxSpec{Sc: spec.Sc, Suppressed: spec.Suppressed}
		if spec.NoSpan {
			cs.Sc = nil
		}
		carrier := &jsvalue.Object{}
		NewW3CTraceContextPropagator().Inject(buildContext(cs), carrier, recSetter{})
		rec("carrier " + enc(carrier))
	})

	run("traceState", func(raw json.RawMessage) {
		runTraceStateOps(raw, func(r any) api.TraceState {
			s, _ := r.(string) // JavaScript treats a non-string like no header
			return NewTraceState(s)
		})
	})

	run("baggageExtract", func(raw json.RawMessage) {
		var spec struct {
			Header json.RawMessage
			Absent bool
		}
		decode(raw, &spec)
		carrier := &jsvalue.Object{}
		if !spec.Absent {
			carrier.Set("baggage", build(spec.Header))
		}
		ctx := NewW3CBaggagePropagator().Extract(api.ROOT_CONTEXT, carrier, recGetter{})
		if ctx == api.ROOT_CONTEXT {
			rec("same context")
		} else {
			rec("baggage " + encBaggage(api.Propagation.GetBaggage(ctx)))
		}
	})

	run("baggageInject", func(raw json.RawMessage) {
		var spec struct {
			Entries    [][]string
			Via        string
			Suppressed bool
			NoBaggage  bool
		}
		decode(raw, &spec)
		ctx := api.ROOT_CONTEXT
		if !spec.NoBaggage {
			var b api.Baggage
			if spec.Via == "record" {
				var pairs []api.BaggageEntryPair
				for _, e := range spec.Entries {
					pairs = append(pairs, api.BaggageEntryPair{Key: e[0], Entry: entry(e)})
				}
				b = api.Propagation.CreateBaggage(pairs...)
			} else {
				b = api.Propagation.CreateBaggage()
				for _, e := range spec.Entries {
					b = b.SetEntry(e[0], entry(e))
				}
			}
			ctx = api.Propagation.SetBaggage(ctx, b)
		}
		if spec.Suppressed {
			ctx = SuppressTracing(ctx)
		}
		carrier := &jsvalue.Object{}
		NewW3CBaggagePropagator().Inject(ctx, carrier, recSetter{})
		rec("carrier " + enc(carrier))
	})

	run("parseRecord", func(raw json.RawMessage) { rec(enc(ParseKeyPairsIntoRecord(build(raw)))) })

	run("composite", func(raw json.RawMessage) {
		var spec struct {
			Propagators []string
			Op          string
			Ctx         ctxSpec
			Carrier     json.RawMessage
			NoConfig    bool
		}
		decode(raw, &spec)
		var comp *CompositePropagator
		if spec.NoConfig {
			comp = NewCompositePropagator(CompositePropagatorConfig{})
		} else {
			ps := []api.TextMapPropagator{}
			for _, name := range spec.Propagators {
				ps = append(ps, makePropagator(name))
			}
			comp = NewCompositePropagator(CompositePropagatorConfig{Propagators: ps})
		}
		switch spec.Op {
		case "fields":
			f := comp.Fields()
			rec("fields " + enc(f))
			f = append(f, "mutated")
			_ = f
			rec("fields again " + enc(comp.Fields()))
		case "inject":
			carrier := &jsvalue.Object{}
			comp.Inject(buildContext(spec.Ctx), carrier, recSetter{})
			rec("carrier " + enc(carrier))
		case "extract":
			rec(encContext(comp.Extract(api.ROOT_CONTEXT, orderedObject(spec.Carrier), recGetter{})))
		}
	})

	run("sanitize", func(raw json.RawMessage) {
		attrs := build(raw)
		result := SanitizeAttributes(attrs)
		rec("result " + enc(result))
		if jsvalue.TypeOf(attrs) == "object" && !jsvalue.IsNull(attrs) {
			for _, k := range result.Keys() {
				arr, ok := result.Get(k).([]any)
				if !ok || len(arr) == 0 {
					continue
				}
				var in any
				switch a := attrs.(type) {
				case *jsvalue.Object:
					in = a.Get(k)
				case []any:
					i, _ := strconv.Atoi(k)
					in = a[i]
				}
				state := "copied"
				if inArr, ok := in.([]any); ok && len(inArr) > 0 && &inArr[0] == &arr[0] {
					state = "shared"
				}
				rec("array " + k + " " + state)
			}
		}
	})

	run("isAttributeValue", func(raw json.RawMessage) { rec(strconv.FormatBool(IsAttributeValue(build(raw)))) })

	run("merge", func(raw json.RawMessage) {
		var specs []json.RawMessage
		decode(raw, &specs)
		refs := map[string]any{}
		args := make([]any, len(specs))
		for i, s := range specs {
			args[i] = parityspec.BuildJS(s, refs)
		}
		inputs := map[any]string{}
		var walk func(v any)
		walk = func(v any) {
			key, ok := identityKey(v)
			if !ok {
				return
			}
			if _, seen := inputs[key]; seen {
				return
			}
			inputs[key] = "in" + strconv.Itoa(len(inputs))
			switch x := v.(type) {
			case []any:
				for _, it := range x {
					walk(it)
				}
			case *jsvalue.Object:
				x.Range(func(_ string, it any) bool { walk(it); return true })
			case *parityspec.Instance:
				x.Props.Range(func(_ string, it any) bool { walk(it); return true })
			}
		}
		for _, a := range args {
			walk(a)
		}
		encAll := func() string {
			parts := make([]string, len(args))
			for i, a := range args {
				parts[i] = encGraph(a, nil)
			}
			return strings.Join(parts, " | ")
		}
		before := encAll()
		rec("result " + encGraph(Merge(args...), inputs))
		if after := encAll(); after == before {
			rec("inputs unchanged")
		} else {
			rec("inputs changed: " + after)
		}
	})

	run("time", func(raw json.RawMessage) {
		var spec struct {
			Perf struct {
				TimeOrigin json.RawMessage
				Now        []json.RawMessage
			}
			Calls [][]json.RawMessage
		}
		decode(raw, &spec)
		fake := &fakePerformance{origin: jsvalue.Float(build(spec.Perf.TimeOrigin))}
		for _, n := range spec.Perf.Now {
			fake.nows = append(fake.nows, jsvalue.Float(build(n)))
		}
		saved := Otperformance
		Otperformance = fake
		defer func() { Otperformance = saved }()
		for _, call := range spec.Calls {
			var fn string
			decode(call[0], &fn)
			args := make([]any, len(call)-1)
			for i, a := range call[1:] {
				args[i] = build(a)
			}
			func() {
				defer func() {
					if r := recover(); r != nil {
						rec(fn + " " + thrown(r))
					}
				}()
				rec(fn + " -> " + enc(callTime(fn, args)))
			}()
		}
	})

	run("env", func(raw json.RawMessage) {
		var spec struct {
			Env   map[string]string
			Calls [][2]string
		}
		decode(raw, &spec)
		saved := ProcessEnv
		ProcessEnv = func(k string) (string, bool) { v, ok := spec.Env[k]; return v, ok }
		defer func() { ProcessEnv = saved }()
		for _, call := range spec.Calls {
			fn, key := call[0], call[1]
			func() {
				defer func() {
					if r := recover(); r != nil {
						rec(fn + " " + key + " " + thrown(r))
					}
				}()
				var result string
				switch fn {
				case "getNumberFromEnv":
					result = "undefined"
					if v, ok := GetNumberFromEnv(key); ok {
						result = parityspec.EncNumber(v)
					}
				case "getStringFromEnv":
					result = "undefined"
					if v, ok := GetStringFromEnv(key); ok {
						result = enc(v)
					}
				case "getBooleanFromEnv":
					result = strconv.FormatBool(GetBooleanFromEnv(key))
				case "getStringListFromEnv":
					result = "undefined"
					if v, ok := GetStringListFromEnv(key); ok {
						result = enc(v)
					}
				default:
					panic(errBadOp)
				}
				rec(fn + " " + key + " -> " + result)
			}()
		}
	})

	run("diagLevel", func(raw json.RawMessage) {
		var value *string
		if s, ok := build(raw).(string); ok {
			value = &s
		}
		if l, ok := DiagLogLevelFromString(value); ok {
			rec(parityspec.EncNumber(float64(l)))
		} else {
			rec("undefined")
		}
	})

	run("urlMatches", func(raw json.RawMessage) {
		var spec [2]json.RawMessage
		decode(raw, &spec)
		var url string
		decode(spec[0], &url)
		rec(strconv.FormatBool(UrlMatches(url, matcher(spec[1]))))
	})

	run("urlIgnored", func(raw json.RawMessage) {
		var spec [2]json.RawMessage
		decode(raw, &spec)
		var url string
		decode(spec[0], &url)
		var list []json.RawMessage
		var ignored []any
		if json.Unmarshal(spec[1], &list) == nil {
			ignored = []any{}
			for _, m := range list {
				ignored = append(ignored, matcher(m))
			}
		}
		rec(strconv.FormatBool(IsUrlIgnored(url, ignored)))
	})

	run("errorHandler", func(raw json.RawMessage) {
		var spec struct {
			Handler    string
			Exceptions []json.RawMessage
		}
		decode(raw, &spec)
		switch spec.Handler {
		case "recording":
			SetGlobalErrorHandler(func(ex any) { rec("handler " + enc(ex)) })
		case "throwing":
			SetGlobalErrorHandler(func(any) { rec("handler throws"); panic(errors.New("handler failed")) })
		}
		for i, s := range spec.Exceptions {
			rec("exception " + strconv.Itoa(i))
			ex := buildException(s)
			if spec.Handler == "direct" {
				func() {
					defer func() {
						if r := recover(); r != nil {
							rec(thrown(r))
						}
					}()
					LoggingErrorHandler()(ex)
					rec("returned")
				}()
			} else {
				GlobalErrorHandler(ex)
				rec("returned")
			}
		}
	})

	run("anchoredClock", func(raw json.RawMessage) {
		var spec struct {
			System, Monotonic []json.RawMessage
			Calls             int
		}
		decode(raw, &spec)
		ac := NewAnchoredClock(&scriptedClock{name: "system", values: spec.System}, &scriptedClock{name: "monotonic", values: spec.Monotonic})
		for i := 0; i < spec.Calls; i++ {
			rec("now -> " + parityspec.EncNumber(ac.Now()))
		}
	})

	run("rpc", func(raw json.RawMessage) {
		var ops [][]json.RawMessage
		decode(raw, &ops)
		ctx := api.ROOT_CONTEXT
		span := api.Trace.WrapSpanContext(validSC())
		encRpc := func(m *RPCMetadata) string {
			if m == nil {
				return "undefined"
			}
			route := "none"
			if m.Route != "" {
				route = enc(m.Route)
			}
			spanText := "none"
			if m.Span != nil {
				sc := m.Span.SpanContext()
				spanText = encSC(&sc, "undefined")
			}
			return fmt.Sprintf("rpc(type=%s,route=%s,span=%s)", enc(string(m.Type)), route, spanText)
		}
		for _, op := range ops {
			var name string
			decode(op[0], &name)
			switch name {
			case "get":
				rec("get -> " + encRpc(GetRPCMetadata(ctx)))
			case "set":
				var arg struct{ Route *string }
				decode(op[1], &arg)
				meta := &RPCMetadata{Type: RPCType_HTTP, Span: span}
				if arg.Route != nil {
					meta.Route = *arg.Route
				}
				ctx = SetRPCMetadata(ctx, meta)
				rec("set")
			case "setRoute":
				var route string
				decode(op[1], &route)
				GetRPCMetadata(ctx).Route = route
				rec("route set")
			case "delete":
				ctx = DeleteRPCMetadata(ctx)
				rec("delete")
			default:
				panic(errBadOp)
			}
		}
	})

	run("suppress", func(raw json.RawMessage) {
		var ops [][]json.RawMessage
		decode(raw, &ops)
		ctx := api.ROOT_CONTEXT
		key := api.CreateContextKey("OpenTelemetry SDK Context Key SUPPRESS_TRACING")
		for _, op := range ops {
			var name string
			decode(op[0], &name)
			switch name {
			case "is":
				rec("is " + strconv.FormatBool(IsTracingSuppressed(ctx)))
			case "suppress":
				ctx = SuppressTracing(ctx)
				rec("suppress")
			case "unsuppress":
				ctx = UnsuppressTracing(ctx)
				rec("unsuppress")
			case "raw":
				v := build(op[1])
				ctx = ctx.SetValue(key, v)
				rec("raw " + enc(v))
			default:
				panic(errBadOp)
			}
		}
	})

	run("unref", func(raw json.RawMessage) {
		var timer any
		var s string
		if json.Unmarshal(raw, &s) == nil && (s == "withUnref" || s == "withoutUnref") {
			if s == "withUnref" {
				timer = recordingTimer{}
			} else {
				timer = &jsvalue.Object{}
			}
		} else {
			timer = build(raw)
		}
		defer func() {
			if r := recover(); r != nil {
				rec(thrown(r))
			}
		}()
		UnrefTimer(timer)
		rec("returned")
	})

	run("async", func(raw json.RawMessage) { runAsync(raw) })

	checkStatic(t, expected.Core["static"], c)

	total := 0
	for _, n := range c {
		total += n
	}
	t.Logf("%d cases, %d trace lines: %v", countCases(cases.Core), total, c)
}

func countCases(m map[string][]json.RawMessage) int {
	n := 0
	for _, v := range m {
		n += len(v)
	}
	return n
}

// runTraceStateOps is the oracle's traceStateOps.
func runTraceStateOps(raw json.RawMessage, make func(any) api.TraceState) {
	var spec struct {
		Raw json.RawMessage
		Ops [][]string
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		panic(err)
	}
	ts := make(build(spec.Raw))
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
		default:
			panic(errBadOp)
		}
	}
}

func sameOrNew(same bool) string {
	if same {
		return "same"
	}
	return "new"
}

// The propagators of the composite cases (the oracle's classes of the same names).
type ThrowingPropagator struct{}

func (ThrowingPropagator) Inject(api.Context, any, api.TextMapSetter) {
	panic(errors.New("inject failed"))
}
func (ThrowingPropagator) Extract(api.Context, any, api.TextMapGetter) api.Context {
	panic(errors.New("extract failed"))
}
func (ThrowingPropagator) Fields() []string { return []string{"x-throw"} }

type StringThrower struct{}

func (StringThrower) Inject(api.Context, any, api.TextMapSetter)              { panic("just a string") }
func (StringThrower) Extract(api.Context, any, api.TextMapGetter) api.Context { panic("just a string") }
func (StringThrower) Fields() []string                                        { return []string{"traceparent"} }

// noFieldsPropagator stands for an object literal without a fields function: Fields contributes nothing.
type noFieldsPropagator struct{}

func (noFieldsPropagator) Inject(_ api.Context, carrier any, setter api.TextMapSetter) {
	setter.Set(carrier, "x-nofields", "1")
}
func (noFieldsPropagator) Extract(ctx api.Context, _ any, _ api.TextMapGetter) api.Context {
	return ctx.SetValue(noFieldsKey, true)
}
func (noFieldsPropagator) Fields() []string { return nil }

type DupFields struct{}

func (DupFields) Inject(api.Context, any, api.TextMapSetter)                      {}
func (DupFields) Extract(ctx api.Context, _ any, _ api.TextMapGetter) api.Context { return ctx }
func (DupFields) Fields() []string                                                { return []string{"traceparent", "baggage", "traceparent", "x-dup"} }

type SetsTraceparent struct{}

func (SetsTraceparent) Inject(_ api.Context, carrier any, setter api.TextMapSetter) {
	setter.Set(carrier, "traceparent", "overridden")
}
func (SetsTraceparent) Extract(ctx api.Context, _ any, _ api.TextMapGetter) api.Context {
	return api.Trace.SetSpanContext(ctx, api.SpanContext{TraceId: strings.Repeat("1", 32), SpanId: strings.Repeat("2", 16)})
}
func (SetsTraceparent) Fields() []string { return []string{"traceparent"} }

func makePropagator(name string) api.TextMapPropagator {
	switch name {
	case "trace":
		return NewW3CTraceContextPropagator()
	case "baggage":
		return NewW3CBaggagePropagator()
	case "throwing":
		return ThrowingPropagator{}
	case "stringThrower":
		return StringThrower{}
	case "noFields":
		return noFieldsPropagator{}
	case "dupFields":
		return DupFields{}
	case "setsTraceparent":
		return SetsTraceparent{}
	}
	panic("unknown propagator " + name)
}

// identityKey is the identity of an object of the jsvalue model (empty arrays have none, as in the oracle).
func identityKey(v any) (any, bool) {
	type sliceKey struct {
		first *any
		n     int
	}
	switch x := v.(type) {
	case *jsvalue.Object:
		return x, x != nil
	case *parityspec.Instance:
		return x, true
	case *parityspec.Func:
		return x, true
	case *time.Time:
		return x, true
	case *regexp.Regexp:
		return x, true
	case []any:
		if len(x) == 0 {
			return nil, false
		}
		return sliceKey{&x[0], len(x)}, true
	}
	return nil, false
}

// encGraph is the oracle's encGraph.
func encGraph(root any, inputs map[any]string) string {
	seen := map[any]int{}
	var g func(v any) string
	g = func(v any) string {
		key, ok := identityKey(v)
		if !ok {
			if a, isArr := v.([]any); isArr && len(a) == 0 {
				return "[]"
			}
			return enc(v)
		}
		if inputs != nil {
			if id, ok := inputs[key]; ok {
				return "=" + id
			}
		}
		switch v.(type) {
		case *parityspec.Func:
			return "fn"
		case *time.Time, *regexp.Regexp:
			return enc(v)
		}
		if n, ok := seen[key]; ok {
			return "#" + strconv.Itoa(n)
		}
		seen[key] = len(seen)
		props := func(o *jsvalue.Object) string {
			parts := []string{}
			o.Range(func(k string, it any) bool { parts = append(parts, jsvalue.JSONQuote(k)+":"+g(it)); return true })
			return "{" + strings.Join(parts, ",") + "}"
		}
		switch x := v.(type) {
		case []any:
			parts := make([]string, len(x))
			for i, it := range x {
				parts[i] = g(it)
			}
			return "[" + strings.Join(parts, ",") + "]"
		case *jsvalue.Object:
			return props(x)
		case *parityspec.Instance:
			return x.Ctor + props(x.Props)
		}
		panic("encGraph: unexpected " + reflect.TypeOf(v).String())
	}
	return g(root)
}

// fakePerformance is the oracle's patched performance object.
type fakePerformance struct {
	origin float64
	nows   []float64
	i      int
}

func (p *fakePerformance) TimeOrigin() float64 { return p.origin }

func (p *fakePerformance) Now() float64 {
	if p.i >= len(p.nows) {
		panic(errors.New("now() called too often"))
	}
	p.i++
	return p.nows[p.i-1]
}

// hr converts an argument to an HrTime (the time cases only pass two-number arrays where one is expected).
func hr(v any) api.HrTime {
	h, ok := asHrTime(v)
	if !ok {
		panic("not an HrTime")
	}
	return h
}

func callTime(fn string, args []any) any {
	switch fn {
	case "getTimeOrigin":
		return GetTimeOrigin()
	case "hrTime":
		if len(args) > 0 {
			return HrTime(jsvalue.Float(args[0]))
		}
		return HrTime()
	case "millisToHrTime":
		return MillisToHrTime(jsvalue.Float(args[0]))
	case "timeInputToHrTime":
		return TimeInputToHrTime(args[0])
	case "hrTimeDuration":
		return HrTimeDuration(hr(args[0]), hr(args[1]))
	case "addHrTimes":
		return AddHrTimes(hr(args[0]), hr(args[1]))
	case "hrTimeToTimeStamp":
		return HrTimeToTimeStamp(hr(args[0]))
	case "hrTimeToNanoseconds":
		return HrTimeToNanoseconds(hr(args[0]))
	case "hrTimeToMicroseconds":
		return HrTimeToMicroseconds(hr(args[0]))
	case "hrTimeToMilliseconds":
		return HrTimeToMilliseconds(hr(args[0]))
	case "hrTimeToSeconds":
		return HrTimeToSeconds(hr(args[0]))
	case "isTimeInputHrTime":
		return IsTimeInputHrTime(args[0])
	case "isTimeInput":
		return IsTimeInput(args[0])
	}
	panic(errBadOp)
}

// matcher is the oracle's matcher: {re, flags} is a RegExp, anything else a value spec.
func matcher(raw json.RawMessage) any {
	var m struct {
		Re    *string
		Flags string
	}
	if json.Unmarshal(raw, &m) == nil && m.Re != nil {
		prefix := ""
		if m.Flags == "i" {
			prefix = "(?i)"
		}
		return regexp.MustCompile(prefix + *m.Re)
	}
	return build(raw)
}

func buildException(raw json.RawMessage) any {
	var te struct {
		Type string `json:"$"`
		Msg  string
	}
	if json.Unmarshal(raw, &te) == nil && te.Type == "timeoutError" {
		return NewTimeoutError(te.Msg)
	}
	return build(raw)
}

// scriptedClock returns its values in turn, recording each call.
type scriptedClock struct {
	name   string
	values []json.RawMessage
	i      int
}

func (c *scriptedClock) Now() float64 {
	rec(c.name + ".now")
	c.i++
	if c.i > len(c.values) {
		return math.NaN()
	}
	return jsvalue.Float(build(c.values[c.i-1]))
}

type recordingTimer struct{}

func (recordingTimer) Unref() { rec("unref called") }

// recHost is the oracle's VirtualClock on internal/jsloop's Virtual: timers and their clearing are recorded.
type recHost struct{ v *jsloop.Virtual }

type hostTimer struct {
	handle any
	queued bool
}

func (h *recHost) QueueMicrotask(f func()) { h.v.QueueMicrotask(f) }

func (h *recHost) SetTimeout(fn func(), ms int64) any {
	clamped := ms
	if !(ms >= 1 && ms <= 1<<31-1) {
		clamped = 1
	}
	rec(fmt.Sprintf("t=%d setTimeout(%d)", h.v.Now(), clamped))
	t := &hostTimer{queued: true}
	t.handle = h.v.SetTimeout(func() {
		t.queued = false
		rec(fmt.Sprintf("t=%d timer fires", h.v.Now()))
		fn()
	}, ms)
	return t
}

func (h *recHost) ClearTimeout(timer any) {
	t, ok := timer.(*hostTimer)
	state := "other"
	if ok {
		state = "done"
		if t.queued {
			state = "pending"
		}
	}
	rec(fmt.Sprintf("t=%d clearTimeout(%s)", h.v.Now(), state))
	if ok && t.queued {
		t.queued = false
		h.v.ClearTimeout(t.handle)
	}
}

// thenableFunc is an object with a then method.
type thenableFunc func(resolve, reject func(any))

func (f thenableFunc) Then(onFulfilled, onRejected func(any)) { f(onFulfilled, onRejected) }

type exporterFunc func(arg any, done func(ExportResult))

func (f exporterFunc) Export(arg any, done func(ExportResult)) { f(arg, done) }

func runAsync(raw json.RawMessage) {
	var spec struct {
		Kind     string
		Settle   json.RawMessage
		Timeout  float64
		Callback string
		Calls    []struct {
			At   int64
			Args []json.RawMessage
		}
		Exporter string
		Arg      json.RawMessage
		Manager  bool
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		panic(err)
	}
	v := jsloop.NewVirtual(0)
	h := &recHost{v: v}
	und := jsvalue.Undefined{}
	at := func(format string, a ...any) string { return fmt.Sprintf("t=%d ", v.Now()) + fmt.Sprintf(format, a...) }
	// ticker and listen are the oracle's: a chain of six microtasks, and two reactions on one promise.
	ticker := func(label string) {
		n := 0
		var step func(any) any
		step = func(any) any {
			n++
			rec(at("%s %d", label, n))
			if n < 6 {
				jspromise.Resolve(h, und).Then(step, nil)
			}
			return und
		}
		jspromise.Resolve(h, und).Then(step, nil)
	}
	listen := func(p *jspromise.Promise, label string) {
		p.Then(func(x any) any { rec(at("%s ok %s", label, enc(x))); return und }, func(e any) any { rec(at("%s err %s", label, enc(e))); return und })
		p.Then(func(x any) any { rec(at("%s second listener ok %s", label, enc(x))); return und },
			func(e any) any { rec(at("%s second listener err %s", label, enc(e))); return und })
	}
	switch spec.Kind {
	case "cwt":
		var inner any
		var s struct {
			At    *int64
			Ok    json.RawMessage
			Err   json.RawMessage
			Adopt bool
			Now   string
			Value json.RawMessage
			Plain json.RawMessage
		}
		var never string
		switch {
		case json.Unmarshal(spec.Settle, &never) == nil && never == "never":
			inner = jspromise.New(h, func(_, _ func(any)) {})
		case json.Unmarshal(spec.Settle, &s) == nil && s.Plain != nil:
			ticker("tick")
			inner = build(s.Plain)
		case s.Now != "":
			ticker("tick")
			if s.Now == "ok" {
				inner = jspromise.Resolve(h, build(s.Value))
			} else {
				inner = jspromise.Reject(h, build(s.Value))
			}
		default:
			inner = jspromise.New(h, func(resolve, reject func(any)) {
				v.After(*s.At, func() {
					rec(at("settling the promise"))
					ticker("tick")
					switch {
					case s.Ok != nil && s.Adopt:
						resolve(jspromise.Resolve(h, build(s.Ok)))
					case s.Ok != nil:
						resolve(build(s.Ok))
					default:
						reject(build(s.Err))
					}
				})
			})
		}
		p := CallWithTimeout(h, inner, spec.Timeout)
		listen(p, "result")
		if ip, ok := inner.(*jspromise.Promise); ok {
			listen(ip, "promise")
		}
	case "bof":
		cb := func(args ...any) any {
			parts := make([]string, len(args))
			for i, a := range args {
				parts[i] = enc(a)
			}
			rec(at("callback(%s)", strings.Join(parts, ", ")))
			switch spec.Callback {
			case "value":
				return "result"
			case "throw":
				panic(errors.New("callback failed"))
			case "throwNull":
				jspromise.Throw(nil)
			case "resolveLater":
				return jspromise.New(h, func(resolve, _ func(any)) {
					v.After(3, func() { rec(at("resolving")); resolve("later") })
				})
			case "rejectLater":
				return jspromise.New(h, func(_, reject func(any)) {
					v.After(3, func() { rec(at("rejecting")); reject(errors.New("later failure")) })
				})
			case "thenable":
				return thenableFunc(func(resolve, _ func(any)) { rec(at("then called")); resolve("from thenable") })
			case "undefined":
				return und
			}
			panic("bad callback")
		}
		f := NewBindOnceFuture(h, cb)
		rec("isCalled " + strconv.FormatBool(f.IsCalled()))
		listen(f.Promise(), "promise")
		for i, call := range spec.Calls {
			i, call := i, call
			v.After(call.At, func() {
				ticker("tick" + strconv.Itoa(i))
				args := make([]any, len(call.Args))
				for i, a := range call.Args {
					args[i] = build(a)
				}
				p := f.Call(args...)
				which := "another promise"
				if p == f.Promise() {
					which = "the promise"
				}
				rec(at("call -> %s, isCalled %t", which, f.IsCalled()))
			})
		}
	case "export":
		if spec.Manager {
			rec("setGlobalContextManager -> " + strconv.FormatBool(api.Ctx.SetGlobalContextManager(newStackManager())))
		}
		exporter := exporterFunc(func(arg any, done func(ExportResult)) {
			rec(at("export(%s) suppressed=%t", enc(arg), IsTracingSuppressed(api.Ctx.Active())))
			switch spec.Exporter {
			case "sync":
				done(ExportResult{Code: ExportResultCode_SUCCESS})
			case "later":
				v.After(4, func() {
					rec(at("reporting"))
					done(ExportResult{Code: ExportResultCode_FAILED, Error: errors.New("export failed")})
				})
			case "twice":
				done(ExportResult{Code: ExportResultCode_SUCCESS})
				done(ExportResult{Code: ExportResultCode_FAILED})
			case "throws":
				panic(errors.New("exporter threw"))
			case "never":
			}
		})
		ticker("tick")
		p := Internal.Export(h, exporter, build(spec.Arg))
		p.Then(func(r any) any {
			res := r.(ExportResult)
			errText := "undefined"
			if res.Error != nil {
				errText = enc(res.Error)
			}
			rec(at("result code=%s error=%s", parityspec.EncNumber(float64(res.Code)), errText))
			return und
		}, func(e any) any { rec(at("rejected %s", enc(e))); return und })
		p.Then(func(any) any { rec(at("second listener ok")); return und }, func(any) any { rec(at("second listener err")); return und })
		rec("active suppressed=" + strconv.FormatBool(IsTracingSuppressed(api.Ctx.Active())))
	default:
		panic(errBadOp)
	}
	v.Run()
	rec(fmt.Sprintf("end t=%d", v.Now()))
}

// enumEntries is Object.entries of a TypeScript numeric enum: the reverse mappings under their array-index keys,
// ascending, then the members in order.
func enumEntries(names []string, values []float64) [][2]any {
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
	return out
}

// jsExports maps every export of require('@opentelemetry/core') to its Go translation.
var jsExports = map[string]string{
	"W3CBaggagePropagator": "W3CBaggagePropagator", "AnchoredClock": "AnchoredClock", "isAttributeValue": "IsAttributeValue",
	"sanitizeAttributes": "SanitizeAttributes", "globalErrorHandler": "GlobalErrorHandler", "setGlobalErrorHandler": "SetGlobalErrorHandler",
	"loggingErrorHandler": "LoggingErrorHandler", "addHrTimes": "AddHrTimes", "getTimeOrigin": "GetTimeOrigin", "hrTime": "HrTime",
	"hrTimeDuration": "HrTimeDuration", "hrTimeToMicroseconds": "HrTimeToMicroseconds", "hrTimeToMilliseconds": "HrTimeToMilliseconds",
	"hrTimeToNanoseconds": "HrTimeToNanoseconds", "hrTimeToSeconds": "HrTimeToSeconds", "hrTimeToTimeStamp": "HrTimeToTimeStamp",
	"isTimeInput": "IsTimeInput", "isTimeInputHrTime": "IsTimeInputHrTime", "millisToHrTime": "MillisToHrTime",
	"timeInputToHrTime": "TimeInputToHrTime", "unrefTimer": "UnrefTimer", "ExportResultCode": "ExportResultCode",
	"parseKeyPairsIntoRecord": "ParseKeyPairsIntoRecord", "SDK_INFO": "SDK_INFO",
	"_globalThis":      "N/A: re-export of globalThis (deprecated); Go has no global object",
	"getStringFromEnv": "GetStringFromEnv", "getBooleanFromEnv": "GetBooleanFromEnv", "getNumberFromEnv": "GetNumberFromEnv",
	"getStringListFromEnv": "GetStringListFromEnv", "otperformance": "Otperformance", "CompositePropagator": "CompositePropagator",
	"TRACE_PARENT_HEADER": "TRACE_PARENT_HEADER", "TRACE_STATE_HEADER": "TRACE_STATE_HEADER",
	"W3CTraceContextPropagator": "W3CTraceContextPropagator", "parseTraceParent": "ParseTraceParent", "RPCType": "RPCType",
	"deleteRPCMetadata": "DeleteRPCMetadata", "getRPCMetadata": "GetRPCMetadata", "setRPCMetadata": "SetRPCMetadata",
	"isTracingSuppressed": "IsTracingSuppressed", "suppressTracing": "SuppressTracing", "unsuppressTracing": "UnsuppressTracing",
	"TraceState": "TraceState", "merge": "Merge", "TimeoutError": "TimeoutError", "callWithTimeout": "CallWithTimeout",
	"isUrlIgnored": "IsUrlIgnored", "urlMatches": "UrlMatches", "BindOnceFuture": "BindOnceFuture",
	"diagLogLevelFromString": "DiagLogLevelFromString", "internal": "Internal",
}

func checkStatic(t *testing.T, raw json.RawMessage, c checks) {
	var s struct {
		SdkInfo, ExportResultCode, RpcType string
		Headers, TraceFields               []string
		BaggageFields                      []string
		TimeoutError                       []any
		Exports, Internal                  []string
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatal(err)
	}
	e := NewTimeoutError("msg")
	got := map[string][2]string{
		"sdkInfo":          {s.SdkInfo, enc(SDK_INFO)},
		"exportResultCode": {s.ExportResultCode, enc(enumEntries([]string{"SUCCESS", "FAILED"}, []float64{float64(ExportResultCode_SUCCESS), float64(ExportResultCode_FAILED)}))},
		"rpcType":          {s.RpcType, enc([][2]any{{"HTTP", string(RPCType_HTTP)}})},
		"headers":          {enc(s.Headers), enc([]string{TRACE_PARENT_HEADER, TRACE_STATE_HEADER})},
		"traceFields":      {enc(s.TraceFields), enc(NewW3CTraceContextPropagator().Fields())},
		"baggageFields":    {enc(s.BaggageFields), enc(NewW3CBaggagePropagator().Fields())},
		// TimeoutError's name is Error's, it is an error, and it has no enumerable properties.
		"timeoutError": {enc(s.TimeoutError), enc([]any{"Error", e.Message, true, true, "Error: " + e.Error(), []any{}})},
		"internal":     {enc(s.Internal), enc([]string{"_export"})},
	}
	for k, v := range got {
		c["static"]++
		if v[0] != v[1] {
			t.Errorf("static %s:\n--- js %s\n+++ go %s", k, v[0], v[1])
		}
	}
	for _, name := range s.Exports {
		c["exports"]++
		if _, ok := jsExports[name]; !ok {
			t.Errorf("export %s is not accounted for", name)
		}
	}
	if len(s.Exports) != len(jsExports) {
		t.Errorf("the module has %d exports, the table %d", len(s.Exports), len(jsExports))
	}
}
