package api

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// countingLogger is a goroutine-safe DiagLogger that counts its messages.
type countingLogger struct{ n atomic.Int64 }

func (l *countingLogger) Error(string, ...any)   { l.n.Add(1) }
func (l *countingLogger) Warn(string, ...any)    { l.n.Add(1) }
func (l *countingLogger) Info(string, ...any)    { l.n.Add(1) }
func (l *countingLogger) Debug(string, ...any)   { l.n.Add(1) }
func (l *countingLogger) Verbose(string, ...any) { l.n.Add(1) }

// The registry, the diag API, the proxy tracer provider and its tracers, contexts, baggage and trace state are safe
// for concurrent use (run with -race).
func TestConcurrentUse(t *testing.T) {
	reset()
	defer reset()
	var wg sync.WaitGroup
	logger := &countingLogger{}
	sc := SpanContext{TraceId: tid, SpanId: sid, TraceFlags: 1}
	ctx := Trace.SetSpanContext(ROOT_CONTEXT, sc)
	baggage := CreateBaggage(BaggageEntryPair{Key: "k", Entry: BaggageEntry{Value: "v"}})
	ctx = Propagation.SetBaggage(ctx, baggage)
	ts := CreateTraceState("a=1,b=2")
	pp := NewProxyTracerProvider()
	tracer := pp.GetTracer("t", "", nil)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				switch j % 5 {
				case 0:
					Diag.SetLogger(logger, DiagLoggerOptions{LogLevel: level(DiagLogLevel_ALL), SuppressOverrideMessage: true})
				case 1:
					Diag.Warn("warning", j)
				case 2:
					pp.SetDelegate(noopTracerProvider{})
				case 3:
					Trace.SetGlobalTracerProvider(pp)
					Trace.GetTracer("x", "")
				case 4:
					Metrics.GetMeter("m", "", nil)
					Propagation.Fields()
				}
				if s := tracer.StartSpan("s", nil, ctx); s.SpanContext().TraceId != tid {
					t.Errorf("span %v", s.SpanContext())
				}
				if b := Propagation.GetBaggage(ctx.SetValue(CreateContextKey(fmt.Sprint(i)), j)); b != baggage {
					t.Error("baggage lost")
				}
				if v, _ := ts.Set("c", "3").Get("a"); v != "1" {
					t.Error("trace state")
				}
			}
		}(i)
	}
	wg.Wait()
	if logger.n.Load() == 0 {
		t.Error("no message reached the logger")
	}
}

// A value of another type stored under the span or baggage key reads as none (JavaScript returns the raw value, so
// that span.spanContext() or baggage.getAllEntries() throws later).
func TestForeignContextValues(t *testing.T) {
	ctx := ROOT_CONTEXT.SetValue(CreateContextKey("OpenTelemetry Context Key SPAN"), "not a span")
	ctx = ctx.SetValue(CreateContextKey("OpenTelemetry Baggage Key"), 42)
	if Trace.GetSpan(ctx) != nil || Trace.GetSpanContext(ctx) != nil || Propagation.GetBaggage(ctx) != nil {
		t.Error("a foreign value was taken for a span or baggage")
	}
}

// The default Console writes Node's console format: util.format(...args) and a newline.
func TestConsoleFormat(t *testing.T) {
	f, err := os.Create(filepath.Join(t.TempDir(), "out"))
	if err != nil {
		t.Fatal(err)
	}
	writeConsole(f, "Trace: ", []any{"message %s", "x", 1.0})
	writeConsole(f, "", []any{"plain", true})
	f.Close()
	b, _ := os.ReadFile(f.Name())
	if got, want := string(b), "Trace: message x 1\nplain true\n"; got != want {
		t.Errorf("console output %q, want %q", got, want)
	}
}

// DiagConsoleLogger falls back to console.log for a missing method, and writes nothing without a console.
func TestDiagConsoleLoggerFallsBackToLog(t *testing.T) {
	saved := Console
	defer func() { Console = saved }()
	var lines []string
	Console = &ConsoleMethods{Log: func(args ...any) { lines = append(lines, fmt.Sprint(args...)) }}
	l := NewDiagConsoleLogger()
	l.Error("e")
	l.Verbose("v", 1)
	Console = nil
	l.Info("dropped")
	if got := strings.Join(lines, "|"); got != "e|v1" {
		t.Errorf("lines %q", got)
	}
}
