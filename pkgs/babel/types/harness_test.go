package types

// The Go twin of parity/babel_spec.cjs (value specs and their encoding) and of the oracle's helpers.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gav4you/npmgo/internal/jsnum"
	"github.com/gav4you/npmgo/internal/jsvalue"
)

// ordered is a JSON object with its keys in order.
type ordered struct {
	keys []string
	vals []any
}

func (o *ordered) get(k string) (any, bool) {
	for i, key := range o.keys {
		if key == k {
			return o.vals[i], true
		}
	}
	return nil, false
}

// decodeOrdered decodes JSON keeping object key order: objects are *ordered, arrays []any, numbers json.Number.
func decodeOrdered(raw []byte) any {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	v, err := decodeValue(dec)
	if err != nil {
		panic(err)
	}
	return v
}

func decodeValue(dec *json.Decoder) (any, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch x := tok.(type) {
	case json.Delim:
		switch x {
		case '{':
			o := &ordered{}
			for dec.More() {
				kt, err := dec.Token()
				if err != nil {
					return nil, err
				}
				v, err := decodeValue(dec)
				if err != nil {
					return nil, err
				}
				o.keys = append(o.keys, kt.(string))
				o.vals = append(o.vals, v)
			}
			_, err := dec.Token()
			return o, err
		case '[':
			var a []any
			for dec.More() {
				v, err := decodeValue(dec)
				if err != nil {
					return nil, err
				}
				a = append(a, v)
			}
			_, err := dec.Token()
			return a, err
		}
	}
	return tok, nil
}

// instance is an object of a class (the {"$":"inst"} spec).
type instance struct{ ctor string }

// buildSpec builds a JavaScript value of the jsvalue model from a spec (see parity/babel_spec.cjs).
func buildSpec(spec any, refs map[string]any) any {
	switch s := spec.(type) {
	case nil, string, bool:
		return s
	case json.Number:
		f, err := strconv.ParseFloat(string(s), 64)
		if err != nil {
			panic(err)
		}
		return f
	case []any:
		a := &jsvalue.Array{}
		for _, it := range s {
			a.Push(buildSpec(it, refs))
		}
		return a
	case *ordered:
		kind, special := s.get("$")
		if !special {
			o := jsvalue.NewObject()
			for i, k := range s.keys {
				o.Set(k, buildSpec(s.vals[i], refs))
			}
			return o
		}
		str := func(k string) string {
			v, _ := s.get(k)
			x, _ := v.(string)
			return x
		}
		switch kind {
		case "undef":
			return jsvalue.Undefined{}
		case "num":
			switch str("v") {
			case "NaN":
				return math.NaN()
			case "Infinity":
				return math.Inf(1)
			case "-Infinity":
				return math.Inf(-1)
			case "-0":
				return math.Copysign(0, -1)
			}
		case "big":
			b, ok := new(big.Int).SetString(str("v"), 10)
			if !ok {
				panic("bad bigint " + str("v"))
			}
			return b
		case "sym":
			sym := jsvalue.NewSymbol(str("d"))
			if id := str("id"); id != "" {
				refs[id] = sym
			}
			return sym
		case "symfor":
			return jsvalue.SymbolFor(str("k"))
		case "ref":
			v, ok := refs[str("id")]
			if !ok {
				panic("unknown ref " + str("id"))
			}
			return v
		case "obj":
			o := jsvalue.NewObject()
			if id := str("id"); id != "" {
				refs[id] = o
			}
			if props, ok := s.get("props"); ok && props != nil {
				for _, p := range props.([]any) {
					pair := p.([]any)
					o.Set(pair[0].(string), buildSpec(pair[1], refs))
				}
			}
			if syms, ok := s.get("syms"); ok && syms != nil {
				for _, p := range syms.([]any) {
					pair := p.([]any)
					o.SetSymbol(buildSpec(pair[0], refs).(jsvalue.Symbol), buildSpec(pair[1], refs))
				}
			}
			return o
		case "arr":
			a := &jsvalue.Array{}
			if id := str("id"); id != "" {
				refs[id] = a
			}
			itemsV, _ := s.get("items")
			for _, it := range itemsV.([]any) {
				a.Push(buildSpec(it, refs))
			}
			return a
		case "re":
			return &jsvalue.RegExp{Source: str("source"), Flags: str("flags")}
		case "date":
			v, _ := s.get("ms")
			ms, _ := strconv.ParseInt(string(v.(json.Number)), 10, 64)
			d := time.UnixMilli(ms).UTC()
			return &d
		case "inst":
			return &instance{str("ctor")}
		case "map":
			return map[string]any{}
		case "fn":
			name := str("name")
			return &namedFunc{name}
		}
		panic(fmt.Sprintf("bad spec %v", s.keys))
	}
	panic(fmt.Sprintf("bad spec %T", spec))
}

// namedFunc is a JavaScript function value (the {"$":"fn"} spec).
type namedFunc struct{ name string }

// JSTypeOf reports "function".
func (*namedFunc) JSTypeOf() string { return "function" }

// buildArgs builds the arguments of a call, with one table of ids.
func buildArgs(specs []any) []any {
	refs := map[string]any{}
	out := make([]any, len(specs))
	for i, s := range specs {
		out[i] = buildSpec(s, refs)
	}
	return out
}

// identity is the key under which enc counts and labels an object.
func identity(v any) (any, bool) {
	switch x := v.(type) {
	case *jsvalue.Object, *jsvalue.Array, *Validator, *objectPrototypeValue, *namedFunc, *instance, *jsvalue.RegExp, *time.Time:
		return x, !jsvalue.IsNull(v)
	case builtin:
		return x, true
	case map[string]any:
		return reflect.ValueOf(x).Pointer(), true
	}
	if jsvalue.TypeOf(v) == "function" && !jsvalue.IsNull(v) {
		return reflect.ValueOf(v).Pointer(), true
	}
	return nil, false
}

// children lists what enc descends into, in order.
func children(v any) []any {
	switch x := v.(type) {
	case *jsvalue.Array:
		return x.Items()
	case *jsvalue.Object:
		var out []any
		for _, k := range x.Keys() {
			out = append(out, x.Get(k))
		}
		for _, s := range x.SymbolKeys() {
			out = append(out, x.GetSymbol(s))
		}
		return out
	case *Validator:
		var out []any
		for _, k := range x.props.Keys() {
			out = append(out, x.props.Get(k))
		}
		return out
	}
	return nil
}

func encSymbol(s jsvalue.Symbol) string {
	if key, ok := jsvalue.SymbolKeyFor(s); ok {
		return "Symbol.for(" + jsvalue.JSONQuote(key) + ")"
	}
	return "Symbol(" + s.Description() + ")"
}

func encNumber(x float64) string {
	if x == 0 && math.Signbit(x) {
		return "-0"
	}
	return jsnum.String(x)
}

func enc(v any) string { return encAll([]any{v})[0] }

// encAll is the oracle's encAll: several values with one set of labels.
func encAll(roots []any) []string {
	counts := map[any]int{}
	var visit func(v any)
	visit = func(v any) {
		id, ok := identity(v)
		if !ok {
			return
		}
		counts[id]++
		if counts[id] > 1 {
			return
		}
		for _, c := range children(v) {
			visit(c)
		}
	}
	for _, r := range roots {
		visit(r)
	}
	labels := map[any]int{}
	next := 0
	var out func(v any) string
	out = func(v any) string {
		switch x := v.(type) {
		case jsvalue.Undefined:
			return "undefined"
		case nil:
			return "null"
		case string:
			return jsvalue.JSONQuote(x)
		case bool:
			return strconv.FormatBool(x)
		case *big.Int:
			if x == nil {
				return "null"
			}
			return x.String() + "n"
		case jsvalue.Symbol:
			return encSymbol(x)
		}
		if jsvalue.IsNumber(v) {
			return encNumber(jsvalue.Float(v))
		}
		id, ok := identity(v)
		if !ok {
			if jsvalue.IsNull(v) {
				return "null"
			}
			return fmt.Sprintf("<unencodable %T>", v)
		}
		if n, seen := labels[id]; seen {
			return "#" + strconv.Itoa(n)
		}
		prefix := ""
		if counts[id] > 1 {
			labels[id] = next
			prefix = "#" + strconv.Itoa(next) + "="
			next++
		}
		switch x := v.(type) {
		case *jsvalue.Array:
			parts := make([]string, len(x.Items()))
			for i, it := range x.Items() {
				parts[i] = out(it)
			}
			return prefix + "[" + strings.Join(parts, ",") + "]"
		case *jsvalue.Object:
			var parts []string
			for _, k := range x.Keys() {
				parts = append(parts, jsvalue.JSONQuote(k)+":"+out(x.Get(k)))
			}
			for _, s := range x.SymbolKeys() {
				parts = append(parts, "["+encSymbol(s)+"]:"+out(x.GetSymbol(s)))
			}
			return prefix + "{" + strings.Join(parts, ",") + "}"
		case *Validator:
			var parts []string
			for _, k := range x.props.Keys() {
				parts = append(parts, jsvalue.JSONQuote(k)+":"+out(x.props.Get(k)))
			}
			return prefix + "fn " + jsvalue.JSONQuote(x.name) + "{" + strings.Join(parts, ",") + "}"
		case builtin:
			return prefix + "fn " + jsvalue.JSONQuote(toStr(getProp(x, "name"))) + "{}"
		case *namedFunc:
			return prefix + "fn " + jsvalue.JSONQuote(x.name) + "{}"
		case *objectPrototypeValue:
			return prefix + "{}"
		case *jsvalue.RegExp:
			return prefix + "RegExp(" + x.String() + ")"
		case *time.Time:
			return prefix + "Date(" + encNumber(float64(x.UnixMilli())) + ")"
		case map[string]any:
			return prefix + "object Map"
		case *instance:
			return prefix + "object " + x.ctor
		}
		return prefix + "fn \"\"{}"
	}
	res := make([]string, len(roots))
	for i, r := range roots {
		res[i] = out(r)
	}
	return res
}

// thrown renders a panic as the oracle renders an exception.
func thrown(r any) string {
	switch e := r.(type) {
	case *jsvalue.TypeError:
		return "throws TypeError: " + e.Message
	case *jsvalue.RangeError:
		return "throws RangeError: " + e.Message
	case *jsvalue.Error:
		return "throws Error: " + e.Message
	case *jsvalue.URIError:
		return "throws URIError: " + e.Message
	}
	panic(r) // a Go runtime error: a bug of the port, not a JavaScript exception
}

// warnSink collects console.warn calls while a case runs, as the oracle does.
var warnSink *[]string

func init() {
	ConsoleWarn = func(msg string) {
		lines := strings.Split(msg, "\n")
		line := "warn " + lines[0]
		if len(lines) > 1 {
			line += fmt.Sprintf(" (+%d)", len(lines)-1)
		}
		if warnSink != nil {
			*warnSink = append(*warnSink, line)
		}
	}
}

// run is the oracle's run: f's warnings, then its result or exception, with the arguments afterwards.
func run(f func(args []any) any, args []any, argsOnThrow bool) (trace []string) {
	trace = []string{}
	warnSink = &trace
	defer func() {
		warnSink = nil
		if r := recover(); r != nil {
			line := thrown(r)
			if argsOnThrow {
				line += " args " + enc(jsvalue.NewArray(args...))
			}
			trace = append(trace, line)
		}
	}()
	ret := f(args)
	trace = append(trace, "ret "+enc(jsvalue.NewArray(append([]any{ret}, args...)...)))
	return trace
}

// loneSurrogatesAsFFFD replaces each lone surrogate in an encoded trace (JSON.stringify writes one as the escape
// \udXXX; Go strings cannot hold one, the port gives U+FFFD in its place) by U+FFFD, and reports whether it did. A
// pair of surrogates is never escaped by JSON.stringify, and an escaped backslash before "u" is not an escape.
func loneSurrogatesAsFFFD(s string) (string, bool) {
	var b strings.Builder
	changed := false
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			b.WriteByte(s[i])
			continue
		}
		if s[i+1] == 'u' && i+6 <= len(s) {
			if v, err := strconv.ParseUint(s[i+2:i+6], 16, 16); err == nil && v >= 0xd800 && v <= 0xdfff {
				b.WriteString("�")
				changed = true
				i += 5
				continue
			}
		}
		b.WriteString(s[i : i+2])
		i++
	}
	return b.String(), changed
}

// mismatches reports the first differences between expected and got traces.
type mismatches struct {
	t          *testing.T
	n          int
	checks     int
	surrogates int // traces compared with their lone surrogates as U+FFFD
}

func (m *mismatches) compare(section string, i int, want, got []string, context func() string) {
	m.checks++
	fixed := make([]string, len(want))
	for j, w := range want {
		f, changed := loneSurrogatesAsFFFD(w)
		if changed {
			m.surrogates++
		}
		fixed[j] = f
	}
	want = fixed
	if len(want) == len(got) {
		same := true
		for j := range want {
			if want[j] != got[j] {
				same = false
				break
			}
		}
		if same {
			return
		}
	}
	m.n++
	if m.n <= 25 {
		m.t.Errorf("%s[%d] %s\nwant %q\n got %q", section, i, context(), want, got)
	}
}
