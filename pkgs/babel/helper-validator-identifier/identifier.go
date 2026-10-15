// Copyright (c) 2014-present Sebastian McKenzie and other contributors. MIT License (see LICENSE).
// Translated to Go by the npmgo contributors from @babel/helper-validator-identifier 7.28.5 (lib/identifier.js,
// lib/keyword.js).

// Package helpervalidatoridentifier is a translation of @babel/helper-validator-identifier 7.28.5: whether a code
// point can start or continue a JavaScript identifier (Unicode 17.0, the tables of the original), whether a string is
// an identifier name, and the reserved-word predicates.
//
// Code points are float64, as the JavaScript functions take any number (a fraction, NaN or a value outside the code
// point range answers as it does there). Strings are read as UTF-16 code units, as JavaScript reads them.
package helpervalidatoridentifier

import (
	"sort"
	"unicode/utf16"
)

// unitRange is an inclusive range of UTF-16 code units.
type unitRange struct{ lo, hi uint16 }

// classRanges reads chars as the inside of a RegExp character class: "a-b" is a range, anything else one unit. The
// result is sorted and merged (membership is all that the class is used for).
func classRanges(chars string) []unitRange {
	units := utf16.Encode([]rune(chars))
	var ranges []unitRange
	for i := 0; i < len(units); i++ {
		if i+2 < len(units) && units[i+1] == '-' {
			ranges = append(ranges, unitRange{units[i], units[i+2]})
			i += 2
		} else {
			ranges = append(ranges, unitRange{units[i], units[i]})
		}
	}
	sort.Slice(ranges, func(i, j int) bool { return ranges[i].lo < ranges[j].lo })
	merged := ranges[:0:0]
	for _, r := range ranges {
		if n := len(merged); n > 0 && int(r.lo) <= int(merged[n-1].hi)+1 {
			if r.hi > merged[n-1].hi {
				merged[n-1].hi = r.hi
			}
			continue
		}
		merged = append(merged, r)
	}
	return merged
}

// nonASCIIidentifierStart and nonASCIIidentifier are identifier.js's RegExps of the same names:
// new RegExp("[" + nonASCIIidentifierStartChars + "]") and
// new RegExp("[" + nonASCIIidentifierStartChars + nonASCIIidentifierChars + "]").
var (
	nonASCIIidentifierStart = classRanges(nonASCIIidentifierStartChars)
	nonASCIIidentifier      = classRanges(nonASCIIidentifierStartChars + nonASCIIidentifierChars)
)

// test is RegExp.prototype.test of a character class on a one-unit string.
func test(class []unitRange, u uint16) bool {
	i := sort.Search(len(class), func(i int) bool { return class[i].hi >= u })
	return i < len(class) && class[i].lo <= u
}

// fromCharCode is the code unit String.fromCharCode makes of a number in [0xaa, 0xffff]: ToUint16, which truncates a
// fraction.
func fromCharCode(code float64) uint16 { return uint16(int64(code)) }

// isInAstralSet reports whether code is in a set coded as alternating skips and run lengths from 0x10000.
func isInAstralSet(code float64, set []float64) bool {
	pos := float64(0x10000)
	for i := 0; i < len(set); i += 2 {
		pos += set[i]
		if pos > code {
			return false
		}
		pos += set[i+1]
		if pos >= code {
			return true
		}
	}
	return false
}

// IsIdentifierStart is isIdentifierStart(code): whether the code point can start an identifier.
func IsIdentifierStart(code float64) bool {
	if code < 65 {
		return code == 36
	}
	if code <= 90 {
		return true
	}
	if code < 97 {
		return code == 95
	}
	if code <= 122 {
		return true
	}
	if code <= 0xffff {
		return code >= 0xaa && test(nonASCIIidentifierStart, fromCharCode(code))
	}
	return isInAstralSet(code, astralIdentifierStartCodes)
}

// IsIdentifierChar is isIdentifierChar(code): whether the code point can continue an identifier.
func IsIdentifierChar(code float64) bool {
	if code < 48 {
		return code == 36
	}
	if code < 58 {
		return true
	}
	if code < 65 {
		return false
	}
	if code <= 90 {
		return true
	}
	if code < 97 {
		return code == 95
	}
	if code <= 122 {
		return true
	}
	if code <= 0xffff {
		return code >= 0xaa && test(nonASCIIidentifier, fromCharCode(code))
	}
	return isInAstralSet(code, astralIdentifierStartCodes) || isInAstralSet(code, astralIdentifierCodes)
}

// IsIdentifierName is isIdentifierName(name): whether name is a non-empty identifier name. It reads name's UTF-16
// code units, joining surrogate pairs.
func IsIdentifierName(name string) bool {
	units := utf16.Encode([]rune(name))
	isFirst := true
	for i := 0; i < len(units); i++ {
		cp := int(units[i])
		if cp&0xfc00 == 0xd800 && i+1 < len(units) {
			i++
			trail := int(units[i])
			if trail&0xfc00 == 0xdc00 {
				cp = 0x10000 + ((cp & 0x3ff) << 10) + (trail & 0x3ff)
			}
		}
		if isFirst {
			isFirst = false
			if !IsIdentifierStart(float64(cp)) {
				return false
			}
		} else if !IsIdentifierChar(float64(cp)) {
			return false
		}
	}
	return !isFirst
}
