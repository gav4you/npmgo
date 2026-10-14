package react

import "github.com/gav4you/npmgo/internal/jsvalue"

// Class describes a class component (a JavaScript class extending
// React.Component): its constructor and its static properties.
type Class struct {
	// Name is the class name (used in component names), DisplayName its
	// static displayName.
	Name, DisplayName string
	// New is the constructor (new Class(props, context)). The returned
	// instance must embed Component; set its State here, like `this.state =`.
	New func(props *Props, context any) ComponentInstance
	// ContextType is the static contextType (nil for none).
	ContextType *Context
	// DefaultProps is the static defaultProps.
	DefaultProps *Props
	// GetDerivedStateFromProps is the static getDerivedStateFromProps.
	GetDerivedStateFromProps func(props *Props, state any) any
	// Pure marks a PureComponent subclass.
	Pure bool
}

// JSTypeOf reports "function" (classes are functions in JavaScript).
func (*Class) JSTypeOf() string { return "function" }

func (c *Class) reactDefaultProps() *Object { return c.DefaultProps }

// ComponentInstance is a class component instance: a type that embeds
// Component and renders.
type ComponentInstance interface {
	Render() Node
	ReactComponent() *Component
}

// Updater is a class instance's this.updater: the renderer's update queue.
type Updater interface {
	IsMounted(c *Component) bool
	EnqueueSetState(c *Component, partialState any, callback func(), callerName string)
	EnqueueReplaceState(c *Component, state any, callback func(), callerName string)
	EnqueueForceUpdate(c *Component, callback func(), callerName string)
}

// Component is the base of class components; embed it in the instance
// struct, as `class X extends React.Component` does.
type Component struct {
	Props   *Props
	State   any
	Context any
	Refs    *Object
	// Updater is this.updater; the renderer installs its own.
	Updater Updater
	// Internals is this._reactInternals, owned by the renderer.
	Internals any
}

// ReactComponent returns the embedded Component.
func (c *Component) ReactComponent() *Component { return c }

type noopUpdater struct{}

func (noopUpdater) IsMounted(*Component) bool                           { return false }
func (noopUpdater) EnqueueSetState(*Component, any, func(), string)     {}
func (noopUpdater) EnqueueReplaceState(*Component, any, func(), string) {}
func (noopUpdater) EnqueueForceUpdate(*Component, func(), string)       {}

func (c *Component) updater() Updater {
	if c.Updater == nil {
		return noopUpdater{}
	}
	return c.Updater
}

// SetState is this.setState(partialState, callback). partialState is an
// *Object, nil, or an updater func(prevState any, props *Props) any (or
// func(prevState any) any).
func (c *Component) SetState(partialState any, callback ...func()) {
	t := jsvalue.TypeOf(partialState)
	if t != "object" && t != "function" && !jsvalue.IsNullish(partialState) {
		panic(newError("takes an object of state variables to update or a function which returns an object of state variables."))
	}
	var cb func()
	if len(callback) > 0 {
		cb = callback[0]
	}
	c.updater().EnqueueSetState(c, partialState, cb, "setState")
}

// ForceUpdate is this.forceUpdate(callback).
func (c *Component) ForceUpdate(callback ...func()) {
	var cb func()
	if len(callback) > 0 {
		cb = callback[0]
	}
	c.updater().EnqueueForceUpdate(c, cb, "forceUpdate")
}

// NewComponent returns a Component initialised like the JavaScript
// constructor Component(props, context, updater).
func NewComponent(props *Props, context any) Component {
	return Component{Props: props, Context: context, Refs: &Object{}}
}
