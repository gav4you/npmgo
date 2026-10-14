package server_test

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gav4you/npmgo/pkgs/react"
	reactdom "github.com/gav4you/npmgo/pkgs/react-dom"
	"github.com/gav4you/npmgo/pkgs/react-dom/server"
)

var h = react.CreateElement

func mustRender(t *testing.T, node react.Node) string {
	t.Helper()
	html, err := server.RenderToString(node)
	if err != nil {
		t.Fatal(err)
	}
	return html
}

// The Go function-component shapes listed in the react package doc.
func TestComponentSignatures(t *testing.T) {
	withHooks := func(hk react.Hooks, p *react.Props) react.Node {
		n, _ := hk.UseState(2)
		return fmt.Sprintf("hooks:%v%v", n, p.Get("x"))
	}
	propsOnly := func(p *react.Props) react.Node { return h("b", nil, p.Get("x")) }
	noArgs := func() react.Node { return "none" }
	withErr := func(hk react.Hooks, p *react.Props) (react.Node, error) { return "ok", nil }
	named := &react.FunctionComponent{Name: "Named", Render: propsOnly}
	got := mustRender(t, h("div", nil,
		h(withHooks, react.Obj("x", 1)), h(propsOnly, react.Obj("x", "p")), h(noArgs, nil), h(withErr, nil), h(named, react.Obj("x", "n"))))
	want := `<div>hooks:21<b>p</b>none<!-- -->ok<b>n</b></div>`
	if got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}

var errBoom = errors.New("boom")

func TestErrorsAreReturned(t *testing.T) {
	failing := func(hk react.Hooks, p *react.Props) (react.Node, error) { return nil, errBoom }
	if _, err := server.RenderToString(h("div", nil, h(failing, nil))); !errors.Is(err, errBoom) {
		t.Fatalf("returned error: %v", err)
	}
	panicking := func() react.Node { panic("raw panic") }
	_, err := server.RenderToString(h(panicking, nil))
	var p *react.Panic
	if !errors.As(err, &p) || p.Value != "raw panic" {
		t.Fatalf("panic value: %#v", err)
	}
	// Inside a Suspense boundary the error client-renders the boundary.
	html, err := server.RenderToString(h(react.Suspense, react.Obj("fallback", "fb"), h(failing, nil)))
	if err != nil || html != "<!--$!--><template></template>fb<!--/$-->" {
		t.Fatalf("boundary: %q %v", html, err)
	}
}

type flushWriter struct {
	bytes.Buffer
	flushes int
}

func (w *flushWriter) Flush() { w.flushes++ }

func TestPipeStreamsPromiseSettledOnAnotherGoroutine(t *testing.T) {
	p, resolve, _ := react.NewPromise()
	data := func(hk react.Hooks, _ *react.Props) react.Node { return h("p", nil, hk.Use(p)) }
	var events []string
	s := server.RenderToPipeableStream(h("main", nil, h(react.Suspense, react.Obj("fallback", "loading"), h(data, nil))), &server.StreamOptions{
		OnShellReady: func() { events = append(events, "shell") },
		OnAllReady:   func() { events = append(events, "all") },
	})
	go func() {
		time.Sleep(5 * time.Millisecond)
		resolve("loaded")
	}()
	var w flushWriter
	if err := s.Pipe(&w); err != nil {
		t.Fatal(err)
	}
	out := w.String()
	if !strings.HasPrefix(out, `<main><!--$?--><template id="B:0"></template>loading<!--/$--></main>`) ||
		!strings.Contains(out, `<div hidden id="S:0"><p>loaded</p></div>`) || !strings.HasSuffix(out, `$RC("B:0","S:0")</script>`) {
		t.Fatalf("unexpected stream:\n%s", out)
	}
	if fmt.Sprint(events) != "[shell all]" || w.flushes < 2 {
		t.Fatalf("events %v flushes %d", events, w.flushes)
	}
}

func TestPipeFromCallbacks(t *testing.T) {
	tree := h("div", nil, "a", "b")
	for _, when := range []string{"shell", "all"} {
		var buf bytes.Buffer
		var s *server.PipeableStream
		pipe := func() {
			if err := s.Pipe(&buf); err != nil {
				t.Error(err)
			}
		}
		opts := &server.StreamOptions{}
		if when == "shell" {
			opts.OnShellReady = pipe
		} else {
			opts.OnAllReady = pipe
		}
		s = server.RenderToPipeableStream(tree, opts)
		if err := s.Wait(); err != nil {
			t.Fatal(err)
		}
		if buf.String() != "<div>a<!-- -->b</div>" {
			t.Fatalf("%s: %q", when, buf.String())
		}
	}
}

func TestAbortFromAnotherGoroutine(t *testing.T) {
	never, _, _ := react.NewPromise()
	waits := func(hk react.Hooks, _ *react.Props) react.Node { return hk.Use(never) }
	var onErr []string
	s := server.RenderToPipeableStream(h("div", nil, h(react.Suspense, react.Obj("fallback", "fb"), h(waits, nil))), &server.StreamOptions{
		OnError: func(err error, _ server.ErrorInfo) string { onErr = append(onErr, err.Error()); return "dgst" },
	})
	time.AfterFunc(5*time.Millisecond, func() { s.Abort(errors.New("timed out")) })
	var buf bytes.Buffer
	if err := s.Pipe(&buf); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `$RX("B:0","dgst")`) || fmt.Sprint(onErr) != "[timed out]" {
		t.Fatalf("stream %q errors %v", buf.String(), onErr)
	}
}

func TestShellErrorEndsPipe(t *testing.T) {
	failing := func() react.Node { panic(errBoom) }
	var shellErr error
	s := server.RenderToPipeableStream(h(failing, nil), &server.StreamOptions{
		OnShellError: func(err error) { shellErr = err },
		OnError:      func(error, server.ErrorInfo) string { return "" },
	})
	var buf bytes.Buffer
	if err := s.Pipe(&buf); !errors.Is(err, errBoom) || !errors.Is(shellErr, errBoom) || buf.Len() != 0 {
		t.Fatalf("pipe %v shell %v out %q", err, shellErr, buf.String())
	}
}

type failingWriter struct{}

var errWrite = errors.New("disk full")

func (failingWriter) Write([]byte) (int, error) { return 0, errWrite }

func TestWriteErrorIsReturned(t *testing.T) {
	s := server.RenderToPipeableStream(h("div", nil, "x"), &server.StreamOptions{OnError: func(error, server.ErrorInfo) string { return "" }})
	if err := s.Pipe(failingWriter{}); !errors.Is(err, errWrite) {
		t.Fatalf("got %v", err)
	}
}

// Renders on many goroutines at once share contexts, a lazy component and a
// cached promise; run with -race.
func TestConcurrentRenders(t *testing.T) {
	theme := react.CreateContext("light")
	shared := react.ResolvedPromise("shared")
	shared.SetThenableStatus("fulfilled", "shared", nil)
	inner := &react.FunctionComponent{Name: "Inner", Render: func(hk react.Hooks, p *react.Props) react.Node {
		return h("span", react.Obj("id", hk.UseId()), hk.UseContext(theme), "/", hk.Use(shared), "/", p.Get("n"))
	}}
	lazy := react.Lazy(func() react.Thenable {
		p, resolve, _ := react.NewPromise()
		go resolve(inner)
		return p
	})
	app := func(n int) react.Node {
		return h(theme, react.Obj("value", fmt.Sprint("t", n)),
			h(react.Suspense, react.Obj("fallback", "…"), h(lazy, react.Obj("n", n))))
	}
	var wg sync.WaitGroup
	results := make([]string, 40)
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			var buf bytes.Buffer
			s := server.RenderToPipeableStream(app(i), &server.StreamOptions{OnShellReady: func() {}})
			if i%2 == 0 {
				_ = s.Pipe(&buf)
			} else {
				html, _ := server.RenderToString(app(i))
				buf.WriteString(html)
			}
			results[i] = buf.String()
		}(i)
	}
	wg.Wait()
	for i, r := range results {
		want := fmt.Sprintf(`<span id="_R_0_">t%d<!-- -->/<!-- -->shared<!-- -->/<!-- -->%d</span>`, i, i)
		if !strings.Contains(r, want) && !strings.Contains(r, "<!--$!--><template></template>…<!--/$-->") {
			t.Errorf("render %d: %s", i, r)
		}
	}
}

type counter struct {
	react.Component
}

func (c *counter) ComponentWillMount() {
	c.SetState(react.Obj("count", 1))
	c.SetState(func(prev any, _ *react.Props) any {
		n := prev.(*react.Object).Get("count").(int)
		return react.Obj("count", n+1)
	})
}

func (c *counter) Render() react.Node {
	return h("b", nil, c.Props.Get("label"), ":", c.State.(*react.Object).Get("count"))
}

func TestGoClassComponent(t *testing.T) {
	cls := &react.Class{Name: "Counter", New: func(props *react.Props, _ any) react.ComponentInstance {
		c := &counter{Component: react.NewComponent(props, nil)}
		c.State = react.Obj("count", 0)
		return c
	}}
	if got := mustRender(t, h(cls, react.Obj("label", "n"))); got != "<b>n<!-- -->:<!-- -->2</b>" {
		t.Fatalf("got %s", got)
	}
}

func TestResourceHintsNeedTheRendersHooks(t *testing.T) {
	reactdom.Preload(nil, "/x.woff", react.Obj("as", "font")) // outside a render: no effect, no panic
	comp := func(hk react.Hooks, _ *react.Props) react.Node {
		reactdom.Preload(hk, "/font.woff2", react.Obj("as", "font"))
		return "x"
	}
	want := `<link rel="preload" href="/font.woff2" as="font" crossorigin=""/><p>x</p>`
	if got := mustRender(t, h("p", nil, h(comp, nil))); got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}

func TestPromiseTrackedAcrossRenders(t *testing.T) {
	p := react.ResolvedPromise("ready")
	comp := func(hk react.Hooks, _ *react.Props) react.Node { return hk.Use(p) }
	tree := h(react.Suspense, react.Obj("fallback", "fb"), h(comp, nil))
	first, second := mustRender(t, tree), mustRender(t, tree)
	if first != "<!--$!--><template></template>fb<!--/$-->" || second != "<!--$-->ready<!--/$-->" {
		t.Fatalf("first %s second %s", first, second)
	}
}
