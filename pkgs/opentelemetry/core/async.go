// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
//
// Translated to Go by the npmgo contributors from @opentelemetry/core 2.11.0 (build/src/utils/timeout.js,
// utils/callback.js, utils/promise.js and internal/exporter.js).

package core

import (
	"github.com/gav4you/npmgo/internal/jsloop"
	"github.com/gav4you/npmgo/internal/jspromise"
	"github.com/gav4you/npmgo/internal/jsvalue"
	"github.com/gav4you/npmgo/pkgs/opentelemetry/api"
)

// Host is the event loop the promise utilities run on: the microtask queue that runs promise reactions, and Node's
// setTimeout and clearTimeout (jsloop.Virtual is one, on a virtual clock). Promises are not safe for concurrent use:
// use them on the goroutine that runs the host.
type Host interface {
	jsloop.Microtasks
	// SetTimeout runs fn after ms milliseconds (a delay outside 1..2^31-1 counts as 1, as in Node) and returns the
	// timer.
	SetTimeout(fn func(), ms int64) any
	// ClearTimeout cancels a timer SetTimeout returned (other values are ignored).
	ClearTimeout(timer any)
}

// TimeoutError is the error CallWithTimeout rejects with. Like the original's class, which extends Error without
// naming itself, its name is "Error".
type TimeoutError struct {
	Message string
}

// NewTimeoutError is new TimeoutError(message).
func NewTimeoutError(message string) *TimeoutError { return &TimeoutError{Message: message} }

func (e *TimeoutError) Error() string { return e.Message }

// timerDelay is the delay Node's setTimeout(fn, ms) waits, in whole milliseconds: 1 outside 1..2^31-1 (NaN
// included), otherwise ms truncated: Node's timers list a timer under Math.trunc of its delay and fire it once that
// many whole milliseconds have passed (checked against Node v25.9.0: a 30.9 ms timer fires with the 30 ms ones,
// before a 31 ms one, and a 1.9 ms timer after 1 ms).
func timerDelay(ms float64) int64 {
	if !(ms >= 1 && ms <= 1<<31-1) {
		return 1
	}
	return int64(ms)
}

// CallWithTimeout returns a promise settled like promise, or rejected with TimeoutError("Operation timed out.")
// after timeout milliseconds if that comes first; the timer is cleared when it settles. promise itself carries on.
// A value that is not a promise is adopted (Promise.resolve) as Promise.race does.
func CallWithTimeout(host Host, promise any, timeout float64) *jspromise.Promise {
	var timeoutHandle any
	timeoutPromise := jspromise.New(host, func(_, reject func(any)) {
		timeoutHandle = host.SetTimeout(func() { reject(NewTimeoutError("Operation timed out.")) }, timerDelay(timeout))
	})
	return jspromise.Race(host, []any{promise, timeoutPromise}).Then(
		func(result any) any {
			host.ClearTimeout(timeoutHandle)
			return result
		},
		func(reason any) any {
			host.ClearTimeout(timeoutHandle)
			jspromise.Throw(reason)
			return nil
		})
}

// deferred is Deferred: a promise with its resolve and reject functions.
type deferred struct {
	promise *jspromise.Promise
	resolve func(any)
	reject  func(any)
}

func newDeferred(q jsloop.Microtasks) *deferred {
	d := &deferred{}
	d.promise = jspromise.New(q, func(resolve, reject func(any)) { d.resolve, d.reject = resolve, reject })
	return d
}

// BindOnceFuture calls its callback once, however many times Call is called, and settles its promise like the
// callback's result. JavaScript's `that` (the callback's this) has no Go counterpart: bind a receiver with a method
// value.
type BindOnceFuture struct {
	q        jsloop.Microtasks
	isCalled bool
	deferred *deferred
	callback func(args ...any) any
}

// NewBindOnceFuture is new BindOnceFuture(callback, that); its promises run on q.
func NewBindOnceFuture(q jsloop.Microtasks, callback func(args ...any) any) *BindOnceFuture {
	return &BindOnceFuture{q: q, deferred: newDeferred(q), callback: callback}
}

// IsCalled reports whether Call was called.
func (b *BindOnceFuture) IsCalled() bool { return b.isCalled }

// Promise returns the promise of the callback's result.
func (b *BindOnceFuture) Promise() *jspromise.Promise { return b.deferred.promise }

// Call calls the callback with args the first time, and returns the promise of its result every time: fulfilled
// with its value (a promise or thenable is adopted), or rejected with what it threw.
func (b *BindOnceFuture) Call(args ...any) *jspromise.Promise {
	if !b.isCalled {
		b.isCalled = true
		func() {
			defer func() {
				if r := recover(); r != nil {
					b.deferred.reject(jspromise.Caught(r))
				}
			}()
			jspromise.Resolve(b.q, b.callback(args...)).Then(
				func(val any) any { b.deferred.resolve(val); return jsvalue.Undefined{} },
				func(err any) any { b.deferred.reject(err); return jsvalue.Undefined{} })
		}()
	}
	return b.deferred.promise
}

// Exporter is an exporter as internal._export calls it: Export(arg, resultCallback), which reports its result
// through resultCallback.
type Exporter interface {
	Export(arg any, resultCallback func(result ExportResult))
}

// Internal is core's `internal` export.
var Internal = internal{}

type internal struct{}

// Export is internal._export(exporter, arg): it calls exporter.Export with tracing suppressed in the active context
// (so that exporting creates no spans) and returns a promise of the first result the exporter reports; a panic in
// Export rejects it.
func (internal) Export(q jsloop.Microtasks, exporter Exporter, arg any) *jspromise.Promise {
	return jspromise.New(q, func(resolve, _ func(any)) {
		api.Ctx.With(SuppressTracing(api.Ctx.Active()), func(...any) any {
			exporter.Export(arg, func(result ExportResult) { resolve(result) })
			return jsvalue.Undefined{}
		})
	})
}
