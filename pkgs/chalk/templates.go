package chalk

// source/templates.js and chalkTag: the tagged-template syntax,
// chalk`{bold.red text {~bold not bold}}`.
//
// templates.js drives a String.prototype.replace with regular expressions
// that Go's regexp cannot express (a backreference, a lookahead) and whose
// '.' excludes \r, U+2028 and U+2029. The scanners below match exactly what
// those expressions match, in the same order. JavaScript strings are UTF-16
// and an escape such as \uD83D may produce half of a surrogate pair; the
// text is kept as WTF-8 (a lone surrogate encoded like any other code point)
// until it is returned, when pairs are joined and lone halves, which a Go
// string cannot hold, become U+FFFD.

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/gav4you/npmgo/internal/jsnum"
	"github.com/gav4you/npmgo/internal/jsvalue"
	colorname "github.com/gav4you/npmgo/pkgs/color-name"
)

// Tag is calling the chalk as a tagged template, chalk`…` or chalk.red`…`
// (chalkTag): literals are the template's raw strings (strings.raw) and
// values the interpolated expressions, converted with String() and with
// their {, } and \ escaped, so that they are never read as template syntax.
// A builder also wraps the result in its own styles.
//
// The template syntax is chalk's: {style text} styles text, {~style text}
// removes a style of an enclosing block, styles chain with dots
// ({bold.red …}), take arguments ({rgb(255,0,0) …}, {hex('#FF0000') …}) and
// nest, and backslash escapes (\n, \u001B, \u{1F600}, \x1b, \{, …) work in
// the text and in quoted arguments. A malformed template returns chalk's
// error (an extraneous }, missing closing brackets, an unknown style, an
// invalid argument); an escape beyond U+10FFFF returns a *jsvalue.RangeError.
//
// Two JavaScript behaviours are approximated: a style name that is not a
// chalk style but another property of the chalk object (level, constructor,
// Instance, call, …) reports "Unknown Chalk style" here, and a colour model
// given arguments of the wrong kind or number, which JavaScript passes on
// unchecked, returns an error.
func (c *Chalk) Tag(literals []string, values ...any) (string, error) {
	var parts []string
	if len(literals) > 0 {
		parts = append(parts, literals[0])
	}
	for i := 1; i < len(literals); i++ {
		var v any = jsvalue.Undefined{}
		if i-1 < len(values) {
			v = values[i-1]
		}
		parts = append(parts, templateEscaper.Replace(jsvalue.String(v)), literals[i])
	}
	s, err := template(c, strings.Join(parts, ""))
	if err != nil {
		return "", err
	}
	if c.builder {
		s = c.applyStyle(s)
	}
	return wtf8ToUTF8(s), nil
}

// Template is Tag for a template without interpolations, chalk`text`. text
// is the raw template, as written between the backquotes.
func (c *Chalk) Template(text string) (string, error) { return c.Tag([]string{text}) }

// templateEscaper is String(value).replace(/[{}\\]/g, '\\$&').
var templateEscaper = strings.NewReplacer(`{`, `\{`, `}`, `\}`, `\`, `\\`)

type styleCall struct {
	name string
	args []any // float64 and string arguments; nil without parentheses
}

type layer struct {
	inverse bool
	styles  []styleCall
}

// template is templates.js's export, template(chalk, temporary).
func template(c *Chalk, temporary string) (string, error) {
	var (
		layers []layer
		chunks []string
		chunk  []byte
	)
	for i := 0; i < len(temporary); {
		switch temporary[i] {
		case '\\':
			if esc, n := scanEscape(temporary[i+1:]); n > 0 {
				s, err := unescape(esc)
				if err != nil {
					return "", err
				}
				chunk = append(chunk, s...)
				i += 1 + n
				continue
			}
		case '{':
			if inverse, style, n := scanStyle(temporary[i:]); n > 0 {
				s := string(chunk)
				chunk = nil
				if len(layers) > 0 {
					var err error
					if s, err = buildStyle(c, layers, s); err != nil {
						return "", err
					}
				}
				chunks = append(chunks, s)
				styles, err := parseStyle(style)
				if err != nil {
					return "", err
				}
				layers = append(layers, layer{inverse, styles})
				i += n
				continue
			}
		case '}':
			if len(layers) == 0 {
				return "", errors.New("Found extraneous } in Chalk template literal")
			}
			s, err := buildStyle(c, layers, string(chunk))
			if err != nil {
				return "", err
			}
			chunks = append(chunks, s)
			chunk = nil
			layers = layers[:len(layers)-1]
			i++
			continue
		}
		// Any other character. U+2028 and U+2029 match no alternative of
		// TEMPLATE_REGEX, so replace skips them and they are lost.
		r, size := utf8.DecodeRuneInString(temporary[i:])
		if r != ' ' && r != ' ' {
			chunk = append(chunk, temporary[i:i+size]...)
		}
		i += size
	}
	chunks = append(chunks, string(chunk))
	if n := len(layers); n > 0 {
		plural := "s"
		if n == 1 {
			plural = ""
		}
		return "", fmt.Errorf("Chalk template literal is missing %d closing bracket%s (`}`)", n, plural)
	}
	return strings.Join(chunks, ""), nil
}

func isHexDigit(b byte) bool {
	return b >= '0' && b <= '9' || b >= 'a' && b <= 'f' || b >= 'A' && b <= 'F'
}

func isWordChar(b byte) bool {
	return b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b == '_'
}

// scanEscape matches what follows a backslash against
// u(?:[a-f\d]{4}|\{[a-f\d]{1,6}\})|x[a-f\d]{2}|. (case-insensitively) and
// returns the match and its length; 0 for a line terminator or the end.
func scanEscape(rest string) (string, int) {
	if rest == "" {
		return "", 0
	}
	switch rest[0] {
	case 'u', 'U':
		if len(rest) >= 5 && isHexDigit(rest[1]) && isHexDigit(rest[2]) && isHexDigit(rest[3]) && isHexDigit(rest[4]) {
			return rest[:5], 5
		}
		if len(rest) >= 3 && rest[1] == '{' {
			h := 0
			for h < 6 && 2+h < len(rest) && isHexDigit(rest[2+h]) {
				h++
			}
			if h > 0 && 2+h < len(rest) && rest[2+h] == '}' {
				return rest[:3+h], 3 + h
			}
		}
	case 'x', 'X':
		if len(rest) >= 3 && isHexDigit(rest[1]) && isHexDigit(rest[2]) {
			return rest[:3], 3
		}
	}
	r, size := utf8.DecodeRuneInString(rest)
	if r == '\n' || r == '\r' || r == ' ' || r == ' ' {
		return "", 0
	}
	return rest[:size], size
}

var escapes = map[string]string{
	"n": "\n", "r": "\r", "t": "\t", "b": "\b", "f": "\f", "v": "\v",
	"0": "\x00", "\\": "\\", "e": "\x1b", "a": "\x07",
}

// unescape is templates.js's unescape(c). Only a lower-case u or x starts a
// numeric escape: \U0041 and \X41 match the case-insensitive pattern but
// unescape to "U0041" and "X41".
func unescape(c string) (string, error) {
	u := c[0] == 'u'
	bracket := len(c) > 1 && c[1] == '{'
	if (u && !bracket && len(c) == 5) || (c[0] == 'x' && len(c) == 3) {
		n, _ := strconv.ParseUint(c[1:], 16, 16)
		return codePoint(rune(n)), nil // String.fromCharCode
	}
	if u && bracket {
		n, _ := strconv.ParseUint(c[2:len(c)-1], 16, 32)
		if n > utf8.MaxRune {
			return "", &jsvalue.RangeError{Message: "Invalid code point " + strconv.FormatUint(n, 10)}
		}
		return codePoint(rune(n)), nil // String.fromCodePoint
	}
	if e, ok := escapes[c]; ok {
		return e, nil
	}
	return c, nil
}

// codePoint encodes r, a surrogate half included (WTF-8).
func codePoint(r rune) string {
	if r >= 0xD800 && r <= 0xDFFF {
		return string([]byte{0xED, byte(0x80 | r>>6&0x3F), byte(0x80 | r&0x3F)})
	}
	return string(r)
}

// surrogateAt decodes a WTF-8 surrogate half at s[i:].
func surrogateAt(s string, i int) (rune, bool) {
	if i+2 < len(s) && s[i] == 0xED && s[i+1] >= 0xA0 && s[i+1] <= 0xBF && s[i+2] >= 0x80 && s[i+2] <= 0xBF {
		return 0xD000 | rune(s[i+1]&0x3F)<<6 | rune(s[i+2]&0x3F), true
	}
	return 0, false
}

// wtf8ToUTF8 joins surrogate pairs and replaces lone halves with U+FFFD.
func wtf8ToUTF8(s string) string {
	if !strings.Contains(s, "\xED") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		if hi, ok := surrogateAt(s, i); ok {
			if lo, ok := surrogateAt(s, i+3); ok && hi < 0xDC00 && lo >= 0xDC00 {
				b.WriteRune(utf16.DecodeRune(hi, lo))
				i += 6
				continue
			}
			b.WriteRune(utf8.RuneError)
			i += 3
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

// scanStyle matches s, which starts with '{', against
// \{(~)?(\w+(?:\([^)]*\))?(?:\.\w+(?:\([^)]*\))?)*)(?:[ \t]|(?=\r?\n)): it
// returns whether the block is inverse, the style chain and the length of
// the match, 0 if there is none. Every quantifier is greedy and no
// backtracking can produce a different match, so a single forward scan
// decides.
func scanStyle(s string) (inverse bool, style string, n int) {
	i := 1
	if i < len(s) && s[i] == '~' {
		inverse = true
		i++
	}
	start := i
	segment := func() bool { // \w+(?:\([^)]*\))?
		j := i
		for j < len(s) && isWordChar(s[j]) {
			j++
		}
		if j == i {
			return false
		}
		i = j
		if i < len(s) && s[i] == '(' {
			if k := strings.IndexByte(s[i+1:], ')'); k >= 0 {
				i += k + 2
			}
		}
		return true
	}
	if !segment() {
		return false, "", 0
	}
	for i+1 < len(s) && s[i] == '.' && isWordChar(s[i+1]) {
		i++
		segment()
	}
	style = s[start:i]
	switch {
	case i < len(s) && (s[i] == ' ' || s[i] == '\t'):
		return inverse, style, i + 1
	case strings.HasPrefix(s[i:], "\n") || strings.HasPrefix(s[i:], "\r\n"):
		return inverse, style, i
	}
	return false, "", 0
}

// parseStyle is templates.js's parseStyle: STYLE_REGEX
// ((?:^|\.)(\w+)(?:\(([^)]*)\))?) over a chain scanStyle matched.
func parseStyle(style string) ([]styleCall, error) {
	var results []styleCall
	for i := 0; i < len(style); {
		if i > 0 {
			i++ // the '.'
		}
		j := i
		for j < len(style) && isWordChar(style[j]) {
			j++
		}
		call := styleCall{name: style[i:j]}
		i = j
		if i < len(style) && style[i] == '(' {
			k := strings.IndexByte(style[i:], ')')
			if inner := style[i+1 : i+k]; inner != "" { // matches[2] is truthy
				args, err := parseArguments(call.name, inner)
				if err != nil {
					return nil, err
				}
				call.args = args
			}
			i += k + 1
		}
		results = append(results, call)
	}
	return results, nil
}

// parseArguments is templates.js's parseArguments: comma-separated numbers
// (Number(chunk)) and quoted strings with escapes.
func parseArguments(name, arguments string) ([]any, error) {
	var results []any
	// .trim().split(/\s*,\s*/g): split at commas, dropping the whitespace
	// around them.
	for _, chunk := range strings.Split(jsnum.TrimSpace(arguments), ",") {
		chunk = jsnum.TrimSpace(chunk)
		if number := jsnum.Number(chunk); !math.IsNaN(number) {
			results = append(results, number)
		} else if inner, ok := stringArgument(chunk); ok {
			s, err := unescapeAll(inner)
			if err != nil {
				return nil, err
			}
			results = append(results, s)
		} else {
			return nil, fmt.Errorf("Invalid Chalk template style argument: %s (in style '%s')", chunk, name)
		}
	}
	return results, nil
}

// stringArgument matches STRING_REGEX, ^(['"])((?:\\.|(?!\1)[^\\])*)\1$,
// and returns the text between the quotes.
func stringArgument(chunk string) (string, bool) {
	if len(chunk) < 2 || (chunk[0] != '\'' && chunk[0] != '"') {
		return "", false
	}
	quote := chunk[0]
	for i := 1; i < len(chunk); {
		switch chunk[i] {
		case '\\': // \\. : any character but a line terminator
			r, size := utf8.DecodeRuneInString(chunk[i+1:])
			if size == 0 || r == '\n' || r == '\r' || r == ' ' || r == ' ' {
				return "", false
			}
			i += 1 + size
		case quote: // only the last character may be the unescaped quote
			if i == len(chunk)-1 {
				return chunk[1:i], true
			}
			return "", false
		default:
			_, size := utf8.DecodeRuneInString(chunk[i:])
			i += size
		}
	}
	return "", false
}

// unescapeAll is .replace(ESCAPE_REGEX, …): every escape unescaped, every
// other character kept.
func unescapeAll(s string) (string, error) {
	var b []byte
	for i := 0; i < len(s); {
		if s[i] == '\\' {
			if esc, n := scanEscape(s[i+1:]); n > 0 {
				u, err := unescape(esc)
				if err != nil {
					return "", err
				}
				b = append(b, u...)
				i += 1 + n
				continue
			}
		}
		_, size := utf8.DecodeRuneInString(s[i:])
		b = append(b, s[i:i+size]...)
		i += size
	}
	return string(b), nil
}

// enabledStyle is one entry of buildStyle's enabled object.
type enabledStyle struct {
	name    string
	args    []any
	inverse bool // the value is null
}

// arrayIndex reports whether a property key is an array index, which
// JavaScript objects list before their other keys, in numeric order.
func arrayIndex(k string) (uint64, bool) {
	if k == "" || len(k) > 10 || (k[0] == '0' && len(k) > 1) {
		return 0, false
	}
	n, err := strconv.ParseUint(k, 10, 64)
	return n, err == nil && n <= 1<<32-2
}

// enabled builds buildStyle's enabled object, {name: args or null}, and
// returns its entries in Object.entries order. Assigning enabled.__proto__
// sets the object's prototype rather than creating an entry.
func enabled(layers []layer) []enabledStyle {
	var keys []string
	values := map[string]enabledStyle{}
	for _, l := range layers {
		for _, s := range l.styles {
			if s.name == "__proto__" {
				continue
			}
			if _, ok := values[s.name]; !ok {
				keys = append(keys, s.name)
			}
			values[s.name] = enabledStyle{s.name, s.args, l.inverse}
		}
	}
	sort.SliceStable(keys, func(a, b int) bool {
		na, ia := arrayIndex(keys[a])
		nb, ib := arrayIndex(keys[b])
		return ia && (!ib || na < nb)
	})
	entries := make([]enabledStyle, len(keys))
	for i, k := range keys {
		entries[i] = values[k]
	}
	return entries
}

// modelFunc is the function a colour-model getter returns, kept as the
// current value of buildStyle when the model is used without arguments.
type modelFunc struct {
	name  string
	level int // read by the getter
}

// buildStyle is templates.js's buildStyle(chalk, styles) applied to text.
// It walks the enabled styles from chalk: a style without arguments is its
// getter (a builder; a colour model gives its function), a style with
// arguments is called (a colour model makes a builder, a named style styles
// its arguments into a string), and the result is called with text.
func buildStyle(c *Chalk, layers []layer, text string) (string, error) {
	var current any = c
	for _, e := range enabled(layers) {
		if e.inverse {
			continue
		}
		cur, isChalk := current.(*Chalk)
		switch {
		case !isChalk && current != nil:
			if s, ok := current.(string); ok { // styleName in "…"
				return "", &jsvalue.TypeError{Message: "Cannot use 'in' operator to search for '" + e.name + "' in " + s}
			}
			return "", fmt.Errorf("Unknown Chalk style: %s", e.name) // a model's function has no styles
		case models[e.name].key == "" && !isStyle(e.name):
			return "", fmt.Errorf("Unknown Chalk style: %s", e.name)
		}
		_, isModel := models[e.name]
		switch {
		case isModel && len(e.args) > 0:
			b, err := cur.templateModel(e.name, e.args)
			if err != nil {
				return "", err
			}
			current = b
		case isModel:
			current = modelFunc{e.name, cur.Level()}
		case len(e.args) > 0:
			current = cur.styleByName(e.name).Sprint(e.args...)
		default:
			current = cur.styleByName(e.name)
		}
	}
	switch cur := current.(type) {
	case *Chalk:
		return cur.Sprint(text), nil
	case modelFunc:
		return "", cur.callUnbound(text)
	}
	return "", &jsvalue.TypeError{Message: "buildStyle(...) is not a function"}
}

// templateModel is current[name](...args) for a colour model. The arguments
// JavaScript would handle the same way as the Go methods' are accepted:
// three numbers for rgb, hsl, hsv and hwb (more are ignored, as there); for
// hex and keyword a string, or several arguments read as their joined string
// form; a number for ansi and ansi256 (with more, the conversions read NaN).
func (c *Chalk) templateModel(name string, args []any) (b *Chalk, err error) {
	defer func() {
		if r := recover(); r != nil {
			e, ok := r.(*jsvalue.TypeError)
			if !ok {
				panic(r)
			}
			b, err = nil, e
		}
	}()
	m := models[name]
	unsupported := fmt.Errorf("chalk: the Go port does not support %s with arguments %v in a template", name, args)
	switch m.key {
	case "rgb", "hsl", "hsv", "hwb":
		var v [3]float64
		for i := range v {
			if i >= len(args) {
				return nil, unsupported
			}
			f, ok := args[i].(float64)
			if !ok {
				return nil, unsupported
			}
			v[i] = f
		}
		methods := map[string][2]func(*Chalk, float64, float64, float64) *Chalk{
			"rgb": {(*Chalk).RGB, (*Chalk).BgRGB}, "hsl": {(*Chalk).HSL, (*Chalk).BgHSL},
			"hsv": {(*Chalk).HSV, (*Chalk).BgHSV}, "hwb": {(*Chalk).HWB, (*Chalk).BgHWB},
		}
		return methods[m.key][bgIndex(m.bg)](c, v[0], v[1], v[2]), nil
	case "hex", "keyword":
		s, ok := args[0].(string)
		if !ok || len(utf16.Encode([]rune(s))) <= 1 { // arg0.length > 1 passes arg0, else the args array
			s = jsvalue.Join(args, ",")
		}
		if m.key == "hex" {
			return [2]func(*Chalk, string) *Chalk{(*Chalk).Hex, (*Chalk).BgHex}[bgIndex(m.bg)](c, s), nil
		}
		return [2]func(*Chalk, string) *Chalk{(*Chalk).Keyword, (*Chalk).BgKeyword}[bgIndex(m.bg)](c, s), nil
	}
	n, ok := args[0].(float64)
	if !ok {
		return nil, unsupported
	}
	level := c.Level()
	identity := (m.key == "ansi" && (level == 0 || level == 1)) || (m.key == "ansi256" && level == 2)
	if len(args) > 1 && !identity {
		n = math.NaN() // the conversion reads Number([n, …])
	}
	if m.key == "ansi" {
		return [2]func(*Chalk, float64) *Chalk{(*Chalk).ANSI, (*Chalk).BgANSI}[bgIndex(m.bg)](c, n), nil
	}
	return [2]func(*Chalk, float64) *Chalk{(*Chalk).ANSI256, (*Chalk).BgANSI256}[bgIndex(m.bg)](c, n), nil
}

func bgIndex(bg bool) int {
	if bg {
		return 1
	}
	return 0
}

// callUnbound is buildStyle's result called with text when it is a colour
// model's function: called without this, it computes the colour code for
// text as its only argument and then fails reading this._styler, unless the
// conversion has already thrown (an unknown keyword, or an hsv hue whose
// floor is negative or NaN).
func (m modelFunc) callUnbound(text string) error {
	key := models[m.name].key
	if m.level < 0 || m.level > 3 {
		return &jsvalue.TypeError{Message: "Cannot read properties of undefined (reading '" + key + "')"}
	}
	conversionFails := false
	switch key {
	case "keyword":
		_, known := colorname.RGB[text]
		conversionFails = !known
	case "hsv":
		// args is text itself when it is longer than one code unit, else
		// [text]; hsv.rgb returns undefined when Math.floor(args[0] / 60) % 6
		// is negative or NaN.
		u := utf16.Encode([]rune(text))
		first := text
		if len(u) > 1 {
			first = string(utf16.Decode(u[:1]))
		}
		h := jsnum.Number(first) / 60
		conversionFails = math.IsNaN(h) || math.IsInf(h, 0) || math.Floor(h) < 0
	}
	if conversionFails {
		if m.level <= 1 {
			return &jsvalue.TypeError{Message: "args is not iterable"}
		}
		return &jsvalue.TypeError{Message: "Cannot read properties of undefined (reading '0')"}
	}
	return &jsvalue.TypeError{Message: "Cannot read properties of undefined (reading '_styler')"}
}
