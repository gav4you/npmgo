package nodeutil

import (
	"regexp"
	"sort"
	"strconv"
)

// Prototype-chain support: translation of getConstructorName and
// addPrototypeProperties from inspect.js, over the Go value model.

func wellKnownProto(name string, parent *Object) *Object {
	o := &Object{wellKnown: name}
	if parent == nil {
		o.Proto = Proto{Null: true}
	} else {
		o.Proto = Proto{Prototype: parent}
	}
	return o
}

// Well-known built-in prototype sentinels (inspect.js wellKnownPrototypes).
var (
	ObjectPrototype         = wellKnownProto("Object", nil)
	ArrayPrototype          = wellKnownProto("Array", ObjectPrototype)
	ArrayBufferPrototype    = wellKnownProto("ArrayBuffer", ObjectPrototype)
	FunctionPrototype       = wellKnownProto("Function", ObjectPrototype)
	MapPrototype            = wellKnownProto("Map", ObjectPrototype)
	SetPrototype            = wellKnownProto("Set", ObjectPrototype)
	TypedArrayPrototype     = wellKnownProto("TypedArray", ObjectPrototype)
	RegExpPrototype         = wellKnownProto("RegExp", ObjectPrototype)
	DatePrototype           = wellKnownProto("Date", ObjectPrototype)
	DataViewPrototype       = wellKnownProto("DataView", ObjectPrototype)
	ErrorPrototype          = wellKnownProto("Error", ObjectPrototype)
	AggregateErrorPrototype = wellKnownProto("AggregateError", ErrorPrototype)
	RangeErrorPrototype     = wellKnownProto("RangeError", ErrorPrototype)
	TypeErrorPrototype      = wellKnownProto("TypeError", ErrorPrototype)
	BooleanPrototype        = wellKnownProto("Boolean", ObjectPrototype)
	NumberPrototype         = wellKnownProto("Number", ObjectPrototype)
	StringPrototype         = wellKnownProto("String", ObjectPrototype)
	PromisePrototype        = wellKnownProto("Promise", ObjectPrototype)
	WeakMapPrototype        = wellKnownProto("WeakMap", ObjectPrototype)
	WeakSetPrototype        = wellKnownProto("WeakSet", ObjectPrototype)
)

// builtInObjects is the set of capitalised globalThis names in Node v25.9.0.
var builtInObjects = func() map[string]bool {
	m := map[string]bool{}
	for _, n := range []string{"AbortController", "AbortSignal", "AggregateError", "Array", "ArrayBuffer",
		"AsyncDisposableStack", "Atomics", "BigInt", "BigInt64Array", "BigUint64Array", "Blob", "Boolean",
		"BroadcastChannel", "Buffer", "ByteLengthQueuingStrategy", "CloseEvent", "CompressionStream",
		"CountQueuingStrategy", "Crypto", "CryptoKey", "CustomEvent", "DOMException", "DataView", "Date",
		"DecompressionStream", "DisposableStack", "Error", "ErrorEvent", "EvalError", "Event", "EventTarget",
		"File", "FinalizationRegistry", "Float16Array", "Float32Array", "Float64Array", "FormData", "Function",
		"Headers", "Infinity", "Int16Array", "Int32Array", "Int8Array", "Intl", "Iterator", "JSON", "Map",
		"Math", "MessageChannel", "MessageEvent", "MessagePort", "NaN", "Navigator", "Number", "Object",
		"Performance", "PerformanceEntry", "PerformanceMark", "PerformanceMeasure", "PerformanceObserver",
		"PerformanceObserverEntryList", "PerformanceResourceTiming", "Promise", "Proxy", "RangeError",
		"ReadableByteStreamController", "ReadableStream", "ReadableStreamBYOBReader", "ReadableStreamBYOBRequest",
		"ReadableStreamDefaultController", "ReadableStreamDefaultReader", "ReferenceError", "Reflect", "RegExp",
		"Request", "Response", "Set", "SharedArrayBuffer", "Storage", "String", "SubtleCrypto", "SuppressedError",
		"Symbol", "SyntaxError", "TextDecoder", "TextDecoderStream", "TextEncoder", "TextEncoderStream",
		"TransformStream", "TransformStreamDefaultController", "TypeError", "URIError", "URL", "URLPattern",
		"URLSearchParams", "Uint16Array", "Uint32Array", "Uint8Array", "Uint8ClampedArray", "WeakMap", "WeakRef",
		"WeakSet", "WebAssembly", "WebSocket", "WritableStream", "WritableStreamDefaultController",
		"WritableStreamDefaultWriter"} {
		m[n] = true
	}
	return m
}()

// protoField returns the embedded Proto of a modelled object value.
func protoField(v any) (Proto, bool) {
	switch x := v.(type) {
	case *Object:
		return x.Proto, true
	case *Array:
		return x.Proto, true
	case *Map:
		return x.Proto, true
	case *Set:
		return x.Proto, true
	case *Date:
		return x.Proto, true
	case *RegExp:
		return x.Proto, true
	case *Error:
		return x.Proto, true
	case *Function:
		return x.Proto, true
	case *Boxed:
		return x.Proto, true
	case *TypedArray:
		return x.Proto, true
	case *Promise:
		return x.Proto, true
	case *WeakSet:
		return x.Proto, true
	case *WeakMap:
		return x.Proto, true
	case *ArrayBuffer:
		return x.Proto, true
	case *DataView:
		return x.Proto, true
	}
	return Proto{}, false
}

// OwnProps returns a modelled value's own properties.
func OwnProps(v any) []Prop {
	switch x := v.(type) {
	case *Object:
		return x.Props
	case *Array:
		return x.Props
	case *Map:
		return x.Props
	case *Set:
		return x.Props
	case *Date:
		return x.Props
	case *RegExp:
		return x.Props
	case *Error:
		return x.Props
	case *Function:
		return x.Props
	case *Boxed:
		return x.Props
	case *TypedArray:
		return x.Props
	case *Promise:
		return x.Props
	case *WeakSet:
		return x.Props
	case *WeakMap:
		return x.Props
	case *ArrayBuffer:
		return x.Props
	case *DataView:
		return x.Props
	}
	return nil
}

// internalClassName ports V8's GetConstructorName (internal binding): the
// class name V8 records for the object's kind.
func internalClassName(v any) string {
	switch x := v.(type) {
	case *Array:
		return "Array"
	case *Map:
		return "Map"
	case *Set:
		return "Set"
	case *Date:
		return "Date"
	case *RegExp:
		return "RegExp"
	case *Error:
		return "Error"
	case *Function:
		return "Function"
	case *Promise:
		return "Promise"
	case *WeakSet:
		return "WeakSet"
	case *WeakMap:
		return "WeakMap"
	case *ArrayBuffer:
		if x.Shared {
			return "SharedArrayBuffer"
		}
		return "ArrayBuffer"
	case *DataView:
		return "DataView"
	case *TypedArray:
		return typedName(x)
	case *Boxed:
		return boxedType(x.Value)
	}
	return "Object"
}

// protoOf returns [[Prototype]]; ok=false means null.
func protoOf(v any) (*Object, bool) {
	p, has := protoField(v)
	if !has {
		return nil, false
	}
	if p.Prototype != nil {
		return p.Prototype, true
	}
	if p.Null {
		return nil, false
	}
	return defaultProto(v), true
}

func defaultProto(v any) *Object {
	switch x := v.(type) {
	case *Object:
		if x.wellKnown != "" {
			return nil
		}
		return ObjectPrototype
	case *Array:
		return ArrayPrototype
	case *Map:
		return MapPrototype
	case *Set:
		return SetPrototype
	case *Date:
		return DatePrototype
	case *RegExp:
		return RegExpPrototype
	case *Error:
		switch x.Constructor {
		case "RangeError":
			return RangeErrorPrototype
		case "TypeError":
			return TypeErrorPrototype
		case "AggregateError":
			return AggregateErrorPrototype
		}
		return ErrorPrototype
	case *Function:
		return FunctionPrototype
	case *Promise:
		return PromisePrototype
	case *WeakSet:
		return WeakSetPrototype
	case *WeakMap:
		return WeakMapPrototype
	case *ArrayBuffer:
		return ArrayBufferPrototype
	case *DataView:
		return DataViewPrototype
	case *TypedArray:
		return TypedArrayPrototype
	case *Boxed:
		switch x.Value.(type) {
		case float64, int:
			return NumberPrototype
		case string:
			return StringPrototype
		case bool:
			return BooleanPrototype
		}
	}
	return ObjectPrototype
}

// inChain reports whether target appears in v's prototype chain (instanceof).
func inChain(v any, target *Object) bool {
	if target == nil {
		return false
	}
	cur := v
	for i := 0; i < 10000; i++ {
		p, ok := protoOf(cur)
		if !ok || p == nil {
			return false
		}
		if p == target {
			return true
		}
		cur = p
	}
	return false
}

func ownProp(v any, key string) (Prop, bool) {
	for _, p := range OwnProps(v) {
		if p.Key == key {
			return p, true
		}
	}
	return Prop{}, false
}

func ctorFn(v any) (*Function, bool) {
	p, ok := ownProp(v, "constructor")
	if !ok || p.IsAccessor || p.Getter != nil {
		return nil, false
	}
	f, ok := p.Value.(*Function)
	return f, ok
}

// usesChain reports whether a value opted into explicit prototype walking.
func usesChain(v any) bool {
	p, ok := protoField(v)
	return ok && p.Prototype != nil
}

// constructorOf picks the shorthand Proto fast path or the full chain walk.
func constructorOf(c *ctx, v any, p Proto, def string, rt int, protoProps *[]string) *string {
	if p.Prototype == nil {
		return ctorOr(p, def)
	}
	return getConstructorName(c, v, rt, protoProps)
}

func getConstructorName(c *ctx, obj any, recurseTimes int, protoProps *[]string) *string {
	tmp := obj
	var firstProto any
	firstSet := false
	for obj != nil {
		if o, ok := obj.(*Object); ok && o.wellKnown != "" {
			if inChain(tmp, o) {
				if protoProps != nil && (!firstSet || firstProto != any(o)) {
					first := tmp
					if firstSet && firstProto != nil {
						first = firstProto
					}
					addPrototypeProperties(c, tmp, first, recurseTimes, protoProps)
				}
				name := o.wellKnown
				return &name
			}
		}
		if f, ok := ctorFn(obj); ok && f.Name != "" && inChain(tmp, f.PrototypeObject) {
			if protoProps != nil && (!firstSet || firstProto != obj || !builtInObjects[f.Name]) {
				first := tmp
				if firstSet && firstProto != nil {
					first = firstProto
				}
				addPrototypeProperties(c, tmp, first, recurseTimes, protoProps)
			}
			name := f.Name
			return &name
		}
		p, ok := protoOf(obj)
		if ok && p != nil {
			obj = p
		} else {
			obj = nil
		}
		if !firstSet {
			firstSet = true
			if obj != nil {
				firstProto = obj
			}
		}
	}
	if firstProto == nil {
		return nil
	}
	res := internalClassName(tmp)
	if float64(recurseTimes) > c.Depth {
		s := res + " <Complex prototype>"
		return &s
	}
	protoConstr := getConstructorName(c, firstProto, recurseTimes+1, protoProps)
	if protoConstr == nil {
		o := c.Options
		o.CustomInspect = false
		o.Depth = -1
		s := res + " <" + Inspect(firstProto, o) + ">"
		return &s
	}
	s := res + " <" + *protoConstr + ">"
	return &s
}

// ownKeysOrdered ports Reflect.ownKeys order: integer-like keys ascending,
// then string keys in insertion order, then symbols.
func ownKeysOrdered(props []Prop) []Prop {
	var ints, strs, syms []Prop
	for _, p := range props {
		switch k := p.Key.(type) {
		case Symbol:
			syms = append(syms, p)
		case string:
			if numberKeyRe.MatchString(k) && len(k) < 10 {
				ints = append(ints, p)
			} else {
				strs = append(strs, p)
			}
		}
	}
	sortIntKeys(ints)
	return append(append(ints, strs...), syms...)
}

func keyEqual(a, b any) bool {
	if sa, ok := a.(Symbol); ok {
		sb, ok := b.(Symbol)
		return ok && sa.Description == sb.Description
	}
	return a == b
}

func addPrototypeProperties(c *ctx, main any, obj any, recurseTimes int, output *[]string) {
	depth := 0
	var keys []Prop
	var keySet []any
	for {
		if depth != 0 || main == obj {
			p, ok := protoOf(obj)
			if !ok || p == nil {
				return
			}
			obj = p
			if f, ok := ctorFn(obj); ok && builtInObjects[f.Name] {
				return
			}
		}
		if depth == 0 {
			keySet = nil
		} else {
			for _, k := range keys {
				keySet = append(keySet, k.Key)
			}
		}
		keys = ownKeysOrdered(OwnProps(obj))
		c.seen = append(c.seen, main)
		for _, k := range keys {
			if k.Key == "constructor" {
				continue
			}
			if hasOwn(main, k.Key) {
				continue
			}
			if depth != 0 && containsKey(keySet, k.Key) {
				continue
			}
			if _, isFn := k.Value.(*Function); isFn && !k.IsAccessor && k.Getter == nil {
				continue
			}
			value := formatProperty(c, k, recurseTimes, kObjectType)
			if c.Colors {
				*output = append(*output, "\x1b[2m"+value+"\x1b[22m")
			} else {
				*output = append(*output, value)
			}
		}
		c.seen = c.seen[:len(c.seen)-1]
		depth++
		if depth == 3 {
			return
		}
	}
}

func hasOwn(v any, key any) bool {
	for _, p := range OwnProps(v) {
		if keyEqual(p.Key, key) {
			return true
		}
	}
	return false
}

func containsKey(set []any, key any) bool {
	for _, k := range set {
		if keyEqual(k, key) {
			return true
		}
	}
	return false
}

// numberKeyRe is inspect.js numberRegExp (array-index-like keys).
var numberKeyRe = regexp.MustCompile(`^(0|[1-9][0-9]*)$`)

// sortIntKeys orders integer-like keys numerically, as Reflect.ownKeys does.
func sortIntKeys(p []Prop) {
	sort.SliceStable(p, func(i, j int) bool {
		a, _ := strconv.ParseUint(p[i].Key.(string), 10, 64)
		b, _ := strconv.ParseUint(p[j].Key.(string), 10, 64)
		return a < b
	})
}

// typedName is the element-type name of a typed array (V8 class name).
func typedName(t *TypedArray) string {
	if t.Kind == "" {
		return "Uint8Array"
	}
	return t.Kind
}

// errorView adapts a non-native object whose chain includes Error.prototype
// into the Error model, reading name/message/stack the way property access
// would (own property first, then the prototype chain).
func errorView(o *Object) *Error {
	e := &Error{Proto: o.Proto, Props: o.Props, origin: o}
	e.Name = chainString(o, "name", "Error")
	e.Message = chainString(o, "message", "")
	if p, ok := lookup(o, "stack"); ok {
		e.Stack = p.Value
	}
	if p, ok := ownProp(o, "cause"); ok {
		e.HasCause, e.Cause = true, p.Value
	}
	return e
}

// lookup walks own props then the prototype chain for key.
func lookup(v any, key string) (Prop, bool) {
	cur := v
	for i := 0; i < 10000 && cur != nil; i++ {
		if p, ok := ownProp(cur, key); ok {
			return p, true
		}
		if o, ok := cur.(*Object); ok && o.wellKnown != "" {
			if key == "name" && inChain(o, ErrorPrototype) || o == ErrorPrototype {
				if key == "name" {
					return Prop{Key: key, Value: o.wellKnown}, true
				}
				if key == "message" {
					return Prop{Key: key, Value: ""}, true
				}
			}
		}
		p, ok := protoOf(cur)
		if !ok || p == nil {
			break
		}
		cur = p
	}
	return Prop{}, false
}

func chainString(v any, key, def string) string {
	if p, ok := lookup(v, key); ok {
		if s, ok := p.Value.(string); ok {
			return s
		}
	}
	return def
}

// identity returns the JS object a (possibly adapted) value stands for, so
// circular-reference tracking sees one object.
func identity(v any) any {
	if e, ok := v.(*Error); ok && e.origin != nil {
		return e.origin
	}
	return v
}
