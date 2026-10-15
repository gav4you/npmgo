// Copyright (c) 2014-present Sebastian McKenzie and other contributors. MIT License (see LICENSE).
// Translated to Go by the npmgo contributors from @babel/helper-string-parser 7.27.1 (lib/index.js).

// Package helperstringparser is a translation of @babel/helper-string-parser 7.27.1: the readers of string and
// template literal contents, integers and code-point escapes that @babel/parser and @babel/types share.
//
// Input strings are read as UTF-16 code units and positions count code units, as in JavaScript. Positions, line
// numbers and radixes are ints (JavaScript accepts any number; a fractional one is not modelled). A string result is
// assembled from code units, so two escapes that produce the halves of a surrogate pair join, as in JavaScript; a lone
// surrogate left in a result becomes U+FFFD (Go strings cannot hold one).
package helperstringparser

import (
	"math"
	"strconv"
	"unicode/utf16"

	"github.com/gav4you/npmgo/internal/jsnum"
	"github.com/gav4you/npmgo/internal/jsvalue"
)

// ErrorHandlers is the errors object the readers report to. A nil function is a missing method: calling it panics
// with JavaScript's TypeError ("errors.unterminated is not a function"). InvalidDigit returns whether the error was
// handled and reading should go on.
type ErrorHandlers struct {
	Unterminated                     func(initialPos, initialLineStart, initialCurLine int)
	StrictNumericEscape              func(pos, lineStart, curLine int)
	InvalidEscapeSequence            func(pos, lineStart, curLine int)
	NumericSeparatorInEscapeSequence func(pos, lineStart, curLine int)
	UnexpectedNumericSeparator       func(pos, lineStart, curLine int)
	InvalidDigit                     func(pos, lineStart, curLine, radix int) bool
	InvalidCodePoint                 func(pos, lineStart, curLine int)
}

func missing(name string) {
	panic(&jsvalue.TypeError{Message: "errors." + name + " is not a function"})
}

func (e *ErrorHandlers) unterminated(pos, lineStart, curLine int) {
	if e.Unterminated == nil {
		missing("unterminated")
	}
	e.Unterminated(pos, lineStart, curLine)
}

func (e *ErrorHandlers) strictNumericEscape(pos, lineStart, curLine int) {
	if e.StrictNumericEscape == nil {
		missing("strictNumericEscape")
	}
	e.StrictNumericEscape(pos, lineStart, curLine)
}

func (e *ErrorHandlers) invalidEscapeSequence(pos, lineStart, curLine int) {
	if e.InvalidEscapeSequence == nil {
		missing("invalidEscapeSequence")
	}
	e.InvalidEscapeSequence(pos, lineStart, curLine)
}

func (e *ErrorHandlers) numericSeparatorInEscapeSequence(pos, lineStart, curLine int) {
	if e.NumericSeparatorInEscapeSequence == nil {
		missing("numericSeparatorInEscapeSequence")
	}
	e.NumericSeparatorInEscapeSequence(pos, lineStart, curLine)
}

func (e *ErrorHandlers) unexpectedNumericSeparator(pos, lineStart, curLine int) {
	if e.UnexpectedNumericSeparator == nil {
		missing("unexpectedNumericSeparator")
	}
	e.UnexpectedNumericSeparator(pos, lineStart, curLine)
}

func (e *ErrorHandlers) invalidDigit(pos, lineStart, curLine, radix int) bool {
	if e.InvalidDigit == nil {
		missing("invalidDigit")
	}
	return e.InvalidDigit(pos, lineStart, curLine, radix)
}

func (e *ErrorHandlers) invalidCodePoint(pos, lineStart, curLine int) {
	if e.InvalidCodePoint == nil {
		missing("invalidCodePoint")
	}
	e.InvalidCodePoint(pos, lineStart, curLine)
}

// Loc is a {pos, lineStart, curLine} location.
type Loc struct{ Pos, LineStart, CurLine int }

// StringContentsResult is readStringContents' result.
type StringContentsResult struct {
	Pos             int
	Str             string
	FirstInvalidLoc *Loc // nil where JavaScript has null
	LineStart       int
	CurLine         int
	ContainsInvalid bool
}

// IntResult is readInt's result: N is nil where JavaScript returns null, a float64 otherwise.
type IntResult struct {
	N   any
	Pos int
}

// CodePointResult is readCodePoint's result: Code is nil where JavaScript returns null, a float64 otherwise.
type CodePointResult struct {
	Code any
	Pos  int
}

// text is a string as UTF-16 code units.
type text []uint16

// at is charCodeAt: -1 stands for the NaN of a position outside the string (every comparison with it fails as with
// NaN, and it is no digit).
func (t text) at(i int) int {
	if i < 0 || i >= len(t) {
		return -1
	}
	return int(t[i])
}

// slice is String.prototype.slice(start, end), negative positions counting from the end.
func (t text) slice(start, end int) text {
	n := len(t)
	clamp := func(i int) int {
		if i < 0 {
			i += n
			if i < 0 {
				i = 0
			}
		}
		if i > n {
			i = n
		}
		return i
	}
	s, e := clamp(start), clamp(end)
	if s >= e {
		return nil
	}
	return t[s:e]
}

// indexOf is String.prototype.indexOf(c, from) for a one-unit search string.
func (t text) indexOf(c uint16, from int) int {
	if from < 0 {
		from = 0
	}
	for i := from; i < len(t); i++ {
		if t[i] == c {
			return i
		}
	}
	return -1
}

func decode(units []uint16) string { return string(utf16.Decode(units)) }

var nullText = []uint16{'n', 'u', 'l', 'l'}

const nan = -1 // the NaN of text.at

func isDigit(code int) bool { return code >= 48 && code <= 57 }

var forbiddenNumericSeparatorSiblings = struct{ decBinOct, hex map[int]bool }{
	decBinOct: map[int]bool{46: true, 66: true, 69: true, 79: true, 95: true, 98: true, 101: true, 111: true},
	hex:       map[int]bool{46: true, 88: true, 95: true, 120: true},
}

var isAllowedNumericSeparatorSibling = struct{ bin, oct, dec, hex func(ch int) bool }{
	bin: func(ch int) bool { return ch == 48 || ch == 49 },
	oct: func(ch int) bool { return ch >= 48 && ch <= 55 },
	dec: func(ch int) bool { return ch >= 48 && ch <= 57 },
	hex: func(ch int) bool { return ch >= 48 && ch <= 57 || ch >= 65 && ch <= 70 || ch >= 97 && ch <= 102 },
}

// ReadStringContents is readStringContents(type, input, pos, lineStart, curLine, errors): it reads the contents of a
// string literal ("double", or any other type for single quotes) or a template ("template") from pos up to the
// closing quote, the backtick or "${", or the end of the input.
func ReadStringContents(typ string, input string, pos, lineStart, curLine int, errors ErrorHandlers) StringContentsResult {
	in := text(utf16.Encode([]rune(input)))
	initialPos := pos
	initialLineStart := lineStart
	initialCurLine := curLine
	var out []uint16
	var firstInvalidLoc *Loc
	chunkStart := pos
	length := len(in)
	for {
		if pos >= length {
			errors.unterminated(initialPos, initialLineStart, initialCurLine)
			out = append(out, in.slice(chunkStart, pos)...)
			break
		}
		ch := in.at(pos)
		if isStringEnd(typ, ch, in, pos) {
			out = append(out, in.slice(chunkStart, pos)...)
			break
		}
		if ch == 92 {
			out = append(out, in.slice(chunkStart, pos)...)
			res := readEscapedChar(in, pos, lineStart, curLine, typ == "template", &errors)
			if res.null && firstInvalidLoc == nil {
				firstInvalidLoc = &Loc{pos, lineStart, curLine}
			} else if res.null {
				out = append(out, nullText...) // out += null
			} else {
				out = append(out, res.ch...)
			}
			pos, lineStart, curLine = res.pos, res.lineStart, res.curLine
			chunkStart = pos
		} else if ch == 8232 || ch == 8233 {
			pos++
			curLine++
			lineStart = pos
		} else if ch == 10 || ch == 13 {
			if typ == "template" {
				out = append(out, in.slice(chunkStart, pos)...)
				out = append(out, '\n')
				pos++
				if ch == 13 && in.at(pos) == 10 {
					pos++
				}
				curLine++
				lineStart = pos
				chunkStart = pos
			} else {
				errors.unterminated(initialPos, initialLineStart, initialCurLine)
			}
		} else {
			pos++
		}
	}
	return StringContentsResult{
		Pos:             pos,
		Str:             decode(out),
		FirstInvalidLoc: firstInvalidLoc,
		LineStart:       lineStart,
		CurLine:         curLine,
		ContainsInvalid: firstInvalidLoc != nil,
	}
}

func isStringEnd(typ string, ch int, in text, pos int) bool {
	if typ == "template" {
		return ch == 96 || ch == 36 && in.at(pos+1) == 123
	}
	if typ == "double" {
		return ch == 34
	}
	return ch == 39
}

// escaped is readEscapedChar's result: ch is the text of the escape, null stands for JavaScript's null.
type escaped struct {
	pos       int
	ch        []uint16
	null      bool
	lineStart int
	curLine   int
}

// fromCharCode is String.fromCharCode(code) for one number (ToUint16; NaN gives U+0000).
func fromCharCode(code float64) []uint16 {
	if math.IsNaN(code) || math.IsInf(code, 0) {
		return []uint16{0}
	}
	return []uint16{uint16(int64(math.Trunc(code)))}
}

// fromCodePoint is String.fromCodePoint(code): a RangeError for anything but an integer in [0, 0x10FFFF].
func fromCodePoint(code float64) []uint16 {
	if code != math.Trunc(code) || code < 0 || code > 0x10ffff {
		panic(&jsvalue.RangeError{Message: "Invalid code point " + jsnum.String(code)})
	}
	if code < 0x10000 {
		return []uint16{uint16(code)}
	}
	r1, r2 := utf16.EncodeRune(rune(code))
	return []uint16{uint16(r1), uint16(r2)}
}

func readEscapedChar(in text, pos, lineStart, curLine int, inTemplate bool, errors *ErrorHandlers) escaped {
	throwOnInvalid := !inTemplate
	pos++
	res := func(ch []uint16, null bool) escaped { return escaped{pos, ch, null, lineStart, curLine} }
	ch := in.at(pos)
	pos++
	switch ch {
	case 110:
		return res([]uint16{'\n'}, false)
	case 114:
		return res([]uint16{'\r'}, false)
	case 120:
		var code float64
		var ok bool
		code, ok, pos = readHexChar(in, pos, lineStart, curLine, 2, true, false, throwOnInvalid, errors)
		if !ok {
			return res(nil, true)
		}
		return res(fromCharCode(code), false)
	case 117:
		var code float64
		var ok bool
		code, ok, pos = readCodePoint(in, pos, lineStart, curLine, throwOnInvalid, errors)
		if !ok {
			return res(nil, true)
		}
		return res(fromCodePoint(code), false)
	case 116:
		return res([]uint16{'\t'}, false)
	case 98:
		return res([]uint16{'\b'}, false)
	case 118:
		return res([]uint16{'\v'}, false)
	case 102:
		return res([]uint16{'\f'}, false)
	case 13, 10, 8232, 8233:
		if ch == 13 && in.at(pos) == 10 {
			pos++
		}
		if ch == 13 || ch == 10 {
			lineStart = pos
			curLine++
		}
		return res([]uint16{}, false)
	case 56, 57:
		if inTemplate {
			return res(nil, true)
		}
		errors.strictNumericEscape(pos-1, lineStart, curLine)
	}
	if ch >= 48 && ch <= 55 {
		startPos := pos - 1
		window := in.slice(startPos, pos+2)
		n := 0
		for n < len(window) && window[n] >= '0' && window[n] <= '7' {
			n++
		}
		octalStr := window[:n] // /^[0-7]+/ matches: ch itself is an octal digit
		octal := parseOctal(octalStr)
		if octal > 255 {
			octalStr = octalStr[:len(octalStr)-1]
			octal = parseOctal(octalStr)
		}
		pos += len(octalStr) - 1
		next := in.at(pos)
		if !(len(octalStr) == 1 && octalStr[0] == '0') || next == 56 || next == 57 {
			if inTemplate {
				return res(nil, true)
			}
			errors.strictNumericEscape(startPos, lineStart, curLine)
		}
		return res(fromCharCode(float64(octal)), false)
	}
	if ch == nan {
		return res(fromCharCode(math.NaN()), false)
	}
	return res(fromCharCode(float64(ch)), false)
}

func parseOctal(digits []uint16) int {
	v, _ := strconv.ParseInt(string(utf16.Decode(digits)), 8, 64)
	return int(v)
}

// readHexChar reads a hexadecimal escape of len digits (hasLen false for no length limit).
func readHexChar(in text, pos, lineStart, curLine int, length float64, hasLen, forceLen, throwOnInvalid bool, errors *ErrorHandlers) (code float64, ok bool, newPos int) {
	initialPos := pos
	var n float64
	n, ok, pos = readInt(in, pos, lineStart, curLine, 16, length, hasLen, forceLen, false, errors, !throwOnInvalid)
	if !ok {
		if throwOnInvalid {
			errors.invalidEscapeSequence(initialPos, lineStart, curLine)
		} else {
			pos = initialPos - 1
		}
	}
	return n, ok, pos
}

// readInt reads digits of radix from pos; ok false is JavaScript's null. allowNumSeparator is a JavaScript value:
// false, true or "bail".
func readInt(in text, pos, lineStart, curLine int, radix int, length float64, hasLen, forceLen bool, allowNumSeparator any, errors *ErrorHandlers, bailOnError bool) (n float64, ok bool, newPos int) {
	start := pos
	forbiddenSiblings := forbiddenNumericSeparatorSiblings.decBinOct
	if radix == 16 {
		forbiddenSiblings = forbiddenNumericSeparatorSiblings.hex
	}
	var isAllowedSibling func(int) bool
	switch radix {
	case 16:
		isAllowedSibling = isAllowedNumericSeparatorSibling.hex
	case 10:
		isAllowedSibling = isAllowedNumericSeparatorSibling.dec
	case 8:
		isAllowedSibling = isAllowedNumericSeparatorSibling.oct
	default:
		isAllowedSibling = isAllowedNumericSeparatorSibling.bin
	}
	invalid := false
	total := 0.0
	e := math.Inf(1)
	if hasLen {
		e = length
	}
	for i := 0.0; i < e; i++ {
		code := in.at(pos)
		var val float64
		if code == 95 && !(allowNumSeparator == "bail") {
			prev := in.at(pos - 1)
			next := in.at(pos + 1)
			if !jsvalue.Truthy(allowNumSeparator) {
				if bailOnError {
					return 0, false, pos
				}
				errors.numericSeparatorInEscapeSequence(pos, lineStart, curLine)
			} else if next == nan || !isAllowedSibling(next) || forbiddenSiblings[prev] || forbiddenSiblings[next] {
				if bailOnError {
					return 0, false, pos
				}
				errors.unexpectedNumericSeparator(pos, lineStart, curLine)
			}
			pos++
			continue
		}
		switch {
		case code >= 97:
			val = float64(code - 97 + 10)
		case code >= 65:
			val = float64(code - 65 + 10)
		case isDigit(code):
			val = float64(code - 48)
		default:
			val = math.Inf(1)
		}
		if val >= float64(radix) {
			if val <= 9 && bailOnError {
				return 0, false, pos
			} else if val <= 9 && errors.invalidDigit(pos, lineStart, curLine, radix) {
				val = 0
			} else if forceLen {
				val = 0
				invalid = true
			} else {
				break
			}
		}
		pos++
		total = total*float64(radix) + val
	}
	if pos == start || hasLen && float64(pos-start) != length || invalid {
		return 0, false, pos
	}
	return total, true, pos
}

func readCodePoint(in text, pos, lineStart, curLine int, throwOnInvalid bool, errors *ErrorHandlers) (code float64, ok bool, newPos int) {
	ch := in.at(pos)
	if ch == 123 {
		pos++
		code, ok, pos = readHexChar(in, pos, lineStart, curLine, float64(in.indexOf('}', pos)-pos), true, true, throwOnInvalid, errors)
		pos++
		if ok && code > 0x10ffff {
			if throwOnInvalid {
				errors.invalidCodePoint(pos, lineStart, curLine)
			} else {
				return 0, false, pos
			}
		}
	} else {
		code, ok, pos = readHexChar(in, pos, lineStart, curLine, 4, true, false, throwOnInvalid, errors)
	}
	return code, ok, pos
}

// lengthArg reads readInt's len argument: undefined or null for no limit, a number otherwise.
func lengthArg(len any) (float64, bool) {
	if jsvalue.IsNullish(len) {
		return 0, false
	}
	return jsvalue.ToNumber(len), true
}

// ReadInt is readInt(input, pos, lineStart, curLine, radix, len, forceLen, allowNumSeparator, errors, bailOnError):
// it reads an integer of radix from pos. len is undefined or nil for no length, or a number of units to read;
// allowNumSeparator is false, true or "bail".
func ReadInt(input string, pos, lineStart, curLine int, radix int, len any, forceLen bool, allowNumSeparator any, errors ErrorHandlers, bailOnError bool) IntResult {
	in := text(utf16.Encode([]rune(input)))
	length, hasLen := lengthArg(len)
	n, ok, pos := readInt(in, pos, lineStart, curLine, radix, length, hasLen, forceLen, allowNumSeparator, &errors, bailOnError)
	if !ok {
		return IntResult{nil, pos}
	}
	return IntResult{n, pos}
}

// ReadCodePoint is readCodePoint(input, pos, lineStart, curLine, throwOnInvalid, errors): it reads the code point of
// a \u escape whose "u" is just before pos (four hexadecimal digits, or any number of them in braces).
func ReadCodePoint(input string, pos, lineStart, curLine int, throwOnInvalid bool, errors ErrorHandlers) CodePointResult {
	in := text(utf16.Encode([]rune(input)))
	code, ok, pos := readCodePoint(in, pos, lineStart, curLine, throwOnInvalid, &errors)
	if !ok {
		return CodePointResult{nil, pos}
	}
	return CodePointResult{code, pos}
}
