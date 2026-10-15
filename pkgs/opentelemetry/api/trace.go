// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
//
// Translated to Go by the npmgo contributors from @opentelemetry/api 1.9.0 (build/src/trace/*.js and api/trace.js:
// trace_flags, span_kind, status, SamplingResult, invalid-span-constants, spancontext-utils, NonRecordingSpan,
// NoopTracer, NoopTracerProvider, ProxyTracer, ProxyTracerProvider and context-utils).

package api

import (
	"regexp"
	"sync"

	"github.com/gav4you/npmgo/internal/jsvalue"
)

// TraceFlags are the trace flags of a span context (the TraceFlags enum; any number is accepted, as in JavaScript).
type TraceFlags float64

// The TraceFlags members.
const (
	TraceFlags_NONE    TraceFlags = 0
	TraceFlags_SAMPLED TraceFlags = 1
)

// SpanKind is the SpanKind enum.
type SpanKind float64

// The SpanKind members.
const (
	SpanKind_INTERNAL SpanKind = 0
	SpanKind_SERVER   SpanKind = 1
	SpanKind_CLIENT   SpanKind = 2
	SpanKind_PRODUCER SpanKind = 3
	SpanKind_CONSUMER SpanKind = 4
)

// SpanStatusCode is the SpanStatusCode enum.
type SpanStatusCode float64

// The SpanStatusCode members.
const (
	SpanStatusCode_UNSET SpanStatusCode = 0
	SpanStatusCode_OK    SpanStatusCode = 1
	SpanStatusCode_ERROR SpanStatusCode = 2
)

// SamplingDecision is the SamplingDecision enum (deprecated: sdk-trace-base declares its own).
type SamplingDecision float64

// The SamplingDecision members.
const (
	SamplingDecision_NOT_RECORD         SamplingDecision = 0
	SamplingDecision_RECORD             SamplingDecision = 1
	SamplingDecision_RECORD_AND_SAMPLED SamplingDecision = 2
)

// SpanContext identifies a span. IsRemote is false where JavaScript leaves isRemote undefined, and a nil TraceState
// stands for an undefined traceState.
type SpanContext struct {
	TraceId    string
	SpanId     string
	IsRemote   bool
	TraceFlags TraceFlags
	TraceState TraceState
}

// SpanStatus is a span's status.
type SpanStatus struct {
	Code    SpanStatusCode
	Message string
}

// Link is a link to another span.
type Link struct {
	Context                SpanContext
	Attributes             any
	DroppedAttributesCount float64
}

// SpanOptions are the options of Tracer.StartSpan. Attributes is an attributes object and StartTime a time input
// (nil: undefined).
type SpanOptions struct {
	Kind       SpanKind
	Attributes any
	Links      []Link
	StartTime  any
	Root       bool
}

// TracerOptions are the options of TracerProvider.GetTracer.
type TracerOptions struct {
	SchemaUrl string
}

// Span is a span. The optional trailing arguments of AddEvent, End and RecordException are variadic.
type Span interface {
	SpanContext() SpanContext
	SetAttribute(key string, value any) Span
	SetAttributes(attributes any) Span
	AddEvent(name string, attributesOrStartTime ...any) Span
	AddLink(link Link) Span
	AddLinks(links []Link) Span
	SetStatus(status SpanStatus) Span
	UpdateName(name string) Span
	End(endTime ...any)
	IsRecording() bool
	RecordException(exception any, time ...any)
}

// Tracer starts spans.
type Tracer interface {
	// StartSpan starts a span; nil options are undefined, a nil context is the active one.
	StartSpan(name string, options *SpanOptions, context Context) Span
	// StartActiveSpan is startActiveSpan(name, fn), (name, options, fn) or (name, options, context, fn), chosen by
	// the number of arguments as JavaScript chooses by arguments.length: it starts a span, runs fn(span) with a
	// context holding the span active and returns fn's result. fn is a func(Span) any; options a *SpanOptions and
	// context a Context, either of them nil (or jsvalue.Undefined{}) for undefined.
	StartActiveSpan(name string, args ...any) any
}

// TracerProvider provides tracers. An empty version stands for undefined.
type TracerProvider interface {
	GetTracer(name string, version string, options *TracerOptions) Tracer
}

// INVALID_SPANID is the invalid span id.
const INVALID_SPANID = "0000000000000000"

// INVALID_TRACEID is the invalid trace id.
const INVALID_TRACEID = "00000000000000000000000000000000"

// INVALID_SPAN_CONTEXT is the span context of a span that has none.
var INVALID_SPAN_CONTEXT = SpanContext{TraceId: INVALID_TRACEID, SpanId: INVALID_SPANID, TraceFlags: TraceFlags_NONE}

var (
	validTraceIdRe = regexp.MustCompile(`^(?i)([0-9a-f]{32})$`)
	validSpanIdRe  = regexp.MustCompile(`^(?i)[0-9a-f]{16}$`)
)

// IsValidTraceId reports whether traceId is 32 hexadecimal digits (either case) and not all zero.
func IsValidTraceId(traceId string) bool {
	return validTraceIdRe.MatchString(traceId) && traceId != INVALID_TRACEID
}

// IsValidSpanId reports whether spanId is 16 hexadecimal digits (either case) and not all zero.
func IsValidSpanId(spanId string) bool {
	return validSpanIdRe.MatchString(spanId) && spanId != INVALID_SPANID
}

// IsSpanContextValid reports whether the span context has a valid trace id and span id.
func IsSpanContextValid(spanContext SpanContext) bool {
	return IsValidTraceId(spanContext.TraceId) && IsValidSpanId(spanContext.SpanId)
}

// wrapSpanContext wraps a span context in a non-recording span.
func wrapSpanContext(spanContext SpanContext) Span { return newNonRecordingSpan(spanContext) }

// nonRecordingSpan is NonRecordingSpan: a span that records nothing; its methods return the span itself.
type nonRecordingSpan struct {
	spanContext SpanContext
}

func newNonRecordingSpan(spanContext SpanContext) *nonRecordingSpan {
	return &nonRecordingSpan{spanContext: spanContext}
}

func (s *nonRecordingSpan) SpanContext() SpanContext      { return s.spanContext }
func (s *nonRecordingSpan) SetAttribute(string, any) Span { return s }
func (s *nonRecordingSpan) SetAttributes(any) Span        { return s }
func (s *nonRecordingSpan) AddEvent(string, ...any) Span  { return s }
func (s *nonRecordingSpan) AddLink(Link) Span             { return s }
func (s *nonRecordingSpan) AddLinks([]Link) Span          { return s }
func (s *nonRecordingSpan) SetStatus(SpanStatus) Span     { return s }
func (s *nonRecordingSpan) UpdateName(string) Span        { return s }
func (s *nonRecordingSpan) End(...any)                    {}
func (s *nonRecordingSpan) IsRecording() bool             { return false }
func (s *nonRecordingSpan) RecordException(any, ...any)   {}

// spanKey is the context key of the active span.
var spanKey = CreateContextKey("OpenTelemetry Context Key SPAN")

func getSpan(context Context) Span {
	s, _ := context.GetValue(spanKey).(Span)
	if jsvalue.IsNull(s) {
		return nil
	}
	return s
}

func setSpan(context Context, span Span) Context { return context.SetValue(spanKey, span) }

func deleteSpan(context Context) Context { return context.DeleteValue(spanKey) }

func setSpanContext(context Context, spanContext SpanContext) Context {
	return setSpan(context, newNonRecordingSpan(spanContext))
}

func getSpanContext(context Context) *SpanContext {
	s := getSpan(context)
	if s == nil {
		return nil
	}
	sc := s.SpanContext()
	return &sc
}

// noopTracer is NoopTracer: its spans are non-recording spans carrying the parent's span context, when it is valid.
type noopTracer struct{}

func (t noopTracer) StartSpan(name string, options *SpanOptions, context Context) Span {
	if context == nil {
		context = Ctx.Active()
	}
	if options != nil && options.Root {
		return newNonRecordingSpan(INVALID_SPAN_CONTEXT)
	}
	if parent := getSpanContext(context); parent != nil && IsSpanContextValid(*parent) {
		return newNonRecordingSpan(*parent)
	}
	return newNonRecordingSpan(INVALID_SPAN_CONTEXT)
}

func (t noopTracer) StartActiveSpan(name string, args ...any) any {
	return startActiveSpan(t, name, args)
}

// startActiveSpan is NoopTracer.startActiveSpan's argument handling and body.
func startActiveSpan(t Tracer, name string, args []any) any {
	var opts, ctx, fn any = jsvalue.Undefined{}, jsvalue.Undefined{}, jsvalue.Undefined{}
	switch len(args) {
	case 0:
		return jsvalue.Undefined{}
	case 1:
		fn = args[0]
	case 2:
		opts, fn = args[0], args[1]
	default:
		opts, ctx, fn = args[0], args[1], args[2]
	}
	parentContext, _ := ctx.(Context)
	if jsvalue.IsNull(parentContext) {
		parentContext = Ctx.Active()
	}
	options, _ := opts.(*SpanOptions)
	span := t.StartSpan(name, options, parentContext)
	contextWithSpanSet := setSpan(parentContext, span)
	f, ok := fn.(func(Span) any)
	if !ok {
		// JavaScript throws when the context manager calls fn; these are the no-op context manager's messages
		// (fn.call(thisArg, ...args)).
		switch {
		case fn == nil:
			panic(&jsvalue.TypeError{Message: "Cannot read properties of null (reading 'call')"})
		case jsvalue.IsUndefined(fn):
			panic(&jsvalue.TypeError{Message: "Cannot read properties of undefined (reading 'call')"})
		}
		panic(&jsvalue.TypeError{Message: "fn.call is not a function"})
	}
	return Ctx.With(contextWithSpanSet, func(a ...any) any { return f(a[0].(Span)) }, span)
}

var noopTracerInstance Tracer = noopTracer{}

// noopTracerProvider is NoopTracerProvider: every tracer is a no-op tracer.
type noopTracerProvider struct{}

func (noopTracerProvider) GetTracer(string, string, *TracerOptions) Tracer { return noopTracer{} }

var noopTracerProviderInstance TracerProvider = noopTracerProvider{}

// DelegateTracerGetter is what a ProxyTracer asks for the tracer it forwards to (ProxyTracerProvider is one).
type DelegateTracerGetter interface {
	GetDelegateTracer(name string, version string, options *TracerOptions) Tracer
}

// ProxyTracer forwards to the tracer of its provider's delegate once there is one, and to a no-op tracer until then.
type ProxyTracer struct {
	provider DelegateTracerGetter
	Name     string
	Version  string
	Options  *TracerOptions

	mu       sync.Mutex
	delegate Tracer
}

// NewProxyTracer is new ProxyTracer(provider, name, version, options).
func NewProxyTracer(provider DelegateTracerGetter, name, version string, options *TracerOptions) *ProxyTracer {
	return &ProxyTracer{provider: provider, Name: name, Version: version, Options: options}
}

// StartSpan starts a span with the delegate tracer.
func (t *ProxyTracer) StartSpan(name string, options *SpanOptions, context Context) Span {
	return t.getTracer().StartSpan(name, options, context)
}

// StartActiveSpan calls the delegate tracer's StartActiveSpan with the same arguments.
func (t *ProxyTracer) StartActiveSpan(name string, args ...any) any {
	return t.getTracer().StartActiveSpan(name, args...)
}

// getTracer returns the delegate tracer, asking the provider until it has one (which is then kept), or a no-op
// tracer.
func (t *ProxyTracer) getTracer() Tracer {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.delegate != nil {
		return t.delegate
	}
	tracer := t.provider.GetDelegateTracer(t.Name, t.Version, t.Options)
	if tracer == nil || jsvalue.IsNull(tracer) {
		return noopTracerInstance
	}
	t.delegate = tracer
	return tracer
}

// ProxyTracerProvider provides ProxyTracers until a delegate provider is set, and the delegate's tracers after.
// Tracers handed out before the delegate was set forward to it from then on.
type ProxyTracerProvider struct {
	mu       sync.RWMutex
	delegate TracerProvider
}

// NewProxyTracerProvider is new ProxyTracerProvider().
func NewProxyTracerProvider() *ProxyTracerProvider { return &ProxyTracerProvider{} }

// GetTracer returns the delegate's tracer, or a ProxyTracer when there is no delegate (or it returned none).
func (p *ProxyTracerProvider) GetTracer(name, version string, options *TracerOptions) Tracer {
	if t := p.GetDelegateTracer(name, version, options); t != nil && !jsvalue.IsNull(t) {
		return t
	}
	return NewProxyTracer(p, name, version, options)
}

// GetDelegate returns the delegate, or a no-op provider.
func (p *ProxyTracerProvider) GetDelegate() TracerProvider {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.delegate == nil || jsvalue.IsNull(p.delegate) {
		return noopTracerProviderInstance
	}
	return p.delegate
}

// SetDelegate sets the delegate provider.
func (p *ProxyTracerProvider) SetDelegate(delegate TracerProvider) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.delegate = delegate
}

// GetDelegateTracer returns the delegate's tracer, or nil without a delegate.
func (p *ProxyTracerProvider) GetDelegateTracer(name, version string, options *TracerOptions) Tracer {
	p.mu.RLock()
	d := p.delegate
	p.mu.RUnlock()
	if d == nil || jsvalue.IsNull(d) {
		return nil
	}
	return d.GetTracer(name, version, options)
}

const traceAPIName = "trace"

// TraceAPI is the tracing API (JavaScript's TraceAPI class; its instance is Trace).
type TraceAPI struct {
	mu                  sync.Mutex
	proxyTracerProvider *ProxyTracerProvider
}

func newTraceAPI() *TraceAPI { return &TraceAPI{proxyTracerProvider: NewProxyTracerProvider()} }

func (t *TraceAPI) proxy() *ProxyTracerProvider {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.proxyTracerProvider
}

// SetGlobalTracerProvider registers the API's proxy provider as the global trace provider and, when that succeeds,
// makes provider its delegate. It reports whether it succeeded.
func (t *TraceAPI) SetGlobalTracerProvider(provider TracerProvider) bool {
	proxy := t.proxy()
	success := registerGlobal(traceAPIName, proxy, Diag, false)
	if success {
		proxy.SetDelegate(provider)
	}
	return success
}

// GetTracerProvider returns the global trace provider, or the API's proxy provider.
func (t *TraceAPI) GetTracerProvider() TracerProvider {
	if p, ok := getGlobal(traceAPIName).(TracerProvider); ok && !jsvalue.IsNull(p) {
		return p
	}
	return t.proxy()
}

// GetTracer returns a tracer of the global trace provider (an empty version is undefined).
func (t *TraceAPI) GetTracer(name, version string) Tracer {
	return t.GetTracerProvider().GetTracer(name, version, nil)
}

// Disable removes the global trace provider and replaces the API's proxy provider with a new one.
func (t *TraceAPI) Disable() {
	unregisterGlobal(traceAPIName, Diag)
	t.mu.Lock()
	t.proxyTracerProvider = NewProxyTracerProvider()
	t.mu.Unlock()
}

// WrapSpanContext wraps a span context in a non-recording span.
func (t *TraceAPI) WrapSpanContext(spanContext SpanContext) Span { return wrapSpanContext(spanContext) }

// IsSpanContextValid reports whether the span context is valid.
func (t *TraceAPI) IsSpanContextValid(spanContext SpanContext) bool {
	return IsSpanContextValid(spanContext)
}

// DeleteSpan returns a context without a span.
func (t *TraceAPI) DeleteSpan(context Context) Context { return deleteSpan(context) }

// GetSpan returns the span of the context, or nil.
func (t *TraceAPI) GetSpan(context Context) Span { return getSpan(context) }

// GetActiveSpan returns the span of the active context, or nil.
func (t *TraceAPI) GetActiveSpan() Span { return getSpan(Ctx.Active()) }

// GetSpanContext returns the span context of the context's span, or nil.
func (t *TraceAPI) GetSpanContext(context Context) *SpanContext { return getSpanContext(context) }

// SetSpan returns a context holding the span.
func (t *TraceAPI) SetSpan(context Context, span Span) Context { return setSpan(context, span) }

// SetSpanContext returns a context holding a non-recording span with the span context.
func (t *TraceAPI) SetSpanContext(context Context, spanContext SpanContext) Context {
	return setSpanContext(context, spanContext)
}
