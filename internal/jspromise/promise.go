// Package jspromise models ECMAScript promises (ECMA-262 §27.2) on a
// microtask queue, so that Go translations of promise-based JavaScript run
// their reactions in the same order as V8.
//
// Values follow the jsvalue model (nil is null, jsvalue.Undefined{} is
// undefined). A handler or executor that panics is a JavaScript throw: the
// panic value becomes the rejection reason (Throw throws any value, null
// included). Resolving a promise with a
// *Promise or with a Thenable adopts its state through a
// NewPromiseResolveThenableJob, which costs the same microtask turns as in
// V8.
//
// A promise belongs to the queue it was created on (its derived promises
// too), and like a JavaScript promise it is not safe for concurrent use:
// create, settle and observe it on the goroutine that runs that queue.
// Unhandled rejections are not tracked (Node's default reports them and
// exits).
package jspromise

import (
	"github.com/gav4you/npmgo/internal/jsloop"
	"github.com/gav4you/npmgo/internal/jsvalue"
)

// State is a promise's [[PromiseState]].
type State int

const (
	Pending State = iota
	Fulfilled
	Rejected
)

// Thenable is a non-promise value with a then method: resolving a promise
// with one calls Then (in a microtask) with the promise's resolving
// functions.
type Thenable interface {
	Then(onFulfilled, onRejected func(value any))
}

// Promise is a JavaScript promise.
type Promise struct {
	q         jsloop.Microtasks
	state     State
	result    any
	reactions []reaction // while pending, in registration order
}

// reaction is one then() registration: the fulfil and reject
// PromiseReaction records, which the spec keeps in two parallel lists.
type reaction struct {
	derived     *capability // nil: no promise to settle
	onFulfilled func(any) any
	onRejected  func(any) any
}

// capability is a PromiseCapability record.
type capability struct {
	promise *Promise
	resolve func(any)
	reject  func(any)
}

func newCapability(q jsloop.Microtasks) *capability {
	p := &Promise{q: q}
	resolve, reject := p.resolvingFunctions()
	return &capability{p, resolve, reject}
}

// New is new Promise(executor): executor runs at once with the resolving
// functions; a panic in it rejects the promise (unless already resolved).
func New(q jsloop.Microtasks, executor func(resolve, reject func(any))) *Promise {
	c := newCapability(q)
	func() {
		defer func() {
			if r := recover(); r != nil {
				c.reject(Caught(r))
			}
		}()
		executor(c.resolve, c.reject)
	}()
	return c.promise
}

// Resolve is Promise.resolve(x): x itself if it is a promise, otherwise a new
// promise resolved with x (adopting a Thenable's state).
func Resolve(q jsloop.Microtasks, x any) *Promise {
	if p, ok := x.(*Promise); ok {
		return p
	}
	c := newCapability(q)
	c.resolve(x)
	return c.promise
}

// Reject is Promise.reject(reason).
func Reject(q jsloop.Microtasks, reason any) *Promise {
	c := newCapability(q)
	c.reject(reason)
	return c.promise
}

// Race is Promise.race(values): settled like the first of
// Promise.resolve(value) for each value to settle.
func Race(q jsloop.Microtasks, values []any) *Promise {
	c := newCapability(q)
	for _, v := range values {
		Resolve(q, v).Then(
			func(x any) any { c.resolve(x); return jsvalue.Undefined{} },
			func(r any) any { c.reject(r); return jsvalue.Undefined{} })
	}
	return c.promise
}

// State returns the promise's state.
func (p *Promise) State() State { return p.state }

// Result returns the fulfilment value or rejection reason (undefined while
// pending).
func (p *Promise) Result() any {
	if p.state == Pending {
		return jsvalue.Undefined{}
	}
	return p.result
}

// Then is promise.then(onFulfilled, onRejected); a nil handler passes the
// value or reason through, as a non-callable argument does.
func (p *Promise) Then(onFulfilled, onRejected func(any) any) *Promise {
	c := newCapability(p.q)
	p.performThen(reaction{c, onFulfilled, onRejected})
	return c.promise
}

// Catch is promise.catch(onRejected).
func (p *Promise) Catch(onRejected func(any) any) *Promise { return p.Then(nil, onRejected) }

// performThen is PerformPromiseThen.
func (p *Promise) performThen(r reaction) {
	switch p.state {
	case Pending:
		p.reactions = append(p.reactions, r)
	default:
		p.queueReactionJob(r, p.state, p.result)
	}
}

// resolvingFunctions is CreateResolvingFunctions: resolve and reject
// sharing one alreadyResolved flag.
func (p *Promise) resolvingFunctions() (resolve, reject func(any)) {
	already := false
	resolve = func(resolution any) {
		if already {
			return
		}
		already = true
		p.resolveWith(resolution)
	}
	reject = func(reason any) {
		if already {
			return
		}
		already = true
		p.settle(Rejected, reason)
	}
	return resolve, reject
}

// resolveWith is the body of a promise resolve function after the
// alreadyResolved check.
func (p *Promise) resolveWith(resolution any) {
	switch r := resolution.(type) {
	case *Promise:
		if r == p {
			p.settle(Rejected, &jsvalue.TypeError{Message: "Chaining cycle detected for promise #<Promise>"})
			return
		}
		// NewPromiseResolveThenableJob with the intrinsic then.
		p.q.QueueMicrotask(func() {
			resolve, reject := p.resolvingFunctions()
			r.performThen(reaction{newCapability(r.q), wrap(resolve), wrap(reject)})
		})
	case Thenable:
		if jsvalue.IsNull(resolution) {
			p.settle(Fulfilled, resolution)
			return
		}
		p.q.QueueMicrotask(func() {
			resolve, reject := p.resolvingFunctions()
			defer func() {
				if x := recover(); x != nil {
					reject(Caught(x))
				}
			}()
			r.Then(resolve, reject)
		})
	default:
		p.settle(Fulfilled, resolution)
	}
}

// wrap turns a resolving function into a reaction handler returning
// undefined.
func wrap(f func(any)) func(any) any {
	return func(v any) any { f(v); return jsvalue.Undefined{} }
}

// settle is FulfillPromise / RejectPromise: it records the result and queues
// a job for each reaction, in order.
func (p *Promise) settle(state State, result any) {
	if p.state != Pending {
		return
	}
	p.state, p.result = state, result
	reactions := p.reactions
	p.reactions = nil
	for _, r := range reactions {
		p.queueReactionJob(r, state, result)
	}
}

// queueReactionJob queues NewPromiseReactionJob(reaction, argument).
func (p *Promise) queueReactionJob(r reaction, state State, argument any) {
	p.q.QueueMicrotask(func() {
		handler := r.onFulfilled
		if state == Rejected {
			handler = r.onRejected
		}
		var value any
		abrupt := false
		switch {
		case handler == nil && state == Fulfilled:
			value = argument
		case handler == nil:
			value, abrupt = argument, true
		default:
			value, abrupt = call(handler, argument)
		}
		if r.derived == nil {
			return
		}
		if abrupt {
			r.derived.reject(value)
		} else {
			r.derived.resolve(value)
		}
	})
}

// call runs a handler, turning a panic into an abrupt completion.
func call(handler func(any) any, argument any) (value any, abrupt bool) {
	defer func() {
		if x := recover(); x != nil {
			value, abrupt = Caught(x), true
		}
	}()
	return handler(argument), false
}

// thrown carries a value thrown with Throw.
type thrown struct{ value any }

// Throw throws value (JavaScript's throw value) from a handler or executor: the promise is rejected with value
// itself, which may be nil (null), unlike panic(nil), which Go turns into a *runtime.PanicNilError.
func Throw(value any) { panic(thrown{value}) }

// Caught returns the thrown value a recovered panic stands for: the value given to Throw, or the panic value.
func Caught(recovered any) any {
	if t, ok := recovered.(thrown); ok {
		return t.value
	}
	return recovered
}
