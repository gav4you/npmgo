// Package jsvalue models the JavaScript values that React's server renderer
// inspects, and reproduces the JavaScript coercions it relies on: typeof,
// ToString ("" + v), String(v), ToNumber, ToBoolean, Object.is,
// String.prototype.trim and JSON.stringify.
//
// Go model of JavaScript values:
//
//	JS             Go
//	undefined      Undefined{}
//	null           nil, or a nil pointer / func / map
//	boolean        bool
//	number         any Go integer or float type (converted to float64)
//	bigint         *big.Int
//	string         string (UTF-8; lone surrogates cannot be represented)
//	symbol         Symbol
//	object         *Object (ordered plain object), slices (arrays), and any other Go value
//	function       any Go func value, or a value whose JSTypeOf method reports "function"
package jsvalue

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/gav4you/npmgo/internal/jsnum"
)

// Undefined is JavaScript's undefined.
type Undefined struct{}

// TypeOfer lets a Go type report its JavaScript typeof (for example a React
// class or named function component reports "function").
type TypeOfer interface{ JSTypeOf() string }

// Symbol is a JavaScript symbol. Symbols compare by identity; SymbolFor
// returns the same symbol for the same key, like Symbol.for.
type Symbol struct{ s *symbolData }

type symbolData struct {
	desc    string
	hasDesc bool
}

var registry sync.Map // key -> Symbol

// NewSymbol returns a fresh symbol, like Symbol(description).
func NewSymbol(description string) Symbol {
	return Symbol{&symbolData{desc: description, hasDesc: true}}
}

// SymbolFor returns the registered symbol for key, like Symbol.for(key).
func SymbolFor(key string) Symbol {
	if s, ok := registry.Load(key); ok {
		return s.(Symbol)
	}
	s, _ := registry.LoadOrStore(key, Symbol{&symbolData{desc: key, hasDesc: true}})
	return s.(Symbol)
}

// Description is the symbol's description.
func (s Symbol) Description() string {
	if s.s == nil {
		return ""
	}
	return s.s.desc
}

// String renders the symbol like String(symbol): "Symbol(description)".
func (s Symbol) String() string { return "Symbol(" + s.Description() + ")" }

// TypeError is a JavaScript TypeError raised by a coercion or by a property
// read on undefined; Message is V8's message.
type TypeError struct{ Message string }

func (e *TypeError) Error() string { return e.Message }

// RangeError is a JavaScript RangeError (such as String.fromCodePoint's
// "Invalid code point"); Message is V8's message.
type RangeError struct{ Message string }

func (e *RangeError) Error() string { return e.Message }

// Error is a plain JavaScript Error (new Error(message)).
type Error struct{ Message string }

func (e *Error) Error() string { return e.Message }

// RegExp is a JavaScript RegExp value as its properties describe it: Source is its source (escaped the way
// JavaScript escapes it, "(?:)" for an empty pattern) and Flags its flags. It is not a matcher.
type RegExp struct{ Source, Flags string }

// String is RegExp.prototype.toString: "/source/flags".
func (r *RegExp) String() string { return "/" + r.Source + "/" + r.Flags }

// IsNull reports whether v is JavaScript null (nil, or a nil pointer, func,
// map, channel or interface).
func IsNull(v any) bool {
	switch x := v.(type) {
	case nil:
		return true
	case *Object:
		return x == nil
	case *Array:
		return x == nil
	case Undefined, bool, string, float64, int, Symbol:
		return false
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Pointer, reflect.Func, reflect.Map, reflect.Chan, reflect.Interface, reflect.UnsafePointer:
		return rv.IsNil()
	}
	return false
}

// IsNullish reports v == null in JavaScript (null or undefined).
func IsNullish(v any) bool {
	if _, ok := v.(Undefined); ok {
		return true
	}
	return IsNull(v)
}

// IsUndefined reports whether v is undefined.
func IsUndefined(v any) bool { _, ok := v.(Undefined); return ok }

// TypeOf returns the JavaScript typeof of v.
func TypeOf(v any) string {
	switch x := v.(type) {
	case nil:
		return "object"
	case Undefined:
		return "undefined"
	case bool:
		return "boolean"
	case string:
		return "string"
	case float64, float32, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, uintptr:
		return "number"
	case *big.Int:
		if x == nil {
			return "object"
		}
		return "bigint"
	case Symbol:
		return "symbol"
	case *Object:
		return "object"
	case TypeOfer:
		if IsNull(v) {
			return "object"
		}
		return x.JSTypeOf()
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Func && !rv.IsNil() {
		return "function"
	}
	return "object"
}

// IsNumber reports whether v is a JavaScript number.
func IsNumber(v any) bool {
	switch v.(type) {
	case float64, float32, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, uintptr:
		return true
	}
	return false
}

// Float returns the float64 value of a JavaScript number.
func Float(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case float32:
		return float64(x)
	case int:
		return float64(x)
	case int8:
		return float64(x)
	case int16:
		return float64(x)
	case int32:
		return float64(x)
	case int64:
		return float64(x)
	case uint:
		return float64(x)
	case uint8:
		return float64(x)
	case uint16:
		return float64(x)
	case uint32:
		return float64(x)
	case uint64:
		return float64(x)
	case uintptr:
		return float64(x)
	}
	return math.NaN()
}

// IsFunction reports typeof v === "function".
func IsFunction(v any) bool { return TypeOf(v) == "function" }

// IsArray reports Array.isArray(v): a non-nil *Array, and every Go slice except []byte.
func IsArray(v any) bool {
	switch x := v.(type) {
	case []any:
		return true
	case *Array:
		return x != nil
	case []byte, nil, string:
		return false
	}
	rv := reflect.ValueOf(v)
	return rv.Kind() == reflect.Slice
}

// Items returns the elements of an array value (see IsArray).
func Items(v any) []any {
	if a, ok := v.([]any); ok {
		return a
	}
	if a, ok := v.(*Array); ok {
		return a.Items()
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Slice {
		return nil
	}
	out := make([]any, rv.Len())
	for i := range out {
		out[i] = rv.Index(i).Interface()
	}
	return out
}

// funcSource stands in for Function.prototype.toString, which Go cannot
// reproduce (there is no source text).
const funcSource = "function () { [native code] }"

// ToString is JavaScript's "" + v. It panics with a *TypeError for symbols,
// as JavaScript throws.
func ToString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case nil:
		return "null"
	case Undefined:
		return "undefined"
	case bool:
		if x {
			return "true"
		}
		return "false"
	case float64:
		return jsnum.String(x)
	case int:
		return jsnum.String(float64(x))
	case *big.Int:
		if x == nil {
			return "null"
		}
		return x.String()
	case Symbol:
		panic(&TypeError{"Cannot convert a Symbol value to a string"})
	case *Object:
		if x == nil {
			return "null"
		}
		if f, ok := x.Lookup("toString"); ok {
			switch fn := f.(type) {
			case func() string:
				return fn()
			case func() any:
				return ToString(fn())
			}
		}
		return "[object Object]"
	case error:
		if IsNull(v) {
			return "null"
		}
		return "Error: " + x.Error()
	}
	if IsNumber(v) {
		return jsnum.String(Float(v))
	}
	if IsNull(v) {
		return "null"
	}
	if IsArray(v) {
		return joinArray(Items(v), ",")
	}
	if s, ok := v.(fmt.Stringer); ok {
		return s.String()
	}
	if TypeOf(v) == "function" {
		return funcSource
	}
	return "[object Object]"
}

// Join is Array.prototype.join(sep).
func Join(items []any, sep string) string { return joinArray(items, sep) }

func joinArray(items []any, sep string) string {
	var b strings.Builder
	for i, it := range items {
		if i > 0 {
			b.WriteString(sep)
		}
		if !IsNullish(it) {
			b.WriteString(ToString(it))
		}
	}
	return b.String()
}

// String is JavaScript's String(v): like ToString, but symbols become
// "Symbol(description)".
func String(v any) string {
	if s, ok := v.(Symbol); ok {
		return s.String()
	}
	return ToString(v)
}

// ToNumber is JavaScript's Number(v) / unary plus. It panics with a
// *TypeError for bigint and symbol values, as JavaScript throws.
func ToNumber(v any) float64 {
	switch x := v.(type) {
	case nil:
		return 0
	case Undefined:
		return math.NaN()
	case bool:
		if x {
			return 1
		}
		return 0
	case string:
		return StringToNumber(x)
	case *big.Int:
		panic(&TypeError{"Cannot convert a BigInt value to a number"})
	case Symbol:
		panic(&TypeError{"Cannot convert a Symbol value to a number"})
	}
	if IsNumber(v) {
		return Float(v)
	}
	if IsNull(v) {
		return 0
	}
	if IsArray(v) || TypeOf(v) == "object" {
		return StringToNumber(ToString(v))
	}
	return math.NaN()
}

// StringToNumber is JavaScript's StringToNumber.
func StringToNumber(s string) float64 {
	t := Trim(s)
	if strings.HasPrefix(t, "\u0085") || strings.HasSuffix(t, "\u0085") {
		return math.NaN() // not JS whitespace, but strings.TrimSpace would strip it
	}
	return jsnum.Number(t)
}

// IsNaN is the global isNaN(v).
func IsNaN(v any) bool { return math.IsNaN(ToNumber(v)) }

// Truthy is JavaScript's ToBoolean.
func Truthy(v any) bool {
	switch x := v.(type) {
	case nil, Undefined:
		return false
	case bool:
		return x
	case string:
		return x != ""
	case float64:
		return x != 0 && !math.IsNaN(x)
	case *big.Int:
		return x != nil && x.Sign() != 0
	}
	if IsNumber(v) {
		f := Float(v)
		return f != 0 && !math.IsNaN(f)
	}
	return !IsNull(v)
}

// StrictEquals is JavaScript's ===.
func StrictEquals(a, b any) bool {
	if IsNumber(a) && IsNumber(b) {
		return Float(a) == Float(b)
	}
	return ObjectIs(a, b)
}

// SameValueZero is the comparison of Array.prototype.includes, Set and Map: Object.is, except that +0 and -0 are
// equal.
func SameValueZero(a, b any) bool {
	if IsNumber(a) && IsNumber(b) {
		x, y := Float(a), Float(b)
		return x == y || (math.IsNaN(x) && math.IsNaN(y))
	}
	return ObjectIs(a, b)
}

// ObjectIs is JavaScript's Object.is. Objects compare by identity: pointers
// directly, slices / maps / funcs by their underlying pointer (an
// approximation: Go closures from the same literal share a code pointer).
func ObjectIs(a, b any) bool {
	if IsNumber(a) || IsNumber(b) {
		if !IsNumber(a) || !IsNumber(b) {
			return false
		}
		x, y := Float(a), Float(b)
		if math.IsNaN(x) && math.IsNaN(y) {
			return true
		}
		return x == y && math.Signbit(x) == math.Signbit(y)
	}
	if IsNull(a) || IsNull(b) {
		return IsNull(a) && IsNull(b)
	}
	switch x := a.(type) {
	case string:
		y, ok := b.(string)
		return ok && x == y
	case bool:
		y, ok := b.(bool)
		return ok && x == y
	case Undefined:
		_, ok := b.(Undefined)
		return ok
	case *big.Int:
		y, ok := b.(*big.Int)
		return ok && x.Cmp(y) == 0
	}
	ra, rb := reflect.ValueOf(a), reflect.ValueOf(b)
	if ra.Type() != rb.Type() {
		return false
	}
	switch ra.Kind() {
	case reflect.Slice:
		return ra.Pointer() == rb.Pointer() && ra.Len() == rb.Len()
	case reflect.Map, reflect.Func, reflect.Pointer, reflect.Chan, reflect.UnsafePointer:
		return ra.Pointer() == rb.Pointer()
	}
	if ra.Type().Comparable() {
		return a == b
	}
	return false
}

// isJSSpace reports JavaScript WhiteSpace or LineTerminator.
func isJSSpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ', 0xA0, 0x1680, 0x2028, 0x2029, 0x202F, 0x205F, 0x3000, 0xFEFF:
		return true
	}
	return r >= 0x2000 && r <= 0x200A
}

// Trim is String.prototype.trim.
func Trim(s string) string { return strings.TrimFunc(s, isJSSpace) }

// JSONQuote is JSON.stringify(s) for a string.
func JSONQuote(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 {
				fmt.Fprintf(&b, `\u%04x`, r)
			} else if r == utf8.RuneError {
				b.WriteRune(r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// errUnserializable marks values JSON.stringify omits (undefined, functions, symbols).
var errUnserializable = errors.New("unserializable")

// JSONStringify is JSON.stringify(v) for the modelled values. ok is false
// where JSON.stringify returns undefined. Like JSON.stringify it panics with a
// *TypeError for a BigInt and for a circular structure (with V8's message,
// which describes the circle).
func JSONStringify(v any) (string, bool) {
	var s jsonStringifier
	if err := s.write(&s.b, "", v); err != nil {
		return "", false
	}
	return s.b.String(), true
}

// jsonFrame is an object being serialised: the key it was reached by (a string, or an int index of an array) and its
// identity.
type jsonFrame struct {
	key   any
	id    any
	array bool
}

type jsonStringifier struct {
	b     strings.Builder
	stack []jsonFrame
}

// identity returns a comparable identity for an object or array value (nil for values that have none).
func jsonIdentity(v any) any {
	switch x := v.(type) {
	case *Object:
		return x
	case *Array:
		return x
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Slice {
		return [2]uintptr{rv.Pointer(), uintptr(rv.Len())}
	}
	return nil
}

// enter pushes an object, or panics with V8's circular-structure TypeError when it is already being serialised.
func (s *jsonStringifier) enter(key, v any, array bool) {
	id := jsonIdentity(v)
	if id != nil {
		for i, f := range s.stack {
			if f.id == id {
				panic(&TypeError{s.circularMessage(key, i)})
			}
		}
	}
	s.stack = append(s.stack, jsonFrame{key, id, array})
}

func (s *jsonStringifier) leave() { s.stack = s.stack[:len(s.stack)-1] }

// circularMessage is V8's JsonStringifier::ConstructCircularStructureErrorMessage: the object the circle starts at,
// at most two lines after it, an ellipsis, the last line, and the key that closes the circle.
func (s *jsonStringifier) circularMessage(lastKey any, start int) string {
	const prefixCount, postfixCount = 2, 1
	ctor := func(f jsonFrame) string {
		if f.array {
			return "object with constructor 'Array'"
		}
		return "object with constructor 'Object'"
	}
	keyText := func(k any) string {
		if i, ok := k.(int); ok {
			return "index " + strconv.Itoa(i)
		}
		if k.(string) == "" {
			return "<anonymous>"
		}
		return "property '" + k.(string) + "'"
	}
	var b strings.Builder
	b.WriteString("Converting circular structure to JSON")
	index := start
	b.WriteString("\n    --> starting at " + ctor(s.stack[index]))
	index++
	prefixEnd := min(len(s.stack), index+prefixCount)
	for ; index < prefixEnd; index++ {
		b.WriteString("\n    |     " + keyText(s.stack[index].key) + " -> " + ctor(s.stack[index]))
	}
	if len(s.stack) > index+postfixCount {
		b.WriteString("\n    |     ...")
	}
	index = max(index, len(s.stack)-postfixCount)
	for ; index < len(s.stack); index++ {
		b.WriteString("\n    |     " + keyText(s.stack[index].key) + " -> " + ctor(s.stack[index]))
	}
	b.WriteString("\n    --- " + keyText(lastKey) + " closes the circle")
	return b.String()
}

func (s *jsonStringifier) write(b *strings.Builder, key, v any) error {
	switch x := v.(type) {
	case nil:
		b.WriteString("null")
		return nil
	case Undefined, Symbol:
		return errUnserializable
	case bool:
		if x {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
		return nil
	case string:
		b.WriteString(JSONQuote(x))
		return nil
	case *big.Int:
		panic(&TypeError{"Do not know how to serialize a BigInt"})
	case *Object:
		if x == nil {
			b.WriteString("null")
			return nil
		}
		if f, ok := x.Lookup("toJSON"); ok {
			if fn, ok := f.(func() any); ok {
				return s.write(b, key, fn())
			}
		}
		s.enter(key, x, false)
		b.WriteByte('{')
		first := true
		for _, k := range x.Keys() {
			var sub strings.Builder
			if err := s.write(&sub, k, x.Get(k)); err != nil {
				continue
			}
			if !first {
				b.WriteByte(',')
			}
			first = false
			b.WriteString(JSONQuote(k))
			b.WriteByte(':')
			b.WriteString(sub.String())
		}
		b.WriteByte('}')
		s.leave()
		return nil
	}
	if IsNumber(v) {
		f := Float(v)
		if math.IsNaN(f) || math.IsInf(f, 0) {
			b.WriteString("null")
		} else {
			b.WriteString(jsnum.String(f))
		}
		return nil
	}
	if IsNull(v) {
		b.WriteString("null")
		return nil
	}
	if IsArray(v) {
		s.enter(key, v, true)
		b.WriteByte('[')
		for i, it := range Items(v) {
			if i > 0 {
				b.WriteByte(',')
			}
			if err := s.write(b, i, it); err != nil {
				b.WriteString("null")
			}
		}
		b.WriteByte(']')
		s.leave()
		return nil
	}
	if TypeOf(v) == "function" {
		return errUnserializable
	}
	b.WriteString("{}")
	return nil
}

// ObjectKeys is Object.keys(v) for the modelled objects (other Go values
// have no own enumerable properties).
func ObjectKeys(v any) []string {
	switch x := v.(type) {
	case *Object:
		return x.Keys()
	}
	if IsArray(v) {
		items := Items(v)
		keys := make([]string, len(items))
		for i := range items {
			keys[i] = strconv.Itoa(i)
		}
		return keys
	}
	return nil
}
