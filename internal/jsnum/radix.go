package jsnum

import "math"

// StringRadix matches JavaScript Number.prototype.toString(radix) for a radix
// from 2 to 36 (it panics for any other radix, where JavaScript throws a
// RangeError). Radix 10 is String; NaN and the infinities print as in
// String; any other number is V8's DoubleToRadixCString
// (src/numbers/conversions.cc): the integer digits, then fractional digits
// only up to the precision of the double, rounded to even.
func StringRadix(x float64, radix int) string {
	if radix < 2 || radix > 36 {
		panic("jsnum: toString() radix must be between 2 and 36")
	}
	switch {
	case radix == 10:
		return String(x)
	case math.IsNaN(x):
		return "NaN"
	case math.IsInf(x, 1):
		return "Infinity"
	case math.IsInf(x, -1):
		return "-Infinity"
	}
	return doubleToRadix(x, radix)
}

const radixChars = "0123456789abcdefghijklmnopqrstuvwxyz"

// doubleToRadix is V8's DoubleToRadixCString for a finite x. Products are
// wrapped in float64() so that no multiply is fused with a later addition
// (V8 rounds every operation).
func doubleToRadix(value float64, radix int) string {
	// V8 writes the integer digits leftwards and the fraction rightwards from
	// the middle of a 2200-character buffer.
	const bufferSize = 2200
	var buffer [bufferSize]byte
	integerCursor := bufferSize / 2
	fractionCursor := integerCursor
	r := float64(radix)

	negative := value < 0
	if negative {
		value = -value
	}

	// Split the value into an integer part and a fractional part.
	integer := math.Floor(value)
	fraction := value - integer
	// Fractional digits are computed only up to the input's precision.
	delta := 0.5 * (math.Nextafter(value, math.Inf(1)) - value)
	delta = math.Max(math.Nextafter(0, 1), delta)
	if fraction >= delta {
		buffer[fractionCursor] = '.'
		fractionCursor++
		for {
			// Shift up by one digit.
			fraction = float64(fraction * r)
			delta = float64(delta * r)
			digit := int(fraction)
			buffer[fractionCursor] = radixChars[digit]
			fractionCursor++
			// Calculate the remainder.
			fraction -= float64(digit)
			// Round to even.
			if fraction > 0.5 || (fraction == 0.5 && digit&1 != 0) {
				if fraction+delta > 1 {
					// Propagate the carry through the digits already written.
					for {
						fractionCursor--
						if fractionCursor == bufferSize/2 {
							// Carry over to the integer part.
							integer++
							break
						}
						c := buffer[fractionCursor]
						d := int(c - '0')
						if c > '9' {
							d = int(c-'a') + 10
						}
						if d+1 < radix {
							buffer[fractionCursor] = radixChars[d+1]
							fractionCursor++
							break
						}
					}
					break
				}
			}
			if !(fraction >= delta) {
				break
			}
		}
	}

	// Compute the integer digits; digits below the double's precision are zeros.
	for exponent(integer/r) > 0 {
		integer /= r
		integerCursor--
		buffer[integerCursor] = '0'
	}
	for {
		remainder := math.Mod(integer, r)
		integerCursor--
		buffer[integerCursor] = radixChars[int(remainder)]
		integer = (integer - remainder) / r
		if !(integer > 0) {
			break
		}
	}

	if negative {
		integerCursor--
		buffer[integerCursor] = '-'
	}
	return string(buffer[integerCursor:fractionCursor])
}

// exponent is V8's Double::Exponent: the binary exponent of the significand
// read as a 53-bit integer, so exponent(x) > 0 exactly when x >= 2^53.
func exponent(x float64) int {
	bits := math.Float64bits(x)
	biased := int(bits>>52) & 0x7FF
	if biased == 0 { // zero or denormal
		return -1074
	}
	return biased - 1075
}
