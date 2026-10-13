package nodeutil

import (
	"encoding/json"
	"math"
	"math/big"
	"strings"
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
	case *Array:
		if len(x.Items) == 0 {
			return 0
		}
		if len(x.Items) == 1 {
			return ToNumber(jsString(x.Items[0]))
		}
	}
	return math.NaN()
}

// jsString ports String(v) for primitives.
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
		parts := make([]string, len(x.Items))
		for i, it := range x.Items {
			switch it.(type) {
			case nil, Undefined, Hole:
			default:
				parts[i] = jsString(it)
			}
		}
		return strings.Join(parts, ",")
	case *Error:
		return errorToString(x)
	case *Function:
		return x.Source
	case *Date:
		return jsnum.ISOString(x.Ms)
	case *RegExp:
		return "/" + x.Source + "/" + x.Flags
	}
	return "[object Object]"
}

// hasBuiltInToString approximates the Node check: plain objects, arrays,
// errors and the other modelled builtins use the built-in toString.
func hasBuiltInToString(v any) bool {
	switch v.(type) {
	case *Object, *Array, *Map, *Set, *Function, *Boxed, *TypedArray, *Error, *Date, *RegExp:
		return true
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
		b, _ := json.Marshal(x)
		s := strings.NewReplacer(`<`, "<", `>`, ">", `&`, "&", ` `, " ", ` `, " ").Replace(string(b))
		return s, true, false
	case *big.Int:
		panic("TypeError: Do not know how to serialize a BigInt")
	case *Date:
		if math.IsNaN(x.Ms) {
			return "null", true, false
		}
		return `"` + jsnum.ISOString(x.Ms) + `"`, true, false
	}
	for _, s := range seen {
		if s == v {
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
	case *Boxed:
		switch b := x.Value.(type) {
		case string, float64, bool, int:
			return jsonStringify(b, seen)
		}
	}
	var parts []string
	for _, p := range props {
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
		kj, _, _ := jsonStringify(k, nil)
		parts = append(parts, kj+":"+s)
	}
	return "{" + strings.Join(parts, ",") + "}", true, false
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
						if _, isFn := t.(*Function); isFn || isPrimitive(t) || t == nil || !hasBuiltInToString(t) {
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
