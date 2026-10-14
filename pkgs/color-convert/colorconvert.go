// Package colorconvert is a Go translation of the npm package
// color-convert@2.0.1 (https://github.com/Qix-/color-convert): conversions
// between 15 colour models, routed through the direct conversions of
// conversions.js by route.js's breadth-first search. The original copyright
// notice and MIT license are preserved in LICENSE.
//
// Convert[from][to] is convert[from][to]: Call rounds every channel of the
// result with Math.round, Raw (convert[from][to].raw) does not:
//
//	colorconvert.Convert["rgb"]["hsl"].Call([]float64{140, 200, 100}) // []float64{96, 48, 59}
//	colorconvert.Convert["keyword"]["hex"].Call("blue")              // "0000FF"
//
// Values take the shapes the JavaScript functions take and return:
//
//	[]float64  the channels of rgb, hsl, hsv, hwb, cmyk, xyz, lab, lch, hcg, apple and gray
//	float64    an ansi16 or ansi256 code
//	string     a hex colour or a CSS keyword
//	nil        undefined (or null)
//
// A conversion returns nil where the JavaScript one returns undefined:
// keyword → rgb for an unknown name, hsv → rgb when Math.floor(h / 60) % 6 is
// negative or NaN, rgb → keyword when no keyword is at a finite distance.
// Where the JavaScript throws (a later step reading a channel of undefined)
// the conversion panics with a *jsvalue.TypeError carrying V8's message.
//
// The arithmetic follows JavaScript double semantics step by step: Math.round
// (jsnum.Round), % (math.Mod), the bitwise operators (jsnum.ToInt32),
// Math.atan2 / Math.sin / Math.cos exactly as V8 computes them (jsnum's fdlibm
// port). ** uses math.Pow: V8 calls the C library's pow, so Node's own xyz,
// lab and lch results differ between platforms in the last bit, and so may
// these.
package colorconvert

import (
	"fmt"

	"github.com/gav4you/npmgo/internal/jsnum"
	"github.com/gav4you/npmgo/internal/jsvalue"
)

// Value is a colour in one model, or a conversion's result; see the package
// documentation for its shapes.
type Value = any

// Models lists the colour models in conversions.js order
// (Object.keys(require('color-convert'))).
var Models = []string{"rgb", "hsl", "hsv", "hwb", "cmyk", "xyz", "lab", "lch", "hex", "keyword", "ansi16", "ansi256", "hcg", "apple", "gray"}

// Channels is convert[model].channels: how many values a colour has.
var Channels = map[string]int{
	"rgb": 3, "hsl": 3, "hsv": 3, "hwb": 3, "cmyk": 4, "xyz": 3, "lab": 3, "lch": 3,
	"hex": 1, "keyword": 1, "ansi16": 1, "ansi256": 1, "hcg": 3, "apple": 3, "gray": 1,
}

// Labels is convert[model].labels, one label per channel. JavaScript stores
// the labels of rgb, hsl, hsv, hwb, cmyk, xyz, lab and lch as one string
// ('rgb'), one character per channel; here they are split.
var Labels = map[string][]string{
	"rgb": {"r", "g", "b"}, "hsl": {"h", "s", "l"}, "hsv": {"h", "s", "v"}, "hwb": {"h", "w", "b"},
	"cmyk": {"c", "m", "y", "k"}, "xyz": {"x", "y", "z"}, "lab": {"l", "a", "b"}, "lch": {"l", "c", "h"},
	"hex": {"hex"}, "keyword": {"keyword"}, "ansi16": {"ansi16"}, "ansi256": {"ansi256"},
	"hcg": {"h", "c", "g"}, "apple": {"r16", "g16", "b16"}, "gray": {"gray"},
}

// Conversion is one routed conversion, convert[from][to].
type Conversion struct {
	From, To string
	// Path is the function's .conversion property: the models it passes
	// through, From and To included. As in the original, Convert["gray"]["hsl"]
	// reports ["gray", "hsv"], because conversions.js assigns one function
	// to gray.hsl and gray.hsv.
	Path []string
	fn   rawFunc
}

// Convert is the convert object: Convert[from][to] for every pair of
// distinct models. Treat it as read-only.
var Convert = func() map[string]map[string]*Conversion {
	convert := make(map[string]map[string]*Conversion, len(Models))
	for _, from := range Models {
		convert[from] = map[string]*Conversion{}
		for to, r := range route(from) {
			convert[from][to] = &Conversion{From: from, To: to, Path: r.path, fn: r.fn}
		}
	}
	return convert
}()

// Call is convert[from][to](...args): the conversion, with every channel of
// an array result rounded by Math.round (other results are returned as
// they are). A nil value is returned as it is, as the wrapper returns an
// undefined or null first argument.
//
// The value is what the JavaScript function is called with: the channels of
// From as a []float64 or, for one channel, a number; a number for ansi16 and
// ansi256; a string for hex and keyword. As in JavaScript, a number or an
// array given to hex or keyword is read as its string form ([1, 2] as
// "1,2"), and an array given to ansi16 or ansi256 as the number its string
// form spells. A one-element []float64 is read as its number (JavaScript
// passes the array itself, which differs only for -0 and for channels a
// conversion returns unchanged).
func (c *Conversion) Call(value Value) Value {
	if value == nil {
		return nil
	}
	result := c.fn(input(c.From, value))
	if a, ok := result.([]float64); ok {
		for i := range a {
			a[i] = jsnum.Round(a[i])
		}
	}
	return result
}

// Raw is convert[from][to].raw(...args): the conversion without rounding.
func (c *Conversion) Raw(value Value) Value {
	if value == nil {
		return nil
	}
	return c.fn(input(c.From, value))
}

// input turns a Call / Raw value into the value the conversion function of
// model receives.
func input(model string, v Value) Value {
	a, isArray := v.([]float64)
	switch model {
	case "hex", "keyword": // args.toString(16), cssKeywords[args]
		if s, ok := v.(string); ok {
			return s
		}
		if isArray {
			return joinNumbers(a)
		}
		if jsvalue.IsNumber(v) {
			return jsnum.String(jsvalue.Float(v))
		}
	case "ansi16", "ansi256": // arithmetic on the args array
		if isArray {
			return jsnum.Number(joinNumbers(a))
		}
		if jsvalue.IsNumber(v) {
			return jsvalue.Float(v)
		}
	default:
		if isArray {
			return a
		}
		if jsvalue.IsNumber(v) {
			return []float64{jsvalue.Float(v)}
		}
	}
	panic(fmt.Sprintf("colorconvert: a %s colour cannot be a %T", model, v))
}
