package jsloop

import (
	"container/heap"
	"fmt"
)

// Virtual is a deterministic, single-goroutine event loop on a virtual clock,
// for replaying timer-driven code the same way in Go and in a Node oracle.
// The parity oracles implement the same model in JavaScript
// (parity/oracle_gracefulfs.cjs, class Virtual):
//
//   - Now is the virtual time in milliseconds. It changes only when Run
//     moves on to a task that is due later.
//   - NextTick queues a callback on the tick queue. Run drains the tick
//     queue, including callbacks queued while it drains, before the first
//     task and after every task, as Node drains process.nextTick callbacks.
//   - SetTimeout schedules a timer ms milliseconds ahead. As in Node, a delay
//     below 1 or above 2^31-1 becomes 1 ms, so setTimeout(fn, 0) runs 1 ms
//     later.
//   - After schedules host work, such as the completion of an I/O request,
//     exactly ms milliseconds ahead (0: later in the current millisecond),
//     without the timer clamp.
//   - Run runs timers and host work in order of due time, then in the order
//     they were scheduled, until neither tasks nor ticks remain.
//   - After an uncaught exception (see Uncaught) the pending ticks are not
//     run at once: Node skips the tick queue when a callback throws and
//     queues a no-op setImmediate so that the next callback to complete
//     runs them. Here they wait for the next task due at the current time
//     to complete normally, and run at once when no task is due.
//
// A Virtual is not safe for concurrent use.
type Virtual struct {
	// Uncaught, if set, receives the value of a panic raised by a task or a
	// tick callback, and Run carries on (an uncaught exception the process
	// survives). If nil, the panic propagates out of Run.
	Uncaught func(r any)
	// MaxTasks, if positive, makes Run panic when it is about to run more
	// than that many tasks: a guard against runaway timers in tests.
	MaxTasks int

	now   int64
	seq   uint64
	ticks []func()
	tasks taskQueue
	ran   int
	// held is set after an uncaught exception: the ticks wait for the next
	// task due now, if any, to complete.
	held bool
}

// NewVirtual returns an empty loop whose clock reads now.
func NewVirtual(now int64) *Virtual { return &Virtual{now: now} }

// Now returns the virtual time (Date.now()).
func (v *Virtual) Now() int64 { return v.now }

// NextTick queues fn on the tick queue (process.nextTick).
func (v *Virtual) NextTick(fn func()) { v.ticks = append(v.ticks, fn) }

// timeoutMax is Node's TIMEOUT_MAX, 2^31-1.
const timeoutMax = 1<<31 - 1

// SetTimeout schedules fn to run ms milliseconds from now (setTimeout) and
// returns the timer, for ClearTimeout. Delays outside 1..2^31-1 become 1.
func (v *Virtual) SetTimeout(fn func(), ms int64) any {
	if ms < 1 || ms > timeoutMax {
		ms = 1
	}
	return v.schedule(ms, fn)
}

// ClearTimeout cancels a timer returned by SetTimeout (clearTimeout). Any
// other value, a nil one included, and a timer that already ran are ignored.
func (v *Virtual) ClearTimeout(timer any) {
	t, ok := timer.(*vtask)
	if !ok || t.index < 0 {
		return
	}
	heap.Remove(&v.tasks, t.index)
}

// After schedules fn to run ms milliseconds from now (a negative delay counts
// as 0), without SetTimeout's clamp.
func (v *Virtual) After(ms int64, fn func()) {
	if ms < 0 {
		ms = 0
	}
	v.schedule(ms, fn)
}

func (v *Virtual) schedule(ms int64, fn func()) *vtask {
	t := &vtask{due: v.now + ms, seq: v.seq, fn: fn}
	v.seq++
	heap.Push(&v.tasks, t)
	return t
}

// Pending returns the number of scheduled tasks (timers and host work).
func (v *Virtual) Pending() int { return len(v.tasks) }

// Run runs ticks and tasks until none remain.
func (v *Virtual) Run() {
	for {
		// Ticks run before the first task and after every task, except that
		// after an uncaught exception they wait for a task due now to
		// complete (Node's no-op setImmediate runs them otherwise).
		if !v.held || len(v.tasks) == 0 || v.tasks[0].due > v.now {
			v.held = false
			v.drainTicks()
			if v.held { // a tick threw: look again
				continue
			}
		}
		if len(v.tasks) == 0 {
			return
		}
		if v.MaxTasks > 0 && v.ran >= v.MaxTasks {
			panic(fmt.Sprintf("jsloop: virtual loop ran more than %d tasks", v.MaxTasks))
		}
		v.ran++
		t := heap.Pop(&v.tasks).(*vtask)
		if t.due > v.now {
			v.now = t.due
		}
		v.held = !v.call(t.fn)
	}
}

// drainTicks runs the queued ticks, and the ticks they queue, in order,
// stopping (with held set) at one that throws.
func (v *Virtual) drainTicks() {
	for len(v.ticks) > 0 {
		fn := v.ticks[0]
		v.ticks[0] = nil
		v.ticks = v.ticks[1:]
		if !v.call(fn) {
			v.held = true
			return
		}
	}
}

// call runs fn and reports whether it completed; a panic goes to Uncaught
// when that is set, otherwise it propagates.
func (v *Virtual) call(fn func()) (completed bool) {
	if v.Uncaught != nil {
		defer func() {
			if r := recover(); r != nil {
				v.Uncaught(r)
			}
		}()
	}
	fn()
	return true
}

// vtask is a scheduled timer or piece of host work; index is its position in
// the heap, -1 once it has run or been cleared.
type vtask struct {
	due   int64
	seq   uint64
	fn    func()
	index int
}

// taskQueue is a min-heap of tasks by due time, then scheduling order.
type taskQueue []*vtask

func (q taskQueue) Len() int { return len(q) }
func (q taskQueue) Less(i, j int) bool {
	if q[i].due != q[j].due {
		return q[i].due < q[j].due
	}
	return q[i].seq < q[j].seq
}
func (q taskQueue) Swap(i, j int) {
	q[i], q[j] = q[j], q[i]
	q[i].index = i
	q[j].index = j
}
func (q *taskQueue) Push(x any) {
	t := x.(*vtask)
	t.index = len(*q)
	*q = append(*q, t)
}
func (q *taskQueue) Pop() any {
	old := *q
	n := len(old)
	t := old[n-1]
	old[n-1] = nil
	t.index = -1
	*q = old[:n-1]
	return t
}
