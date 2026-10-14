package chalk

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/gav4you/npmgo/internal/jsvalue"
	supportscolor "github.com/gav4you/npmgo/pkgs/supports-color"
)

func panicText(f func()) (s string) {
	defer func() { s = fmt.Sprint(recover()) }()
	f()
	return ""
}

func TestInstances(t *testing.T) {
	c := New(Options{Level: 2})
	if c.Level() != 2 || c.Red().Bold().Level() != 2 {
		t.Errorf("level %d", c.Level())
	}
	if got := New().Level(); got != Default.Level() {
		t.Errorf("New() level %d, Default %d", got, Default.Level())
	}
	if got := panicText(func() { New(Options{Level: 4}) }); got != levelError {
		t.Errorf("New(level 4) panicked with %q", got)
	}
	if got := panicText(func() { New(Options{Level: -1}) }); got != levelError {
		t.Errorf("New(level -1) panicked with %q", got)
	}
	m := Load(&supportscolor.Level{Level: 3, HasBasic: true, Has256: true, Has16m: true}, nil)
	if m.Level() != 3 || m.SupportsColor.Level != 3 || m.Stderr.Level() != 0 || m.Stderr.SupportsColor != nil {
		t.Errorf("Load: %d %v %d %v", m.Level(), m.SupportsColor, m.Stderr.Level(), m.Stderr.SupportsColor)
	}
	if m.Instance().Level() != 3 || m.Stderr.Instance().Level() != 3 || m.Red().Stderr != nil {
		t.Error("Instance() should take the module's stdout level; builders have no Stderr")
	}
	if got := panicText(func() { Load(nil, &supportscolor.Level{Level: -1}) }); got != levelError {
		t.Errorf("Load with stderr level -1 panicked with %q", got)
	}
}

func TestBuilders(t *testing.T) {
	c := New(Options{Level: 1})
	red := c.Red()
	_ = red.Bold() // does not change red
	if got := red.Sprint("x"); got != "\x1b[31mx\x1b[39m" {
		t.Errorf("red: %q", got)
	}
	if got := c.Sprint("a", 1, nil, true); got != "a 1  true" {
		t.Errorf("instance call: %q", got)
	}
	if got := red.Sprint(nil); got != "\x1b[31mnull\x1b[39m" {
		t.Errorf("red(null): %q", got)
	}
	red.SetLevel(0) // sets the instance's level
	if c.Level() != 0 || red.Sprint("x") != "x" || c.Visible().Sprint("x") != "" {
		t.Errorf("after SetLevel(0) on a builder: level %d", c.Level())
	}
	c.SetLevel(7)
	if got := panicText(func() { c.RGB(1, 2, 3) }); got != "Cannot read properties of undefined (reading 'rgb')" {
		t.Errorf("rgb at level 7 panicked with %q", got)
	}
	if got := red.Sprint("x"); got != "\x1b[31mx\x1b[39m" {
		t.Errorf("red at level 7: %q", got)
	}
	c.SetLevel(3)
	var typeErr *jsvalue.TypeError
	func() {
		defer func() { typeErr, _ = recover().(*jsvalue.TypeError) }()
		c.Keyword("nope")
	}()
	if typeErr == nil || typeErr.Message != "Cannot read properties of undefined (reading '0')" {
		t.Errorf("keyword('nope'): %v", typeErr)
	}
}

func TestTemplateAPI(t *testing.T) {
	c := New(Options{Level: 1})
	s, err := c.Tag([]string{"{bold ", "} is {red ", "}"}, "chalk", "{not a style}")
	if err != nil || s != "\x1b[1mchalk\x1b[22m is \x1b[31m{not a style}\x1b[39m" {
		t.Errorf("Tag: %q, %v", s, err)
	}
	if _, err := c.Template("{red oops"); err == nil || err.Error() != "Chalk template literal is missing 1 closing bracket (`}`)" {
		t.Errorf("Template error: %v", err)
	}
	if _, err := c.Template(`\u{110000}`); err == nil || err.Error() != "Invalid code point 1114112" {
		t.Errorf("RangeError: %v", err)
	} else if _, ok := err.(*jsvalue.RangeError); !ok {
		t.Errorf("RangeError type: %T", err)
	}
	if _, err := c.Template("{rgb(1) x}"); err == nil || !strings.Contains(err.Error(), "does not support") {
		t.Errorf("unsupported model arguments: %v", err)
	}
	// The template styles from red, then red wraps it again and reopens after the inner close.
	if s, err := c.Red().Template("{bold x}"); err != nil || s != "\x1b[31m\x1b[31m\x1b[1mx\x1b[22m\x1b[39m\x1b[31m\x1b[39m" {
		t.Errorf("builder template: %q, %v", s, err)
	}
	if s, _ := c.Template(`😀 \uD83D`); s != "😀 �" {
		t.Errorf("surrogates: %q", s)
	}
}

// TestConcurrent styles from several goroutines while the level changes
// (run with -race).
func TestConcurrent(t *testing.T) {
	c := New(Options{Level: 3})
	b := c.Red().Bold()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				_ = b.Sprint("x", i, j)
				_ = c.RGB(float64(i), float64(j), 0).Sprint("y")
				if _, err := c.Template("{blue z}"); err != nil {
					t.Error(err)
				}
				if j%50 == 0 {
					c.SetLevel(1 + j%3)
				}
			}
		}(i)
	}
	wg.Wait()
}

func ExampleNew() {
	c := New(Options{Level: 1})
	fmt.Printf("%q\n", c.Red().Bold().Sprint("error:", 42))
	fmt.Printf("%q\n", c.BgHex("#0000FF").Sprint("blue background"))
	// Output:
	// "\x1b[31m\x1b[1merror: 42\x1b[22m\x1b[39m"
	// "\x1b[104mblue background\x1b[49m"
}

func ExampleChalk_Template() {
	c := New(Options{Level: 2})
	s, err := c.Template("{bold.underline CPU:} {rgb(255,131,0) 87%}")
	fmt.Printf("%q %v\n", s, err)
	// Output:
	// "\x1b[1m\x1b[4mCPU:\x1b[24m\x1b[22m \x1b[38;5;214m87%\x1b[39m" <nil>
}
