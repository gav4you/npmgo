package nodeutil

import (
	"bytes"
	"encoding/json"
	"math"
	"math/big"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/gav4you/npmgo/internal/jsnum"
)

// ToNumber ports JavaScript Number(v) for the modelled value types.
func ToNumber(v any) float64 {
	switch x := normalize(v).(type) {
	case float64:
		return x
	case bool:
		if x {
			return 1
		}
		return 0
	case nil:
		return 0
	case string:
		return jsnum.Number(x)
	case *big.Int:
		f, _ := new(big.Float).SetInt(x).Float64()
		return f
	case *Date:
		return x.Ms
	case *Boxed: // valueOf
		if _, isSym := x.Value.(Symbol); isSym {
			return math.NaN() // Node throws: Cannot convert a Symbol value to a number
		}
		return ToNumber(x.Value)
	case *Array: // ToPrimitive: Array.prototype.toString, i.e. join(',')
		return jsnum.Number(joinItems(x.Items))
	}
	return math.NaN()
}

// jsString ports String(v) for the modelled values (ToString with the
// built-in toString methods; user toString functions cannot run).
func jsString(v any) string {
	switch x := normalize(v).(type) {
	case nil:
		return "null"
	case Undefined, Hole:
		return "undefined"
	case string:
		return x
	case bool:
		if x {
			return "true"
		}
		return "false"
	case float64:
		return jsnum.String(x)
	case *big.Int:
		return x.String()
	case Symbol:
		return x.String()
	case *Array:
		return joinItems(x.Items)
	case *TypedArray:
		if x.BigItems != nil {
			items := make([]any, len(x.BigItems))
			for i, b := range x.BigItems {
				items[i] = b
			}
			return joinItems(items)
		}
		items := make([]any, len(x.Items))
		for i, f := range x.Items {
			items[i] = f
		}
		return joinItems(items)
	case *Boxed:
		return jsString(x.Value)
	case *Error:
		return errorToString(x)
	case *Function:
		return x.Source
	case *Date:
		return dateToString(x)
	case *RegExp:
		return "/" + x.Source + "/" + x.Flags
	case *URL:
		return x.Href
	case *URLSearchParams:
		return x.serialize()
	case *Proxy:
		if x.Revoked || x.Target == nil {
			return "[object Object]"
		}
		if _, isFn := x.Target.(*Function); isFn {
			return "function () { [native code] }" // Function.prototype.toString on a proxy
		}
		return jsString(x.Target)
	}
	return "[object Object]"
}

// joinItems ports Array.prototype.join with the default separator.
func joinItems(items []any) string {
	parts := make([]string, len(items))
	for i, it := range items {
		switch it.(type) {
		case nil, Undefined, Hole:
		default:
			parts[i] = jsString(it)
		}
	}
	return strings.Join(parts, ",")
}

// dateToString ports Date.prototype.toString: the local time with Go's zone
// abbreviation in parentheses, where V8 prints ICU's long zone name.
func dateToString(d *Date) string {
	if math.IsNaN(d.Ms) {
		return "Invalid Date"
	}
	t := time.UnixMilli(int64(d.Ms))
	return t.Format("Mon Jan 02 2006 15:04:05 GMT-0700 (MST)")
}

// hasBuiltInToString ports hasBuiltInToString: false when the value or a
// prototype before a built-in one defines toString / Symbol.toPrimitive, in
// which case util.format('%s') calls String(value) instead of inspecting it.
func hasBuiltInToString(v any) bool {
	if p, ok := v.(*Proxy); ok {
		if p.Revoked || p.Target == nil {
			return true
		}
		v = normalize(p.Target)
	}
	switch v.(type) {
	case *URL, *URLSearchParams, *TypedArray:
		// URL.prototype, URLSearchParams.prototype and %TypedArray%.prototype
		// define toString, and their constructors (URL, URLSearchParams,
		// TypedArray) are not in builtInObjects.
		return false
	}
	cur := v
	for depth := 0; depth < 10000; depth++ {
		if hasOwnFn(cur, "toString") || hasOwnSym(cur, "Symbol.toPrimitive") {
			if depth == 0 {
				return false
			}
			f, ok := ctorFn(cur)
			return ok && builtInObjects[f.Name]
		}
		if o, ok := cur.(*Object); ok && o.wellKnown != "" {
			// A built-in prototype: its toString (or Object.prototype's) is built in.
			return true
		}
		if _, ok := cur.(*ModuleNamespace); ok {
			return true // null prototype, no toString at all
		}
		p, ok := protoOf(cur)
		if !ok || p == nil {
			return true // no toString in the chain at all
		}
		cur = p
	}
	return true
}

func hasOwnFn(v any, key string) bool {
	for _, p := range OwnProps(v) {
		if p.Key == key {
			_, isFn := p.Value.(*Function)
			return isFn
		}
	}
	return false
}

func hasOwnSym(v any, description string) bool {
	for _, p := range OwnProps(v) {
		if s, ok := p.Key.(Symbol); ok && s.Description != nil && *s.Description == description {
			return true
		}
	}
	return false
}

// jsonStringify ports JSON.stringify for the modelled value types, returning
// ok=false for top-level undefined/functions/symbols (JS returns undefined).
func jsonStringify(v any, seen []any) (string, bool, bool) {
	v = normalize(v)
	switch x := v.(type) {
	case nil:
		return "null", true, false
	case Undefined, Hole, Symbol, *Function:
		return "", false, false
	case bool:
		return jsString(x), true, false
	case float64:
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return "null", true, false
		}
		return jsnum.String(x), true, false
	case string:
		return jsonQuote(x), true, false
	case *big.Int:
		panic("TypeError: Do not know how to serialize a BigInt")
	case *Date:
		if math.IsNaN(x.Ms) {
			return "null", true, false
		}
		return `"` + jsnum.ISOString(x.Ms) + `"`, true, false
	}
	for _, s := range seen {
		if sameRef(s, v) {
			return "", false, true
		}
	}
	seen = append(seen, v)
	switch x := v.(type) {
	case *Array:
		parts := make([]string, len(x.Items))
		for i, it := range x.Items {
			s, ok, circ := jsonStringify(it, seen)
			if circ {
				return "", false, true
			}
			if !ok {
				s = "null"
			}
			parts[i] = s
		}
		return "[" + strings.Join(parts, ",") + "]", true, false
	case *TypedArray:
		parts := make([]string, len(x.Items))
		for i, it := range x.Items {
			parts[i] = `"` + jsnum.String(float64(i)) + `":` + jsnum.String(it)
		}
		return "{" + strings.Join(parts, ",") + "}", true, false
	}
	var props []Prop
	switch x := v.(type) {
	case *Object:
		props = x.Props
	case *Error:
		props = x.Props
	case *Map:
		props = x.Props
	case *Set:
		props = x.Props
	case *RegExp:
		props = x.Props
	case *ModuleNamespace:
		props = x.Props
	case *Boxed:
		switch b := x.Value.(type) {
		case string, float64, bool, int:
			return jsonStringify(b, seen)
		}
	}
	var parts []string
	for _, p := range ownKeysOrdered(props) {
		k, isStr := p.Key.(string)
		if !isStr || p.Hidden {
			continue
		}
		val := p.Value
		if p.Getter != nil {
			val, _ = p.Getter()
		}
		s, ok, circ := jsonStringify(val, seen)
		if circ {
			return "", false, true
		}
		if !ok {
			continue
		}
		parts = append(parts, jsonQuote(k)+":"+s)
	}
	return "{" + strings.Join(parts, ",") + "}", true, false
}

// jsonQuote ports JSON.stringify for a string: control characters escaped,
// everything else (including <, >, &, U+2028 and U+2029) kept literally.
func jsonQuote(s string) string {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.Encode(s)
	out := strings.TrimSuffix(b.String(), "\n")
	// encoding/json still escapes U+2028 and U+2029; JSON.stringify does not.
	bs := string(rune(92)) // a backslash, spelled so no editor turns ` ` into the character
	return strings.NewReplacer(bs+"u2028", string(rune(0x2028)), bs+"u2029", string(rune(0x2029))).Replace(out)
}

func tryStringify(v any) string {
	s, ok, circ := jsonStringify(v, nil)
	if circ {
		return "[Circular]"
	}
	if !ok {
		return "undefined"
	}
	return s
}

// Format ports util.format(...args).
func Format(args ...any) string { return formatWithOptionsInternal(nil, args) }

// FormatWithOptions ports util.formatWithOptions(opts, ...args).
func FormatWithOptions(opts Options, args ...any) string {
	return formatWithOptionsInternal(&opts, args)
}

func optsOr(o *Options) Options {
	if o == nil {
		return DefaultOptions()
	}
	return *o
}

func formatNumberNoColor(n float64, o *Options) string {
	return formatNumber(func(s, _ string) string { return s }, n, o != nil && o.NumericSeparator)
}

func formatBigIntNoColor(b *big.Int, o *Options) string {
	return formatBigInt(func(s, _ string) string { return s }, b, o != nil && o.NumericSeparator)
}

// isCallable reports typeof v === 'function': functions and proxies of them.
func isCallable(v any) bool {
	switch x := v.(type) {
	case *Function:
		return true
	case *Proxy:
		return !x.Revoked && isCallable(x.Target)
	}
	return false
}

func formatWithOptionsInternal(o *Options, args []any) string {
	for i := range args {
		args[i] = normalize(args[i])
	}
	if len(args) == 0 {
		return ""
	}
	a := 0
	var str strings.Builder
	join := ""
	if first, ok := args[0].(string); ok {
		if len(args) == 1 {
			return first
		}
		u := utf16.Encode([]rune(first))
		slice := func(from, to int) string { return string(utf16.Decode(u[from:to])) }
		lastPos := 0
		var tempStr string
		for i := 0; i < len(u)-1; i++ {
			if u[i] != '%' {
				continue
			}
			i++
			next := u[i]
			if a+1 != len(args) {
				switch next {
				case 's':
					a++
					t := args[a]
					switch x := t.(type) {
					case float64:
						tempStr = formatNumberNoColor(x, o)
					case *big.Int:
						tempStr = formatBigIntNoColor(x, o)
					default:
						if isCallable(t) || isPrimitive(t) || t == nil || !hasBuiltInToString(t) {
							tempStr = jsString(t)
						} else {
							io := optsOr(o)
							io.Compact, io.CompactTrue, io.CompactFalse = 3, false, false
							io.Colors = false
							io.Depth = 0
							tempStr = Inspect(t, io)
						}
					}
				case 'j':
					a++
					tempStr = tryStringify(args[a])
				case 'd':
					a++
					switch x := args[a].(type) {
					case *big.Int:
						tempStr = formatBigIntNoColor(x, o)
					case Symbol:
						tempStr = "NaN"
					default:
						tempStr = formatNumberNoColor(ToNumber(x), o)
					}
				case 'O':
					a++
					tempStr = Inspect(args[a], optsOr(o))
				case 'o':
					a++
					io := optsOr(o)
					io.ShowHidden = true
					io.ShowProxy = true
					io.Depth = 4
					tempStr = Inspect(args[a], io)
				case 'i':
					a++
					switch x := args[a].(type) {
					case *big.Int:
						tempStr = formatBigIntNoColor(x, o)
					case Symbol:
						tempStr = "NaN"
					default:
						tempStr = formatNumberNoColor(jsnum.ParseInt(jsString(x)), o)
					}
				case 'f':
					a++
					if _, ok := args[a].(Symbol); ok {
						tempStr = "NaN"
					} else {
						tempStr = formatNumberNoColor(jsnum.ParseFloat(jsString(args[a])), o)
					}
				case 'c':
					a++
					tempStr = ""
				case '%':
					str.WriteString(slice(lastPos, i))
					lastPos = i + 1
					continue
				default:
					continue
				}
				if lastPos != i-1 {
					str.WriteString(slice(lastPos, i-1))
				}
				str.WriteString(tempStr)
				lastPos = i + 1
			} else if next == '%' {
				str.WriteString(slice(lastPos, i))
				lastPos = i + 1
			}
		}
		if lastPos != 0 {
			a++
			join = " "
			if lastPos < len(u) {
				str.WriteString(slice(lastPos, len(u)))
			}
		}
	}
	for a < len(args) {
		str.WriteString(join)
		if s, ok := args[a].(string); ok {
			str.WriteString(s)
		} else {
			str.WriteString(Inspect(args[a], optsOr(o)))
		}
		join = " "
		a++
	}
	return str.String()
}
