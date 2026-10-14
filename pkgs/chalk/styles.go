package chalk

// The style getters of chalk's prototype (Chalk.prototype and the builders'
// proto): one per ansi-styles style, visible, and the colour models.

import (
	ansistyles "github.com/gav4you/npmgo/pkgs/ansi-styles"
)

// named is a named style's getter: a builder adding the style.
func (c *Chalk) named(name string) *Chalk {
	st := ansistyles.Styles[name]
	return c.createBuilder(createStyler(st.Open, st.Close, c.styler), c.isEmpty)
}

// Visible is chalk.visible: a builder with the same styles that returns ""
// instead of the text when the level is 0 (or the text is empty).
func (c *Chalk) Visible() *Chalk { return c.createBuilder(c.styler, true) }

// Reset is chalk.reset.
func (c *Chalk) Reset() *Chalk { return c.named("reset") }

// Bold is chalk.bold.
func (c *Chalk) Bold() *Chalk { return c.named("bold") }

// Dim is chalk.dim.
func (c *Chalk) Dim() *Chalk { return c.named("dim") }

// Italic is chalk.italic.
func (c *Chalk) Italic() *Chalk { return c.named("italic") }

// Underline is chalk.underline.
func (c *Chalk) Underline() *Chalk { return c.named("underline") }

// Inverse is chalk.inverse.
func (c *Chalk) Inverse() *Chalk { return c.named("inverse") }

// Hidden is chalk.hidden.
func (c *Chalk) Hidden() *Chalk { return c.named("hidden") }

// Strikethrough is chalk.strikethrough.
func (c *Chalk) Strikethrough() *Chalk { return c.named("strikethrough") }

// Black is chalk.black.
func (c *Chalk) Black() *Chalk { return c.named("black") }

// Red is chalk.red.
func (c *Chalk) Red() *Chalk { return c.named("red") }

// Green is chalk.green.
func (c *Chalk) Green() *Chalk { return c.named("green") }

// Yellow is chalk.yellow.
func (c *Chalk) Yellow() *Chalk { return c.named("yellow") }

// Blue is chalk.blue.
func (c *Chalk) Blue() *Chalk { return c.named("blue") }

// Magenta is chalk.magenta.
func (c *Chalk) Magenta() *Chalk { return c.named("magenta") }

// Cyan is chalk.cyan.
func (c *Chalk) Cyan() *Chalk { return c.named("cyan") }

// White is chalk.white.
func (c *Chalk) White() *Chalk { return c.named("white") }

// BlackBright is chalk.blackBright.
func (c *Chalk) BlackBright() *Chalk { return c.named("blackBright") }

// RedBright is chalk.redBright.
func (c *Chalk) RedBright() *Chalk { return c.named("redBright") }

// GreenBright is chalk.greenBright.
func (c *Chalk) GreenBright() *Chalk { return c.named("greenBright") }

// YellowBright is chalk.yellowBright.
func (c *Chalk) YellowBright() *Chalk { return c.named("yellowBright") }

// BlueBright is chalk.blueBright.
func (c *Chalk) BlueBright() *Chalk { return c.named("blueBright") }

// MagentaBright is chalk.magentaBright.
func (c *Chalk) MagentaBright() *Chalk { return c.named("magentaBright") }

// CyanBright is chalk.cyanBright.
func (c *Chalk) CyanBright() *Chalk { return c.named("cyanBright") }

// WhiteBright is chalk.whiteBright.
func (c *Chalk) WhiteBright() *Chalk { return c.named("whiteBright") }

// Gray is chalk.gray.
func (c *Chalk) Gray() *Chalk { return c.named("gray") }

// Grey is chalk.grey.
func (c *Chalk) Grey() *Chalk { return c.named("grey") }

// BgBlack is chalk.bgBlack.
func (c *Chalk) BgBlack() *Chalk { return c.named("bgBlack") }

// BgRed is chalk.bgRed.
func (c *Chalk) BgRed() *Chalk { return c.named("bgRed") }

// BgGreen is chalk.bgGreen.
func (c *Chalk) BgGreen() *Chalk { return c.named("bgGreen") }

// BgYellow is chalk.bgYellow.
func (c *Chalk) BgYellow() *Chalk { return c.named("bgYellow") }

// BgBlue is chalk.bgBlue.
func (c *Chalk) BgBlue() *Chalk { return c.named("bgBlue") }

// BgMagenta is chalk.bgMagenta.
func (c *Chalk) BgMagenta() *Chalk { return c.named("bgMagenta") }

// BgCyan is chalk.bgCyan.
func (c *Chalk) BgCyan() *Chalk { return c.named("bgCyan") }

// BgWhite is chalk.bgWhite.
func (c *Chalk) BgWhite() *Chalk { return c.named("bgWhite") }

// BgBlackBright is chalk.bgBlackBright.
func (c *Chalk) BgBlackBright() *Chalk { return c.named("bgBlackBright") }

// BgRedBright is chalk.bgRedBright.
func (c *Chalk) BgRedBright() *Chalk { return c.named("bgRedBright") }

// BgGreenBright is chalk.bgGreenBright.
func (c *Chalk) BgGreenBright() *Chalk { return c.named("bgGreenBright") }

// BgYellowBright is chalk.bgYellowBright.
func (c *Chalk) BgYellowBright() *Chalk { return c.named("bgYellowBright") }

// BgBlueBright is chalk.bgBlueBright.
func (c *Chalk) BgBlueBright() *Chalk { return c.named("bgBlueBright") }

// BgMagentaBright is chalk.bgMagentaBright.
func (c *Chalk) BgMagentaBright() *Chalk { return c.named("bgMagentaBright") }

// BgCyanBright is chalk.bgCyanBright.
func (c *Chalk) BgCyanBright() *Chalk { return c.named("bgCyanBright") }

// BgWhiteBright is chalk.bgWhiteBright.
func (c *Chalk) BgWhiteBright() *Chalk { return c.named("bgWhiteBright") }

// BgGray is chalk.bgGray.
func (c *Chalk) BgGray() *Chalk { return c.named("bgGray") }

// BgGrey is chalk.bgGrey.
func (c *Chalk) BgGrey() *Chalk { return c.named("bgGrey") }

// RGB is chalk.rgb(r, g, b): a foreground colour. Level 3 writes the values
// as they are (fractions, negatives, NaN printed as JavaScript prints them);
// levels 1 and 2 convert them with color-convert to the 16 / 256 colours.
func (c *Chalk) RGB(r, g, b float64) *Chalk {
	return c.model("rgb", false, func(d ansistyles.Dynamic) string { return d.RGB(r, g, b) })
}

// Hex is chalk.hex(color): a foreground colour from a hex string ("#FF8800",
// "f80"; the first run of six or three hex digits counts, none is black).
func (c *Chalk) Hex(color string) *Chalk {
	return c.model("hex", false, func(d ansistyles.Dynamic) string { return d.Hex(color) })
}

// Keyword is chalk.keyword(name): a foreground colour from a CSS keyword. An
// unknown keyword panics with the TypeError JavaScript throws.
func (c *Chalk) Keyword(name string) *Chalk {
	return c.model("keyword", false, func(d ansistyles.Dynamic) string { return d.Keyword(name) })
}

// HSL is chalk.hsl(h, s, l): a foreground colour.
func (c *Chalk) HSL(h, s, l float64) *Chalk {
	return c.model("hsl", false, func(d ansistyles.Dynamic) string { return d.HSL(h, s, l) })
}

// HSV is chalk.hsv(h, s, v): a foreground colour. A hue whose floor is
// negative or NaN panics with the TypeError JavaScript throws.
func (c *Chalk) HSV(h, s, v float64) *Chalk {
	return c.model("hsv", false, func(d ansistyles.Dynamic) string { return d.HSV(h, s, v) })
}

// HWB is chalk.hwb(h, w, b): a foreground colour.
func (c *Chalk) HWB(h, w, b float64) *Chalk {
	return c.model("hwb", false, func(d ansistyles.Dynamic) string { return d.HWB(h, w, b) })
}

// ANSI is chalk.ansi(code): a foreground colour from a 16-colour code
// (30-37, 90-97), written as it is at levels 0 and 1.
func (c *Chalk) ANSI(code float64) *Chalk {
	return c.model("ansi", false, func(d ansistyles.Dynamic) string { return d.ANSI(code) })
}

// ANSI256 is chalk.ansi256(code): a foreground colour from the 256-colour
// palette, written as it is at level 2.
func (c *Chalk) ANSI256(code float64) *Chalk {
	return c.model("ansi256", false, func(d ansistyles.Dynamic) string { return d.ANSI256(code) })
}

// BgRGB is chalk.bgRgb(r, g, b): a background colour (see RGB).
func (c *Chalk) BgRGB(r, g, b float64) *Chalk {
	return c.model("rgb", true, func(d ansistyles.Dynamic) string { return d.RGB(r, g, b) })
}

// BgHex is chalk.bgHex(color): a background colour (see Hex).
func (c *Chalk) BgHex(color string) *Chalk {
	return c.model("hex", true, func(d ansistyles.Dynamic) string { return d.Hex(color) })
}

// BgKeyword is chalk.bgKeyword(name): a background colour (see Keyword).
func (c *Chalk) BgKeyword(name string) *Chalk {
	return c.model("keyword", true, func(d ansistyles.Dynamic) string { return d.Keyword(name) })
}

// BgHSL is chalk.bgHsl(h, s, l): a background colour.
func (c *Chalk) BgHSL(h, s, l float64) *Chalk {
	return c.model("hsl", true, func(d ansistyles.Dynamic) string { return d.HSL(h, s, l) })
}

// BgHSV is chalk.bgHsv(h, s, v): a background colour (see HSV).
func (c *Chalk) BgHSV(h, s, v float64) *Chalk {
	return c.model("hsv", true, func(d ansistyles.Dynamic) string { return d.HSV(h, s, v) })
}

// BgHWB is chalk.bgHwb(h, w, b): a background colour.
func (c *Chalk) BgHWB(h, w, b float64) *Chalk {
	return c.model("hwb", true, func(d ansistyles.Dynamic) string { return d.HWB(h, w, b) })
}

// BgANSI is chalk.bgAnsi(code): a background colour from a 16-colour
// foreground code (31 gives the red background, 41).
func (c *Chalk) BgANSI(code float64) *Chalk {
	return c.model("ansi", true, func(d ansistyles.Dynamic) string { return d.ANSI(code) })
}

// BgANSI256 is chalk.bgAnsi256(code): a background colour from the
// 256-colour palette.
func (c *Chalk) BgANSI256(code float64) *Chalk {
	return c.model("ansi256", true, func(d ansistyles.Dynamic) string { return d.ANSI256(code) })
}

// models are the colour-model getters by name: the model's color-convert
// table key and whether it is a background colour.
var models = map[string]struct {
	key string
	bg  bool
}{
	"rgb": {"rgb", false}, "hex": {"hex", false}, "keyword": {"keyword", false}, "hsl": {"hsl", false},
	"hsv": {"hsv", false}, "hwb": {"hwb", false}, "ansi": {"ansi", false}, "ansi256": {"ansi256", false},
	"bgRgb": {"rgb", true}, "bgHex": {"hex", true}, "bgKeyword": {"keyword", true}, "bgHsl": {"hsl", true},
	"bgHsv": {"hsv", true}, "bgHwb": {"hwb", true}, "bgAnsi": {"ansi", true}, "bgAnsi256": {"ansi256", true},
}

// isStyle reports whether name is a getter of chalk's prototype that
// returns a builder: an ansi-styles style or visible.
func isStyle(name string) bool {
	_, ok := ansistyles.Styles[name]
	return ok || name == "visible"
}

// styleByName is the getter chalk[name] for a style or visible.
func (c *Chalk) styleByName(name string) *Chalk {
	if name == "visible" {
		return c.Visible()
	}
	return c.named(name)
}
