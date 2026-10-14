package nodeutil_test

import (
	"math"
	"sync"
	"testing"

	. "github.com/gav4you/npmgo/internal/nodeutil"
)

// Go-side behaviour that the Node parity cases cannot express.

// TestUnmodelledGoValues: values outside the JS model print with fmt and must
// not panic, including uncomparable types (the circular check compares
// interfaces).
func TestUnmodelledGoValues(t *testing.T) {
	for _, v := range []any{[]int{1, 2}, map[string]int{"a": 1}, struct{ A int }{1}, func() {}, [2]int{1, 2}, make(chan int)} {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Inspect(%T) panicked: %v", v, r)
				}
			}()
			_ = Inspect(v)
			_ = Format("%s %O %j %d", v, v, v, v)
		}()
	}
	// A custom inspector returning a slice is inspected, not compared.
	if got := Inspect(sliceReturner{}); got != "[1 2]" {
		t.Errorf("got %q", got)
	}
}

type sliceReturner struct{}

func (sliceReturner) InspectCustom(float64, CustomOptions, func(any, Options) string) any {
	return []int{1, 2}
}

// TestOptionExtremes: non-finite option values take Node's comparison
// semantics instead of Go's int conversion.
func TestOptionExtremes(t *testing.T) {
	long := &Object{Props: []Prop{{Key: "alpha", Value: "aaaaaaaaaaaaaaaaaaaa"}, {Key: "beta", Value: "bbbbbbbbbbbbbbbbbbbbbbbb"},
		{Key: "gamma", Value: "cccccccccccccccccccccccc"}, {Key: "delta", Value: 1.0}}}
	for _, bl := range []float64{math.Inf(1), math.NaN()} {
		o := DefaultOptions()
		o.BreakLength = bl
		if got := Inspect(long, o); got != "{ alpha: 'aaaaaaaaaaaaaaaaaaaa', beta: 'bbbbbbbbbbbbbbbbbbbbbbbb', gamma: 'cccccccccccccccccccccccc', delta: 1 }" {
			t.Errorf("breakLength %v: %q", bl, got)
		}
	}
	o := DefaultOptions()
	o.MaxStringLength = -1
	if got := Inspect("abcdef", o); got != "'abcde'... 7 more characters" {
		t.Errorf("maxStringLength -1: %q", got)
	}
	o.MaxStringLength = math.Inf(-1)
	if got := Inspect("abcdef", o); got != "''... Infinity more characters" {
		t.Errorf("maxStringLength -Infinity: %q", got)
	}
}

// TestConcurrentInspect: Inspect and Format keep no shared mutable state.
func TestConcurrentInspect(t *testing.T) {
	v := &Object{Props: []Prop{{Key: "a", Value: &Array{Items: []any{1.0, "x", &Map{Keys: []any{"k"}, Values: []any{true}}}}}}}
	want := Inspect(v)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				if got := Inspect(v); got != want {
					t.Errorf("got %q", got)
				}
				_ = Format("%s %j", v, v)
			}
		}()
	}
	wg.Wait()
}
