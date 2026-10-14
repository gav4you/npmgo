// Package chalk is a Go translation of the npm package chalk@4.1.2
// (https://github.com/chalk/chalk): terminal string styling. The original
// copyright notice and MIT license are preserved in LICENSE.
//
//	c := chalk.New(chalk.Options{Level: 2})
//	fmt.Println(c.Red().Bold().Sprint("error:"), c.Hex("#FF8800").Sprint("warning"))
//	s, err := c.Template("{bold.underline CPU:} {red 99%}")
//
// A *Chalk is a chalk instance (require('chalk'), new chalk.Instance()) or a
// builder (chalk.red, chalk.red.bold, …). Each style method is the getter of
// the same name: it returns a builder that adds the style to the receiver's.
// Sprint is calling it, Tag and Template are calling it as a tagged
// template. JavaScript caches each getter's builder on its object; here a
// style method returns a new builder each time, which is not observable.
//
// The level decides how colours are written: 0 writes no styles at all, 1
// uses the 16 basic colours, 2 the 256-colour palette and 3 truecolor. A
// builder reads the level of its instance when it styles, so SetLevel on an
// instance (or on any builder, which sets its instance's) affects every
// builder derived from it. The colour-model methods (RGB, Hex, …) choose
// their codes for the level at the time they are called.
//
// Where JavaScript throws, Go panics with an error carrying the same message:
// a level option outside 0 to 3 (an error), a colour-model method whose
// colour cannot be converted (an unknown keyword, an HSV hue whose floor is
// negative or NaN) or that is called while the level is outside 0 to 3 (a
// *jsvalue.TypeError with V8's message). Template parse errors are returned.
//
// Builders are immutable and the level is stored atomically, so a Chalk may
// be used from several goroutines. Default, its Stderr and their
// SupportsColor fields are set when the package is initialised.
package chalk

import (
	"errors"
	"strings"
	"sync/atomic"

	"github.com/gav4you/npmgo/internal/jsvalue"
	ansistyles "github.com/gav4you/npmgo/pkgs/ansi-styles"
	supportscolor "github.com/gav4you/npmgo/pkgs/supports-color"
)

// Options is the options object of new chalk.Instance(options).
type Options struct {
	// Level is options.level: 0 (no colour), 1 (16 colours), 2 (256) or 3
	// (truecolor). Any other value panics with chalk's error.
	Level int
}

// Chalk is a chalk instance or a style builder.
type Chalk struct {
	// SupportsColor is chalk.supportsColor: the supports-color result the
	// level was detected from (stdout's for the module instance, stderr's
	// for its Stderr), nil where JavaScript has false. Only the instances
	// Load returns have it.
	SupportsColor *supportscolor.Level
	// Stderr is chalk.stderr: an instance whose level is supports-color's
	// stderr level. Only the instance Load returns has it.
	Stderr *Chalk

	gen     *generator
	styler  *styler // _styler; nil for an instance
	isEmpty bool    // _isEmpty
	builder bool    // false for an instance, which joins without styling
}

// generator is the state an instance shares with its builders: chalk.level,
// and the stdout level of the module it belongs to, which new
// chalk.Instance() takes when no level is given.
type generator struct {
	level    atomic.Int64
	detected int
}

// styler is createStyler's result: a style's codes and those of all the
// styles under it.
type styler struct {
	open, close, openAll, closeAll string
	parent                         *styler
}

const levelError = "The `level` option should be an integer from 0 to 3"

// Load evaluates chalk's module body (source/index.js) with supports-color's
// stdout and stderr results (nil where supports-color has false) and returns
// the module export: an instance at stdout's level whose SupportsColor is
// stdout and whose Stderr is an instance at stderr's level. Like
// require('chalk'), it panics with the level error when stderr's level is
// negative (FORCE_COLOR=-1 and nothing else detected).
func Load(stdout, stderr *supportscolor.Level) *Chalk {
	detected := 0
	if stdout != nil {
		detected = stdout.Level
	}
	c := newInstance(detected, nil)
	c.SupportsColor = stdout
	errLevel := 0
	if stderr != nil {
		errLevel = stderr.Level
	}
	c.Stderr = newInstance(detected, &Options{Level: errLevel})
	c.Stderr.SupportsColor = stderr
	return c
}

// Default is require('chalk'): Load for this process's supports-color
// results (supportscolor.Stdout and supportscolor.Stderr).
var Default = Load(supportscolor.Stdout, supportscolor.Stderr)

// New is new chalk.Instance(options), Default.Instance(options...).
func New(options ...Options) *Chalk { return Default.Instance(options...) }

// Instance is new chalk.Instance(options): a new instance with its own
// level, options[0].Level, or, without options (new chalk.Instance() or
// new chalk.Instance({})), the stdout level detected when the module was
// loaded. Further options are ignored.
func (c *Chalk) Instance(options ...Options) *Chalk {
	if len(options) == 0 {
		return newInstance(c.gen.detected, nil)
	}
	return newInstance(c.gen.detected, &options[0])
}

// newInstance is chalkFactory with applyOptions.
func newInstance(detected int, o *Options) *Chalk {
	g := &generator{detected: detected}
	level := detected
	if o != nil {
		if o.Level != 0 && (o.Level < 0 || o.Level > 3) {
			panic(errors.New(levelError))
		}
		level = o.Level
	}
	g.level.Store(int64(level))
	return &Chalk{gen: g}
}

// Level is chalk.level; a builder reports its instance's.
func (c *Chalk) Level() int { return int(c.gen.level.Load()) }

// SetLevel sets chalk.level; on a builder it sets its instance's. Like the
// setter it does not validate: any level above 0 writes the styles, and the
// colour-model methods panic while it is outside 0 to 3.
func (c *Chalk) SetLevel(level int) { c.gen.level.Store(int64(level)) }

// Sprint is calling the chalk, chalk.red(...args): the arguments joined with
// spaces (Array.prototype.join, so nil and undefined add nothing), or a single
// argument converted like "" + arg (nil gives "null"), then wrapped in the
// builder's styles. Calling an instance, chalk(...args), only joins.
// Arguments convert as in JavaScript: numbers as Number.prototype.toString
// prints them, booleans as true / false, slices as their elements joined with
// commas, errors as "Error: " + message, fmt.Stringers through String; any
// other value prints "[object Object]".
func (c *Chalk) Sprint(args ...any) string {
	if !c.builder {
		return jsvalue.Join(args, " ")
	}
	if len(args) == 1 {
		return c.applyStyle(jsvalue.ToString(args[0]))
	}
	return c.applyStyle(jsvalue.Join(args, " "))
}

func createStyler(open, close string, parent *styler) *styler {
	if parent == nil {
		return &styler{open: open, close: close, openAll: open, closeAll: close}
	}
	return &styler{open: open, close: close, openAll: parent.openAll + open, closeAll: close + parent.closeAll, parent: parent}
}

func (c *Chalk) createBuilder(s *styler, isEmpty bool) *Chalk {
	return &Chalk{gen: c.gen, styler: s, isEmpty: isEmpty, builder: true}
}

// applyStyle wraps s in the builder's styles. Close codes of the same
// styles already inside s are followed by their open codes again, so that
// nested styles do not end the outer ones early, and each line break is
// enclosed in the codes (CRLF kept together) so that styles do not bleed
// into the next line.
func (c *Chalk) applyStyle(s string) string {
	if c.Level() <= 0 || s == "" {
		if c.isEmpty {
			return ""
		}
		return s
	}
	st := c.styler
	if st == nil {
		return s
	}
	openAll, closeAll := st.openAll, st.closeAll
	if strings.Contains(s, "\x1b") {
		for ; st != nil; st = st.parent {
			// Replace any instances already present with a re-opening code,
			// otherwise only the part of the string until said closing code
			// would be coloured and the rest would be plain.
			s = stringReplaceAll(s, st.close, st.open)
		}
	}
	// Close the styling before a line break and reopen it after the next
	// line, to fix a bleed issue on macOS (chalk/chalk#92).
	if lf := strings.IndexByte(s, '\n'); lf != -1 {
		s = stringEncaseCRLFWithFirstIndex(s, closeAll, openAll, lf)
	}
	return openAll + s + closeAll
}

// levelDynamic is ansiStyles.color[levelMapping[level]] (or bgColor's): the
// colour-code table for the chalk's level. A level outside 0 to 3 has no
// table, and reading the model's function from undefined throws.
func (c *Chalk) levelDynamic(model string, bg bool) ansistyles.Dynamic {
	g := &colorGroup
	if bg {
		g = &bgColorGroup
	}
	switch c.Level() {
	case 0, 1:
		return g.ANSI
	case 2:
		return g.ANSI256
	case 3:
		return g.ANSI16M
	}
	panic(&jsvalue.TypeError{Message: "Cannot read properties of undefined (reading '" + model + "')"})
}

// model is the function a colour-model getter returns, called: a builder
// adding the code that code computes from the level's table, closed by the
// colour (or background colour) reset.
func (c *Chalk) model(model string, bg bool, code func(ansistyles.Dynamic) string) *Chalk {
	open := code(c.levelDynamic(model, bg))
	close := colorGroup.Close
	if bg {
		close = bgColorGroup.Close
	}
	return c.createBuilder(createStyler(open, close, c.styler), c.isEmpty)
}
