package gracefulfs

import (
	"slices"
	"unicode/utf16"

	"github.com/gav4you/npmgo/internal/jsnum"
	"github.com/gav4you/npmgo/internal/jsvalue"
)

// sortFiles is readdir's files.sort(): JavaScript's default sort, in place.
// It orders elements by their string forms compared as UTF-16 code units (so
// U+1F600 sorts before U+E000, unlike Go's byte order) and keeps equal
// elements in their order. The string form of a string is itself and that of
// a []byte (a Node Buffer) is its UTF-8 decoding; nil (null), booleans and
// numbers have JavaScript's; anything else is an object whose string form is
// "[object Object]", as for the Dirent objects of withFileTypes, so those are
// never reordered. jsvalue.Undefined{} elements go to the end uncompared.
func sortFiles(files []any) {
	type keyed struct {
		v   any
		key []uint16
	}
	var defined, undefined []keyed
	for _, f := range files {
		if _, ok := f.(jsvalue.Undefined); ok {
			undefined = append(undefined, keyed{v: f})
			continue
		}
		defined = append(defined, keyed{v: f, key: sortKey(f)})
	}
	slices.SortStableFunc(defined, func(a, b keyed) int { return slices.Compare(a.key, b.key) })
	i := 0
	for _, k := range defined {
		files[i] = k.v
		i++
	}
	for _, k := range undefined {
		files[i] = k.v
		i++
	}
}

var objectObject = utf16.Encode([]rune("[object Object]"))

// sortKey is the UTF-16 form of String(v) for a readdir element.
func sortKey(v any) []uint16 {
	switch x := v.(type) {
	case nil:
		return utf16.Encode([]rune("null"))
	case string:
		return decodeUTF8([]byte(x))
	case []byte:
		return decodeUTF8(x)
	case bool:
		if x {
			return utf16.Encode([]rune("true"))
		}
		return utf16.Encode([]rune("false"))
	}
	if jsvalue.IsNumber(v) {
		return utf16.Encode([]rune(jsnum.String(jsvalue.Float(v))))
	}
	return objectObject
}

// decodeUTF8 decodes b to UTF-16 code units as Node's Buffer.toString('utf8')
// does, by the WHATWG Encoding Standard's UTF-8 decoder: each maximal invalid
// subsequence becomes one U+FFFD (Go's decoder replaces every invalid byte).
// A Go string read from a file system may hold such bytes; Node would have
// decoded the name this way.
func decodeUTF8(b []byte) []uint16 {
	out := make([]uint16, 0, len(b))
	var cp rune
	needed, seen := 0, 0
	lower, upper := byte(0x80), byte(0xBF)
	for i := 0; i < len(b); i++ {
		c := b[i]
		if needed == 0 {
			switch {
			case c <= 0x7F:
				out = append(out, uint16(c))
			case c >= 0xC2 && c <= 0xDF:
				needed, cp = 1, rune(c&0x1F)
			case c >= 0xE0 && c <= 0xEF:
				if c == 0xE0 {
					lower = 0xA0
				}
				if c == 0xED {
					upper = 0x9F
				}
				needed, cp = 2, rune(c&0xF)
			case c >= 0xF0 && c <= 0xF4:
				if c == 0xF0 {
					lower = 0x90
				}
				if c == 0xF4 {
					upper = 0x8F
				}
				needed, cp = 3, rune(c&0x7)
			default:
				out = append(out, 0xFFFD)
			}
			continue
		}
		if c < lower || c > upper {
			cp, needed, seen = 0, 0, 0
			lower, upper = 0x80, 0xBF
			out = append(out, 0xFFFD)
			i-- // prepend the byte to the stream: decode it again
			continue
		}
		lower, upper = 0x80, 0xBF
		cp = cp<<6 | rune(c&0x3F)
		seen++
		if seen != needed {
			continue
		}
		out = utf16.AppendRune(out, cp)
		cp, needed, seen = 0, 0, 0
	}
	if needed != 0 {
		out = append(out, 0xFFFD)
	}
	return out
}
