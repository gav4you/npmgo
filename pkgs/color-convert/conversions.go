package colorconvert

// conversions.js: the direct conversions between models. Each function takes
// the value the wrapper in index.js passes on (after input) and returns a
// fresh Value. The arithmetic follows the JavaScript expression by
// expression; every product that feeds an addition or subtraction is wrapped
// in float64() so that no Go compiler fuses it into a fused multiply-add,
// which JavaScript never does.

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/gav4you/npmgo/internal/jsnum"
	"github.com/gav4you/npmgo/internal/jsvalue"
	colorname "github.com/gav4you/npmgo/pkgs/color-name"
)

type rawFunc func(Value) Value

// conversion is one function of conversions.js, convert[from][to]. id names
// the JavaScript function object: convert.gray.hsv is assigned
// convert.gray.hsl, so the two share one object (and one .conversion
// property, see route).
type conversion struct {
	from, to string
	fn       rawFunc
	id       string
}

// conversions lists the functions in the order conversions.js assigns them,
// which is the order Object.keys returns them in and so the order in which
// route.js's breadth-first search visits a model's neighbours.
var conversions = []conversion{
	{"rgb", "hsl", rgbToHSL, ""},
	{"rgb", "hsv", rgbToHSV, ""},
	{"rgb", "hwb", rgbToHWB, ""},
	{"rgb", "cmyk", rgbToCMYK, ""},
	{"rgb", "keyword", rgbToKeyword, ""},
	{"keyword", "rgb", keywordToRGB, ""},
	{"rgb", "xyz", rgbToXYZ, ""},
	{"rgb", "lab", rgbToLab, ""},
	{"hsl", "rgb", hslToRGB, ""},
	{"hsl", "hsv", hslToHSV, ""},
	{"hsv", "rgb", hsvToRGB, ""},
	{"hsv", "hsl", hsvToHSL, ""},
	{"hwb", "rgb", hwbToRGB, ""},
	{"cmyk", "rgb", cmykToRGB, ""},
	{"xyz", "rgb", xyzToRGB, ""},
	{"xyz", "lab", xyzToLab, ""},
	{"lab", "xyz", labToXYZ, ""},
	{"lab", "lch", labToLCH, ""},
	{"lch", "lab", lchToLab, ""},
	{"rgb", "ansi16", rgbToANSI16, ""},
	{"hsv", "ansi16", hsvToANSI16, ""},
	{"rgb", "ansi256", rgbToANSI256, ""},
	{"ansi16", "rgb", ansi16ToRGB, ""},
	{"ansi256", "rgb", ansi256ToRGB, ""},
	{"rgb", "hex", rgbToHex, ""},
	{"hex", "rgb", hexToRGB, ""},
	{"rgb", "hcg", rgbToHCG, ""},
	{"hsl", "hcg", hslToHCG, ""},
	{"hsv", "hcg", hsvToHCG, ""},
	{"hcg", "rgb", hcgToRGB, ""},
	{"hcg", "hsv", hcgToHSV, ""},
	{"hcg", "hsl", hcgToHSL, ""},
	{"hcg", "hwb", hcgToHWB, ""},
	{"hwb", "hcg", hwbToHCG, ""},
	{"apple", "rgb", appleToRGB, ""},
	{"rgb", "apple", rgbToApple, ""},
	{"gray", "rgb", grayToRGB, ""},
	{"gray", "hsl", grayToHSL, "gray.hsl"},
	{"gray", "hsv", grayToHSL, "gray.hsl"}, // convert.gray.hsv = convert.gray.hsl
	{"gray", "hwb", grayToHWB, ""},
	{"gray", "cmyk", grayToCMYK, ""},
	{"gray", "lab", grayToLab, ""},
	{"gray", "hex", grayToHex, ""},
	{"rgb", "gray", rgbToGray, ""},
}

const (
	readZero    = "Cannot read properties of undefined (reading '0')"
	notIterable = "args is not iterable" // const [r, g, b] = args
)

// channels returns the channel values a conversion reads as args[0],
// args[1], …. Reading them from undefined (a nil Value, which keyword → rgb
// and hsv → rgb can produce) throws V8's TypeError.
func channels(v Value) []float64 {
	switch x := v.(type) {
	case []float64:
		return x
	case nil:
		panic(&jsvalue.TypeError{Message: readZero})
	}
	panic(fmt.Sprintf("colorconvert: unexpected %T value", v))
}

// at is args[i]; past the end JavaScript reads undefined, which every
// arithmetic use turns into NaN.
func at(args []float64, i int) float64 {
	if i < len(args) {
		return args[i]
	}
	return math.NaN()
}

// orZero is x || 0: NaN and -0 become 0.
func orZero(x float64) float64 {
	if x == 0 || math.IsNaN(x) {
		return 0
	}
	return x
}

func rgbToHSL(v Value) Value {
	rgb := channels(v)
	r := at(rgb, 0) / 255
	g := at(rgb, 1) / 255
	b := at(rgb, 2) / 255
	mn := min(r, g, b)
	mx := max(r, g, b)
	delta := mx - mn
	h := math.NaN() // stays undefined when a channel is NaN
	switch {
	case mx == mn:
		h = 0
	case r == mx:
		h = (g - b) / delta
	case g == mx:
		h = 2 + (b-r)/delta
	case b == mx:
		h = 4 + (r-g)/delta
	}
	h = min(h*60, 360)
	if h < 0 {
		h += 360
	}
	l := (mn + mx) / 2
	var s float64
	switch {
	case mx == mn:
		s = 0
	case l <= 0.5:
		s = delta / (mx + mn)
	default:
		s = delta / (2 - mx - mn)
	}
	return []float64{h, s * 100, l * 100}
}

func rgbToHSV(v Value) Value {
	rgb := channels(v)
	r := at(rgb, 0) / 255
	g := at(rgb, 1) / 255
	b := at(rgb, 2) / 255
	val := max(r, g, b)
	diff := val - min(r, g, b)
	diffc := func(c float64) float64 { return (val-c)/6/diff + 1.0/2 }
	h, s := math.NaN(), 0.0 // h stays undefined when a channel is NaN
	if diff != 0 {
		s = diff / val
		rdif, gdif, bdif := diffc(r), diffc(g), diffc(b)
		switch {
		case r == val:
			h = bdif - gdif
		case g == val:
			h = 1.0/3 + rdif - bdif
		case b == val:
			h = 2.0/3 + gdif - rdif
		}
		if h < 0 {
			h++
		} else if h > 1 {
			h--
		}
	} else {
		h = 0
	}
	return []float64{h * 360, s * 100, val * 100}
}

func rgbToHWB(v Value) Value {
	rgb := channels(v)
	r, g, b := at(rgb, 0), at(rgb, 1), at(rgb, 2)
	h := channels(rgbToHSL(rgb))[0]
	w := 1.0 / 255 * min(r, min(g, b))
	b = 1 - float64(1.0/255*max(r, max(g, b)))
	return []float64{h, w * 100, b * 100}
}

func rgbToCMYK(v Value) Value {
	rgb := channels(v)
	r := at(rgb, 0) / 255
	g := at(rgb, 1) / 255
	b := at(rgb, 2) / 255
	k := min(1-r, 1-g, 1-b)
	c := orZero((1 - r - k) / (1 - k))
	m := orZero((1 - g - k) / (1 - k))
	y := orZero((1 - b - k) / (1 - k))
	return []float64{c * 100, m * 100, y * 100, k * 100}
}

// comparativeDistance is the squared Euclidean distance. JavaScript squares
// with ** 2, which V8 hands to the C library's pow; d*d is that value
// whenever it is exact (integer channels) and the correctly rounded one
// otherwise.
func comparativeDistance(x []float64, y [3]uint8) float64 {
	d0 := at(x, 0) - float64(y[0])
	d1 := at(x, 1) - float64(y[1])
	d2 := at(x, 2) - float64(y[2])
	return float64(d0*d0) + float64(d1*d1) + float64(d2*d2)
}

// reverseKeywords maps String([r, g, b]) to a keyword; where two keywords
// share a value (aqua and cyan, …) the later one wins, as in the JavaScript
// object.
var reverseKeywords = func() map[string]string {
	m := make(map[string]string, len(colorname.Names))
	for _, name := range colorname.Names {
		c := colorname.RGB[name]
		m[fmt.Sprintf("%d,%d,%d", c[0], c[1], c[2])] = name
	}
	return m
}()

// joinNumbers is String(array) for an array of numbers.
func joinNumbers(a []float64) string {
	parts := make([]string, len(a))
	for i, x := range a {
		parts[i] = jsnum.String(x)
	}
	return strings.Join(parts, ",")
}

func rgbToKeyword(v Value) Value {
	rgb := channels(v)
	if name, ok := reverseKeywords[joinNumbers(rgb)]; ok {
		return name
	}
	closest := math.Inf(1)
	var keyword Value // undefined until a distance is below Infinity
	for _, name := range colorname.Names {
		if d := comparativeDistance(rgb, colorname.RGB[name]); d < closest {
			closest = d
			keyword = name
		}
	}
	return keyword
}

func keywordToRGB(v Value) Value {
	c, ok := colorname.RGB[v.(string)]
	if !ok {
		return nil
	}
	return []float64{float64(c[0]), float64(c[1]), float64(c[2])}
}

// linear is the sRGB transfer function's inverse used by rgb.xyz.
func linear(c float64) float64 {
	if c > 0.04045 {
		return jsnum.Pow((c+0.055)/1.055, 2.4)
	}
	return c / 12.92
}

func rgbToXYZ(v Value) Value {
	rgb := channels(v)
	r := linear(at(rgb, 0) / 255)
	g := linear(at(rgb, 1) / 255)
	b := linear(at(rgb, 2) / 255)
	x := float64(r*0.4124) + float64(g*0.3576) + float64(b*0.1805)
	y := float64(r*0.2126) + float64(g*0.7152) + float64(b*0.0722)
	z := float64(r*0.0193) + float64(g*0.1192) + float64(b*0.9505)
	return []float64{x * 100, y * 100, z * 100}
}

// labF is the CIELAB companding step shared by rgb.lab and xyz.lab.
func labF(t float64) float64 {
	if t > 0.008856 {
		return jsnum.Pow(t, 1.0/3)
	}
	return float64(7.787*t) + 16.0/116
}

// xyzLab is the body rgb.lab and xyz.lab share after obtaining x, y, z.
func xyzLab(x, y, z float64) Value {
	x = labF(x / 95.047)
	y = labF(y / 100)
	z = labF(z / 108.883)
	l := float64(116*y) - 16
	a := 500 * (x - y)
	b := 200 * (y - z)
	return []float64{l, a, b}
}

func rgbToLab(v Value) Value {
	xyz := channels(rgbToXYZ(v))
	return xyzLab(xyz[0], xyz[1], xyz[2])
}

func hslToRGB(v Value) Value {
	hsl := channels(v)
	h := at(hsl, 0) / 360
	s := at(hsl, 1) / 100
	l := at(hsl, 2) / 100
	if s == 0 {
		val := l * 255
		return []float64{val, val, val}
	}
	var t2 float64
	if l < 0.5 {
		t2 = float64(l * (1 + s))
	} else {
		t2 = l + s - float64(l*s)
	}
	t1 := float64(2*l) - t2
	rgb := make([]float64, 3)
	for i, k := range [3]float64{1, math.Copysign(0, -1), -1} { // -(i - 1)
		t3 := h + float64(1.0/3*k)
		if t3 < 0 {
			t3++
		}
		if t3 > 1 {
			t3--
		}
		var val float64
		switch {
		case 6*t3 < 1:
			val = t1 + float64((t2-t1)*6*t3)
		case 2*t3 < 1:
			val = t2
		case 3*t3 < 2:
			val = t1 + float64((t2-t1)*(2.0/3-t3)*6)
		default:
			val = t1
		}
		rgb[i] = val * 255
	}
	return rgb
}

func hslToHSV(v Value) Value {
	hsl := channels(v)
	h := at(hsl, 0)
	s := at(hsl, 1) / 100
	l := at(hsl, 2) / 100
	smin := s
	lmin := max(l, 0.01)
	l = float64(l * 2)
	if l <= 1 {
		s = float64(s * l)
	} else {
		s = float64(s * (2 - l))
	}
	if lmin <= 1 {
		smin = float64(smin * lmin)
	} else {
		smin = float64(smin * (2 - lmin))
	}
	val := (l + s) / 2
	var sv float64
	if l == 0 {
		sv = (2 * smin) / (lmin + smin)
	} else {
		sv = (2 * s) / (l + s)
	}
	return []float64{h, sv * 100, val * 100}
}

func hsvToRGB(v Value) Value {
	hsv := channels(v)
	h := at(hsv, 0) / 60
	s := at(hsv, 1) / 100
	val := at(hsv, 2) / 100
	hi := math.Mod(math.Floor(h), 6)
	f := h - math.Floor(h)
	p := 255 * val * (1 - s)
	q := 255 * val * (1 - float64(s*f))
	t := 255 * val * (1 - float64(s*(1-f)))
	val *= 255
	switch hi {
	case 0:
		return []float64{val, t, p}
	case 1:
		return []float64{q, val, p}
	case 2:
		return []float64{p, val, t}
	case 3:
		return []float64{p, q, val}
	case 4:
		return []float64{t, p, val}
	case 5:
		return []float64{val, p, q}
	}
	return nil // Math.floor(h) % 6 is negative or NaN: no case matches
}

func hsvToHSL(v Value) Value {
	hsv := channels(v)
	h := at(hsv, 0)
	s := at(hsv, 1) / 100
	val := at(hsv, 2) / 100
	vmin := max(val, 0.01)
	l := (2 - s) * val
	lmin := float64((2 - s) * vmin)
	sl := s * vmin
	if lmin <= 1 {
		sl /= lmin
	} else {
		sl /= 2 - lmin
	}
	sl = orZero(sl)
	l /= 2
	return []float64{h, sl * 100, l * 100}
}

func hwbToRGB(v Value) Value {
	hwb := channels(v)
	h := at(hwb, 0) / 360
	wh := at(hwb, 1) / 100
	bl := at(hwb, 2) / 100
	if ratio := wh + bl; ratio > 1 { // wh + bl can't be > 1
		wh /= ratio
		bl /= ratio
	}
	i := math.Floor(6 * h)
	val := 1 - bl
	f := float64(6*h) - i
	if jsnum.ToInt32(i)&0x01 != 0 {
		f = 1 - f
	}
	n := wh + float64(f*(val-wh)) // linear interpolation
	var r, g, b float64
	switch i {
	case 1:
		r, g, b = n, val, wh
	case 2:
		r, g, b = wh, val, n
	case 3:
		r, g, b = wh, n, val
	case 4:
		r, g, b = n, wh, val
	case 5:
		r, g, b = val, wh, n
	default: // case 6, case 0 and anything else
		r, g, b = val, n, wh
	}
	return []float64{r * 255, g * 255, b * 255}
}

func cmykToRGB(v Value) Value {
	cmyk := channels(v)
	c := at(cmyk, 0) / 100
	m := at(cmyk, 1) / 100
	y := at(cmyk, 2) / 100
	k := at(cmyk, 3) / 100
	r := 1 - min(1, float64(c*(1-k))+k)
	g := 1 - min(1, float64(m*(1-k))+k)
	b := 1 - min(1, float64(y*(1-k))+k)
	return []float64{r * 255, g * 255, b * 255}
}

// gamma is the sRGB transfer function used by xyz.rgb.
func gamma(c float64) float64 {
	if c > 0.0031308 {
		return float64(1.055*jsnum.Pow(c, 1.0/2.4)) - 0.055
	}
	return c * 12.92
}

func xyzToRGB(v Value) Value {
	xyz := channels(v)
	x := at(xyz, 0) / 100
	y := at(xyz, 1) / 100
	z := at(xyz, 2) / 100
	r := float64(x*3.2406) + float64(y*-1.5372) + float64(z*-0.4986)
	g := float64(x*-0.9689) + float64(y*1.8758) + float64(z*0.0415)
	b := float64(x*0.0557) + float64(y*-0.2040) + float64(z*1.0570)
	r = min(max(0, gamma(r)), 1)
	g = min(max(0, gamma(g)), 1)
	b = min(max(0, gamma(b)), 1)
	return []float64{r * 255, g * 255, b * 255}
}

func xyzToLab(v Value) Value {
	xyz := channels(v)
	return xyzLab(at(xyz, 0), at(xyz, 1), at(xyz, 2))
}

func labToXYZ(v Value) Value {
	lab := channels(v)
	l, a, b := at(lab, 0), at(lab, 1), at(lab, 2)
	y := (l + 16) / 116
	x := a/500 + y
	z := y - b/200
	y2 := jsnum.Pow(y, 3)
	x2 := jsnum.Pow(x, 3)
	z2 := jsnum.Pow(z, 3)
	if y2 > 0.008856 {
		y = y2
	} else {
		y = (y - 16.0/116) / 7.787
	}
	if x2 > 0.008856 {
		x = x2
	} else {
		x = (x - 16.0/116) / 7.787
	}
	if z2 > 0.008856 {
		z = z2
	} else {
		z = (z - 16.0/116) / 7.787
	}
	return []float64{x * 95.047, y * 100, z * 108.883}
}

func labToLCH(v Value) Value {
	lab := channels(v)
	l, a, b := at(lab, 0), at(lab, 1), at(lab, 2)
	hr := jsnum.Atan2(b, a)
	h := hr * 360 / 2 / math.Pi
	if h < 0 {
		h += 360
	}
	c := math.Sqrt(float64(a*a) + float64(b*b))
	return []float64{l, c, h}
}

func lchToLab(v Value) Value {
	lch := channels(v)
	l, c, h := at(lch, 0), at(lch, 1), at(lch, 2)
	hr := h / 360 * 2 * math.Pi
	a := c * jsnum.Cos(hr)
	b := c * jsnum.Sin(hr)
	return []float64{l, a, b}
}

func rgbToANSI16(v Value) Value { return ansi16(v, nil) }

// ansi16 is convert.rgb.ansi16(args, saturation = null); a nil saturation
// is null.
func ansi16(v Value, saturation *float64) float64 {
	if v == nil { // const [r, g, b] = args
		panic(&jsvalue.TypeError{Message: notIterable})
	}
	args := channels(v)
	r, g, b := at(args, 0), at(args, 1), at(args, 2)
	var value float64
	if saturation == nil {
		value = channels(rgbToHSV(args))[2]
	} else {
		value = *saturation // hsv -> ansi16 optimization
	}
	value = jsnum.Round(value / 50)
	if value == 0 {
		return 30
	}
	ansi := 30 + float64(jsnum.ToInt32(jsnum.Round(b/255))<<2|
		jsnum.ToInt32(jsnum.Round(g/255))<<1|
		jsnum.ToInt32(jsnum.Round(r/255)))
	if value == 2 {
		ansi += 60
	}
	return ansi
}

func hsvToANSI16(v Value) Value {
	args := channels(v)
	var saturation *float64 // args[2]; undefined takes the default, null
	if len(args) > 2 {
		saturation = &args[2]
	}
	return ansi16(hsvToRGB(args), saturation)
}

func rgbToANSI256(v Value) Value {
	args := channels(v)
	r, g, b := at(args, 0), at(args, 1), at(args, 2)
	// The extended greyscale palette, except for black and white.
	if r == g && g == b {
		if r < 8 {
			return 16.0
		}
		if r > 248 {
			return 231.0
		}
		return jsnum.Round((r-8)/247*24) + 232
	}
	return 16 + float64(36*jsnum.Round(r/255*5)) + float64(6*jsnum.Round(g/255*5)) + jsnum.Round(b/255*5)
}

// code is a number argument as an ansi16 / ansi256 conversion reads it:
// arithmetic on the one-element array [n] makes -0 into +0.
func code(v Value) float64 {
	n := v.(float64)
	if n == 0 {
		return 0
	}
	return n
}

func ansi16ToRGB(v Value) Value {
	n := code(v)
	color := math.Mod(n, 10)
	if color == 0 || color == 7 { // greyscale
		if n > 50 {
			color += 3.5
		}
		color = color / 10.5 * 255
		return []float64{color, color, color}
	}
	mult := 0.5
	if n > 50 {
		mult = 1
	}
	c := jsnum.ToInt32(color)
	r := float64(c&1) * mult * 255
	g := float64((c>>1)&1) * mult * 255
	b := float64((c>>2)&1) * mult * 255
	return []float64{r, g, b}
}

func ansi256ToRGB(v Value) Value {
	n := code(v)
	if n >= 232 { // greyscale
		c := float64((n-232)*10) + 8
		return []float64{c, c, c}
	}
	n -= 16
	r := math.Floor(n/36) / 5 * 255
	rem := math.Mod(n, 36)
	g := math.Floor(rem/6) / 5 * 255
	b := math.Mod(rem, 6) / 5 * 255
	return []float64{r, g, b}
}

// byte8 is Math.round(x) & 0xFF.
func byte8(x float64) int32 { return jsnum.ToInt32(jsnum.Round(x)) & 0xFF }

func rgbToHex(v Value) Value {
	args := channels(v)
	integer := byte8(at(args, 0))<<16 + byte8(at(args, 1))<<8 + byte8(at(args, 2))
	return fmt.Sprintf("%06X", integer)
}

var hexRe = regexp.MustCompile(`(?i)[a-f0-9]{6}|[a-f0-9]{3}`)

func hexToRGB(v Value) Value {
	m := hexRe.FindString(v.(string))
	if m == "" {
		return []float64{0, 0, 0}
	}
	if len(m) == 3 {
		m = string([]byte{m[0], m[0], m[1], m[1], m[2], m[2]})
	}
	integer, _ := strconv.ParseInt(m, 16, 64)
	return []float64{float64(integer >> 16 & 0xFF), float64(integer >> 8 & 0xFF), float64(integer & 0xFF)}
}

func rgbToHCG(v Value) Value {
	rgb := channels(v)
	r := at(rgb, 0) / 255
	g := at(rgb, 1) / 255
	b := at(rgb, 2) / 255
	mx := max(max(r, g), b)
	mn := min(min(r, g), b)
	chroma := mx - mn
	grayscale := 0.0
	if chroma < 1 {
		grayscale = mn / (1 - chroma)
	}
	var hue float64
	switch {
	case chroma <= 0:
		hue = 0
	case mx == r:
		hue = math.Mod((g-b)/chroma, 6)
	case mx == g:
		hue = 2 + (b-r)/chroma
	default:
		hue = 4 + (r-g)/chroma
	}
	hue /= 6
	hue = math.Mod(hue, 1)
	return []float64{hue * 360, chroma * 100, grayscale * 100}
}

func hslToHCG(v Value) Value {
	hsl := channels(v)
	s := at(hsl, 1) / 100
	l := at(hsl, 2) / 100
	var c float64
	if l < 0.5 {
		c = float64(2.0 * s * l)
	} else {
		c = float64(2.0 * s * (1.0 - l))
	}
	f := 0.0
	if c < 1.0 {
		f = (l - float64(0.5*c)) / (1.0 - c)
	}
	return []float64{at(hsl, 0), c * 100, f * 100}
}

func hsvToHCG(v Value) Value {
	hsv := channels(v)
	s := at(hsv, 1) / 100
	val := at(hsv, 2) / 100
	c := float64(s * val)
	f := 0.0
	if c < 1.0 {
		f = (val - c) / (1 - c)
	}
	return []float64{at(hsv, 0), c * 100, f * 100}
}

func hcgToRGB(v Value) Value {
	hcg := channels(v)
	h := at(hcg, 0) / 360
	c := at(hcg, 1) / 100
	g := at(hcg, 2) / 100
	if c == 0.0 {
		return []float64{g * 255, g * 255, g * 255}
	}
	hi := math.Mod(h, 1) * 6
	val := math.Mod(hi, 1)
	w := 1 - val
	var pure [3]float64
	switch math.Floor(hi) {
	case 0:
		pure = [3]float64{1, val, 0}
	case 1:
		pure = [3]float64{w, 1, 0}
	case 2:
		pure = [3]float64{0, 1, val}
	case 3:
		pure = [3]float64{0, w, 1}
	case 4:
		pure = [3]float64{val, 0, 1}
	default:
		pure = [3]float64{1, 0, w}
	}
	mg := float64((1.0 - c) * g)
	return []float64{
		(float64(c*pure[0]) + mg) * 255,
		(float64(c*pure[1]) + mg) * 255,
		(float64(c*pure[2]) + mg) * 255,
	}
}

func hcgToHSV(v Value) Value {
	hcg := channels(v)
	c := at(hcg, 1) / 100
	g := at(hcg, 2) / 100
	val := c + float64(g*(1.0-c))
	f := 0.0
	if val > 0.0 {
		f = c / val
	}
	return []float64{at(hcg, 0), f * 100, val * 100}
}

func hcgToHSL(v Value) Value {
	hcg := channels(v)
	c := at(hcg, 1) / 100
	g := at(hcg, 2) / 100
	l := float64(g*(1.0-c)) + float64(0.5*c)
	s := 0.0
	if l > 0.0 && l < 0.5 {
		s = c / (2 * l)
	} else if l >= 0.5 && l < 1.0 {
		s = c / (2 * (1 - l))
	}
	return []float64{at(hcg, 0), s * 100, l * 100}
}

func hcgToHWB(v Value) Value {
	hcg := channels(v)
	c := at(hcg, 1) / 100
	g := at(hcg, 2) / 100
	val := c + float64(g*(1.0-c))
	return []float64{at(hcg, 0), (val - c) * 100, (1 - val) * 100}
}

func hwbToHCG(v Value) Value {
	hwb := channels(v)
	w := at(hwb, 1) / 100
	b := at(hwb, 2) / 100
	val := 1 - b
	c := val - w
	g := 0.0
	if c < 1 {
		g = (val - c) / (1 - c)
	}
	return []float64{at(hwb, 0), c * 100, g * 100}
}

func appleToRGB(v Value) Value {
	apple := channels(v)
	return []float64{(at(apple, 0) / 65535) * 255, (at(apple, 1) / 65535) * 255, (at(apple, 2) / 65535) * 255}
}

func rgbToApple(v Value) Value {
	rgb := channels(v)
	return []float64{(at(rgb, 0) / 255) * 65535, (at(rgb, 1) / 255) * 65535, (at(rgb, 2) / 255) * 65535}
}

func grayToRGB(v Value) Value {
	g := at(channels(v), 0)
	return []float64{g / 100 * 255, g / 100 * 255, g / 100 * 255}
}

func grayToHSL(v Value) Value { return []float64{0, 0, at(channels(v), 0)} }

func grayToHWB(v Value) Value { return []float64{0, 100, at(channels(v), 0)} }

func grayToCMYK(v Value) Value { return []float64{0, 0, 0, at(channels(v), 0)} }

func grayToLab(v Value) Value { return []float64{at(channels(v), 0), 0, 0} }

func grayToHex(v Value) Value {
	val := byte8(at(channels(v), 0) / 100 * 255)
	return fmt.Sprintf("%06X", val<<16+val<<8+val)
}

func rgbToGray(v Value) Value {
	rgb := channels(v)
	val := (at(rgb, 0) + at(rgb, 1) + at(rgb, 2)) / 3
	return []float64{val / 255 * 100}
}
