package server

import (
	"math"
	"math/bits"
	"strconv"
	"strings"

	"github.com/gav4you/npmgo/internal/jsloop"
	"github.com/gav4you/npmgo/internal/jsvalue"
	"github.com/gav4you/npmgo/pkgs/react"
)

type flavor int

const (
	legacyFlavor flavor = iota // renderToString / renderToStaticMarkup (react-dom-server-legacy)
	streamFlavor               // renderToPipeableStream (react-dom-server.node)
)

// Request status.
const (
	statusOpening  = 10
	statusOpen     = 11
	statusAborting = 12
	statusClosing  = 13
	statusClosed   = 14
)

// Segment status.
const (
	segPending   = 0
	segCompleted = 1
	segFlushed   = 2
	segAborted   = 3
	segErrored   = 4
	segPostponed = 5
	segRendering = 6
)

// Boundary status.
const (
	boundaryPending        = 0
	boundaryCompleted      = 1
	boundaryClientRendered = 4
	boundaryPostponed      = 5
)

// keyPath is React's [parentKeyPath, name, keyOrIndex].
type keyPath struct {
	parent *keyPath
	name   any // string or nil
	key    any // string or float64
}

func (k *keyPath) json() string {
	if k == nil {
		return "null"
	}
	name := "null"
	if s, ok := k.name.(string); ok {
		name = jsvalue.JSONQuote(s)
	}
	var key string
	switch x := k.key.(type) {
	case string:
		key = jsvalue.JSONQuote(x)
	default:
		key, _ = jsvalue.JSONStringify(x)
	}
	return "[" + k.parent.json() + "," + name + "," + key + "]"
}

type treeContext struct {
	id       uint32
	overflow string
}

var emptyTreeContext = treeContext{id: 1}

func clz32(x uint32) int { return bits.LeadingZeros32(x) }

func pushTreeContext(base treeContext, totalChildren, index int) treeContext {
	baseIDWithLeadingBit := base.id
	baseOverflow := base.overflow
	baseLength := 32 - clz32(baseIDWithLeadingBit) - 1
	baseID := baseIDWithLeadingBit &^ (1 << baseLength)
	slot := uint32(index + 1)
	length := 32 - clz32(uint32(totalChildren)) + baseLength
	if length > 30 {
		numberOfOverflowBits := baseLength - baseLength%5
		newOverflowBits := baseID & ((1 << numberOfOverflowBits) - 1)
		newOverflow := strconv.FormatUint(uint64(newOverflowBits), 32)
		restOfBaseID := baseID >> numberOfOverflowBits
		restOfBaseLength := baseLength - numberOfOverflowBits
		return treeContext{
			id:       (1 << (32 - clz32(uint32(totalChildren)) + restOfBaseLength)) | (slot << restOfBaseLength) | restOfBaseID,
			overflow: newOverflow + baseOverflow,
		}
	}
	return treeContext{id: (1 << length) | (slot << baseLength) | baseID, overflow: baseOverflow}
}

type componentStackNode struct {
	parent *componentStackNode
	typ    any
}

type suspenseListRow struct {
	pendingTasks        int
	boundaries          []*boundary
	hasBoundaries       bool
	hoistables          *hoistableState
	inheritedHoistables *hoistableState
	together            bool
	next                *suspenseListRow
}

type boundary struct {
	status                 int
	rootSegmentID          int
	parentFlushed          bool
	pendingTasks           int
	row                    *suspenseListRow
	completedSegments      []*segment
	byteSize               int
	fallbackAbortableTasks *orderedSet[*task]
	errorDigest            any
	contentState           *hoistableState
	fallbackState          *hoistableState
	contentPreamble        *preambleState
	fallbackPreamble       *preambleState
}

type segment struct {
	status              int
	parentFlushed       bool
	id                  int
	index               int
	chunks              []string
	children            []*segment
	preambleChildren    []*segment
	parentFormatContext *formatContext
	boundary            *boundary
	lastPushedText      bool
	textEmbedded        bool
}

type task struct {
	node            any
	childIndex      int
	ping            func()
	blockedBoundary *boundary
	blockedSegment  *segment
	blockedPreamble *preambleState
	hoistableState  *hoistableState
	abortSet        *orderedSet[*task]
	keyPath         *keyPath
	formatContext   *formatContext
	context         *contextSnapshot
	treeContext     treeContext
	row             *suspenseListRow
	componentStack  *componentStackNode
	thenableState   *[]react.Thenable
}

// ErrorInfo is the second argument of onError.
type ErrorInfo struct {
	// ComponentStack lists the components above the error, innermost
	// first, one "\n    at Name (<anonymous>)" line each (the JavaScript
	// stack also carries source locations Go cannot reproduce).
	ComponentStack string
}

type request struct {
	flavor                    flavor
	loop                      *jsloop.Loop
	destination               destination
	flushScheduled            bool
	resumableState            *resumableState
	renderState               *renderState
	rootFormatContext         *formatContext
	progressiveChunkSize      float64
	status                    int
	fatalError                any
	nextSegmentID             int
	allPendingTasks           int
	pendingRootTasks          int
	completedRootSegment      *segment
	completedPreambleSegments [][]*segment
	hasCompletedPreamble      bool
	byteSize                  int
	abortableTasks            *orderedSet[*task]
	pingedTasks               []*task
	clientRenderedBoundaries  []*boundary
	completedBoundaries       []*boundary
	partialBoundaries         []*boundary
	onError                   func(err any, info *ErrorInfo) any
	onAllReady                func()
	onShellReady              func()
	onShellError              func(err any)
	onFatalError              func(err any)
	formState                 any // nil is null

	// Module-level state of the JavaScript renderer, kept per request.
	performingWork                             bool
	currentActiveSnapshot                      *contextSnapshot
	contextValues                              map[*react.Context]any
	flushedByteSize                            int
	flushingPartialBoundaries                  bool
	currentlyRenderingBoundaryHasStylesToHoist bool
	destinationHasCapacity                     bool
	suspendedThenable                          react.Thenable
	hasSuspendedThenable                       bool
	hooks
	dispatcher *dispatcher
}

func defaultErrorHandler(err any, _ *ErrorInfo) any {
	ConsoleError(err)
	return nil
}

func noopCallback()       {}
func noopErrCallback(any) {}

func newRequest(fl flavor, children any, rs *resumableState, st *renderState, rootFC *formatContext, progressiveChunkSize float64,
	onError func(any, *ErrorInfo) any, onAllReady, onShellReady func(), onShellError, onFatalError func(any), formState any) *request {
	r := &request{
		flavor:               fl,
		loop:                 jsloop.New(),
		resumableState:       rs,
		renderState:          st,
		rootFormatContext:    rootFC,
		progressiveChunkSize: progressiveChunkSize,
		status:               statusOpening,
		abortableTasks:       newSet[*task](),
		onError:              onError,
		onAllReady:           onAllReady,
		onShellReady:         onShellReady,
		onShellError:         onShellError,
		onFatalError:         onFatalError,
		formState:            formState,
		contextValues:        map[*react.Context]any{},
	}
	if r.onError == nil {
		r.onError = defaultErrorHandler
	}
	if r.onAllReady == nil {
		r.onAllReady = noopCallback
	}
	if r.onShellReady == nil {
		r.onShellReady = noopCallback
	}
	if r.onShellError == nil {
		r.onShellError = noopErrCallback
	}
	if r.onFatalError == nil {
		r.onFatalError = noopErrCallback
	}
	r.hooks.actionStateMatchingIndex = -1
	r.dispatcher = &dispatcher{r: r}
	rootSegment := r.createPendingSegment(0, nil, rootFC, false, false)
	rootSegment.parentFlushed = true
	t := r.createRenderTask(nil, children, -1, nil, rootSegment, nil, nil, r.abortableTasks, nil, rootFC, nil, emptyTreeContext, nil, nil)
	pushComponentStack(t)
	r.pingedTasks = append(r.pingedTasks, t)
	return r
}

func (r *request) pingTask(t *task) {
	r.pingedTasks = append(r.pingedTasks, t)
	if len(r.pingedTasks) != 1 {
		return
	}
	r.flushScheduled = r.destination != nil
	if r.flavor == legacyFlavor {
		r.performWork()
		return
	}
	if r.status == statusOpening {
		r.loop.QueueMicrotask(r.performWork)
	} else {
		r.loop.SetImmediate(r.performWork)
	}
}

func (r *request) createSuspenseBoundary(row *suspenseListRow, fallbackAbortableTasks *orderedSet[*task], contentPreamble, fallbackPreamble *preambleState) *boundary {
	b := &boundary{
		status:                 boundaryPending,
		rootSegmentID:          -1,
		row:                    row,
		fallbackAbortableTasks: fallbackAbortableTasks,
		contentState:           createHoistableState(),
		fallbackState:          createHoistableState(),
		contentPreamble:        contentPreamble,
		fallbackPreamble:       fallbackPreamble,
	}
	if row != nil {
		row.pendingTasks++
		if row.hasBoundaries {
			r.allPendingTasks++
			b.pendingTasks++
			row.boundaries = append(row.boundaries, b)
		}
		if row.inheritedHoistables != nil {
			hoistHoistables(b.contentState, row.inheritedHoistables)
		}
	}
	return b
}

func (r *request) createRenderTask(thenableState *[]react.Thenable, node any, childIndex int, blockedBoundary *boundary, blockedSegment *segment,
	blockedPreamble *preambleState, hoistable *hoistableState, abortSet *orderedSet[*task], kp *keyPath, fc *formatContext,
	ctx *contextSnapshot, tc treeContext, row *suspenseListRow, componentStack *componentStackNode) *task {
	r.allPendingTasks++
	if blockedBoundary == nil {
		r.pendingRootTasks++
	} else {
		blockedBoundary.pendingTasks++
	}
	if row != nil {
		row.pendingTasks++
	}
	t := &task{
		node:            node,
		childIndex:      childIndex,
		blockedBoundary: blockedBoundary,
		blockedSegment:  blockedSegment,
		blockedPreamble: blockedPreamble,
		hoistableState:  hoistable,
		abortSet:        abortSet,
		keyPath:         kp,
		formatContext:   fc,
		context:         ctx,
		treeContext:     tc,
		row:             row,
		componentStack:  componentStack,
		thenableState:   thenableState,
	}
	t.ping = func() { r.pingTask(t) }
	abortSet.add(t)
	return t
}

func (r *request) createPendingSegment(index int, b *boundary, parentFC *formatContext, lastPushedText, textEmbedded bool) *segment {
	return &segment{
		status:              segPending,
		id:                  -1,
		index:               index,
		parentFormatContext: parentFC,
		boundary:            b,
		lastPushedText:      lastPushedText,
		textEmbedded:        textEmbedded,
	}
}

func pushComponentStack(t *task) {
	if el, ok := t.node.(*react.Element); ok && el != nil {
		t.componentStack = &componentStackNode{parent: t.componentStack, typ: el.Type}
	}
}

func replaceSuspenseComponentStackWithSuspenseFallbackStack(cs *componentStackNode) *componentStackNode {
	if cs == nil {
		return nil
	}
	return &componentStackNode{parent: cs.parent, typ: "Suspense Fallback"}
}

func (r *request) getThrownInfo(cs *componentStackNode) *ErrorInfo {
	info := &ErrorInfo{}
	if cs != nil {
		var b strings.Builder
		for n := cs; n != nil; n = n.parent {
			b.WriteString(r.describeComponentStackByType(n.typ))
		}
		info.ComponentStack = b.String()
	}
	return info
}

func describeBuiltInComponentFrame(name string) string { return "\n    at " + name + " (<anonymous>)" }

func (r *request) describeComponentStackByType(typ any) string {
	if s, ok := typ.(string); ok {
		return describeBuiltInComponentFrame(s)
	}
	switch t := typ.(type) {
	case *react.ForwardRefType:
		if t.Name != "" {
			return describeBuiltInComponentFrame(t.Name)
		}
		return ""
	case *react.MemoType:
		return r.describeComponentStackByType(t.Type)
	case *react.LazyType:
		var resolved any
		if _, panicked := try(func() { resolved = t.Init(r.loop) }); panicked {
			return describeBuiltInComponentFrame("Lazy")
		}
		return r.describeComponentStackByType(resolved)
	case react.Symbol:
		switch t {
		case react.SuspenseList:
			return describeBuiltInComponentFrame("SuspenseList")
		case react.Suspense:
			return describeBuiltInComponentFrame("Suspense")
		}
		return ""
	}
	if jsvalue.IsFunction(typ) {
		if name, ok := functionName(typ); ok && name != "" {
			return describeBuiltInComponentFrame(name)
		}
	}
	return ""
}

// logRecoverableError calls onError and returns the digest (a string or nil).
func (r *request) logRecoverableError(err any, info *ErrorInfo) any {
	d := r.onError(err, info)
	if d == nil {
		return nil
	}
	if _, ok := d.(string); ok {
		return d
	}
	return nil
}

func (r *request) failFatal(err any) {
	onShellError, onFatalError := r.onShellError, r.onFatalError
	onShellError(err)
	onFatalError(err)
	if r.destination != nil {
		r.status = statusClosed
		r.destination.destroy(err)
	} else {
		r.status = statusClosing
		r.fatalError = err
	}
}

func (r *request) finishSuspenseListRow(row *suspenseListRow) {
	r.unblockSuspenseListRow(row.next, row.hoistables)
}

func (r *request) unblockSuspenseListRow(row *suspenseListRow, inherited *hoistableState) {
	for row != nil {
		if inherited != nil {
			hoistHoistables(row.hoistables, inherited)
			row.inheritedHoistables = inherited
		}
		if row.hasBoundaries {
			bs := row.boundaries
			row.boundaries, row.hasBoundaries = nil, false
			for _, b := range bs {
				if inherited != nil {
					hoistHoistables(b.contentState, inherited)
				}
				r.finishedTask(b, nil, nil)
			}
		}
		row.pendingTasks--
		if row.pendingTasks > 0 {
			break
		}
		inherited = row.hoistables
		row = row.next
	}
}

func (r *request) tryToResolveTogetherRow(row *suspenseListRow) {
	if !row.hasBoundaries || row.pendingTasks != len(row.boundaries) {
		return
	}
	for _, b := range row.boundaries {
		if b.pendingTasks != 1 || b.parentFlushed || r.isEligibleForOutlining(b) {
			return
		}
	}
	r.unblockSuspenseListRow(row, row.hoistables)
}

func createSuspenseListRow(previous *suspenseListRow) *suspenseListRow {
	row := &suspenseListRow{pendingTasks: 1, hoistables: createHoistableState()}
	if previous != nil && previous.pendingTasks > 0 {
		row.pendingTasks++
		row.hasBoundaries = true
		row.boundaries = []*boundary{}
		previous.next = row
	}
	return row
}

func (r *request) pushSegmentFinale(seg *segment) {
	if r.renderState.generateStaticMarkup {
		return
	}
	if seg.lastPushedText && seg.textEmbedded {
		seg.chunks = append(seg.chunks, textSeparator)
	}
}

func (r *request) renderSuspenseListRows(t *task, kp *keyPath, rows any, revealOrder any) {
	prevKeyPath, prevTreeContext, prevRow := t.keyPath, t.treeContext, t.row
	t.keyPath = kp
	items := jsvalue.Items(rows)
	n := len(items)
	var previous *suspenseListRow
	if revealOrder != "backwards" && revealOrder != "unstable_legacy-backwards" {
		for i := 0; i < n; i++ {
			previous = createSuspenseListRow(previous)
			t.row = previous
			t.treeContext = pushTreeContext(prevTreeContext, n, i)
			r.renderNode(t, items[i], i)
			previous.pendingTasks--
			if previous.pendingTasks == 0 {
				r.finishSuspenseListRow(previous)
			}
		}
	} else {
		parent := t.blockedSegment
		childrenLen := len(parent.children)
		chunksLen := len(parent.chunks)
		for i := n - 1; i >= 0; i-- {
			previous = createSuspenseListRow(previous)
			t.row = previous
			t.treeContext = pushTreeContext(prevTreeContext, n, i)
			lastPushed := true
			if i == 0 {
				lastPushed = parent.lastPushedText
			}
			seg := r.createPendingSegment(chunksLen, nil, t.formatContext, lastPushed, true)
			parent.children = append(parent.children, nil)
			copy(parent.children[childrenLen+1:], parent.children[childrenLen:])
			parent.children[childrenLen] = seg
			t.blockedSegment = seg
			row := previous
			thrown, panicked := try(func() {
				r.renderNode(t, items[i], i)
				if r.flavor == legacyFlavor {
					r.pushSegmentFinale(seg)
				} else if seg.lastPushedText && seg.textEmbedded {
					seg.chunks = append(seg.chunks, textSeparator)
				}
				seg.status = segCompleted
				if r.flavor == streamFlavor {
					r.finishedSegment(t.blockedBoundary, seg)
				}
				row.pendingTasks--
				if row.pendingTasks == 0 {
					r.finishSuspenseListRow(row)
				}
			})
			if panicked {
				if r.status == statusAborting {
					seg.status = segAborted
				} else {
					seg.status = segErrored
				}
				panic(thrown)
			}
		}
		t.blockedSegment = parent
		parent.lastPushedText = false
	}
	if prevRow != nil && previous != nil && previous.pendingTasks > 0 {
		prevRow.pendingTasks++
		previous.next = prevRow
	}
	t.treeContext = prevTreeContext
	t.row = prevRow
	t.keyPath = prevKeyPath
}

func (r *request) renderWithHooks(t *task, kp *keyPath, component any, props *react.Props, secondArg any) any {
	h := &r.hooks
	prevThenableState := t.thenableState
	t.thenableState = nil
	h.currentlyRenderingComponent = &componentIdentity{}
	h.currentlyRenderingTask = t
	h.currentlyRenderingKeyPath = kp
	h.actionStateCounter, h.localIdCounter = 0, 0
	h.actionStateMatchingIndex = -1
	h.thenableIndexCounter = 0
	h.thenableState = prevThenableState
	result := r.callComponent(component, props, secondArg)
	for h.didScheduleRenderPhaseUpdate {
		h.didScheduleRenderPhaseUpdate = false
		h.actionStateCounter, h.localIdCounter = 0, 0
		h.actionStateMatchingIndex = -1
		h.thenableIndexCounter = 0
		h.numberOfReRenders++
		h.workInProgressHook = nil
		result = r.callComponent(component, props, secondArg)
	}
	r.resetHooksState()
	return result
}

func (r *request) finishFunctionComponent(t *task, kp *keyPath, children any, hasID bool, actionStateCount, actionStateMatchingIndex int) {
	didEmitActionStateMarkers := false
	if actionStateCount != 0 && r.formState != nil {
		if seg := t.blockedSegment; seg != nil {
			didEmitActionStateMarkers = true
			for i := 0; i < actionStateCount; i++ {
				if i == actionStateMatchingIndex {
					seg.chunks = append(seg.chunks, "<!--F!-->")
				} else {
					seg.chunks = append(seg.chunks, "<!--F-->")
				}
			}
		}
	}
	prevKeyPath := t.keyPath
	t.keyPath = kp
	switch {
	case hasID:
		prevTreeContext := t.treeContext
		t.treeContext = pushTreeContext(prevTreeContext, 1, 0)
		r.renderNode(t, children, -1)
		t.treeContext = prevTreeContext
	case didEmitActionStateMarkers:
		r.renderNode(t, children, -1)
	default:
		r.renderNodeDestructive(t, children, -1)
	}
	t.keyPath = prevKeyPath
}

var emptyContextObject = &react.Object{}

type classInternals struct {
	queue    []any
	hasQueue bool
	replace  bool
}

type classUpdater struct{}

func (classUpdater) IsMounted(*react.Component) bool { return false }
func (classUpdater) EnqueueSetState(c *react.Component, payload any, _ func(), _ string) {
	if in, ok := c.Internals.(*classInternals); ok && in.hasQueue {
		in.queue = append(in.queue, payload)
	}
}
func (classUpdater) EnqueueReplaceState(c *react.Component, payload any, _ func(), _ string) {
	if in, ok := c.Internals.(*classInternals); ok {
		in.replace = true
		in.queue = []any{payload}
		in.hasQueue = true
	}
}
func (classUpdater) EnqueueForceUpdate(*react.Component, func(), string) {}

func assignNew(sources ...any) *react.Object {
	return (&react.Object{}).Assign(sources...)
}

func (r *request) renderClassComponent(t *task, kp *keyPath, typ *react.Class, props *react.Props) {
	newProps := props
	if props.Has("ref") {
		newProps = &react.Object{}
		props.Range(func(k string, v any) bool {
			if k != "ref" {
				newProps.Set(k, v)
			}
			return true
		})
	}
	if dp := typ.DefaultProps; dp != nil {
		if newProps == props {
			newProps = assignNew(newProps, props)
		}
		dp.Range(func(k string, v any) bool {
			if jsvalue.IsUndefined(newProps.Get(k)) {
				newProps.Set(k, v)
			}
			return true
		})
	}
	props = newProps
	var ctxValue any = emptyContextObject
	if typ.ContextType != nil {
		ctxValue = r.readContext(typ.ContextType)
	}
	if typ.New == nil {
		panic(&jsvalue.TypeError{Message: "Class constructor is missing"})
	}
	inst := typ.New(props, ctxValue)
	c := inst.ReactComponent()
	var initialState any = c.State
	if jsvalue.IsUndefined(initialState) {
		initialState = nil
	}
	c.Updater = classUpdater{}
	c.Props = props
	c.State = initialState
	internals := &classInternals{queue: []any{}, hasQueue: true}
	c.Internals = internals
	if typ.ContextType != nil {
		c.Context = r.readContext(typ.ContextType)
	} else {
		c.Context = emptyContextObject
	}
	if gdsfp := typ.GetDerivedStateFromProps; gdsfp != nil {
		partial := gdsfp(props, initialState)
		if !jsvalue.IsNullish(partial) {
			initialState = assignNew(initialState, partial)
		}
		c.State = initialState
	}
	_, hasSnapshot := inst.(interface {
		GetSnapshotBeforeUpdate(prevProps *react.Props, prevState any) any
	})
	cwm, hasCWM := inst.(interface{ ComponentWillMount() })
	ucwm, hasUCWM := inst.(interface{ UNSAFE_componentWillMount() })
	if typ.GetDerivedStateFromProps == nil && !hasSnapshot && (hasUCWM || hasCWM) {
		oldState := c.State
		if hasCWM {
			cwm.ComponentWillMount()
		}
		if hasUCWM {
			ucwm.UNSAFE_componentWillMount()
		}
		if !jsvalue.StrictEquals(oldState, c.State) {
			classUpdater{}.EnqueueReplaceState(c, c.State, nil, "")
		}
		if internals.hasQueue && len(internals.queue) > 0 {
			queue, replace := internals.queue, internals.replace
			internals.queue, internals.hasQueue, internals.replace = nil, false, false
			if replace && len(queue) == 1 {
				c.State = queue[0]
			} else {
				var nextState any = c.State
				start := 0
				if replace {
					nextState = queue[0]
					start = 1
				}
				dontMutate := true
				for i := start; i < len(queue); i++ {
					partial := queue[i]
					var partialState any = partial
					if jsvalue.IsFunction(partial) {
						partialState = callUpdater(partial, nextState, props)
					}
					if !jsvalue.IsNullish(partialState) {
						if dontMutate {
							dontMutate = false
							nextState = assignNew(nextState, partialState)
						} else if o, ok := nextState.(*react.Object); ok {
							o.Assign(partialState)
						}
					}
				}
				c.State = nextState
			}
		} else {
			internals.queue, internals.hasQueue = nil, false
		}
	}
	children := inst.Render()
	if r.status == statusAborting {
		panic(react.Throw(nil))
	}
	prevKeyPath := t.keyPath
	t.keyPath = kp
	r.renderNodeDestructive(t, children, -1)
	t.keyPath = prevKeyPath
}

func callUpdater(fn any, state any, props *react.Props) any {
	switch f := fn.(type) {
	case func(any, *react.Props) any:
		return f(state, props)
	case func(any) any:
		return f(state)
	}
	v, _ := jsvalue.Call(fn, state, props, jsvalue.Undefined{})
	return v
}

func (r *request) isEligibleForOutlining(b *boundary) bool {
	if r.flavor == legacyFlavor {
		return b.byteSize > 500 && b.contentPreamble == nil
	}
	return (b.byteSize > 500 || hasSuspenseyContent(b.contentState)) && b.contentPreamble == nil
}

func (r *request) renderElement(t *task, kp *keyPath, typ any, props *react.Props, ref any) {
	if jsvalue.IsFunction(typ) {
		if cls, ok := typ.(*react.Class); ok {
			r.renderClassComponent(t, kp, cls, props)
			return
		}
		children := r.renderWithHooks(t, kp, typ, props, jsvalue.Undefined{})
		if r.status == statusAborting {
			panic(react.Throw(nil))
		}
		r.finishFunctionComponent(t, kp, children, r.hooks.localIdCounter != 0, r.hooks.actionStateCounter, r.hooks.actionStateMatchingIndex)
		return
	}
	if tag, ok := typ.(string); ok {
		r.renderHostElement(t, kp, tag, props)
		return
	}
	if sym, ok := typ.(react.Symbol); ok {
		switch sym {
		case react.LegacyHidden, react.StrictMode, react.Profiler, react.Fragment:
			prev := t.keyPath
			t.keyPath = kp
			r.renderNodeDestructive(t, props.Get("children"), -1)
			t.keyPath = prev
			return
		case react.Activity:
			seg := t.blockedSegment
			if props.Get("mode") != "hidden" {
				if !r.renderState.generateStaticMarkup {
					seg.chunks = append(seg.chunks, "<!--&-->")
				}
				seg.lastPushedText = false
				prev := t.keyPath
				t.keyPath = kp
				r.renderNode(t, props.Get("children"), -1)
				t.keyPath = prev
				if !r.renderState.generateStaticMarkup {
					seg.chunks = append(seg.chunks, "<!--/&-->")
				}
				seg.lastPushedText = false
			}
			return
		case react.SuspenseList:
			r.renderSuspenseList(t, kp, props)
			return
		case react.ViewTransition, react.Scope:
			panic(newError("ReactDOMServer does not yet support scope components."))
		case react.Suspense:
			r.renderSuspenseBoundary(t, kp, props)
			return
		}
	}
	if jsvalue.TypeOf(typ) == "object" && !jsvalue.IsNull(typ) {
		switch x := typ.(type) {
		case *react.ForwardRefType:
			newProps := props
			if props.Has("ref") {
				newProps = &react.Object{}
				props.Range(func(k string, v any) bool {
					if k != "ref" {
						newProps.Set(k, v)
					}
					return true
				})
			}
			children := r.renderWithHooks(t, kp, x, newProps, ref)
			r.finishFunctionComponent(t, kp, children, r.hooks.localIdCounter != 0, r.hooks.actionStateCounter, r.hooks.actionStateMatchingIndex)
			return
		case *react.MemoType:
			r.renderElement(t, kp, x.Type, props, ref)
			return
		case *react.Context:
			children := props.Get("children")
			prevKeyPath := t.keyPath
			value := props.Get("value")
			prevValue := r.readContext(x)
			r.setContextValue(x, value)
			prevSnapshot := r.currentActiveSnapshot
			depth := 0
			if prevSnapshot != nil {
				depth = prevSnapshot.depth + 1
			}
			snap := &contextSnapshot{parent: prevSnapshot, depth: depth, context: x, parentValue: prevValue, value: value}
			r.currentActiveSnapshot = snap
			t.context = snap
			t.keyPath = kp
			r.renderNodeDestructive(t, children, -1)
			cur := r.currentActiveSnapshot
			if cur == nil {
				panic(newError("Tried to pop a Context at the root of the app. This is a bug in React."))
			}
			r.setContextValue(cur.context, cur.parentValue)
			r.currentActiveSnapshot = cur.parent
			t.context = cur.parent
			t.keyPath = prevKeyPath
			return
		case *react.Consumer:
			render := props.Get("children")
			value := r.readContext(x.Context)
			result, ok := jsvalue.Call(render, value)
			if !ok {
				panic(&jsvalue.TypeError{Message: "props is not a function"})
			}
			prev := t.keyPath
			t.keyPath = kp
			r.renderNodeDestructive(t, result, -1)
			t.keyPath = prev
			return
		case *react.LazyType:
			resolved := x.Init(r.loop)
			if r.status == statusAborting {
				panic(react.Throw(nil))
			}
			r.renderElement(t, kp, resolved, props, ref)
			return
		}
	}
	var got string
	if jsvalue.IsNullish(typ) {
		got = jsvalue.ToString(typ)
	} else {
		got = jsvalue.TypeOf(typ)
	}
	panic(newError("Element type is invalid: expected a string (for built-in components) or a class/function (for composite components) but got: " + got + "."))
}

func (r *request) renderHostElement(t *task, kp *keyPath, typ string, props *react.Props) {
	seg := t.blockedSegment
	children := r.pushStartInstance(&seg.chunks, typ, props, t.blockedPreamble, t.hoistableState, t.formatContext, seg.lastPushedText)
	seg.lastPushedText = false
	prevFC := t.formatContext
	prevKeyPath := t.keyPath
	t.keyPath = kp
	t.formatContext = getChildFormatContext(prevFC, typ, props)
	if t.formatContext.insertionMode == htmlHeadMode {
		preambleSeg := r.createPendingSegment(0, nil, t.formatContext, false, false)
		seg.preambleChildren = append(seg.preambleChildren, preambleSeg)
		t.blockedSegment = preambleSeg
		func() {
			defer func() { t.blockedSegment = seg }()
			preambleSeg.status = segRendering
			r.renderNode(t, children, -1)
			if r.flavor == legacyFlavor {
				r.pushSegmentFinale(preambleSeg)
			} else if preambleSeg.lastPushedText && preambleSeg.textEmbedded {
				preambleSeg.chunks = append(preambleSeg.chunks, textSeparator)
			}
			preambleSeg.status = segCompleted
			if r.flavor == streamFlavor {
				r.finishedSegment(t.blockedBoundary, preambleSeg)
			}
		}()
	} else {
		r.renderNode(t, children, -1)
	}
	t.formatContext = prevFC
	t.keyPath = prevKeyPath
	rs := r.resumableState
	pushEnd := true
	switch typ {
	case "title", "style", "script", "area", "base", "br", "col", "embed", "hr", "img", "input", "keygen", "link", "meta", "param", "source", "track", "wbr":
		pushEnd = false
	case "body":
		if prevFC.insertionMode <= htmlHTMLMode {
			rs.hasBody = true
			pushEnd = false
		}
	case "html":
		if prevFC.insertionMode == rootHTMLMode {
			rs.hasHtml = true
			pushEnd = false
		}
	case "head":
		if prevFC.insertionMode <= htmlHTMLMode {
			pushEnd = false
		}
	}
	if pushEnd {
		seg.chunks = append(seg.chunks, endChunkForTag(typ))
	}
	seg.lastPushedText = false
}

func (r *request) renderSuspenseList(t *task, kp *keyPath, props *react.Props) {
	children := props.Get("children")
	revealOrder := props.Get("revealOrder")
	if revealOrder == "forwards" || revealOrder == "backwards" || revealOrder == "unstable_legacy-backwards" {
		if jsvalue.IsArray(children) {
			r.renderSuspenseListRows(t, kp, children, revealOrder)
			return
		}
		if seq, ok := react.IteratorOf(children); ok {
			any := false
			for range seq {
				any = true
			}
			if any {
				r.renderSuspenseListRows(t, kp, children, revealOrder)
			}
			return
		}
	}
	if revealOrder == "together" {
		prevKeyPath, prevRow := t.keyPath, t.row
		row := createSuspenseListRow(nil)
		t.row = row
		row.boundaries, row.hasBoundaries = []*boundary{}, true
		row.together = true
		t.keyPath = kp
		r.renderNodeDestructive(t, children, -1)
		row.pendingTasks--
		if row.pendingTasks == 0 {
			r.finishSuspenseListRow(row)
		}
		t.keyPath = prevKeyPath
		t.row = prevRow
		if prevRow != nil && row.pendingTasks > 0 {
			prevRow.pendingTasks++
			row.next = prevRow
		}
		return
	}
	prev := t.keyPath
	t.keyPath = kp
	r.renderNodeDestructive(t, children, -1)
	t.keyPath = prev
}

func (r *request) renderSuspenseBoundary(t *task, kp *keyPath, props *react.Props) {
	prevKeyPath := t.keyPath
	parentFC := t.formatContext
	prevRow := t.row
	parentBoundary := t.blockedBoundary
	parentPreamble := t.blockedPreamble
	parentHoistable := t.hoistableState
	parentSegment := t.blockedSegment
	fallback := props.Get("fallback")
	content := props.Get("children")
	fallbackAbortSet := newSet[*task]()
	var nb *boundary
	if r.flavor == streamFlavor && t.formatContext.insertionMode < htmlMode {
		nb = r.createSuspenseBoundary(t.row, fallbackAbortSet, &preambleState{}, &preambleState{})
	} else {
		nb = r.createSuspenseBoundary(t.row, fallbackAbortSet, nil, nil)
	}
	boundarySegment := r.createPendingSegment(len(parentSegment.chunks), nb, t.formatContext, false, false)
	parentSegment.children = append(parentSegment.children, boundarySegment)
	parentSegment.lastPushedText = false
	contentRoot := r.createPendingSegment(0, nil, t.formatContext, false, false)
	contentRoot.parentFlushed = true

	t.blockedBoundary = nb
	t.blockedPreamble = nb.contentPreamble
	t.hoistableState = nb.contentState
	t.blockedSegment = contentRoot
	t.keyPath = kp
	t.formatContext = getSuspenseContentFormatContext(r.resumableState, parentFC)
	t.row = nil
	contentRoot.status = segRendering
	skipFallback := false
	func() {
		defer func() {
			t.blockedBoundary = parentBoundary
			t.blockedPreamble = parentPreamble
			t.hoistableState = parentHoistable
			t.blockedSegment = parentSegment
			t.keyPath = prevKeyPath
			t.formatContext = parentFC
			t.row = prevRow
		}()
		thrown, panicked := try(func() {
			r.renderNode(t, content, -1)
			if r.flavor == legacyFlavor {
				r.pushSegmentFinale(contentRoot)
			} else if contentRoot.lastPushedText && contentRoot.textEmbedded {
				contentRoot.chunks = append(contentRoot.chunks, textSeparator)
			}
			contentRoot.status = segCompleted
			if r.flavor == streamFlavor {
				r.finishedSegment(nb, contentRoot)
			}
			queueCompletedSegment(nb, contentRoot)
			if nb.pendingTasks == 0 && nb.status == boundaryPending {
				nb.status = boundaryCompleted
				if !r.isEligibleForOutlining(nb) {
					if prevRow != nil {
						prevRow.pendingTasks--
						if prevRow.pendingTasks == 0 {
							r.finishSuspenseListRow(prevRow)
						}
					}
					if r.pendingRootTasks == 0 && t.blockedPreamble != nil {
						r.preparePreamble()
					}
					skipFallback = true
				}
			} else if prevRow != nil && prevRow.together {
				r.tryToResolveTogetherRow(prevRow)
			}
		})
		if panicked {
			nb.status = boundaryClientRendered
			var errVal any
			if r.status == statusAborting {
				contentRoot.status = segAborted
				errVal = r.fatalError
			} else {
				contentRoot.status = segErrored
				errVal = react.Thrown(thrown)
			}
			info := r.getThrownInfo(t.componentStack)
			nb.errorDigest = r.logRecoverableError(errVal, info)
		}
	}()
	if skipFallback {
		return
	}
	ft := r.createRenderTask(nil, fallback, -1, parentBoundary, boundarySegment, nb.fallbackPreamble, nb.fallbackState, fallbackAbortSet,
		&keyPath{parent: kp.parent, name: "Suspense Fallback", key: kp.key},
		getSuspenseFallbackFormatContext(r.resumableState, t.formatContext), t.context, t.treeContext, t.row,
		replaceSuspenseComponentStackWithSuspenseFallbackStack(t.componentStack))
	pushComponentStack(ft)
	r.pingedTasks = append(r.pingedTasks, ft)
}

func (r *request) renderNodeDestructive(t *task, node any, childIndex int) {
	t.node = node
	t.childIndex = childIndex
	prevStack := t.componentStack
	pushComponentStack(t)
	r.retryNode(t)
	t.componentStack = prevStack
}

// objectKeysForMessage is Object.keys(node) for the "Objects are not valid
// as a React child" message.
func objectKeysForMessage(node any) []string {
	switch x := node.(type) {
	case *react.ForwardRefType:
		keys := []string{"$$typeof", "render"}
		if x.DisplayName != "" {
			keys = append(keys, "displayName")
		}
		return keys
	case *react.MemoType:
		keys := []string{"$$typeof", "type", "compare"}
		if x.DisplayName != "" {
			keys = append(keys, "displayName")
		}
		return keys
	case *react.Consumer:
		return []string{"$$typeof", "_context"}
	}
	return jsvalue.ObjectKeys(node)
}

func objectTag(node any) string {
	if _, ok := node.(error); ok {
		return "[object Error]"
	}
	return "[object Object]"
}

func (r *request) retryNode(t *task) {
	node, childIndex := t.node, t.childIndex
	if jsvalue.IsNull(node) {
		return
	}
	switch jsvalue.TypeOf(node) {
	case "object":
		switch x := node.(type) {
		case *react.Element:
			typ, props := x.Type, x.Props
			if props == nil {
				props = &react.Object{}
			}
			var ref any
			if v, ok := props.Lookup("ref"); ok && !jsvalue.IsUndefined(v) {
				ref = v
			}
			name := r.getComponentNameFromType(typ)
			var keyOrIndex any
			if x.Key == nil {
				if childIndex == -1 {
					keyOrIndex = float64(0)
				} else {
					keyOrIndex = float64(childIndex)
				}
			} else {
				keyOrIndex = *x.Key
			}
			r.renderElement(t, &keyPath{parent: t.keyPath, name: name, key: keyOrIndex}, typ, props, ref)
			return
		case *react.Portal:
			panic(newError("Portals are not currently supported by the server renderer. Render them conditionally so that they only appear on the client render."))
		case *react.LazyType:
			resolved := x.Init(r.loop)
			if r.status == statusAborting {
				panic(react.Throw(nil))
			}
			r.renderNodeDestructive(t, resolved, childIndex)
			return
		}
		if jsvalue.IsArray(node) {
			r.renderChildrenArray(t, jsvalue.Items(node), childIndex)
			return
		}
		if seq, ok := react.IteratorOf(node); ok {
			var items []any
			for v := range seq {
				items = append(items, v)
			}
			if len(items) > 0 {
				r.renderChildrenArray(t, items, childIndex)
			}
			return
		}
		if th, ok := react.IsThenable(node); ok {
			t.thenableState = nil
			r.renderNodeDestructive(t, r.unwrapThenable(th), childIndex)
			return
		}
		if c, ok := node.(*react.Context); ok {
			r.renderNodeDestructive(t, r.readContext(c), childIndex)
			return
		}
		tag := objectTag(node)
		found := tag
		if tag == "[object Object]" {
			found = "object with keys {" + strings.Join(objectKeysForMessage(node), ", ") + "}"
		}
		panic(newError(react.NotValidChildMessage(found)))
	case "string":
		if seg := t.blockedSegment; seg != nil {
			seg.lastPushedText = r.pushTextInstance(seg, node.(string))
		}
	case "number", "bigint":
		if seg := t.blockedSegment; seg != nil {
			seg.lastPushedText = r.pushTextInstance(seg, jsvalue.ToString(node))
		}
	}
}

func (r *request) pushTextInstance(seg *segment, text string) bool {
	if r.renderState.generateStaticMarkup {
		seg.chunks = append(seg.chunks, escapeString(text))
		return false
	}
	if text == "" {
		return seg.lastPushedText
	}
	if seg.lastPushedText {
		seg.chunks = append(seg.chunks, textSeparator)
	}
	seg.chunks = append(seg.chunks, escapeString(text))
	return true
}

func (r *request) renderChildrenArray(t *task, children []any, childIndex int) {
	prevKeyPath := t.keyPath
	if childIndex != -1 {
		t.keyPath = &keyPath{parent: t.keyPath, name: "Fragment", key: float64(childIndex)}
	}
	prevTreeContext := t.treeContext
	n := len(children)
	for j := 0; j < n; j++ {
		t.treeContext = pushTreeContext(prevTreeContext, n, j)
		r.renderNode(t, children[j], j)
	}
	t.treeContext = prevTreeContext
	t.keyPath = prevKeyPath
}

func (r *request) spawnNewSuspendedRenderTask(t *task, thenableState *[]react.Thenable) *task {
	seg := t.blockedSegment
	newSeg := r.createPendingSegment(len(seg.chunks), nil, t.formatContext, seg.lastPushedText, true)
	seg.children = append(seg.children, newSeg)
	seg.lastPushedText = false
	return r.createRenderTask(thenableState, t.node, t.childIndex, t.blockedBoundary, newSeg, t.blockedPreamble, t.hoistableState,
		t.abortSet, t.keyPath, t.formatContext, t.context, t.treeContext, t.row, t.componentStack)
}

// try runs f, reporting a panic (a JavaScript throw) instead of propagating it.
func try(f func()) (thrown any, panicked bool) {
	defer func() {
		if p := recover(); p != nil {
			thrown, panicked = p, true
		}
	}()
	f()
	return nil, false
}

func (r *request) renderNode(t *task, node any, childIndex int) {
	prevFC, prevContext, prevKeyPath, prevTreeContext, prevStack := t.formatContext, t.context, t.keyPath, t.treeContext, t.componentStack
	seg := t.blockedSegment
	childrenLen, chunksLen := len(seg.children), len(seg.chunks)
	thrown, panicked := try(func() { r.renderNodeDestructive(t, node, childIndex) })
	if !panicked {
		return
	}
	r.resetHooksState()
	seg.children = seg.children[:childrenLen]
	seg.chunks = seg.chunks[:chunksLen]
	var x any
	isSuspense := thrown == suspenseException
	if isSuspense {
		x = r.getSuspendedThenable()
	} else {
		x = react.Thrown(thrown)
	}
	if r.status != statusAborting && jsvalue.TypeOf(x) == "object" && !jsvalue.IsNull(x) {
		if th, ok := react.IsThenable(x); ok {
			var ts *[]react.Thenable
			if isSuspense {
				ts = r.getThenableStateAfterSuspending()
			}
			ping := r.spawnNewSuspendedRenderTask(t, ts).ping
			jsloop.Then(r.loop, th, func(any) { ping() }, func(any) { ping() })
			t.formatContext, t.context, t.keyPath, t.treeContext, t.componentStack = prevFC, prevContext, prevKeyPath, prevTreeContext, prevStack
			r.switchContext(prevContext)
			return
		}
	}
	t.formatContext, t.context, t.keyPath, t.treeContext = prevFC, prevContext, prevKeyPath, prevTreeContext
	r.switchContext(prevContext)
	panic(react.Throw(x))
}

func (r *request) abortTaskSoft(t *task) {
	if seg := t.blockedSegment; seg != nil {
		seg.status = segAborted
		r.finishedTask(t.blockedBoundary, t.row, seg)
	}
}

func (r *request) abortTask(t *task, err any) {
	b, seg := t.blockedBoundary, t.blockedSegment
	if seg != nil {
		if seg.status == segRendering {
			return
		}
		seg.status = segAborted
	}
	info := r.getThrownInfo(t.componentStack)
	if b == nil {
		if r.status != statusClosing && r.status != statusClosed {
			r.logRecoverableError(err, info)
			r.failFatal(err)
			return
		}
	} else {
		if b.status != boundaryClientRendered {
			b.status = boundaryClientRendered
			digest := r.logRecoverableError(err, info)
			b.status = boundaryClientRendered
			b.errorDigest = digest
			if b.parentFlushed {
				r.clientRenderedBoundaries = append(r.clientRenderedBoundaries, b)
			}
		}
		b.pendingTasks--
		if row := b.row; row != nil {
			row.pendingTasks--
			if row.pendingTasks == 0 {
				r.finishSuspenseListRow(row)
			}
		}
		b.fallbackAbortableTasks.forEach(func(ft *task) { r.abortTask(ft, err) })
		b.fallbackAbortableTasks.clear()
	}
	if row := t.row; row != nil {
		row.pendingTasks--
		if row.pendingTasks == 0 {
			r.finishSuspenseListRow(row)
		}
	}
	r.allPendingTasks--
	if r.allPendingTasks == 0 {
		r.completeAll()
	}
}

func (r *request) safelyEmitEarlyPreloads(shellComplete bool) {
	thrown, panicked := try(func() {
		st := r.renderState
		if st.onHeaders == nil || st.headers == nil {
			return
		}
		headers := st.headers
		st.headers = nil
		link := headers.preconnects
		if headers.fontPreloads != "" {
			if link != "" {
				link += ", "
			}
			link += headers.fontPreloads
		}
		if headers.highImagePreloads != "" {
			if link != "" {
				link += ", "
			}
			link += headers.highImagePreloads
		}
		if !shellComplete {
			done := false
			st.styles.forEach(func(_ string, q *styleQueue) {
				if done || headers.remainingCapacity <= 0 {
					return
				}
				q.sheets.forEach(func(_ string, s *stylesheet) {
					if done || headers.remainingCapacity <= 0 {
						return
					}
					p := s.props
					header := getPreloadAsHeader(p.Get("href"), "style", newObj(
						"crossOrigin", p.Get("crossOrigin"),
						"integrity", p.Get("integrity"),
						"nonce", p.Get("nonce"),
						"type", p.Get("type"),
						"fetchPriority", p.Get("fetchPriority"),
						"referrerPolicy", p.Get("referrerPolicy"),
						"media", p.Get("media"),
					))
					headers.remainingCapacity -= utf16Len(header) + 2
					if headers.remainingCapacity >= 0 {
						if link != "" {
							link += ", "
						}
						link += header
					} else {
						done = true
					}
				})
			})
		}
		if link != "" {
			st.onHeaders(map[string]string{"Link": link})
		} else {
			st.onHeaders(map[string]string{})
		}
	})
	if panicked {
		r.logRecoverableError(react.Thrown(thrown), &ErrorInfo{})
	}
}

func (r *request) completeShell() {
	r.safelyEmitEarlyPreloads(true)
	r.preparePreamble()
	r.onShellError = noopErrCallback
	r.onShellReady()
}

func (r *request) completeAll() {
	r.safelyEmitEarlyPreloads(true)
	r.preparePreamble()
	r.onAllReady()
}

func queueCompletedSegment(b *boundary, seg *segment) {
	if len(seg.chunks) == 0 && len(seg.children) == 1 && seg.children[0].boundary == nil && seg.children[0].id == -1 {
		child := seg.children[0]
		child.id = seg.id
		child.parentFlushed = true
		if child.status == segCompleted || child.status == segAborted || child.status == segErrored {
			queueCompletedSegment(b, child)
		}
	} else {
		b.completedSegments = append(b.completedSegments, seg)
	}
}

func (r *request) finishedSegment(b *boundary, seg *segment) {
	size := 0
	for _, c := range seg.chunks {
		size += len(c)
	}
	if b == nil {
		r.byteSize += size
	} else {
		b.byteSize += size
	}
}

func (r *request) finishedTask(b *boundary, row *suspenseListRow, seg *segment) {
	if row != nil {
		row.pendingTasks--
		if row.pendingTasks == 0 {
			r.finishSuspenseListRow(row)
		} else if row.together {
			r.tryToResolveTogetherRow(row)
		}
	}
	r.allPendingTasks--
	if b == nil {
		if seg != nil && seg.parentFlushed {
			if r.completedRootSegment != nil {
				panic(newError("There can only be one root segment. This is a bug in React."))
			}
			r.completedRootSegment = seg
		}
		r.pendingRootTasks--
		if r.pendingRootTasks == 0 {
			r.completeShell()
		}
	} else {
		b.pendingTasks--
		if b.status != boundaryClientRendered {
			if b.pendingTasks == 0 {
				if b.status == boundaryPending {
					b.status = boundaryCompleted
				}
				if seg != nil && seg.parentFlushed && (seg.status == segCompleted || seg.status == segAborted) {
					queueCompletedSegment(b, seg)
				}
				if b.parentFlushed {
					r.completedBoundaries = append(r.completedBoundaries, b)
				}
				if b.status == boundaryCompleted {
					brow := b.row
					if brow != nil {
						hoistHoistables(brow.hoistables, b.contentState)
					}
					if !r.isEligibleForOutlining(b) {
						b.fallbackAbortableTasks.forEach(r.abortTaskSoft)
						b.fallbackAbortableTasks.clear()
						if brow != nil {
							brow.pendingTasks--
							if brow.pendingTasks == 0 {
								r.finishSuspenseListRow(brow)
							}
						}
					}
					if r.pendingRootTasks == 0 && b.contentPreamble != nil {
						r.preparePreamble()
					}
				} else if b.status == boundaryPostponed && b.row != nil {
					brow := b.row
					brow.pendingTasks--
					if brow.pendingTasks == 0 {
						r.finishSuspenseListRow(brow)
					}
				}
			} else {
				if seg != nil && seg.parentFlushed && (seg.status == segCompleted || seg.status == segAborted) {
					queueCompletedSegment(b, seg)
					if len(b.completedSegments) == 1 && b.parentFlushed {
						r.partialBoundaries = append(r.partialBoundaries, b)
					}
				}
				if brow := b.row; brow != nil && brow.together {
					r.tryToResolveTogetherRow(brow)
				}
			}
		}
	}
	if r.allPendingTasks == 0 {
		r.completeAll()
	}
}

func (r *request) performWork() {
	if r.status == statusClosed || r.status == statusClosing {
		return
	}
	prevPerforming := r.performingWork
	prevResumable := r.hooks.currentResumableState
	r.performingWork = true
	r.hooks.currentResumableState = r.resumableState
	defer func() {
		r.hooks.currentResumableState = prevResumable
		r.performingWork = prevPerforming
	}()
	thrown, panicked := try(func() {
		i := 0
		for ; i < len(r.pingedTasks); i++ {
			t := r.pingedTasks[i]
			seg := t.blockedSegment
			if seg == nil || seg.status != segPending {
				continue
			}
			seg.status = segRendering
			r.switchContext(t.context)
			childrenLen, chunksLen := len(seg.children), len(seg.chunks)
			thrownValue, failed := try(func() {
				r.retryNode(t)
				if r.flavor == legacyFlavor {
					r.pushSegmentFinale(seg)
				} else if seg.lastPushedText && seg.textEmbedded {
					seg.chunks = append(seg.chunks, textSeparator)
				}
				t.abortSet.delete(t)
				seg.status = segCompleted
				if r.flavor == streamFlavor {
					r.finishedSegment(t.blockedBoundary, seg)
				}
				r.finishedTask(t.blockedBoundary, t.row, seg)
			})
			if !failed {
				continue
			}
			r.resetHooksState()
			seg.children = seg.children[:childrenLen]
			seg.chunks = seg.chunks[:chunksLen]
			isSuspense := thrownValue == suspenseException
			var x any
			switch {
			case isSuspense:
				x = r.getSuspendedThenable()
			case r.status == statusAborting:
				x = r.fatalError
			default:
				x = react.Thrown(thrownValue)
			}
			if th, ok := react.IsThenable(x); ok && jsvalue.TypeOf(x) == "object" {
				seg.status = segPending
				if isSuspense {
					t.thenableState = r.getThenableStateAfterSuspending()
				} else {
					t.thenableState = nil
				}
				ping := t.ping
				jsloop.Then(r.loop, th, func(any) { ping() }, func(any) { ping() })
				continue
			}
			info := r.getThrownInfo(t.componentStack)
			t.abortSet.delete(t)
			seg.status = segErrored
			b, row := t.blockedBoundary, t.row
			if row != nil {
				row.pendingTasks--
				if row.pendingTasks == 0 {
					r.finishSuspenseListRow(row)
				}
			}
			r.allPendingTasks--
			digest := r.logRecoverableError(x, info)
			if b == nil {
				r.failFatal(x)
			} else {
				b.pendingTasks--
				if b.status != boundaryClientRendered {
					b.status = boundaryClientRendered
					b.errorDigest = digest
					if brow := b.row; brow != nil {
						brow.pendingTasks--
						if brow.pendingTasks == 0 {
							r.finishSuspenseListRow(brow)
						}
					}
					if b.parentFlushed {
						r.clientRenderedBoundaries = append(r.clientRenderedBoundaries, b)
					}
					if r.pendingRootTasks == 0 && b.contentPreamble != nil {
						r.preparePreamble()
					}
				}
			}
			if r.allPendingTasks == 0 {
				r.completeAll()
			}
		}
		r.pingedTasks = splice(r.pingedTasks, i)
		if r.destination != nil {
			r.flushCompletedQueues(r.destination)
		}
	})
	if panicked {
		err := react.Thrown(thrown)
		r.logRecoverableError(err, &ErrorInfo{})
		r.failFatal(err)
	}
}

func (r *request) preparePreambleFromSubtree(seg *segment, collected *[][]*segment) bool {
	if len(seg.preambleChildren) > 0 {
		*collected = append(*collected, seg.preambleChildren)
	}
	pending := false
	for _, child := range seg.children {
		pending = r.preparePreambleFromSegment(child, collected) || pending
	}
	return pending
}

func (r *request) preparePreambleFromSegment(seg *segment, collected *[][]*segment) bool {
	b := seg.boundary
	if b == nil {
		return r.preparePreambleFromSubtree(seg, collected)
	}
	preamble, fallbackPreamble := b.contentPreamble, b.fallbackPreamble
	if preamble == nil || fallbackPreamble == nil {
		return false
	}
	switch b.status {
	case boundaryCompleted:
		hoistPreambleState(r.renderState, preamble)
		r.byteSize += b.byteSize
		if len(b.completedSegments) == 0 {
			panic(newError("A previously unvisited boundary must have exactly one root segment. This is a bug in React."))
		}
		return r.preparePreambleFromSubtree(b.completedSegments[0], collected)
	case boundaryPostponed, boundaryClientRendered:
		if seg.status == segCompleted {
			hoistPreambleState(r.renderState, fallbackPreamble)
			return r.preparePreambleFromSubtree(seg, collected)
		}
	}
	return true
}

func hoistPreambleState(st *renderState, p *preambleState) {
	rp := st.preamble
	if rp.htmlChunks == nil && p.htmlChunks != nil {
		rp.htmlChunks = p.htmlChunks
	}
	if rp.headChunks == nil && p.headChunks != nil {
		rp.headChunks = p.headChunks
	}
	if rp.bodyChunks == nil && p.bodyChunks != nil {
		rp.bodyChunks = p.bodyChunks
	}
}

func (r *request) preparePreamble() {
	if r.completedRootSegment == nil || r.completedPreambleSegments != nil {
		return
	}
	collected := [][]*segment{}
	origByteSize := r.byteSize
	pending := r.preparePreambleFromSegment(r.completedRootSegment, &collected)
	p := r.renderState.preamble
	if !pending || (p.headChunks != nil && p.bodyChunks != nil) {
		r.completedPreambleSegments = collected
	} else {
		r.byteSize = origByteSize
	}
}

func (r *request) enqueueFlush() {
	if r.flushScheduled || len(r.pingedTasks) != 0 || r.destination == nil {
		return
	}
	r.flushScheduled = true
	if r.flavor == legacyFlavor {
		if d := r.destination; d != nil {
			r.flushCompletedQueues(d)
		} else {
			r.flushScheduled = false
		}
		return
	}
	r.loop.SetImmediate(func() {
		if d := r.destination; d != nil {
			r.flushCompletedQueues(d)
		} else {
			r.flushScheduled = false
		}
	})
}

func (r *request) startFlowing(d destination) {
	if r.status == statusClosing {
		r.status = statusClosed
		d.destroy(r.fatalError)
		return
	}
	if r.status == statusClosed || r.destination != nil {
		return
	}
	r.destination = d
	if thrown, panicked := try(func() { r.flushCompletedQueues(d) }); panicked {
		err := react.Thrown(thrown)
		r.logRecoverableError(err, &ErrorInfo{})
		r.failFatal(err)
	}
}

// abortReason converts abort(reason) into the error tasks are aborted with.
func abortReason(reason any) any {
	if jsvalue.IsUndefined(reason) {
		return newError("The render was aborted by the server without a reason.")
	}
	if _, ok := react.IsThenable(reason); ok && jsvalue.TypeOf(reason) == "object" {
		return newError("The render was aborted by the server with a promise.")
	}
	return reason
}

func (r *request) abort(reason any) {
	if r.status == statusOpen || r.status == statusOpening {
		r.status = statusAborting
	}
	thrown, panicked := try(func() {
		if r.abortableTasks.size() > 0 {
			err := abortReason(reason)
			r.fatalError = err
			r.abortableTasks.forEach(func(t *task) { r.abortTask(t, err) })
			r.abortableTasks.clear()
		}
		if r.destination != nil {
			r.flushCompletedQueues(r.destination)
		}
	})
	if panicked {
		err := react.Thrown(thrown)
		r.logRecoverableError(err, &ErrorInfo{})
		r.failFatal(err)
	}
}

// getComponentNameFromType is React's getComponentNameFromType; ok is false
// for null.
func (r *request) getComponentNameFromType(typ any) any {
	if jsvalue.IsNullish(typ) {
		return nil
	}
	if jsvalue.IsFunction(typ) {
		if name, ok := functionName(typ); ok && name != "" {
			return name
		}
		return nil
	}
	if s, ok := typ.(string); ok {
		return s
	}
	switch t := typ.(type) {
	case react.Symbol:
		switch t {
		case react.Fragment:
			return "Fragment"
		case react.Profiler:
			return "Profiler"
		case react.StrictMode:
			return "StrictMode"
		case react.Suspense:
			return "Suspense"
		case react.SuspenseList:
			return "SuspenseList"
		case react.Activity:
			return "Activity"
		}
		return nil
	case *react.Portal:
		return "Portal"
	case *react.Context:
		if t.DisplayName != "" {
			return t.DisplayName
		}
		return "Context"
	case *react.Consumer:
		name := "Context"
		if t.Context != nil && t.Context.DisplayName != "" {
			name = t.Context.DisplayName
		}
		return name + ".Consumer"
	case *react.ForwardRefType:
		if t.DisplayName != "" {
			return t.DisplayName
		}
		if t.Name != "" {
			return "ForwardRef(" + t.Name + ")"
		}
		return "ForwardRef"
	case *react.MemoType:
		if t.DisplayName != "" {
			return t.DisplayName
		}
		if n := r.getComponentNameFromType(t.Type); n != nil {
			return n
		}
		return "Memo"
	case *react.LazyType:
		var name any
		try(func() { name = r.getComponentNameFromType(t.Init(r.loop)) })
		return name
	}
	return nil
}

// functionName is fn.displayName || fn.name for the Go function types that
// carry names.
func functionName(fn any) (string, bool) {
	switch f := fn.(type) {
	case *react.FunctionComponent:
		if f.DisplayName != "" {
			return f.DisplayName, true
		}
		return f.Name, true
	case *react.Class:
		if f.DisplayName != "" {
			return f.DisplayName, true
		}
		return f.Name, true
	}
	return "", false
}

func newObj(kv ...any) *react.Object { return react.Obj(kv...) }

func lower(s string) string { return strings.ToLower(s) }

// infinity is the legacy renderer's progressiveChunkSize.
var infinity = math.Inf(1)
