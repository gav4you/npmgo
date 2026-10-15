package types

// Tests of the Go API's own paths, which the parity cases (JavaScript values only) cannot reach: Go slices for arrays,
// Go regexps and typed nils, the Pusher scope, the typed traversal handlers, the export table, and concurrent use.

import (
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/gav4you/npmgo/internal/jsvalue"
)

// panicText runs f and returns how the oracle would render its exception ("" if it returns).
func panicText(f func()) (text string) {
	defer func() {
		if r := recover(); r != nil {
			text = thrown(r)
		}
	}()
	f()
	return ""
}

func TestGoSlicesAsArrays(t *testing.T) {
	// A builder accepts a Go slice where JavaScript takes an array, and the node keeps it.
	body := []any{ExpressionStatement(Identifier("a"))}
	b := BlockStatement(body)
	if got, ok := b.Get("body").([]any); !ok || len(got) != 1 {
		t.Fatalf("body = %#v", b.Get("body"))
	}
	if !IsNodesEquivalent(b, BlockStatement(jsvalue.NewArray(ExpressionStatement(Identifier("a"))))) {
		t.Error("a slice body is not equivalent to an array body")
	}
	if msg := panicText(func() { BlockStatement([]any{Identifier("a")}) }); msg != `throws TypeError: Property body[0] of BlockStatement expected node to be of a type ["Statement"] but instead got "Identifier"` {
		t.Errorf("element check through a slice: %s", msg)
	}
	// Pushing onto a slice gives the node a longer slice; the caller's slice keeps its length.
	trailing := []any{jsvalue.NewObject("type", "CommentLine", "value", "a")}
	n := Identifier("x")
	n.Set("trailingComments", trailing)
	AddComment(n, "trailing", "b")
	if len(trailing) != 1 || len(items(n.Get("trailingComments"))) != 2 {
		t.Errorf("trailing comments: caller %d, node %d", len(trailing), len(items(n.Get("trailingComments"))))
	}
	// An *jsvalue.Array is pushed onto in place, as in JavaScript.
	shared := jsvalue.NewArray(jsvalue.NewObject("type", "CommentLine", "value", "a"))
	m := Identifier("y")
	m.Set("innerComments", shared)
	AddComment(m, "inner", "b")
	if shared.Len() != 2 || m.Get("innerComments") != shared {
		t.Errorf("inner comments not pushed in place: %d", shared.Len())
	}
	// removeTypeDuplicates merges type parameters held in a slice into a new array.
	gen := func(params any) *jsvalue.Object {
		return GenericTypeAnnotation(Identifier("T"), jsvalue.NewObject("type", "TypeParameterInstantiation", "params", params))
	}
	first := gen([]any{StringTypeAnnotation()})
	got := RemoveTypeDuplicates([]any{first, gen([]any{NumberTypeAnnotation()})})
	if got.Len() != 1 || len(items(getProp(getProp(first, "typeParameters"), "params"))) != 2 {
		t.Errorf("merge through slices: %d types, %v", got.Len(), getProp(getProp(first, "typeParameters"), "params"))
	}
}

func TestValueToNodeGoValues(t *testing.T) {
	re := ValueToNode(regexp.MustCompile(`a/b+`))
	if re.Get("type") != "RegExpLiteral" || re.Get("pattern") != "a/b+" || re.Get("flags") != "" {
		t.Errorf("*regexp.Regexp: %v %v %v", re.Get("type"), re.Get("pattern"), re.Get("flags"))
	}
	js := ValueToNode(&jsvalue.RegExp{Source: `a\/b`, Flags: "gi"})
	if js.Get("pattern") != `a\/b` || js.Get("flags") != "gi" {
		t.Errorf("*jsvalue.RegExp: %v %v", js.Get("pattern"), js.Get("flags"))
	}
	if msg := panicText(func() { ValueToNode(&jsvalue.RegExp{Source: "a", Flags: "G"}) }); msg != "throws TypeError: Cannot read properties of null (reading '1')" {
		t.Errorf("flags that are no RegExp's: %s", msg)
	}
	if v := ValueToNode((*jsvalue.Object)(nil)); v.Get("type") != "NullLiteral" {
		t.Errorf("typed nil: %v", v.Get("type"))
	}
	if msg := panicText(func() { ValueToNode(map[string]any{"a": 1.0}) }); msg != "throws Error: don't know how to turn this value into a node" {
		t.Errorf("Go map: %s", msg)
	}
	if v := ValueToNode(int64(-3)); v.Get("type") != "UnaryExpression" || getProp(v.Get("argument"), "value") != 3.0 {
		t.Errorf("int64: %v", v.Get("type"))
	}
}

func TestTraversalHandlerForms(t *testing.T) {
	tree := Program(jsvalue.NewArray(ExpressionStatement(Identifier("a"))))
	var seen []string
	rec := func(node any, ancestors *jsvalue.Array, state any) { seen = append(seen, toStr(getProp(node, "type"))) }
	Traverse(tree, TraversalHandler(rec))
	Traverse(tree, rec)
	Traverse(tree, &TraversalHandlers{Enter: rec})
	Traverse(tree, TraversalHandlers{Exit: rec})
	if len(seen) != 12 {
		t.Errorf("handler calls: %v", seen)
	}
	if msg := panicText(func() { Traverse(tree, (*TraversalHandlers)(nil)) }); msg != "throws TypeError: Cannot destructure property 'enter' of 'handlers' as it is null." {
		t.Errorf("nil *TraversalHandlers: %s", msg)
	}
	if msg := panicText(func() { TraverseFast(tree, nil) }); msg != "throws TypeError: enter is not a function" {
		t.Errorf("nil enter: %s", msg)
	}
	count := 0
	stopped := TraverseFast(tree, func(node, opts any) any {
		count++
		if IsIdentifier(node) {
			return TraverseFastStop
		}
		return nil
	})
	if !stopped || count != 3 {
		t.Errorf("TraverseFast: stopped %v after %d", stopped, count)
	}
}

type pusher struct{ got []any }

func (p *pusher) Push(declar any) { p.got = append(p.got, declar) }

func TestSequenceExpressionScopes(t *testing.T) {
	nodes := func() *jsvalue.Array {
		return jsvalue.NewArray(VariableDeclaration("var", jsvalue.NewArray(VariableDeclarator(Identifier("v"), NumericLiteral(1.0)))))
	}
	p := &pusher{}
	ToSequenceExpression(nodes(), p)
	if len(p.got) != 1 || getProp(getProp(p.got[0], "id"), "name") != "v" {
		t.Errorf("Pusher scope: %v", p.got)
	}
	var nilPusher *pusher
	if msg := panicText(func() { ToSequenceExpression(nodes(), nilPusher) }); msg != "throws TypeError: Cannot read properties of null (reading 'push')" {
		t.Errorf("nil Pusher: %s", msg)
	}
	pushed := 0
	scope := jsvalue.NewObject("push", func(declar any) { pushed++ })
	ToSequenceExpression(nodes(), scope)
	if pushed != 1 {
		t.Errorf("object scope pushed %d", pushed)
	}
	if msg := panicText(func() { ToSequenceExpression(nodes(), jsvalue.NewObject()) }); msg != "throws TypeError: scope.push is not a function" {
		t.Errorf("scope without push: %s", msg)
	}
	if v := ToSequenceExpression(jsvalue.NewArray(ExpressionStatement(Identifier("a"))), nil); getProp(v, "name") != "a" {
		t.Errorf("no declarations, no scope needed: %v", v)
	}
}

func TestExportTable(t *testing.T) {
	if _, ok := Export("noSuchExport"); ok {
		t.Error("an unknown name is found")
	}
	if v, _ := Export("VISITOR_KEYS"); v != VISITOR_KEYS {
		t.Error("VISITOR_KEYS")
	}
	if n := len(ExportNames()); n != 1356 {
		t.Errorf("%d export names", n)
	}
	f, _ := Export("isIdentifier")
	if !f.(func(any, ...any) bool)(Identifier("x")) {
		t.Error("isIdentifier through the table")
	}
	b, _ := Export("tsAnyKeyword")
	if b.(func(...any) *jsvalue.Object)().Get("type") != "TSAnyKeyword" {
		t.Error("tsAnyKeyword through the table")
	}
}

func TestValidatorValues(t *testing.T) {
	if breaking8 {
		t.Skip("BABEL_TYPES_8_BREAKING is set: these are the default environment's validators")
	}
	v := getProp(getProp(getProp(NODE_FIELDS, "Identifier"), "name"), "validate").(*Validator)
	if v.Name() != "validate" || v.Length() != 3 || v.Props().Get("type") != "string" || v.JSTypeOf() != "function" {
		t.Errorf("validator %q/%d %v", v.Name(), v.Length(), v.Props().Keys())
	}
	if c := getProp(getProp(getProp(NODE_FIELDS, "ArrayExpression"), "elements"), "validate").(*Validator); c.Length() != 0 || !c.Props().Has("chainOf") {
		t.Errorf("chain %q/%d %v", c.Name(), c.Length(), c.Props().Keys())
	}
	if msg := panicText(func() { v.Call(Identifier("a"), "name", 1.0) }); msg != "throws TypeError: Property name expected type of string but got number" {
		t.Errorf("Call: %s", msg)
	}
	if NODE_PARENT_VALIDATIONS.Len() != 0 {
		t.Errorf("%d parent validations in the default environment", NODE_PARENT_VALIDATIONS.Len())
	}
}

// TestConcurrentUse is meant for -race: toKeyAlias's counter and the deprecation cache are the package's own mutable
// state; builders, validators and clones only touch the nodes they are given.
func TestConcurrentUse(t *testing.T) {
	old := ConsoleWarn
	defer func() { ConsoleWarn = old }()
	var mu sync.Mutex
	var warned []string
	ConsoleWarn = func(m string) {
		mu.Lock()
		warned = append(warned, m)
		mu.Unlock()
	}
	resetDeprecationWarnings()
	defer resetDeprecationWarnings()
	oldUID := ToKeyAliasUid()
	defer SetToKeyAliasUid(oldUID)
	SetToKeyAliasUid(0)
	const goroutines, rounds = 8, 200
	aliases := make([][]any, goroutines)
	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < rounds; i++ {
				aliases[g] = append(aliases[g], ToKeyAlias(jsvalue.NewObject("kind", "method")))
				IsNumberLiteral(nil)
				InternalDeprecationWarning("concurrentOld", "concurrentNew")
				n := CloneNode(BinaryExpression("+", Identifier("a"), NumericLiteral(1.0)))
				if !IsBinaryExpression(n) || !IsExpression(n) {
					t.Error("clone")
				}
			}
		}(g)
	}
	wg.Wait()
	seen := map[any]bool{}
	for _, list := range aliases {
		for _, a := range list {
			if seen[a] {
				t.Fatalf("method alias %v given twice", a)
			}
			seen[a] = true
		}
	}
	if ToKeyAliasUid() != goroutines*rounds {
		t.Errorf("uid %v", ToKeyAliasUid())
	}
	// IsNumberLiteral's caller is in this package (internal: no warning); InternalDeprecationWarning's caller's caller
	// is the goroutine's start (external), so it warns, once.
	if len(warned) != 1 || !strings.HasPrefix(warned[0], "`concurrentOld` has been deprecated, please migrate to `concurrentNew`\n") {
		t.Errorf("warnings: %q", warned)
	}
}

// TestCyclicTrees: a tree that contains itself recurses without end; JavaScript throws a RangeError where V8's stack
// runs out, Go raises the same RangeError at maxCallDepth, as a recoverable panic. A deep but finite tree is fine.
func TestCyclicTrees(t *testing.T) {
	const want = "throws RangeError: Maximum call stack size exceeded"
	cyclic := ArrayExpression()
	cyclic.Get("elements").(*jsvalue.Array).Push(cyclic)
	block := BlockStatement(jsvalue.NewArray())
	block.Get("body").(*jsvalue.Array).Push(block)
	list := jsvalue.NewArray()
	list.Push(list)
	qualified := jsvalue.NewObject("type", "QualifiedTypeIdentifier", "id", Identifier("q"))
	qualified.Set("qualification", qualified)
	generic := GenericTypeAnnotation(qualified)
	for name, f := range map[string]func(){
		"cloneNode":            func() { CloneNode(cyclic) },
		"traverse":             func() { Traverse(cyclic, func(any, *jsvalue.Array, any) {}) },
		"traverseFast":         func() { TraverseFast(cyclic, func(any, any) any { return nil }) },
		"removePropertiesDeep": func() { RemovePropertiesDeep(cyclic) },
		"isNodesEquivalent":    func() { IsNodesEquivalent(cyclic, cyclic) },
		"toSequenceExpression": func() { ToSequenceExpression(jsvalue.NewArray(block), jsvalue.NewArray()) },
		"valueToNode":          func() { ValueToNode(list) },
		"removeTypeDuplicates": func() { RemoveTypeDuplicates(jsvalue.NewArray(generic)) },
	} {
		if got := panicText(f); got != want {
			t.Errorf("%s on a cycle: %q", name, got)
		}
	}
	deep := Identifier("x")
	for i := 0; i < 5000; i++ {
		deep = ArrayExpression(jsvalue.NewArray(deep))
	}
	if got := panicText(func() {
		if !IsNodesEquivalent(CloneNode(deep), deep) {
			t.Error("a deep clone is not equivalent to its original")
		}
		count := 0
		Traverse(deep, func(any, *jsvalue.Array, any) { count++ })
		if count != 5001 {
			t.Errorf("traverse visited %d nodes", count)
		}
	}); got != "" {
		t.Errorf("a 5000-level tree: %s", got)
	}
}
