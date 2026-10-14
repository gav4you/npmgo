package server

import "github.com/gav4you/npmgo/pkgs/react"

// contextSnapshot is one pushed Provider value (React's context stack node).
type contextSnapshot struct {
	parent      *contextSnapshot
	depth       int
	context     *react.Context
	parentValue any
	value       any
}

// Context values live on the request rather than on the shared context
// object (JavaScript's context._currentValue / _currentValue2), so that
// concurrent renders do not interfere. The observable values are the same.

func (r *request) readContext(c *react.Context) any {
	if v, ok := r.contextValues[c]; ok {
		return v
	}
	return c.DefaultValue
}

func (r *request) setContextValue(c *react.Context, v any) { r.contextValues[c] = v }

func (r *request) popToNearestCommonAncestor(prev, next *contextSnapshot) {
	if prev == next {
		return
	}
	r.setContextValue(prev.context, prev.parentValue)
	prevParent := prev.parent
	nextParent := next.parent
	if prevParent == nil {
		if nextParent != nil {
			panic(newError("The stacks must reach the root at the same time. This is a bug in React."))
		}
	} else {
		if nextParent == nil {
			panic(newError("The stacks must reach the root at the same time. This is a bug in React."))
		}
		r.popToNearestCommonAncestor(prevParent, nextParent)
	}
	r.setContextValue(next.context, next.value)
}

func (r *request) popAllPrevious(prev *contextSnapshot) {
	r.setContextValue(prev.context, prev.parentValue)
	if prev.parent != nil {
		r.popAllPrevious(prev.parent)
	}
}

func (r *request) pushAllNext(next *contextSnapshot) {
	if next.parent != nil {
		r.pushAllNext(next.parent)
	}
	r.setContextValue(next.context, next.value)
}

func (r *request) popPreviousToCommonLevel(prev, next *contextSnapshot) {
	r.setContextValue(prev.context, prev.parentValue)
	prevParent := prev.parent
	if prevParent == nil {
		panic(newError("The depth must equal at least at zero before reaching the root. This is a bug in React."))
	}
	if prevParent.depth == next.depth {
		r.popToNearestCommonAncestor(prevParent, next)
	} else {
		r.popPreviousToCommonLevel(prevParent, next)
	}
}

func (r *request) popNextToCommonLevel(prev, next *contextSnapshot) {
	nextParent := next.parent
	if nextParent == nil {
		panic(newError("The depth must equal at least at zero before reaching the root. This is a bug in React."))
	}
	if prev.depth == nextParent.depth {
		r.popToNearestCommonAncestor(prev, nextParent)
	} else {
		r.popNextToCommonLevel(prev, nextParent)
	}
	r.setContextValue(next.context, next.value)
}

func (r *request) switchContext(newSnapshot *contextSnapshot) {
	prev := r.currentActiveSnapshot
	if prev == newSnapshot {
		return
	}
	switch {
	case prev == nil:
		r.pushAllNext(newSnapshot)
	case newSnapshot == nil:
		r.popAllPrevious(prev)
	case prev.depth == newSnapshot.depth:
		r.popToNearestCommonAncestor(prev, newSnapshot)
	case prev.depth > newSnapshot.depth:
		r.popPreviousToCommonLevel(prev, newSnapshot)
	default:
		r.popNextToCommonLevel(prev, newSnapshot)
	}
	r.currentActiveSnapshot = newSnapshot
}
