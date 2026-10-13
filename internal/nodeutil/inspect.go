// Package nodeutil is a Go translation of Node.js util.inspect / util.format
// (lib/internal/util/inspect.js, Node v25.9.0). The Node.js license is kept
// in LICENSE.
//
// JavaScript values are modelled by the types in values.go. Not translated:
// proxies, custom [util.inspect.custom] hooks, WeakMap/WeakSet/Promise/iterator
// internals (V8-only state), stack-frame colouring, and the RegExp syntax
// highlighter (regexps are coloured as a whole in red, as Node did before v24).
package nodeutil

import (
	"fmt"
	"math"
	"math/big"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"

	"golang.org/x/text/unicode/norm"

	"github.com/gav4you/npmgo/internal/jsnum"
)

// Options mirrors util.inspect options. Use DefaultOptions() as a base.
type Options struct {
	ShowHidden       bool
	Depth            float64 // math.Inf(1) for null/Infinity
	Colors           bool
	MaxArrayLength   float64
	MaxStringLength  float64
	BreakLength      float64
	Compact          int  // ignored when CompactTrue
	CompactTrue      bool // compact: true
	CompactFalse     bool // compact: false
	Sorted           bool
	Getters          bool
	NumericSeparator bool
	ShowProxy        bool
	CustomInspect    bool // honour CustomInspector (default true)
	// GettersMode is "get" or "set" for getters: 'get' / 'set' (Getters stays false).
	GettersMode string
	// SortFunc is a custom `sorted` comparator (Sorted must also be true).
	SortFunc func(a, b string) int
}

// DefaultOptions returns inspectDefaultOptions.
func DefaultOptions() Options {
	return Options{
		Depth:           2,
		MaxArrayLength:  100,
		MaxStringLength: 10000,
		BreakLength:     80,
		Compact:         3,
		CustomInspect:   true,
	}
}

const (
	kObjectType = iota
	kArrayType
	kArrayExtrasType
)

const kMinLineLength = 16

var colors = map[string][2]int{
	"reset": {0, 0}, "bold": {1, 22}, "dim": {2, 22}, "italic": {3, 23},
	"underline": {4, 24}, "blink": {5, 25}, "inverse": {7, 27}, "hidden": {8, 28},
	"strikethrough": {9, 29}, "doubleunderline": {21, 24},
	"black": {30, 39}, "red": {31, 39}, "green": {32, 39}, "yellow": {33, 39},
	"blue": {34, 39}, "magenta": {35, 39}, "cyan": {36, 39}, "white": {37, 39},
	"bgBlack": {40, 49}, "bgRed": {41, 49}, "bgGreen": {42, 49}, "bgYellow": {43, 49},
	"bgBlue": {44, 49}, "bgMagenta": {45, 49}, "bgCyan": {46, 49}, "bgWhite": {47, 49},
	"framed": {51, 54}, "overlined": {53, 55},
	"gray": {90, 39}, "grey": {90, 39}, "blackBright": {90, 39},
	"redBright": {91, 39}, "greenBright": {92, 39}, "yellowBright": {93, 39}, "blueBright": {94, 39},
	"magentaBright": {95, 39}, "cyanBright": {96, 39}, "whiteBright": {97, 39},
	"bgGray": {100, 49}, "bgGrey": {100, 49}, "bgBlackBright": {100, 49},
	"bgRedBright": {101, 49}, "bgGreenBright": {102, 49}, "bgYellowBright": {103, 49},
	"bgBlueBright": {104, 49}, "bgMagentaBright": {105, 49}, "bgCyanBright": {106, 49}, "bgWhiteBright": {107, 49},
	"faint": {2, 22}, "crossedout": {9, 29}, "strikeThrough": {9, 29}, "crossedOut": {9, 29},
	"conceal": {8, 28}, "swapColors": {7, 27}, "swapcolors": {7, 27}, "doubleUnderline": {21, 24},
}

var styles = map[string]string{
	"special": "cyan", "number": "yellow", "bigint": "yellow", "boolean": "yellow",
	"undefined": "grey", "null": "bold", "string": "green", "symbol": "green",
	"date": "magenta", "module": "underline",
}

type ctx struct {
	Options
	budget         map[int]int
	indentationLvl int
	seen           []any
	currentDepth   int
	circular       map[any]int
	circularOrder  int
}

func (c *ctx) compactNum() (int, bool) {
	if c.CompactTrue || c.CompactFalse {
		return 0, false
	}
	return c.Compact, true
}

func (c *ctx) stylize(str, styleType string) string {
	if !c.Colors {
		return str
	}
	if styleType == "regexp" {
		return highlightRegExp(str)
	}
	if style, ok := styles[styleType]; ok {
		if col, ok := colors[style]; ok {
			return fmt.Sprintf("\x1b[%dm%s\x1b[%dm", col[0], str, col[1])
		}
	}
	return str
}

// Inspect ports util.inspect(value, opts).
func Inspect(v any, opts ...Options) string {
	o := DefaultOptions()
	if len(opts) > 0 {
		o = opts[0]
	}
	c := &ctx{Options: o, budget: map[int]int{}}
	return formatValue(c, v, 0, false)
}

func normalize(v any) any {
	switch x := v.(type) {
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case float32:
		return float64(x)
	}
	return v
}

func isPrimitive(v any) bool {
	switch v.(type) {
	case Undefined, bool, float64, string, *big.Int, Symbol:
		return true
	}
	return false
}

func formatValue(c *ctx, value any, recurseTimes int, typedArray bool) string {
	value = normalize(value)
	if isPrimitive(value) {
		return formatPrimitive(c.stylize, value, c)
	}
	if value == nil {
		return c.stylize("null", "null")
	}
	context := value
	isProxy := false
	if p, ok := value.(*Proxy); ok {
		if p.Revoked || p.Target == nil {
			return c.stylize("<Revoked Proxy>", "special")
		}
		if c.ShowProxy {
			return formatProxy(c, p, recurseTimes)
		}
		isProxy = true
		value = normalize(p.Target)
		if isPrimitive(value) {
			return formatPrimitive(c.stylize, value, c)
		}
		// Unwrap nested proxies the way getProxyDetails(value, false) does.
		for {
			inner, ok := value.(*Proxy)
			if !ok {
				break
			}
			if inner.Revoked || inner.Target == nil {
				return c.stylize("<Revoked Proxy>", "special")
			}
			value = inner.Target
		}
	}
	if c.CustomInspect {
		if ci, ok := value.(CustomInspector); ok {
			depth := c.Depth - float64(recurseTimes)
			opts := CustomOptions{Options: c.Options, Stylize: c.stylize}
			_ = isProxy
			ret := ci.InspectCustom(depth, opts, func(v any, o Options) string { return Inspect(v, o) })
			if ret != context && ret != value {
				if s, ok := ret.(string); ok {
					return strings.ReplaceAll(s, "\n", "\n"+strings.Repeat(" ", c.indentationLvl))
				}
				return formatValue(c, ret, recurseTimes, false)
			}
		}
	}
	if m, ok := value.(JSModeler); ok {
		value = m.JSModel()
	}
	for _, s := range c.seen {
		if s == identity(value) {
			idx := 1
			if c.circular == nil {
				c.circular = map[any]int{identity(value): 1}
				c.circularOrder = 1
			} else if i, ok := c.circular[identity(value)]; ok {
				idx = i
			} else {
				c.circularOrder = len(c.circular) + 1
				idx = c.circularOrder
				c.circular[identity(value)] = idx
			}
			return c.stylize(fmt.Sprintf("[Circular *%d]", idx), "special")
		}
	}
	return formatRaw(c, value, recurseTimes, typedArray)
}

func getPrefix(constructor *string, tag, fallback, size string) string {
	if constructor == nil {
		if tag != "" && fallback != tag {
			return "[" + fallback + size + ": null prototype] [" + tag + "] "
		}
		return "[" + fallback + size + ": null prototype] "
	}
	ctor := *constructor
	result := ctor + size + " "
	if tag != "" {
		pos := strings.Index(ctor, tag)
		if pos == -1 {
			result += "[" + tag + "] "
		} else {
			end := pos + len(tag)
			if end != len(ctor) && strings.ToLower(ctor[end:end+1]) == ctor[end:end+1] {
				result += "[" + tag + "] "
			}
		}
	}
	return result
}

func visibleKeys(props []Prop, showHidden bool) []Prop {
	var out []Prop
	var syms []Prop
	for _, p := range props {
		if p.Hidden && !showHidden {
			continue
		}
		if _, ok := p.Key.(Symbol); ok {
			syms = append(syms, p)
		} else {
			out = append(out, p)
		}
	}
	return append(out, syms...)
}

type formatterFn func(c *ctx, recurseTimes int) []string

func formatRaw(c *ctx, value any, recurseTimes int, typedArray bool) string {
	var (
		keys        []Prop
		constructor *string
		tag         string
		base        string
		braces                  = [2]string{"{", "}"}
		extrasType              = kObjectType
		formatter   formatterFn = func(*ctx, int) []string { return nil }
		extraKeys   []Prop
	)
	depthExceeded := func() bool { return float64(recurseTimes) > c.Depth }
	var protoProps []string
	var pp *[]string
	if c.ShowHidden && float64(recurseTimes) <= c.Depth {
		pp = &protoProps
	}
	noProto := func() bool { return len(protoProps) == 0 }

	switch v := value.(type) {
	case *Array:
		constructor = constructorOf(c, v, v.Proto, "Array", recurseTimes, pp)
		tag = v.Tag
		prefix := ""
		if constructor == nil || *constructor != "Array" || tag != "" {
			prefix = getPrefix(constructor, tag, "Array", fmt.Sprintf("(%d)", len(v.Items)))
		}
		keys = visibleKeys(v.Props, c.ShowHidden)
		braces = [2]string{prefix + "[", "]"}
		if len(v.Items) == 0 && len(keys) == 0 && noProto() {
			return braces[0] + "]"
		}
		extrasType = kArrayExtrasType
		formatter = func(c *ctx, rt int) []string { return formatArray(c, v, rt) }
	case *Set:
		constructor = constructorOf(c, v, v.Proto, "Set", recurseTimes, pp)
		tag = v.Tag
		prefix := getPrefix(constructor, tag, "Set", fmt.Sprintf("(%d)", len(v.Items)))
		keys = visibleKeys(v.Props, c.ShowHidden)
		formatter = func(c *ctx, rt int) []string { return formatSet(c, v, rt) }
		if len(v.Items) == 0 && len(keys) == 0 && noProto() {
			return prefix + "{}"
		}
		braces = [2]string{prefix + "{", "}"}
	case *Map:
		constructor = constructorOf(c, v, v.Proto, "Map", recurseTimes, pp)
		tag = v.Tag
		prefix := getPrefix(constructor, tag, "Map", fmt.Sprintf("(%d)", len(v.Keys)))
		keys = visibleKeys(v.Props, c.ShowHidden)
		formatter = func(c *ctx, rt int) []string { return formatMap(c, v, rt) }
		if len(v.Keys) == 0 && len(keys) == 0 && noProto() {
			return prefix + "{}"
		}
		braces = [2]string{prefix + "{", "}"}
	case *TypedArray:
		constructor = constructorOf(c, v, v.Proto, typedName(v), recurseTimes, pp)
		keys = visibleKeys(v.Props, c.ShowHidden)
		fallback := ""
		if constructor == nil {
			fallback = typedName(v)
		}
		size := typedLen(v)
		prefix := getPrefix(constructor, tag, fallback, fmt.Sprintf("(%d)", size))
		braces = [2]string{prefix + "[", "]"}
		if size == 0 && len(keys) == 0 && !c.ShowHidden {
			return braces[0] + "]"
		}
		formatter = func(c *ctx, rt int) []string { return formatTypedArray(c, v) }
		extrasType = kArrayExtrasType
		if c.ShowHidden {
			buf := typedBuffer(v)
			bpe := float64(bytesPerElement(typedName(v)))
			extraKeys = []Prop{
				{Key: "BYTES_PER_ELEMENT", Value: bpe},
				{Key: "length", Value: float64(size)},
				{Key: "byteLength", Value: float64(size) * bpe},
				{Key: "byteOffset", Value: float64(v.ByteOffset)},
				{Key: "buffer", Value: buf},
			}
			typedArray = true
		}
	case *ArrayBuffer:
		def := "ArrayBuffer"
		if v.Shared {
			def = "SharedArrayBuffer"
		}
		constructor = constructorOf(c, v, v.Proto, def, recurseTimes, pp)
		keys = visibleKeys(v.Props, c.ShowHidden)
		prefix := getPrefix(constructor, tag, def, "")
		if !typedArray {
			formatter = func(c *ctx, rt int) []string { return formatArrayBuffer(c, v) }
		} else if len(keys) == 0 && noProto() {
			return prefix + "{ [byteLength]: " + formatNumber(c.stylize, float64(len(v.Bytes)), false) + " }"
		}
		braces[0] = prefix + "{"
		extraKeys = []Prop{{Key: "byteLength", Value: float64(len(v.Bytes))}}
	case *DataView:
		constructor = constructorOf(c, v, v.Proto, "DataView", recurseTimes, pp)
		keys = visibleKeys(v.Props, c.ShowHidden)
		braces[0] = getPrefix(constructor, tag, "DataView", "") + "{"
		extraKeys = []Prop{
			{Key: "byteLength", Value: float64(v.ByteLength)},
			{Key: "byteOffset", Value: float64(v.ByteOffset)},
			{Key: "buffer", Value: v.Buffer},
		}
	case *ModuleNamespace:
		s := "Module"
		constructor = nil
		_ = s
		keys = visibleKeys(v.Props, c.ShowHidden)
		braces[0] = getPrefix(nil, tag, "Module", "") + "{"
		nsKeys := keys
		keys = nil
		formatter = func(c *ctx, rt int) []string { return formatNamespaceObject(c, v, nsKeys, rt) }
	case *URL:
		if float64(recurseTimes) <= c.Depth {
			keys = visibleKeys(v.Props, c.ShowHidden)
			base = v.Href
			if len(keys) == 0 && noProto() {
				return base
			}
		} else {
			// Not isURL-eligible past depth: generic object branch, which
			// returns `URL {}` before the depth check when there are no keys.
			s := "URL"
			constructor = &s
			keys = visibleKeys(v.Props, c.ShowHidden)
			style := getCtxStyle(constructor, tag, "Object")
			if len(keys) == 0 && noProto() {
				return style + "{}"
			}
			braces[0] = style + "{"
		}
	case *External:
		return c.stylize(fmt.Sprintf("[External: %x]", v.Address), "special")
	case *Function:
		def := "Function"
		if v.Generator {
			def = "GeneratorFunction"
		}
		if v.Async {
			def = "Async" + def
		}
		constructor = constructorOf(c, v, v.Proto, def, recurseTimes, pp)
		keys = visibleKeys(v.Props, c.ShowHidden)
		base = getFunctionBase(c, v, constructor, tag)
		if len(keys) == 0 && noProto() {
			return c.stylize(base, "special")
		}
	case *Object:
		// isError(): anything with Error.prototype in its chain is an error.
		if usesChain(v) && inChain(v, ErrorPrototype) {
			return formatRaw(c, errorView(v), recurseTimes, typedArray)
		}
		constructor = constructorOf(c, v, v.Proto, "Object", recurseTimes, pp)
		tag = v.Tag
		keys = visibleKeys(v.Props, c.ShowHidden)
		if constructor != nil && *constructor == "Object" {
			if v.Arguments {
				braces[0] = "[Arguments] {"
			} else if tag != "" {
				braces[0] = getPrefix(constructor, tag, "Object", "") + "{"
			}
			if len(keys) == 0 && noProto() {
				return braces[0] + "}"
			}
		} else {
			style := getCtxStyle(constructor, tag, "Object")
			if len(keys) == 0 && noProto() {
				return style + "{}"
			}
			braces[0] = style + "{"
		}
	case *RegExp:
		constructor = constructorOf(c, v, v.Proto, "RegExp", recurseTimes, pp)
		keys = visibleKeys(v.Props, c.ShowHidden)
		base = "/" + v.Source + "/" + v.Flags
		if prefix := getPrefix(constructor, tag, "RegExp", ""); prefix != "RegExp " {
			base = prefix + base
		}
		base = c.stylize(base, "regexp")
		if (len(keys) == 0 && noProto()) || depthExceeded() {
			return base
		}
	case *Date:
		constructor = constructorOf(c, v, v.Proto, "Date", recurseTimes, pp)
		keys = visibleKeys(v.Props, c.ShowHidden)
		if math.IsNaN(v.Ms) {
			base = "Invalid Date"
		} else {
			base = jsnum.ISOString(v.Ms)
		}
		if prefix := getPrefix(constructor, tag, "Date", ""); prefix != "Date " {
			base = prefix + base
		}
		if len(keys) == 0 && noProto() {
			return c.stylize(base, "date")
		}
	case *Error:
		constructor = constructorOf(c, v, v.Proto, "Error", recurseTimes, pp)
		keys = visibleKeys(v.Props, c.ShowHidden)
		base = formatError(v, constructor, tag, c, &keys)
		if len(keys) == 0 && noProto() {
			return base
		}
	case *Promise:
		constructor = constructorOf(c, v, v.Proto, "Promise", recurseTimes, pp)
		keys = visibleKeys(v.Props, c.ShowHidden)
		braces[0] = getPrefix(constructor, tag, "Promise", "") + "{"
		formatter = func(c *ctx, rt int) []string { return formatPromise(c, v, rt) }
	case *WeakSet:
		constructor = constructorOf(c, v, v.Proto, "WeakSet", recurseTimes, pp)
		keys = visibleKeys(v.Props, c.ShowHidden)
		braces[0] = getPrefix(constructor, tag, "WeakSet", "") + "{"
		if c.ShowHidden {
			formatter = func(c *ctx, rt int) []string { return formatSetIterInner(c, rt, v.Items, kWeak) }
		} else {
			formatter = func(c *ctx, rt int) []string { return []string{c.stylize("<items unknown>", "special")} }
		}
	case *WeakMap:
		constructor = constructorOf(c, v, v.Proto, "WeakMap", recurseTimes, pp)
		keys = visibleKeys(v.Props, c.ShowHidden)
		braces[0] = getPrefix(constructor, tag, "WeakMap", "") + "{"
		if c.ShowHidden {
			flat := make([]any, 0, 2*len(v.Keys))
			for i := range v.Keys {
				flat = append(flat, v.Keys[i], v.Values[i])
			}
			formatter = func(c *ctx, rt int) []string { return formatMapIterInner(c, rt, flat, kWeak) }
		} else {
			formatter = func(c *ctx, rt int) []string { return []string{c.stylize("<items unknown>", "special")} }
		}
	case *MapIterator:
		s := "Map"
		constructor = &s
		tag = v.Tag
		if tag == "" {
			tag = "Map Iterator"
		}
		keys = visibleKeys(v.Props, c.ShowHidden)
		braces = getIteratorBraces("Map", tag)
		formatter = func(c *ctx, rt int) []string { return formatIterator(&braces, c, v.Items, v.Entries, rt) }
	case *SetIterator:
		s := "Set"
		constructor = &s
		tag = v.Tag
		if tag == "" {
			tag = "Set Iterator"
		}
		keys = visibleKeys(v.Props, c.ShowHidden)
		braces = getIteratorBraces("Set", tag)
		formatter = func(c *ctx, rt int) []string { return formatIterator(&braces, c, v.Items, v.Entries, rt) }
	case *Boxed:
		constructor = constructorOf(c, v, v.Proto, boxedType(v.Value), recurseTimes, pp)
		tag = v.Tag
		keys = visibleKeys(v.Props, c.ShowHidden)
		base = getBoxedBase(v, c, keys, constructor, tag)
		if len(keys) == 0 && noProto() {
			return base
		}
	default:
		return fmt.Sprintf("%v", value)
	}

	if depthExceeded() {
		name := getCtxStyle(constructor, tag, fallbackName(value))
		name = name[:len(name)-1]
		if constructor != nil {
			name = "[" + name + "]"
		}
		return c.stylize(name, "special")
	}
	recurseTimes++
	c.seen = append(c.seen, identity(value))
	c.currentDepth = recurseTimes
	output := formatter(c, recurseTimes)
	for _, k := range extraKeys {
		output = append(output, formatExtraProperty(c, k, recurseTimes, typedArray))
	}
	for _, k := range keys {
		output = append(output, formatProperty(c, k, recurseTimes, extrasType))
	}
	output = append(output, protoProps...)
	if c.circular != nil {
		if idx, ok := c.circular[identity(value)]; ok {
			ref := c.stylize(fmt.Sprintf("<ref *%d>", idx), "special")
			if !c.CompactTrue {
				if base == "" {
					base = ref
				} else {
					base = ref + " " + base
				}
			} else {
				braces[0] = ref + " " + braces[0]
			}
		}
	}
	c.seen = c.seen[:len(c.seen)-1]

	if c.Sorted {
		sortOut := func(s []string) {
			if c.SortFunc != nil {
				sort.SliceStable(s, func(i, j int) bool { return c.SortFunc(s[i], s[j]) < 0 })
			} else {
				sortJS(s)
			}
		}
		if extrasType == kObjectType {
			sortOut(output)
		} else if len(keys) > 1 {
			sortOut(output[len(output)-len(keys):])
		}
	}

	res := reduceToSingleString(c, output, base, braces, extrasType, recurseTimes, value)
	newLength := c.budget[c.indentationLvl] + utf16Len(res)
	c.budget[c.indentationLvl] = newLength
	if newLength > 1<<27 {
		c.Depth = -1
	}
	return res
}

func fallbackName(v any) string {
	switch v.(type) {
	case *Array:
		return "Array"
	case *Map:
		return "Map"
	case *Set:
		return "Set"
	case *Function:
		return "Function"
	case *Error:
		return "Error"
	case *Date:
		return "Date"
	case *RegExp:
		return "RegExp"
	}
	return "Object"
}

func getCtxStyle(constructor *string, tag, fallback string) string {
	fb := ""
	if constructor == nil {
		fb = fallback
		if fb == tag {
			fb = "Object"
		}
	}
	return getPrefix(constructor, tag, fb, "")
}

func boxedType(v any) string {
	switch v.(type) {
	case float64, int:
		return "Number"
	case string:
		return "String"
	case bool:
		return "Boolean"
	case *big.Int:
		return "BigInt"
	}
	return "Symbol"
}

func getBoxedBase(v *Boxed, c *ctx, keys []Prop, constructor *string, tag string) string {
	typ := boxedType(v.Value)
	base := "[" + typ
	if constructor == nil {
		base += " (null prototype)"
	} else if *constructor != typ {
		base += " (" + *constructor + ")"
	}
	noColor := func(s, _ string) string { return s }
	base += ": " + formatPrimitive(noColor, normalize(v.Value), c) + "]"
	if tag != "" && (constructor == nil || tag != *constructor) {
		base += " [" + tag + "]"
	}
	if len(keys) != 0 || !c.Colors {
		return base
	}
	return c.stylize(base, strings.ToLower(typ))
}

func getFunctionBase(c *ctx, v *Function, constructor *string, tag string) string {
	if v.Class {
		name := v.Name
		if name == "" {
			name = "(anonymous)"
		}
		base := "class " + name
		if constructor != nil && *constructor != "Function" {
			base += " [" + *constructor + "]"
		}
		if tag != "" && (constructor == nil || *constructor != tag) {
			base += " [" + tag + "]"
		}
		if constructor != nil {
			if v.Extends != "" {
				base += " extends " + v.Extends
			}
		} else {
			base += " extends [null prototype]"
		}
		return "[" + base + "]"
	}
	typ := "Function"
	if v.Generator {
		typ = "Generator" + typ
	}
	if v.Async {
		typ = "Async" + typ
	}
	base := "[" + typ
	if constructor == nil {
		base += " (null prototype)"
	}
	if v.Name == "" {
		base += " (anonymous)"
	} else {
		base += ": " + v.Name
	}
	base += "]"
	if constructor != nil && *constructor != typ {
		base += " " + *constructor
	}
	if tag != "" && (constructor == nil || *constructor != tag) {
		base += " [" + tag + "]"
	}
	return base
}

func errorToString(e *Error) string {
	name, msg := e.Name, e.Message
	if name == "" {
		return msg
	}
	if msg == "" {
		return name
	}
	return name + ": " + msg
}

func formatError(e *Error, constructor *string, tag string, c *ctx, keys *[]Prop) string {
	var stack string
	switch s := e.Stack.(type) {
	case string:
		if s != "" {
			stack = s
		} else {
			stack = errorToString(e)
		}
	case nil:
		stack = errorToString(e)
	default:
		c.seen = append(c.seen, e)
		c.indentationLvl += 4
		r := formatValue(c, s, 0, false)
		c.indentationLvl -= 4
		c.seen = c.seen[:len(c.seen)-1]
		stack = errorToString(e) + "\n    " + r
	}
	name := e.Name
	if !c.ShowHidden && len(*keys) != 0 {
		filtered := (*keys)[:0:0]
		for _, k := range *keys {
			ks, _ := k.Key.(string)
			if ks == "stack" {
				continue
			}
			if ks == "message" {
				if s, ok := k.Value.(string); !ok || strings.Contains(stack, s) {
					continue
				}
			}
			if ks == "name" {
				if s, ok := k.Value.(string); !ok || strings.Contains(stack, s) {
					continue
				}
			}
			filtered = append(filtered, k)
		}
		*keys = filtered
	}
	if name == "" {
		name = "Error"
	}
	for _, p := range e.Props {
		if p.Key == "errors" {
			if _, isArr := p.Value.(*Array); isArr {
				has := false
				for _, k := range *keys {
					if k.Key == "errors" {
						has = true
					}
				}
				if !has {
					defer func(p Prop) { *keys = append(*keys, p) }(p)
				}
			}
		}
	}
	if e.HasCause {
		has := false
		for _, k := range *keys {
			if k.Key == "cause" {
				has = true
			}
		}
		if !has {
			*keys = append(*keys, Prop{Key: "cause", Value: e.Cause, Hidden: true})
		}
	}
	stack = improveStack(stack, constructor, name, tag)

	pos := -1
	if e.Message != "" {
		pos = strings.Index(stack, e.Message)
		if pos == 0 {
			pos = -1 // JS: (0 || -1)
		}
	}
	if pos != -1 {
		pos += len(e.Message)
	}
	from := pos
	if from < 0 {
		from = 0
	}
	stackStart := -1
	if i := strings.Index(stack[from:], "\n    at"); i != -1 {
		stackStart = from + i
	}
	if stackStart == -1 {
		stack = "[" + stack + "]"
	} else {
		newStack := stack[:stackStart]
		lines := getStackFrames(c, e, stack[stackStart+1:])
		if c.Colors {
			wd, wdErr := Cwd()
			esm := ""
			for _, line := range lines {
				if m := coreModuleRe.FindStringSubmatch(line); m != nil && isBuiltinModule(m[1]) {
					newStack += "\n" + c.stylize(line, "undefined")
					continue
				}
				newStack += "\n"
				line = markNodeModules(c, line)
				if wdErr == nil {
					nl := markCwd(c, line, wd)
					if nl == line {
						if esm == "" {
							esm = pathToFileURLHref(wd)
						}
						nl = markCwd(c, line, esm)
					}
					line = nl
				}
				newStack += line
			}
		} else {
			newStack += "\n" + strings.Join(lines, "\n")
		}
		stack = newStack
	}
	if c.indentationLvl != 0 {
		stack = strings.ReplaceAll(stack, "\n", "\n"+strings.Repeat(" ", c.indentationLvl))
	}
	return stack
}

var errStartRe = regexp.MustCompile(`^([A-Z][a-z_ A-Z0-9[\]()-]+)(?::|\n {4}at)`)
var errOnlyRe = regexp.MustCompile(`^([a-z_A-Z0-9-]*Error)$`)

func improveStack(stack string, constructor *string, name, tag string) string {
	l := len(name)
	if constructor == nil || (strings.HasSuffix(name, "Error") && strings.HasPrefix(stack, name) &&
		(len(stack) == l || stack[l] == ':' || stack[l] == '\n')) {
		fallback := "Error"
		if constructor == nil {
			fallback = ""
			if m := errStartRe.FindStringSubmatch(stack); m != nil {
				fallback = m[1]
			} else if m := errOnlyRe.FindStringSubmatch(stack); m != nil {
				fallback = m[1]
			}
			l = len(fallback)
			if fallback == "" {
				fallback = "Error"
			}
		}
		prefix := getPrefix(constructor, tag, fallback, "")
		prefix = prefix[:len(prefix)-1]
		if name != prefix {
			if strings.Contains(prefix, name) {
				if l == 0 {
					stack = prefix + ": " + stack
				} else {
					stack = prefix + stack[l:]
				}
			} else {
				stack = prefix + " [" + name + "]" + stack[l:]
			}
		}
	}
	return stack
}

func remainingText(n int) string {
	s := ""
	if n > 1 {
		s = "s"
	}
	return fmt.Sprintf("... %d more item%s", n, s)
}

func maxLen(c *ctx, n int) int {
	m := math.Min(math.Max(0, c.MaxArrayLength), float64(n))
	return int(m)
}

func formatArray(c *ctx, v *Array, rt int) []string {
	valLen := len(v.Items)
	l := maxLen(c, valLen)
	var output []string
	for i := 0; i < l; i++ {
		if _, hole := v.Items[i].(Hole); hole {
			return formatSpecialArray(c, v, rt, l, output, i)
		}
		output = append(output, formatProperty(c, Prop{Key: strconv.Itoa(i), Value: v.Items[i]}, rt, kArrayType))
	}
	if valLen-l > 0 {
		output = append(output, remainingText(valLen-l))
	}
	return output
}

func formatSpecialArray(c *ctx, v *Array, rt, maxLength int, output []string, i int) []string {
	index := i
	for ; i < len(v.Items) && len(output) < maxLength; i++ {
		if _, hole := v.Items[i].(Hole); hole {
			continue
		}
		if i != index {
			empty := i - index
			s := ""
			if empty > 1 {
				s = "s"
			}
			output = append(output, c.stylize(fmt.Sprintf("<%d empty item%s>", empty, s), "undefined"))
			index = i
			if len(output) == maxLength {
				break
			}
		}
		output = append(output, formatProperty(c, Prop{Key: strconv.Itoa(i), Value: v.Items[i]}, rt, kArrayType))
		index++
	}
	remaining := len(v.Items) - index
	if len(output) != maxLength {
		if remaining > 0 {
			s := ""
			if remaining > 1 {
				s = "s"
			}
			output = append(output, c.stylize(fmt.Sprintf("<%d empty item%s>", remaining, s), "undefined"))
		}
	} else if remaining > 0 {
		output = append(output, remainingText(remaining))
	}
	return output
}

func formatTypedArray(c *ctx, v *TypedArray) []string {
	n := typedLen(v)
	m := maxLen(c, n)
	out := make([]string, 0, m+1)
	for i := 0; i < m; i++ {
		if v.BigItems != nil {
			out = append(out, formatBigInt(c.stylize, v.BigItems[i], c.NumericSeparator))
		} else {
			out = append(out, formatNumber(c.stylize, v.Items[i], c.NumericSeparator))
		}
	}
	if n-m > 0 {
		out = append(out, remainingText(n-m))
	}
	return out
}

func formatSet(c *ctx, v *Set, rt int) []string {
	m := maxLen(c, len(v.Items))
	var out []string
	c.indentationLvl += 2
	for i := 0; i < m; i++ {
		out = append(out, formatValue(c, v.Items[i], rt, false))
	}
	if len(v.Items)-m > 0 {
		out = append(out, remainingText(len(v.Items)-m))
	}
	c.indentationLvl -= 2
	return out
}

func formatMap(c *ctx, v *Map, rt int) []string {
	m := maxLen(c, len(v.Keys))
	var out []string
	c.indentationLvl += 2
	for i := 0; i < m; i++ {
		out = append(out, formatValue(c, v.Keys[i], rt, false)+" => "+formatValue(c, v.Values[i], rt, false))
	}
	if len(v.Keys)-m > 0 {
		out = append(out, remainingText(len(v.Keys)-m))
	}
	c.indentationLvl -= 2
	return out
}

var keyStrRe = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z_0-9]*$`)

func formatProperty(c *ctx, p Prop, rt int, typ int) string {
	var str string
	extra := " "
	switch {
	case !p.IsAccessor && p.Getter == nil:
		diff := 2
		if c.CompactTrue && typ == kObjectType {
			diff = 3
		}
		c.indentationLvl += diff
		if _, u := p.Value.(Undefined); u {
			str = c.stylize("undefined", "undefined")
		} else {
			str = formatValue(c, p.Value, rt, false)
		}
		if diff == 3 && int(c.BreakLength) < getStringWidth(str, c.Colors) {
			extra = "\n" + strings.Repeat(" ", c.indentationLvl)
		}
		c.indentationLvl -= diff
	case p.Getter != nil:
		label := "Getter"
		if p.HasSetter {
			label = "Getter/Setter"
		}
		if c.Getters || (c.GettersMode == "get" && !p.HasSetter) || (c.GettersMode == "set" && p.HasSetter) {
			c.indentationLvl += 2
			tmp, err := p.Getter()
			tmp = normalize(tmp)
			switch {
			case err != nil:
				msg := "<Inspection threw (" + formatValue(c, &Error{Name: "Error", Message: err.Error()}, rt, false) + ")>"
				str = c.stylize("["+label+":", "special") + " " + msg + c.stylize("]", "special")
			case tmp == nil:
				str = c.stylize("["+label+":", "special") + " " + c.stylize("null", "null") + c.stylize("]", "special")
			case !isPrimitive(tmp):
				str = c.stylize("["+label+"]", "special") + " " + formatValue(c, tmp, rt, false)
			default:
				str = c.stylize("["+label+":", "special") + " " + formatPrimitive(c.stylize, tmp, c) + c.stylize("]", "special")
			}
			c.indentationLvl -= 2
		} else {
			str = c.stylize("["+label+"]", "special")
		}
	case p.HasSetter:
		str = c.stylize("[Setter]", "special")
	default:
		str = c.stylize("undefined", "undefined")
	}
	if typ == kArrayType {
		return str
	}
	var name string
	switch k := p.Key.(type) {
	case Symbol:
		name = c.stylize(escapeAll(k.String()), "symbol")
	case string:
		if keyStrRe.MatchString(k) {
			if k == "__proto__" {
				name = "['__proto__']"
			} else {
				name = c.stylize(k, "name")
			}
		} else {
			name = c.stylize(strEscape(k), "string")
		}
	}
	if p.Hidden {
		name = "[" + name + "]"
	}
	return name + ":" + extra + str
}

func isBelowBreakLength(c *ctx, output []string, start int, base string) bool {
	total := len(output) + start
	if total+len(output) > int(c.BreakLength) {
		return false
	}
	for _, o := range output {
		if c.Colors {
			total += utf16Len(removeColors(o))
		} else {
			total += utf16Len(o)
		}
		if total > int(c.BreakLength) {
			return false
		}
	}
	return base == "" || !strings.Contains(base, "\n")
}

func reduceToSingleString(c *ctx, output []string, base string, braces [2]string, extrasType, rt int, value any) string {
	if !c.CompactTrue {
		if compact, ok := c.compactNum(); ok && compact >= 1 {
			entries := len(output)
			if extrasType == kArrayExtrasType && entries > 6 {
				output = groupArrayElements(c, output, value)
			}
			if c.currentDepth-rt < compact && entries == len(output) {
				start := len(output) + c.indentationLvl + utf16Len(braces[0]) + utf16Len(base) + 10
				if isBelowBreakLength(c, output, start, base) {
					joined := strings.Join(output, ", ")
					if !strings.Contains(joined, "\n") {
						b := ""
						if base != "" {
							b = base + " "
						}
						return b + braces[0] + " " + joined + " " + braces[1]
					}
				}
			}
		}
		ind := "\n" + strings.Repeat(" ", c.indentationLvl)
		b := ""
		if base != "" {
			b = base + " "
		}
		return b + braces[0] + ind + "  " + strings.Join(output, ","+ind+"  ") + ind + braces[1]
	}
	if isBelowBreakLength(c, output, 0, base) {
		b := ""
		if base != "" {
			b = " " + base
		}
		return braces[0] + b + " " + strings.Join(output, ", ") + " " + braces[1]
	}
	ind := strings.Repeat(" ", c.indentationLvl)
	ln := " "
	if !(base == "" && utf16Len(braces[0]) == 1) {
		b := ""
		if base != "" {
			b = " " + base
		}
		ln = b + "\n" + ind + "  "
	}
	return braces[0] + ln + strings.Join(output, ",\n"+ind+"  ") + " " + braces[1]
}

func groupArrayElements(c *ctx, output []string, value any) []string {
	totalLength, maxLength := 0, 0
	outputLength := len(output)
	if c.MaxArrayLength < float64(len(output)) {
		outputLength--
	}
	const sep = 2
	dataLen := make([]int, outputLength)
	for i := 0; i < outputLength; i++ {
		l := getStringWidth(output[i], c.Colors)
		dataLen[i] = l
		totalLength += l + sep
		if maxLength < l {
			maxLength = l
		}
	}
	actualMax := maxLength + sep
	if float64(actualMax*3+c.indentationLvl) < c.BreakLength &&
		(float64(totalLength)/float64(actualMax) > 5 || maxLength <= 6) {
		averageBias := math.Sqrt(float64(actualMax) - float64(totalLength)/float64(len(output)))
		biasedMax := math.Max(float64(actualMax)-3-averageBias, 1)
		compact, _ := c.compactNum()
		columns := math.Min(math.Min(
			jsnum.Round(math.Sqrt(2.5*biasedMax*float64(outputLength))/biasedMax),
			math.Floor((c.BreakLength-float64(c.indentationLvl))/float64(actualMax))),
			math.Min(float64(compact*4), 15))
		if columns <= 1 {
			return output
		}
		cols := int(columns)
		var tmp []string
		maxLineLength := make([]int, cols)
		for i := 0; i < cols; i++ {
			lineMax := 0
			for j := i; j < len(output); j += cols {
				if j < len(dataLen) && dataLen[j] > lineMax {
					lineMax = dataLen[j]
				}
			}
			maxLineLength[i] = lineMax + sep
		}
		padStart := true
		if value != nil {
			padStart = allNumeric(value, len(output))
		}
		for i := 0; i < outputLength; i += cols {
			max := i + cols
			if outputLength < max {
				max = outputLength
			}
			var str strings.Builder
			j := i
			for ; j < max-1; j++ {
				padding := maxLineLength[j-i] + utf16Len(output[j]) - dataLen[j]
				str.WriteString(pad(output[j]+", ", padding, padStart))
			}
			if padStart {
				padding := maxLineLength[j-i] + utf16Len(output[j]) - dataLen[j] - sep
				str.WriteString(pad(output[j], padding, true))
			} else {
				str.WriteString(output[j])
			}
			tmp = append(tmp, str.String())
		}
		if c.MaxArrayLength < float64(len(output)) {
			tmp = append(tmp, output[outputLength])
		}
		output = tmp
	}
	return output
}

// allNumeric reports whether value[0..n) are all numbers (or bigints), like
// the `typeof value[i] !== 'number'` scan in groupArrayElements.
func allNumeric(value any, n int) bool {
	switch v := value.(type) {
	case *TypedArray:
		return true
	case *Array:
		for i := 0; i < n; i++ {
			if i >= len(v.Items) {
				return false
			}
			switch normalize(v.Items[i]).(type) {
			case float64, *big.Int:
			default:
				return false
			}
		}
		return true
	}
	return false
}

func pad(s string, width int, start bool) string {
	n := width - utf16Len(s)
	if n <= 0 {
		return s
	}
	if start {
		return strings.Repeat(" ", n) + s
	}
	return s + strings.Repeat(" ", n)
}

func addNumericSeparator(s string) string {
	result := ""
	i := len(s)
	start := 0
	if s[0] == '-' {
		start = 1
	}
	for ; i >= start+4; i -= 3 {
		result = "_" + s[i-3:i] + result
	}
	if i == len(s) {
		return s
	}
	return s[:i] + result
}

func addNumericSeparatorEnd(s string) string {
	result := ""
	i := 0
	for ; i < len(s)-3; i += 3 {
		result += s[i:i+3] + "_"
	}
	if i == 0 {
		return s
	}
	return result + s[i:]
}

func formatNumber(fn func(string, string) string, n float64, sep bool) string {
	if !sep {
		if n == 0 && math.Signbit(n) {
			return fn("-0", "number")
		}
		return fn(jsnum.String(n), "number")
	}
	s := jsnum.String(n)
	if math.Trunc(n) == n {
		if math.IsInf(n, 0) || strings.Contains(s, "e") {
			return fn(s, "number")
		}
		return fn(addNumericSeparator(s), "number")
	}
	if math.IsNaN(n) {
		return fn(s, "number")
	}
	dot := strings.Index(s, ".")
	if dot == -1 {
		return fn(s, "number")
	}
	return fn(addNumericSeparator(s[:dot])+"."+addNumericSeparatorEnd(s[dot+1:]), "number")
}

func formatBigInt(fn func(string, string) string, b *big.Int, sep bool) string {
	s := b.String()
	if !sep {
		return fn(s+"n", "bigint")
	}
	return fn(addNumericSeparator(s)+"n", "bigint")
}

var lineSplit = regexp.MustCompile(`\n`)

func formatPrimitive(fn func(string, string) string, value any, c *ctx) string {
	switch v := value.(type) {
	case string:
		trailer := ""
		u := utf16.Encode([]rune(v))
		if float64(len(u)) > c.MaxStringLength {
			remaining := len(u) - int(c.MaxStringLength)
			v = string(utf16.Decode(u[:int(c.MaxStringLength)]))
			u = u[:int(c.MaxStringLength)]
			s := ""
			if remaining > 1 {
				s = "s"
			}
			trailer = fmt.Sprintf("... %d more character%s", remaining, s)
		}
		if !c.CompactTrue && len(u) > kMinLineLength && float64(len(u)) > c.BreakLength-float64(c.indentationLvl)-4 {
			// split on /(?<=\n)/ : keep the newline at the end of each piece
			var parts []string
			rest := v
			for {
				i := strings.IndexByte(rest, '\n')
				if i == -1 {
					parts = append(parts, rest)
					break
				}
				parts = append(parts, rest[:i+1])
				rest = rest[i+1:]
				if rest == "" {
					break
				}
			}
			for i, p := range parts {
				parts[i] = fn(strEscape(p), "string")
			}
			return strings.Join(parts, " +\n"+strings.Repeat(" ", c.indentationLvl+2)) + trailer
		}
		return fn(strEscape(v), "string") + trailer
	case float64:
		return formatNumber(fn, v, c.NumericSeparator)
	case *big.Int:
		return formatBigInt(fn, v, c.NumericSeparator)
	case bool:
		return fn(strconv.FormatBool(v), "boolean")
	case Undefined:
		return fn("undefined", "undefined")
	case Symbol:
		return fn(v.String(), "symbol")
	}
	return fmt.Sprint(value)
}

var meta = func() [160]string {
	var m [160]string
	for i := 0; i < 32; i++ {
		m[i] = fmt.Sprintf(`\x%02X`, i)
	}
	m['\b'], m['\t'], m['\n'], m['\f'], m['\r'] = `\b`, `\t`, `\n`, `\f`, `\r`
	m['\''] = `\'`
	m['\\'] = `\\`
	for i := 0x7f; i < 0xa0; i++ {
		m[i] = fmt.Sprintf(`\x%02X`, i)
	}
	return m
}()

func escapeUnit(u uint16) string {
	if int(u) < len(meta) {
		return meta[u]
	}
	return `\u` + strconv.FormatInt(int64(u), 16)
}

// escapeAll applies strEscapeSequencesReplacer (used for symbol keys).
func escapeAll(s string) string {
	return escapeWith(utf16.Encode([]rune(s)), 39)
}

func escapeWith(u []uint16, singleQuote int) string {
	var b []uint16
	last := 0
	for i := 0; i < len(u); i++ {
		p := int(u[i])
		if p == singleQuote || p == 92 || p < 32 || (p > 126 && p < 160) {
			b = append(b, u[last:i]...)
			b = append(b, utf16.Encode([]rune(meta[p]))...)
			last = i + 1
		} else if p >= 0xd800 && p <= 0xdfff {
			if p <= 0xdbff && i+1 < len(u) {
				q := int(u[i+1])
				if q >= 0xdc00 && q <= 0xdfff {
					i++
					continue
				}
			}
			b = append(b, u[last:i]...)
			b = append(b, utf16.Encode([]rune(escapeUnit(uint16(p))))...)
			last = i + 1
		}
	}
	b = append(b, u[last:]...)
	return string(utf16.Decode(b))
}

func strEscape(str string) string {
	singleQuote := 39
	if strings.Contains(str, "'") {
		if !strings.Contains(str, `"`) {
			singleQuote = -1
		} else if !strings.Contains(str, "`") && !strings.Contains(str, "${") {
			singleQuote = -2
		}
	}
	// Go strings cannot hold lone surrogates from JS; invalid UTF-8 is mapped
	// to U+FFFD by the conversion below, matching JS for well-formed input.
	escaped := escapeWith(utf16.Encode([]rune(str)), singleQuote)
	switch singleQuote {
	case -1:
		return `"` + escaped + `"`
	case -2:
		return "`" + escaped + "`"
	}
	return "'" + escaped + "'"
}

var ansiRe = regexp.MustCompile(`[\x{1B}\x{9B}][[\]()#;?]*` +
	`(?:(?:(?:(?:;[-a-zA-Z\d/#&.:=?%@~_]+)*` +
	`|[a-zA-Z\d]+(?:;[-a-zA-Z\d/#&.:=?%@~_]*)*)?` +
	`(?:\x{07}|\x{1B}\\|\x{9C}))` +
	`|(?:(?:\d{1,4}(?:;\d{0,4})*)?` +
	`[\dA-PR-TZcf-nq-uy=><~]))`)

var colorRe = regexp.MustCompile("\u001b\\[\\d\\d?m")

func removeColors(s string) string { return colorRe.ReplaceAllString(s, "") }

// StripVTControlCharacters ports util.stripVTControlCharacters.
func StripVTControlCharacters(s string) string {
	if !strings.ContainsAny(s, "\u001B\u009B") {
		return s
	}
	return ansiRe.ReplaceAllString(s, "")
}

func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		if r >= 0x10000 {
			n += 2
		} else {
			n++
		}
	}
	return n
}

func isFullWidth(code rune) bool {
	return code >= 0x1100 && (code <= 0x115f || code == 0x2329 || code == 0x232a ||
		(code >= 0x2e80 && code <= 0x3247 && code != 0x303f) ||
		(code >= 0x3250 && code <= 0x4dbf) || (code >= 0x4e00 && code <= 0xa4c6) ||
		(code >= 0xa960 && code <= 0xa97c) || (code >= 0xac00 && code <= 0xd7a3) ||
		(code >= 0xf900 && code <= 0xfaff) || (code >= 0xfe10 && code <= 0xfe19) ||
		(code >= 0xfe30 && code <= 0xfe6b) || (code >= 0xff01 && code <= 0xff60) ||
		(code >= 0xffe0 && code <= 0xffe6) || (code >= 0x1b000 && code <= 0x1b001) ||
		(code >= 0x1f200 && code <= 0x1f251) || (code >= 0x1f300 && code <= 0x1f64f) ||
		(code >= 0x20000 && code <= 0x3fffd))
}

func isZeroWidth(code rune) bool {
	return code <= 0x1F || (code >= 0x7F && code <= 0x9F) || (code >= 0x300 && code <= 0x36F) ||
		(code >= 0x200B && code <= 0x200F) || (code >= 0x20D0 && code <= 0x20FF) ||
		(code >= 0xFE00 && code <= 0xFE0F) || (code >= 0xFE20 && code <= 0xFE2F) ||
		(code >= 0xE0100 && code <= 0xE01EF)
}

// getStringWidth ports the ICU build of getStringWidth (official Node builds):
// an ASCII fast path, then ICU column widths over the NFC-normalised rest.
func getStringWidth(s string, removeControlChars bool) int {
	if removeControlChars {
		s = StripVTControlCharacters(s)
	}
	w := 0
	for i := 0; i < len(s); i++ {
		code := s[i]
		if code >= 127 {
			w += icuStringWidth(norm.NFC.String(s[i:]))
			break
		}
		if code >= 32 {
			w++
		}
	}
	return w
}

// icuStringWidth ports node_i18n.cc GetStringWidth with its defaults
// (ambiguous_as_full_width=false, expand_emoji_sequence=true).
func icuStringWidth(s string) int {
	w := 0
	for _, r := range s {
		w += icuColumnWidth(r)
	}
	return w
}

// icuColumnWidth ports GetColumnWidth via the table generated from Node's ICU.
func icuColumnWidth(r rune) int {
	if r < 0x80 {
		if r < 0x20 || r == 0x7f {
			return 0 // Cc
		}
		return 1
	}
	lo, hi := 0, len(icuWidthRanges)-1
	for lo <= hi {
		m := (lo + hi) / 2
		switch {
		case r < icuWidthRanges[m].lo:
			hi = m - 1
		case r > icuWidthRanges[m].hi:
			lo = m + 1
		default:
			return int(icuWidthRanges[m].w)
		}
	}
	return 1
}

// isFullWidth / isZeroWidth port the non-ICU helpers that util exports
// (isZeroWidthCodePoint is public in inspect.js's exports).
var _ = isFullWidth

// IsZeroWidthCodePoint ports the exported isZeroWidthCodePoint.
func IsZeroWidthCodePoint(code rune) bool { return isZeroWidth(code) }

// GetStringWidth ports the exported getStringWidth(str, removeControlChars = true).
func GetStringWidth(s string, removeControlChars ...bool) int {
	rc := true
	if len(removeControlChars) > 0 {
		rc = removeControlChars[0]
	}
	return getStringWidth(s, rc)
}
