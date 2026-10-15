// Copyright (c) 2014-present Sebastian McKenzie and other contributors. MIT License (see LICENSE).
// Translated to Go by the npmgo contributors from @babel/types 7.29.0 (lib/traverse/*.js).

package types

import "github.com/gav4you/npmgo/internal/jsvalue"

// TraversalHandler is a handler of Traverse: the node, its ancestors (an array of {node, key, index} objects, the
// innermost last; the array is the one traverse keeps, so it changes as the traversal goes on) and the state.
type TraversalHandler func(node any, ancestors *jsvalue.Array, state any)

// TraversalHandlers are the enter and exit handlers of Traverse; either may be nil.
type TraversalHandlers struct {
	Enter, Exit TraversalHandler
}

// Traverse is traverse(node, handlers, state): a depth-first walk of node's children by VISITOR_KEYS, calling enter
// before and exit after each node's children. handlers is a TraversalHandler (or a func of its signature) for enter
// alone, TraversalHandlers or *TraversalHandlers, or a *jsvalue.Object with enter and exit functions. A cyclic tree
// panics with JavaScript's RangeError (maxCallDepth).
func Traverse(node any, handlers any, state ...any) {
	var enter, exit any = jsvalue.Undefined{}, jsvalue.Undefined{}
	switch h := handlers.(type) {
	case TraversalHandler:
		enter = h
	case func(any, *jsvalue.Array, any):
		enter = TraversalHandler(h)
	case TraversalHandlers:
		enter, exit = handlerValue(h.Enter), handlerValue(h.Exit)
	case *TraversalHandlers:
		if h != nil {
			enter, exit = handlerValue(h.Enter), handlerValue(h.Exit)
			break
		}
		panic(typeError("Cannot destructure property 'enter' of 'handlers' as it is null."))
	default:
		if nullish(handlers) {
			what := "null"
			if jsvalue.IsUndefined(handlers) {
				what = "undefined"
			}
			panic(typeError("Cannot destructure property 'enter' of 'handlers' as it is " + what + "."))
		}
		enter, exit = getProp(handlers, "enter"), getProp(handlers, "exit")
	}
	traverseSimpleImpl(node, enter, exit, optArg(state), &jsvalue.Array{}, 0)
}

// handlerValue is a handler as a JavaScript value: a nil handler is undefined.
func handlerValue(h TraversalHandler) any {
	if h == nil {
		return jsvalue.Undefined{}
	}
	return h
}

func callHandler(h any, name string, node any, ancestors *jsvalue.Array, state any) {
	if th, ok := h.(TraversalHandler); ok {
		th(node, ancestors, state)
		return
	}
	callValue(h, name, node, ancestors, state)
}

func traverseSimpleImpl(node, enter, exit, state any, ancestors *jsvalue.Array, depth int) {
	depth = deeper(depth)
	keys := tableGet(VISITOR_KEYS, getProp(node, "type"))
	if !truthy(keys) {
		return
	}
	if truthy(enter) {
		callHandler(enter, "enter", node, ancestors, state)
	}
	for _, key := range iterate(keys, "keys") {
		k := toStr(key)
		subNode := getProp(node, k)
		if isArray(subNode) {
			for i := 0; i < len(items(subNode)); i++ {
				child := items(subNode)[i]
				if !truthy(child) {
					continue
				}
				ancestors.Push(jsvalue.NewObject("node", node, "key", key, "index", float64(i)))
				traverseSimpleImpl(child, enter, exit, state, ancestors, depth)
				ancestors.Pop()
			}
		} else if truthy(subNode) {
			ancestors.Push(jsvalue.NewObject("node", node, "key", key))
			traverseSimpleImpl(subNode, enter, exit, state, ancestors, depth)
			ancestors.Pop()
		}
	}
	if truthy(exit) {
		callHandler(exit, "exit", node, ancestors, state)
	}
}

// TraverseFastSkip and TraverseFastStop are traverseFast.skip and traverseFast.stop: an enter function returns one
// to skip the node's children, or to stop the whole traversal.
var (
	TraverseFastSkip = jsvalue.NewSymbol("")
	TraverseFastStop = jsvalue.NewSymbol("")
)

// TraverseFast is traverseFast(node, enter, opts): a depth-first walk by VISITOR_KEYS calling enter(node, opts) on
// each node (opts is a new empty object when not given or falsy). It reports whether enter returned
// TraverseFastStop. A cyclic tree panics with JavaScript's RangeError (maxCallDepth).
func TraverseFast(node any, enter func(node any, opts any) any, opts ...any) bool {
	return traverseFast(node, enter, optArg(opts), 0)
}

func traverseFast(node any, enter func(node any, opts any) any, opts any, depth int) bool {
	depth = deeper(depth)
	if !truthy(node) {
		return false
	}
	keys := tableGet(VISITOR_KEYS, getProp(node, "type"))
	if !truthy(keys) {
		return false
	}
	if !truthy(opts) {
		opts = jsvalue.NewObject()
	}
	if enter == nil {
		panic(typeError("enter is not a function"))
	}
	ret := enter(node, opts)
	if ret != nil && !jsvalue.IsUndefined(ret) {
		if s, ok := ret.(jsvalue.Symbol); ok {
			switch s {
			case TraverseFastSkip:
				return false
			case TraverseFastStop:
				return true
			}
		}
	}
	for _, key := range iterate(keys, "keys") {
		subNode := getPropKey(node, key)
		if !truthy(subNode) {
			continue
		}
		if isArray(subNode) {
			for i := 0; i < len(items(subNode)); i++ {
				if traverseFast(items(subNode)[i], enter, opts, depth) {
					return true
				}
			}
		} else if traverseFast(subNode, enter, opts, depth) {
			return true
		}
	}
	return false
}
