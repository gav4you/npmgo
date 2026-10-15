// Copyright (c) 2014-present Sebastian McKenzie and other contributors. MIT License (see LICENSE).
// Translated to Go by the npmgo contributors from @babel/types 7.29.0 (lib/comments/*.js, lib/utils/inherit.js).

package types

import "github.com/gav4you/npmgo/internal/jsvalue"

// AddComment is addComment(node, type, content, line): it adds one comment ({type: "CommentLine" or "CommentBlock",
// value: content}) to node's leading, inner or trailing comments (type "leading", "inner" or "trailing").
func AddComment(node any, typ string, content any, line ...bool) any {
	commentType := "CommentBlock"
	if len(line) > 0 && line[0] {
		commentType = "CommentLine"
	}
	return AddComments(node, typ, jsvalue.NewArray(jsvalue.NewObject("type", commentType, "value", content)))
}

// AddComments is addComments(node, type, comments): leading comments are put before the node's own, the others after
// them (pushed onto the node's own array, which every holder of that array sees), and a node without any gets the
// comments array itself. A Go slice cannot be pushed onto: the node gets the longer slice instead.
func AddComments(node any, typ string, comments any) any {
	if !truthy(comments) || !truthy(node) {
		return node
	}
	key := typ + "Comments"
	existing := getProp(node, key)
	if truthy(existing) {
		if typ == "leading" {
			setProp(node, key, concat(comments, "comments", existing))
		} else {
			// node[key].push(...comments): the spread is evaluated before the call is attempted.
			added := spreadItems(comments, "comments")
			if a, ok := existing.(*jsvalue.Array); ok {
				a.Push(added...)
			} else if isArray(existing) {
				setProp(node, key, append(append([]any{}, items(existing)...), added...))
			} else {
				panic(typeError("node[key].push is not a function"))
			}
		}
	} else {
		setProp(node, key, comments)
	}
	return node
}

// concat is v.concat(...args): a new array of v's elements and each argument (an array's elements); a string
// concatenates strings.
func concat(v any, expr string, args ...any) any {
	if s, ok := v.(string); ok {
		for _, a := range args {
			s += toStr(a)
		}
		return s
	}
	methodTarget(v, "concat", expr)
	out := jsvalue.NewArray(items(v)...)
	for _, a := range args {
		if isArray(a) {
			out.Push(items(a)...)
		} else {
			out.Push(a)
		}
	}
	return out
}

// inherit is lib/utils/inherit.js: child[key] becomes the distinct truthy comments of child and parent, in order.
func inherit(key string, child, parent any) {
	if truthy(child) && truthy(parent) {
		var all []any
		for _, v := range []any{getProp(child, key), getProp(parent, key)} {
			if isArray(v) {
				all = append(all, items(v)...)
			} else {
				all = append(all, v)
			}
		}
		seen := newJSMap()
		out := &jsvalue.Array{}
		for _, v := range all {
			if truthy(v) && !seen.has(v) {
				seen.set(v, nil)
				out.Push(v)
			}
		}
		setProp(child, key, out)
	}
}

// InheritInnerComments is inheritInnerComments(child, parent).
func InheritInnerComments(child, parent any) { inherit("innerComments", child, parent) }

// InheritLeadingComments is inheritLeadingComments(child, parent).
func InheritLeadingComments(child, parent any) { inherit("leadingComments", child, parent) }

// InheritTrailingComments is inheritTrailingComments(child, parent).
func InheritTrailingComments(child, parent any) { inherit("trailingComments", child, parent) }

// InheritsComments is inheritsComments(child, parent): child gets parent's trailing, leading and inner comments too.
func InheritsComments(child, parent any) any {
	InheritTrailingComments(child, parent)
	InheritLeadingComments(child, parent)
	InheritInnerComments(child, parent)
	return child
}

// RemoveComments is removeComments(node): its leading, trailing and inner comments are set to null.
func RemoveComments(node any) any {
	for _, key := range items(COMMENT_KEYS) {
		setProp(node, toStr(key), nil)
	}
	return node
}
