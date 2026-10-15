// Copyright (c) 2014-present Sebastian McKenzie and other contributors. MIT License (see LICENSE).
// Translated to Go by the npmgo contributors from @babel/types 7.29.0 (lib/converters/*.js).

package types

import (
	"math"
	"math/big"
	"regexp"
	"sync"

	"github.com/gav4you/npmgo/internal/jsvalue"
	hvi "github.com/gav4you/npmgo/pkgs/babel/helper-validator-identifier"
)

// EnsureBlock is ensureBlock(node, key = "body"): node[key] becomes a block statement (toBlock), which is returned.
func EnsureBlock(node any, key ...string) any {
	k := "body"
	if len(key) > 0 {
		k = key[0]
	}
	result := ToBlock(getProp(node, k), node)
	setProp(node, k, result)
	return result
}

// ToBindingIdentifierName is toBindingIdentifierName(name): toIdentifier, and an underscore before eval and
// arguments.
func ToBindingIdentifierName(name any) string {
	s := ToIdentifier(name)
	if s == "eval" || s == "arguments" {
		s = "_" + s
	}
	return s
}

// ToBlock is toBlock(node, parent): node itself if it is a block statement, else a block statement of it (an
// expression becomes a return statement in a function, an expression statement elsewhere; an empty statement gives an
// empty block).
func ToBlock(node any, parent any) any {
	if IsBlockStatement(node) {
		return node
	}
	blockNodes := &jsvalue.Array{}
	if !IsEmptyStatement(node) {
		if !IsStatement(node) {
			if IsFunction(parent) {
				node = ReturnStatement(node)
			} else {
				node = ExpressionStatement(node)
			}
		}
		blockNodes = jsvalue.NewArray(node)
	}
	return BlockStatement(blockNodes)
}

// ToComputedKey is toComputedKey(node, key = node.key || node.property): the key as an expression (a string literal
// for a non-computed identifier).
func ToComputedKey(node any, key ...any) any {
	k := optArg(key)
	if jsvalue.IsUndefined(k) {
		k = getProp(node, "key")
		if !truthy(k) {
			k = getProp(node, "property")
		}
	}
	if !truthy(getProp(node, "computed")) && IsIdentifier(k) {
		k = StringLiteral(getProp(k, "name"))
	}
	return k
}

// ToExpression is toExpression(node): the expression of an expression statement, an expression itself, a class or
// function declaration turned into an expression (its type changed in place); anything else panics with JavaScript's
// Error.
func ToExpression(node any) any {
	if IsExpressionStatement(node) {
		node = getProp(node, "expression")
	}
	if IsExpression(node) {
		return node
	}
	if IsClass(node) {
		setProp(node, "type", "ClassExpression")
		setProp(node, "abstract", false)
	} else if IsFunction(node) {
		setProp(node, "type", "FunctionExpression")
	}
	if !IsExpression(node) {
		panic(&jsvalue.Error{Message: "cannot turn " + toStr(getProp(node, "type")) + " to an expression"})
	}
	return node
}

var jsWhiteSpace = map[uint16]bool{
	'\t': true, '\n': true, '\v': true, '\f': true, '\r': true, ' ': true, 0xa0: true, 0x1680: true, 0x2028: true,
	0x2029: true, 0x202f: true, 0x205f: true, 0x3000: true, 0xfeff: true,
}

func isRegExpSpace(u uint16) bool { return jsWhiteSpace[u] || u >= 0x2000 && u <= 0x200a }

func isLineTerminator(u uint16) bool { return u == '\n' || u == '\r' || u == 0x2028 || u == 0x2029 }

// ToIdentifier is toIdentifier(input): a valid identifier made from the string of input (characters that cannot be
// in an identifier become word breaks, camel-cased; an underscore in front where needed).
func ToIdentifier(input any) string {
	s := toStr(input)
	var name []uint16
	for _, r := range s {
		if hvi.IsIdentifierChar(float64(r)) {
			name = append(name, utf16Units(string(r))...)
		} else {
			name = append(name, '-')
		}
	}
	// name.replace(/^[-0-9]+/, "")
	i := 0
	for i < len(name) && (name[i] == '-' || name[i] >= '0' && name[i] <= '9') {
		i++
	}
	name = name[i:]
	// name.replace(/[-\s]+(.)?/g, (match, c) => c ? c.toUpperCase() : ""): "." is one code unit, not a line terminator.
	var out []uint16
	for j := 0; j < len(name); {
		if name[j] == '-' || isRegExpSpace(name[j]) {
			k := j
			for k < len(name) && (name[k] == '-' || isRegExpSpace(name[k])) {
				k++
			}
			if k < len(name) && !isLineTerminator(name[k]) {
				c := name[k]
				if c >= 0xd800 && c <= 0xdfff {
					out = append(out, c) // a lone surrogate has no case
				} else {
					out = append(out, utf16Units(jsvalue.ToUpperCase(unitString(c)))...)
				}
				k++
			}
			j = k
			continue
		}
		out = append(out, name[j])
		j++
	}
	result := unitsString(out)
	if !IsValidIdentifier(result) {
		result = "_" + result
	}
	if result == "" {
		return "_"
	}
	return result
}

var toKeyAliasState struct {
	sync.Mutex
	uid float64
}

const maxSafeInteger = 9007199254740991

// ToKeyAliasIncrement is toKeyAlias.increment(): the counter's value before it is incremented, or 0 when it has
// reached Number.MAX_SAFE_INTEGER (it is reset). Safe for concurrent use.
func ToKeyAliasIncrement() float64 {
	toKeyAliasState.Lock()
	defer toKeyAliasState.Unlock()
	if toKeyAliasState.uid >= maxSafeInteger {
		toKeyAliasState.uid = 0
		return 0
	}
	old := toKeyAliasState.uid
	toKeyAliasState.uid++
	return old
}

// ToKeyAliasUid reads toKeyAlias.uid, the counter that numbers method keys.
func ToKeyAliasUid() float64 {
	toKeyAliasState.Lock()
	defer toKeyAliasState.Unlock()
	return toKeyAliasState.uid
}

// SetToKeyAliasUid is the assignment toKeyAlias.uid = uid.
func SetToKeyAliasUid(uid float64) {
	toKeyAliasState.Lock()
	defer toKeyAliasState.Unlock()
	toKeyAliasState.uid = uid
}

// ToKeyAlias is toKeyAlias(node, key = node.key): a string that identifies the key of a class or object member
// (methods get a fresh number). It returns JavaScript's value: a string, except where an identifier's name is not one
// or JSON.stringify gives undefined.
func ToKeyAlias(node any, key ...any) any {
	k := optArg(key)
	if jsvalue.IsUndefined(k) {
		k = getProp(node, "key")
	}
	var alias any
	if strictEquals(getProp(node, "kind"), "method") {
		return toStr(ToKeyAliasIncrement())
	} else if IsIdentifier(k) {
		alias = getProp(k, "name")
	} else if IsStringLiteral(k) {
		alias = jsonValue(getProp(k, "value"))
	} else {
		alias = jsonValue(RemovePropertiesDeep(CloneNode(k)))
	}
	if truthy(getProp(node, "computed")) {
		alias = "[" + toStr(alias) + "]"
	}
	if truthy(getProp(node, "static")) {
		alias = "static:" + toStr(alias)
	}
	return alias
}

// jsonValue is JSON.stringify(v): a string, or undefined.
func jsonValue(v any) any {
	if s, ok := jsvalue.JSONStringify(v); ok {
		return s
	}
	return jsvalue.Undefined{}
}

// Pusher is what toSequenceExpression's scope must have: Babel's Scope.push. A *jsvalue.Array is accepted too.
type Pusher interface{ Push(declar any) }

// ToSequenceExpression is toSequenceExpression(nodes, scope): one expression that does what the statements do, or
// undefined where they cannot be expressed so; the var declarations it removes are pushed onto scope as {kind, id}.
func ToSequenceExpression(nodes any, scope any) any {
	if !(!nullish(nodes) && truthy(getProp(nodes, "length"))) {
		return jsvalue.Undefined{}
	}
	declars := &jsvalue.Array{}
	result := gatherSequenceExpressions(nodes, declars, 0)
	if !truthy(result) {
		return jsvalue.Undefined{}
	}
	for _, declar := range items(declars) {
		switch s := scope.(type) {
		case Pusher:
			if !jsvalue.IsNull(s) {
				s.Push(declar)
				continue
			}
		case *jsvalue.Array:
			if s != nil {
				s.Push(declar)
				continue
			}
		}
		if nullish(scope) {
			panic(readError(scope, "push"))
		}
		callValue(getProp(scope, "push"), "scope.push", declar)
	}
	return result
}

func gatherSequenceExpressions(nodes any, declars *jsvalue.Array, depth int) any {
	depth = deeper(depth)
	exprs := &jsvalue.Array{}
	ensureLastUndefined := true
	for _, node := range iterate(nodes, "nodes") {
		if !IsEmptyStatement(node) {
			ensureLastUndefined = false
		}
		switch {
		case IsExpression(node):
			exprs.Push(node)
		case IsExpressionStatement(node):
			exprs.Push(getProp(node, "expression"))
		case IsVariableDeclaration(node):
			if !strictEquals(getProp(node, "kind"), "var") {
				return jsvalue.Undefined{}
			}
			for _, declar := range iterate(getProp(node, "declarations"), "node.declarations") {
				bindings := GetBindingIdentifiers(declar)
				for _, key := range bindings.Keys() {
					declars.Push(jsvalue.NewObject("kind", getProp(node, "kind"), "id", CloneNode(bindings.Get(key))))
				}
				if truthy(getProp(declar, "init")) {
					exprs.Push(AssignmentExpression("=", getProp(declar, "id"), getProp(declar, "init")))
				}
			}
			ensureLastUndefined = true
		case IsIfStatement(node):
			var consequent, alternate any
			if truthy(getProp(node, "consequent")) {
				consequent = gatherSequenceExpressions(jsvalue.NewArray(getProp(node, "consequent")), declars, depth)
			} else {
				consequent = BuildUndefinedNode()
			}
			if truthy(getProp(node, "alternate")) {
				alternate = gatherSequenceExpressions(jsvalue.NewArray(getProp(node, "alternate")), declars, depth)
			} else {
				alternate = BuildUndefinedNode()
			}
			if !truthy(consequent) || !truthy(alternate) {
				return jsvalue.Undefined{}
			}
			exprs.Push(ConditionalExpression(getProp(node, "test"), consequent, alternate))
		case IsBlockStatement(node):
			body := gatherSequenceExpressions(getProp(node, "body"), declars, depth)
			if !truthy(body) {
				return jsvalue.Undefined{}
			}
			exprs.Push(body)
		case IsEmptyStatement(node):
			if indexOf(nodes, node) == 0 {
				ensureLastUndefined = true
			}
		default:
			return jsvalue.Undefined{}
		}
	}
	if ensureLastUndefined {
		exprs.Push(BuildUndefinedNode())
	}
	if exprs.Len() == 1 {
		return exprs.At(0)
	}
	return SequenceExpression(exprs)
}

// indexOf is v.indexOf(x) for an array (strict equality) or a string.
func indexOf(v any, x any) int {
	if s, ok := v.(string); ok {
		return indexUnits(utf16Units(s), utf16Units(toStr(x)), 0)
	}
	for i, it := range items(v) {
		if strictEquals(it, x) {
			return i
		}
	}
	return -1
}

// ToStatement is toStatement(node, ignore): node itself if it is a statement, an expression statement of an
// assignment, a class or function expression with an id turned into a declaration (its type changed in place); for
// anything else false when ignore is true, JavaScript's Error otherwise.
func ToStatement(node any, ignore ...bool) any {
	if IsStatement(node) {
		return node
	}
	mustHaveID := false
	var newType any = jsvalue.Undefined{}
	if IsClass(node) {
		mustHaveID = true
		newType = "ClassDeclaration"
	} else if IsFunction(node) {
		mustHaveID = true
		newType = "FunctionDeclaration"
	} else if IsAssignmentExpression(node) {
		return ExpressionStatement(node)
	}
	if mustHaveID && !truthy(getProp(node, "id")) {
		newType = false
	}
	if !truthy(newType) {
		if len(ignore) > 0 && ignore[0] {
			return false
		}
		panic(&jsvalue.Error{Message: "cannot turn " + toStr(getProp(node, "type")) + " to a statement"})
	}
	setProp(node, "type", newType)
	return node
}

var flagsAtEnd = regexp.MustCompile(`/([a-z]*)$`)

// isPlainObject is valueToNode.js's isPlainObject: an object whose prototype is Object.prototype or null.
func isPlainObject(v any) bool {
	o, ok := v.(*jsvalue.Object)
	return ok && o != nil
}

// ValueToNode is valueToNode(value): the expression node that evaluates to value: undefined, booleans, null, strings,
// numbers (negative and non-finite ones through - and /), BigInts, RegExps (*jsvalue.RegExp; a *regexp.Regexp gives
// its Go pattern and no flags), arrays and plain objects (*jsvalue.Object). Anything else panics with JavaScript's
// Error, and a cyclic value with its RangeError (maxCallDepth).
func ValueToNode(value any) *jsvalue.Object { return valueToNode(value, 0) }

func valueToNode(value any, depth int) *jsvalue.Object {
	depth = deeper(depth)
	switch v := value.(type) {
	case jsvalue.Undefined:
		return Identifier("undefined")
	case bool:
		return BooleanLiteral(v)
	case nil:
		return NullLiteral()
	case string:
		return StringLiteral(v)
	case *big.Int:
		if v == nil {
			return NullLiteral()
		}
		if v.Sign() < 0 {
			return UnaryExpression("-", BigIntLiteral(new(big.Int).Neg(v)))
		}
		return BigIntLiteral(v)
	}
	if jsvalue.IsNumber(value) {
		f := jsvalue.Float(value)
		var result *jsvalue.Object
		if !math.IsNaN(f) && !math.IsInf(f, 0) {
			result = NumericLiteral(math.Abs(f))
		} else {
			var numerator *jsvalue.Object
			if math.IsNaN(f) {
				numerator = NumericLiteral(0.0)
			} else {
				numerator = NumericLiteral(1.0)
			}
			result = BinaryExpression("/", numerator, NumericLiteral(0.0))
		}
		if f < 0 || f == 0 && math.Signbit(f) {
			result = UnaryExpression("-", result)
		}
		return result
	}
	if jsvalue.IsNull(value) {
		return NullLiteral()
	}
	if source, text, ok := regExpParts(value); ok {
		m := flagsAtEnd.FindStringSubmatch(text)
		if m == nil {
			panic(typeError("Cannot read properties of null (reading '1')"))
		}
		return RegExpLiteral(source, m[1])
	}
	if isArray(value) {
		elements := &jsvalue.Array{}
		for _, it := range items(value) {
			elements.Push(valueToNode(it, depth))
		}
		return ArrayExpression(elements)
	}
	if isPlainObject(value) {
		o := value.(*jsvalue.Object)
		props := &jsvalue.Array{}
		for _, key := range o.Keys() {
			var nodeKey *jsvalue.Object
			computed := false
			if IsValidIdentifier(key) {
				if key == "__proto__" {
					computed = true
					nodeKey = StringLiteral(key)
				} else {
					nodeKey = Identifier(key)
				}
			} else {
				nodeKey = StringLiteral(key)
			}
			props.Push(ObjectProperty(nodeKey, valueToNode(o.Get(key), depth), computed))
		}
		return ObjectExpression(props)
	}
	panic(&jsvalue.Error{Message: "don't know how to turn this value into a node"})
}

// regExpParts returns a RegExp value's source and its toString.
func regExpParts(v any) (source, text string, ok bool) {
	switch r := v.(type) {
	case *jsvalue.RegExp:
		if r != nil {
			return r.Source, r.String(), true
		}
	case *regexp.Regexp:
		if r != nil {
			return r.String(), "/" + r.String() + "/", true
		}
	}
	return "", "", false
}
