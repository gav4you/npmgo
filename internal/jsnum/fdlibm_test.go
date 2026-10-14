package jsnum

import (
	"math"
	"math/big"
	"testing"
)

// Expected bits were produced by V8 (node v25.9.0). The fdlibm port also
// matched V8 on a sweep of 193,468 sin / cos / atan2 inputs across the whole
// double range (random magnitudes and bit patterns, multiples of pi/2,
// specials).

func sameBits(got, want float64) bool {
	if math.IsNaN(want) {
		return math.IsNaN(got) // NaN sign bits differ between CPUs
	}
	return math.Float64bits(got) == math.Float64bits(want)
}

func TestSinCos(t *testing.T) {
	for _, c := range [][3]uint64{
		{0x0000000000000000, 0x0000000000000000, 0x3ff0000000000000},
		{0x8000000000000000, 0x8000000000000000, 0x3ff0000000000000},
		{0x3ff0000000000000, 0x3feaed548f090cee, 0x3fe14a280fb5068c},
		{0xbff0000000000000, 0xbfeaed548f090cee, 0x3fe14a280fb5068c},
		{0x3fe0000000000000, 0x3fdeaee8744b05f0, 0x3fec1528065b7d50},
		{0x4000000000000000, 0x3fed18f6ead1b446, 0xbfdaa22657537205},
		{0x4008000000000000, 0x3fc210386db6d55b, 0xbfefae04be85e5d2},
		{0x400921fb54442d18, 0x3ca1a62633145c07, 0xbff0000000000000}, // pi
		{0x3ff921fb54442d18, 0x3ff0000000000000, 0x3c91a62633145c07}, // pi/2
		{0x3fe921fb54442d18, 0x3fe6a09e667f3bcc, 0x3fe6a09e667f3bcd}, // pi/4
		{0x3fe921fb54442d19, 0x3fe6a09e667f3bcd, 0x3fe6a09e667f3bcc},
		{0x4002d97c7f3321d2, 0x3fe6a09e667f3bcd, 0xbfe6a09e667f3bcc}, // 3pi/4
		{0x4024000000000000, 0xbfe1689ef5f34f52, 0xbfead9ac890c6b1f},
		{0x4059000000000000, 0xbfe03425b78c4db8, 0x3feb981dbf665fdf},
		{0x40f86a0000000000, 0x3fa24daa9c527e96, 0xbfeffac3841b3da7},
		{0x412921fc00000000, 0x3fd51102efad6266, 0x3fee377c09c4cce7}, // 823550: medium reduction
		{0x412921fe00000000, 0x3fef1e1d322aef27, 0x3fcdd9e00a413200}, // 823551: Payne-Hanek
		{0x412e848000000000, 0xbfd6664b2568d867, 0x3fedf9df9906d32c},
		{0x41cdcd6500000000, 0x3fe1778cae83c69b, 0x3feacff8c7364234},
		{0x430c6bf526340000, 0x3feb76f88136ceba, 0xbfe06c154609d33e},
		{0x4480f0cf064dd592, 0xbfeb453ab76bf397, 0x3fe0be2cef01c8f4}, // 1e22
		{0x54b249ad2594c37d, 0xbfd85c5e5b929359, 0x3fed9757496841f5},
		{0x7e37e43c8800759c, 0xbfea2c16b010e385, 0xbfe2699022adc4c1},
		{0x7fefffffffffffff, 0x3f7452fc98b34e97, 0xbfefffe62ecfab75}, // max double
		{0xc480f0cf064dd592, 0x3feb453ab76bf397, 0x3fe0be2cef01c8f4},
		{0x0000000000000001, 0x0000000000000001, 0x3ff0000000000000},
		{0x3ddb7cdfd9d7bdbb, 0x3ddb7cdfd9d7bdbb, 0x3ff0000000000000},
		{0x7506ac5b262ca1ff, 0x3ff0000000000000, 0xbc214ae72e6ba22f}, // hard case for reduction
		{0x40478fdb9effea46, 0x3cf845e1e7f18b23, 0xbff0000000000000},
		{0xc00f6a7a2955385e, 0x3fe6a09e667f3bcc, 0xbfe6a09e667f3bce},
		{0x7ff0000000000000, 0x7ff8000000000000, 0x7ff8000000000000}, // Infinity: NaN
		{0x7ff8000000000000, 0x7ff8000000000000, 0x7ff8000000000000},
	} {
		x := math.Float64frombits(c[0])
		if got, want := Sin(x), math.Float64frombits(c[1]); !sameBits(got, want) {
			t.Errorf("Sin(%v) = %v, want %v", x, got, want)
		}
		if got, want := Cos(x), math.Float64frombits(c[2]); !sameBits(got, want) {
			t.Errorf("Cos(%v) = %v, want %v", x, got, want)
		}
	}
}

func TestAtan2(t *testing.T) {
	for _, c := range [][3]uint64{
		{0x0000000000000000, 0x0000000000000000, 0x0000000000000000},
		{0x8000000000000000, 0x0000000000000000, 0x8000000000000000},
		{0x0000000000000000, 0x8000000000000000, 0x400921fb54442d18},
		{0x8000000000000000, 0x8000000000000000, 0xc00921fb54442d18},
		{0x3ff0000000000000, 0x0000000000000000, 0x3ff921fb54442d18},
		{0xbff0000000000000, 0x0000000000000000, 0xbff921fb54442d18},
		{0x0000000000000000, 0xbff0000000000000, 0x400921fb54442d18},
		{0x8000000000000000, 0xbff0000000000000, 0xc00921fb54442d18},
		{0x3ff0000000000000, 0x3ff0000000000000, 0x3fe921fb54442d18},
		{0x3ff0000000000000, 0xbff0000000000000, 0x4002d97c7f3321d2},
		{0xbff0000000000000, 0xbff0000000000000, 0xc002d97c7f3321d2},
		{0xbff0000000000000, 0x3ff0000000000000, 0xbfe921fb54442d18},
		{0x7ff0000000000000, 0x7ff0000000000000, 0x3fe921fb54442d18},
		{0xfff0000000000000, 0x7ff0000000000000, 0xbfe921fb54442d18},
		{0x7ff0000000000000, 0xfff0000000000000, 0x4002d97c7f3321d2},
		{0xfff0000000000000, 0xfff0000000000000, 0xc002d97c7f3321d2},
		{0x3ff0000000000000, 0x7ff0000000000000, 0x0000000000000000},
		{0xbff0000000000000, 0x7ff0000000000000, 0x8000000000000000},
		{0x3ff0000000000000, 0xfff0000000000000, 0x400921fb54442d18},
		{0xbff0000000000000, 0xfff0000000000000, 0xc00921fb54442d18},
		{0x7ff0000000000000, 0x3ff0000000000000, 0x3ff921fb54442d18},
		{0xfff0000000000000, 0x3ff0000000000000, 0xbff921fb54442d18},
		{0x7ff8000000000000, 0x3ff0000000000000, 0x7ff8000000000000},
		{0x3ff0000000000000, 0x7ff8000000000000, 0x7ff8000000000000},
		{0x4008000000000000, 0x4010000000000000, 0x3fe4978fa3269ee1},
		{0xc008000000000000, 0x4010000000000000, 0xbfe4978fa3269ee1},
		{0x7e37e43c8800759c, 0x01a56e1fc2f8f359, 0x3ff921fb54442d18},
		{0x01a56e1fc2f8f359, 0xfe37e43c8800759c, 0x400921fb54442d18},
		{0x43c0000000000000, 0x3ff0000000000000, 0x3ff921fb54442d18},
		{0x3ff0000000000000, 0xc3c0000000000000, 0x400921fb54442d18},
		{0x3fe0000000000000, 0x3fd0000000000000, 0x3ff1b6e192ebbe44},
		{0x402899999999999a, 0xc046cccccccccccd, 0x40070666fbb97e94},
		{0xc05059999999999a, 0xc0400ccccccccccd, 0xc00037747424f4ae},
		{0x4014000000000000, 0x3ff0000000000000, 0x3ff5f97315254857},
	} {
		y, x := math.Float64frombits(c[0]), math.Float64frombits(c[1])
		if got, want := Atan2(y, x), math.Float64frombits(c[2]); !sameBits(got, want) {
			t.Errorf("Atan2(%v, %v) = %v, want %v", y, x, got, want)
		}
	}
}

// bigPi computes pi to prec bits with Machin's formula,
// pi = 16 atan(1/5) - 4 atan(1/239).
func bigPi(prec uint) *big.Float {
	newF := func() *big.Float { return new(big.Float).SetPrec(prec) }
	atanInv := func(n int64) *big.Float {
		x := newF().Quo(newF().SetInt64(1), newF().SetInt64(n))
		x2 := newF().Mul(x, x)
		sum, term := newF().Set(x), newF().Set(x)
		eps := newF().SetMantExp(newF().SetInt64(1), -int(prec))
		for k := int64(1); ; k++ {
			term.Mul(term, x2)
			t := newF().Quo(term, newF().SetInt64(2*k+1))
			if t.Cmp(eps) < 0 {
				return sum
			}
			if k%2 == 1 {
				sum.Sub(sum, t)
			} else {
				sum.Add(sum, t)
			}
		}
	}
	a := newF().Mul(newF().SetInt64(16), atanInv(5))
	b := newF().Mul(newF().SetInt64(4), atanInv(239))
	return a.Sub(a, b)
}

// TestReductionTables checks fdlibm's 2/pi and pi/2 tables against pi
// computed with math/big.
func TestReductionTables(t *testing.T) {
	const prec = 1800
	pi := bigPi(prec)
	v := new(big.Float).SetPrec(prec).Quo(new(big.Float).SetPrec(prec).SetInt64(2), pi)
	two24 := new(big.Float).SetPrec(prec).SetInt64(1 << 24)
	for i, want := range twoOverPi {
		v.Mul(v, two24)
		d, _ := v.Int64()
		if int32(d) != want {
			t.Fatalf("twoOverPi[%d] = %#x, want %#x", i, want, d)
		}
		v.Sub(v, new(big.Float).SetPrec(prec).SetInt64(d))
	}
	half := new(big.Float).SetPrec(prec).Quo(pi, new(big.Float).SetPrec(prec).SetInt64(2))
	for i, chunk := range pio2Chunks {
		shift := 23 + 24*i
		scaled := new(big.Float).SetPrec(prec).SetMantExp(half, shift)
		n, _ := scaled.Int(nil)
		n.And(n, big.NewInt(1<<24-1))
		want, _ := new(big.Float).SetMantExp(new(big.Float).SetInt(n), -shift).Float64()
		if chunk != want {
			t.Errorf("pio2Chunks[%d] = %v, want %v", i, chunk, want)
		}
	}
}
