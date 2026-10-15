// Copyright (c) 2014-present Sebastian McKenzie and other contributors. MIT License (see LICENSE).
// Written for the Go translation of @babel/types 7.29.0 by the npmgo contributors.

package types

import (
	"strconv"
	"unicode/utf16"
)

func itoa(i int) string { return strconv.Itoa(i) }

// JavaScript strings are sequences of UTF-16 code units; these helpers read Go strings that way.

// utf16Units returns s as UTF-16 code units (a byte that is not valid UTF-8 is U+FFFD).
func utf16Units(s string) []uint16 { return utf16.Encode([]rune(s)) }

// unitsString assembles code units into a string; a lone surrogate becomes U+FFFD.
func unitsString(u []uint16) string { return string(utf16.Decode(u)) }

// unitString is the one-unit string s[i] of JavaScript.
func unitString(u uint16) string { return unitsString([]uint16{u}) }

// containsUnits is String.prototype.includes on code units.
func containsUnits(hay, needle []uint16) bool {
	return indexUnits(hay, needle, 0) >= 0
}

// indexUnits is String.prototype.indexOf on code units.
func indexUnits(hay, needle []uint16, from int) int {
	if from < 0 {
		from = 0
	}
	for i := from; i+len(needle) <= len(hay); i++ {
		match := true
		for j := range needle {
			if hay[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}
