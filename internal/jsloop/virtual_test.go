package jsloop

import (
	"fmt"
	"strings"
	"testing"
)

// The model the parity oracles share: ticks before tasks and after every
// task, tasks by due time then scheduling order, Node's setTimeout clamp.
func TestVirtualOrder(t *testing.T) {
	v := NewVirtual(1000)
	var log []string
	rec := func(s string) func() {
		return func() { log = append(log, fmt.Sprintf("%d %s", v.Now()-1000, s)) }
	}
	v.SetTimeout(rec("t0"), 0) // clamped to 1 ms
	v.After(0, func() {
		rec("io0")()
		v.NextTick(rec("tick-in-io0"))
		v.SetTimeout(rec("t-from-io0"), 1) // due 1, scheduled after t0
	})
	v.NextTick(func() {
		rec("tick")()
		v.NextTick(rec("tick2"))
	})
	v.SetTimeout(rec("neg"), -5)         // clamped to 1
	v.SetTimeout(rec("big"), 1<<31)      // above TIMEOUT_MAX: clamped to 1
	v.SetTimeout(rec("max"), 1<<31-1)    // kept
	late := v.SetTimeout(rec("late"), 2) // runs at 2; io3 clears it too late
	v.After(3, func() { rec("io3")(); v.ClearTimeout(late) })
	v.ClearTimeout(nil)
	v.ClearTimeout("not a timer")
	v.After(2, rec("io2"))
	v.Run()
	want := []string{
		"0 tick", "0 tick2", "0 io0", "0 tick-in-io0",
		"1 t0", "1 neg", "1 big", "1 t-from-io0", "2 late", "2 io2", "3 io3", fmt.Sprintf("%d max", 1<<31-1),
	}
	if got := strings.Join(log, "|"); got != strings.Join(want, "|") {
		t.Fatalf("order:\n got %s\nwant %s", got, strings.Join(want, "|"))
	}
	if v.Pending() != 0 {
		t.Fatalf("pending %d", v.Pending())
	}
}

// Ticks, then microtasks, again while ticks were queued, before the first
// task and after every task. The order is Node v25.9.0's for the same
// program written with process.nextTick, Promise.resolve().then,
// queueMicrotask and setTimeout.
func TestVirtualMicrotasks(t *testing.T) {
	v := NewVirtual(0)
	var log []string
	rec := func(s string) func() { return func() { log = append(log, s) } }
	v.NextTick(rec("tick1"))
	v.QueueMicrotask(func() {
		rec("micro1")()
		v.NextTick(rec("tick-from-micro1"))
		v.QueueMicrotask(rec("micro-from-micro1"))
	})
	v.QueueMicrotask(rec("micro2"))
	v.NextTick(func() { rec("tick2")(); v.QueueMicrotask(rec("micro-from-tick2")) })
	v.SetTimeout(func() {
		rec("timer1")()
		v.QueueMicrotask(rec("micro-in-timer1"))
		v.NextTick(rec("tick-in-timer1"))
	}, 1)
	v.SetTimeout(func() { rec("timer2")(); v.QueueMicrotask(rec("micro-in-timer2")) }, 1)
	v.Run()
	want := "tick1|tick2|micro1|micro2|micro-from-tick2|micro-from-micro1|tick-from-micro1|timer1|tick-in-timer1|micro-in-timer1|timer2|micro-in-timer2"
	if got := strings.Join(log, "|"); got != want {
		t.Fatalf("order:\n got %s\nwant %s", got, want)
	}

	// A microtask that panics goes to Uncaught; the others still run.
	v = NewVirtual(0)
	log = nil
	v.Uncaught = func(r any) { log = append(log, fmt.Sprint("uncaught ", r)) }
	v.QueueMicrotask(func() { panic("m1") })
	v.QueueMicrotask(rec("m2"))
	v.Run()
	if got := strings.Join(log, "|"); got != "uncaught m1|m2" {
		t.Fatalf("panicking microtask: %s", got)
	}
}

func TestVirtualClear(t *testing.T) {
	v := NewVirtual(0)
	ran := false
	tm := v.SetTimeout(func() { ran = true }, 5)
	v.After(1, func() { v.ClearTimeout(tm) })
	v.Run()
	if ran || v.Now() != 1 {
		t.Fatalf("ran %v now %d", ran, v.Now())
	}
	// Clearing a timer that already ran is a no-op.
	tm = v.SetTimeout(func() {}, 1)
	v.Run()
	v.ClearTimeout(tm)
	if v.Pending() != 0 || v.Now() != 2 {
		t.Fatalf("pending %d now %d", v.Pending(), v.Now())
	}
}

func TestVirtualUncaught(t *testing.T) {
	v := NewVirtual(0)
	var caught []any
	v.Uncaught = func(r any) { caught = append(caught, r) }
	after := false
	v.NextTick(func() { panic("tick") })
	v.After(0, func() { panic("task") })
	v.After(1, func() { after = true })
	v.Run()
	if fmt.Sprint(caught) != "[tick task]" || !after {
		t.Fatalf("caught %v, later task ran %v", caught, after)
	}

	v = NewVirtual(0)
	func() {
		defer func() {
			if r := recover(); r != "boom" {
				t.Fatalf("recovered %v", r)
			}
		}()
		v.After(0, func() { panic("boom") })
		v.Run()
	}()
}

// After an uncaught exception the pending ticks wait for the next task due
// at the current time to complete, as Node runs them from the no-op
// setImmediate it queues then; with no task due they run at once.
func TestVirtualTicksAfterUncaught(t *testing.T) {
	v := NewVirtual(0)
	var log []string
	rec := func(s string) func() {
		return func() { log = append(log, fmt.Sprintf("%d %s", v.Now(), s)) }
	}
	v.Uncaught = func(r any) { log = append(log, fmt.Sprintf("%d uncaught %v", v.Now(), r)) }
	v.NextTick(func() { rec("t")(); panic("t") })
	v.NextTick(rec("u"))   // waits for io0, due now
	v.After(0, rec("io0")) // runs before u
	v.SetTimeout(func() { rec("a")(); v.NextTick(rec("tick-from-a")); panic("a") }, 1)
	v.SetTimeout(rec("b"), 1) // due with a: tick-from-a runs after b
	v.SetTimeout(func() { rec("c")(); v.NextTick(rec("tick-from-c")); panic("c") }, 3)
	v.SetTimeout(rec("d"), 4) // nothing due with c: tick-from-c runs at once
	v.SetTimeout(func() { rec("e")(); v.NextTick(rec("tick-from-e")); panic("e") }, 5)
	v.SetTimeout(func() { rec("f")(); panic("f") }, 5) // throws too: the tick waits for g
	v.SetTimeout(rec("g"), 5)
	v.Run()
	want := []string{
		"0 t", "0 uncaught t", "0 io0", "0 u",
		"1 a", "1 uncaught a", "1 b", "1 tick-from-a",
		"3 c", "3 uncaught c", "3 tick-from-c", "4 d",
		"5 e", "5 uncaught e", "5 f", "5 uncaught f", "5 g", "5 tick-from-e",
	}
	if got := strings.Join(log, "|"); got != strings.Join(want, "|") {
		t.Fatalf("order:\n got %s\nwant %s", got, strings.Join(want, "|"))
	}
}

func TestVirtualMaxTasks(t *testing.T) {
	v := NewVirtual(0)
	v.MaxTasks = 10
	var tick func()
	tick = func() { v.SetTimeout(tick, 0) }
	v.SetTimeout(tick, 0)
	defer func() {
		if r := recover(); r == nil || !strings.Contains(fmt.Sprint(r), "more than 10 tasks") {
			t.Fatalf("recovered %v", r)
		}
		if v.Now() != 10 {
			t.Fatalf("now %d", v.Now())
		}
	}()
	v.Run()
}
