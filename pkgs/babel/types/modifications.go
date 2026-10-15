// Copyright (c) 2014-present Sebastian McKenzie and other contributors. MIT License (see LICENSE).
// Translated to Go by the npmgo contributors from @babel/types 7.29.0 (lib/modifications/*.js,
// lib/modifications/flow/removeTypeDuplicates.js, lib/modifications/typescript/removeTypeDuplicates.js).

package types

import (
	"strings"

	"github.com/gav4you/npmgo/internal/jsvalue"
)

// AppendToMemberExpression is appendToMemberExpression(member, append, computed = false): member becomes
// member.append (member's old object and property move into a new member expression), in place.
func AppendToMemberExpression(member, appendNode any, computed ...bool) any {
	setProp(member, "object", MemberExpression(getProp(member, "object"), getProp(member, "property"), getProp(member, "computed")))
	setProp(member, "property", appendNode)
	setProp(member, "computed", len(computed) > 0 && computed[0])
	return member
}

// Inherits is inherits(child, parent): child takes parent's typeAnnotation, typeParameters and returnType where it
// has none, its properties whose names start with an underscore (but __clone), its start, loc and end, and its
// comments.
func Inherits(child, parent any) any {
	if !truthy(child) || !truthy(parent) {
		return child
	}
	for _, key := range items(getProp(INHERIT_KEYS, "optional")) {
		k := toStr(key)
		if nullish(getProp(child, k)) {
			setProp(child, k, getProp(parent, k))
		}
	}
	for _, key := range objectKeys(parent) {
		if strings.HasPrefix(key, "_") && key != "__clone" {
			setProp(child, key, getProp(parent, key))
		}
	}
	for _, key := range items(getProp(INHERIT_KEYS, "force")) {
		k := toStr(key)
		setProp(child, k, getProp(parent, k))
	}
	InheritsComments(child, parent)
	return child
}

// PrependToMemberExpression is prependToMemberExpression(member, prepend): member becomes prepend.member, in place.
// A super property access panics with JavaScript's Error.
func PrependToMemberExpression(member, prepend any) any {
	if IsSuper(getProp(member, "object")) {
		panic(&jsvalue.Error{Message: "Cannot prepend node to super property access (`super.foo`)."})
	}
	setProp(member, "object", MemberExpression(prepend, getProp(member, "object")))
	return member
}

var clearKeys = []string{"tokens", "start", "end", "loc", "raw", "rawValue"}

// clearKeysPlusComments is removeProperties.js's CLEAR_KEYS_PLUS_COMMENTS, made from COMMENT_KEYS when the module
// loads.
var clearKeysPlusComments = func() []string {
	var keys []string
	for _, k := range items(COMMENT_KEYS) {
		keys = append(keys, toStr(k))
	}
	return append(append(keys, "comments"), clearKeys...)
}()

// RemoveProperties is removeProperties(node, opts = {}): the location, token, raw and (unless opts.preserveComments)
// comment properties, and those whose names start with an underscore, are set to undefined where not null or
// undefined; symbol-keyed properties are set to null.
func RemoveProperties(node any, opts ...any) {
	o := optArg(opts)
	if jsvalue.IsUndefined(o) {
		o = jsvalue.NewObject()
	}
	keys := clearKeysPlusComments
	if truthy(getProp(o, "preserveComments")) {
		keys = clearKeys
	}
	for _, key := range keys {
		if !nullish(getProp(node, key)) {
			setProp(node, key, jsvalue.Undefined{})
		}
	}
	for _, key := range objectKeys(node) {
		if strings.HasPrefix(key, "_") && !nullish(getProp(node, key)) {
			setProp(node, key, jsvalue.Undefined{})
		}
	}
	if obj, ok := node.(*jsvalue.Object); ok && obj != nil {
		for _, sym := range obj.SymbolKeys() {
			obj.SetSymbol(sym, nil)
		}
	}
}

// RemovePropertiesDeep is removePropertiesDeep(tree, opts): removeProperties on every node of the tree.
func RemovePropertiesDeep(tree any, opts ...any) any {
	TraverseFast(tree, func(node, o any) any {
		RemoveProperties(node, o)
		return jsvalue.Undefined{}
	}, opts...)
	return tree
}

// flowQualifiedName is the flow removeTypeDuplicates' getQualifiedName.
func flowQualifiedName(node any) any {
	if IsIdentifier(node) {
		return getProp(node, "name")
	}
	return toStr(getProp(getProp(node, "id"), "name")) + "." + toStr(flowQualifiedName(getProp(node, "qualification")))
}

// RemoveTypeDuplicates is removeTypeDuplicates(nodes) for flow types: the distinct types, with nested union types
// flattened, the base annotations and the generic types (merging their type parameters) last; an any type alone.
func RemoveTypeDuplicates(nodesIn any) *jsvalue.Array {
	return removeTypeDuplicates(nodesIn, typeDuplicatesFlow)
}

// typeDuplicatesFlavour is what differs between the flow and the TypeScript removeTypeDuplicates.
type typeDuplicatesFlavour struct {
	ts                     bool
	isAny, isBase, isUnion func(node any, opts ...any) bool
	// generic reports a generic type and its qualified name (TypeScript: a type reference with type arguments).
	generic func(node any) (name any, ok bool)
	// The expressions V8 names in its TypeErrors.
	pushExpr, spreadExpr string
}

var typeDuplicatesFlow = typeDuplicatesFlavour{
	isAny:   IsAnyTypeAnnotation,
	isBase:  IsFlowBaseAnnotation,
	isUnion: IsUnionTypeAnnotation,
	generic: func(node any) (any, bool) {
		if !IsGenericTypeAnnotation(node) {
			return nil, false
		}
		return flowQualifiedName(getProp(node, "id")), true
	},
	pushExpr:   "existing.typeParameters.params.push",
	spreadExpr: "node.typeParameters.params",
}

// tsQualifiedName is the TypeScript removeTypeDuplicates' getQualifiedName.
func tsQualifiedName(node any) any {
	if IsIdentifier(node) {
		return getProp(node, "name")
	}
	if IsThisExpression(node) {
		return "this"
	}
	return toStr(getProp(getProp(node, "right"), "name")) + "." + toStr(tsQualifiedName(getProp(node, "left")))
}

var typeDuplicatesTS = typeDuplicatesFlavour{
	ts:      true,
	isAny:   IsTSAnyKeyword,
	isBase:  IsTSBaseType,
	isUnion: IsTSUnionType,
	generic: func(node any) (any, bool) {
		if !IsTSTypeReference(node) || !truthy(getProp(node, "typeParameters")) {
			return nil, false
		}
		return tsQualifiedName(getProp(node, "typeName")), true
	},
	pushExpr:   "existingTypeArguments.params.push",
	spreadExpr: "typeArguments.params",
}

// removeTypeDuplicates is the shared body of the two removeTypeDuplicates.
func removeTypeDuplicates(nodesIn any, f typeDuplicatesFlavour) *jsvalue.Array {
	nodes := arrayFrom(nodesIn)
	generics := newJSMap()
	bases := newJSMap()
	typeGroups := newJSMap()
	types := &jsvalue.Array{}
	for i := 0; i < len(nodes); i++ {
		node := nodes[i]
		if !truthy(node) {
			continue
		}
		if types.Includes(node) {
			continue
		}
		if f.isAny(node) {
			return jsvalue.NewArray(node)
		}
		if f.isBase(node) {
			bases.set(getProp(node, "type"), node)
			continue
		}
		if f.isUnion(node) {
			group := getProp(node, "types")
			if !typeGroups.has(group) {
				nodes = append(nodes, spreadItems(group, "node.types")...)
				typeGroups.set(group, nil)
			}
			continue
		}
		if name, ok := f.generic(node); ok {
			if generics.has(name) {
				existing, _ := generics.get(name)
				if f.ts {
					typeArguments := getProp(node, "typeParameters")
					if existingTypeArguments := getProp(existing, "typeParameters"); truthy(existingTypeArguments) {
						mergeParams(existingTypeArguments, typeArguments, f)
					}
				} else if truthy(getProp(existing, "typeParameters")) {
					if truthy(getProp(node, "typeParameters")) {
						mergeParams(getProp(existing, "typeParameters"), getProp(node, "typeParameters"), f)
					}
				}
			} else {
				generics.set(name, node)
			}
			continue
		}
		types.Push(node)
	}
	for _, baseType := range bases.vals {
		types.Push(baseType)
	}
	for _, genericName := range generics.vals {
		types.Push(genericName)
	}
	return types
}

// mergeParams is `existing.params.push(...other.params); existing.params = removeTypeDuplicates(existing.params)`:
// the push is on existing's own array, which every holder of it sees (a Go slice is replaced by a longer one).
func mergeParams(existing, other any, f typeDuplicatesFlavour) {
	params := getProp(existing, "params")
	if nullish(params) {
		panic(readError(params, "push")) // the callee is read before the arguments
	}
	added := spreadItems(getProp(other, "params"), f.spreadExpr)
	if a, ok := params.(*jsvalue.Array); ok {
		a.Push(added...)
	} else if isArray(params) {
		params = append(append([]any{}, items(params)...), added...)
	} else {
		panic(typeError(f.pushExpr + " is not a function"))
	}
	setProp(existing, "params", removeTypeDuplicates(params, f))
}
