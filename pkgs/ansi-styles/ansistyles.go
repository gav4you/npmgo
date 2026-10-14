// Package ansistyles is a Go translation of the npm package ansi-styles@4.3.0
// (https://github.com/chalk/ansi-styles): the ANSI escape codes that style
// text in a terminal. The original copyright notice and MIT license are
// preserved in LICENSE.
//
// Styles[name] is require('ansi-styles')[name]; Modifier, Color and BgColor
// are its .modifier, .color and .bgColor groups, and Codes is .codes. The
// colour groups carry the dynamic tables .ansi, .ansi256 and .ansi16m, which
// turn a colour in any color-convert model into an escape code:
//
//	ansistyles.Color.ANSI256.RGB(255, 0, 0)   // "\x1b[38;5;196m"
//	ansistyles.BgColor.ANSI16M.Hex("#00FF00") // "\x1b[48;2;0;255;0m"
//
// JavaScript assembles a fresh object on every require; the Go tables are
// shared, so treat them as read-only.
package ansistyles

import (
	"strconv"

	"github.com/gav4you/npmgo/internal/jsnum"
	"github.com/gav4you/npmgo/internal/jsvalue"
	colorconvert "github.com/gav4you/npmgo/pkgs/color-convert"
)

// Style is one style's escape codes, {open, close}.
type Style struct{ Open, Close string }

// Group is one of the style groups (ansiStyles.modifier, .color, .bgColor).
type Group struct {
	// Names lists the group's styles in key order.
	Names []string
	// Styles maps each of them to its codes.
	Styles map[string]Style
}

// ColorGroup is ansiStyles.color or ansiStyles.bgColor: the colour styles,
// the code that resets the colour and the dynamic colour-code tables.
type ColorGroup struct {
	Group
	// Close is color.close ("\x1b[39m") or bgColor.close ("\x1b[49m").
	Close string
	// ANSI, ANSI256 and ANSI16M are .ansi, .ansi256 and .ansi16m: 16-colour,
	// 256-colour and truecolor codes.
	ANSI, ANSI256, ANSI16M Dynamic
}

var (
	// Names lists every style in the order of the module's keys
	// (Object.keys(require('ansi-styles'))): the modifiers, the colours and
	// the background colours, each group followed by its gray / grey aliases.
	Names []string
	// Styles maps every style name to its codes, like ansiStyles[name].
	Styles map[string]Style
	// Modifier is ansiStyles.modifier (reset, bold, dim, …).
	Modifier Group
	// Color is ansiStyles.color, the foreground colours.
	Color ColorGroup
	// BgColor is ansiStyles.bgColor, the background colours.
	BgColor ColorGroup
	// Codes is ansiStyles.codes: each style's open code number mapped to its
	// close code number. (JavaScript's Map also keeps their order.)
	Codes map[int]int
)

// init builds the tables like the module's assembleStyles.
func init() {
	type code struct {
		name        string
		open, close int
	}
	modifier := []code{
		{"reset", 0, 0},
		// 21 isn't widely supported and 22 does the same thing
		{"bold", 1, 22},
		{"dim", 2, 22},
		{"italic", 3, 23},
		{"underline", 4, 24},
		{"inverse", 7, 27},
		{"hidden", 8, 28},
		{"strikethrough", 9, 29},
	}
	color := []code{
		{"black", 30, 39}, {"red", 31, 39}, {"green", 32, 39}, {"yellow", 33, 39},
		{"blue", 34, 39}, {"magenta", 35, 39}, {"cyan", 36, 39}, {"white", 37, 39},
		// Bright colors
		{"blackBright", 90, 39}, {"redBright", 91, 39}, {"greenBright", 92, 39}, {"yellowBright", 93, 39},
		{"blueBright", 94, 39}, {"magentaBright", 95, 39}, {"cyanBright", 96, 39}, {"whiteBright", 97, 39},
		// Bright black aliased as gray and grey
		{"gray", 90, 39}, {"grey", 90, 39},
	}
	bgColor := []code{
		{"bgBlack", 40, 49}, {"bgRed", 41, 49}, {"bgGreen", 42, 49}, {"bgYellow", 43, 49},
		{"bgBlue", 44, 49}, {"bgMagenta", 45, 49}, {"bgCyan", 46, 49}, {"bgWhite", 47, 49},
		{"bgBlackBright", 100, 49}, {"bgRedBright", 101, 49}, {"bgGreenBright", 102, 49}, {"bgYellowBright", 103, 49},
		{"bgBlueBright", 104, 49}, {"bgMagentaBright", 105, 49}, {"bgCyanBright", 106, 49}, {"bgWhiteBright", 107, 49},
		{"bgGray", 100, 49}, {"bgGrey", 100, 49},
	}
	Styles = map[string]Style{}
	Codes = map[int]int{}
	group := func(codes []code) Group {
		g := Group{Styles: map[string]Style{}}
		for _, c := range codes {
			s := Style{"\x1b[" + strconv.Itoa(c.open) + "m", "\x1b[" + strconv.Itoa(c.close) + "m"}
			Names = append(Names, c.name)
			Styles[c.name] = s
			g.Names = append(g.Names, c.name)
			g.Styles[c.name] = s
			Codes[c.open] = c.close
		}
		return g
	}
	Modifier = group(modifier)
	Color = ColorGroup{
		Group:   group(color),
		Close:   "\x1b[39m",
		ANSI:    Dynamic{"ansi16", 0},
		ANSI256: Dynamic{"ansi256", 0},
		ANSI16M: Dynamic{"rgb", 0},
	}
	BgColor = ColorGroup{
		Group:   group(bgColor),
		Close:   "\x1b[49m",
		ANSI:    Dynamic{"ansi16", 10},
		ANSI256: Dynamic{"ansi256", 10},
		ANSI16M: Dynamic{"rgb", 10},
	}
}

// Dynamic is one dynamic colour-code table (ansiStyles.color.ansi, .ansi256,
// .ansi16m, or the bgColor ones). Each method takes a colour in one
// color-convert model, converts it to the table's space with color-convert's
// rounded conversion (a colour already in that space is used as it is, not
// rounded) and returns the escape code. A conversion that throws in
// JavaScript panics (see package colorconvert).
type Dynamic struct {
	target string // the color-convert space: ansi16, ansi256 or rgb
	offset int    // 10 for background colours
}

// RGB is the table's .rgb(r, g, b).
func (d Dynamic) RGB(r, g, b float64) string { return d.code("rgb", []float64{r, g, b}) }

// HSL is the table's .hsl(h, s, l).
func (d Dynamic) HSL(h, s, l float64) string { return d.code("hsl", []float64{h, s, l}) }

// HSV is the table's .hsv(h, s, v).
func (d Dynamic) HSV(h, s, v float64) string { return d.code("hsv", []float64{h, s, v}) }

// HWB is the table's .hwb(h, w, b).
func (d Dynamic) HWB(h, w, b float64) string { return d.code("hwb", []float64{h, w, b}) }

// CMYK is the table's .cmyk(c, m, y, k).
func (d Dynamic) CMYK(c, m, y, k float64) string { return d.code("cmyk", []float64{c, m, y, k}) }

// XYZ is the table's .xyz(x, y, z).
func (d Dynamic) XYZ(x, y, z float64) string { return d.code("xyz", []float64{x, y, z}) }

// Lab is the table's .lab(l, a, b).
func (d Dynamic) Lab(l, a, b float64) string { return d.code("lab", []float64{l, a, b}) }

// LCH is the table's .lch(l, c, h).
func (d Dynamic) LCH(l, c, h float64) string { return d.code("lch", []float64{l, c, h}) }

// Hex is the table's .hex(hex).
func (d Dynamic) Hex(hex string) string { return d.code("hex", hex) }

// Keyword is the table's .keyword(keyword).
func (d Dynamic) Keyword(keyword string) string { return d.code("keyword", keyword) }

// ANSI is the table's .ansi(code), a color-convert ansi16 code.
func (d Dynamic) ANSI(code float64) string { return d.code("ansi", code) }

// ANSI256 is the table's .ansi256(code).
func (d Dynamic) ANSI256(code float64) string { return d.code("ansi256", code) }

// HCG is the table's .hcg(h, c, g).
func (d Dynamic) HCG(h, c, g float64) string { return d.code("hcg", []float64{h, c, g}) }

// Apple is the table's .apple(r16, g16, b16).
func (d Dynamic) Apple(r16, g16, b16 float64) string {
	return d.code("apple", []float64{r16, g16, b16})
}

// Gray is the table's .gray(gray).
func (d Dynamic) Gray(gray float64) string { return d.code("gray", []float64{gray}) }

// code is makeDynamicStyles's table entry for key model (color-convert's
// model name, ansi16 renamed to ansi) called with value.
func (d Dynamic) code(model string, value colorconvert.Value) string {
	source := model
	if model == "ansi" {
		source = "ansi16"
	}
	v := value // ansi2ansi, rgb2rgb: the colour is already in the target space
	if source != d.target {
		v = colorconvert.Convert[source][d.target].Call(value)
	}
	switch d.target {
	case "ansi16": // wrapAnsi16
		return "\x1b[" + jsnum.String(v.(float64)+float64(d.offset)) + "m"
	case "ansi256": // wrapAnsi256
		return "\x1b[" + strconv.Itoa(38+d.offset) + ";5;" + jsnum.String(v.(float64)) + "m"
	}
	rgb, ok := v.([]float64) // wrapAnsi16m reads rgb[0], rgb[1], rgb[2]
	if !ok {
		panic(&jsvalue.TypeError{Message: "Cannot read properties of undefined (reading '0')"})
	}
	return "\x1b[" + strconv.Itoa(38+d.offset) + ";2;" +
		jsnum.String(rgb[0]) + ";" + jsnum.String(rgb[1]) + ";" + jsnum.String(rgb[2]) + "m"
}
