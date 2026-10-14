// Package react is a Go translation of the npm package react@19.2.4
// (https://github.com/facebook/react, packages/react; behaviour taken from the
// production build cjs/react.production.js). The original copyright notice
// and MIT license are preserved in LICENSE.
//
// # Values
//
// React works on JavaScript values. They are modelled as follows: undefined
// is Undefined{}; null is nil (or a nil pointer, func or map); booleans,
// strings and all Go number types are themselves (numbers behave as float64);
// *big.Int is a bigint; Symbol is a symbol; slices are arrays; *Object is an
// ordered plain object (property order follows JavaScript's own-key order,
// which is what fixes attribute and style order in the HTML); Go funcs are
// functions. Props are an *Object (Props is an alias).
//
// # Elements
//
// CreateElement(type, config, children...) and CloneElement mirror their
// JavaScript namesakes, including key extraction and children arity: no
// child argument leaves props.children unset, one sets it to that value, two
// or more set it to a []any. A Node is anything renderable: nil, Undefined,
// bool, string, numbers, *big.Int, *Element, slices of nodes, an Iterable,
// a Thenable (rendered once it resolves), a *LazyType, or a *Context (renders
// its current value).
//
// # Components
//
// An element type is a host tag (string), one of the symbols Fragment,
// StrictMode, Profiler, Suspense, Activity, SuspenseList, a function
// component, a *Class, or a *ForwardRefType / *MemoType / *LazyType /
// *Context / *Consumer.
//
// Function components are Go funcs of one of these shapes:
//
//	func(h react.Hooks, props *react.Props) react.Node
//	func(props *react.Props) react.Node
//	func(h react.Hooks, props *react.Props) (react.Node, error)
//	func(props *react.Props) (react.Node, error)
//	func() react.Node
//
// Wrap one in a *FunctionComponent to give it a name, displayName or
// defaultProps (JavaScript functions carry these as properties). A non-nil
// error return is thrown, as is a panic.
//
// # Hooks
//
// Hooks are methods of the Hooks value a renderer passes to each function
// component (Go has no implicit "current component"): h.UseState(0),
// h.UseId(), h.Use(promise), h.UseContext(ctx) and so on. Each mirrors the
// JavaScript hook of the same name; see Hooks.
//
// # Class components
//
// A *Class describes a class: its constructor (New), statics (ContextType,
// DefaultProps, GetDerivedStateFromProps) and name. Instances embed
// Component (like `extends React.Component`), implement Render, and may
// implement ComponentWillMount / UNSAFE_componentWillMount.
//
// # Context, refs, memo, lazy
//
// CreateContext, CreateRef, ForwardRef, Memo and Lazy return the
// corresponding type objects. Lazy takes a loader returning a Thenable that
// resolves to a module: an *Object whose "default" property is the component
// (anything else is an undefined component, as in React).
//
// # Promises
//
// Promise models a JavaScript promise (reactions never run synchronously),
// which matters to React: an untracked promise always suspends once.
// ThenableState lets any type become a Thenable React can track.
//
// Not ported: development-only warnings and checks, the React Server
// Components build (react.react-server), the jsx-runtime entry points (use
// CreateElement) and the compiler runtime entry point beyond Hooks.UseMemoCache.
package react

import (
	"github.com/gav4you/npmgo/internal/jsvalue"
)

// Version is the React version this package translates.
const Version = "19.2.4"

// Node is anything React can render.
type Node = any

// Object is an ordered JavaScript plain object; see the package doc.
type Object = jsvalue.Object

// Props is the props object of an element.
type Props = jsvalue.Object

// Undefined is JavaScript's undefined.
type Undefined = jsvalue.Undefined

// Symbol is a JavaScript symbol.
type Symbol = jsvalue.Symbol

// Obj builds an *Object from alternating key, value arguments:
// Obj("className", "box", "id", "main").
func Obj(kv ...any) *Object { return jsvalue.NewObject(kv...) }

// SymbolFor is Symbol.for(key).
func SymbolFor(key string) Symbol { return jsvalue.SymbolFor(key) }

// NewSymbol is Symbol(description).
func NewSymbol(description string) Symbol { return jsvalue.NewSymbol(description) }

// The built-in element types (React's well-known symbols).
var (
	Fragment   = SymbolFor("react.fragment")
	StrictMode = SymbolFor("react.strict_mode")
	Profiler   = SymbolFor("react.profiler")
	Suspense   = SymbolFor("react.suspense")
	Activity   = SymbolFor("react.activity")

	// The following symbols are not exported by react@19.2.4 but
	// react-dom/server renders them (they are reachable with Symbol.for).
	SuspenseList   = SymbolFor("react.suspense_list")
	ViewTransition = SymbolFor("react.view_transition")
	LegacyHidden   = SymbolFor("react.legacy_hidden")
	Scope          = SymbolFor("react.scope")

	// MemoCacheSentinel fills the slots returned by Hooks.UseMemoCache.
	MemoCacheSentinel = SymbolFor("react.memo_cache_sentinel")
)

// Element is a React element ({$$typeof, type, key, ref, props}).
type Element struct {
	Type  any
	Key   *string // nil is null
	Ref   any     // props.ref, or nil
	Props *Props
}

// Portal is what react-dom's createPortal returns.
type Portal struct {
	Key            *string
	Children       any
	ContainerInfo  any
	Implementation any
}

func newElement(typ any, key *string, props *Props) *Element {
	var ref any
	if r, ok := props.Lookup("ref"); ok && !jsvalue.IsUndefined(r) {
		ref = r
	}
	return &Element{Type: typ, Key: key, Ref: ref, Props: props}
}

// defaultPropser is implemented by types that carry defaultProps.
type defaultPropser interface{ reactDefaultProps() *Object }

func defaultPropsOf(typ any) *Object {
	if !jsvalue.Truthy(typ) {
		return nil
	}
	if d, ok := typ.(defaultPropser); ok {
		return d.reactDefaultProps()
	}
	return nil
}

func childrenArg(props *Props, children []Node) {
	switch len(children) {
	case 0:
	case 1:
		props.Set("children", children[0])
	default:
		props.Set("children", append([]any(nil), children...))
	}
}

// CreateElement is React.createElement(type, config, ...children).
func CreateElement(typ any, config *Props, children ...Node) *Element {
	props := &Object{}
	var key *string
	if config != nil {
		if k, ok := config.Lookup("key"); ok && !jsvalue.IsUndefined(k) {
			s := jsvalue.ToString(k)
			key = &s
		}
		config.Range(func(name string, v any) bool {
			if name != "key" && name != "__self" && name != "__source" {
				props.Set(name, v)
			}
			return true
		})
	}
	childrenArg(props, children)
	if dp := defaultPropsOf(typ); dp != nil {
		dp.Range(func(name string, v any) bool {
			if jsvalue.IsUndefined(props.Get(name)) {
				props.Set(name, v)
			}
			return true
		})
	}
	return newElement(typ, key, props)
}

// CloneElement is React.cloneElement(element, config, ...children).
func CloneElement(element *Element, config *Props, children ...Node) *Element {
	if element == nil {
		panic(newError("The argument must be a React element, but you passed null."))
	}
	props := element.Props.Clone()
	key := element.Key
	if config != nil {
		if k, ok := config.Lookup("key"); ok && !jsvalue.IsUndefined(k) {
			s := jsvalue.ToString(k)
			key = &s
		}
		config.Range(func(name string, v any) bool {
			if name == "key" || name == "__self" || name == "__source" || (name == "ref" && jsvalue.IsUndefined(v)) {
				return true
			}
			props.Set(name, v)
			return true
		})
	}
	childrenArg(props, children)
	return newElement(element.Type, key, props)
}

// IsValidElement is React.isValidElement.
func IsValidElement(v any) bool {
	e, ok := v.(*Element)
	return ok && e != nil
}

// Ref is a ref object ({current}).
type Ref struct{ Current any }

// CreateRef is React.createRef.
func CreateRef() *Ref { return &Ref{} }

// FunctionComponent is a function component together with the properties a
// JavaScript function component can carry. Render is one of the function
// component shapes listed in the package doc.
type FunctionComponent struct {
	Name         string // the function's name (fn.name)
	DisplayName  string
	DefaultProps *Props
	Render       any
}

// JSTypeOf reports "function".
func (*FunctionComponent) JSTypeOf() string { return "function" }

func (f *FunctionComponent) reactDefaultProps() *Object { return f.DefaultProps }

// ForwardRefType is what ForwardRef returns.
type ForwardRefType struct {
	Render       func(h Hooks, props *Props, ref any) Node
	DisplayName  string
	Name         string // the render function's name, used in "ForwardRef(name)"
	DefaultProps *Props
}

func (f *ForwardRefType) reactDefaultProps() *Object { return f.DefaultProps }

// ForwardRef is React.forwardRef(render).
func ForwardRef(render func(h Hooks, props *Props, ref any) Node) *ForwardRefType {
	return &ForwardRefType{Render: render}
}

// MemoType is what Memo returns.
type MemoType struct {
	Type         any
	Compare      func(prevProps, nextProps *Props) bool // nil is null
	DisplayName  string
	DefaultProps *Props
}

func (m *MemoType) reactDefaultProps() *Object { return m.DefaultProps }

// Memo is React.memo(type, compare).
func Memo(typ any, compare func(prevProps, nextProps *Props) bool) *MemoType {
	return &MemoType{Type: typ, Compare: compare}
}

// Context is a React context. Context.Provider is the context itself (React
// 19), so the context can be used directly as an element type.
type Context struct {
	DefaultValue any
	DisplayName  string
	Provider     *Context
	Consumer     *Consumer
}

// Consumer is Context.Consumer; its children prop is a func(value any) Node.
type Consumer struct {
	Context *Context
}

// CreateContext is React.createContext(defaultValue).
func CreateContext(defaultValue any) *Context {
	c := &Context{DefaultValue: defaultValue}
	c.Provider = c
	c.Consumer = &Consumer{Context: c}
	return c
}

// Cache is React.cache(fn) outside React Server Components: it returns fn
// unchanged in behaviour (each call calls fn).
func Cache[F any](fn F) F { return fn }

// CacheSignal is React.cacheSignal(): nil outside React Server Components.
func CacheSignal() any { return nil }

// StartTransition is React.startTransition(scope). Outside a client renderer
// it just runs scope; a panic is reported (like reportGlobalError) instead of
// propagating.
func StartTransition(scope func()) {
	defer func() {
		if r := recover(); r != nil {
			ReportGlobalError(r)
		}
	}()
	scope()
}

// ReportGlobalError is called with errors React reports globally (Node:
// process.emit("uncaughtException")). Replace it to observe them.
var ReportGlobalError = func(err any) {}

// Error is an Error thrown by React (JavaScript `Error(message)`).
type Error struct{ Message string }

func (e *Error) Error() string { return e.Message }

func newError(msg string) *Error { return &Error{Message: msg} }
