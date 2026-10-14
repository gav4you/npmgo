// Package debug is a Go translation of the npm package debug@4.4.3 (Node
// build, https://github.com/debug-js/debug). The original copyright notice
// and MIT license are preserved in LICENSE.
//
// Not ported: the supports-color 256-color palette (an optional dependency
// Postiz does not install) and the deprecated destroy() stub.
//
// Like the JavaScript module, the package keeps process-wide state: the
// enabled namespaces (Enable, Disable, the DEBUG environment variable), the
// DEBUG_* options (InspectOpts), the Formatters table and the Log and Now
// hooks. Enable, Disable, Enabled and the methods of a Debugger are safe for
// concurrent use; set Formatters, InspectOpts, Log and Now before logging from
// several goroutines, as there is no lock around them.
package debug

import (
	"fmt"
	"math"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf16"

	"golang.org/x/term"

	"github.com/gav4you/npmgo/internal/jsnum"
	"github.com/gav4you/npmgo/internal/nodeutil"
	"github.com/gav4you/npmgo/pkgs/ms"
)

// Colors is the ANSI palette used by SelectColor (Node build, basic palette).
var Colors = []int{6, 2, 3, 4, 5, 1}

// Formatter handles a custom %X placeholder, like createDebug.formatters[X]
// called with the Debugger as `this`. A Formatter runs while the Debugger is
// locked, so it must not log through the same Debugger.
type Formatter func(d *Debugger, v any) string

var (
	mu         sync.Mutex
	names      []string
	skips      []string
	generation int

	// Formatters maps a placeholder letter to its handler. %o and %O are
	// preset: they set the instance's InspectOpts["colors"] to its UseColors
	// and inspect with the instance's options, as the originals do.
	Formatters = map[byte]Formatter{
		'o': func(d *Debugger, v any) string {
			d.InspectOpts["colors"] = d.UseColors
			lines := strings.Split(nodeutil.Inspect(v, InspectOptions(d.InspectOpts)), "\n")
			for i, l := range lines {
				lines[i] = jsnum.TrimSpace(l)
			}
			return strings.Join(lines, " ")
		},
		'O': func(d *Debugger, v any) string {
			d.InspectOpts["colors"] = d.UseColors
			return nodeutil.Inspect(v, InspectOptions(d.InspectOpts))
		},
	}

	// InspectOpts mirrors exports.inspectOpts: the DEBUG_* environment
	// variables, parsed by LoadInspectOpts. It is the options object of the
	// final util.formatWithOptions call and the source of hideDate; every New
	// Debugger takes a copy of it for its own %o / %O formatting.
	InspectOpts = LoadInspectOpts(os.Environ())

	// Log is the default sink (stderr), like exports.log.
	Log = func(line string) { fmt.Fprint(os.Stderr, line+"\n") }

	// Now is the clock; replaceable for deterministic tests.
	Now = time.Now
)

var (
	placeholder = regexp.MustCompile(`%([a-zA-Z%])`)
	envKey      = regexp.MustCompile(`(?i)^debug_`)
	camelRe     = regexp.MustCompile(`_([a-z])`)
	yesRe       = regexp.MustCompile(`(?i)^(yes|on|true|enabled)$`)
	noRe        = regexp.MustCompile(`(?i)^(no|off|false|disabled)$`)
)

// Debugger is a namespaced logger, the equivalent of the function debug() returns.
type Debugger struct {
	Namespace string
	UseColors bool
	Color     int
	// Log, when set, replaces the Log sink for this instance (debug.log = fn).
	Log func(line string)
	// InspectOpts is this instance's copy of InspectOpts, taken when it was
	// created (createDebug.init); %o and %O format with it.
	InspectOpts map[string]any
	// Diff, Prev and Curr are the timing of the last Print, in milliseconds
	// since the epoch (debug.diff, .prev, .curr); Print writes them.
	Diff, Prev, Curr float64

	mu          sync.Mutex
	prevTime    float64
	override    *bool
	cacheGen    int
	cacheResult bool
}

func init() {
	v, _ := os.LookupEnv("DEBUG")
	Enable(v)
}

// LoadInspectOpts ports the DEBUG_* environment parsing into inspectOpts:
// yes/on/true/enabled and no/off/false/disabled become booleans, null nil,
// anything else Number(value).
func LoadInspectOpts(env []string) map[string]any {
	opts := map[string]any{}
	for _, kv := range env {
		k, val, _ := strings.Cut(kv, "=")
		if !envKey.MatchString(k) {
			continue
		}
		prop := camelRe.ReplaceAllStringFunc(strings.ToLower(k[6:]), func(s string) string {
			return strings.ToUpper(s[1:])
		})
		switch {
		case yesRe.MatchString(val):
			opts[prop] = true
		case noRe.MatchString(val):
			opts[prop] = false
		case val == "null":
			opts[prop] = nil
		default:
			opts[prop] = jsnum.Number(val)
		}
	}
	return opts
}

// truthy ports ToBoolean for the values an options map can hold.
func truthy(v any) bool {
	switch x := v.(type) {
	case nil, nodeutil.Undefined:
		return false
	case bool:
		return x
	case float64:
		return x != 0 && !math.IsNaN(x)
	case int:
		return x != 0
	case string:
		return x != ""
	}
	return true
}

// InspectOptions converts an inspectOpts map (see LoadInspectOpts) into the
// util.inspect options Node derives from it: numbers are used as they are,
// booleans count as 0 / 1 where a number is compared, null means Infinity for
// depth, maxArrayLength and maxStringLength but 0 for breakLength, null or NaN
// compact disables the compact layout, and getters are only read for `true`.
func InspectOptions(m map[string]any) nodeutil.Options {
	o := nodeutil.DefaultOptions()
	num := func(v any, null float64) (float64, bool) {
		switch x := v.(type) {
		case float64:
			return x, true
		case bool:
			if x {
				return 1, true
			}
			return 0, true
		case nil:
			return null, true
		}
		return 0, false
	}
	for k, v := range m {
		switch k {
		case "colors":
			o.Colors = truthy(v)
		case "showHidden":
			o.ShowHidden = truthy(v)
		case "depth":
			if n, ok := num(v, math.Inf(1)); ok {
				o.Depth = n
			}
		case "breakLength":
			if n, ok := num(v, 0); ok {
				o.BreakLength = n
			}
		case "maxArrayLength":
			if n, ok := num(v, math.Inf(1)); ok {
				o.MaxArrayLength = n
			}
		case "maxStringLength":
			if n, ok := num(v, math.Inf(1)); ok {
				o.MaxStringLength = n
			}
		case "compact":
			switch x := v.(type) {
			case bool:
				o.CompactTrue, o.CompactFalse = x, !x
			case float64:
				if math.IsNaN(x) {
					o.CompactTrue, o.CompactFalse = false, true
				} else {
					o.Compact = int(x)
				}
			case nil:
				o.CompactTrue, o.CompactFalse = false, true
			}
		case "sorted":
			o.Sorted = truthy(v)
		case "getters":
			o.Getters = v == true
		case "numericSeparator":
			o.NumericSeparator = truthy(v)
		case "showProxy":
			o.ShowProxy = truthy(v)
		case "customInspect":
			o.CustomInspect = truthy(v)
		}
	}
	return o
}

// UseColors ports useColors(): DEBUG_COLORS wins, else whether stderr is a TTY.
func UseColors() bool {
	if v, ok := InspectOpts["colors"]; ok {
		return truthy(v)
	}
	return term.IsTerminal(int(os.Stderr.Fd()))
}

// SelectColor hashes a namespace over UTF-16 code units with int32 wraparound.
func SelectColor(namespace string) int {
	var hash int32
	for _, c := range utf16.Encode([]rune(namespace)) {
		hash = (hash << 5) - hash + int32(c)
	}
	h := math.Abs(float64(hash))
	return Colors[int(math.Mod(h, float64(len(Colors))))]
}

// New creates a Debugger, the equivalent of require('debug')(namespace). It
// reads UseColors and copies InspectOpts at this moment.
func New(namespace string) *Debugger {
	opts := make(map[string]any, len(InspectOpts))
	for k, v := range InspectOpts {
		opts[k] = v
	}
	return &Debugger{
		Namespace:   namespace,
		UseColors:   UseColors(),
		Color:       SelectColor(namespace),
		InspectOpts: opts,
		cacheGen:    -1,
	}
}

// Extend ports debug.extend(namespace, delimiter). Pass nil for the default ":".
func (d *Debugger) Extend(namespace string, delimiter *string) *Debugger {
	sep := ":"
	if delimiter != nil {
		sep = *delimiter
	}
	n := New(d.Namespace + sep + namespace)
	n.Log = d.Log
	return n
}

// Enabled ports the `enabled` getter.
func (d *Debugger) Enabled() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.override != nil {
		return *d.override
	}
	mu.Lock()
	defer mu.Unlock()
	if d.cacheGen != generation {
		d.cacheGen = generation
		d.cacheResult = enabledLocked(d.Namespace)
	}
	return d.cacheResult
}

// SetEnabled ports the `enabled` setter.
func (d *Debugger) SetEnabled(v bool) {
	d.mu.Lock()
	d.override = &v
	d.mu.Unlock()
}

// Coerce ports createDebug.coerce: an error becomes `stack || message`, so a
// non-string truthy stack is returned as it is (and then logged with %O).
func Coerce(v any) any {
	if e, ok := v.(*nodeutil.Error); ok {
		if truthy(e.Stack) {
			return e.Stack
		}
		return e.Message
	}
	if e, ok := v.(error); ok {
		return e.Error()
	}
	return v
}

// Print ports calling debug(...args).
func (d *Debugger) Print(args ...any) {
	if !d.Enabled() {
		return
	}
	d.mu.Lock()
	if d.InspectOpts == nil { // a zero-value Debugger rather than one from New
		d.InspectOpts = map[string]any{}
	}
	curr := float64(Now().UnixMilli())
	prev := d.prevTime
	if prev == 0 {
		prev = curr
	}
	d.Diff = curr - prev
	d.Prev = d.prevTime
	d.Curr = curr
	d.prevTime = curr

	args = append([]any(nil), args...)
	if len(args) == 0 {
		args = []any{nodeutil.Undefined{}}
	}
	args[0] = Coerce(args[0])
	f, ok := args[0].(string)
	if !ok {
		args = append([]any{"%O"}, args...)
		f = "%O"
	}
	index := 0
	f = placeholder.ReplaceAllStringFunc(f, func(match string) string {
		if match == "%%" {
			return "%"
		}
		index++
		if fn, ok := Formatters[match[1]]; ok {
			var val any = nodeutil.Undefined{}
			if index < len(args) {
				val = args[index]
				args = append(args[:index], args[index+1:]...)
			}
			index--
			return fn(d, val)
		}
		return match
	})
	args[0] = f
	args = d.formatArgs(args)
	line := nodeutil.FormatWithOptions(InspectOptions(InspectOpts), args...)
	logFn := d.Log
	d.mu.Unlock()
	if logFn == nil {
		logFn = Log
	}
	logFn(line)
}

func (d *Debugger) formatArgs(args []any) []any {
	s := args[0].(string)
	if d.UseColors {
		code := fmt.Sprintf("\x1b[3%d", d.Color)
		if d.Color >= 8 {
			code = fmt.Sprintf("\x1b[38;5;%d", d.Color)
		}
		prefix := "  " + code + ";1m" + d.Namespace + " \x1b[0m"
		args[0] = prefix + strings.Join(strings.Split(s, "\n"), "\n"+prefix)
		h, _ := ms.Format(d.Diff, false)
		return append(args, code+"m+"+h+"\x1b[0m")
	}
	args[0] = getDate() + d.Namespace + " " + s
	return args
}

func getDate() string {
	if truthy(InspectOpts["hideDate"]) {
		return ""
	}
	return Now().UTC().Format("2006-01-02T15:04:05.000Z") + " "
}

// Enable ports createDebug.enable(namespaces): it also stores the string in
// the DEBUG environment variable (createDebug.save), or removes the variable
// for "". Names are separated by commas or JavaScript whitespace.
func Enable(ns string) {
	mu.Lock()
	defer mu.Unlock()
	if ns != "" {
		os.Setenv("DEBUG", ns)
	} else {
		os.Unsetenv("DEBUG")
	}
	generation++
	names, skips = nil, nil
	fields := strings.FieldsFunc(ns, func(r rune) bool { return r == ',' || jsnum.IsWhiteSpace(r) })
	for _, f := range fields {
		if f[0] == '-' {
			skips = append(skips, f[1:])
		} else {
			names = append(names, f)
		}
	}
}

// Disable ports createDebug.disable(): returns the previous namespaces.
func Disable() string {
	mu.Lock()
	parts := append([]string{}, names...)
	for _, s := range skips {
		parts = append(parts, "-"+s)
	}
	mu.Unlock()
	Enable("")
	return strings.Join(parts, ",")
}

// Enabled ports createDebug.enabled(name).
func Enabled(name string) bool {
	mu.Lock()
	defer mu.Unlock()
	return enabledLocked(name)
}

func enabledLocked(name string) bool {
	for _, s := range skips {
		if matchesTemplate(name, s) {
			return false
		}
	}
	for _, n := range names {
		if matchesTemplate(name, n) {
			return true
		}
	}
	return false
}

// matchesTemplate ports matchesTemplate over UTF-16 code units, as the
// original indexes the strings.
func matchesTemplate(searchS, templateS string) bool {
	search := utf16.Encode([]rune(searchS))
	template := utf16.Encode([]rune(templateS))
	si, ti, star, mi := 0, 0, -1, 0
	for si < len(search) {
		if ti < len(template) && (template[ti] == search[si] || template[ti] == '*') {
			if template[ti] == '*' {
				star, mi = ti, si
				ti++
			} else {
				si++
				ti++
			}
		} else if star != -1 {
			ti = star + 1
			mi++
			si = mi
		} else {
			return false
		}
	}
	for ti < len(template) && template[ti] == '*' {
		ti++
	}
	return ti == len(template)
}
