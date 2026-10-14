package colorconvert

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/gav4you/npmgo/internal/jsnum"
	"github.com/gav4you/npmgo/internal/jsvalue"
)

func load(t *testing.T, path string, v any) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatal(path, err)
	}
}

// num decodes a case number: a JSON number or "NaN", "Infinity",
// "-Infinity", "-0".
func num(raw json.RawMessage) float64 {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		switch s {
		case "NaN":
			return math.NaN()
		case "Infinity":
			return math.Inf(1)
		case "-Infinity":
			return math.Inf(-1)
		case "-0":
			return math.Copysign(0, -1)
		}
		panic("bad number " + s)
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err != nil {
		panic(err)
	}
	return f
}

// caseValue decodes a case input the way the oracle's call() reads it:
// strings stay strings for hex and keyword, arrays become channel lists.
func caseValue(model string, raw json.RawMessage) Value {
	var items []json.RawMessage
	if json.Unmarshal(raw, &items) == nil {
		a := make([]float64, len(items))
		for i, it := range items {
			a[i] = num(it)
		}
		return a
	}
	var s string
	if (model == "hex" || model == "keyword") && json.Unmarshal(raw, &s) == nil {
		return s
	}
	return num(raw)
}

func encNum(x float64) string {
	if x == 0 && math.Signbit(x) {
		return "-0"
	}
	return jsnum.String(x)
}

// enc is the oracle's enc for conversion results.
func enc(v Value) string {
	switch x := v.(type) {
	case nil:
		return "undefined"
	case float64:
		return encNum(x)
	case string:
		return jsvalue.JSONQuote(x)
	case []float64:
		parts := make([]string, len(x))
		for i, f := range x {
			parts[i] = encNum(f)
		}
		return "[" + strings.Join(parts, ",") + "]"
	}
	return fmt.Sprintf("unexpected %T", v)
}

// attempt is the oracle's attempt: the encoded result, or "throws …".
func attempt(f func() Value) (s string) {
	defer func() {
		if r := recover(); r != nil {
			e, ok := r.(*jsvalue.TypeError)
			if !ok {
				panic(r)
			}
			s = "throws TypeError: " + e.Message
		}
	}()
	return enc(f())
}

// powRoute reports whether a route computes **: rgb.xyz, rgb.lab, xyz.rgb,
// xyz.lab and lab.xyz do. V8 computes ** with the C library's pow, so the
// last bits of these results depend on the platform Node runs on (and
// math.Pow has its own); they are compared within powTolerance.
func powRoute(path []string) bool {
	for i := 0; i+1 < len(path); i++ {
		switch path[i] + "." + path[i+1] {
		case "rgb.xyz", "rgb.lab", "xyz.rgb", "xyz.lab", "lab.xyz":
			return true
		}
	}
	return false
}

const powTolerance = 1e-9 // relative to max(1, |js|)

// decodeArray parses an encoded channel array; ok is false for other results.
func decodeArray(s string) (a []float64, ok bool) {
	if !strings.HasPrefix(s, "[") {
		return nil, false
	}
	for _, p := range strings.Split(strings.Trim(s, "[]"), ",") {
		var f float64
		switch p {
		case "NaN", "undefined":
			f = math.NaN()
		case "Infinity":
			f = math.Inf(1)
		case "-Infinity":
			f = math.Inf(-1)
		default:
			var err error
			if f, err = strconv.ParseFloat(p, 64); err != nil {
				return nil, false
			}
		}
		a = append(a, f)
	}
	return a, true
}

func near(g, j float64) bool {
	if math.IsNaN(g) || math.IsNaN(j) || math.IsInf(j, 0) {
		return math.IsNaN(g) == math.IsNaN(j) && (math.IsNaN(g) || g == j)
	}
	return math.Abs(g-j) <= powTolerance*max(1, math.Abs(j))
}

// hueNoise reports whether channel i of a to-model result is a hue that is
// meaningless because the colour is achromatic: its chroma or saturation is
// rounding noise, so the hue is the angle of that noise and differs with
// every last-bit difference of pow.
func hueNoise(to string, i int, jsRaw []float64) bool {
	const zero = 1e-6
	switch to {
	case "lch":
		return i == 2 && math.Abs(jsRaw[1]) < zero
	case "hsl", "hsv", "hcg":
		return i == 0 && math.Abs(jsRaw[1]) < zero
	case "hwb":
		return i == 0 && math.Abs(100-jsRaw[1]-jsRaw[2]) < zero
	}
	return false
}

// powStats summarises how the ** routes differed.
type powStats struct {
	maxDev               float64 // largest |go - js| / max(1, |js|) of a raw channel
	noiseHues, straddles int
}

// powMatch compares the results of a ** route: raw channels within
// powTolerance, rounded channels exactly unless the raw values straddle a
// rounding boundary, skipping noise hues.
func powMatch(st *powStats, to string, jsRounded, goRounded, jsRawS, goRawS string) (roundedOK, rawOK bool) {
	jr, ok1 := decodeArray(jsRawS)
	gr, ok2 := decodeArray(goRawS)
	jo, ok3 := decodeArray(jsRounded)
	gO, ok4 := decodeArray(goRounded)
	if !(ok1 && ok2 && ok3 && ok4) || len(jr) != len(gr) || len(jo) != len(gO) || len(jr) != len(jo) {
		return jsRounded == goRounded, jsRawS == goRawS
	}
	roundedOK, rawOK = true, true
	for i := range jr {
		if hueNoise(to, i, jr) {
			if gr[i] != jr[i] {
				st.noiseHues++
			}
			continue
		}
		if !near(gr[i], jr[i]) {
			rawOK = false
		} else if d := math.Abs(gr[i]-jr[i]) / max(1, math.Abs(jr[i])); d > st.maxDev {
			st.maxDev = d
		}
		if gO[i] == jo[i] || (math.IsNaN(gO[i]) && math.IsNaN(jo[i])) {
			continue // equal, or -0 and 0
		}
		if near(gr[i], jr[i]) && math.Floor(gr[i]+0.5) != math.Floor(jr[i]+0.5) {
			st.straddles++
			continue // a rounding boundary lies between two values a few ulps apart
		}
		roundedOK = false
	}
	return roundedOK, rawOK
}

// TestParity diffs color-convert against the original color-convert@2.0.1 as
// recorded by parity/oracle_chalk.cjs: the models, their channels and labels,
// the route of every pair and every conversion of every case input, rounded
// (Call) and raw (Raw).
func TestParity(t *testing.T) {
	var c struct {
		Convert map[string][]json.RawMessage `json:"convert"`
	}
	var e struct {
		ColorConvert struct {
			Models   []string                       `json:"models"`
			Channels map[string]int                 `json:"channels"`
			Labels   map[string]json.RawMessage     `json:"labels"`
			Paths    map[string]map[string][]string `json:"paths"`
			Sweep    map[string][]map[string][2]string
		} `json:"colorConvert"`
	}
	load(t, "../../parity/chalk/cases.json", &c)
	load(t, "../../parity/chalk/expected.json", &e)
	cc := e.ColorConvert

	if !reflect.DeepEqual(cc.Models, Models) {
		t.Errorf("models: -js %v +go %v", cc.Models, Models)
	}
	if !reflect.DeepEqual(cc.Channels, Channels) {
		t.Errorf("channels: -js %v +go %v", cc.Channels, Channels)
	}
	for m, raw := range cc.Labels {
		var labels []string
		var s string
		if json.Unmarshal(raw, &s) == nil {
			labels = strings.Split(s, "") // 'rgb': one label per character
		} else if err := json.Unmarshal(raw, &labels); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(labels, Labels[m]) {
			t.Errorf("labels %s: -js %v +go %v", m, labels, Labels[m])
		}
	}
	routes := 0
	for from, tos := range cc.Paths {
		if len(tos) != len(Convert[from]) {
			t.Errorf("%s: -js %d routes +go %d", from, len(tos), len(Convert[from]))
		}
		for to, path := range tos {
			routes++
			if got := Convert[from][to]; got == nil || !reflect.DeepEqual(got.Path, path) {
				t.Errorf("convert.%s.%s.conversion: -js %v +go %v", from, to, path, got)
			}
		}
	}

	checks, exact, tolerated := 0, 0, 0
	var st powStats
	for _, from := range Models {
		inputs := c.Convert[from]
		if len(inputs) != len(cc.Sweep[from]) {
			t.Fatalf("%s: %d inputs, %d recorded", from, len(inputs), len(cc.Sweep[from]))
		}
		for i, raw := range inputs {
			for to, want := range cc.Sweep[from][i] {
				conv := Convert[from][to]
				in := func() Value { return caseValue(from, raw) }
				rounded := attempt(func() Value { return conv.Call(in()) })
				rawOut := attempt(func() Value { return conv.Raw(in()) })
				checks += 2
				roundedOK, rawOK := rounded == want[0], rawOut == want[1]
				if powRoute(conv.Path) && !(roundedOK && rawOK) {
					roundedOK, rawOK = powMatch(&st, to, want[0], rounded, want[1], rawOut)
					if roundedOK && rawOK {
						tolerated++
					}
				} else if roundedOK && rawOK {
					exact++
				}
				if !roundedOK {
					t.Errorf("convert.%s.%s(%s):\n--- js  %s\n+++ go  %s", from, to, raw, want[0], rounded)
				}
				if !rawOK {
					t.Errorf("convert.%s.%s.raw(%s):\n--- js  %s\n+++ go  %s", from, to, raw, want[1], rawOut)
				}
			}
		}
	}
	t.Logf("%d routes; %d conversion checks: %d input/route pairs exact, %d on ** routes within %g (largest deviation %.3g, %d noise hues skipped, %d rounding boundaries straddled)",
		routes, checks, exact, tolerated, powTolerance, st.maxDev, st.noiseHues, st.straddles)
}
