// Package jsnum reproduces JavaScript number semantics needed for parity.
package jsnum

import (
	"fmt"
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// Round matches JavaScript Math.round: the closest integer, halves rounding
// toward +Infinity, and -0 for -0.5 <= x < 0. (floor(x + 0.5) is wrong for
// 0.49999999999999994 and for odd integers >= 2^52, where x + 0.5 rounds.)
func Round(x float64) float64 {
	if math.IsNaN(x) || math.IsInf(x, 0) || x == math.Trunc(x) {
		return x
	}
	f := math.Floor(x)
	if x-f >= 0.5 { // exact: x - floor(x) is representable
		f++
	}
	if f == 0 && x < 0 {
		return math.Copysign(0, -1)
	}
	return f
}

// String matches JavaScript Number.prototype.toString().
func String(x float64) string {
	switch {
	case math.IsNaN(x):
		return "NaN"
	case math.IsInf(x, 1):
		return "Infinity"
	case math.IsInf(x, -1):
		return "-Infinity"
	case x == 0:
		return "0"
	}
	abs := math.Abs(x)
	if abs >= 1e21 || abs < 1e-6 {
		s := strconv.FormatFloat(x, 'e', -1, 64)
		mant, exp, _ := strings.Cut(s, "e")
		if exp[0] == '+' {
			exp = exp[1:]
			exp = strings.TrimLeft(exp, "0")
			return mant + "e+" + exp
		}
		exp = strings.TrimLeft(exp[1:], "0")
		return mant + "e-" + exp
	}
	return strconv.FormatFloat(x, 'f', -1, 64)
}

// IsWhiteSpace reports whether r is a JavaScript StrWhiteSpaceChar: WhiteSpace
// (TAB, VT, FF, SP, NBSP, ZWNBSP and any Zs) or a LineTerminator (LF, CR, LS,
// PS). It is also the set matched by \s in a JavaScript RegExp. It differs
// from unicode.IsSpace: U+0085 (NEL) is not JavaScript whitespace and U+FEFF is.
func IsWhiteSpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ', 0xA0, 0xFEFF, 0x2028, 0x2029:
		return true
	}
	return r > 0x7f && unicode.Is(unicode.Zs, r)
}

// TrimSpace matches String.prototype.trim().
func TrimSpace(s string) string { return strings.TrimFunc(s, IsWhiteSpace) }

// strDecimalRe is StrDecimalLiteral without the Infinity forms.
var strDecimalRe = regexp.MustCompile(`^[+-]?(\d+\.?\d*|\.\d+)([eE][+-]?\d+)?$`)

// Number matches JavaScript Number(string) (StringToNumber): whitespace is
// trimmed, decimal literals with optional sign and exponent, Infinity, and
// unsigned 0x / 0o / 0b integers of any length; anything else is NaN.
func Number(s string) float64 {
	t := TrimSpace(s)
	if t == "" {
		return 0
	}
	switch t {
	case "Infinity", "+Infinity":
		return math.Inf(1)
	case "-Infinity":
		return math.Inf(-1)
	}
	if len(t) > 2 && t[0] == '0' {
		base := 0
		switch t[1] {
		case 'x', 'X':
			base = 16
		case 'o', 'O':
			base = 8
		case 'b', 'B':
			base = 2
		}
		if base != 0 {
			return radixInt(t[2:], base)
		}
	}
	if !strDecimalRe.MatchString(t) {
		return math.NaN()
	}
	n, err := strconv.ParseFloat(t, 64)
	if err != nil {
		if ne, ok := err.(*strconv.NumError); ok && ne.Err == strconv.ErrRange {
			return n // +-Inf on overflow, 0 on underflow, as in JS
		}
		return math.NaN()
	}
	return n
}

// radixInt converts a string of base-radix digits (no sign, no separators) to
// the nearest double, NaN if digits is empty or has other characters.
func radixInt(digits string, base int) float64 {
	if digits == "" {
		return math.NaN()
	}
	for i := 0; i < len(digits); i++ {
		if digitValue(digits[i]) >= base {
			return math.NaN()
		}
	}
	n, ok := new(big.Int).SetString(digits, base)
	if !ok {
		return math.NaN()
	}
	f, _ := new(big.Float).SetInt(n).Float64() // round to nearest even
	return f
}

func digitValue(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'z':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'Z':
		return int(c-'A') + 10
	}
	return 99
}

// ToInt32 matches the ECMAScript ToInt32 conversion that the bitwise
// operators (|, &, ^, ~, <<, >>) apply to their operands: the number
// truncated toward zero and wrapped modulo 2^32 into the int32 range. NaN and
// the infinities give 0.
func ToInt32(x float64) int32 {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return 0
	}
	m := math.Mod(math.Trunc(x), 1<<32) // exact
	if m < 0 {
		m += 1 << 32
	}
	return int32(uint32(m))
}

// Pow matches the JavaScript ** operator and Math.pow. It is math.Pow except
// where ECMAScript differs from IEEE 754: a NaN exponent always gives NaN
// (1 ** NaN) and so does (+-1) ** +-Infinity.
//
// V8 computes ** with the C library's pow, so Node's own results differ in
// the last bit between platforms (and, with glibc, between CPUs with and
// without FMA); math.Pow agrees with Node on the value but not always on
// that bit.
func Pow(x, y float64) float64 {
	if math.IsNaN(y) || (math.IsInf(y, 0) && (x == 1 || x == -1)) {
		return math.NaN()
	}
	return math.Pow(x, y)
}

// ISOString matches Date.prototype.toISOString for a finite millisecond time.
func ISOString(ms float64) string {
	t := time.UnixMilli(int64(ms)).UTC()
	y := t.Year()
	switch {
	case y < 0:
		return "-" + fmt.Sprintf("%06d", -y) + t.Format("-01-02T15:04:05.000Z")
	case y > 9999:
		return "+" + fmt.Sprintf("%06d", y) + t.Format("-01-02T15:04:05.000Z")
	}
	return t.Format("2006-01-02T15:04:05.000Z")
}

var parseFloatRe = regexp.MustCompile(`^[+-]?(Infinity|(\d+\.?\d*|\.\d+)([eE][+-]?\d+)?)`)

// ParseInt matches the global parseInt(s) with no radix: leading whitespace,
// an optional sign, a 0x prefix selecting base 16, then the longest run of
// digits (NaN when there is none, e.g. "0x" or "0xg").
func ParseInt(s string) float64 { return ParseIntRadix(s, 0) }

// ParseIntRadix matches the global parseInt(s, radix). A radix of 0 behaves
// like no radix (base 10, or 16 after a 0x prefix); 16 also skips a 0x
// prefix; any other radix outside 2..36 gives NaN. Every other radix reads
// digits only, so parseInt("0x3", 10) is 0. Values in a radix that is not a
// power of two are rounded to nearest, which V8 may not do for numbers
// beyond 2^53 in such a radix.
func ParseIntRadix(s string, radix int) float64 {
	t := strings.TrimLeftFunc(s, IsWhiteSpace)
	sign := 1.0
	if t != "" && (t[0] == '-' || t[0] == '+') {
		if t[0] == '-' {
			sign = -1
		}
		t = t[1:]
	}
	stripPrefix := true
	switch {
	case radix == 0:
		radix = 10
	case radix < 2 || radix > 36:
		return math.NaN()
	case radix != 16:
		stripPrefix = false
	}
	if stripPrefix && len(t) >= 2 && t[0] == '0' && (t[1] == 'x' || t[1] == 'X') {
		radix = 16
		t = t[2:]
	}
	i := 0
	for i < len(t) && digitValue(t[i]) < radix {
		i++
	}
	if i == 0 {
		return math.NaN()
	}
	var f float64
	if radix == 10 {
		f, _ = strconv.ParseFloat(t[:i], 64)
	} else {
		f = radixInt(t[:i], radix)
	}
	return sign * f // sign * 0 keeps -0 for "-0"
}

// ParseFloat matches the global parseFloat(s).
func ParseFloat(s string) float64 {
	t := strings.TrimLeftFunc(s, IsWhiteSpace)
	m := parseFloatRe.FindString(t)
	if m == "" {
		return math.NaN()
	}
	switch strings.TrimLeft(m, "+") {
	case "Infinity":
		return math.Inf(1)
	case "-Infinity":
		return math.Inf(-1)
	}
	f, _ := strconv.ParseFloat(m, 64)
	return f
}
