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

// Round matches JavaScript Math.round (halves round toward +Infinity).
func Round(x float64) float64 {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return x
	}
	r := math.Floor(x + 0.5)
	if r == 0 && math.Signbit(x) {
		return math.Copysign(0, -1)
	}
	return r
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

// Number matches JavaScript Number(string) for decimal/hex/Infinity input.
func Number(s string) float64 {
	t := strings.TrimSpace(s)
	if t == "" {
		return 0
	}
	switch t {
	case "Infinity", "+Infinity":
		return math.Inf(1)
	case "-Infinity":
		return math.Inf(-1)
	}
	if len(t) > 2 && t[0] == '0' && (t[1] == 'x' || t[1] == 'X' || t[1] == 'o' || t[1] == 'O' || t[1] == 'b' || t[1] == 'B') {
		if n, err := strconv.ParseInt(t, 0, 64); err == nil {
			return float64(n)
		}
		return math.NaN()
	}
	if strings.ContainsAny(t, "_xXpPnNiI") {
		return math.NaN()
	}
	n, err := strconv.ParseFloat(t, 64)
	if err != nil {
		if ne, ok := err.(*strconv.NumError); ok && ne.Err == strconv.ErrRange {
			return n
		}
		return math.NaN()
	}
	return n
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

var (
	parseIntRe   = regexp.MustCompile(`^[+-]?(0[xX][0-9a-fA-F]+|[0-9]+)`)
	parseFloatRe = regexp.MustCompile(`^[+-]?(Infinity|(\d+\.?\d*|\.\d+)([eE][+-]?\d+)?)`)
)

// ParseInt matches the global parseInt(s) with no radix.
func ParseInt(s string) float64 {
	t := strings.TrimLeftFunc(s, isJSSpace)
	m := parseIntRe.FindString(t)
	if m == "" {
		return math.NaN()
	}
	neg := strings.HasPrefix(m, "-")
	m = strings.TrimLeft(m, "+-")
	var f float64
	if len(m) > 1 && (m[1] == 'x' || m[1] == 'X') {
		n := new(big.Int)
		n.SetString(m[2:], 16)
		f, _ = new(big.Float).SetInt(n).Float64()
	} else {
		f, _ = strconv.ParseFloat(m, 64)
	}
	if neg {
		f = -f
	}
	return f
}

// ParseFloat matches the global parseFloat(s).
func ParseFloat(s string) float64 {
	t := strings.TrimLeftFunc(s, isJSSpace)
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

func isJSSpace(r rune) bool { return unicode.IsSpace(r) || r == 0xFEFF }
