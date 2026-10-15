// Copyright (c) 2014-present Sebastian McKenzie and other contributors. MIT License (see LICENSE).
// Written for the Go translation of @babel/types 7.29.0 by the npmgo contributors.

package types

// The JavaScript object model the translation needs: property reads and writes on any value of the jsvalue model,
// with V8's TypeErrors for null and undefined, and lookups in the plain objects @babel/types keeps its tables in
// (NODE_FIELDS, VISITOR_KEYS, ...), which inherit Object.prototype: `VISITOR_KEYS["toString"]` is a function, and
// `"constructor" in FLIPPED_ALIAS_KEYS` is true. Those inherited properties are modelled, because a type name is a
// string the caller chooses (is(type, node), isType, validate(node, key, val)).

import (
	"math/big"
	"strconv"

	"github.com/gav4you/npmgo/internal/jsnum"
	"github.com/gav4you/npmgo/internal/jsvalue"
)

// objectPrototypeFunctions are Object.prototype's methods with their lengths; "__proto__" is its accessor.
var objectPrototypeFunctions = map[string]int{
	"constructor": 1, "__defineGetter__": 2, "__defineSetter__": 2, "hasOwnProperty": 1, "__lookupGetter__": 1,
	"__lookupSetter__": 1, "isPrototypeOf": 1, "propertyIsEnumerable": 1, "toString": 0, "valueOf": 0, "toLocaleString": 0,
}

// builtin is one of Object.prototype's methods (Object itself for "constructor"), read through an object that
// inherits it.
type builtin struct{ name string }

// JSTypeOf reports "function".
func (builtin) JSTypeOf() string { return "function" }

// String is Function.prototype.toString of a built-in function.
func (b builtin) String() string {
	name := b.name
	if name == "constructor" {
		name = "Object"
	}
	return "function " + name + "() { [native code] }"
}

// objectPrototypeValue is Object.prototype itself, which obj.__proto__ reads.
type objectPrototypeValue struct{}

var objectPrototype = &objectPrototypeValue{}

// inheritedProp is the value of an Object.prototype property read through an ordinary object.
func inheritedProp(key string) (any, bool) {
	if key == "__proto__" {
		return objectPrototype, true
	}
	if _, ok := objectPrototypeFunctions[key]; ok {
		return builtin{key}, true
	}
	return nil, false
}

// propertyKey is ToPropertyKey: a symbol stays a symbol (ok false: no table has symbol keys), anything else becomes
// its string.
func propertyKey(v any) (string, bool) {
	switch x := v.(type) {
	case string:
		return x, true
	case jsvalue.Symbol:
		return "", false
	}
	return jsvalue.ToString(v), true
}

// tableGet is table[key] for one of the package's plain-object tables.
func tableGet(table *jsvalue.Object, key any) any {
	k, ok := propertyKey(key)
	if !ok {
		return jsvalue.Undefined{}
	}
	if v, ok := table.Lookup(k); ok {
		return v
	}
	if v, ok := inheritedProp(k); ok {
		return v
	}
	return jsvalue.Undefined{}
}

// tableHas is `key in table`.
func tableHas(table *jsvalue.Object, key any) bool {
	k, ok := propertyKey(key)
	if !ok {
		return false
	}
	if table.Has(k) {
		return true
	}
	_, ok = inheritedProp(k)
	return ok
}

func typeError(msg string) *jsvalue.TypeError { return &jsvalue.TypeError{Message: msg} }

// readError is V8's TypeError for reading a property of null or undefined.
func readError(v any, key string) *jsvalue.TypeError {
	what := "null"
	if jsvalue.IsUndefined(v) {
		what = "undefined"
	}
	return typeError("Cannot read properties of " + what + " (reading '" + key + "')")
}

// getProp is v[key] for a string key: own properties of objects, elements and length of arrays and strings, the
// metadata of validators, Object.prototype's members through any of them. It panics with V8's TypeError for null and
// undefined.
func getProp(v any, key string) any {
	switch x := v.(type) {
	case nil, jsvalue.Undefined:
		panic(readError(v, key))
	case *jsvalue.Object:
		if x == nil {
			panic(readError(nil, key))
		}
		if val, ok := x.Lookup(key); ok {
			return val
		}
	case string:
		units := utf16Units(x)
		if key == "length" {
			return float64(len(units))
		}
		if i, ok := jsvalue.ArrayIndex(key); ok {
			if i < uint64(len(units)) {
				return unitString(units[i])
			}
			return jsvalue.Undefined{}
		}
	case *Validator:
		if x == nil {
			panic(readError(nil, key))
		}
		if val, ok := x.props.Lookup(key); ok {
			return val
		}
		switch key {
		case "name":
			return x.name
		case "length":
			return float64(x.length)
		}
	case builtin:
		switch key {
		case "length":
			return float64(objectPrototypeFunctions[x.name])
		case "name":
			if x.name == "constructor" {
				return "Object"
			}
			return x.name
		}
	default:
		if jsvalue.IsArray(v) {
			items := jsvalue.Items(v)
			if key == "length" {
				return float64(len(items))
			}
			if i, ok := jsvalue.ArrayIndex(key); ok {
				if i < uint64(len(items)) {
					return items[i]
				}
				return jsvalue.Undefined{}
			}
		} else if jsvalue.IsNull(v) {
			panic(readError(nil, key))
		}
	}
	if val, ok := inheritedProp(key); ok {
		return val
	}
	return jsvalue.Undefined{}
}

// getIndex is v[i] for a number key.
func getIndex(v any, i int) any { return getProp(v, strconv.Itoa(i)) }

// getSymbolProp is v[sym]: the own symbol-keyed property of an object (no other value has one).
func getSymbolProp(v any, sym jsvalue.Symbol) any {
	switch x := v.(type) {
	case nil, jsvalue.Undefined:
		panic(readError(v, sym.String()))
	case *jsvalue.Object:
		if x == nil {
			panic(readError(nil, sym.String()))
		}
		return x.GetSymbol(sym)
	}
	if jsvalue.IsNull(v) {
		panic(readError(nil, sym.String()))
	}
	return jsvalue.Undefined{}
}

// setProp is the assignment v[key] = val in strict mode. Named properties of arrays and functions are not modelled:
// such an assignment is dropped.
func setProp(v any, key string, val any) {
	what := ""
	switch x := v.(type) {
	case nil, jsvalue.Undefined:
		w := "null"
		if jsvalue.IsUndefined(v) {
			w = "undefined"
		}
		panic(typeError("Cannot set properties of " + w + " (setting '" + key + "')"))
	case *jsvalue.Object:
		if x == nil {
			panic(typeError("Cannot set properties of null (setting '" + key + "')"))
		}
		x.Put(key, val)
		return
	case string:
		what = "string '" + x + "'"
	case bool:
		what = "boolean '" + strconv.FormatBool(x) + "'"
	case *big.Int:
		if x != nil {
			what = "bigint '" + x.String() + "'"
		}
	case jsvalue.Symbol:
		what = "symbol '" + x.String() + "'"
	}
	if what == "" && jsvalue.IsNumber(v) {
		what = "number '" + jsnum.String(jsvalue.Float(v)) + "'"
	}
	if what != "" {
		panic(typeError("Cannot create property '" + key + "' on " + what))
	}
	if jsvalue.IsNull(v) {
		panic(typeError("Cannot set properties of null (setting '" + key + "')"))
	}
}

// objectKeys is Object.keys(v).
func objectKeys(v any) []string {
	switch x := v.(type) {
	case nil, jsvalue.Undefined:
		panic(typeError("Cannot convert undefined or null to object"))
	case *jsvalue.Object:
		if x == nil {
			panic(typeError("Cannot convert undefined or null to object"))
		}
		return x.Keys()
	case string:
		n := len(utf16Units(x))
		keys := make([]string, n)
		for i := range keys {
			keys[i] = strconv.Itoa(i)
		}
		return keys
	case *Validator:
		return x.props.Keys()
	}
	if jsvalue.IsArray(v) {
		return jsvalue.ObjectKeys(v)
	}
	if jsvalue.IsNull(v) {
		panic(typeError("Cannot convert undefined or null to object"))
	}
	return nil
}

// toStr is the string a template literal makes of v (`${v}`): ToString, a TypeError for a symbol.
func toStr(v any) string { return jsvalue.ToString(v) }

// jsonText is `${JSON.stringify(v)}`: "undefined" where JSON.stringify returns undefined.
func jsonText(v any) string {
	if s, ok := jsvalue.JSONStringify(v); ok {
		return s
	}
	return "undefined"
}

// isArray is Array.isArray.
func isArray(v any) bool { return jsvalue.IsArray(v) }

// items returns the elements of an array (nil for anything else).
func items(v any) []any { return jsvalue.Items(v) }

// includes is v.includes(x) for v read from a table or a node: an array's SameValueZero search, a string's substring
// search, and V8's TypeError naming the expression for anything else.
func includes(v any, x any, expr string) bool {
	if isArray(v) {
		for _, it := range items(v) {
			if jsvalue.SameValueZero(it, x) {
				return true
			}
		}
		return false
	}
	if s, ok := v.(string); ok {
		return containsUnits(utf16Units(s), utf16Units(toStr(x)))
	}
	panic(typeError(expr + ".includes is not a function"))
}

// spreadItems is what a spread argument (...v) contributes: an array's elements, a string's code points; anything
// else panics with V8's TypeError.
func spreadItems(v any, expr string) []any {
	if isArray(v) {
		return items(v)
	}
	if s, ok := v.(string); ok {
		var out []any
		for _, r := range s {
			out = append(out, string(r))
		}
		return out
	}
	if nullish(v) {
		what := "null"
		if jsvalue.IsUndefined(v) {
			what = "undefined"
		}
		panic(typeError(expr + " is not iterable (cannot read property " + what + ")"))
	}
	panic(typeError("Spread syntax requires ...iterable[Symbol.iterator] to be a function"))
}

// arrayFrom is Array.from(v): the elements of an array, the code points of a string, the elements of an array-like
// object (by its length), and nothing for any other value; null and undefined throw.
func arrayFrom(v any) []any {
	if isArray(v) {
		return append([]any(nil), items(v)...)
	}
	switch x := v.(type) {
	case nil:
		panic(typeError("object null is not iterable (cannot read property Symbol(Symbol.iterator))"))
	case jsvalue.Undefined:
		panic(typeError("undefined is not iterable (cannot read property Symbol(Symbol.iterator))"))
	case string:
		return spreadItems(x, "")
	case *jsvalue.Object:
		if x == nil {
			panic(typeError("object null is not iterable (cannot read property Symbol(Symbol.iterator))"))
		}
		n := jsvalue.ToNumber(x.Get("length"))
		var out []any
		for i := 0; float64(i) < n && i < 1<<24; i++ {
			out = append(out, getIndex(x, i))
		}
		return out
	}
	return nil
}

// methodTarget checks that v has the array method being called on it (v.map(...), v.concat(...), v.push(...)):
// V8's TypeError for null and undefined (reading the method) and for any other value that is not an array.
func methodTarget(v any, method, expr string) {
	if isArray(v) {
		return
	}
	if nullish(v) {
		panic(readError(v, method))
	}
	panic(typeError(expr + "." + method + " is not a function"))
}

// inOperand is how V8 names the right operand of `in` in its TypeError.
func inOperand(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case jsvalue.Undefined:
		return "undefined"
	case jsvalue.Symbol:
		return x.String()
	}
	return toStr(v)
}

// hasIn is `key in v`: own or inherited properties of an object; V8's TypeError for anything that is not an object.
func hasIn(v any, key string) bool {
	switch x := v.(type) {
	case *jsvalue.Object:
		if x != nil {
			if x.Has(key) {
				return true
			}
			_, ok := inheritedProp(key)
			return ok
		}
	case *Validator:
		if x != nil {
			_, ok := inheritedProp(key)
			return x.props.Has(key) || key == "name" || key == "length" || ok
		}
	}
	if isArray(v) {
		if key == "length" {
			return true
		}
		if i, ok := jsvalue.ArrayIndex(key); ok {
			return i < uint64(len(items(v)))
		}
		_, ok := inheritedProp(key)
		return ok
	}
	if jsvalue.TypeOf(v) == "object" && !jsvalue.IsNull(v) || jsvalue.TypeOf(v) == "function" {
		_, ok := inheritedProp(key)
		return ok
	}
	panic(typeError("Cannot use 'in' operator to search for '" + key + "' in " + inOperand(v)))
}

// hasOwn is Object.prototype.hasOwnProperty.call(v, key).
func hasOwn(v any, key string) bool {
	switch x := v.(type) {
	case nil, jsvalue.Undefined:
		panic(typeError("Cannot convert undefined or null to object"))
	case *jsvalue.Object:
		if x == nil {
			panic(typeError("Cannot convert undefined or null to object"))
		}
		return x.Has(key)
	case string:
		if key == "length" {
			return true
		}
		if i, ok := jsvalue.ArrayIndex(key); ok {
			return i < uint64(len(utf16Units(x)))
		}
		return false
	case *Validator:
		return x.props.Has(key) || key == "name" || key == "length"
	}
	if isArray(v) {
		if key == "length" {
			return true
		}
		if i, ok := jsvalue.ArrayIndex(key); ok {
			return i < uint64(len(items(v)))
		}
	}
	return false
}

// iterate is the sequence a for-of loop over v visits: an array's elements, a string's code points; anything else
// panics with V8's TypeError naming the expression.
func iterate(v any, expr string) []any {
	if isArray(v) {
		return items(v)
	}
	if s, ok := v.(string); ok {
		var out []any
		for _, r := range s {
			out = append(out, string(r))
		}
		return out
	}
	panic(typeError(expr + " is not iterable"))
}

// arg is the i-th argument of a JavaScript call: undefined when it was not passed.
func arg(args []any, i int) any {
	if i < len(args) {
		return args[i]
	}
	return jsvalue.Undefined{}
}

// optArg is the optional trailing argument of a Go variadic parameter.
func optArg(opts []any) any { return arg(opts, 0) }

// stringProp is node[key] when it is an own string property of an object.
func stringProp(node any, key string) (string, bool) {
	if o, ok := node.(*jsvalue.Object); ok && o != nil {
		if v, ok := o.Lookup(key); ok {
			s, ok := v.(string)
			return s, ok
		}
	}
	return "", false
}

// optsMatch is the tail of every generated validator: opts == null || shallowEqual(node, opts).
func optsMatch(node any, opts []any) bool {
	o := optArg(opts)
	return jsvalue.IsNullish(o) || ShallowEqual(node, o)
}

// truthy is ToBoolean.
func truthy(v any) bool { return jsvalue.Truthy(v) }

// nullish is v == null.
func nullish(v any) bool { return jsvalue.IsNullish(v) }

// strictEquals is ===.
func strictEquals(a, b any) bool { return jsvalue.StrictEquals(a, b) }

// strArray is a new array of strings.
func strArray(s []string) *jsvalue.Array {
	a := &jsvalue.Array{}
	for _, x := range s {
		a.Push(x)
	}
	return a
}

// callValue calls a JavaScript function value: a validator, or a Go func through jsvalue.Call. Anything else panics
// with V8's TypeError naming the expression.
func callValue(fn any, expr string, args ...any) any {
	if v, ok := fn.(*Validator); ok && v != nil {
		v.fn(arg(args, 0), arg(args, 1), arg(args, 2))
		return jsvalue.Undefined{}
	}
	if r, ok := jsvalue.Call(fn, args...); ok {
		return r
	}
	panic(typeError(expr + " is not a function"))
}
