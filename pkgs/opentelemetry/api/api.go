// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
//
// Translated to Go by the npmgo contributors from @opentelemetry/api 1.9.0 (build/src/index.js, version.js and the
// *-api.js entry modules).

// Package api is a translation of @opentelemetry/api 1.9.0, the OpenTelemetry API for JavaScript: contexts and their
// keys, the diagnostic logger, baggage, text-map propagation, the trace API (span contexts, trace state, the no-op and
// proxy tracers) and the no-op metrics API, with the process-wide registry the JavaScript API keeps on globalThis.
//
// The Go API keeps the JavaScript shape. The singletons context, diag, metrics, propagation and trace are Ctx, Diag,
// Metrics, Propagation and Trace (Context is the context interface). Numbers are float64 (enum types such as
// TraceFlags and DiagLogLevel are float64 types, their members TraceFlags_SAMPLED and so on), undefined is
// jsvalue.Undefined{} or a nil interface, and values the API does not inspect (attributes, carriers) are any.
// Where JavaScript throws, Go panics.
package api

// VERSION is the API version (version.js), checked against the version of the registry.
const VERSION = "1.9.0"

// The API singletons (context-api.js, diag-api.js, metrics-api.js, propagation-api.js, trace-api.js).
var (
	// Ctx is the context API (JavaScript's `context` export).
	Ctx = &ContextAPI{}
	// Diag is the diagnostic logging API (`diag`).
	Diag = &DiagAPI{}
	// Metrics is the metrics API (`metrics`).
	Metrics = &MetricsAPI{}
	// Propagation is the propagation API (`propagation`).
	Propagation = &PropagationAPI{}
	// Trace is the tracing API (`trace`).
	Trace = newTraceAPI()
)
