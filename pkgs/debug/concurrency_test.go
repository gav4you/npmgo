package debug

import (
	"strings"
	"sync"
	"testing"
)

// TestConcurrentUse logs through one Debugger from several goroutines while
// the enabled namespaces change; run under -race.
func TestConcurrentUse(t *testing.T) {
	Enable("*")
	d := New("race")
	var sink sync.Mutex
	var lines []string
	d.Log = func(line string) {
		sink.Lock()
		lines = append(lines, line)
		sink.Unlock()
	}
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				d.Print("hello %s %O", "x", map[string]int{"a": j})
				_ = d.Enabled()
				_ = Enabled("race")
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 20; j++ {
			Enable("race,-other")
			d.SetEnabled(true)
			_ = Disable()
			Enable("*")
		}
	}()
	wg.Wait()
	for _, l := range lines {
		if !strings.Contains(l, "hello x") {
			t.Errorf("unexpected line %q", l)
		}
	}
}

// TestPrintDoesNotMutateArgs: Node's debug() splices its own args array; the
// Go port must not change the caller's slice.
func TestPrintDoesNotMutateArgs(t *testing.T) {
	Enable("*")
	d := New("args")
	d.Log = func(string) {}
	args := []any{"%s and %O", "a", 1}
	d.Print(args...)
	if args[0] != "%s and %O" || args[1] != "a" || args[2] != 1 {
		t.Errorf("args changed: %v", args)
	}
}
