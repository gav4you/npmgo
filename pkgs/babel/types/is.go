// Copyright (c) 2014-present Sebastian McKenzie and other contributors. MIT License (see LICENSE).
// Translated to Go by the npmgo contributors from @babel/types 7.29.0 (lib/validators/*.js but generated/,
// lib/validators/react/*.js, lib/utils/shallowEqual.js, lib/asserts/assertNode.js and the assert helper of
// lib/asserts/generated/index.js).

package types

import (
	"regexp"
	"strings"

	"github.com/gav4you/npmgo/internal/jsvalue"
	hvi "github.com/gav4you/npmgo/pkgs/babel/helper-validator-identifier"
)

// Is is is(type, node, opts): whether node is of the node type or alias type (a Placeholder that stands for it
// counts when no opts are given), and, if opts is not undefined, has its properties (shallowEqual).
func Is(typ any, node any, opts ...any) bool {
	if !truthy(node) {
		return false
	}
	matches := IsType(getProp(node, "type"), typ)
	if !matches {
		if !truthy(optArg(opts)) && strictEquals(getProp(node, "type"), "Placeholder") && tableHas(FLIPPED_ALIAS_KEYS, typ) {
			return IsPlaceholderType(getProp(node, "expectedNode"), typ)
		}
		return false
	}
	o := optArg(opts)
	if jsvalue.IsUndefined(o) {
		return true
	}
	return ShallowEqual(node, o)
}

// IsType is isType(nodeType, targetType): whether a node type is the target type or one of its node types when the
// target is an alias.
func IsType(nodeType, targetType any) bool {
	if strictEquals(nodeType, targetType) {
		return true
	}
	if nullish(nodeType) {
		return false
	}
	if truthy(tableGet(ALIAS_KEYS, targetType)) {
		return false
	}
	aliases := tableGet(FLIPPED_ALIAS_KEYS, targetType)
	if !nullish(aliases) && includes(aliases, nodeType, "aliases") {
		return true
	}
	return false
}

// IsPlaceholderType is isPlaceholderType(placeholderType, targetType): whether a Placeholder's expected node type
// is the target or has it among its aliases.
func IsPlaceholderType(placeholderType, targetType any) bool {
	if strictEquals(placeholderType, targetType) {
		return true
	}
	aliases := tableGet(PLACEHOLDERS_ALIAS, placeholderType)
	if !nullish(aliases) && includes(aliases, targetType, "aliases") {
		return true
	}
	return false
}

// IsNode is isNode(node): whether node has a type with visitor keys.
func IsNode(node any) bool {
	return truthy(node) && truthy(tableGet(VISITOR_KEYS, getProp(node, "type")))
}

// ShallowEqual is shallowEqual(actual, expected): whether actual has each of expected's own enumerable properties,
// with the same value (===).
func ShallowEqual(actual, expected any) bool {
	for _, key := range objectKeys(expected) {
		if !strictEquals(getProp(actual, key), getProp(expected, key)) {
			return false
		}
	}
	return true
}

// IsBinding is isBinding(node, parent, grandparent): whether node is in a binding position of parent.
func IsBinding(node, parent any, grandparent ...any) bool {
	gp := optArg(grandparent)
	if truthy(gp) && strictEquals(getProp(node, "type"), "Identifier") && strictEquals(getProp(parent, "type"), "ObjectProperty") &&
		strictEquals(getProp(gp, "type"), "ObjectExpression") {
		return false
	}
	keys := tableGet(GetBindingIdentifiersKeys, getProp(parent, "type"))
	if truthy(keys) {
		for i := 0; float64(i) < jsvalue.ToNumber(getProp(keys, "length")); i++ {
			key := getIndex(keys, i)
			val := getPropKey(parent, key)
			if isArray(val) {
				if includes(val, node, "val") {
					return true
				}
			} else if strictEquals(val, node) {
				return true
			}
		}
	}
	return false
}

// getPropKey is v[key] for a key of any type.
func getPropKey(v any, key any) any {
	if s, ok := key.(jsvalue.Symbol); ok {
		return getSymbolProp(v, s)
	}
	return getProp(v, toStr(key))
}

// IsBlockScoped is isBlockScoped(node): a function or class declaration, or what isLet returns.
func IsBlockScoped(node any) any {
	if IsFunctionDeclaration(node) || IsClassDeclaration(node) {
		return true
	}
	return IsLet(node)
}

// IsImmutable is isImmutable(node): an Immutable node type, or the identifier undefined.
func IsImmutable(node any) bool {
	if IsType(getProp(node, "type"), "Immutable") {
		return true
	}
	if IsIdentifier(node) {
		return strictEquals(getProp(node, "name"), "undefined")
	}
	return false
}

// IsLet is isLet(node): JavaScript's `isVariableDeclaration(node) && (node.kind !== "var" ||
// node[BLOCK_SCOPED_SYMBOL])`, so false, true, or the value of the node's BLOCK_SCOPED_SYMBOL property (undefined for
// a plain var).
func IsLet(node any) any {
	if !IsVariableDeclaration(node) {
		return false
	}
	if !strictEquals(getProp(node, "kind"), "var") {
		return true
	}
	return getSymbolProp(node, BLOCK_SCOPED_SYMBOL)
}

// IsVar is isVar(node): a var declaration not marked block scoped.
func IsVar(node any) bool {
	return IsVariableDeclaration(node, jsvalue.NewObject("kind", "var")) && !truthy(getSymbolProp(node, BLOCK_SCOPED_SYMBOL))
}

// IsNodesEquivalent is isNodesEquivalent(a, b): a deep comparison of the fields of two nodes.
func IsNodesEquivalent(a, b any) bool {
	if jsvalue.TypeOf(a) != "object" || jsvalue.TypeOf(b) != "object" || nullish(a) || nullish(b) {
		return strictEquals(a, b)
	}
	if !strictEquals(getProp(a, "type"), getProp(b, "type")) {
		return false
	}
	fieldsOf := tableGet(NODE_FIELDS, getProp(a, "type"))
	if !truthy(fieldsOf) {
		fieldsOf = getProp(a, "type")
	}
	fields := objectKeys(fieldsOf)
	visitorKeys := tableGet(VISITOR_KEYS, getProp(a, "type"))
	for _, field := range fields {
		valA := getProp(a, field)
		valB := getProp(b, field)
		if jsvalue.TypeOf(valA) != jsvalue.TypeOf(valB) {
			return false
		}
		if nullish(valA) && nullish(valB) {
			continue
		} else if nullish(valA) || nullish(valB) {
			return false
		}
		if isArray(valA) {
			if !isArray(valB) {
				return false
			}
			if len(items(valA)) != len(items(valB)) {
				return false
			}
			for i := 0; i < len(items(valA)); i++ {
				if !IsNodesEquivalent(items(valA)[i], items(valB)[i]) {
					return false
				}
			}
			continue
		}
		if jsvalue.TypeOf(valA) == "object" && !(!nullish(visitorKeys) && includes(visitorKeys, field, "visitorKeys")) {
			for _, key := range objectKeys(valA) {
				if !strictEquals(getProp(valA, key), getProp(valB, key)) {
					return false
				}
			}
			continue
		}
		if !IsNodesEquivalent(valA, valB) {
			return false
		}
	}
	return true
}

// IsReferenced is isReferenced(node, parent, grandparent): whether node, a child of parent, is a reference to a
// binding (rather than a name, a key or a binding itself).
func IsReferenced(node, parent any, grandparent ...any) bool {
	gp := optArg(grandparent)
	is := func(key string) bool { return strictEquals(getProp(parent, key), node) }
	typ, _ := getProp(parent, "type").(string)
	_, isString := getProp(parent, "type").(string)
	if !isString {
		return true
	}
	switch typ {
	case "MemberExpression", "OptionalMemberExpression":
		if is("property") {
			return truthy(getProp(parent, "computed"))
		}
		return is("object")
	case "JSXMemberExpression":
		return is("object")
	case "VariableDeclarator":
		return is("init")
	case "ArrowFunctionExpression":
		return is("body")
	case "PrivateName":
		return false
	case "ClassMethod", "ClassPrivateMethod", "ObjectMethod":
		if is("key") {
			return truthy(getProp(parent, "computed"))
		}
		return false
	case "ObjectProperty":
		if is("key") {
			return truthy(getProp(parent, "computed"))
		}
		var gpType any = jsvalue.Undefined{}
		if !nullish(gp) {
			gpType = getProp(gp, "type")
		}
		return !strictEquals(gpType, "ObjectPattern")
	case "ClassProperty", "ClassAccessorProperty":
		if is("key") {
			return truthy(getProp(parent, "computed"))
		}
		return true
	case "ClassPrivateProperty":
		return !is("key")
	case "ClassDeclaration", "ClassExpression":
		return is("superClass")
	case "AssignmentExpression":
		return is("right")
	case "AssignmentPattern":
		return is("right")
	case "LabeledStatement":
		return false
	case "CatchClause":
		return false
	case "RestElement":
		return false
	case "BreakStatement", "ContinueStatement":
		return false
	case "FunctionDeclaration", "FunctionExpression":
		return false
	case "ExportNamespaceSpecifier", "ExportDefaultSpecifier":
		return false
	case "ExportSpecifier":
		if !nullish(gp) && truthy(getProp(gp, "source")) {
			return false
		}
		return is("local")
	case "ImportDefaultSpecifier", "ImportNamespaceSpecifier", "ImportSpecifier":
		return false
	case "ImportAttribute":
		return false
	case "JSXAttribute":
		return false
	case "ObjectPattern", "ArrayPattern":
		return false
	case "MetaProperty":
		return false
	case "ObjectTypeProperty":
		return !is("key")
	case "TSEnumMember":
		return !is("id")
	case "TSPropertySignature":
		if is("key") {
			return truthy(getProp(parent, "computed"))
		}
		return true
	}
	return true
}

// IsScope is isScope(node, parent): whether node makes a scope (a function's or catch clause's block does not; their
// patterns do).
func IsScope(node, parent any) bool {
	if IsBlockStatement(node) && (IsFunction(parent) || IsCatchClause(parent)) {
		return false
	}
	if IsPattern(node) && (IsFunction(parent) || IsCatchClause(parent)) {
		return true
	}
	return IsScopable(node)
}

// IsSpecifierDefault is isSpecifierDefault(specifier): an import default specifier, or one named default.
func IsSpecifierDefault(specifier any) bool {
	if IsImportDefaultSpecifier(specifier) {
		return true
	}
	name := getProp(specifier, "imported")
	if !truthy(name) {
		name = getProp(specifier, "exported")
	}
	return IsIdentifier(name, jsvalue.NewObject("name", "default"))
}

var reservedWordsES3Only = map[string]bool{
	"abstract": true, "boolean": true, "byte": true, "char": true, "double": true, "enum": true, "final": true,
	"float": true, "goto": true, "implements": true, "int": true, "interface": true, "long": true, "native": true,
	"package": true, "private": true, "protected": true, "public": true, "short": true, "static": true,
	"synchronized": true, "throws": true, "transient": true, "volatile": true,
}

// IsValidES3Identifier is isValidES3Identifier(name): a valid identifier that ES3 does not reserve either.
func IsValidES3Identifier(name any) bool {
	if !IsValidIdentifier(name) {
		return false
	}
	s, _ := name.(string)
	return !reservedWordsES3Only[s]
}

// IsValidIdentifier is isValidIdentifier(name, reserved = true): a string that is an identifier name and, unless
// reserved is false, not a keyword or a strict-mode reserved word.
func IsValidIdentifier(name any, reserved ...bool) bool {
	s, ok := name.(string)
	if !ok {
		return false
	}
	if len(reserved) == 0 || reserved[0] {
		if hvi.IsKeyword(s) || hvi.IsStrictReservedWord(s, true) {
			return false
		}
	}
	return hvi.IsIdentifierName(s)
}

func isMemberExpressionLike(node any) bool { return IsMemberExpression(node) || IsMetaProperty(node) }

// MatchesPattern is matchesPattern(member, match, allowPartial): whether a member expression is the dotted path
// match (a string, or an array of its parts); with allowPartial it may continue further.
func MatchesPattern(member, match any, allowPartial ...bool) bool {
	if !isMemberExpressionLike(member) {
		return false
	}
	var parts []any
	if isArray(match) {
		parts = items(match)
	} else if s, ok := match.(string); ok {
		for _, p := range strings.Split(s, ".") {
			parts = append(parts, p)
		}
	} else {
		panic(typeError("match.split is not a function"))
	}
	return matchesParts(member, parts, len(allowPartial) > 0 && allowPartial[0])
}

func matchesParts(member any, parts []any, allowPartial bool) bool {
	if !isMemberExpressionLike(member) {
		return false
	}
	var nodes []any
	node := member
	for isMemberExpressionLike(node) {
		nodes = append(nodes, getProp(node, "property"))
		next := getProp(node, "object")
		if nullish(next) {
			next = getProp(node, "meta")
		}
		node = next
	}
	nodes = append(nodes, node)
	if len(nodes) < len(parts) {
		return false
	}
	if !allowPartial && len(nodes) > len(parts) {
		return false
	}
	for i, j := 0, len(nodes)-1; i < len(parts); i, j = i+1, j-1 {
		node := nodes[j]
		var value any
		switch {
		case IsIdentifier(node):
			value = getProp(node, "name")
		case IsStringLiteral(node):
			value = getProp(node, "value")
		case IsThisExpression(node):
			value = "this"
		case IsSuper(node):
			value = "super"
		case IsPrivateName(node):
			value = "#" + toStr(getProp(getProp(node, "id"), "name"))
		default:
			return false
		}
		if !strictEquals(parts[i], value) {
			return false
		}
	}
	return true
}

// BuildMatchMemberExpression is buildMatchMemberExpression(match, allowPartial): a matcher of the dotted path.
func BuildMatchMemberExpression(match string, allowPartial ...bool) func(member any) bool {
	var parts []any
	for _, p := range strings.Split(match, ".") {
		parts = append(parts, p)
	}
	partial := len(allowPartial) > 0 && allowPartial[0]
	return func(member any) bool { return matchesParts(member, parts, partial) }
}

var lowercaseStart = regexp.MustCompile(`^[a-z]`)

// isCompatTag is react.isCompatTag(tagName): a name that starts with a lowercase ASCII letter (an HTML tag).
func isCompatTag(tagName any) bool {
	return truthy(tagName) && lowercaseStart.MatchString(toStr(tagName))
}

// React is the react export: isReactComponent (React.Component), isCompatTag and buildChildren.
var React = struct {
	IsReactComponent func(member any) bool
	IsCompatTag      func(tagName any) bool
	BuildChildren    func(node any) *jsvalue.Array
}{
	IsReactComponent: BuildMatchMemberExpression("React.Component"),
	IsCompatTag:      isCompatTag,
	BuildChildren:    buildChildren,
}

// AssertNode is assertNode(node): it panics with JavaScript's TypeError when node is not a node.
func AssertNode(node any) {
	if !IsNode(node) {
		var typ any = jsvalue.Undefined{}
		if !nullish(node) {
			typ = getProp(node, "type")
		}
		text := ""
		if nullish(typ) {
			text = jsonText(node)
		} else {
			text = toStr(typ)
		}
		panic(typeError(`Not a valid node of type "` + text + `"`))
	}
}

// assertType is the assert(type, node, opts) helper of the generated asserts.
func assertType(typ string, node any, opts any) {
	if !Is(typ, node, opts) {
		msg := `Expected type "` + typ + `" with option ` + jsonText(opts) + ", "
		panic(&jsvalue.Error{Message: msg + `but instead got "` + toStr(getProp(node, "type")) + `".`})
	}
}
