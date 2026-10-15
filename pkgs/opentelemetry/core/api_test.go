package core

import (
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gav4you/npmgo/internal/jsvalue"
	"github.com/gav4you/npmgo/pkgs/opentelemetry/api"
)

// A TraceState is parsed on first use and is safe for concurrent use; so are the global error handler and the
// propagators (run with -race).
func TestConcurrentUse(t *testing.T) {
	reset()
	defer reset()
	var wg sync.WaitGroup
	ts := NewTraceState("a=1,b=2,c=3")
	var handled atomic.Int64
	ctx := api.Trace.SetSpanContext(api.ROOT_CONTEXT, api.SpanContext{TraceId: tid, SpanId: sid, TraceFlags: 1, TraceState: ts})
	ctx = api.Propagation.SetBaggage(ctx, api.CreateBaggage(api.BaggageEntryPair{Key: "k", Entry: api.BaggageEntry{Value: "v"}}))
	comp := NewCompositePropagator(CompositePropagatorConfig{Propagators: []api.TextMapPropagator{NewW3CTraceContextPropagator(), NewW3CBaggagePropagator()}})
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				if v, ok := ts.Get("b"); !ok || v != "2" {
					t.Error("trace state get")
				}
				if s := ts.Set("d", "4").Serialize(); s != "d=4,a=1,b=2,c=3" {
					t.Errorf("serialize %q", s)
				}
				SetGlobalErrorHandler(func(any) { handled.Add(1) })
				GlobalErrorHandler("x")
				carrier := map[string]string{}
				comp.Inject(ctx, carrier, api.DefaultTextMapSetter)
				back := comp.Extract(api.ROOT_CONTEXT, carrier, api.DefaultTextMapGetter)
				if sc := api.Trace.GetSpanContext(back); sc == nil || sc.TraceId != tid {
					t.Error("round trip")
				}
			}
		}()
	}
	wg.Wait()
	if handled.Load() != 8*200 {
		t.Errorf("%d exceptions handled", handled.Load())
	}
}

// The real Otperformance measures from package initialisation on the monotonic clock.
func TestRealPerformance(t *testing.T) {
	p := realPerformance{origin: time.Now()}
	if d := math.Abs(p.TimeOrigin() - float64(time.Now().UnixNano())/1e6); d > 1000 {
		t.Errorf("time origin %v is %v ms away", p.TimeOrigin(), d)
	}
	a := p.Now()
	time.Sleep(2 * time.Millisecond)
	if b := p.Now(); !(a >= 0 && b > a) {
		t.Errorf("now %v then %v", a, b)
	}
	if h := HrTime(); h[0] < 1.7e9 {
		t.Errorf("hrTime %v", h)
	}
}

// ProcessEnv reads the real environment by default.
func TestProcessEnv(t *testing.T) {
	t.Setenv("NPMGO_OTEL_TEST", " 12 ")
	if v, ok := GetNumberFromEnv("NPMGO_OTEL_TEST"); !ok || v != 12 {
		t.Errorf("GetNumberFromEnv = %v, %v", v, ok)
	}
	if v, ok := GetStringFromEnv("NPMGO_OTEL_TEST"); !ok || v != " 12 " {
		t.Errorf("GetStringFromEnv = %q, %v", v, ok)
	}
}

// RPC metadata of another type stored under the key reads as none (JavaScript returns the raw value).
func TestForeignRPCMetadata(t *testing.T) {
	ctx := api.ROOT_CONTEXT.SetValue(api.CreateContextKey("OpenTelemetry SDK Context Key RPC_METADATA"), "raw")
	if GetRPCMetadata(ctx) != nil {
		t.Error("a string was taken for RPC metadata")
	}
}

// timerDelay is Node's setTimeout delay: 1 outside 1..2^31-1, fractions rounded up (not covered by the recording,
// whose virtual clock takes whole milliseconds only).
func TestTimerDelay(t *testing.T) {
	for _, c := range []struct {
		in   float64
		want int64
	}{{0, 1}, {-5, 1}, {math.NaN(), 1}, {1, 1}, {1.5, 2}, {10, 10}, {1<<31 - 1, 1<<31 - 1}, {1 << 31, 1}, {math.Inf(1), 1}} {
		if got := timerDelay(c.in); got != c.want {
			t.Errorf("timerDelay(%v) = %d, want %d", c.in, got, c.want)
		}
	}
}

// Merge keeps the right-hand value's type for Go maps and structs, which are objects that are not plain.
func TestMergeNonPlainObjects(t *testing.T) {
	m := map[string]any{"a": 1}
	if got := Merge(jsvalue.NewObject("x", 1.0), m); !jsvalue.ObjectIs(got, m) {
		t.Errorf("Merge returned %v", got)
	}
}
