package react

// Dispatch is the setter returned by UseState / UseReducer / UseOptimistic.
// For UseState an action that is a func(prev any) any is applied as an
// updater, like a JavaScript function action.
type Dispatch func(action any)

// FormStatus is what useFormStatus (Hooks.UseHostTransitionStatus) returns.
type FormStatus struct {
	Pending bool
	Data    any
	Method  any
	Action  any
}

// NotPendingFormStatus is the status of a form that is not submitting
// (React's sharedNotPendingObject).
var NotPendingFormStatus = &FormStatus{}

// FormActionFields is what an action's $$FORM_ACTION returns: how to submit
// it as a plain HTML form. Each string field may be nil / Undefined{}.
type FormActionFields struct {
	Name, Action, EncType, Method, Target any
	// Data are extra hidden fields (FormData entries); values must be strings.
	Data []FormField
}

// FormField is one FormData entry.
type FormField struct {
	Name  string
	Value any
}

// Action is a function that carries React's server-action properties. Any Go
// func is a valid action; use Action when it needs $$FORM_ACTION,
// $$IS_SIGNATURE_EQUAL or a custom bind (as a server action reference has).
type Action struct {
	// Fn is the function itself; it receives the call's arguments.
	Fn func(args ...any) any
	// FormAction is $$FORM_ACTION (nil if absent).
	FormAction func(identifierPrefix string) *FormActionFields
	// IsSignatureEqual is $$IS_SIGNATURE_EQUAL (nil if absent).
	IsSignatureEqual func(referenceID, boundArgsLength any) bool
	// Bind is a custom action.bind(null, ...args) (nil: plain binding).
	Bind func(args ...any) any
}

// JSTypeOf reports "function".
func (*Action) JSTypeOf() string { return "function" }

// CallJS calls the action's function.
func (a *Action) CallJS(args ...any) any {
	if a.Fn == nil {
		return Undefined{}
	}
	return a.Fn(args...)
}

// Hooks is the hooks dispatcher a renderer passes to each function component
// (React's ReactSharedInternals.H). Every method mirrors the JavaScript hook
// of the same name and may only be called while that component renders.
//
// Where JavaScript accepts a function or a value, Go accepts both: a func()
// any initial state is called (lazy initialisation), a func(any) any state
// action is applied to the previous state. deps of nil is "no dependency
// array"; []any{} is an empty one.
type Hooks interface {
	// ReadContext reads a context without subscribing (readContext).
	ReadContext(ctx *Context) any
	// Use is use(usable): a Thenable or a *Context.
	Use(usable any) any
	UseContext(ctx *Context) any
	UseState(initialState any) (state any, setState Dispatch)
	UseReducer(reducer func(state, action any) any, initialArg any, init func(arg any) any) (state any, dispatch Dispatch)
	UseMemo(create func() any, deps []any) any
	UseCallback(callback any, deps []any) any
	UseRef(initialValue any) *Ref
	UseId() string
	// UseSyncExternalStore returns getServerSnapshot() on the server.
	UseSyncExternalStore(subscribe any, getSnapshot func() any, getServerSnapshot func() any) any
	// UseDeferredValue(value) or UseDeferredValue(value, initialValue).
	UseDeferredValue(value any, initialValue ...any) any
	UseTransition() (isPending bool, startTransition func(scope func()))
	UseOptimistic(passthrough any, reducer func(state, action any) any) (any, Dispatch)
	// UseActionState returns [state, dispatch, isPending]; dispatch is a
	// *Action. permalink is optional.
	UseActionState(action any, initialState any, permalink ...string) (state any, dispatch *Action, isPending bool)
	// UseFormState is react-dom's useFormState (an alias of UseActionState).
	UseFormState(action any, initialState any, permalink ...string) (state any, dispatch *Action, isPending bool)
	// UseHostTransitionStatus backs react-dom's useFormStatus.
	UseHostTransitionStatus() *FormStatus
	UseEffect(create func() func(), deps []any)
	UseLayoutEffect(create func() func(), deps []any)
	UseInsertionEffect(create func() func(), deps []any)
	UseImperativeHandle(ref any, create func() any, deps []any)
	UseDebugValue(value any, format ...func(any) any)
	// UseEffectEvent returns a function that throws if called during render.
	UseEffectEvent(callback any) func(args ...any) any
	// UseMemoCache backs the React Compiler runtime (react/compiler-runtime c()).
	UseMemoCache(size int) []any
	// UseCacheRefresh is unstable_useCacheRefresh.
	UseCacheRefresh() func()
}
