// Math.atan2, Math.sin and Math.cos as V8 computes them: V8 implements these
// with its own port of fdlibm (src/base/ieee754.cc), so Node returns the same
// bits on every platform. This file translates the same fdlibm routines
// (e_atan2.c, s_atan.c, s_sin.c, s_cos.c, k_sin.c, k_cos.c, e_rem_pio2.c,
// k_rem_pio2.c, s_scalbn.c).
//
// Every product that feeds an addition or subtraction is wrapped in an explicit
// float64 conversion: Go may otherwise fuse it into a fused multiply-add on
// arm64, ppc64le, s390x and riscv64, which changes the last bit.
//
// ====================================================
// Copyright (C) 1993 by Sun Microsystems, Inc. All rights reserved.
//
// Developed at SunSoft, a Sun Microsystems, Inc. business.
// Permission to use, copy, modify, and distribute this
// software is freely granted, provided that this notice
// is preserved.
// ====================================================

package jsnum

import "math"

// High and low 32-bit words of a float64, as fdlibm's EXTRACT_WORDS.
func hiWord(x float64) int32  { return int32(math.Float64bits(x) >> 32) }
func loWord(x float64) uint32 { return uint32(math.Float64bits(x)) }

func fromWords(hi int32, lo uint32) float64 {
	return math.Float64frombits(uint64(uint32(hi))<<32 | uint64(lo))
}

func withHiWord(x float64, hi int32) float64 { return fromWords(hi, loWord(x)) }

// Atan2's constants are variables, not constants, so that expressions such as
// pi + tiny are evaluated in float64 like C does, not exactly by the compiler.
var (
	tiny = 1.0e-300
	piO4 = 7.8539816339744827900e-01 // 0x3FE921FB, 0x54442D18
	piO2 = 1.5707963267948965580e+00 // 0x3FF921FB, 0x54442D18
	pi   = 3.1415926535897931160e+00 // 0x400921FB, 0x54442D18
	piLo = 1.2246467991473531772e-16 // 0x3CA1A626, 0x33145C07
)

// Atan2 matches JavaScript Math.atan2(y, x) (fdlibm __ieee754_atan2).
func Atan2(y, x float64) float64 {
	hx, lx := hiWord(x), loWord(x)
	ix := hx & 0x7fffffff
	hy, ly := hiWord(y), loWord(y)
	iy := hy & 0x7fffffff
	if ix|int32((lx|-lx)>>31) > 0x7ff00000 || iy|int32((ly|-ly)>>31) > 0x7ff00000 {
		return x + y // x or y is NaN
	}
	if (hx-0x3ff00000)|int32(lx) == 0 {
		return atan(y) // x = 1.0
	}
	m := (hy>>31)&1 | (hx>>30)&2 // 2*sign(x) + sign(y)

	if iy|int32(ly) == 0 { // y = 0
		switch m {
		case 0, 1:
			return y // atan(+-0, +anything) = +-0
		case 2:
			return pi + tiny // atan(+0, -anything) = pi
		default:
			return -pi - tiny // atan(-0, -anything) = -pi
		}
	}
	if ix|int32(lx) == 0 { // x = 0
		if hy < 0 {
			return -piO2 - tiny
		}
		return piO2 + tiny
	}
	if ix == 0x7ff00000 { // x is INF
		if iy == 0x7ff00000 {
			switch m {
			case 0:
				return piO4 + tiny // atan(+INF, +INF)
			case 1:
				return -piO4 - tiny // atan(-INF, +INF)
			case 2:
				return float64(3.0*piO4) + tiny // atan(+INF, -INF)
			default:
				return float64(-3.0*piO4) - tiny // atan(-INF, -INF)
			}
		}
		switch m {
		case 0:
			return 0 // atan(+..., +INF)
		case 1:
			return math.Copysign(0, -1) // atan(-..., +INF)
		case 2:
			return pi + tiny // atan(+..., -INF)
		default:
			return -pi - tiny // atan(-..., -INF)
		}
	}
	if iy == 0x7ff00000 { // y is INF
		if hy < 0 {
			return -piO2 - tiny
		}
		return piO2 + tiny
	}

	var z float64
	k := (iy - ix) >> 20
	switch {
	case k > 60: // |y/x| > 2**60
		z = piO2 + float64(0.5*piLo)
		m &= 1
	case hx < 0 && k < -60: // 0 > |y|/x > -2**-60
		z = 0.0
	default:
		z = atan(math.Abs(y / x))
	}
	switch m {
	case 0:
		return z // atan(+, +)
	case 1:
		return -z // atan(-, +)
	case 2:
		return pi - (z - piLo) // atan(+, -)
	default:
		return (z - piLo) - pi // atan(-, -)
	}
}

var (
	atanHi = [4]float64{
		4.63647609000806093515e-01, // atan(0.5)hi 0x3FDDAC67, 0x0561BB4F
		7.85398163397448278999e-01, // atan(1.0)hi 0x3FE921FB, 0x54442D18
		9.82793723247329054082e-01, // atan(1.5)hi 0x3FEF730B, 0xD281F69B
		1.57079632679489655800e+00, // atan(inf)hi 0x3FF921FB, 0x54442D18
	}
	atanLo = [4]float64{
		2.26987774529616870924e-17, // atan(0.5)lo 0x3C7A2B7F, 0x222F65E2
		3.06161699786838301793e-17, // atan(1.0)lo 0x3C81A626, 0x33145C07
		1.39033110312309984516e-17, // atan(1.5)lo 0x3C700788, 0x7AF0CBBD
		6.12323399573676603587e-17, // atan(inf)lo 0x3C91A626, 0x33145C07
	}
	aT = [11]float64{
		3.33333333333329318027e-01,  // 0x3FD55555, 0x5555550D
		-1.99999999998764832476e-01, // 0xBFC99999, 0x9998EBC4
		1.42857142725034663711e-01,  // 0x3FC24924, 0x920083FF
		-1.11111104054623557880e-01, // 0xBFBC71C6, 0xFE231671
		9.09088713343650656196e-02,  // 0x3FB745CD, 0xC54C206E
		-7.69187620504482999495e-02, // 0xBFB3B0F2, 0xAF749A6D
		6.66107313738753120669e-02,  // 0x3FB10D66, 0xA0D03D51
		-5.83357013379057348645e-02, // 0xBFADDE2D, 0x52DEFD9A
		4.97687799461593236017e-02,  // 0x3FA97B4B, 0x24760DEB
		-3.65315727442169155270e-02, // 0xBFA2B444, 0x2C6A6C2F
		1.62858201153657823623e-02,  // 0x3F90AD3A, 0xE322DA11
	}
)

// atan is fdlibm's atan (s_atan.c).
func atan(x float64) float64 {
	const huge = 1.0e300
	hx := hiWord(x)
	ix := hx & 0x7fffffff
	if ix >= 0x44100000 { // |x| >= 2^66
		if ix > 0x7ff00000 || (ix == 0x7ff00000 && loWord(x) != 0) {
			return x + x // NaN
		}
		if hx > 0 {
			return atanHi[3] + atanLo[3]
		}
		return -atanHi[3] - atanLo[3]
	}
	id := -1
	if ix < 0x3fdc0000 { // |x| < 0.4375
		if ix < 0x3e400000 && huge+x > 1 { // |x| < 2^-27
			return x
		}
	} else {
		x = math.Abs(x)
		switch {
		case ix < 0x3fe60000: // 7/16 <= |x| < 11/16
			id = 0
			x = (float64(2.0*x) - 1) / (2.0 + x)
		case ix < 0x3ff30000: // 11/16 <= |x| < 19/16
			id = 1
			x = (x - 1) / (x + 1)
		case ix < 0x40038000: // |x| < 2.4375
			id = 2
			x = (x - 1.5) / (1 + float64(1.5*x))
		default: // 2.4375 <= |x| < 2^66
			id = 3
			x = -1.0 / x
		}
	}
	z := x * x
	w := z * z
	// Break the sum of aT[i]*z**(i+1) into odd and even polynomials.
	s1 := float64(z * (aT[0] + float64(w*(aT[2]+float64(w*(aT[4]+float64(w*(aT[6]+float64(w*(aT[8]+float64(w*aT[10])))))))))))
	s2 := float64(w * (aT[1] + float64(w*(aT[3]+float64(w*(aT[5]+float64(w*(aT[7]+float64(w*aT[9])))))))))
	if id < 0 {
		return x - float64(x*(s1+s2))
	}
	z = atanHi[id] - ((float64(x*(s1+s2)) - atanLo[id]) - x)
	if hx < 0 {
		return -z
	}
	return z
}

// Sin matches JavaScript Math.sin(x) (fdlibm sin).
func Sin(x float64) float64 {
	ix := hiWord(x) & 0x7fffffff
	switch {
	case ix <= 0x3fe921fb: // |x| ~< pi/4
		return kernelSin(x, 0, false)
	case ix >= 0x7ff00000: // Inf or NaN
		return x - x
	}
	n, y0, y1 := remPio2(x)
	switch n & 3 {
	case 0:
		return kernelSin(y0, y1, true)
	case 1:
		return kernelCos(y0, y1)
	case 2:
		return -kernelSin(y0, y1, true)
	default:
		return -kernelCos(y0, y1)
	}
}

// Cos matches JavaScript Math.cos(x) (fdlibm cos).
func Cos(x float64) float64 {
	ix := hiWord(x) & 0x7fffffff
	switch {
	case ix <= 0x3fe921fb: // |x| ~< pi/4
		return kernelCos(x, 0)
	case ix >= 0x7ff00000: // Inf or NaN
		return x - x
	}
	n, y0, y1 := remPio2(x)
	switch n & 3 {
	case 0:
		return kernelCos(y0, y1)
	case 1:
		return -kernelSin(y0, y1, true)
	case 2:
		return -kernelCos(y0, y1)
	default:
		return kernelSin(y0, y1, true)
	}
}

// kernelSin is __kernel_sin(x, y, iy): sin(x+y) for |x| ~< pi/4, where y is
// the tail of x; iy reports whether y is used.
func kernelSin(x, y float64, iy bool) float64 {
	const (
		half = 5.00000000000000000000e-01  // 0x3FE00000, 0x00000000
		S1   = -1.66666666666666324348e-01 // 0xBFC55555, 0x55555549
		S2   = 8.33333333332248946124e-03  // 0x3F811111, 0x1110F8A6
		S3   = -1.98412698298579493134e-04 // 0xBF2A01A0, 0x19C161D5
		S4   = 2.75573137070700676789e-06  // 0x3EC71DE3, 0x57B1FE7D
		S5   = -2.50507602534068634195e-08 // 0xBE5AE5E6, 0x8A2B9CEB
		S6   = 1.58969099521155010221e-10  // 0x3DE5D93A, 0x5ACFD57C
	)
	if hiWord(x)&0x7fffffff < 0x3e400000 && int(x) == 0 { // |x| < 2**-27
		return x
	}
	z := x * x
	v := z * x
	r := S2 + float64(z*(S3+float64(z*(S4+float64(z*(S5+float64(z*S6)))))))
	if !iy {
		return x + float64(v*(S1+float64(z*r)))
	}
	return x - ((float64(z*(float64(half*y)-float64(v*r))) - y) - float64(v*S1))
}

// kernelCos is __kernel_cos(x, y): cos(x+y) for |x| ~< pi/4, where y is the
// tail of x.
func kernelCos(x, y float64) float64 {
	const (
		C1 = 4.16666666666666019037e-02  // 0x3FA55555, 0x5555554C
		C2 = -1.38888888888741095749e-03 // 0xBF56C16C, 0x16C15177
		C3 = 2.48015872894767294178e-05  // 0x3EFA01A0, 0x19CB1590
		C4 = -2.75573143513906633035e-07 // 0xBE927E4F, 0x809C52AD
		C5 = 2.08757232129817482790e-09  // 0x3E21EE9E, 0xBDB4B1C4
		C6 = -1.13596475577881948265e-11 // 0xBDA8FAE9, 0xBE8838D4
	)
	ix := hiWord(x) & 0x7fffffff
	if ix < 0x3e400000 && int(x) == 0 { // |x| < 2**-27
		return 1
	}
	z := x * x
	r := z * (C1 + float64(z*(C2+float64(z*(C3+float64(z*(C4+float64(z*(C5+float64(z*C6))))))))))
	if ix < 0x3FD33333 { // |x| < 0.3
		return 1 - (float64(0.5*z) - (float64(z*r) - float64(x*y)))
	}
	qx := 0.28125 // |x| > 0.78125
	if ix <= 0x3fe90000 {
		qx = fromWords(ix-0x00200000, 0) // x/4
	}
	hz := float64(0.5*z) - qx
	a := 1 - qx
	return a - (hz - (float64(z*r) - float64(x*y)))
}

// twoOverPi holds the 24-bit chunks of the binary expansion of 2/pi
// (fdlibm's two_over_pi).
var twoOverPi = [66]int32{
	0xA2F983, 0x6E4E44, 0x1529FC, 0x2757D1, 0xF534DD, 0xC0DB62,
	0x95993C, 0x439041, 0xFE5163, 0xABDEBB, 0xC561B7, 0x246E3A,
	0x424DD2, 0xE00649, 0x2EEA09, 0xD1921C, 0xFE1DEB, 0x1CB129,
	0xA73EE8, 0x8235F5, 0x2EBB44, 0x84E99C, 0x7026B4, 0x5F7E41,
	0x3991D6, 0x398353, 0x39F49C, 0x845F8B, 0xBDF928, 0x3B1FF8,
	0x97FFDE, 0x05980F, 0xEF2F11, 0x8B5A0A, 0x6D1F6D, 0x367ECF,
	0x27CB09, 0xB74F46, 0x3F669E, 0x5FEA2D, 0x7527BA, 0xC7EBE5,
	0xF17B3D, 0x0739F7, 0x8A5292, 0xEA6BFB, 0x5FB11F, 0x8D5D08,
	0x560330, 0x46FC7B, 0x6BABF0, 0xCFBC20, 0x9AF436, 0x1DA9E3,
	0x91615E, 0xE61B08, 0x659985, 0x5F14A0, 0x68408D, 0xFFD880,
	0x4D7327, 0x310606, 0x1556CA, 0x73A8C9, 0x60E27B, 0xC08C6B,
}

// npio2Hw holds the high words of n*pi/2 for n = 1..32.
var npio2Hw = [32]int32{
	0x3FF921FB, 0x400921FB, 0x4012D97C, 0x401921FB, 0x401F6A7A, 0x4022D97C,
	0x4025FDBB, 0x402921FB, 0x402C463A, 0x402F6A7A, 0x4031475C, 0x4032D97C,
	0x40346B9C, 0x4035FDBB, 0x40378FDB, 0x403921FB, 0x403AB41B, 0x403C463A,
	0x403DD85A, 0x403F6A7A, 0x40407E4C, 0x4041475C, 0x4042106C, 0x4042D97C,
	0x4043A28C, 0x40446B9C, 0x404534AC, 0x4045FDBB, 0x4046C6CB, 0x40478FDB,
	0x404858EB, 0x404921FB,
}

// remPio2 is __ieee754_rem_pio2: x reduced modulo pi/2 to y0 + y1, and n,
// the number of quadrants removed.
func remPio2(x float64) (n int32, y0, y1 float64) {
	const (
		half    = 5.00000000000000000000e-01 // 0x3FE00000, 0x00000000
		two24   = 1.67772160000000000000e+07 // 0x41700000, 0x00000000
		invpio2 = 6.36619772367581382433e-01 // 0x3FE45F30, 0x6DC9C883: 53 bits of 2/pi
		pio2_1  = 1.57079632673412561417e+00 // 0x3FF921FB, 0x54400000: first 33 bits of pi/2
		pio2_1t = 6.07710050650619224932e-11 // 0x3DD0B461, 0x1A626331: pi/2 - pio2_1
		pio2_2  = 6.07710050630396597660e-11 // 0x3DD0B461, 0x1A600000: second 33 bits of pi/2
		pio2_2t = 2.02226624879595063154e-21 // 0x3BA3198A, 0x2E037073: pi/2 - (pio2_1+pio2_2)
		pio2_3  = 2.02226624871116645580e-21 // 0x3BA3198A, 0x2E000000: third 33 bits of pi/2
		pio2_3t = 8.47842766036889956997e-32 // 0x397B839A, 0x252049C1: pi/2 - (pio2_1+pio2_2+pio2_3)
	)
	hx := hiWord(x)
	ix := hx & 0x7fffffff
	if ix <= 0x3fe921fb { // |x| ~<= pi/4: no reduction needed
		return 0, x, 0
	}
	if ix < 0x4002d97c { // |x| < 3pi/4: special case with n = +-1
		if hx > 0 {
			z := x - pio2_1
			if ix != 0x3ff921fb { // 33+53 bit pi is good enough
				y0 = z - pio2_1t
				y1 = (z - y0) - pio2_1t
			} else { // near pi/2: use 33+33+53 bit pi
				z -= pio2_2
				y0 = z - pio2_2t
				y1 = (z - y0) - pio2_2t
			}
			return 1, y0, y1
		}
		z := x + pio2_1
		if ix != 0x3ff921fb {
			y0 = z + pio2_1t
			y1 = (z - y0) + pio2_1t
		} else {
			z += pio2_2
			y0 = z + pio2_2t
			y1 = (z - y0) + pio2_2t
		}
		return -1, y0, y1
	}
	if ix <= 0x413921fb { // |x| ~<= 2^19*(pi/2): medium size
		t := math.Abs(x)
		n = int32(float64(t*invpio2) + half)
		fn := float64(n)
		r := t - float64(fn*pio2_1)
		w := float64(fn * pio2_1t) // 1st round good to 85 bits
		if n < 32 && ix != npio2Hw[n-1] {
			y0 = r - w // quick check: no cancellation
		} else {
			j := ix >> 20
			y0 = r - w
			if i := j - (hiWord(y0)>>20)&0x7ff; i > 16 { // 2nd iteration needed, good to 118 bits
				t = r
				w = float64(fn * pio2_2)
				r = t - w
				w = float64(fn*pio2_2t) - ((t - r) - w)
				y0 = r - w
				if i = j - (hiWord(y0)>>20)&0x7ff; i > 49 { // 3rd iteration, 151 bits
					t = r
					w = float64(fn * pio2_3)
					r = t - w
					w = float64(fn*pio2_3t) - ((t - r) - w)
					y0 = r - w
				}
			}
		}
		y1 = (r - y0) - w
		if hx < 0 {
			return -n, -y0, -y1
		}
		return n, y0, y1
	}
	if ix >= 0x7ff00000 { // Inf or NaN
		return 0, x - x, x - x
	}
	// All other (large) arguments: z = scalbn(|x|, ilogb(x)-23) split into
	// three 24-bit pieces.
	e0 := ix>>20 - 1046 // e0 = ilogb(z) - 23
	z := fromWords(ix-e0<<20, loWord(x))
	var tx [3]float64
	for i := 0; i < 2; i++ {
		tx[i] = float64(int32(z))
		z = (z - tx[i]) * two24
	}
	tx[2] = z
	nx := 3
	for tx[nx-1] == 0 { // skip zero terms
		nx--
	}
	var y [2]float64
	n = kernelRemPio2(tx[:nx], y[:], int(e0))
	if hx < 0 {
		return -n, -y[0], -y[1]
	}
	return n, y[0], y[1]
}

// pio2Chunks holds pi/2 in pieces of 24 bits (k_rem_pio2.c's PIo2).
var pio2Chunks = [8]float64{
	1.57079625129699707031e+00, // 0x3FF921FB, 0x40000000
	7.54978941586159635335e-08, // 0x3E74442D, 0x00000000
	5.39030252995776476554e-15, // 0x3CF84698, 0x80000000
	3.28200341580791294123e-22, // 0x3B78CC51, 0x60000000
	1.27065575308067607349e-29, // 0x39F01B83, 0x80000000
	1.22933308981111328932e-36, // 0x387A2520, 0x40000000
	2.73370053816464559624e-44, // 0x36E38222, 0x80000000
	2.16741683877804819444e-51, // 0x3569F31D, 0x00000000
}

// kernelRemPio2 is __kernel_rem_pio2(x, y, e0, nx, prec=2, two_over_pi):
// the Payne-Hanek reduction of a large argument given as 24-bit pieces x
// with exponent e0. It stores the reduced value in y[0] + y[1] and returns
// the low three bits of the quadrant.
func kernelRemPio2(x, y []float64, e0 int) int32 {
	const (
		two24  = 1.67772160000000000000e+07 // 0x41700000, 0x00000000
		twon24 = 5.96046447753906250000e-08 // 0x3E700000, 0x00000000
		jk     = 4                          // init_jk[prec] for prec = 2 (double)
		jp     = jk
	)
	var (
		iq       [20]int32
		f, fq, q [20]float64
	)
	nx := len(x)
	// Determine jx, jv, q0; note that 3 > q0.
	jx := nx - 1
	jv := (e0 - 3) / 24
	if jv < 0 {
		jv = 0
	}
	q0 := e0 - 24*(jv+1)

	// Set up f[0] to f[jx+jk] where f[jx+jk] = twoOverPi[jv+jk].
	j := jv - jx
	m := jx + jk
	for i := 0; i <= m; i, j = i+1, j+1 {
		if j >= 0 {
			f[i] = float64(twoOverPi[j])
		}
	}
	// Compute q[0], q[1], ... q[jk].
	for i := 0; i <= jk; i++ {
		fw := 0.0
		for j := 0; j <= jx; j++ {
			fw += float64(x[j] * f[jx+i-j])
		}
		q[i] = fw
	}

	jz := jk
	var z float64
	var n, ih int32
	for {
		// Distill q[] into iq[] reversingly.
		i := 0
		z = q[jz]
		for j := jz; j > 0; i, j = i+1, j-1 {
			fw := float64(int32(float64(twon24 * z)))
			iq[i] = int32(z - float64(two24*fw))
			z = q[j-1] + fw
		}

		// Compute n.
		z = scalbn(z, q0)                       // actual value of z
		z -= float64(8.0 * math.Floor(z*0.125)) // trim off integer >= 8
		n = int32(z)
		z -= float64(n)
		ih = 0
		switch {
		case q0 > 0: // need iq[jz-1] to determine n
			i := iq[jz-1] >> (24 - q0)
			n += i
			iq[jz-1] -= i << (24 - q0)
			ih = iq[jz-1] >> (23 - q0)
		case q0 == 0:
			ih = iq[jz-1] >> 23
		case z >= 0.5:
			ih = 2
		}

		if ih > 0 { // q > 0.5
			n++
			carry := int32(0)
			for i := 0; i < jz; i++ { // compute 1 - q
				j := iq[i]
				if carry == 0 {
					if j != 0 {
						carry = 1
						iq[i] = 0x1000000 - j
					}
				} else {
					iq[i] = 0xffffff - j
				}
			}
			if q0 > 0 { // rare case: chance is 1 in 12
				switch q0 {
				case 1:
					iq[jz-1] &= 0x7fffff
				case 2:
					iq[jz-1] &= 0x3fffff
				}
			}
			if ih == 2 {
				z = 1 - z
				if carry != 0 {
					z -= scalbn(1, q0)
				}
			}
		}

		// Check if recomputation is needed.
		if z != 0 {
			break
		}
		j := int32(0)
		for i := jz - 1; i >= jk; i-- {
			j |= iq[i]
		}
		if j != 0 {
			break
		}
		k := 1 // k = number of terms needed
		for iq[jk-k] == 0 {
			k++
		}
		for i := jz + 1; i <= jz+k; i++ { // add q[jz+1] to q[jz+k]
			f[jx+i] = float64(twoOverPi[jv+i])
			fw := 0.0
			for j := 0; j <= jx; j++ {
				fw += float64(x[j] * f[jx+i-j])
			}
			q[i] = fw
		}
		jz += k
	}

	// Chop off zero terms.
	if z == 0 {
		jz--
		q0 -= 24
		for iq[jz] == 0 {
			jz--
			q0 -= 24
		}
	} else { // break z into 24-bit pieces if necessary
		z = scalbn(z, -q0)
		if z >= two24 {
			fw := float64(int32(float64(twon24 * z)))
			iq[jz] = int32(z - float64(two24*fw))
			jz++
			q0 += 24
			iq[jz] = int32(fw)
		} else {
			iq[jz] = int32(z)
		}
	}

	// Convert the integer "bit" chunks to floating-point values.
	fw := scalbn(1, q0)
	for i := jz; i >= 0; i-- {
		q[i] = fw * float64(iq[i])
		fw *= twon24
	}

	// Compute pio2Chunks[0..jp] * q[jz..0].
	for i := jz; i >= 0; i-- {
		fw := 0.0
		for k := 0; k <= jp && k <= jz-i; k++ {
			fw += float64(pio2Chunks[k] * q[i+k])
		}
		fq[jz-i] = fw
	}

	// Compress fq[] into y[] (prec = 2).
	fw = 0.0
	for i := jz; i >= 0; i-- {
		fw += fq[i]
	}
	if ih == 0 {
		y[0] = fw
	} else {
		y[0] = -fw
	}
	fw = fq[0] - fw
	for i := 1; i <= jz; i++ {
		fw += fq[i]
	}
	if ih == 0 {
		y[1] = fw
	} else {
		y[1] = -fw
	}
	return n & 7
}

// scalbn is fdlibm's scalbn(x, n): x * 2**n computed by exponent manipulation.
func scalbn(x float64, n int) float64 {
	const (
		two54  = 1.80143985094819840000e+16 // 0x43500000, 0x00000000
		twom54 = 5.55111512312578270212e-17 // 0x3C900000, 0x00000000
		huge   = 1.0e+300
		tiny   = 1.0e-300
	)
	hx, lx := hiWord(x), loWord(x)
	k := int((hx & 0x7ff00000) >> 20) // extract exponent
	if k == 0 {                       // 0 or subnormal x
		if lx|uint32(hx&0x7fffffff) == 0 {
			return x // +-0
		}
		x *= two54
		hx = hiWord(x)
		k = int((hx&0x7ff00000)>>20) - 54
		if n < -50000 {
			return tiny * x // underflow
		}
	}
	if k == 0x7ff {
		return x + x // NaN or Inf
	}
	k += n
	if k > 0x7fe {
		return huge * math.Copysign(huge, x) // overflow
	}
	if k > 0 { // normal result
		return withHiWord(x, hx&^0x7ff00000|int32(k)<<20)
	}
	if k <= -54 {
		if n > 50000 { // in case integer overflow in n+k
			return huge * math.Copysign(huge, x) // overflow
		}
		return tiny * math.Copysign(tiny, x) // underflow
	}
	k += 54 // subnormal result
	x = withHiWord(x, hx&^0x7ff00000|int32(k)<<20)
	return x * twom54
}
