package react

import (
	"sync"

	"github.com/gav4you/npmgo/internal/jsloop"
)

// Thenable is anything React can wait on: use(thenable), a thenable child,
// or what a Lazy loader returns. Like a JavaScript thenable it has a Then
// method; React also stores its bookkeeping on the thenable (the status,
// value and reason expando properties), which is what ThenableStatus and
// SetThenableStatus read and write. Embed ThenableState to get them.
//
// Then may call a callback synchronously (a "synchronous thenable": React
// then sees the result without suspending) or later, from any goroutine.
type Thenable interface {
	Then(onFulfilled, onRejected func(value any))
	ThenableStatus() (status string, value, reason any)
	SetThenableStatus(status string, value, reason any)
}

// ThenableState holds React's status / value / reason bookkeeping for a
// thenable. Status "" means untracked (JavaScript undefined); React sets
// "pending", "fulfilled" or "rejected". Set it to "fulfilled" yourself to
// hand React an already-resolved value that it reads without suspending.
// It is safe for concurrent use.
type ThenableState struct {
	mu     sync.Mutex
	status string
	value  any
	reason any
}

// ThenableStatus returns the status, value and reason.
func (s *ThenableState) ThenableStatus() (status string, value, reason any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status, s.value, s.reason
}

// SetThenableStatus sets the status, value and reason.
func (s *ThenableState) SetThenableStatus(status string, value, reason any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status, s.value, s.reason = status, value, reason
}

// Microtasks is a JavaScript microtask queue; renderers pass theirs to
// Promise.ThenOn so that promise reactions run on the render's event loop.
type Microtasks = jsloop.Microtasks

// Promise models a JavaScript Promise. Like a JavaScript promise it never
// calls a reaction synchronously from Then: React therefore suspends at least
// once on an untracked Promise even if it has already settled. Settle it from
// any goroutine with the resolve / reject functions from NewPromise.
type Promise struct {
	ThenableState

	mu        sync.Mutex
	settled   bool
	rejected  bool
	result    any
	reactions []reaction
	deferred  []func() // reactions with no microtask queue, waiting for one
}

type reaction struct {
	q         Microtasks
	onFulfill func(any)
	onReject  func(any)
}

// NewPromise returns a pending promise and its resolve and reject functions.
func NewPromise() (p *Promise, resolve func(value any), reject func(reason any)) {
	p = &Promise{}
	return p, func(v any) { p.settle(false, v) }, func(r any) { p.settle(true, r) }
}

// ResolvedPromise is Promise.resolve(value): fulfilled, but not yet tracked
// by React (its status is untracked).
func ResolvedPromise(value any) *Promise {
	p := &Promise{settled: true, result: value}
	return p
}

// RejectedPromise is Promise.reject(reason).
func RejectedPromise(reason any) *Promise {
	return &Promise{settled: true, rejected: true, result: reason}
}

func (p *Promise) settle(rejected bool, v any) {
	p.mu.Lock()
	if p.settled {
		p.mu.Unlock()
		return
	}
	p.settled, p.rejected, p.result = true, rejected, v
	rs := p.reactions
	p.reactions = nil
	p.mu.Unlock()
	for _, r := range rs {
		p.deliver(r, rejected, v, false)
	}
}

// deliver runs or queues one reaction for a settled promise. Reactions
// without a queue run directly when the promise settles later, but are held
// back when registered on an already-settled promise (JavaScript would run
// them as a microtask) until a queue-bearing registration flushes them.
func (p *Promise) deliver(r reaction, rejected bool, v any, registeredSettled bool) {
	f := r.onFulfill
	if rejected {
		f = r.onReject
	}
	if f == nil {
		return
	}
	if r.q != nil {
		r.q.QueueMicrotask(func() { f(v) })
		return
	}
	if registeredSettled {
		p.mu.Lock()
		p.deferred = append(p.deferred, func() { f(v) })
		p.mu.Unlock()
		return
	}
	f(v)
}

// Then registers reactions. Reactions registered on a pending promise run on
// the goroutine that settles it; reactions registered on a settled promise
// are deferred like JavaScript microtasks and run when a render next waits on
// the promise.
func (p *Promise) Then(onFulfilled, onRejected func(value any)) {
	p.ThenOn(nil, onFulfilled, onRejected)
}

// ThenOn registers reactions that are delivered as microtasks on q when the
// promise settles (immediately queued if it already has).
func (p *Promise) ThenOn(q Microtasks, onFulfilled, onRejected func(value any)) {
	r := reaction{q, onFulfilled, onRejected}
	p.mu.Lock()
	if !p.settled {
		p.reactions = append(p.reactions, r)
		p.mu.Unlock()
		return
	}
	var deferred []func()
	if q != nil {
		deferred, p.deferred = p.deferred, nil
	}
	rejected, v := p.rejected, p.result
	p.mu.Unlock()
	for _, d := range deferred {
		q.QueueMicrotask(d)
	}
	p.deliver(r, rejected, v, true)
}

// Settled reports whether the promise has settled, and how.
func (p *Promise) Settled() (settled, rejected bool, result any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.settled, p.rejected, p.result
}

func noop(any) {}
