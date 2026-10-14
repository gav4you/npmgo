// Package parityspec builds Go models of JS values from the JSON value specs
// shared with parity/build.cjs.
package parityspec

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"strings"

	"github.com/gav4you/npmgo/internal/jsnum"
	. "github.com/gav4you/npmgo/internal/nodeutil"
)

// CWD replaces "{CWD}" in spec strings (the oracle substitutes process.cwd()).
var CWD string

// Build turns a JSON value spec into the nodeutil value model.
func Build(raw json.RawMessage) any { return build(raw, map[string]any{}) }

func build(raw json.RawMessage, refs map[string]any) any {
	raw = bytes.TrimSpace(raw)
	switch raw[0] {
	case '[':
		var items []json.RawMessage
		json.Unmarshal(raw, &items)
		out := make([]any, len(items))
		for i, it := range items {
			out[i] = build(it, refs)
		}
		return out
	case '{':
	default:
		var v any
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.UseNumber()
		dec.Decode(&v)
		if s, ok := v.(string); ok {
			return strings.ReplaceAll(s, "{CWD}", CWD)
		}
		if n, ok := v.(json.Number); ok {
			f, _ := n.Float64()
			return f
		}
		return v
	}
	var s map[string]json.RawMessage
	json.Unmarshal(raw, &s)
	str := func(k string) string {
		var v string
		json.Unmarshal(s[k], &v)
		return v
	}
	flag := func(k string) bool {
		var v bool
		json.Unmarshal(s[k], &v)
		return v
	}
	proto := func(def string) Proto {
		r, ok := s["ctor"]
		if !ok {
			return Proto{}
		}
		if string(r) == "null" {
			return Proto{Null: true}
		}
		var c string
		json.Unmarshal(r, &c)
		if c == def {
			return Proto{}
		}
		return Proto{Constructor: c}
	}
	props := func() []Prop {
		var list [][]json.RawMessage
		json.Unmarshal(s["props"], &list)
		var out []Prop
		for _, p := range list {
			var key string
			json.Unmarshal(p[0], &key)
			var f struct {
				Sym, Hidden, Getter, Setter bool
				Throws                      string
			}
			if len(p) > 2 {
				json.Unmarshal(p[2], &f)
			}
			var k any = key
			if f.Sym {
				k = Symbol{Description: &key}
			}
			pr := Prop{Key: k, Hidden: f.Hidden, HasSetter: f.Setter}
			if f.Getter || f.Setter {
				pr.IsAccessor = true
				if f.Getter {
					val := p[1]
					if f.Throws != "" {
						msg := f.Throws
						pr.Getter = func() (any, error) { return nil, errors.New(msg) }
					} else {
						pr.Getter = func() (any, error) { return build(val, refs), nil }
					}
				}
			} else {
				pr.Value = build(p[1], refs)
			}
			out = append(out, pr)
		}
		return out
	}
	items := func(k string) []any {
		var list []json.RawMessage
		json.Unmarshal(s[k], &list)
		out := make([]any, len(list))
		for i, it := range list {
			out[i] = build(it, refs)
		}
		return out
	}
	switch str("$") {
	case "undef":
		return Undefined{}
	case "hole":
		return Hole{}
	case "num":
		switch v := str("v"); v {
		case "-0":
			return math.Copysign(0, -1)
		default:
			return jsnum.Number(v)
		}
	case "big":
		b, _ := new(big.Int).SetString(str("v"), 10)
		return b
	case "sym":
		if _, ok := s["d"]; !ok {
			return Symbol{}
		}
		d := str("d")
		return Symbol{Description: &d}
	case "ref":
		return refs[str("id")]
	case "obj":
		o := &Object{Proto: proto("Object"), Tag: str("tag"), Arguments: flag("args")}
		if id := str("id"); id != "" {
			refs[id] = o
		}
		o.Props = props()
		return o
	case "arr":
		a := &Array{Proto: proto("Array")}
		if id := str("id"); id != "" {
			refs[id] = a
		}
		a.Items = items("items")
		a.Props = props()
		return a
	case "map":
		m := &Map{}
		if id := str("id"); id != "" {
			refs[id] = m
		}
		var es [][2]json.RawMessage
		json.Unmarshal(s["entries"], &es)
		for _, e := range es {
			m.Keys = append(m.Keys, build(e[0], refs))
			m.Values = append(m.Values, build(e[1], refs))
		}
		m.Props = props()
		return m
	case "set":
		st := &Set{}
		if id := str("id"); id != "" {
			refs[id] = st
		}
		st.Items = items("items")
		st.Props = props()
		return st
	case "date":
		d := &Date{Ms: math.NaN()}
		if string(s["ms"]) != "null" {
			json.Unmarshal(s["ms"], &d.Ms)
		}
		d.Props = props()
		return d
	case "re":
		r := &RegExp{Source: str("src"), Flags: str("flags"), Props: props()}
		if raw, ok := s["lastIndex"]; ok {
			json.Unmarshal(raw, &r.LastIndex)
		}
		return r
	case "u8":
		var f []float64
		json.Unmarshal(s["items"], &f)
		return &TypedArray{Items: f}
	case "boxed":
		return &Boxed{Value: build(s["v"], refs)}
	case "fn":
		src := "function () {}"
		switch {
		case flag("class") && str("extends") != "":
			src = "class extends Base {}"
		case flag("class"):
			src = "class {}"
		case flag("async") && flag("gen"):
			src = "async function* () {}"
		case flag("async"):
			src = "async function () {}"
		case flag("gen"):
			src = "function* () {}"
		}
		f := &Function{Name: str("name"), Async: flag("async"), Generator: flag("gen"), Source: src,
			Class: flag("class"), Extends: str("extends")}
		if id := str("id"); id != "" {
			refs[id] = f
		}
		f.Props = props()
		return f
	case "err":
		ctor := str("ctor")
		if ctor == "" {
			ctor = "Error"
		}
		// A subclass made by build.cjs (`class Foo extends Error {}`) inherits
		// the name "Error"; only the native error classes name themselves.
		e := &Error{Proto: Proto{Constructor: ctor}, Name: "Error", Message: str("msg")}
		if nativeErrorNames[ctor] {
			e.Name = ctor
		}
		if id := str("id"); id != "" {
			refs[id] = e
		}
		if _, ok := s["name"]; ok {
			e.Name = str("name")
		}
		if raw, ok := s["stack"]; ok {
			// A string is the stack text; any other spec is built (an object,
			// false, 0 or null stack), as in build.cjs.
			if bytes.HasPrefix(bytes.TrimSpace(raw), []byte(`"`)) {
				e.Stack = str("stack")
			} else {
				e.Stack = build(raw, refs)
			}
		}
		if c, ok := s["cause"]; ok {
			e.HasCause, e.Cause = true, build(c, refs)
		}
		e.Props = props()
		if flag("nullProto") {
			e.Proto = Proto{Null: true}
		}
		return e
	case "proxy":
		return &Proxy{Target: build(s["target"], refs), Handler: build(s["handler"], refs), Revoked: flag("revoked")}
	case "promise":
		p := &Promise{}
		switch str("state") {
		case "pending":
			p.State = Pending
		case "rejected":
			p.State, p.Result = Rejected, build(s["v"], refs)
		default:
			p.State, p.Result = Fulfilled, build(s["v"], refs)
		}
		return p
	case "weakset":
		return &WeakSet{Items: items("items")}
	case "weakmap", "mapiter":
		var es [][2]json.RawMessage
		json.Unmarshal(s["entries"], &es)
		var keys, vals, flat []any
		for _, e := range es {
			k, v := build(e[0], refs), build(e[1], refs)
			keys, vals = append(keys, k), append(vals, v)
			flat = append(flat, k, v)
		}
		if str("$") == "weakmap" {
			return &WeakMap{Keys: keys, Values: vals}
		}
		switch str("kind") {
		case "keys":
			return &MapIterator{Items: keys, Props: props()}
		case "values":
			return &MapIterator{Items: vals, Props: props()}
		}
		return &MapIterator{Items: flat, Entries: true, Props: props()}
	case "setiter":
		it := items("items")
		if str("kind") == "entries" {
			var flat []any
			for _, v := range it {
				flat = append(flat, v, v)
			}
			return &SetIterator{Items: flat, Entries: true, Props: props()}
		}
		return &SetIterator{Items: it, Props: props()}
	case "wk":
		return wellKnown[str("name")]
	case "chain":
		o := &Object{}
		if string(s["proto"]) == "null" {
			o.Proto = Proto{Null: true}
		} else {
			p, _ := build(s["proto"], refs).(*Object)
			o.Proto = Proto{Prototype: p}
		}
		if id := str("id"); id != "" {
			refs[id] = o
		}
		o.Props = props()
		return o
	case "ctorfn":
		f := &Function{Name: str("name"), Source: "function () {}"}
		if id := str("protoRef"); id != "" {
			f.PrototypeObject, _ = refs[id].(*Object)
		}
		return f
	case "ab":
		var b []byte
		var ints []int
		json.Unmarshal(s["bytes"], &ints)
		for _, x := range ints {
			b = append(b, byte(x))
		}
		ab := &ArrayBuffer{Bytes: b, Shared: flag("shared"), Detached: flag("detached"), Props: props()}
		if ab.Detached {
			ab.Bytes = nil
		}
		return ab
	case "dv":
		var ints []int
		json.Unmarshal(s["bytes"], &ints)
		b := make([]byte, len(ints))
		for i, x := range ints {
			b[i] = byte(x)
		}
		var off int
		json.Unmarshal(s["offset"], &off)
		length := len(b) - off
		if _, ok := s["length"]; ok {
			json.Unmarshal(s["length"], &length)
		}
		return &DataView{Buffer: &ArrayBuffer{Bytes: b}, ByteOffset: off, ByteLength: length, Props: props()}
	case "typed":
		t := &TypedArray{Kind: str("kind"), Props: props()}
		if flag("big") {
			var raws []json.Number
			dec := json.NewDecoder(bytes.NewReader(s["items"]))
			dec.UseNumber()
			dec.Decode(&raws)
			for _, r := range raws {
				n, _ := new(big.Int).SetString(r.String(), 10)
				t.BigItems = append(t.BigItems, n)
			}
			if t.BigItems == nil {
				t.BigItems = []*big.Int{}
			}
		} else {
			json.Unmarshal(s["items"], &t.Items)
			if t.Items == nil {
				t.Items = []float64{}
			}
		}
		return t
	case "url":
		var parts map[string]string
		json.Unmarshal(s["parts"], &parts)
		u := &URL{Href: parts["href"], Origin: parts["origin"], Protocol: parts["protocol"],
			Username: parts["username"], Password: parts["password"], Host: parts["host"],
			Hostname: parts["hostname"], Port: parts["port"], Pathname: parts["pathname"],
			Search: parts["search"], Hash: parts["hash"], SearchParams: &URLSearchParams{}}
		var params [][2]string
		json.Unmarshal(s["params"], &params)
		for _, p := range params {
			u.SearchParams.Names = append(u.SearchParams.Names, p[0])
			u.SearchParams.Values = append(u.SearchParams.Values, p[1])
		}
		if raw, ok := s["ctx"]; ok {
			var cx struct {
				Href          string
				ProtocolEnd   float64 `json:"protocol_end"`
				UsernameEnd   float64 `json:"username_end"`
				HostStart     float64 `json:"host_start"`
				HostEnd       float64 `json:"host_end"`
				PathnameStart float64 `json:"pathname_start"`
				SearchStart   float64 `json:"search_start"`
				HashStart     float64 `json:"hash_start"`
				Port          float64 `json:"port"`
				SchemeType    float64 `json:"scheme_type"`
			}
			json.Unmarshal(raw, &cx)
			u.Context = &URLContext{Href: cx.Href, ProtocolEnd: cx.ProtocolEnd, UsernameEnd: cx.UsernameEnd,
				HostStart: cx.HostStart, HostEnd: cx.HostEnd, PathnameStart: cx.PathnameStart,
				SearchStart: cx.SearchStart, HashStart: cx.HashStart, Port: cx.Port, SchemeType: cx.SchemeType}
		}
		return u
	case "usp":
		p := &URLSearchParams{}
		var params [][2]string
		json.Unmarshal(s["params"], &params)
		for _, kv := range params {
			p.Names = append(p.Names, kv[0])
			p.Values = append(p.Values, kv[1])
		}
		return p
	case "ns":
		ns := &ModuleNamespace{Uninitialized: map[string]bool{}}
		var exps [][]json.RawMessage
		json.Unmarshal(s["exports"], &exps)
		for _, e := range exps {
			var k string
			json.Unmarshal(e[0], &k)
			var v any = Undefined{}
			if len(e) > 1 {
				v = build(e[1], refs)
			}
			ns.Props = append(ns.Props, Prop{Key: k, Value: v})
		}
		var tdz []string
		json.Unmarshal(s["tdz"], &tdz)
		for _, k := range tdz {
			ns.Uninitialized[k] = true
		}
		return ns
	case "custom":
		ci := &customSpec{mode: str("mode")}
		if r, ok := s["ret"]; ok {
			ci.ret = build(r, refs)
		}
		return ci
	}
	panic("bad spec " + string(raw))
}

// nativeErrorNames are the globalThis error classes build.cjs uses directly.
var nativeErrorNames = map[string]bool{"Error": true, "TypeError": true, "RangeError": true, "SyntaxError": true,
	"ReferenceError": true, "EvalError": true, "URIError": true, "AggregateError": true, "SuppressedError": true}

// customSpec mirrors the [util.inspect.custom] objects made by build.cjs.
type customSpec struct {
	mode  string
	ret   any
	model *Object
}

func (c *customSpec) InspectCustom(depth float64, opts CustomOptions, inspect func(any, Options) string) any {
	switch c.mode {
	case "depth":
		s := "custom@" + jsnum.String(depth)
		if opts.Colors {
			s += ":c"
		}
		return s
	case "self":
		return c
	case "inspect":
		return "<" + inspect(c.ret, opts.Options) + ">"
	}
	return c.ret
}

// JSModel describes the object build.cjs creates: one enumerable own
// Symbol(nodejs.util.inspect.custom) method.
func (c *customSpec) JSModel() any {
	d := InspectCustomDescription
	if c.model == nil {
		c.model = &Object{Props: []Prop{{
			Key:   Symbol{Description: &d},
			Value: &Function{Name: "[" + d + "]"},
		}}}
	}
	return c.model
}

var wellKnown = map[string]*Object{
	"Object": ObjectPrototype, "Array": ArrayPrototype, "ArrayBuffer": ArrayBufferPrototype,
	"Function": FunctionPrototype, "Map": MapPrototype, "Set": SetPrototype, "RegExp": RegExpPrototype,
	"Date": DatePrototype, "DataView": DataViewPrototype, "Error": ErrorPrototype,
	"AggregateError": AggregateErrorPrototype, "RangeError": RangeErrorPrototype,
	"TypeError": TypeErrorPrototype, "Boolean": BooleanPrototype, "Number": NumberPrototype,
	"String": StringPrototype, "Promise": PromisePrototype, "WeakMap": WeakMapPrototype,
	"WeakSet": WeakSetPrototype,
}
