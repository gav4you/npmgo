package jsloop

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestOrderBeforeClose(t *testing.T) {
	l := New()
	var order []string
	l.SetImmediate(func() {
		order = append(order, "macro1")
		l.QueueMicrotask(func() { order = append(order, "micro-in-macro1") })
	})
	l.QueueMicrotask(func() {
		order = append(order, "micro1")
		l.QueueMicrotask(func() { order = append(order, "micro2") })
	})
	l.SetImmediate(func() { order = append(order, "macro2") })
	l.RunUntilIdle()
	if got := fmt.Sprint(order); got != "[micro1 micro2 macro1 micro-in-macro1 macro2]" {
		t.Fatal(got)
	}
	if !l.Idle() {
		t.Fatal("not idle")
	}
}

func TestRunWakesOnWorkFromAnotherGoroutine(t *testing.T) {
	l := New()
	done := false
	go func() {
		time.Sleep(5 * time.Millisecond)
		l.QueueMicrotask(func() { done = true })
	}()
	l.Run(func() bool { return done })
	if !done {
		t.Fatal("did not run the queued work")
	}
}

// Work queued after Close runs on the queuing goroutine, and work that such a
// task queues itself runs after it instead of deadlocking (a promise reaction
// pings a task, which schedules performWork with setImmediate).
func TestQueueFromTaskAfterClose(t *testing.T) {
	l := New()
	l.QueueMicrotask(func() {}) // queued before Close: run by Close
	l.Close()
	finished := make(chan string, 1)
	go func() {
		var order []int
		l.QueueMicrotask(func() {
			order = append(order, 1)
			l.SetImmediate(func() { order = append(order, 3) })
			order = append(order, 2)
		})
		finished <- fmt.Sprint(order)
	}()
	select {
	case got := <-finished:
		if got != "[1 2 3]" {
			t.Fatal(got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("queueing from a task run after Close deadlocked")
	}
}

// Late work from many goroutines runs one task at a time (run with -race:
// the counter is unprotected).
func TestLateWorkIsSerialized(t *testing.T) {
	l := New()
	l.Close()
	n := 0
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			l.QueueMicrotask(func() {
				n++
				l.SetImmediate(func() { n++ })
			})
		}()
	}
	wg.Wait()
	if n != 100 {
		t.Fatalf("ran %d of 100 tasks", n)
	}
}
