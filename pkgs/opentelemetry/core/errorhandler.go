// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
//
// Translated to Go by the npmgo contributors from @opentelemetry/core 2.11.0 (build/src/common/global-error-handler.js
// and common/logging-error-handler.js).

package core

import (
	"sync"

	"github.com/gav4you/npmgo/internal/jsvalue"
	"github.com/gav4you/npmgo/pkgs/opentelemetry/api"
)

// ErrorHandler handles an exception: a string, a Go error (a JavaScript Error), a *jsvalue.Object, or any other
// value.
type ErrorHandler func(ex any)

var (
	delegateMu      sync.RWMutex
	delegateHandler = LoggingErrorHandler()
)

// SetGlobalErrorHandler sets the handler GlobalErrorHandler delegates to (initially a LoggingErrorHandler).
func SetGlobalErrorHandler(handler ErrorHandler) {
	delegateMu.Lock()
	defer delegateMu.Unlock()
	delegateHandler = handler
}

// GlobalErrorHandler passes ex to the global error handler; a panic in the handler is swallowed.
func GlobalErrorHandler(ex any) {
	delegateMu.RLock()
	h := delegateHandler
	delegateMu.RUnlock()
	defer func() { _ = recover() }()
	h(ex)
}

// LoggingErrorHandler returns a handler that logs the exception as an error to api.Diag: a string as it is, anything
// else as the JSON of its flattened properties (see stringifyException).
func LoggingErrorHandler() ErrorHandler {
	return func(ex any) { api.Diag.Error(stringifyException(ex)) }
}

// stringifyException is a string as it is, or JSON.stringify(flattenException(ex)).
func stringifyException(ex any) string {
	if s, ok := ex.(string); ok {
		return s
	}
	s, _ := jsvalue.JSONStringify(flattenException(ex))
	return s
}

// protoProperty is a property of a built-in prototype: its name and value (a builtinFunction, or a string).
type protoProperty struct {
	name  string
	value any
}

func methods(names ...string) []protoProperty {
	out := make([]protoProperty, len(names))
	for i, n := range names {
		out[i] = protoProperty{n, builtinFunction{n}}
	}
	return out
}

// The own properties of the built-in prototypes flattenException walks, in V8's order (Object.prototype's add
// nothing: every one of its names is already "set" on the result through its prototype).
var (
	numberPrototype  = methods("constructor", "toExponential", "toFixed", "toPrecision", "toString", "valueOf", "toLocaleString")
	booleanPrototype = methods("constructor", "toString", "valueOf")
	errorPrototype   = []protoProperty{{"constructor", builtinFunction{"Error"}}, {"name", "Error"}, {"message", ""}, {"toString", builtinFunction{"toString"}}}
)

// nativeErrorPrototype is the prototype of a native error class other than Error: constructor, name and message.
func nativeErrorPrototype(name string) []protoProperty {
	return []protoProperty{{"constructor", builtinFunction{name}}, {"name", name}, {"message", ""}}
}

// String renders a built-in function as String(fn) does.
func (f builtinFunction) String() string { return "function " + f.name + "() { [native code] }" }

// flattenException collects ex's properties along its prototype chain, each name once (the first truthy one; a name
// that Object.prototype has, such as toString, is never taken, because result[name] already finds it there), as
// strings (String(value)).
//
// The chain is modelled for null (nothing), undefined (TypeError, as Object.getOwnPropertyNames(undefined) throws), a
// Go error (an Error without a stack: its message, then the prototype of its class: name TypeError, RangeError or
// URIError for the jsvalue errors, Error otherwise), a *jsvalue.Object (its own properties), numbers and booleans
// (their prototypes' methods). Any other value counts as an object without properties.
func flattenException(ex any) *jsvalue.Object {
	result := &jsvalue.Object{}
	add := func(name string, value any) {
		if v, ok := result.Lookup(name); (ok && jsvalue.Truthy(v)) || objectPrototypeMethods[name] || name == "__proto__" {
			return
		}
		if jsvalue.Truthy(value) {
			result.Set(name, jsvalue.String(value))
		}
	}
	addAll := func(props []protoProperty) {
		for _, p := range props {
			add(p.name, p.value)
		}
	}
	switch e := ex.(type) {
	case nil:
	case jsvalue.Undefined:
		panic(&jsvalue.TypeError{Message: "Cannot convert undefined or null to object"})
	case bool:
		addAll(booleanPrototype)
	case *jsvalue.Object:
		e.Range(func(k string, v any) bool { add(k, v); return true })
	case error:
		name, message := "Error", e.Error()
		switch x := e.(type) {
		case *jsvalue.TypeError:
			name, message = "TypeError", x.Message
		case *jsvalue.RangeError:
			name, message = "RangeError", x.Message
		case *jsvalue.URIError:
			name, message = "URIError", x.Message
		}
		add("message", message)
		if name != "Error" {
			addAll(nativeErrorPrototype(name))
		}
		addAll(errorPrototype)
	default:
		if jsvalue.IsNumber(ex) {
			addAll(numberPrototype)
		}
	}
	return result
}
