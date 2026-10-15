// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
//
// Translated to Go by the npmgo contributors from @opentelemetry/core 2.11.0 (build/src/index.js, version.js,
// semconv.js, ExportResult.js, platform/node/sdk-info.js, trace/rpc-metadata.js and trace/suppress-tracing.js).

// Package core is a translation of @opentelemetry/core 2.11.0 as Node loads it (the node platform): the W3C trace
// context and baggage propagators, the composite propagator, TraceState, the time utilities, attribute
// sanitisation, the environment readers, the global error handler, the anchored clock, merge, the promise utilities
// (callWithTimeout, BindOnceFuture, internal._export) and the small helpers.
//
// The Go API keeps the JavaScript shape (see package api): exported names are the JavaScript ones (capitalised),
// SCREAMING_CASE constants keep their names, enum members are Enum_MEMBER, numbers are float64, plain objects
// (attributes, records, SDK_INFO) are *jsvalue.Object, undefined is jsvalue.Undefined{} or a nil interface, and where
// JavaScript throws, Go panics. Functions that read process.env, performance or the timers read ProcessEnv,
// Otperformance and a Host.
package core

import (
	"github.com/gav4you/npmgo/internal/jsvalue"
	"github.com/gav4you/npmgo/pkgs/opentelemetry/api"
	semconv "github.com/gav4you/npmgo/pkgs/opentelemetry/semantic-conventions"
)

// version is the package version (version.js).
const version = "2.11.0"

// attrProcessRuntimeName is semconv.js's copy of the unstable ATTR_PROCESS_RUNTIME_NAME.
const attrProcessRuntimeName = "process.runtime.name"

// SDK_INFO describes the SDK in use: telemetry.sdk.name, process.runtime.name, telemetry.sdk.language and
// telemetry.sdk.version, as the node platform reports them ("opentelemetry", "node", "nodejs", "2.11.0"), in that
// order. It is a shared, mutable object, as in JavaScript.
var SDK_INFO = jsvalue.NewObject(
	semconv.ATTR_TELEMETRY_SDK_NAME, "opentelemetry",
	attrProcessRuntimeName, "node",
	semconv.ATTR_TELEMETRY_SDK_LANGUAGE, semconv.TELEMETRY_SDK_LANGUAGE_VALUE_NODEJS,
	semconv.ATTR_TELEMETRY_SDK_VERSION, version,
)

// ExportResultCode is the ExportResultCode enum.
type ExportResultCode float64

// The ExportResultCode members.
const (
	ExportResultCode_SUCCESS ExportResultCode = 0
	ExportResultCode_FAILED  ExportResultCode = 1
)

// ExportResult is the result an exporter reports (a nil Error is undefined).
type ExportResult struct {
	Code  ExportResultCode
	Error error
}

// RPCType is the RPCType enum.
type RPCType string

// RPCType_HTTP is RPCType.HTTP.
const RPCType_HTTP RPCType = "http"

// RPCMetadata is the RPC metadata an instrumentation keeps in the context (HTTPMetadata: type, route and span). It is
// stored by pointer, so code that finds it can update it (an instrumentation sets Route), as in JavaScript.
type RPCMetadata struct {
	Type  RPCType
	Route string
	Span  api.Span
}

var rpcMetadataKey = api.CreateContextKey("OpenTelemetry SDK Context Key RPC_METADATA")

// SetRPCMetadata returns a context holding meta.
func SetRPCMetadata(context api.Context, meta *RPCMetadata) api.Context {
	return context.SetValue(rpcMetadataKey, meta)
}

// DeleteRPCMetadata returns a context without RPC metadata.
func DeleteRPCMetadata(context api.Context) api.Context { return context.DeleteValue(rpcMetadataKey) }

// GetRPCMetadata returns the context's RPC metadata, or nil.
func GetRPCMetadata(context api.Context) *RPCMetadata {
	m, _ := context.GetValue(rpcMetadataKey).(*RPCMetadata)
	return m
}

var suppressTracingKey = api.CreateContextKey("OpenTelemetry SDK Context Key SUPPRESS_TRACING")

// SuppressTracing returns a context in which tracing is suppressed.
func SuppressTracing(context api.Context) api.Context {
	return context.SetValue(suppressTracingKey, true)
}

// UnsuppressTracing returns a context in which tracing is not suppressed.
func UnsuppressTracing(context api.Context) api.Context {
	return context.DeleteValue(suppressTracingKey)
}

// IsTracingSuppressed reports whether the context's suppress-tracing value is true.
func IsTracingSuppressed(context api.Context) bool {
	v, ok := context.GetValue(suppressTracingKey).(bool)
	return ok && v
}
