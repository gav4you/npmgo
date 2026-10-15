// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
//
// Translated to Go by the npmgo contributors from @opentelemetry/core 2.11.0
// (build/src/trace/W3CTraceContextPropagator.js).

package core

import (
	"strings"
	"unicode/utf8"

	"github.com/gav4you/npmgo/internal/jsnum"
	"github.com/gav4you/npmgo/internal/jsvalue"
	"github.com/gav4you/npmgo/pkgs/opentelemetry/api"
)

// TRACE_PARENT_HEADER is the name of the traceparent header.
const TRACE_PARENT_HEADER = "traceparent"

// TRACE_STATE_HEADER is the name of the tracestate header.
const TRACE_STATE_HEADER = "tracestate"

const traceParentVersion = "00"

func isLowerHex(c byte) bool { return '0' <= c && c <= '9' || 'a' <= c && c <= 'f' }

// hexRun reports whether s[at:at+n] exists and is n lowercase hexadecimal digits.
func hexRun(s string, at, n int) bool {
	if at+n > len(s) {
		return false
	}
	for i := at; i < at+n; i++ {
		if !isLowerHex(s[i]) {
			return false
		}
	}
	return true
}

// isLineTerminator reports whether r is one of the characters JavaScript's . does not match.
func isLineTerminator(r rune) bool { return r == '\n' || r == '\r' || r == 0x2028 || r == 0x2029 }

// ParseTraceParent parses a traceparent header ({version}-{trace-id}-{parent-id}-{trace-flags}) into a span context
// with TraceId, SpanId and TraceFlags, or returns nil (JavaScript's null). It matches what
//
//	/^\s?((?!ff)[\da-f]{2})-((?![0]{32})[\da-f]{32})-((?![0]{16})[\da-f]{16})-([\da-f]{2})(-.*)?\s?$/
//
// matches (Go's regexp has no lookahead, so the match is written out): optional JavaScript whitespace, a version
// other than ff, a trace id and a parent id that are not all zeros, the flags, then either a "-" and further
// characters (which version 00 does not allow) or nothing, either followed by at most one whitespace character.
func ParseTraceParent(traceParent string) *api.SpanContext {
	s := traceParent
	p := 0
	if r, size := utf8.DecodeRuneInString(s); size > 0 && jsnum.IsWhiteSpace(r) && r != utf8.RuneError {
		p = size
	}
	// version "-" trace-id "-" parent-id "-" flags: 2+1+32+1+16+1+2 = 55 ASCII characters.
	if !hexRun(s, p, 2) || s[p:p+2] == "ff" || !hexRun(s, p+3, 32) || !hexRun(s, p+36, 16) || !hexRun(s, p+53, 2) ||
		s[p+2] != '-' || s[p+35] != '-' || s[p+52] != '-' {
		return nil
	}
	version, traceId, spanId, flags := s[p:p+2], s[p+3:p+35], s[p+36:p+52], s[p+53:p+55]
	if traceId == strings.Repeat("0", 32) || spanId == strings.Repeat("0", 16) {
		return nil
	}
	rest := s[p+55:]
	hasMore := false // whether the (-.*) group takes part in the match
	if strings.HasPrefix(rest, "-") {
		// (-.*) then \s?$: every character after the "-" but the last must match ., and the last may be any
		// whitespace (a line terminator is whitespace).
		runes := []rune(rest[1:])
		for i, r := range runes {
			if isLineTerminator(r) && i != len(runes)-1 {
				return nil
			}
		}
		hasMore = true
	} else if rest != "" {
		// \s?$: a single whitespace character.
		r, size := utf8.DecodeRuneInString(rest)
		if size != len(rest) || !jsnum.IsWhiteSpace(r) || r == utf8.RuneError {
			return nil
		}
	}
	// Version 00 has no further fields (https://www.w3.org/TR/trace-context/#versioning-of-traceparent).
	if version == "00" && hasMore {
		return nil
	}
	return &api.SpanContext{TraceId: traceId, SpanId: spanId, TraceFlags: api.TraceFlags(jsnum.ParseIntRadix(flags, 16))}
}

// W3CTraceContextPropagator propagates span contexts in the traceparent and tracestate headers
// (https://www.w3.org/TR/trace-context/).
type W3CTraceContextPropagator struct{}

// NewW3CTraceContextPropagator is new W3CTraceContextPropagator().
func NewW3CTraceContextPropagator() *W3CTraceContextPropagator { return &W3CTraceContextPropagator{} }

// Inject writes the context's span context, when it has a valid one and tracing is not suppressed: traceparent is
// 00-{traceId}-{spanId}-0{flags in hexadecimal} (so flags of 16 or more give three digits, as in JavaScript) and
// tracestate the serialized trace state, when there is one.
func (p *W3CTraceContextPropagator) Inject(context api.Context, carrier any, setter api.TextMapSetter) {
	spanContext := api.Trace.GetSpanContext(context)
	if spanContext == nil || IsTracingSuppressed(context) || !api.IsSpanContextValid(*spanContext) {
		return
	}
	flags := float64(spanContext.TraceFlags)
	if !jsvalue.Truthy(flags) {
		flags = float64(api.TraceFlags_NONE)
	}
	traceParent := traceParentVersion + "-" + spanContext.TraceId + "-" + spanContext.SpanId + "-0" + jsnum.StringRadix(flags, 16)
	setter.Set(carrier, TRACE_PARENT_HEADER, traceParent)
	if spanContext.TraceState != nil && !jsvalue.IsNull(spanContext.TraceState) {
		setter.Set(carrier, TRACE_STATE_HEADER, spanContext.TraceState.Serialize())
	}
}

// Extract returns context with a remote span context parsed from the carrier's traceparent header (the first, when
// the getter returns a list) and its tracestate header (several joined with commas; a header that is neither a
// string nor a list gives an empty TraceState), or context itself when there is no valid traceparent.
func (p *W3CTraceContextPropagator) Extract(context api.Context, carrier any, getter api.TextMapGetter) api.Context {
	traceParentHeader := getter.Get(carrier, TRACE_PARENT_HEADER)
	if !jsvalue.Truthy(traceParentHeader) {
		return context
	}
	traceParent := traceParentHeader
	if jsvalue.IsArray(traceParentHeader) {
		traceParent = jsvalue.Undefined{}
		if items := jsvalue.Items(traceParentHeader); len(items) > 0 {
			traceParent = items[0]
		}
	}
	tp, ok := traceParent.(string)
	if !ok {
		return context
	}
	spanContext := ParseTraceParent(tp)
	if spanContext == nil {
		return context
	}
	spanContext.IsRemote = true
	traceStateHeader := getter.Get(carrier, TRACE_STATE_HEADER)
	if jsvalue.Truthy(traceStateHeader) {
		var state any = traceStateHeader
		if jsvalue.IsArray(traceStateHeader) {
			state = jsvalue.Join(jsvalue.Items(traceStateHeader), ",")
		}
		raw, _ := state.(string)
		spanContext.TraceState = NewTraceState(raw)
	}
	return api.Trace.SetSpanContext(context, *spanContext)
}

// Fields returns ["traceparent", "tracestate"].
func (p *W3CTraceContextPropagator) Fields() []string {
	return []string{TRACE_PARENT_HEADER, TRACE_STATE_HEADER}
}
