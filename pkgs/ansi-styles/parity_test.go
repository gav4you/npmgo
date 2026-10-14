package ansistyles

import (
	"encoding/json"
	"math"
	"os"
	"reflect"
	"testing"

	"github.com/gav4you/npmgo/internal/jsvalue"
)

func num(raw json.RawMessage) float64 {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return map[string]float64{"NaN": math.NaN(), "Infinity": math.Inf(1), "-Infinity": math.Inf(-1), "-0": math.Copysign(0, -1)}[s]
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err != nil {
		panic(err)
	}
	return f
}

// call invokes the table's method for key with a case input, through the
// typed methods, and encodes the result like the oracle's attempt.
func call(d Dynamic, key string, raw json.RawMessage) (out string) {
	defer func() {
		if r := recover(); r != nil {
			e, ok := r.(*jsvalue.TypeError)
			if !ok {
				panic(r)
			}
			out = "throws TypeError: " + e.Message
		}
	}()
	var s string
	if key == "hex" || key == "keyword" {
		if err := json.Unmarshal(raw, &s); err != nil {
			panic(err)
		}
		if key == "hex" {
			return jsvalue.JSONQuote(d.Hex(s))
		}
		return jsvalue.JSONQuote(d.Keyword(s))
	}
	var items []json.RawMessage
	if json.Unmarshal(raw, &items) != nil {
		items = []json.RawMessage{raw}
	}
	a := make([]float64, len(items))
	for i, it := range items {
		a[i] = num(it)
	}
	var r string
	switch key {
	case "rgb":
		r = d.RGB(a[0], a[1], a[2])
	case "hsl":
		r = d.HSL(a[0], a[1], a[2])
	case "hsv":
		r = d.HSV(a[0], a[1], a[2])
	case "hwb":
		r = d.HWB(a[0], a[1], a[2])
	case "cmyk":
		r = d.CMYK(a[0], a[1], a[2], a[3])
	case "xyz":
		r = d.XYZ(a[0], a[1], a[2])
	case "lab":
		r = d.Lab(a[0], a[1], a[2])
	case "lch":
		r = d.LCH(a[0], a[1], a[2])
	case "ansi":
		r = d.ANSI(a[0])
	case "ansi256":
		r = d.ANSI256(a[0])
	case "hcg":
		r = d.HCG(a[0], a[1], a[2])
	case "apple":
		r = d.Apple(a[0], a[1], a[2])
	case "gray":
		r = d.Gray(a[0])
	default:
		panic("unknown key " + key)
	}
	return jsvalue.JSONQuote(r)
}

// TestParity diffs the tables against the original ansi-styles@4.3.0 as
// recorded by parity/oracle_chalk.cjs.
func TestParity(t *testing.T) {
	b, err := os.ReadFile("../../parity/chalk/expected.json")
	if err != nil {
		t.Fatal(err)
	}
	var e struct {
		AnsiStyles struct {
			Names   []string
			Styles  map[string][2]string
			Groups  map[string][]string
			Close   map[string]string
			Codes   [][2]int
			Inputs  map[string][]json.RawMessage
			Dynamic map[string]map[string]json.RawMessage
		} `json:"ansiStyles"`
	}
	if err := json.Unmarshal(b, &e); err != nil {
		t.Fatal(err)
	}
	as := e.AnsiStyles
	checks := 0
	if !reflect.DeepEqual(as.Names, Names) {
		t.Errorf("names: -js %v\n+go %v", as.Names, Names)
	}
	for name, s := range as.Styles {
		checks++
		if got := Styles[name]; got != (Style{s[0], s[1]}) {
			t.Errorf("%s: -js %q +go %q", name, s, got)
		}
	}
	tables := []string{"ansi", "ansi256", "ansi16m"}
	for g, group := range map[string]Group{"modifier": Modifier, "color": Color.Group, "bgColor": BgColor.Group} {
		want := group.Names
		if g != "modifier" {
			want = append(append([]string{}, want...), append([]string{"close"}, tables...)...)
		}
		if !reflect.DeepEqual(as.Groups[g], want) {
			t.Errorf("%s keys: -js %v\n+go %v", g, as.Groups[g], want)
		}
		for _, n := range group.Names {
			if group.Styles[n] != Styles[n] {
				t.Errorf("%s.%s: %q, top level %q", g, n, group.Styles[n], Styles[n])
			}
		}
	}
	if as.Close["color"] != Color.Close || as.Close["bgColor"] != BgColor.Close {
		t.Errorf("close: -js %q +go %q %q", as.Close, Color.Close, BgColor.Close)
	}
	if len(as.Codes) != len(Codes) {
		t.Errorf("codes: -js %d entries +go %d", len(as.Codes), len(Codes))
	}
	for _, c := range as.Codes {
		checks++
		if got, ok := Codes[c[0]]; !ok || got != c[1] {
			t.Errorf("codes.get(%d): -js %d +go %d", c[0], c[1], got)
		}
	}

	for name, d := range map[string]Dynamic{
		"color.ansi": Color.ANSI, "color.ansi256": Color.ANSI256, "color.ansi16m": Color.ANSI16M,
		"bgColor.ansi": BgColor.ANSI, "bgColor.ansi256": BgColor.ANSI256, "bgColor.ansi16m": BgColor.ANSI16M,
	} {
		table := as.Dynamic[name]
		var keys []string
		if err := json.Unmarshal(table["keys"], &keys); err != nil {
			t.Fatal(err)
		}
		if len(keys) != 15 {
			t.Errorf("%s: %d keys", name, len(keys))
		}
		for _, key := range keys {
			var want []string
			if err := json.Unmarshal(table[key], &want); err != nil {
				t.Fatal(err)
			}
			for i, in := range as.Inputs[key] {
				checks++
				if got := call(d, key, in); got != want[i] {
					t.Errorf("%s.%s(%s): -js %s +go %s", name, key, in, want[i], got)
				}
			}
		}
	}
	t.Logf("%d checks", checks)
}
