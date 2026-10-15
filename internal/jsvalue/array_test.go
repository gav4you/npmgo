package jsvalue

import (
	"math"
	"testing"
)

func TestArrayReference(t *testing.T) {
	a := NewArray(1.0, "x")
	b := a
	b.Push(true)
	if a.Len() != 3 || a.At(2) != true {
		t.Fatalf("push not shared: %v", a.Items())
	}
	if !IsArray(a) || IsArray((*Array)(nil)) || !IsNull((*Array)(nil)) {
		t.Fatal("IsArray / IsNull")
	}
	if ToString(a) != "1,x,true" {
		t.Fatalf("ToString = %q", ToString(a))
	}
	if a.Unshift("u", "v") != 5 || ToString(a) != "u,v,1,x,true" {
		t.Fatalf("unshift: %q", ToString(a))
	}
	if a.Shift() != "u" || a.Pop() != true || a.Len() != 3 {
		t.Fatalf("shift/pop: %q", ToString(a))
	}
	a.SetAt(5, "z")
	if a.Len() != 6 || !IsUndefined(a.At(4)) || a.At(5) != "z" || !IsUndefined(a.At(9)) {
		t.Fatalf("SetAt: %v", a.Items())
	}
	n := NewArray(math.NaN(), math.Copysign(0, -1))
	if !n.Includes(math.NaN()) || n.IndexOf(math.NaN()) != -1 || !n.Includes(0.0) || n.IndexOf(0.0) != 1 {
		t.Fatal("includes / indexOf")
	}
	if ObjectIs(NewArray(), NewArray()) || !ObjectIs(a, b) {
		t.Fatal("identity")
	}
	if !IsUndefined(NewArray().Pop()) || !IsUndefined(NewArray().Shift()) {
		t.Fatal("empty pop / shift")
	}
}

func TestObjectSymbols(t *testing.T) {
	s1, s2 := NewSymbol("a"), SymbolFor("b")
	o := NewObject("x", 1.0)
	o.SetSymbol(s1, "one")
	o.SetSymbol(s2, "two")
	o.SetSymbol(s1, "uno")
	if got := o.SymbolKeys(); len(got) != 2 || got[0] != s1 || got[1] != s2 {
		t.Fatalf("SymbolKeys = %v", got)
	}
	if o.GetSymbol(s1) != "uno" || !IsUndefined(o.GetSymbol(NewSymbol("a"))) {
		t.Fatal("GetSymbol")
	}
	c := o.Clone()
	if c.GetSymbol(s2) != "two" || len(c.Keys()) != 1 {
		t.Fatal("Clone")
	}
	d := (&Object{}).Assign(o)
	if d.GetSymbol(s1) != "uno" {
		t.Fatal("Assign")
	}
	if !d.DeleteSymbol(s1) || d.DeleteSymbol(s1) || len(d.SymbolKeys()) != 1 {
		t.Fatal("DeleteSymbol")
	}
	if j, _ := JSONStringify(o); j != `{"x":1}` {
		t.Fatalf("JSON = %s", j)
	}
}

// TestPutProto: o.__proto__ = v goes to Object.prototype's accessor (no property) unless o has an own "__proto__"
// property, which is then assigned like any other.
func TestPutProto(t *testing.T) {
	o := NewObject("a", 1.0)
	o.Put("__proto__", NewObject())
	o.Put("b", 2.0)
	if o.Has("__proto__") || len(o.Keys()) != 2 {
		t.Fatalf("accessor: %v", o.Keys())
	}
	own := NewObject("__proto__", 5.0, "c", 3.0)
	own.Put("__proto__", Undefined{})
	if !IsUndefined(own.Get("__proto__")) || own.Keys()[0] != "__proto__" {
		t.Fatalf("own: %v", own.Keys())
	}
}

func TestJSONCircular(t *testing.T) {
	msg := func(v any) (s string) {
		defer func() {
			if r := recover(); r != nil {
				s = r.(*TypeError).Message
			}
		}()
		j, _ := JSONStringify(v)
		return "ok " + j
	}
	// The messages are Node v25.9.0's.
	a := NewObject()
	a.Set("self", a)
	if got, want := msg(a), "Converting circular structure to JSON\n    --> starting at object with constructor 'Object'\n    --- property 'self' closes the circle"; got != want {
		t.Errorf("circ1:\n%s\nwant\n%s", got, want)
	}
	b := NewObject()
	z := NewObject()
	b.Set("x", NewObject("y", NewObject("z", z)))
	z.Set("back", b)
	if got, want := msg(b), "Converting circular structure to JSON\n    --> starting at object with constructor 'Object'\n    |     property 'x' -> object with constructor 'Object'\n    |     property 'y' -> object with constructor 'Object'\n    |     property 'z' -> object with constructor 'Object'\n    --- property 'back' closes the circle"; got != want {
		t.Errorf("circ2:\n%s\nwant\n%s", got, want)
	}
	k := NewArray()
	c := NewArray(1.0, NewObject("k", k))
	k.Push(c)
	if got, want := msg(c), "Converting circular structure to JSON\n    --> starting at object with constructor 'Array'\n    |     index 1 -> object with constructor 'Object'\n    |     property 'k' -> object with constructor 'Array'\n    --- index 0 closes the circle"; got != want {
		t.Errorf("circ3:\n%s\nwant\n%s", got, want)
	}
	tt := NewObject()
	p := NewObject("q", NewObject("r", NewObject("s", NewObject("t", tt))))
	d := NewObject("p", p)
	tt.Set("u", p)
	if got, want := msg(d), "Converting circular structure to JSON\n    --> starting at object with constructor 'Object'\n    |     property 'q' -> object with constructor 'Object'\n    |     property 'r' -> object with constructor 'Object'\n    |     ...\n    |     property 't' -> object with constructor 'Object'\n    --- property 'u' closes the circle"; got != want {
		t.Errorf("circ4:\n%s\nwant\n%s", got, want)
	}
	g := NewObject()
	inner := NewObject()
	g.Set("a b", inner)
	inner.Set("c", g)
	if got, want := msg(g), "Converting circular structure to JSON\n    --> starting at object with constructor 'Object'\n    |     property 'a b' -> object with constructor 'Object'\n    --- property 'c' closes the circle"; got != want {
		t.Errorf("circ7:\n%s\nwant\n%s", got, want)
	}
	shared := NewObject("v", 1.0)
	if got := msg(NewArray(shared, shared, NewObject("w", shared))); got != `ok [{"v":1},{"v":1},{"w":{"v":1}}]` {
		t.Errorf("shared, not circular: %s", got)
	}
}
