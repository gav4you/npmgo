// Package jsloop is a small deterministic event loop modelled on Node's, used
// to run one React render the way Node would: a microtask queue that is
// drained after every macrotask, and a FIFO macrotask (setImmediate) queue.
//
// Any goroutine may queue work (that is how promises settled elsewhere wake a
// render); only the goroutine driving the loop runs it. After Close, queued
// work runs on the queuing goroutine (or on the goroutine already running
// such work), one task at a time, which models microtasks running after a
// synchronous render has returned.
package jsloop

import "sync"

// Microtasks is something that runs queued JavaScript microtasks.
type Microtasks interface {
	QueueMicrotask(task func())
}

// Loop is one event loop.
type Loop struct {
	mu     sync.Mutex
	micro  []func()
	macro  []func()
	wake   chan struct{}
	closed bool
	// late is work queued after Close; draining is set while a goroutine
	// runs it (see drainLateLocked).
	late     []func()
	draining bool
}

// New returns an empty loop.
func New() *Loop { return &Loop{wake: make(chan struct{}, 1)} }

func (l *Loop) signal() {
	select {
	case l.wake <- struct{}{}:
	default:
	}
}

// QueueMicrotask queues task (queueMicrotask). Safe from any goroutine.
func (l *Loop) QueueMicrotask(task func()) {
	l.mu.Lock()
	if l.closed {
		l.late = append(l.late, task)
		l.drainLateLocked()
		return
	}
	l.micro = append(l.micro, task)
	l.mu.Unlock()
	l.signal()
}

// SetImmediate queues a macrotask (setImmediate). Safe from any goroutine.
func (l *Loop) SetImmediate(task func()) {
	l.mu.Lock()
	if l.closed {
		l.late = append(l.late, task)
		l.drainLateLocked()
		return
	}
	l.macro = append(l.macro, task)
	l.mu.Unlock()
	l.signal()
}

// drainLateLocked runs the work queued after Close, unless another call is
// already doing so: then that call runs what was just queued, after its
// current task. Called with mu held; returns with mu released. One goroutine
// at a time runs late work, so it stays serialized, and a task that queues
// more work (a promise reaction pinging a task, which schedules performWork)
// never waits on itself.
func (l *Loop) drainLateLocked() {
	if l.draining {
		l.mu.Unlock()
		return
	}
	l.draining = true
	for len(l.late) > 0 {
		t := l.late[0]
		l.late[0] = nil
		l.late = l.late[1:]
		l.mu.Unlock()
		t()
		l.mu.Lock()
	}
	l.draining = false
	l.mu.Unlock()
}

func (l *Loop) popMicro() func() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.micro) == 0 {
		return nil
	}
	t := l.micro[0]
	l.micro[0] = nil
	l.micro = l.micro[1:]
	return t
}

func (l *Loop) popMacro() func() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.macro) == 0 {
		return nil
	}
	t := l.macro[0]
	l.macro[0] = nil
	l.macro = l.macro[1:]
	return t
}

// Drain runs microtasks until the queue is empty (a microtask checkpoint).
func (l *Loop) Drain() {
	for t := l.popMicro(); t != nil; t = l.popMicro() {
		t()
	}
}

// Step drains microtasks, then runs one macrotask (draining again after it).
// It reports whether a macrotask ran.
func (l *Loop) Step() bool {
	l.Drain()
	t := l.popMacro()
	if t == nil {
		return false
	}
	t()
	l.Drain()
	return true
}

// Idle reports whether no work is queued.
func (l *Loop) Idle() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.micro) == 0 && len(l.macro) == 0
}

// RunUntilIdle runs queued work until none is left.
func (l *Loop) RunUntilIdle() {
	for l.Step() {
	}
	l.Drain()
}

// Run drives the loop until done reports true, blocking while no work is
// queued. done is checked whenever the loop is idle.
func (l *Loop) Run(done func() bool) {
	for {
		l.RunUntilIdle()
		if done() {
			return
		}
		<-l.wake
	}
}

// Close drains queued microtasks one last time and switches the loop to
// running later work on whichever goroutine queues it.
func (l *Loop) Close() {
	l.Drain()
	l.mu.Lock()
	l.closed = true
	l.late = append(l.late, l.micro...) // queued since Drain emptied the queue
	l.micro, l.macro = nil, nil         // macrotasks of a finished render have nothing left to do
	l.drainLateLocked()
}

// thenOnner is implemented by promises that deliver reactions as microtasks
// on a given queue, like JavaScript promises (react.Promise).
type thenOnner interface {
	ThenOn(q Microtasks, onFulfilled, onRejected func(any))
}

type thener interface {
	Then(onFulfilled, onRejected func(any))
}

// Then registers onFulfilled / onRejected on thenable t like JavaScript's
// t.then(onFulfilled, onRejected), on behalf of code driven by q (q may be
// nil when no loop drives the caller).
//
// Promises that implement ThenOn deliver reactions as microtasks on q, as a
// JavaScript promise does. For other thenables, a callback invoked while t's
// Then method is still running (a synchronous thenable) runs right after Then
// returns; a callback invoked later, from any goroutine, is queued as a
// microtask on q (or run directly when q is nil).
func Then(q Microtasks, t any, onFulfilled, onRejected func(any)) {
	if p, ok := t.(thenOnner); ok {
		p.ThenOn(q, onFulfilled, onRejected)
		return
	}
	th, ok := t.(thener)
	if !ok {
		return
	}
	type call struct {
		f func(any)
		v any
	}
	var (
		mu      sync.Mutex
		inside  = true
		pending []call
	)
	wrap := func(f func(any)) func(any) {
		if f == nil {
			return func(any) {}
		}
		return func(v any) {
			mu.Lock()
			if inside {
				pending = append(pending, call{f, v})
				mu.Unlock()
				return
			}
			mu.Unlock()
			if q == nil {
				f(v)
				return
			}
			q.QueueMicrotask(func() { f(v) })
		}
	}
	th.Then(wrap(onFulfilled), wrap(onRejected))
	mu.Lock()
	inside = false
	calls := pending
	pending = nil
	mu.Unlock()
	for _, c := range calls {
		c.f(c.v)
	}
}
