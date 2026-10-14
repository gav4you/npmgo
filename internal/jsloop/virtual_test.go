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
