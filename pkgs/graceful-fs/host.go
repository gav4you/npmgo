package gracefulfs

import (
	"sync"
	"time"

	"github.com/gav4you/npmgo/internal/jsloop"
)

// realHost is a Host on the wall clock that runs everything on one goroutine,
// like Node's event loop: work posted from any goroutine (file system
// completions, timers firing, calls from the blocking functions) runs there
// one task at a time, and NextTick callbacks run after the current task.
type realHost struct {
	start sync.Once
	loop  *jsloop.Loop
}

func newRealHost() *realHost { return &realHost{loop: jsloop.New()} }

// post runs fn on the loop goroutine, starting it on first use. Safe from any
// goroutine.
func (h *realHost) post(fn func()) {
	h.start.Do(func() { go h.loop.Run(func() bool { return false }) })
	h.loop.SetImmediate(fn)
}

// Now is Date.now().
func (h *realHost) Now() int64 { return time.Now().UnixMilli() }

// NextTick queues fn to run after the current task (loop goroutine only).
func (h *realHost) NextTick(fn func()) { h.loop.QueueMicrotask(fn) }

// realTimer is a pending SetTimeout; cleared is only touched on the loop.
type realTimer struct {
	t       *time.Timer
	cleared bool
}

// SetTimeout runs fn on the loop ms milliseconds from now (loop goroutine
// only). Like Node, it runs a delay outside 1..2^31-1 ms after 1 ms.
func (h *realHost) SetTimeout(fn func(), ms int64) any {
	if ms < 1 || ms > 1<<31-1 {
		ms = 1
	}
	rt := &realTimer{}
	rt.t = time.AfterFunc(time.Duration(ms)*time.Millisecond, func() {
		h.post(func() {
			if !rt.cleared {
				rt.cleared = true
				fn()
			}
		})
	})
	return rt
}

// ClearTimeout cancels a timer from SetTimeout (loop goroutine only).
func (h *realHost) ClearTimeout(timer any) {
	if rt, ok := timer.(*realTimer); ok {
		rt.cleared = true
		rt.t.Stop()
	}
}
