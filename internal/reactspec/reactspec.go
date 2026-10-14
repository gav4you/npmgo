// Package reactspec builds React element trees in Go from the JSON case
// language shared with parity/oracle_react.cjs, which builds the same trees
// with the original React. Each construct below is implemented once per
// language.
//
// A case is
//
//	{"name": ..., "modes": ["string","static","stream"],
//	 "ctxs": {"Name": {"default": expr, "displayName": "..."}},
//	 "comps": {"Name": component},
//	 "promises": [{"n": "p1", "kind": kind, "value": expr, "step": n, "reject": bool}],
//	 "tree": expr, "opts": {"identifierPrefix": "x"}, "twice": bool,
//	 "stream": {"pipe": "now"|"shell"|"all", "opts": {...}, "digest": "msg", "abort": {"step": n, "reason": expr}}}
//
// Promise kinds: pending (settled at step), resolved / rejected
// (Promise.resolve / reject: untracked), fulfilled / rejectedTagged (status
// preset, read synchronously), sync / syncReject (then calls back at once).
//
// Components: {"kind": "fn", "name", "displayName", "defaultProps", "body":
// [stmt], "ret": expr} (scope: props), {"kind": "class", "name", "state",
// "contextType", "defaultProps", "gdsfp", "cwm", "ucwm", "snapshot", "pure",
// "body", "ret"} (scope: props, state, context, self), {"kind": "forwardRef",
// "name", "body", "ret"} (scope: props, ref), {"kind": "memo", "of": type},
// {"kind": "lazy", "p": promise, "throws": expr}.
//
// Statements: {"let": name | [names], "v": expr}, {"do": expr},
// {"if": expr, "then": [stmt], "else": [stmt]}, {"throw": expr},
// {"return": expr}, {"try": [stmt], "catch": name, "handle": [stmt]}.
//
// Expressions: JSON literals and arrays; {"$": "undef"}, {"$": "num", "v"},
// {"$": "big", "v"}, {"$": "sym", "d"}, {"$": "symfor", "k"},
// {"$": "obj", "p": [[k, expr]]}, {"$": "el", "t": type, "p": expr, "c":
// [expr]} (createElement), {"$": "type", "t": type}, {"$": "var", "n"},
// {"$": "prop", "n"}, {"$": "get", "o", "k"}, {"$": "cat", "a"} ("" + ...),
// {"$": "add", "a", "b"}, {"$": "json", "v"}, {"$": "str", "v"},
// {"$": "typeof", "v"}, {"$": "eq"|"lt", "a", "b"}, {"$": "not", "v"},
// {"$": "if", "c", "t", "f"}, {"$": "fn", "ps", "body", "ret"},
// {"$": "thunk", "v"}, {"$": "call", "f", "a"}, {"$": "hook", "h", "a"},
// {"$": "api", "f", "a"}, {"$": "promise", "n"}, {"$": "ctxv", "n"},
// {"$": "compv", "n"}, {"$": "error", "m"}, {"$": "typeerror", "m"},
// {"$": "gget", "n"}, {"$": "gset", "n", "v"}, {"$": "iter", "v"},
// {"$": "setState", "v"}, {"$": "action", "fn", "formAction", "sigEqual"}.
//
// Types: a tag string, {"$": "frag"|"suspense"|"strict"|"profiler"|
// "activity"|"suspenselist"|"viewtransition"|"scope"|"legacyhidden"},
// {"$": "comp"|"ctx"|"provider"|"consumer", "n"}, or any expression.
package reactspec

import (
	"encoding/json"
	"fmt"
	"iter"
	"math"
	"math/big"
	"strings"

	"github.com/gav4you/npmgo/internal/jsnum"
	"github.com/gav4you/npmgo/internal/jsvalue"
	"github.com/gav4you/npmgo/pkgs/react"
	reactdom "github.com/gav4you/npmgo/pkgs/react-dom"
)

// Case is one parity case.
type Case struct {
	Name     string                     `json:"name"`
	Modes    []string                   `json:"modes"`
	Ctxs     map[string]map[string]any `json:"ctxs"`
	Comps    map[string]map[string]any `json:"comps"`
	Promises []map[string]any           `json:"promises"`
	Tree     any                        `json:"tree"`
	Opts     *struct {
		IdentifierPrefix string `json:"identifierPrefix"`
	} `json:"opts"`
	Twice  bool        `json:"twice"`
	Stream *StreamSpec `json:"stream"`
}

// StreamSpec describes how a case is streamed.
type StreamSpec struct {
	Pipe   string         `json:"pipe"`
	Opts   map[string]any `json:"opts"`
	Digest string         `json:"digest"`
	Abort  *struct {
		Step   float64 `json:"step"`
		Reason any     `json:"reason"`
	} `json:"abort"`
}

// HasModes reports whether the case runs in mode.
func (c *Case) HasMode(mode string) bool {
	if len(c.Modes) == 0 {
		return true
	}
	for _, m := range c.Modes {
		if m == mode {
			return true
		}
	}
	return false
}

type resolver struct {
	step int
	run  func()
}

// Env is one fresh instance of a case: its components, contexts, promises.
type Env struct {
	comps     map[string]any
	ctxs      map[string]*react.Context
	promises  map[string]react.Thenable
	resolvers []resolver
	globals   map[string]any
}

type scope map[string]any

const hooksKey = "$hooks"

// Setup instantiates a case.
func Setup(c *Case) *Env {
	env := &Env{comps: map[string]any{}, ctxs: map[string]*react.Context{}, promises: map[string]react.Thenable{}, globals: map[string]any{}}
	for name, def := range c.Ctxs {
		d, ok := def["default"]
		var v any = react.Undefined{}
		if ok {
			v = env.Eval(d, scope{})
		}
		ctx := react.CreateContext(v)
		if dn, ok := def["displayName"].(string); ok {
			ctx.DisplayName = dn
		}
		env.ctxs[name] = ctx
	}
	// memo wraps another component, so memos are created after everything else.
	for name, def := range c.Comps {
		if def["kind"] != "memo" {
			env.comps[name] = env.makeComp(def)
		}
	}
	for name, def := range c.Comps {
		if def["kind"] == "memo" {
			env.comps[name] = env.makeComp(def)
		}
	}
	for _, def := range c.Promises {
		env.addPromise(def)
	}
	return env
}

// Resolve settles the promises scheduled for step.
func (env *Env) Resolve(step int) {
	for _, r := range env.resolvers {
		if r.step == step {
			r.run()
		}
	}
}

// MaxStep is the last step a promise is scheduled for.
func (env *Env) MaxStep() int {
	m := 0
	for _, r := range env.resolvers {
		if r.step > m {
			m = r.step
		}
	}
	return m
}

type syncThenable struct {
	react.ThenableState
	value  func() any
	reject bool
}

func (s *syncThenable) Then(onFulfilled, onRejected func(any)) {
	if s.reject {
		if onRejected != nil {
			onRejected(s.value())
		}
		return
	}
	if onFulfilled != nil {
		onFulfilled(s.value())
	}
}

func (env *Env) addPromise(def map[string]any) {
	name, _ := def["n"].(string)
	valueExpr, hasValue := def["value"]
	value := func() any {
		if !hasValue {
			return react.Undefined{}
		}
		return env.Eval(valueExpr, scope{})
	}
	var t react.Thenable
	switch def["kind"] {
	case "pending":
		p, resolve, reject := react.NewPromise()
		if step, ok := def["step"].(float64); ok {
			rej, _ := def["reject"].(bool)
			env.resolvers = append(env.resolvers, resolver{int(step), func() {
				if rej {
					reject(value())
				} else {
					resolve(value())
				}
			}})
		}
		t = p
	case "resolved":
		t = react.ResolvedPromise(value())
	case "rejected":
		t = react.RejectedPromise(value())
	case "fulfilled":
		v := value()
		p := react.ResolvedPromise(v)
		p.SetThenableStatus("fulfilled", v, nil)
		t = p
	case "rejectedTagged":
		v := value()
		p := react.RejectedPromise(v)
		p.SetThenableStatus("rejected", nil, v)
		t = p
	case "sync":
		t = &syncThenable{value: value}
	case "syncReject":
		t = &syncThenable{value: value, reject: true}
	default:
		panic(fmt.Sprintf("bad promise kind %v", def["kind"]))
	}
	env.promises[name] = t
}

func str(v any) string { s, _ := v.(string); return s }

func stmts(v any) []any { a, _ := v.([]any); return a }

// dslClass is a class component instance built from a case.
type dslClass struct {
	react.Component
	env *Env
	def map[string]any
}

func (c *dslClass) scope() scope {
	return scope{"props": c.Props, "state": c.State, "context": c.Context, "self": c}
}

func (c *dslClass) Render() react.Node { return c.env.runBody(c.def, c.scope()) }

type clsCWM struct{ *dslClass }

func (c clsCWM) ComponentWillMount() { c.env.runStmts(stmts(c.def["cwm"]), c.scope()) }

type clsUCWM struct{ *dslClass }

func (c clsUCWM) UNSAFE_componentWillMount() { c.env.runStmts(stmts(c.def["ucwm"]), c.scope()) }

type clsBoth struct{ *dslClass }

func (c clsBoth) ComponentWillMount()        { c.env.runStmts(stmts(c.def["cwm"]), c.scope()) }
func (c clsBoth) UNSAFE_componentWillMount() { c.env.runStmts(stmts(c.def["ucwm"]), c.scope()) }

type clsSnapCWM struct{ *dslClass }

func (c clsSnapCWM) ComponentWillMount() { c.env.runStmts(stmts(c.def["cwm"]), c.scope()) }
func (c clsSnapCWM) GetSnapshotBeforeUpdate(*react.Props, any) any {
	return nil
}

// blob stands for a JavaScript Blob (a non-string FormData value).
type blob struct{}

type iterable struct{ items []any }

func (it *iterable) Iterate() iter.Seq[any] {
	return func(yield func(any) bool) {
		for _, v := range it.items {
			if !yield(v) {
				return
			}
		}
	}
}

func (env *Env) makeComp(def map[string]any) any {
	name := str(def["name"])
	displayName := str(def["displayName"])
	var defaultProps *react.Object
	if dp, ok := def["defaultProps"]; ok {
		defaultProps, _ = env.Eval(dp, scope{}).(*react.Object)
	}
	switch def["kind"] {
	case "fn":
		return &react.FunctionComponent{Name: name, DisplayName: displayName, DefaultProps: defaultProps,
			Render: func(h react.Hooks, props *react.Props) react.Node {
				return env.runBody(def, scope{"props": props, hooksKey: h})
			}}
	case "class":
		cls := &react.Class{Name: name, DisplayName: displayName, DefaultProps: defaultProps, Pure: def["pure"] == true}
		if ct, ok := def["contextType"].(string); ok {
			cls.ContextType = env.ctxs[ct]
		}
		if g, ok := def["gdsfp"]; ok {
			fn := env.Eval(g, scope{})
			cls.GetDerivedStateFromProps = func(props *react.Props, state any) any {
				return call(fn, props, state)
			}
		}
		_, cwm := def["cwm"]
		_, ucwm := def["ucwm"]
		snap := def["snapshot"] == true
		cls.New = func(props *react.Props, context any) react.ComponentInstance {
			base := &dslClass{Component: react.NewComponent(props, context), env: env, def: def}
			if st, ok := def["state"]; ok {
				base.State = env.Eval(st, scope{"props": props, "context": context, "self": base})
			}
			switch {
			case snap && cwm:
				return clsSnapCWM{base}
			case cwm && ucwm:
				return clsBoth{base}
			case cwm:
				return clsCWM{base}
			case ucwm:
				return clsUCWM{base}
			}
			return base
		}
		return cls
	case "forwardRef":
		f := react.ForwardRef(func(h react.Hooks, props *react.Props, ref any) react.Node {
			return env.runBody(def, scope{"props": props, "ref": ref, hooksKey: h})
		})
		f.Name, f.DisplayName, f.DefaultProps = name, displayName, defaultProps
		return f
	case "memo":
		var compare func(a, b *react.Props) bool
		if c, ok := def["compare"]; ok {
			fn := env.Eval(c, scope{})
			compare = func(a, b *react.Props) bool { return jsvalue.Truthy(call(fn, a, b)) }
		}
		m := react.Memo(env.evalType(def["of"]), compare)
		m.DisplayName, m.DefaultProps = displayName, defaultProps
		return m
	case "lazy":
		p := str(def["p"])
		throws, hasThrows := def["throws"]
		l := react.Lazy(func() react.Thenable {
			if hasThrows {
				panic(react.Throw(env.Eval(throws, scope{})))
			}
			return env.promises[p]
		})
		l.DisplayName = displayName
		return l
	}
	panic(fmt.Sprintf("bad component kind %v", def["kind"]))
}

type returned struct{ value any }

func (env *Env) runBody(def map[string]any, s scope) any {
	if r := env.runStmts(stmts(def["body"]), s); r != nil {
		return r.value
	}
	ret, ok := def["ret"]
	if !ok {
		return nil
	}
	return env.Eval(ret, s)
}

func (env *Env) runStmts(list []any, s scope) *returned {
	for _, raw := range list {
		st := raw.(map[string]any)
		if l, ok := st["let"]; ok {
			v := env.Eval(st["v"], s)
			if names, ok := l.([]any); ok {
				items := jsvalue.Items(v)
				for i, n := range names {
					if i < len(items) {
						s[n.(string)] = items[i]
					} else {
						s[n.(string)] = react.Undefined{}
					}
				}
			} else {
				s[l.(string)] = v
			}
		} else if d, ok := st["do"]; ok {
			env.Eval(d, s)
		} else if c, ok := st["if"]; ok {
			branch := st["else"]
			if jsvalue.Truthy(env.Eval(c, s)) {
				branch = st["then"]
			}
			if r := env.runStmts(stmts(branch), s); r != nil {
				return r
			}
		} else if t, ok := st["throw"]; ok {
			panic(react.Throw(env.Eval(t, s)))
		} else if r, ok := st["return"]; ok {
			return &returned{env.Eval(r, s)}
		} else if body, ok := st["try"]; ok {
			var thrown any
			func() {
				defer func() {
					if p := recover(); p != nil {
						thrown = react.Thrown(p)
						s[str(st["catch"])] = thrown
						env.runStmts(stmts(st["handle"]), s)
					}
				}()
				env.runStmts(stmts(body), s)
			}()
		} else {
			panic(fmt.Sprintf("bad statement %v", st))
		}
	}
	return nil
}

func (env *Env) evalType(t any) any {
	if s, ok := t.(string); ok {
		return s
	}
	if m, ok := t.(map[string]any); ok {
		switch m["$"] {
		case "frag":
			return react.Fragment
		case "suspense":
			return react.Suspense
		case "strict":
			return react.StrictMode
		case "profiler":
			return react.Profiler
		case "activity":
			return react.Activity
		case "suspenselist":
			return react.SuspenseList
		case "viewtransition":
			return react.ViewTransition
		case "scope":
			return react.Scope
		case "legacyhidden":
			return react.LegacyHidden
		case "comp":
			return env.comps[str(m["n"])]
		case "ctx", "provider":
			return env.ctxs[str(m["n"])].Provider
		case "consumer":
			return env.ctxs[str(m["n"])].Consumer
		}
	}
	return env.Eval(t, scope{})
}

// call calls a function value the way JavaScript does.
func call(fn any, args ...any) any {
	v, ok := jsvalue.Call(fn, args...)
	if !ok {
		panic(react.Throw(&jsvalue.TypeError{Message: jsvalue.TypeOf(fn) + " is not a function"}))
	}
	return v
}

func get(o, k any) any {
	if jsvalue.IsNullish(o) {
		what := "null"
		if jsvalue.IsUndefined(o) {
			what = "undefined"
		}
		panic(react.Throw(&jsvalue.TypeError{Message: "Cannot read properties of " + what + " (reading '" + jsvalue.String(k) + "')"}))
	}
	key := jsvalue.ToString(k)
	switch x := o.(type) {
	case *react.Object:
		return x.Get(key)
	case *react.Element:
		switch key {
		case "type":
			return x.Type
		case "key":
			if x.Key == nil {
				return nil
			}
			return *x.Key
		case "props":
			return x.Props
		case "ref":
			return x.Ref
		}
	case *react.Ref:
		if key == "current" {
			return x.Current
		}
	case *react.FormStatus:
		switch key {
		case "pending":
			return x.Pending
		case "data":
			return x.Data
		case "method":
			return x.Method
		case "action":
			return x.Action
		}
	case *react.Context:
		switch key {
		case "Provider":
			return x.Provider
		case "Consumer":
			return x.Consumer
		case "displayName":
			if x.DisplayName == "" {
				return react.Undefined{}
			}
			return x.DisplayName
		}
	case string:
		if key == "length" {
			return float64(len([]rune(x)))
		}
	}
	if jsvalue.IsArray(o) {
		items := jsvalue.Items(o)
		if key == "length" {
			return float64(len(items))
		}
		n := jsvalue.StringToNumber(key)
		if i := int(n); float64(i) == n && i >= 0 && i < len(items) {
			return items[i]
		}
	}
	return react.Undefined{}
}

func add(a, b any) any {
	if ab, ok := a.(*big.Int); ok {
		if bb, ok := b.(*big.Int); ok {
			return new(big.Int).Add(ab, bb)
		}
	}
	_, as := a.(string)
	_, bs := b.(string)
	if as || bs || (jsvalue.TypeOf(a) == "object" && !jsvalue.IsNull(a)) || (jsvalue.TypeOf(b) == "object" && !jsvalue.IsNull(b)) {
		return jsvalue.ToString(a) + jsvalue.ToString(b)
	}
	return jsvalue.ToNumber(a) + jsvalue.ToNumber(b)
}

// Eval evaluates an expression with no variables in scope.
func (env *Env) EvalTop(e any) any { return env.Eval(e, scope{}) }

// Eval evaluates an expression.
func (env *Env) Eval(e any, s scope) any {
	switch x := e.(type) {
	case nil, bool, string, float64:
		return x
	case []any:
		out := make([]any, len(x))
		for i, it := range x {
			out[i] = env.Eval(it, s)
		}
		return out
	case map[string]any:
		return env.evalObj(x, s)
	}
	panic(fmt.Sprintf("bad expr %v", e))
}

func (env *Env) args(m map[string]any, s scope) []any {
	list, _ := m["a"].([]any)
	out := make([]any, len(list))
	for i, a := range list {
		out[i] = env.Eval(a, s)
	}
	return out
}

func (env *Env) evalObj(m map[string]any, s scope) any {
	E := func(k string) any {
		v, ok := m[k]
		if !ok {
			return react.Undefined{}
		}
		return env.Eval(v, s)
	}
	switch m["$"] {
	case "undef":
		return react.Undefined{}
	case "num":
		switch v := str(m["v"]); v {
		case "-0":
			return math.Copysign(0, -1)
		default:
			return jsnum.Number(v)
		}
	case "big":
		b, _ := new(big.Int).SetString(str(m["v"]), 10)
		return b
	case "sym":
		return react.NewSymbol(str(m["d"]))
	case "symfor":
		return react.SymbolFor(str(m["k"]))
	case "obj":
		o := &react.Object{}
		pairs, _ := m["p"].([]any)
		for _, p := range pairs {
			kv := p.([]any)
			o.Set(jsvalue.ToString(kv[0]), env.Eval(kv[1], s))
		}
		return o
	case "el":
		t := env.evalType(m["t"])
		var props *react.Object
		if p, ok := m["p"]; ok && p != nil {
			v := env.Eval(p, s)
			props, _ = v.(*react.Object)
		}
		var children []any
		if c, ok := m["c"].([]any); ok {
			for _, ch := range c {
				children = append(children, env.Eval(ch, s))
			}
		}
		return react.CreateElement(t, props, children...)
	case "type":
		return env.evalType(m["t"])
	case "var":
		n := str(m["n"])
		v, ok := s[n]
		if !ok {
			panic("unbound " + n)
		}
		return v
	case "prop":
		props, _ := s["props"].(*react.Object)
		return props.Get(str(m["n"]))
	case "get":
		return get(E("o"), E("k"))
	case "cat":
		var b strings.Builder
		for _, a := range m["a"].([]any) {
			b.WriteString(jsvalue.ToString(env.Eval(a, s)))
		}
		return b.String()
	case "add":
		return add(E("a"), E("b"))
	case "json":
		j, ok := jsvalue.JSONStringify(E("v"))
		if !ok {
			return "undefined"
		}
		return j
	case "str":
		return jsvalue.String(E("v"))
	case "typeof":
		return jsvalue.TypeOf(E("v"))
	case "eq":
		return jsvalue.StrictEquals(E("a"), E("b"))
	case "lt":
		return jsvalue.ToNumber(E("a")) < jsvalue.ToNumber(E("b"))
	case "not":
		return !jsvalue.Truthy(E("v"))
	case "if":
		if jsvalue.Truthy(E("c")) {
			return E("t")
		}
		return E("f")
	case "fn":
		ps, _ := m["ps"].([]any)
		return func(args ...any) any {
			inner := scope{}
			for k, v := range s {
				inner[k] = v
			}
			for i, p := range ps {
				if i < len(args) {
					inner[p.(string)] = args[i]
				} else {
					inner[p.(string)] = react.Undefined{}
				}
			}
			if r := env.runStmts(stmts(m["body"]), inner); r != nil {
				return r.value
			}
			ret, ok := m["ret"]
			if !ok {
				return react.Undefined{}
			}
			return env.Eval(ret, inner)
		}
	case "thunk":
		v := m["v"]
		return func() any { return env.Eval(v, s) }
	case "call":
		return call(E("f"), env.args(m, s)...)
	case "hook":
		h, _ := s[hooksKey].(react.Hooks)
		if h == nil {
			panic(react.Throw(&jsvalue.TypeError{Message: "Cannot read properties of null (reading '" + str(m["h"]) + "')"}))
		}
		return callHook(h, str(m["h"]), env.args(m, s))
	case "api":
		h, _ := s[hooksKey].(react.Hooks)
		return callAPI(h, str(m["f"]), env.args(m, s))
	case "promise":
		return env.promises[str(m["n"])]
	case "ctxv":
		return env.ctxs[str(m["n"])]
	case "compv":
		return env.comps[str(m["n"])]
	case "error":
		return &react.Error{Message: str(m["m"])}
	case "typeerror":
		return &jsvalue.TypeError{Message: str(m["m"])}
	case "blob":
		return blob{}
	case "gget":
		if v, ok := env.globals[str(m["n"])]; ok {
			return v
		}
		return react.Undefined{}
	case "gset":
		v := E("v")
		env.globals[str(m["n"])] = v
		return v
	case "iter":
		items := env.Eval(m["v"], s).([]any)
		return &iterable{items}
	case "setState":
		self := s["self"].(interface{ ReactComponent() *react.Component })
		self.ReactComponent().SetState(E("v"))
		return react.Undefined{}
	case "action":
		fn := E("fn")
		a := &react.Action{Fn: func(args ...any) any { return call(fn, args...) }}
		if _, ok := m["formAction"]; ok {
			fa := E("formAction")
			a.FormAction = func(prefix string) *react.FormActionFields {
				r := call(fa, prefix)
				o, ok := r.(*react.Object)
				if !ok || o == nil {
					return nil
				}
				f := &react.FormActionFields{Name: o.Get("name"), Action: o.Get("action"), EncType: o.Get("encType"), Method: o.Get("method"), Target: o.Get("target")}
				if d, ok := o.Lookup("data"); ok && !jsvalue.IsUndefined(d) {
					f.Data = []react.FormField{}
					for _, kv := range jsvalue.Items(d) {
						// FormData.append stores strings, except for Blobs.
						pair := jsvalue.Items(kv)
						val := pair[1]
						if _, isBlob := val.(blob); !isBlob {
							val = jsvalue.ToString(val)
						}
						f.Data = append(f.Data, react.FormField{Name: jsvalue.ToString(pair[0]), Value: val})
					}
				}
				return f
			}
		}
		if _, ok := m["sigEqual"]; ok {
			se := E("sigEqual")
			a.IsSignatureEqual = func(x, y any) bool { return jsvalue.Truthy(call(se, x, y)) }
		}
		return a
	}
	panic(fmt.Sprintf("bad expr %v", m))
}

func arg(a []any, i int) any {
	if i < len(a) {
		return a[i]
	}
	return react.Undefined{}
}

func deps(v any) []any {
	if jsvalue.IsNullish(v) {
		return nil
	}
	items := jsvalue.Items(v)
	if items == nil {
		return []any{}
	}
	return items
}

func fn0(v any) func() any {
	if jsvalue.IsNullish(v) {
		return nil
	}
	return func() any { return call(v) }
}

func fn1(v any) func(any) any {
	if jsvalue.IsUndefined(v) {
		return nil
	}
	return func(a any) any { return call(v, a) }
}

func fn2(v any) func(any, any) any {
	return func(a, b any) any { return call(v, a, b) }
}

func callHook(h react.Hooks, name string, a []any) any {
	switch name {
	case "useState":
		s, set := h.UseState(arg(a, 0))
		return []any{s, set}
	case "useReducer":
		var init func(any) any
		if len(a) > 2 {
			init = fn1(a[2])
		}
		s, d := h.UseReducer(fn2(arg(a, 0)), arg(a, 1), init)
		return []any{s, d}
	case "useMemo":
		return h.UseMemo(fn0(arg(a, 0)), deps(arg(a, 1)))
	case "useCallback":
		return h.UseCallback(arg(a, 0), deps(arg(a, 1)))
	case "useRef":
		return h.UseRef(arg(a, 0))
	case "useContext":
		return h.UseContext(arg(a, 0).(*react.Context))
	case "useId":
		return h.UseId()
	case "useSyncExternalStore":
		return h.UseSyncExternalStore(arg(a, 0), fn0(arg(a, 1)), fn0(arg(a, 2)))
	case "use":
		return h.Use(arg(a, 0))
	case "useDeferredValue":
		if len(a) > 1 {
			return h.UseDeferredValue(a[0], a[1])
		}
		return h.UseDeferredValue(arg(a, 0))
	case "useTransition":
		p, start := h.UseTransition()
		return []any{p, start}
	case "useOptimistic":
		var r func(any, any) any
		if !jsvalue.IsUndefined(arg(a, 1)) {
			r = fn2(a[1])
		}
		s, set := h.UseOptimistic(arg(a, 0), r)
		return []any{s, set}
	case "useActionState", "useFormState":
		var permalink []string
		if len(a) > 2 && !jsvalue.IsUndefined(a[2]) {
			permalink = []string{jsvalue.ToString(a[2])}
		}
		var s any
		var d *react.Action
		var p bool
		if name == "useFormState" {
			s, d, p = reactdom.UseFormState(h, arg(a, 0), arg(a, 1), permalink...)
		} else {
			s, d, p = h.UseActionState(arg(a, 0), arg(a, 1), permalink...)
		}
		return []any{s, d, p}
	case "useFormStatus":
		return reactdom.UseFormStatus(h)
	case "useEffect":
		h.UseEffect(nil, deps(arg(a, 1)))
		return react.Undefined{}
	case "useLayoutEffect":
		h.UseLayoutEffect(nil, deps(arg(a, 1)))
		return react.Undefined{}
	case "useInsertionEffect":
		h.UseInsertionEffect(nil, deps(arg(a, 1)))
		return react.Undefined{}
	case "useImperativeHandle":
		h.UseImperativeHandle(arg(a, 0), nil, deps(arg(a, 2)))
		return react.Undefined{}
	case "useDebugValue":
		h.UseDebugValue(arg(a, 0))
		return react.Undefined{}
	case "useEffectEvent":
		return h.UseEffectEvent(arg(a, 0))
	case "useMemoCache":
		return h.UseMemoCache(int(jsvalue.ToNumber(arg(a, 0))))
	case "useCacheRefresh":
		return h.UseCacheRefresh()
	}
	panic("bad hook " + name)
}

func toObject(v any) *react.Object {
	o, _ := v.(*react.Object)
	return o
}

func callAPI(h react.Hooks, name string, a []any) any {
	switch name {
	case "Children.map":
		fn := arg(a, 1)
		return react.Children.Map(arg(a, 0), func(child any, i int) any { return call(fn, child, float64(i)) })
	case "Children.forEach":
		fn := arg(a, 1)
		react.Children.ForEach(arg(a, 0), func(child any, i int) { call(fn, child, float64(i)) })
		return react.Undefined{}
	case "Children.count":
		return float64(react.Children.Count(arg(a, 0)))
	case "Children.toArray":
		return react.Children.ToArray(arg(a, 0))
	case "Children.only":
		return react.Children.Only(arg(a, 0))
	case "cloneElement":
		el, _ := arg(a, 0).(*react.Element)
		var children []any
		if len(a) > 2 {
			children = a[2:]
		}
		return react.CloneElement(el, toObject(arg(a, 1)), children...)
	case "isValidElement":
		return react.IsValidElement(arg(a, 0))
	case "createRef":
		return react.CreateRef()
	case "version":
		return react.Version
	case "startTransition":
		fn := arg(a, 0)
		react.StartTransition(func() { call(fn) })
		return react.Undefined{}
	case "cache":
		return react.Cache(arg(a, 0))
	case "join":
		return jsvalue.Join(jsvalue.Items(arg(a, 0)), jsvalue.ToString(arg(a, 1)))
	case "map":
		fn := arg(a, 1)
		items := jsvalue.Items(arg(a, 0))
		out := make([]any, len(items))
		for i, it := range items {
			out[i] = call(fn, it, float64(i))
		}
		return out
	case "keys":
		keys := jsvalue.ObjectKeys(arg(a, 0))
		out := make([]any, len(keys))
		for i, k := range keys {
			out[i] = k
		}
		return out
	case "preload":
		reactdom.Preload(h, arg(a, 0), toObject(arg(a, 1)))
	case "preloadModule":
		reactdom.PreloadModule(h, arg(a, 0), toObject(arg(a, 1)))
	case "preinit":
		reactdom.Preinit(h, arg(a, 0), toObject(arg(a, 1)))
	case "preinitModule":
		reactdom.PreinitModule(h, arg(a, 0), arg(a, 1))
	case "preconnect":
		reactdom.Preconnect(h, arg(a, 0), toObject(arg(a, 1)))
	case "prefetchDNS":
		reactdom.PrefetchDNS(h, arg(a, 0))
	case "flushSync":
		fn := arg(a, 0)
		if jsvalue.IsNullish(fn) {
			return reactdom.FlushSync(nil)
		}
		return reactdom.FlushSync(func() any { return call(fn) })
	case "createPortal":
		return reactdom.CreatePortal(arg(a, 0), arg(a, 1))
	case "requestFormReset":
		reactdom.RequestFormReset(arg(a, 0))
	case "renderToString":
		if RenderToString == nil {
			panic("RenderToString not wired")
		}
		html, err := RenderToString(arg(a, 0))
		if err != nil {
			panic(err)
		}
		return html
	default:
		panic("bad api " + name)
	}
	return react.Undefined{}
}

// RenderToString is wired by the server package's tests (to avoid an import cycle).
var RenderToString func(node any) (string, error)

// LoadCases parses parity/react/cases.json.
func LoadCases(data []byte) ([]*Case, error) {
	var f struct {
		Cases []*Case `json:"cases"`
	}
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	return f.Cases, nil
}
