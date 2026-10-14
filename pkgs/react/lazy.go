package react

import (
	"sync"

	"github.com/gav4you/npmgo/internal/jsloop"
	"github.com/gav4you/npmgo/internal/jsvalue"
)

// LazyType is what Lazy returns. Its state is shared by every render that
// uses it, like the JavaScript payload object.
type LazyType struct {
	DisplayName string

	initMu sync.Mutex
	mu     sync.Mutex
	status int // -1 uninitialized, 0 pending, 1 resolved, 2 rejected
	load   func() Thenable
	result any // the thenable while pending, the module once resolved, the reason once rejected
}

// Lazy is React.lazy(load). load is called the first time the component
// renders; the Thenable it returns resolves to a module: an *Object whose
// "default" property is the component (as `import()` resolves). Like React,
// a module that is not an object yields an undefined component (an "Element
// type is invalid" error when rendered).
func Lazy(load func() Thenable) *LazyType {
	return &LazyType{status: -1, load: load}
}

// Resolved returns the component (the module's "default" export) once the
// loader's thenable has resolved to a module Init can read. It never triggers
// loading and never panics.
func (l *LazyType) Resolved() (component any, ok bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.status != 1 || jsvalue.IsNullish(l.result) {
		return nil, false
	}
	return moduleDefault(l.result), true
}

// moduleDefault is `module.default`.
func moduleDefault(module any) any {
	if m, ok := module.(*Object); ok && m != nil {
		return m.Get("default")
	}
	if jsvalue.IsNullish(module) {
		panic(&jsvalue.TypeError{Message: "Cannot read properties of " + jsvalue.ToString(module) + " (reading 'default')"})
	}
	return Undefined{}
}

// Init is the lazy type's _init (React's lazyInitializer). It returns the
// component (the resolved module's "default" property) or panics with what
// JavaScript throws: the pending thenable, or the rejection reason. q is the
// caller's microtask queue (nil outside a render).
func (l *LazyType) Init(q Microtasks) any {
	l.initMu.Lock()
	l.mu.Lock()
	status := l.status
	l.mu.Unlock()
	if status == -1 {
		func() {
			defer l.initMu.Unlock()
			t := l.load()
			if jsvalue.IsNull(t) {
				panic(&jsvalue.TypeError{Message: "Cannot read properties of undefined (reading 'then')"})
			}
			jsloop.Then(q, t, func(module any) {
				l.mu.Lock()
				if l.status == 0 || l.status == -1 {
					l.status, l.result = 1, module
				}
				l.mu.Unlock()
			}, func(err any) {
				l.mu.Lock()
				if l.status == 0 || l.status == -1 {
					l.status, l.result = 2, err
				}
				l.mu.Unlock()
			})
			l.mu.Lock()
			if l.status == -1 {
				l.status, l.result = 0, t
			}
			l.mu.Unlock()
		}()
	} else {
		l.initMu.Unlock()
	}
	l.mu.Lock()
	status, result := l.status, l.result
	l.mu.Unlock()
	if status == 1 {
		return moduleDefault(result)
	}
	panic(Throw(result))
}

// thrownNull stands for a JavaScript `throw null` / `throw undefined`, which a
// Go panic cannot carry directly.
type thrownNull struct{ undefined bool }

// Throw returns the panic value that throws v the way JavaScript `throw v`
// does (Go cannot panic with nil).
func Throw(v any) any {
	if v == nil {
		return thrownNull{}
	}
	if jsvalue.IsUndefined(v) {
		return thrownNull{undefined: true}
	}
	return v
}

// Thrown converts a recovered panic value back into the JavaScript value
// that was thrown.
func Thrown(r any) any {
	switch x := r.(type) {
	case thrownNull:
		if x.undefined {
			return Undefined{}
		}
		return nil
	case *Panic:
		return x.Value
	}
	if isPanicNil(r) {
		return nil
	}
	return r
}

// Panic wraps a value a component threw, for callers that need an error.
type Panic struct{ Value any }

func (p *Panic) Error() string { return jsvalue.String(p.Value) }
