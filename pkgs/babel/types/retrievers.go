// Copyright (c) 2014-present Sebastian McKenzie and other contributors. MIT License (see LICENSE).
// Translated to Go by the npmgo contributors from @babel/types 7.29.0 (lib/retrievers/*.js).

package types

import "github.com/gav4you/npmgo/internal/jsvalue"

// putKey is ids[key] = v on an object without a prototype (Object.create(null)): a symbol key is a symbol-keyed
// property, "__proto__" an ordinary one.
func putKey(ids *jsvalue.Object, key any, v any) {
	if s, ok := key.(jsvalue.Symbol); ok {
		ids.SetSymbol(s, v)
		return
	}
	ids.Set(toStr(key), v)
}

// getKey is ids[key] on an object without a prototype.
func getKey(ids *jsvalue.Object, key any) any {
	if s, ok := key.(jsvalue.Symbol); ok {
		return ids.GetSymbol(s)
	}
	return ids.Get(toStr(key))
}

// GetAssignmentIdentifiers is getAssignmentIdentifiers(node): the identifiers node (or an array of nodes) assigns
// to, by name, in an object without a prototype.
func GetAssignmentIdentifiers(node any) *jsvalue.Object {
	search := concat(jsvalue.NewArray(), "[]", node).(*jsvalue.Array)
	ids := jsvalue.NewObject()
	for search.Len() > 0 {
		id := search.Pop()
		if !truthy(id) {
			continue
		}
		typ, _ := getProp(id, "type").(string)
		switch typ {
		case "ArrayPattern":
			search.Push(spreadItems(getProp(id, "elements"), "id.elements")...)
		case "AssignmentExpression", "AssignmentPattern", "ForInStatement", "ForOfStatement":
			search.Push(getProp(id, "left"))
		case "ObjectPattern":
			search.Push(spreadItems(getProp(id, "properties"), "id.properties")...)
		case "ObjectProperty":
			search.Push(getProp(id, "value"))
		case "RestElement", "UpdateExpression":
			search.Push(getProp(id, "argument"))
		case "UnaryExpression":
			if strictEquals(getProp(id, "operator"), "delete") {
				search.Push(getProp(id, "argument"))
			}
		case "Identifier":
			putKey(ids, getProp(id, "name"), id)
		}
	}
	return ids
}

// GetBindingIdentifiersKeys is getBindingIdentifiers.keys: for each node type, the keys that hold its bindings.
var GetBindingIdentifiersKeys = jsvalue.NewObject(
	"DeclareClass", jsvalue.NewArray("id"),
	"DeclareFunction", jsvalue.NewArray("id"),
	"DeclareModule", jsvalue.NewArray("id"),
	"DeclareVariable", jsvalue.NewArray("id"),
	"DeclareInterface", jsvalue.NewArray("id"),
	"DeclareTypeAlias", jsvalue.NewArray("id"),
	"DeclareOpaqueType", jsvalue.NewArray("id"),
	"InterfaceDeclaration", jsvalue.NewArray("id"),
	"TypeAlias", jsvalue.NewArray("id"),
	"OpaqueType", jsvalue.NewArray("id"),
	"CatchClause", jsvalue.NewArray("param"),
	"LabeledStatement", jsvalue.NewArray("label"),
	"UnaryExpression", jsvalue.NewArray("argument"),
	"AssignmentExpression", jsvalue.NewArray("left"),
	"ImportSpecifier", jsvalue.NewArray("local"),
	"ImportNamespaceSpecifier", jsvalue.NewArray("local"),
	"ImportDefaultSpecifier", jsvalue.NewArray("local"),
	"ImportDeclaration", jsvalue.NewArray("specifiers"),
	"TSImportEqualsDeclaration", jsvalue.NewArray("id"),
	"ExportSpecifier", jsvalue.NewArray("exported"),
	"ExportNamespaceSpecifier", jsvalue.NewArray("exported"),
	"ExportDefaultSpecifier", jsvalue.NewArray("exported"),
	"FunctionDeclaration", jsvalue.NewArray("id", "params"),
	"FunctionExpression", jsvalue.NewArray("id", "params"),
	"ArrowFunctionExpression", jsvalue.NewArray("params"),
	"ObjectMethod", jsvalue.NewArray("params"),
	"ClassMethod", jsvalue.NewArray("params"),
	"ClassPrivateMethod", jsvalue.NewArray("params"),
	"ForInStatement", jsvalue.NewArray("left"),
	"ForOfStatement", jsvalue.NewArray("left"),
	"ClassDeclaration", jsvalue.NewArray("id"),
	"ClassExpression", jsvalue.NewArray("id"),
	"RestElement", jsvalue.NewArray("argument"),
	"UpdateExpression", jsvalue.NewArray("argument"),
	"ObjectProperty", jsvalue.NewArray("value"),
	"AssignmentPattern", jsvalue.NewArray("left"),
	"ArrayPattern", jsvalue.NewArray("elements"),
	"ObjectPattern", jsvalue.NewArray("properties"),
	"VariableDeclaration", jsvalue.NewArray("declarations"),
	"VariableDeclarator", jsvalue.NewArray("id"),
)

// GetBindingIdentifiers is getBindingIdentifiers(node, duplicates, outerOnly, newBindingsOnly): the identifiers node
// (or an array of nodes) binds, by name, in an object without a prototype; with duplicates each name maps to an array
// of all of them. Pass the optional flags in order.
func GetBindingIdentifiers(node any, flags ...bool) *jsvalue.Object {
	flag := func(i int) bool { return i < len(flags) && flags[i] }
	duplicates, outerOnly, newBindingsOnly := flag(0), flag(1), flag(2)
	search := concat(jsvalue.NewArray(), "[]", node).(*jsvalue.Array)
	ids := jsvalue.NewObject()
	for search.Len() > 0 {
		id := search.Shift()
		if !truthy(id) {
			continue
		}
		if newBindingsOnly && (IsAssignmentExpression(id) || IsUnaryExpression(id) || IsUpdateExpression(id)) {
			continue
		}
		if IsIdentifier(id) {
			name := getProp(id, "name")
			if duplicates {
				list := getKey(ids, name)
				if !truthy(list) {
					list = jsvalue.NewArray()
				}
				putKey(ids, name, list)
				pushOnto(list, "_ids", id)
			} else {
				putKey(ids, name, id)
			}
			continue
		}
		if IsExportDeclaration(id) && !IsExportAllDeclaration(id) {
			if IsDeclaration(getProp(id, "declaration")) {
				search.Push(getProp(id, "declaration"))
			}
			continue
		}
		if outerOnly {
			if IsFunctionDeclaration(id) {
				search.Push(getProp(id, "id"))
				continue
			}
			if IsFunctionExpression(id) {
				continue
			}
		}
		keys := tableGet(GetBindingIdentifiersKeys, getProp(id, "type"))
		if truthy(keys) {
			for i := 0; float64(i) < jsvalue.ToNumber(getProp(keys, "length")); i++ {
				key := getIndex(keys, i)
				nodes := getPropKey(id, key)
				if truthy(nodes) {
					if isArray(nodes) {
						search.Push(items(nodes)...)
					} else {
						search.Push(nodes)
					}
				}
			}
		}
	}
	return ids
}

// pushOnto is list.push(v).
func pushOnto(list any, expr string, v any) {
	if a, ok := list.(*jsvalue.Array); ok && a != nil {
		a.Push(v)
		return
	}
	if nullish(list) {
		panic(readError(list, "push"))
	}
	panic(typeError(expr + ".push is not a function"))
}

// GetOuterBindingIdentifiers is getOuterBindingIdentifiers(node, duplicates): getBindingIdentifiers without
// descending into functions.
func GetOuterBindingIdentifiers(node any, duplicates ...bool) *jsvalue.Object {
	return GetBindingIdentifiers(node, len(duplicates) > 0 && duplicates[0], true)
}

func getNameFromLiteralID(id any) any {
	if IsNullLiteral(id) {
		return "null"
	}
	if IsRegExpLiteral(id) {
		return "/" + toStr(getProp(id, "pattern")) + "/" + toStr(getProp(id, "flags"))
	}
	if IsTemplateLiteral(id) {
		quasis := getProp(id, "quasis")
		methodTarget(quasis, "map", "id.quasis")
		raws := make([]any, 0, len(items(quasis)))
		for _, quasi := range items(quasis) {
			raws = append(raws, getProp(getProp(quasi, "value"), "raw"))
		}
		return jsvalue.Join(raws, "")
	}
	if v := getProp(id, "value"); !jsvalue.IsUndefined(v) {
		return jsvalue.String(v)
	}
	return nil
}

func getObjectMemberKey(node any) any {
	if !truthy(getProp(node, "computed")) || IsLiteral(getProp(node, "key")) {
		return getProp(node, "key")
	}
	return jsvalue.Undefined{}
}

// GetFunctionName is getFunctionName(node, parent): the name of a function or class ({name, originalNode}) from its
// id, its key as a method, or the property, variable or assignment it is the value of; nil (null) when it has none.
func GetFunctionName(node any, parent ...any) *jsvalue.Object {
	p := optArg(parent)
	if hasIn(node, "id") && truthy(getProp(node, "id")) {
		return jsvalue.NewObject("name", getProp(getProp(node, "id"), "name"), "originalNode", getProp(node, "id"))
	}
	prefix := ""
	var id any = jsvalue.Undefined{}
	if IsObjectProperty(p, jsvalue.NewObject("value", node)) {
		id = getObjectMemberKey(p)
	} else if IsObjectMethod(node) || IsClassMethod(node) {
		id = getObjectMemberKey(node)
		if kind := getProp(node, "kind"); strictEquals(kind, "get") {
			prefix = "get "
		} else if strictEquals(kind, "set") {
			prefix = "set "
		}
	} else if IsVariableDeclarator(p, jsvalue.NewObject("init", node)) {
		id = getProp(p, "id")
	} else if IsAssignmentExpression(p, jsvalue.NewObject("operator", "=", "right", node)) {
		id = getProp(p, "left")
	}
	if !truthy(id) {
		return nil
	}
	var name any
	switch {
	case IsLiteral(id):
		name = getNameFromLiteralID(id)
	case IsIdentifier(id):
		name = getProp(id, "name")
	case IsPrivateName(id):
		name = getProp(getProp(id, "id"), "name")
	default:
		name = nil
	}
	if nullish(name) {
		return nil
	}
	return jsvalue.NewObject("name", prefix+toStr(name), "originalNode", id)
}
