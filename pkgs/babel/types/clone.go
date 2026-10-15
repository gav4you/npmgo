// Copyright (c) 2014-present Sebastian McKenzie and other contributors. MIT License (see LICENSE).
// Translated to Go by the npmgo contributors from @babel/types 7.29.0 (lib/clone/*.js).

package types

import "github.com/gav4you/npmgo/internal/jsvalue"

func cloneIfNode(obj any, deep, withoutLoc bool, commentsCache *jsMap, depth int) any {
	if truthy(obj) {
		if _, ok := getProp(obj, "type").(string); ok {
			return cloneNodeInternal(obj, deep, withoutLoc, commentsCache, depth)
		}
	}
	return obj
}

func cloneIfNodeOrArray(obj any, deep, withoutLoc bool, commentsCache *jsMap, depth int) any {
	if isArray(obj) {
		out := &jsvalue.Array{}
		for _, node := range items(obj) {
			out.Push(cloneIfNode(node, deep, withoutLoc, commentsCache, depth))
		}
		return out
	}
	return cloneIfNode(obj, deep, withoutLoc, commentsCache, depth)
}

// CloneNode is cloneNode(node, deep = true, withoutLoc = false): a copy of node with its fields (cloned too when
// deep), loc (null when withoutLoc), comments (cloned once each when deep) and a copy of extra. Pass the optional
// arguments in order: CloneNode(node, false) is a shallow clone, CloneNode(node, true, true) a deep one without loc.
// A falsy node is returned as it is; an unknown node type panics with JavaScript's Error, and a cyclic tree with its
// RangeError (maxCallDepth).
func CloneNode(node any, opts ...bool) any {
	deep, withoutLoc := true, false
	if len(opts) > 0 {
		deep = opts[0]
	}
	if len(opts) > 1 {
		withoutLoc = opts[1]
	}
	return cloneNodeInternal(node, deep, withoutLoc, newJSMap(), 0)
}

func cloneNodeInternal(node any, deep, withoutLoc bool, commentsCache *jsMap, depth int) any {
	depth = deeper(depth)
	if !truthy(node) {
		return node
	}
	typ := getProp(node, "type")
	newNode := jsvalue.NewObject("type", getProp(node, "type"))
	if IsIdentifier(node) {
		newNode.Set("name", getProp(node, "name"))
		if hasOwn(node, "optional") {
			if b, ok := getProp(node, "optional").(bool); ok {
				newNode.Set("optional", b)
			}
		}
		if hasOwn(node, "typeAnnotation") {
			if deep {
				newNode.Set("typeAnnotation", cloneIfNodeOrArray(getProp(node, "typeAnnotation"), true, withoutLoc, commentsCache, depth))
			} else {
				newNode.Set("typeAnnotation", getProp(node, "typeAnnotation"))
			}
		}
		if hasOwn(node, "decorators") {
			if deep {
				newNode.Set("decorators", cloneIfNodeOrArray(getProp(node, "decorators"), true, withoutLoc, commentsCache, depth))
			} else {
				newNode.Set("decorators", getProp(node, "decorators"))
			}
		}
	} else if k, ok := propertyKey(typ); !ok || !NODE_FIELDS.Has(k) {
		panic(&jsvalue.Error{Message: `Unknown node type: "` + toStr(typ) + `"`})
	} else {
		for _, field := range objectKeys(NODE_FIELDS.Get(k)) {
			if hasOwn(node, field) {
				if deep {
					if IsFile(node) && field == "comments" {
						newNode.Set(field, maybeCloneComments(getProp(node, "comments"), deep, withoutLoc, commentsCache))
					} else {
						newNode.Set(field, cloneIfNodeOrArray(getProp(node, field), true, withoutLoc, commentsCache, depth))
					}
				} else {
					newNode.Set(field, getProp(node, field))
				}
			}
		}
	}
	if hasOwn(node, "loc") {
		if withoutLoc {
			newNode.Set("loc", nil)
		} else {
			newNode.Set("loc", getProp(node, "loc"))
		}
	}
	for _, key := range []string{"leadingComments", "innerComments", "trailingComments"} {
		if hasOwn(node, key) {
			newNode.Set(key, maybeCloneComments(getProp(node, key), deep, withoutLoc, commentsCache))
		}
	}
	if hasOwn(node, "extra") {
		newNode.Set("extra", objectAssign(jsvalue.NewObject(), getProp(node, "extra")))
	}
	return newNode
}

func maybeCloneComments(comments any, deep, withoutLoc bool, commentsCache *jsMap) any {
	if !truthy(comments) || !deep {
		return comments
	}
	methodTarget(comments, "map", "comments")
	out := &jsvalue.Array{}
	for _, comment := range items(comments) {
		if cached, _ := commentsCache.get(comment); truthy(cached) {
			out.Push(cached)
			continue
		}
		if nullish(comment) {
			what := "null"
			if jsvalue.IsUndefined(comment) {
				what = "undefined"
			}
			panic(typeError("Cannot destructure property 'type' of 'comment' as it is " + what + "."))
		}
		ret := jsvalue.NewObject("type", getProp(comment, "type"), "value", getProp(comment, "value"), "loc", getProp(comment, "loc"))
		if withoutLoc {
			ret.Set("loc", nil)
		}
		commentsCache.set(comment, ret)
		out.Push(ret)
	}
	return out
}

// objectAssign is Object.assign(target, source) for one source: an object's own enumerable properties (string keys,
// then symbols), an array's elements and a string's characters by index; null, undefined and other primitives add
// nothing.
func objectAssign(target *jsvalue.Object, source any) *jsvalue.Object {
	switch x := source.(type) {
	case *jsvalue.Object:
		return target.Assign(x)
	case string:
		for i, u := range utf16Units(x) {
			target.Set(itoa(i), unitString(u))
		}
		return target
	}
	if isArray(source) {
		for i, v := range items(source) {
			target.Set(itoa(i), v)
		}
	}
	return target
}

// Clone is clone(node): a shallow cloneNode.
func Clone(node any) any { return CloneNode(node, false) }

// CloneDeep is cloneDeep(node): a deep cloneNode.
func CloneDeep(node any) any { return CloneNode(node) }

// CloneDeepWithoutLoc is cloneDeepWithoutLoc(node): a deep cloneNode with loc set to null.
func CloneDeepWithoutLoc(node any) any { return CloneNode(node, true, true) }

// CloneWithoutLoc is cloneWithoutLoc(node): a shallow cloneNode with loc set to null.
func CloneWithoutLoc(node any) any { return CloneNode(node, false, true) }
