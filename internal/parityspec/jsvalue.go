package parityspec

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/gav4you/npmgo/internal/jsnum"
	"github.com/gav4you/npmgo/internal/jsvalue"
)

// Func is a JavaScript function built from a {"$": "fn"} spec in the jsvalue model: a pointer, so that it has an
// identity.
type Func struct{ Name string }

// JSTypeOf reports "function".
func (*Func) JSTypeOf() string { return "function" }

// Instance is an object of a class built from an {"$": "obj", "ctor": "Name"} spec: not a plain object.
type Instance struct {
	Ctor  string
	Props *jsvalue.Object
}

// BuildJS turns a JSON value spec (the language of parity/build.cjs) into the jsvalue model: undefined is
// jsvalue.Undefined{}, null nil, numbers float64, arrays []any, plain objects (no ctor, or a null prototype)
// *jsvalue.Object, class instances *Instance, functions *Func, Dates *time.Time, RegExps *regexp.Regexp, errors Go
// errors (jsvalue's TypeError and RangeError for those classes), symbols jsvalue.Symbol and BigInts *big.Int. refs
// holds the objects built with an id, shared between calls (nil for a fresh table).
func BuildJS(raw json.RawMessage, refs map[string]any) any {
	if refs == nil {
		refs = map[string]any{}
	}
	return buildJS(raw, refs)
}

func buildJS(raw json.RawMessage, refs map[string]any) any {
	raw = bytes.TrimSpace(raw)
	switch raw[0] {
	case '[':
		var items []json.RawMessage
		mustUnmarshal(raw, &items)
		out := make([]any, len(items))
		for i, it := range items {
			out[i] = buildJS(it, refs)
		}
		return out
	case '{':
	default:
		var v any
		mustUnmarshal(raw, &v)
		return v
	}
	var s map[string]json.RawMessage
	mustUnmarshal(raw, &s)
	str := func(k string) string {
		var v string
		if r, ok := s[k]; ok {
			mustUnmarshal(r, &v)
		}
		return v
	}
	props := func(o *jsvalue.Object) {
		var list [][]json.RawMessage
		if r, ok := s["props"]; ok {
			mustUnmarshal(r, &list)
		}
		for _, p := range list {
			if len(p) > 2 {
				panic("parityspec: property flags are not supported in the jsvalue model")
			}
			var key string
			mustUnmarshal(p[0], &key)
			o.Set(key, buildJS(p[1], refs)) // build.cjs defines __proto__ as an own property too
		}
	}
	switch str("$") {
	case "undef":
		return jsvalue.Undefined{}
	case "num":
		if v := str("v"); v == "-0" {
			return math.Copysign(0, -1)
		} else {
			return jsnum.Number(v)
		}
	case "big":
		b, ok := new(big.Int).SetString(str("v"), 10)
		if !ok {
			panic("parityspec: bad bigint " + str("v"))
		}
		return b
	case "sym":
		return jsvalue.NewSymbol(str("d"))
	case "ref":
		v, ok := refs[str("id")]
		if !ok {
			panic("parityspec: unknown ref " + str("id"))
		}
		return v
	case "obj":
		o := &jsvalue.Object{}
		var out any = o
		if c, ok := s["ctor"]; ok && string(c) != "null" && str("ctor") != "Object" {
			out = &Instance{Ctor: str("ctor"), Props: o}
		}
		if id := str("id"); id != "" {
			refs[id] = out
		}
		props(o)
		return out
	case "arr":
		var items []json.RawMessage
		mustUnmarshal(s["items"], &items)
		a := make([]any, len(items))
		if id := str("id"); id != "" {
			refs[id] = a
		}
		for i, it := range items {
			a[i] = buildJS(it, refs)
		}
		return a
	case "date":
		var ms *float64
		mustUnmarshal(s["ms"], &ms)
		if ms == nil {
			panic("parityspec: an invalid Date has no time.Time")
		}
		t := time.UnixMilli(int64(*ms))
		return &t
	case "re":
		src, flags := str("src"), str("flags")
		prefix := ""
		switch flags {
		case "":
		case "i":
			prefix = "(?i)"
		default:
			panic("parityspec: RegExp flags " + flags + " are not supported")
		}
		return regexp.MustCompile(prefix + src)
	case "fn":
		f := &Func{Name: str("name")}
		if id := str("id"); id != "" {
			refs[id] = f
		}
		return f
	case "err":
		msg := str("msg")
		switch str("ctor") {
		case "", "Error":
			return errors.New(msg)
		case "TypeError":
			return &jsvalue.TypeError{Message: msg}
		case "RangeError":
			return &jsvalue.RangeError{Message: msg}
		}
		panic("parityspec: error class " + str("ctor") + " is not supported")
	}
	panic("parityspec: unsupported spec " + string(raw))
}

func mustUnmarshal(raw json.RawMessage, v any) {
	if err := json.Unmarshal(raw, v); err != nil {
		panic("parityspec: " + err.Error())
	}
}

// Enc renders a value of the jsvalue model the way the OpenTelemetry oracle's enc renders the JavaScript value.
// special, if not nil, renders the values of the package under test first (it reports false for others).
func Enc(v any, special func(any) (string, bool)) string {
	if special != nil {
		if s, ok := special(v); ok {
			return s
		}
	}
	switch x := v.(type) {
	case nil:
		return "null"
	case jsvalue.Undefined:
		return "undefined"
	case string:
		return jsvalue.JSONQuote(x)
	case bool:
		if x {
			return "true"
		}
		return "false"
	case *big.Int:
		return x.String() + "n"
	case jsvalue.Symbol:
		return x.String()
	case *time.Time:
		return "Date(" + EncNumber(float64(x.UnixMilli())) + ")"
	case time.Time:
		return "Date(" + EncNumber(float64(x.UnixMilli())) + ")"
	case *regexp.Regexp:
		src, flags := x.String(), ""
		if strings.HasPrefix(src, "(?i)") {
			src, flags = src[4:], "i"
		}
		return "RegExp(/" + src + "/" + flags + ")"
	case *jsvalue.TypeError:
		return "Error(TypeError: " + x.Message + ")"
	case *jsvalue.RangeError:
		return "Error(RangeError: " + x.Message + ")"
	case *jsvalue.URIError:
		return "Error(URIError: " + x.Message + ")"
	case *jsvalue.Object:
		if x == nil {
			return "null"
		}
		var b strings.Builder
		b.WriteByte('{')
		for i, k := range x.Keys() {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(jsvalue.JSONQuote(k) + ":" + Enc(x.Get(k), special))
		}
		b.WriteByte('}')
		return b.String()
	case *Instance:
		return "object " + x.Ctor
	case error:
		return "Error(Error: " + x.Error() + ")"
	}
	if jsvalue.IsNumber(v) {
		return EncNumber(jsvalue.Float(v))
	}
	if jsvalue.IsNull(v) {
		return "null"
	}
	if jsvalue.TypeOf(v) == "function" {
		return "fn"
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array {
		parts := make([]string, rv.Len())
		for i := range parts {
			parts[i] = Enc(rv.Index(i).Interface(), special)
		}
		return "[" + strings.Join(parts, ",") + "]"
	}
	t := rv.Type()
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return "object " + t.Name()
}

// EncNumber renders a number as String(x) does, and -0 as "-0".
func EncNumber(x float64) string {
	if x == 0 && math.Signbit(x) {
		return "-0"
	}
	return jsnum.String(x)
}
