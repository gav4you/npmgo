package server

import (
	"crypto/md5"
	"encoding/hex"
	"reflect"
	"strconv"

	"github.com/gav4you/npmgo/internal/jsloop"
	"github.com/gav4you/npmgo/internal/jsvalue"
	"github.com/gav4you/npmgo/pkgs/react"
)

// componentIdentity is the `{}` React creates per renderWithHooks call.
type componentIdentity struct{ _ byte }

type hook struct {
	memoizedState any
	queue         *hookQueue
	next          *hook
}

type hookQueue struct {
	dispatch react.Dispatch
}

type update struct {
	action any
	next   *update
}

type memoState struct {
	value any
	deps  []any // nil is null
}

// hooks is the hook-rendering state (module-level variables in JavaScript).
type hooks struct {
	currentlyRenderingComponent  *componentIdentity
	currentlyRenderingTask       *task
	currentlyRenderingKeyPath    *keyPath
	firstWorkInProgressHook      *hook
	workInProgressHook           *hook
	isReRender                   bool
	didScheduleRenderPhaseUpdate bool
	localIdCounter               int
	actionStateCounter           int
	actionStateMatchingIndex     int
	thenableIndexCounter         int
	thenableState                *[]react.Thenable
	renderPhaseUpdates           map[*hookQueue]*update
	numberOfReRenders            int
	currentResumableState        *resumableState
}

var suspenseException = &react.Error{Message: "Suspense Exception: This is not a real error! It's an implementation detail of `use` to interrupt the current render. You must either rethrow it immediately, or move the `use` call outside of the `try/catch` block. Capturing without rethrowing will lead to unexpected behavior.\n\nTo handle async errors, wrap your component in an error boundary, or call the promise's `.catch` method and pass the result to `use`."}

const invalidHookCall = "Invalid hook call. Hooks can only be called inside of the body of a function component. This could happen for one of the following reasons:\n1. You might have mismatching versions of React and the renderer (such as React DOM)\n2. You might be breaking the Rules of Hooks\n3. You might have more than one copy of React in the same app\nSee https://react.dev/link/invalid-hook-call for tips about how to debug and fix this problem."

func (r *request) resolveCurrentlyRenderingComponent() *componentIdentity {
	if r.hooks.currentlyRenderingComponent == nil {
		panic(newError(invalidHookCall))
	}
	return r.hooks.currentlyRenderingComponent
}

func (r *request) createHook() *hook {
	if r.hooks.numberOfReRenders > 0 {
		panic(newError("Rendered more hooks than during the previous render"))
	}
	return &hook{}
}

func (r *request) createWorkInProgressHook() *hook {
	h := &r.hooks
	if h.workInProgressHook == nil {
		if h.firstWorkInProgressHook == nil {
			h.isReRender = false
			h.firstWorkInProgressHook = r.createHook()
			h.workInProgressHook = h.firstWorkInProgressHook
		} else {
			h.isReRender = true
			h.workInProgressHook = h.firstWorkInProgressHook
		}
	} else if h.workInProgressHook.next == nil {
		h.isReRender = false
		h.workInProgressHook.next = r.createHook()
		h.workInProgressHook = h.workInProgressHook.next
	} else {
		h.isReRender = true
		h.workInProgressHook = h.workInProgressHook.next
	}
	return h.workInProgressHook
}

func (r *request) getThenableStateAfterSuspending() *[]react.Thenable {
	s := r.hooks.thenableState
	r.hooks.thenableState = nil
	return s
}

func (r *request) resetHooksState() {
	h := &r.hooks
	h.currentlyRenderingKeyPath = nil
	h.currentlyRenderingTask = nil
	h.currentlyRenderingComponent = nil
	h.didScheduleRenderPhaseUpdate = false
	h.firstWorkInProgressHook = nil
	h.numberOfReRenders = 0
	h.workInProgressHook = nil
	h.renderPhaseUpdates = nil
}

func (r *request) getSuspendedThenable() any {
	if !r.hasSuspendedThenable {
		panic(newError("Expected a suspended thenable. This is a bug in React. Please file an issue."))
	}
	t := r.suspendedThenable
	r.suspendedThenable, r.hasSuspendedThenable = nil, false
	return t
}

func (r *request) trackUsedThenable(state *[]react.Thenable, t react.Thenable, index int) any {
	if index >= len(*state) {
		*state = append(*state, t)
	} else if prev := (*state)[index]; !jsvalue.ObjectIs(prev, t) {
		jsloop.Then(r.loop, t, noopAny, noopAny)
		t = prev
	}
	status, value, reason := t.ThenableStatus()
	switch status {
	case "fulfilled":
		return value
	case "rejected":
		panic(react.Throw(reason))
	}
	if status != "" {
		jsloop.Then(r.loop, t, noopAny, noopAny)
	} else {
		t.SetThenableStatus("pending", nil, nil)
		tt := t
		jsloop.Then(r.loop, t, func(v any) {
			if s, _, _ := tt.ThenableStatus(); s == "pending" {
				tt.SetThenableStatus("fulfilled", v, nil)
			}
		}, func(e any) {
			if s, _, _ := tt.ThenableStatus(); s == "pending" {
				tt.SetThenableStatus("rejected", nil, e)
			}
		})
	}
	status, value, reason = t.ThenableStatus()
	switch status {
	case "fulfilled":
		return value
	case "rejected":
		panic(react.Throw(reason))
	}
	r.suspendedThenable, r.hasSuspendedThenable = t, true
	panic(suspenseException)
}

func noopAny(any) {}

func (r *request) unwrapThenable(t react.Thenable) any {
	h := &r.hooks
	index := h.thenableIndexCounter
	h.thenableIndexCounter++
	if h.thenableState == nil {
		h.thenableState = &[]react.Thenable{}
	}
	return r.trackUsedThenable(h.thenableState, t, index)
}

// callComponent calls a function component (or forwardRef render) with the
// arguments React passes: props, and secondArg (the ref for forwardRef).
func (r *request) callComponent(component any, props *react.Props, secondArg any) any {
	d := r.dispatcher
	var result any
	var err error
	switch f := component.(type) {
	case *react.FunctionComponent:
		return r.callComponent(f.Render, props, secondArg)
	case *react.ForwardRefType:
		if f.Render == nil {
			panic(&jsvalue.TypeError{Message: "render is not a function"})
		}
		return f.Render(d, props, secondArg)
	case func(react.Hooks, *react.Props) any:
		result = f(d, props)
	case func(*react.Props) any:
		result = f(props)
	case func(react.Hooks, *react.Props) (any, error):
		result, err = f(d, props)
	case func(*react.Props) (any, error):
		result, err = f(props)
	case func() any:
		result = f()
	case func(react.Hooks) any:
		result = f(d)
	case func(react.Hooks, *react.Props, any) any:
		result = f(d, props, secondArg)
	default:
		rv := reflect.ValueOf(component)
		if rv.Kind() != reflect.Func || rv.IsNil() {
			panic(&jsvalue.TypeError{Message: "Component is not a function"})
		}
		var ok bool
		result, ok = jsvalue.Call(component, props, secondArg)
		if !ok {
			panic(&jsvalue.TypeError{Message: "Component is not a function"})
		}
	}
	if err != nil {
		panic(err)
	}
	return result
}

// dispatcher implements react.Hooks for one request (React's HooksDispatcher).
type dispatcher struct{ r *request }

func (d *dispatcher) ReadContext(ctx *react.Context) any { return d.r.readContext(ctx) }

func (d *dispatcher) Use(usable any) any {
	if !jsvalue.IsNull(usable) && jsvalue.TypeOf(usable) == "object" {
		if t, ok := react.IsThenable(usable); ok {
			return d.r.unwrapThenable(t)
		}
		if c, ok := usable.(*react.Context); ok {
			return d.r.readContext(c)
		}
	}
	panic(newError("An unsupported type was passed to use(): " + jsvalue.String(usable)))
}

func (d *dispatcher) UseContext(ctx *react.Context) any {
	d.r.resolveCurrentlyRenderingComponent()
	return d.r.readContext(ctx)
}

func basicStateAction(state, action any) any {
	if jsvalue.IsFunction(action) {
		if f, ok := action.(func(any) any); ok {
			return f(state)
		}
		v, _ := jsvalue.Call(action, state)
		return v
	}
	return action
}

func (r *request) useReducer(basic bool, reducer func(state, action any) any, initialArg any, init func(any) any) (any, react.Dispatch) {
	h := &r.hooks
	h.currentlyRenderingComponent = r.resolveCurrentlyRenderingComponent()
	wip := r.createWorkInProgressHook()
	apply := reducer
	if basic {
		apply = basicStateAction
	}
	if h.isReRender {
		queue := wip.queue
		dispatch := queue.dispatch
		if h.renderPhaseUpdates != nil {
			if u, ok := h.renderPhaseUpdates[queue]; ok {
				delete(h.renderPhaseUpdates, queue)
				state := wip.memoizedState
				for ; u != nil; u = u.next {
					state = apply(state, u.action)
				}
				wip.memoizedState = state
				return state, dispatch
			}
		}
		return wip.memoizedState, dispatch
	}
	var state any
	if basic {
		state = initialArg
		if jsvalue.IsFunction(initialArg) {
			if f, ok := initialArg.(func() any); ok {
				state = f()
			} else {
				state, _ = jsvalue.Call(initialArg)
			}
		}
	} else if init != nil {
		state = init(initialArg)
	} else {
		state = initialArg
	}
	wip.memoizedState = state
	queue := &hookQueue{}
	wip.queue = queue
	identity := h.currentlyRenderingComponent
	queue.dispatch = func(action any) { r.dispatchAction(identity, queue, action) }
	return wip.memoizedState, queue.dispatch
}

func (r *request) dispatchAction(identity *componentIdentity, queue *hookQueue, action any) {
	h := &r.hooks
	if h.numberOfReRenders >= 25 {
		panic(newError("Too many re-renders. React limits the number of renders to prevent an infinite loop."))
	}
	if identity != h.currentlyRenderingComponent {
		return
	}
	h.didScheduleRenderPhaseUpdate = true
	u := &update{action: action}
	if h.renderPhaseUpdates == nil {
		h.renderPhaseUpdates = map[*hookQueue]*update{}
	}
	last, ok := h.renderPhaseUpdates[queue]
	if !ok {
		h.renderPhaseUpdates[queue] = u
		return
	}
	for last.next != nil {
		last = last.next
	}
	last.next = u
}

func (d *dispatcher) UseState(initialState any) (any, react.Dispatch) {
	return d.r.useReducer(true, nil, initialState, nil)
}

func (d *dispatcher) UseReducer(reducer func(state, action any) any, initialArg any, init func(any) any) (any, react.Dispatch) {
	return d.r.useReducer(false, reducer, initialArg, init)
}

func (r *request) useMemo(create func() any, deps []any) any {
	h := &r.hooks
	h.currentlyRenderingComponent = r.resolveCurrentlyRenderingComponent()
	wip := r.createWorkInProgressHook()
	if wip != nil {
		if prev, ok := wip.memoizedState.(*memoState); ok && prev != nil && deps != nil {
			prevDeps := prev.deps
			equal := false
			if prevDeps != nil {
				equal = true
				for i := 0; i < len(prevDeps) && i < len(deps); i++ {
					if !jsvalue.ObjectIs(deps[i], prevDeps[i]) {
						equal = false
						break
					}
				}
			}
			if equal {
				return prev.value
			}
		}
	}
	v := create()
	wip.memoizedState = &memoState{value: v, deps: deps}
	return v
}

func (d *dispatcher) UseMemo(create func() any, deps []any) any { return d.r.useMemo(create, deps) }

func (d *dispatcher) UseCallback(callback any, deps []any) any {
	return d.r.useMemo(func() any { return callback }, deps)
}

func (d *dispatcher) UseRef(initialValue any) *react.Ref {
	r := d.r
	r.hooks.currentlyRenderingComponent = r.resolveCurrentlyRenderingComponent()
	wip := r.createWorkInProgressHook()
	if ref, ok := wip.memoizedState.(*react.Ref); ok && ref != nil {
		return ref
	}
	ref := &react.Ref{Current: initialValue}
	wip.memoizedState = ref
	return ref
}

func (d *dispatcher) UseInsertionEffect(func() func(), []any)    {}
func (d *dispatcher) UseLayoutEffect(func() func(), []any)       {}
func (d *dispatcher) UseEffect(func() func(), []any)             {}
func (d *dispatcher) UseImperativeHandle(any, func() any, []any) {}
func (d *dispatcher) UseDebugValue(any, ...func(any) any)        {}

func (d *dispatcher) UseDeferredValue(value any, initialValue ...any) any {
	d.r.resolveCurrentlyRenderingComponent()
	if len(initialValue) > 0 && !jsvalue.IsUndefined(initialValue[0]) {
		return initialValue[0]
	}
	return value
}

func (d *dispatcher) UseTransition() (bool, func(func())) {
	d.r.resolveCurrentlyRenderingComponent()
	return false, func(func()) {
		panic(newError("startTransition cannot be called during server rendering."))
	}
}

func (d *dispatcher) UseId() string {
	r := d.r
	t := r.hooks.currentlyRenderingTask
	if t == nil {
		panic(&jsvalue.TypeError{Message: "Cannot read properties of null (reading 'treeContext')"})
	}
	tc := t.treeContext
	id := tc.id &^ (1 << (32 - clz32(tc.id) - 1))
	treeID := strconv.FormatUint(uint64(id), 32) + tc.overflow
	rs := r.hooks.currentResumableState
	if rs == nil {
		panic(newError("Invalid hook call. Hooks can only be called inside of the body of a function component."))
	}
	localID := r.hooks.localIdCounter
	r.hooks.localIdCounter++
	out := "_" + rs.idPrefix + "R_" + treeID
	if localID > 0 {
		out += "H" + strconv.FormatInt(int64(localID), 32)
	}
	return out + "_"
}

func (d *dispatcher) UseSyncExternalStore(subscribe any, getSnapshot func() any, getServerSnapshot func() any) any {
	if getServerSnapshot == nil {
		panic(newError("Missing getServerSnapshot, which is required for server-rendered content. Will revert to client rendering."))
	}
	return getServerSnapshot()
}

func (d *dispatcher) UseOptimistic(passthrough any, reducer func(state, action any) any) (any, react.Dispatch) {
	d.r.resolveCurrentlyRenderingComponent()
	return passthrough, func(any) { panic(newError("Cannot update optimistic state while rendering.")) }
}

func (d *dispatcher) UseActionState(action any, initialState any, permalink ...string) (any, *react.Action, bool) {
	return d.r.useActionState(action, initialState, permalink)
}

func (d *dispatcher) UseFormState(action any, initialState any, permalink ...string) (any, *react.Action, bool) {
	return d.r.useActionState(action, initialState, permalink)
}

func (d *dispatcher) UseHostTransitionStatus() *react.FormStatus {
	d.r.resolveCurrentlyRenderingComponent()
	return react.NotPendingFormStatus
}

func (d *dispatcher) UseMemoCache(size int) []any {
	data := make([]any, size)
	for i := range data {
		data[i] = react.MemoCacheSentinel
	}
	return data
}

func (d *dispatcher) UseCacheRefresh() func() {
	return func() { panic(newError("Cache cannot be refreshed during server rendering.")) }
}

func (d *dispatcher) UseEffectEvent(any) func(...any) any {
	return func(...any) any {
		panic(newError("A function wrapped in useEffectEvent can't be called during rendering."))
	}
}

// The resource-hint methods make the dispatcher react-dom's
// ReactDOMSharedInternals.d for the request it renders (see reactdom).
func (d *dispatcher) request() *request {
	r := d.r
	if r.flavor == legacyFlavor && !r.performingWork {
		return nil
	}
	return r
}

func (d *dispatcher) PrefetchDNS(href string) { d.request().prefetchDNS(href) }
func (d *dispatcher) Preconnect(href string, crossOrigin any) {
	d.request().preconnect(href, crossOrigin)
}
func (d *dispatcher) Preload(href, as string, opts *react.Object) {
	d.request().preload(href, as, opts)
}
func (d *dispatcher) PreloadModule(href string, opts *react.Object) {
	d.request().preloadModule(href, opts)
}
func (d *dispatcher) PreinitScript(src string, opts *react.Object) {
	d.request().preinitScript(src, opts)
}
func (d *dispatcher) PreinitStyle(href string, precedence any, opts *react.Object) {
	d.request().preinitStyle(href, precedence, opts)
}
func (d *dispatcher) PreinitModuleScript(src string, opts *react.Object) {
	d.request().preinitModuleScript(src, opts)
}

func (r *request) createPostbackActionStateKey(permalink []string, kp *keyPath, hookIndex int) string {
	if len(permalink) > 0 {
		return "p" + permalink[0]
	}
	j := "[" + kp.json() + ",null," + strconv.Itoa(hookIndex) + "]"
	if r.flavor == legacyFlavor {
		return "k" + strconv.FormatUint(uint64(murmurhash3(j, 0)), 10)
	}
	sum := md5.Sum([]byte(j))
	return "k" + hex.EncodeToString(sum[:])
}

func bindAction(action any, args ...any) any {
	if a, ok := action.(*react.Action); ok && a != nil && a.Bind != nil {
		return a.Bind(args...)
	}
	return &react.Action{Fn: func(more ...any) any {
		v, _ := jsvalue.Call(action, append(append([]any{}, args...), more...)...)
		return v
	}}
}

func (r *request) useActionState(action any, initialState any, permalink []string) (any, *react.Action, bool) {
	r.resolveCurrentlyRenderingComponent()
	h := &r.hooks
	hookIndex := h.actionStateCounter
	h.actionStateCounter++
	if a, ok := action.(*react.Action); ok && a != nil && a.FormAction != nil {
		var nextPostbackStateKey *string
		componentKeyPath := h.currentlyRenderingKeyPath
		formState := r.formState
		if fs := jsvalue.Items(formState); formState != nil && a.IsSignatureEqual != nil && len(fs) >= 4 {
			postbackKey := fs[1]
			if a.IsSignatureEqual(fs[2], fs[3]) {
				k := r.createPostbackActionStateKey(permalink, componentKeyPath, hookIndex)
				nextPostbackStateKey = &k
				if postbackKey == k {
					h.actionStateMatchingIndex = hookIndex
					initialState = fs[0]
				}
			}
		}
		bound := bindAction(action, initialState)
		wrapper := &react.Action{Fn: func(args ...any) any {
			var payload any = jsvalue.Undefined{}
			if len(args) > 0 {
				payload = args[0]
			}
			jsvalue.Call(bound, payload)
			return jsvalue.Undefined{}
		}}
		if b, ok := bound.(*react.Action); ok && b.FormAction != nil {
			wrapper.FormAction = func(prefix string) *react.FormActionFields {
				fields := b.FormAction(prefix)
				if len(permalink) > 0 {
					fields.Action = permalink[0]
				}
				if fields.Data != nil {
					if nextPostbackStateKey == nil {
						k := r.createPostbackActionStateKey(permalink, componentKeyPath, hookIndex)
						nextPostbackStateKey = &k
					}
					fields.Data = append(fields.Data, react.FormField{Name: "$ACTION_KEY", Value: *nextPostbackStateKey})
				}
				return fields
			}
		}
		return initialState, wrapper, false
	}
	bound := bindAction(action, initialState)
	return initialState, &react.Action{Fn: func(args ...any) any {
		var payload any = jsvalue.Undefined{}
		if len(args) > 0 {
			payload = args[0]
		}
		jsvalue.Call(bound, payload)
		return jsvalue.Undefined{}
	}}, false
}
