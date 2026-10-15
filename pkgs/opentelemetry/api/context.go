// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
//
// Translated to Go by the npmgo contributors from @opentelemetry/api 1.9.0 (build/src/context/context.js,
// context/NoopContextManager.js and api/context.js).

package api

import "github.com/gav4you/npmgo/internal/jsvalue"

// Context is an immutable map of values: setting or deleting a value returns a new context.
type Context interface {
	// GetValue returns the value under key, or undefined (jsvalue.Undefined{}).
	GetValue(key jsvalue.Symbol) any
	// SetValue returns a new context that inherits this one's values and holds value under key.
	SetValue(key jsvalue.Symbol, value any) Context
	// DeleteValue returns a new context that inherits this one's values without key.
	DeleteValue(key jsvalue.Symbol) Context
}

// CreateContextKey returns the key to store a value under: Symbol.for(description), so the same description gives
// the same key (the original does this so that several copies of a package share their keys).
func CreateContextKey(description string) jsvalue.Symbol { return jsvalue.SymbolFor(description) }

// baseContext is BaseContext: a copy of the parent's Map plus its own change.
type baseContext struct {
	current map[jsvalue.Symbol]any
}

func newBaseContext(parent map[jsvalue.Symbol]any) *baseContext {
	c := &baseContext{current: make(map[jsvalue.Symbol]any, len(parent)+1)}
	for k, v := range parent {
		c.current[k] = v
	}
	return c
}

func (c *baseContext) GetValue(key jsvalue.Symbol) any {
	if v, ok := c.current[key]; ok {
		return v
	}
	return jsvalue.Undefined{}
}

func (c *baseContext) SetValue(key jsvalue.Symbol, value any) Context {
	n := newBaseContext(c.current)
	n.current[key] = value
	return n
}

func (c *baseContext) DeleteValue(key jsvalue.Symbol) Context {
	n := newBaseContext(c.current)
	delete(n.current, key)
	return n
}

// ROOT_CONTEXT is the empty context, the default parent when no context is active.
var ROOT_CONTEXT Context = newBaseContext(nil)

// ContextManager keeps track of the active context (an implementation such as an async-hooks context manager is
// registered with Ctx.SetGlobalContextManager).
//
// JavaScript's with(context, fn, thisArg, ...args) has no thisArg here: Go functions have no receiver to bind.
type ContextManager interface {
	// Active returns the active context.
	Active() Context
	// With runs fn with context active and returns its result.
	With(context Context, fn func(args ...any) any, args ...any) any
	// Bind binds context to target (a function or an event emitter) and returns the result.
	Bind(context Context, target any) any
	// Enable enables the context manager.
	Enable() ContextManager
	// Disable disables the context manager.
	Disable() ContextManager
}

// noopContextManager is NoopContextManager: ROOT_CONTEXT is always active.
type noopContextManager struct{}

func (noopContextManager) Active() Context { return ROOT_CONTEXT }

func (noopContextManager) With(_ Context, fn func(args ...any) any, args ...any) any {
	return fn(args...)
}

func (noopContextManager) Bind(_ Context, target any) any { return target }

func (m noopContextManager) Enable() ContextManager { return m }

func (m noopContextManager) Disable() ContextManager { return m }

var noopContextManagerInstance ContextManager = noopContextManager{}

const contextAPIName = "context"

// ContextAPI is the context API (JavaScript's ContextAPI class; its instance is Ctx).
type ContextAPI struct{}

// SetGlobalContextManager registers the context manager and reports whether it was registered (a second
// registration fails, logging an error to Diag).
func (c *ContextAPI) SetGlobalContextManager(contextManager ContextManager) bool {
	return registerGlobal(contextAPIName, contextManager, Diag, false)
}

// Active returns the active context.
func (c *ContextAPI) Active() Context { return c.contextManager().Active() }

// With runs fn with context active (through the registered context manager) and returns its result.
func (c *ContextAPI) With(context Context, fn func(args ...any) any, args ...any) any {
	return c.contextManager().With(context, fn, args...)
}

// Bind binds context to target (a function or an event emitter) through the registered context manager.
func (c *ContextAPI) Bind(context Context, target any) any {
	return c.contextManager().Bind(context, target)
}

func (c *ContextAPI) contextManager() ContextManager {
	if m, ok := getGlobal(contextAPIName).(ContextManager); ok && !jsvalue.IsNull(m) {
		return m
	}
	return noopContextManagerInstance
}

// Disable disables the registered context manager and removes it.
func (c *ContextAPI) Disable() {
	c.contextManager().Disable()
	unregisterGlobal(contextAPIName, Diag)
}
