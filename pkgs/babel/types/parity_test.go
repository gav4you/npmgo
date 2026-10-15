package types

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/gav4you/npmgo/internal/jsvalue"
	"github.com/gav4you/npmgo/internal/parityspec"
)

const (
	casesPath    = "../../../parity/babel-types/cases.json"
	expectedPath = "../../../parity/babel-types/expected.json"
)

type expectedTypes struct {
	Exports      [][]string
	Load         []string
	Tables       []string
	Deprecations [][]string
	Is           [][]any
	Asserts      [][]any
	Opts         [][]string
	Builders     [][]string
	Calls        [][]string
}

type expectedBreaking struct {
	Load     []string
	Tables   []string
	Builders []json.RawMessage
	Calls    []json.RawMessage
}

type expectedFile struct {
	Node     string
	Types    expectedTypes
	Breaking expectedBreaking
}

func loadExpected(t *testing.T) *expectedFile {
	raw, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatal(err)
	}
	var e expectedFile
	if err := json.Unmarshal(raw, &e); err != nil {
		t.Fatal(err)
	}
	return &e
}

func loadCases(t *testing.T) *ordered {
	raw, err := os.ReadFile(casesPath)
	if err != nil {
		t.Fatal(err)
	}
	return decodeOrdered(raw).(*ordered)
}

// at reads a path of keys through decoded cases.
func at(v any, keys ...string) any {
	for _, k := range keys {
		v, _ = v.(*ordered).get(k)
	}
	return v
}

func list(v any) []any {
	l, _ := v.([]any)
	return l
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

// exportValue is t[name].
func exportValue(name string) any {
	v, ok := Export(name)
	if !ok {
		panic("no export " + name)
	}
	return v
}

// callExternal calls an export from outside the Babel packages.
func callExternal(fn any) func(args []any) any {
	return func(args []any) any { return parityspec.CallFromOutside(fn, args) }
}

// callInternal calls a deprecated export from a Babel package (this one).
func callInternal(fn any, args []any) any {
	arg := func(i int) any {
		if i < len(args) {
			return args[i]
		}
		return jsvalue.Undefined{}
	}
	switch f := fn.(type) {
	case func(...any) *jsvalue.Object:
		return f(args...)
	case func(any, ...any) bool:
		return f(arg(0), args[min(1, len(args)):]...)
	case func(any, ...any):
		f(arg(0), args[min(1, len(args)):]...)
		return jsvalue.Undefined{}
	case func(string, string, ...string):
		return deprecationFromBabel(f, args)
	}
	panic(fmt.Sprintf("callInternal: %T", fn))
}

// deprecationFromBabel and deprecationFromBabelInner call __internal__deprecationWarning from a Babel package, two
// frames deep, as a function of another Babel package would.
func deprecationFromBabel(f func(string, string, ...string), args []any) any {
	return deprecationFromBabelInner(f, args)
}

func deprecationFromBabelInner(f func(string, string, ...string), args []any) any {
	strs := make([]string, len(args))
	for i, a := range args {
		strs[i] = toStr(a)
	}
	f(strs[0], strs[1], strs[2:]...)
	return jsvalue.Undefined{}
}

// tablesTrace is the oracle's tables().
func tablesTrace() []string {
	var labels []string
	var roots []any
	add := func(label string, v any) {
		labels = append(labels, label)
		roots = append(roots, v)
	}
	tables := map[string]*jsvalue.Object{
		"VISITOR_KEYS": VISITOR_KEYS, "ALIAS_KEYS": ALIAS_KEYS, "FLIPPED_ALIAS_KEYS": FLIPPED_ALIAS_KEYS, "NODE_FIELDS": NODE_FIELDS,
		"BUILDER_KEYS": BUILDER_KEYS, "DEPRECATED_KEYS": DEPRECATED_KEYS, "NODE_PARENT_VALIDATIONS": NODE_PARENT_VALIDATIONS,
		"NODE_UNION_SHAPES__PRIVATE": NODE_UNION_SHAPES__PRIVATE, "PLACEHOLDERS_ALIAS": PLACEHOLDERS_ALIAS,
		"PLACEHOLDERS_FLIPPED_ALIAS": PLACEHOLDERS_FLIPPED_ALIAS, "DEPRECATED_ALIASES": DEPRECATED_ALIASES,
	}
	for _, name := range []string{"VISITOR_KEYS", "ALIAS_KEYS", "FLIPPED_ALIAS_KEYS", "NODE_FIELDS", "BUILDER_KEYS", "DEPRECATED_KEYS",
		"NODE_PARENT_VALIDATIONS", "NODE_UNION_SHAPES__PRIVATE", "PLACEHOLDERS_ALIAS", "PLACEHOLDERS_FLIPPED_ALIAS", "DEPRECATED_ALIASES"} {
		table := tables[name]
		add(name+" keys", strArray(table.Keys()))
		for _, k := range table.Keys() {
			add(name+"."+k, table.Get(k))
		}
	}
	for _, e := range exportTable {
		if e.kind == "function" {
			continue
		}
		v := e.value.(func() any)()
		if _, isObj := v.(*jsvalue.Object); isObj || e.name == "react" {
			continue
		}
		add(e.name, v)
	}
	add("INHERIT_KEYS", INHERIT_KEYS)
	add("getBindingIdentifiers.keys", GetBindingIdentifiersKeys)
	add("traverseFast.skip", TraverseFastSkip)
	add("traverseFast.stop", TraverseFastStop)
	add("toKeyAlias.uid", ToKeyAliasUid())
	add("react", jsvalue.NewArray("isReactComponent", "isCompatTag", "buildChildren"))
	encoded := encAll(roots)
	out := make([]string, len(labels))
	for i := range labels {
		out[i] = labels[i] + " = " + encoded[i]
	}
	return out
}

// kindOf is the oracle's kind of an export value: Array.isArray, or typeof.
func kindOf(v any) string {
	switch v.(type) {
	case *jsvalue.Array:
		return "array"
	case jsvalue.Symbol:
		return "symbol"
	}
	if reflect.ValueOf(v).Kind() == reflect.Func {
		return "function"
	}
	return "object"
}

// buildersTrace is the oracle's builders().
func buildersTrace(cases *ordered) [][]string {
	var out [][]string
	for _, b := range list(at(cases, "types", "builders")) {
		names := list(at(b, "names"))
		valid := list(at(b, "valid"))
		bad := list(at(b, "bad"))
		first := exportValue(str(names[0]))
		for _, name := range names {
			out = append(out, run(callExternal(exportValue(str(name))), buildArgs(valid), false))
		}
		for p := range valid {
			for _, badSpec := range list(bad[p]) {
				specs := append([]any{}, valid...)
				specs[p] = badSpec
				out = append(out, run(callExternal(first), buildArgs(specs), false))
			}
		}
		for k := 0; k < len(valid); k++ {
			out = append(out, run(callExternal(first), buildArgs(valid[:k]), false))
		}
	}
	return out
}

// truthyAt is the truthiness of an optional argument.
func truthyAt(args []any, i int) bool { return truthy(arg(args, i)) }

// defaulted is a boolean argument with a default for undefined.
func defaulted(args []any, i int, def bool) bool {
	v := arg(args, i)
	if jsvalue.IsUndefined(v) {
		return def
	}
	return truthy(v)
}

// callAdapters call the other exports with their Go signatures.
var callAdapters = map[string]func(args []any) any{
	"cloneNode": func(a []any) any {
		return CloneNode(arg(a, 0), defaulted(a, 1, true), defaulted(a, 2, false))
	},
	"clone":               func(a []any) any { return Clone(arg(a, 0)) },
	"cloneDeep":           func(a []any) any { return CloneDeep(arg(a, 0)) },
	"cloneDeepWithoutLoc": func(a []any) any { return CloneDeepWithoutLoc(arg(a, 0)) },
	"cloneWithoutLoc":     func(a []any) any { return CloneWithoutLoc(arg(a, 0)) },
	"addComment": func(a []any) any {
		return AddComment(arg(a, 0), toStr(arg(a, 1)), arg(a, 2), truthyAt(a, 3))
	},
	"addComments": func(a []any) any { return AddComments(arg(a, 0), toStr(arg(a, 1)), arg(a, 2)) },
	"inheritInnerComments": func(a []any) any {
		InheritInnerComments(arg(a, 0), arg(a, 1))
		return jsvalue.Undefined{}
	},
	"inheritLeadingComments": func(a []any) any {
		InheritLeadingComments(arg(a, 0), arg(a, 1))
		return jsvalue.Undefined{}
	},
	"inheritTrailingComments": func(a []any) any {
		InheritTrailingComments(arg(a, 0), arg(a, 1))
		return jsvalue.Undefined{}
	},
	"inheritsComments": func(a []any) any { return InheritsComments(arg(a, 0), arg(a, 1)) },
	"removeComments":   func(a []any) any { return RemoveComments(arg(a, 0)) },
	"ensureBlock": func(a []any) any {
		if len(a) > 1 {
			return EnsureBlock(arg(a, 0), toStr(arg(a, 1)))
		}
		return EnsureBlock(arg(a, 0))
	},
	"toBlock":                 func(a []any) any { return ToBlock(arg(a, 0), arg(a, 1)) },
	"toIdentifier":            func(a []any) any { return ToIdentifier(arg(a, 0)) },
	"toBindingIdentifierName": func(a []any) any { return ToBindingIdentifierName(arg(a, 0)) },
	"toComputedKey":           func(a []any) any { return ToComputedKey(arg(a, 0), a[min(1, len(a)):]...) },
	"toExpression":            func(a []any) any { return ToExpression(arg(a, 0)) },
	"toStatement":             func(a []any) any { return ToStatement(arg(a, 0), truthyAt(a, 1)) },
	"toKeyAlias":              func(a []any) any { return ToKeyAlias(arg(a, 0), a[min(1, len(a)):]...) },
	"valueToNode":             func(a []any) any { return ValueToNode(arg(a, 0)) },
	"createTypeAnnotationBasedOnTypeof": func(a []any) any {
		return CreateTypeAnnotationBasedOnTypeof(arg(a, 0))
	},
	"buildUndefinedNode": func(a []any) any { return BuildUndefinedNode() },
	"appendToMemberExpression": func(a []any) any {
		return AppendToMemberExpression(arg(a, 0), arg(a, 1), truthyAt(a, 2))
	},
	"prependToMemberExpression": func(a []any) any { return PrependToMemberExpression(arg(a, 0), arg(a, 1)) },
	"inherits":                  func(a []any) any { return Inherits(arg(a, 0), arg(a, 1)) },
	"removeProperties": func(a []any) any {
		RemoveProperties(arg(a, 0), a[min(1, len(a)):]...)
		return jsvalue.Undefined{}
	},
	"removePropertiesDeep":      func(a []any) any { return RemovePropertiesDeep(arg(a, 0), a[min(1, len(a)):]...) },
	"removeTypeDuplicates":      func(a []any) any { return RemoveTypeDuplicates(arg(a, 0)) },
	"createFlowUnionType":       func(a []any) any { return CreateFlowUnionType(arg(a, 0)) },
	"createUnionTypeAnnotation": func(a []any) any { return CreateUnionTypeAnnotation(arg(a, 0)) },
	"createTSUnionType":         func(a []any) any { return CreateTSUnionType(arg(a, 0)) },
	"getBindingIdentifiers": func(a []any) any {
		return GetBindingIdentifiers(arg(a, 0), truthyAt(a, 1), truthyAt(a, 2), truthyAt(a, 3))
	},
	"getOuterBindingIdentifiers": func(a []any) any { return GetOuterBindingIdentifiers(arg(a, 0), truthyAt(a, 1)) },
	"getAssignmentIdentifiers":   func(a []any) any { return GetAssignmentIdentifiers(arg(a, 0)) },
	"getFunctionName": func(a []any) any {
		r := GetFunctionName(arg(a, 0), a[min(1, len(a)):]...)
		if r == nil {
			return nil
		}
		return r
	},
	"is":                func(a []any) any { return Is(arg(a, 0), arg(a, 1), a[min(2, len(a)):]...) },
	"isType":            func(a []any) any { return IsType(arg(a, 0), arg(a, 1)) },
	"isPlaceholderType": func(a []any) any { return IsPlaceholderType(arg(a, 0), arg(a, 1)) },
	"isNode":            func(a []any) any { return IsNode(arg(a, 0)) },
	"assertNode": func(a []any) any {
		AssertNode(arg(a, 0))
		return jsvalue.Undefined{}
	},
	"shallowEqual":         func(a []any) any { return ShallowEqual(arg(a, 0), arg(a, 1)) },
	"isBinding":            func(a []any) any { return IsBinding(arg(a, 0), arg(a, 1), a[min(2, len(a)):]...) },
	"isBlockScoped":        func(a []any) any { return IsBlockScoped(arg(a, 0)) },
	"isLet":                func(a []any) any { return IsLet(arg(a, 0)) },
	"isVar":                func(a []any) any { return IsVar(arg(a, 0)) },
	"isImmutable":          func(a []any) any { return IsImmutable(arg(a, 0)) },
	"isNodesEquivalent":    func(a []any) any { return IsNodesEquivalent(arg(a, 0), arg(a, 1)) },
	"isReferenced":         func(a []any) any { return IsReferenced(arg(a, 0), arg(a, 1), a[min(2, len(a)):]...) },
	"isScope":              func(a []any) any { return IsScope(arg(a, 0), arg(a, 1)) },
	"isSpecifierDefault":   func(a []any) any { return IsSpecifierDefault(arg(a, 0)) },
	"isValidES3Identifier": func(a []any) any { return IsValidES3Identifier(arg(a, 0)) },
	"isValidIdentifier": func(a []any) any {
		return IsValidIdentifier(arg(a, 0), defaulted(a, 1, true))
	},
	"matchesPattern": func(a []any) any { return MatchesPattern(arg(a, 0), arg(a, 1), truthyAt(a, 2)) },
	"validate": func(a []any) any {
		Validate(arg(a, 0), arg(a, 1), arg(a, 2))
		return jsvalue.Undefined{}
	},
	"react.isReactComponent": func(a []any) any { return React.IsReactComponent(arg(a, 0)) },
	"react.isCompatTag":      func(a []any) any { return React.IsCompatTag(arg(a, 0)) },
	"react.buildChildren":    func(a []any) any { return React.BuildChildren(arg(a, 0)) },
}

// callsTrace is the oracle's calls().
func callsTrace(cases *ordered) [][]string {
	SetToKeyAliasUid(0)
	var out [][]string
	for _, c := range list(at(cases, "types", "calls")) {
		name := str(at(c, "fn"))
		args := buildArgs(list(at(c, "args")))
		var f func(args []any) any
		var extra []func() string
		switch name {
		case "buildMatchMemberExpression":
			member := buildSpec(at(c, "member"), map[string]any{})
			f = func(a []any) any { return BuildMatchMemberExpression(toStr(arg(a, 0)), truthyAt(a, 1))(member) }
		case "traverse":
			rec := func(label string) func(node any, ancestors *jsvalue.Array, state any) {
				return func(node any, ancestors *jsvalue.Array, state any) {
					var path []string
					for _, a := range ancestors.Items() {
						o := a.(*jsvalue.Object)
						p := toStr(getProp(o.Get("node"), "type")) + "." + toStr(o.Get("key"))
						if o.Has("index") {
							p += "[" + encNumber(o.Get("index").(float64)) + "]"
						}
						path = append(path, p)
					}
					*warnSink = append(*warnSink, label+" "+toStr(getProp(node, "type"))+" "+strings.Join(path, "/")+" "+enc(state))
				}
			}
			var handlers any
			switch str(at(c, "handlers")) {
			case "function":
				handlers = TraversalHandler(rec("enter"))
			case "enter":
				handlers = TraversalHandlers{Enter: rec("enter")}
			case "exit":
				handlers = TraversalHandlers{Exit: rec("exit")}
			case "both":
				handlers = TraversalHandlers{Enter: rec("enter"), Exit: rec("exit")}
			case "object":
				handlers = jsvalue.NewObject("enter", rec("enter"), "exit", rec("exit"))
			case "null":
				handlers = nil
			case "undefined":
				handlers = jsvalue.Undefined{}
			case "badEnter":
				handlers = jsvalue.NewObject("enter", 5.0)
			case "badExit":
				handlers = jsvalue.NewObject("exit", "x")
			}
			f = func(a []any) any {
				Traverse(arg(a, 0), handlers, arg(a, 1))
				return jsvalue.Undefined{}
			}
		case "traverseFast":
			kind, typ, _ := strings.Cut(str(at(c, "enter")), ":")
			enter := func(node any, opts any) any {
				*warnSink = append(*warnSink, "enter "+toStr(getProp(node, "type"))+" "+enc(opts))
				if strictEquals(getProp(node, "type"), typ) {
					if kind == "skip" {
						return TraverseFastSkip
					}
					return TraverseFastStop
				}
				return jsvalue.Undefined{}
			}
			var opts []any
			if o := at(c, "opts"); o != nil {
				opts = []any{buildSpec(o, map[string]any{})}
			}
			f = func(a []any) any { return TraverseFast(arg(a, 0), enter, opts...) }
		case "toSequenceExpression":
			var scope any = jsvalue.Undefined{}
			if b, _ := at(c, "scope").(bool); b {
				scope = jsvalue.NewArray()
			}
			f = func(a []any) any { return ToSequenceExpression(arg(a, 0), scope) }
			extra = append(extra, func() string { return "scope " + enc(scope) })
		default:
			adapter, ok := callAdapters[name]
			if !ok {
				// A builder, validator or assert, called as the oracle calls it.
				adapter = callExternal(exportValue(name))
			}
			f = adapter
			if name == "toKeyAlias" {
				if uid, ok := at(c, "setUid").(json.Number); ok {
					v, _ := uid.Float64()
					SetToKeyAliasUid(v)
				}
			}
		}
		trace := run(f, args, true)
		for _, e := range extra {
			trace = append(trace, e())
		}
		if name == "toKeyAlias" {
			trace = append(trace, "uid "+encNumber(ToKeyAliasUid()))
		}
		out = append(out, trace)
	}
	return out
}

// deprecationsTrace is the oracle's deprecations().
func deprecationsTrace(cases *ordered) [][]string {
	var out [][]string
	for _, c := range list(at(cases, "types", "deprecations")) {
		fn := exportValue(str(at(c, "fn")))
		args := buildArgs(list(at(c, "args")))
		if str(at(c, "via")) == "internal" {
			out = append(out, run(func(a []any) any { return callInternal(fn, a) }, args, true))
		} else {
			out = append(out, run(callExternal(fn), args, true))
		}
	}
	return out
}

// generatedValidator is the function of a name of lib/validators/generated (isImmutable there is not the export).
func generatedValidator(name string, exported bool) any {
	if !exported {
		switch name {
		case "isImmutable":
			return isImmutableGenerated
		}
		panic("unexported validator " + name)
	}
	return exportValue(name)
}

func TestParity(t *testing.T) {
	cases := loadCases(t)
	exp := loadExpected(t)
	// The state the oracle starts from: the package just loaded.
	resetDeprecationWarnings()
	SetToKeyAliasUid(0)
	m := &mismatches{t: t}
	none := func() string { return "" }

	// Exports.
	names := ExportNames()
	if len(names) != len(exp.Types.Exports) {
		t.Errorf("exports: %d names, want %d", len(names), len(exp.Types.Exports))
	}
	for i, e := range exp.Types.Exports {
		got := []string{}
		if i < len(names) {
			v, _ := Export(names[i])
			got = []string{names[i], kindOf(v)}
		}
		m.compare("exports", i, e, got, none)
	}
	m.compare("load", 0, exp.Types.Load, []string{}, none)

	// Tables.
	tables := tablesTrace()
	if len(tables) != len(exp.Types.Tables) {
		t.Errorf("tables: %d lines, want %d", len(tables), len(exp.Types.Tables))
	}
	for i := 0; i < len(exp.Types.Tables) && i < len(tables); i++ {
		m.compare("tables", i, []string{exp.Types.Tables[i]}, []string{tables[i]}, none)
	}

	// Deprecations (first: a warning is given once per name).
	deps := deprecationsTrace(cases)
	for i, want := range exp.Types.Deprecations {
		m.compare("deprecations", i, want, deps[i], func() string { return fmt.Sprint(list(at(cases, "types", "deprecations"))[i]) })
	}

	// The is* and assert* matrices.
	samples := list(at(cases, "types", "samples"))
	built := make([]any, len(samples))
	for i, s := range samples {
		built[i] = buildSpec(s, map[string]any{})
	}
	for i, row := range exp.Types.Is {
		name := row[0].(string)
		fn := generatedValidator(name, row[1].(bool))
		trace := []string{}
		warnSink = &trace
		var b strings.Builder
		for _, s := range built {
			func() {
				defer func() {
					if r := recover(); r != nil {
						b.WriteByte('!')
					}
				}()
				if parityspec.CallFromOutside(fn, []any{s}).(bool) {
					b.WriteByte('1')
				} else {
					b.WriteByte('0')
				}
			}()
		}
		warnSink = nil
		got := append([]string{name, fmt.Sprint(row[1]), b.String()}, trace...)
		want := []string{name, fmt.Sprint(row[1])}
		for _, x := range row[2:] {
			want = append(want, x.(string))
		}
		m.compare("is", i, want, got, none)
	}
	messageSamples := map[int]bool{}
	for _, i := range list(at(cases, "types", "assertMessageSamples")) {
		n, _ := i.(json.Number).Int64()
		messageSamples[int(n)] = true
	}
	for i, row := range exp.Types.Asserts {
		name := row[0].(string)
		fn := exportValue(name)
		trace := []string{}
		warnSink = &trace
		var b strings.Builder
		var messages []string
		for j, s := range built {
			func() {
				defer func() {
					if r := recover(); r != nil {
						switch r.(type) {
						case *jsvalue.TypeError:
							b.WriteByte('T')
						case *jsvalue.Error:
							b.WriteByte('E')
						default:
							b.WriteByte('!')
						}
						if messageSamples[j] {
							messages = append(messages, fmt.Sprintf("%d %s", j, thrown(r)))
						}
					}
				}()
				parityspec.CallFromOutside(fn, []any{s})
				b.WriteByte('.')
				if messageSamples[j] {
					messages = append(messages, fmt.Sprintf("%d ok", j))
				}
			}()
		}
		warnSink = nil
		got := append(append([]string{name, b.String()}, trace...), messages...)
		want := []string{}
		for _, x := range row {
			want = append(want, x.(string))
		}
		m.compare("asserts", i, want, got, none)
	}
	var opts [][]string
	optsSamples := list(at(cases, "types", "optsSamples"))
	optsSpecs := list(at(cases, "types", "optsSpecs"))
	for _, name := range append(list(at(cases, "types", "optsValidators")), list(at(cases, "types", "optsAsserts"))...) {
		fn := exportValue(str(name))
		for _, sample := range optsSamples {
			for _, spec := range optsSpecs {
				args := buildArgs([]any{sample, spec})
				if o, ok := spec.(*ordered); ok {
					if k, _ := o.get("$"); k == "undef" {
						args = args[:1]
					}
				}
				opts = append(opts, run(callExternal(fn), args, true))
			}
		}
	}
	for i, want := range exp.Types.Opts {
		m.compare("opts", i, want, opts[i], none)
	}

	// Builders.
	builders := buildersTrace(cases)
	if len(builders) != len(exp.Types.Builders) {
		t.Errorf("builders: %d calls, want %d", len(builders), len(exp.Types.Builders))
	}
	for i := 0; i < len(exp.Types.Builders) && i < len(builders); i++ {
		m.compare("builders", i, exp.Types.Builders[i], builders[i], none)
	}

	// The other exports.
	callCases := list(at(cases, "types", "calls"))
	calls := callsTrace(cases)
	for i, want := range exp.Types.Calls {
		m.compare("calls", i, want, calls[i], func() string { return str(at(callCases[i], "fn")) })
	}
	t.Logf("%d traces compared, %d mismatches, %d compared with a lone surrogate as U+FFFD (recorded with Node %s)", m.checks, m.n, m.surrogates, exp.Node)
}

// TestParityBreaking8 runs TestBreaking8Child in a process with BABEL_TYPES_8_BREAKING set, as the oracle records
// the breaking section in a child process (the variable is read when the package is initialised).
func TestParityBreaking8(t *testing.T) {
	if os.Getenv("NPMGO_BABEL_BREAKING_CHILD") != "" {
		t.Skip("in the child")
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, "-test.run=^TestBreaking8Child$", "-test.count=1", "-test.v")
	cmd.Env = append(os.Environ(), "BABEL_TYPES_8_BREAKING=1", "NPMGO_BABEL_BREAKING_CHILD=1")
	var stdout, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err = cmd.Run()
	exp := loadExpected(t)
	// The warning lib/index.js gives when it loads.
	var load []string
	for _, line := range strings.Split(strings.ReplaceAll(stderr.String(), "\r\n", "\n"), "\n") {
		if strings.HasPrefix(line, "BABEL_TYPES_8_BREAKING") {
			load = append(load, "warn "+line)
		}
	}
	if fmt.Sprint(load) != fmt.Sprint(exp.Breaking.Load) {
		t.Errorf("load warnings %q, want %q", load, exp.Breaking.Load)
	}
	if err != nil || !strings.Contains(stdout.String(), "--- PASS: TestBreaking8Child") {
		t.Fatalf("the breaking child failed (%v):\n%s\n%s", err, stdout.String(), stderr.String())
	}
	for _, line := range strings.Split(stdout.String(), "\n") {
		if strings.Contains(line, "traces compared") {
			t.Log(strings.TrimSpace(line))
		}
	}
}

// TestBreaking8Child is the child of TestParityBreaking8.
func TestBreaking8Child(t *testing.T) {
	if os.Getenv("NPMGO_BABEL_BREAKING_CHILD") == "" {
		t.Skip("run by TestParityBreaking8")
	}
	if !breaking8 {
		t.Fatal("BABEL_TYPES_8_BREAKING was not seen at initialisation")
	}
	cases := loadCases(t)
	exp := loadExpected(t)
	resetDeprecationWarnings()
	m := &mismatches{t: t}
	none := func() string { return "" }
	resolve := func(raw json.RawMessage, def []string) []string {
		var s string
		if json.Unmarshal(raw, &s) == nil && s == "=" {
			return def
		}
		var tr []string
		if err := json.Unmarshal(raw, &tr); err != nil {
			t.Fatal(err)
		}
		return tr
	}
	tables := tablesTrace()
	for i, want := range exp.Breaking.Tables {
		if want == "=" {
			want = exp.Types.Tables[i]
		}
		got := ""
		if i < len(tables) {
			got = tables[i]
		}
		m.compare("breaking tables", i, []string{want}, []string{got}, none)
	}
	builders := buildersTrace(cases)
	for i, raw := range exp.Breaking.Builders {
		m.compare("breaking builders", i, resolve(raw, exp.Types.Builders[i]), builders[i], none)
	}
	calls := callsTrace(cases)
	for i, raw := range exp.Breaking.Calls {
		m.compare("breaking calls", i, resolve(raw, exp.Types.Calls[i]), calls[i], none)
	}
	t.Logf("breaking: %d traces compared, %d mismatches, %d compared with a lone surrogate as U+FFFD", m.checks, m.n, m.surrogates)
}
