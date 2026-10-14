package nodeutil

import (
	"encoding/binary"
	"math"
	"math/big"
	"sort"
	"strings"
	"unicode/utf16"

	"github.com/gav4you/npmgo/internal/jsnum"
)

// Iterator states (kWeak / kIterator / kMapEntries in inspect.js).
const (
	kWeak = iota
	kIterator
	kMapEntries
)

func formatProxy(c *ctx, p *Proxy, recurseTimes int) string {
	if !c.nanDepth && float64(recurseTimes) > c.Depth {
		return c.stylize("Proxy [Array]", "special")
	}
	recurseTimes++
	c.indentationLvl += 2
	res := []string{
		formatValue(c, p.Target, recurseTimes, false),
		formatValue(c, p.Handler, recurseTimes, false),
	}
	c.indentationLvl -= 2
	return reduceToSingleString(c, res, "", [2]string{"Proxy [", "]"}, kArrayExtrasType, recurseTimes, nil)
}

func formatPromise(c *ctx, p *Promise, recurseTimes int) []string {
	if p.State == Pending {
		return []string{c.stylize("<pending>", "special")}
	}
	c.indentationLvl += 2
	str := formatValue(c, p.Result, recurseTimes, false)
	c.indentationLvl -= 2
	if p.State == Rejected {
		return []string{c.stylize("<rejected>", "special") + " " + str}
	}
	return []string{str}
}

func getIteratorBraces(typ, tag string) [2]string {
	if tag != typ+" Iterator" {
		if tag != "" {
			tag += "] ["
		}
		tag += typ + " Iterator"
	}
	return [2]string{"[" + tag + "] {", "}"}
}

func formatIterator(braces *[2]string, c *ctx, entries []any, isKeyValue bool, recurseTimes int) []string {
	if isKeyValue {
		if strings.HasSuffix(braces[0], " Iterator] {") {
			braces[0] = strings.TrimSuffix(braces[0], " Iterator] {") + " Entries] {"
		}
		return formatMapIterInner(c, recurseTimes, entries, kMapEntries)
	}
	return formatSetIterInner(c, recurseTimes, entries, kIterator)
}

// formatSetIterInner / formatMapIterInner: Node allocates `new Array(maxLength)`
// here, which throws RangeError for a fractional or NaN maxArrayLength; the Go
// port formats floor(maxLength) entries instead (see PROGRESS.md).
func formatSetIterInner(c *ctx, recurseTimes int, entries []any, state int) []string {
	maxLength := maxLen(c, len(entries))
	var output []string
	c.indentationLvl += 2
	for i := 0; float64(i) < maxLength; i++ {
		output = append(output, formatValue(c, entries[i], recurseTimes, false))
	}
	c.indentationLvl -= 2
	if state == kWeak && !c.Sorted {
		sortJS(output)
	}
	if remaining := float64(len(entries)) - maxLength; remaining > 0 {
		output = append(output, remainingText(remaining))
	}
	return output
}

func formatMapIterInner(c *ctx, recurseTimes int, entries []any, state int) []string {
	l := len(entries) / 2
	maxLength := maxLen(c, l)
	remaining := float64(l) - math.Max(c.MaxArrayLength, 0)
	var output []string
	c.indentationLvl += 2
	if state == kWeak {
		for i := 0; float64(i) < maxLength; i++ {
			output = append(output, formatValue(c, entries[2*i], recurseTimes, false)+" => "+
				formatValue(c, entries[2*i+1], recurseTimes, false))
		}
		if !c.Sorted {
			sortJS(output)
		}
	} else {
		for i := 0; float64(i) < maxLength; i++ {
			res := []string{
				formatValue(c, entries[2*i], recurseTimes, false),
				formatValue(c, entries[2*i+1], recurseTimes, false),
			}
			output = append(output, reduceToSingleString(c, res, "", [2]string{"[", "]"}, kArrayExtrasType, recurseTimes, nil))
		}
	}
	c.indentationLvl -= 2
	if remaining > 0 {
		output = append(output, remainingText(remaining))
	}
	return output
}

// sortJS sorts like Array.prototype.sort() with no comparator: by UTF-16 code units.
func sortJS(s []string) {
	sort.SliceStable(s, func(i, j int) bool {
		a, b := utf16.Encode([]rune(s[i])), utf16.Encode([]rune(s[j]))
		for k := 0; k < len(a) && k < len(b); k++ {
			if a[k] != b[k] {
				return a[k] < b[k]
			}
		}
		return len(a) < len(b)
	})
}

// RegExpColors is inspect.styles.regexp.colors: the depth-cycled palette.
var RegExpColors = []string{"green", "red", "yellow", "cyan", "magenta"}

// highlightRegExp translates Node's tolerant single-pass RegExp colouriser.
func highlightRegExp(regexpString string) string {
	r := utf16.Encode([]rune(regexpString))
	at := func(i int) string {
		if i < 0 || i >= len(r) {
			return ""
		}
		return string(utf16.Decode(r[i : i+1]))
	}
	slice := func(a, b int) string {
		if a > len(r) {
			a = len(r)
		}
		if b > len(r) {
			b = len(r)
		}
		if a >= b {
			return ""
		}
		return string(utf16.Decode(r[a:b]))
	}
	var palette [][2]string
	for _, name := range RegExpColors {
		if col, ok := colors[name]; ok {
			palette = append(palette, [2]string{ansi(col[0]), ansi(col[1])})
		}
	}
	var out strings.Builder
	i, depth := 0, 0
	inClass := false

	write := func(s string) {
		idx := depth % len(palette)
		if idx < 0 { // palette[-n] is undefined in JS -> palette[0]
			idx = 0
		}
		col := palette[idx]
		out.WriteString(col[0] + s + col[1])
	}
	writeDepth := func(s string, incDepth, incI int) {
		depth += incDepth
		write(s)
		depth -= incDepth
		i += incI
	}
	writeGroup := func(start, end string, decreaseDepth int) {
		seq := ""
		i++
		for i < len(r) && at(i) != end {
			seq += at(i)
			i++
		}
		if i < len(r) {
			depth -= decreaseDepth
			write(start)
			writeDepth(seq, 1, 1)
			write(end)
			depth += decreaseDepth
		} else {
			writeDepth(start, 1, -len(utf16.Encode([]rune(seq))))
		}
	}
	isDigit := func(s string) bool { return s >= "0" && s <= "9" && len(s) == 1 }

	write("/")
	depth++
	i = 1
	for i < len(r) {
		ch := at(i)
		if inClass {
			switch {
			case ch == `\`:
				seq := `\`
				i++
				if i < len(r) {
					seq += at(i)
					i++
					next := seq[1:]
					if next == "u" && at(i) == "{" {
						writeGroup(seq+"{", "}", 0)
						continue
					} else if (next == "p" || next == "P") && at(i) == "{" {
						writeGroup(seq+"{", "}", 0)
						continue
					} else if next == "x" {
						seq += slice(i, i+2)
						i += 2
					}
				}
				write(seq)
			case ch == "]":
				depth--
				write("]")
				i++
				inClass = false
			case ch == "-" && at(i-1) != "[" && i+1 < len(r) && at(i+1) != "]":
				writeDepth("-", 1, 1)
			default:
				write(ch)
				i++
			}
			continue
		}
		switch {
		case ch == "[":
			write("[")
			depth++
			i++
			inClass = true
		case ch == "(":
			write("(")
			depth++
			i++
			if i < len(r) && at(i) == "?" {
				i++
				a := at(i)
				if a == ":" || a == "=" || a == "!" {
					writeDepth("?"+a, -1, 1)
				} else {
					b := at(i + 1)
					if a == "<" && (b == "=" || b == "!") {
						writeDepth("?<"+b, -1, 2)
					} else if a == "<" {
						i++
						start := i
						for i < len(r) && at(i) != ">" {
							i++
						}
						name := slice(start, i)
						if i < len(r) && at(i) == ">" {
							depth--
							write("?<")
							writeDepth(name, 1, 0)
							write(">")
							depth++
							i++
						} else {
							writeDepth("?<", -1, 0)
							write(name)
						}
					} else {
						write("?")
					}
				}
			}
		case ch == ")":
			depth--
			write(")")
			i++
		case ch == `\`:
			seq := `\`
			i++
			if i < len(r) {
				seq += at(i)
				i++
				next := seq[1:]
				if i < len(r) {
					if next == "u" && at(i) == "{" {
						writeGroup(seq+"{", "}", 0)
						continue
					} else if next == "x" {
						seq += slice(i, i+2)
						i += 2
					} else if isDigit(next) {
						for i < len(r) && isDigit(at(i)) {
							seq += at(i)
							i++
						}
					} else if next == "k" && at(i) == "<" {
						writeGroup(seq+"<", ">", 1)
						continue
					} else if (next == "p" || next == "P") && at(i) == "{" {
						writeGroup(seq+"{", "}", 0)
						continue
					}
				}
			}
			writeDepth(seq, 1, 0)
		case ch == "|" || ch == "+" || ch == "*" || ch == "?" || ch == "," || ch == "^" || ch == "$":
			writeDepth(ch, 3, 1)
		case ch == "{":
			i++
			digits := ""
			for i < len(r) && isDigit(at(i)) {
				digits += at(i)
				i++
			}
			if digits != "" {
				write("{")
				depth++
				writeDepth(digits, 1, 0)
			}
			if i < len(r) {
				if at(i) == "," {
					if digits == "" {
						write("{")
						depth++
					}
					write(",")
					i++
				} else if digits == "" {
					depth++
					write("{")
					depth--
					continue
				}
			}
			digits2 := ""
			for i < len(r) && isDigit(at(i)) {
				digits2 += at(i)
				i++
			}
			if digits2 != "" {
				writeDepth(digits2, 1, 0)
			}
			if i < len(r) && at(i) == "}" {
				depth--
				write("}")
				i++
			}
			if i < len(r) && at(i) == "?" {
				writeDepth("?", 3, 1)
			}
		case ch == ".":
			writeDepth(ch, 2, 1)
		case ch == "/":
			goto done
		default:
			writeDepth(ch, 1, 1)
		}
	}
done:
	writeDepth("/", -1, 1)
	if i < len(r) {
		write(slice(i, len(r)))
	}
	return out.String()
}

func ansi(code int) string { return "\x1b[" + itoa(code) + "m" }

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// formatExtraProperty ports formatExtraProperties.
func formatExtraProperty(c *ctx, p Prop, recurseTimes int, typedArray bool) string {
	c.indentationLvl += 2
	str := formatValue(c, p.Value, recurseTimes, typedArray)
	c.indentationLvl -= 2
	return c.stylize("["+p.Key.(string)+"]", "string") + ": " + str
}

func bytesPerElement(kind string) int {
	switch kind {
	case "Int8Array", "Uint8Array", "Uint8ClampedArray":
		return 1
	case "Int16Array", "Uint16Array", "Float16Array":
		return 2
	case "Int32Array", "Uint32Array", "Float32Array":
		return 4
	}
	return 8 // Float64Array, BigInt64Array, BigUint64Array
}

func typedLen(t *TypedArray) int {
	if t.BigItems != nil {
		return len(t.BigItems)
	}
	return len(t.Items)
}

// typedBuffer returns the backing ArrayBuffer, synthesising one (little-endian
// element bytes) when the caller did not supply it.
func typedBuffer(t *TypedArray) *ArrayBuffer {
	if t.Buffer != nil {
		return t.Buffer
	}
	return &ArrayBuffer{Bytes: encodeTyped(t)}
}

func formatArrayBuffer(c *ctx, v *ArrayBuffer) []string {
	if v.Detached {
		return []string{c.stylize("(detached)", "special")}
	}
	// hexSlice(buffer, 0, min(maxArrayLength, length)) truncates its end index.
	n := jsSliceEnd(maxLen(c, len(v.Bytes)), len(v.Bytes))
	var b strings.Builder
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(hexByte(v.Bytes[i]))
	}
	str := b.String()
	if remaining := float64(len(v.Bytes)) - c.MaxArrayLength; remaining > 0 {
		s := ""
		if remaining > 1 {
			s = "s"
		}
		str += " ... " + jsnum.String(remaining) + " more byte" + s
	}
	return []string{c.stylize("[Uint8Contents]", "special") + ": <" + str + ">"}
}

func hexByte(b byte) string {
	const hx = "0123456789abcdef"
	return string([]byte{hx[b>>4], hx[b&15]})
}

// formatNamespaceObject ports formatNamespaceObject (TDZ bindings print
// as <uninitialized>).
func formatNamespaceObject(c *ctx, v *ModuleNamespace, keys []Prop, recurseTimes int) []string {
	out := make([]string, len(keys))
	for i, k := range keys {
		name, _ := k.Key.(string)
		if v.Uninitialized[name] {
			s := formatProperty(c, Prop{Key: k.Key, Value: ""}, recurseTimes, kObjectType)
			pos := strings.LastIndex(s, " ")
			out[i] = s[:pos+1] + c.stylize("<uninitialized>", "special")
			continue
		}
		out[i] = formatProperty(c, k, recurseTimes, kObjectType)
	}
	return out
}

// encodeTyped lays out typed-array elements little-endian, as V8 does on x86/ARM.
func encodeTyped(t *TypedArray) []byte {
	kind := typedName(t)
	bpe := bytesPerElement(kind)
	n := typedLen(t)
	out := make([]byte, n*bpe)
	for i := 0; i < n; i++ {
		b := out[i*bpe : (i+1)*bpe]
		if t.BigItems != nil {
			x := new(big.Int).Set(t.BigItems[i])
			if x.Sign() < 0 {
				x.Add(x, new(big.Int).Lsh(big.NewInt(1), 64))
			}
			binary.LittleEndian.PutUint64(b, x.Uint64())
			continue
		}
		f := t.Items[i]
		switch kind {
		case "Float32Array":
			binary.LittleEndian.PutUint32(b, math.Float32bits(float32(f)))
		case "Float64Array":
			binary.LittleEndian.PutUint64(b, math.Float64bits(f))
		case "Float16Array":
			binary.LittleEndian.PutUint16(b, float16bits(f))
		default:
			u := uint64(int64(f))
			for k := 0; k < bpe; k++ {
				b[k] = byte(u >> (8 * k))
			}
		}
	}
	return out
}

// float16bits converts to IEEE 754 binary16 with round-half-to-even.
func float16bits(f float64) uint16 {
	b := math.Float32bits(float32(f))
	sign := uint16(b>>16) & 0x8000
	exp := int((b>>23)&0xff) - 127 + 15
	mant := b & 0x7fffff
	switch {
	case (b>>23)&0xff == 0xff:
		if mant != 0 {
			return sign | 0x7e00
		}
		return sign | 0x7c00
	case exp >= 0x1f:
		return sign | 0x7c00
	case exp <= 0:
		if exp < -10 {
			return sign
		}
		mant |= 0x800000
		shift := uint(14 - exp)
		half := mant >> shift
		rem := mant & ((1 << shift) - 1)
		mid := uint32(1) << (shift - 1)
		if rem > mid || (rem == mid && half&1 == 1) {
			half++
		}
		return sign | uint16(half)
	}
	half := uint32(exp)<<10 | mant>>13
	rem := mant & 0x1fff
	if rem > 0x1000 || (rem == 0x1000 && half&1 == 1) {
		half++
	}
	return sign | uint16(half)
}
