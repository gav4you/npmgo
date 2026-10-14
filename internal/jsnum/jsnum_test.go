package jsnum

import (
	"math"
	"testing"
)

// Expected values were produced by V8 (node v25.9.0): Math.round, Number(),
// parseInt(), parseFloat() and String() on the same inputs.

var (
	bom  = string(rune(0xFEFF)) // ZWNBSP: JS whitespace
	nel  = string(rune(0x0085)) // NEL: not JS whitespace (but Go's unicode.IsSpace says yes)
	nbsp = string(rune(0x00A0))
	ls   = string(rune(0x2028))
	ps   = string(rune(0x2029))
	mvs  = string(rune(0x180E)) // Mongolian vowel separator: Cf since Unicode 6.3
)

func same(a, b float64) bool {
	if math.IsNaN(a) || math.IsNaN(b) {
		return math.IsNaN(a) && math.IsNaN(b)
	}
	return a == b && math.Signbit(a) == math.Signbit(b)
}

func TestRound(t *testing.T) {
	negZero := math.Copysign(0, -1)
	for _, c := range []struct{ in, want float64 }{
		{0.49999999999999994, 0}, // floor(x + 0.5) would give 1
		{4503599627370497, 4503599627370497},
		{-4503599627370497, -4503599627370497},
		{-0.5, negZero}, {-0.4, negZero}, {-0.6, -1},
		{2.5, 3}, {-2.5, -2}, {1e300, 1e300}, {negZero, negZero},
		{math.NaN(), math.NaN()}, {math.Inf(1), math.Inf(1)}, {math.Inf(-1), math.Inf(-1)},
		{0.5, 1}, {1.4999999999999998, 1}, {-1e-17, negZero},
	} {
		if got := Round(c.in); !same(got, c.want) {
			t.Errorf("Round(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestNumber(t *testing.T) {
	nan := math.NaN()
	for _, c := range []struct {
		in   string
		want float64
	}{
		{"0x_1f", nan}, {nel + "5", nan}, {bom + "5", 5}, {"0xg", nan}, {"0x", nan},
		{"1_0", nan}, {"0b1_1", nan}, {"00x1", nan}, {"0X1F", 31}, {"0b11", 3}, {"0o17", 15},
		{"-0x10", nan}, {"0xFFFFFFFFFFFFFFFFFFFF", 1.2089258196146292e+24},
		{"-0", math.Copysign(0, -1)}, {" \n5\t", 5}, {mvs + "5", nan}, {"5" + nel, nan},
		{nbsp + "5 ", 5}, {ls + "5" + ps, 5}, {"Infinity", math.Inf(1)}, {"-Infinity", math.Inf(-1)},
		{"+Infinity", math.Inf(1)}, {"infinity", nan}, {"1e", nan}, {"1e+", nan}, {".", nan},
		{"+", nan}, {"-", nan}, {"", 0}, {" ", 0}, {"0o", nan}, {"0b", nan}, {"0x1F.5", nan},
		{"1.5e-7", 1.5e-7}, {"1e21", 1e21}, {"1e400", math.Inf(1)}, {".5", 0.5}, {"5.", 5},
		{"+5", 5}, {"1 2", nan}, {"NaN", nan}, {"inf", nan},
	} {
		if got := Number(c.in); !same(got, c.want) {
			t.Errorf("Number(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestParseInt(t *testing.T) {
	nan := math.NaN()
	for _, c := range []struct {
		in   string
		want float64
	}{
		{"0xg", nan}, {"0x", nan}, {"-0x1f", -31}, {"+0x1f", 31}, {"0x1F", 31}, {"  12abc", 12},
		{nel + "1", nan}, {bom + "1", 1}, {"1e3", 1}, {"-", nan}, {"+", nan}, {"", nan},
		{"12345678901234567890123", 1.2345678901234568e+22},
		{"0xFFFFFFFFFFFFFFFFFFFFFF", 3.094850098213451e+26}, {"1_0", 1}, {"0b11", 0},
		{"-0", math.Copysign(0, -1)}, {ls + "3", 3},
	} {
		if got := ParseInt(c.in); !same(got, c.want) {
			t.Errorf("ParseInt(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestParseFloat(t *testing.T) {
	nan := math.NaN()
	for _, c := range []struct {
		in   string
		want float64
	}{
		{nel + "1.5", nan}, {bom + "1.5", 1.5}, {"1.e5", 100000}, {".5e", 0.5}, {"-.5", -0.5},
		{"+.5", 0.5}, {"Infinityx", math.Inf(1)}, {"-Infinity", math.Inf(-1)}, {"infinity", nan},
		{"1e", 1}, {"1e+5", 100000}, {"1E5", 100000}, {"0x10", 0}, {"1_0", 1}, {".", nan}, {"-", nan},
	} {
		if got := ParseFloat(c.in); !same(got, c.want) {
			t.Errorf("ParseFloat(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestString(t *testing.T) {
	for _, c := range []struct {
		in   float64
		want string
	}{
		{math.Copysign(0, -1), "0"}, {-1e-7, "-1e-7"}, {1e21, "1e+21"},
		{123456789012345680000, "123456789012345680000"}, {0.000001, "0.000001"}, {1e-7, "1e-7"},
		{5e-324, "5e-324"}, {1.7976931348623157e308, "1.7976931348623157e+308"},
		{0.30000000000000004, "0.30000000000000004"}, {100, "100"}, {1234567890123456789, "1234567890123456800"},
	} {
		if got := String(c.in); got != c.want {
			t.Errorf("String(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestWhiteSpace(t *testing.T) {
	for _, r := range []rune{'\t', '\n', '\v', '\f', '\r', ' ', 0xA0, 0x1680, 0x2000, 0x200A, 0x2028, 0x2029, 0x202F, 0x205F, 0x3000, 0xFEFF} {
		if !IsWhiteSpace(r) {
			t.Errorf("IsWhiteSpace(%#x) = false", r)
		}
	}
	for _, r := range []rune{0x85, 0x180E, 0x200B, 'a', 0x3007} {
		if IsWhiteSpace(r) {
			t.Errorf("IsWhiteSpace(%#x) = true", r)
		}
	}
	if got := TrimSpace(bom + " a " + nel); got != "a "+nel {
		t.Errorf("TrimSpace = %q", got)
	}
}
