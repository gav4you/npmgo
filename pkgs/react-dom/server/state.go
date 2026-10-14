package server

import (
	"github.com/gav4you/npmgo/internal/jsvalue"
	"github.com/gav4you/npmgo/pkgs/react"
)

// orderedSet is a JavaScript Set: insertion-ordered, visiting additions made
// during forEach and skipping deletions.
type orderedSet[T comparable] struct {
	items []T
	live  []bool
	index map[T]int
	n     int
}

func newSet[T comparable]() *orderedSet[T] { return &orderedSet[T]{index: map[T]int{}} }

func (s *orderedSet[T]) add(v T) {
	if _, ok := s.index[v]; ok {
		return
	}
	s.index[v] = len(s.items)
	s.items = append(s.items, v)
	s.live = append(s.live, true)
	s.n++
}

func (s *orderedSet[T]) has(v T) bool { _, ok := s.index[v]; return ok }

func (s *orderedSet[T]) delete(v T) bool {
	i, ok := s.index[v]
	if !ok {
		return false
	}
	delete(s.index, v)
	s.live[i] = false
	s.n--
	return true
}

func (s *orderedSet[T]) size() int { return s.n }

func (s *orderedSet[T]) clear() {
	s.items, s.live, s.index, s.n = nil, nil, map[T]int{}, 0
}

func (s *orderedSet[T]) forEach(f func(T)) {
	for i := 0; i < len(s.items); i++ {
		if s.live[i] {
			f(s.items[i])
		}
	}
}

// orderedMap is a JavaScript Map with insertion-ordered iteration.
type orderedMap[K comparable, V any] struct {
	keys  []K
	vals  map[K]V
	alive map[K]bool
}

func newMap[K comparable, V any]() *orderedMap[K, V] {
	return &orderedMap[K, V]{vals: map[K]V{}, alive: map[K]bool{}}
}

func (m *orderedMap[K, V]) get(k K) (V, bool) {
	v, ok := m.vals[k]
	return v, ok
}

func (m *orderedMap[K, V]) set(k K, v V) {
	if _, ok := m.vals[k]; !ok {
		m.keys = append(m.keys, k)
	}
	m.vals[k] = v
}

func (m *orderedMap[K, V]) delete(k K) {
	if _, ok := m.vals[k]; !ok {
		return
	}
	delete(m.vals, k)
	for i, x := range m.keys {
		if x == k {
			m.keys = append(m.keys[:i:i], m.keys[i+1:]...)
			break
		}
	}
}

func (m *orderedMap[K, V]) size() int { return len(m.vals) }

func (m *orderedMap[K, V]) clear() {
	m.keys = nil
	m.vals = map[K]V{}
}

func (m *orderedMap[K, V]) forEach(f func(K, V)) {
	for i := 0; i < len(m.keys); i++ {
		k := m.keys[i]
		if v, ok := m.vals[k]; ok {
			f(k, v)
		}
	}
}

// chunkList is a JavaScript array of chunks that is shared by identity
// (resource chunk lists live in Sets and Maps; `resource.length = 0` empties
// them in place).
type chunkList struct{ chunks []string }

func (c *chunkList) push(s ...string) { c.chunks = append(c.chunks, s...) }

// resourceState is a value in a resumable-state resource map: PRELOAD_NO_CREDS
// (creds == nil) or [crossOrigin, integrity] (creds of length 2). A map entry
// holding a nil *resourceState is JavaScript null.
type resourceState struct{ creds []any }

var preloadNoCreds = &resourceState{}

type resourceMap map[string]*resourceState

// lookup reads map[key] as JavaScript does through hasOwnProperty:
// present reports existence, the pointer is nil for null.
func (m resourceMap) lookup(key string) (rs *resourceState, present bool) {
	rs, present = m[key]
	return
}

type resumableState struct {
	idPrefix               string
	nextFormID             int
	bootstrapScriptContent any // undefined or a value
	bootstrapScripts       []any
	hasBootstrapScripts    bool
	bootstrapModules       []any
	hasBootstrapModules    bool
	instructions           int
	hasBody, hasHtml       bool
	unknownResources       map[string]resourceMap
	dnsResources           resourceMap
	connectResources       map[string]resourceMap
	imageResources         resourceMap
	styleResources         resourceMap
	scriptResources        resourceMap
	moduleUnknownResources map[string]resourceMap
	moduleScriptResources  resourceMap
}

func createResumableState(identifierPrefix any, bootstrapScriptContent any, bootstrapScripts, bootstrapModules any) *resumableState {
	rs := &resumableState{
		bootstrapScriptContent: bootstrapScriptContent,
		unknownResources:       map[string]resourceMap{},
		dnsResources:           resourceMap{},
		connectResources:       map[string]resourceMap{"default": {}, "anonymous": {}, "credentials": {}},
		imageResources:         resourceMap{},
		styleResources:         resourceMap{},
		scriptResources:        resourceMap{},
		moduleUnknownResources: map[string]resourceMap{},
		moduleScriptResources:  resourceMap{},
	}
	if !jsvalue.IsUndefined(identifierPrefix) {
		rs.idPrefix = jsvalue.ToString(identifierPrefix)
	}
	if !jsvalue.IsUndefined(bootstrapScripts) {
		rs.hasBootstrapScripts = true
		rs.bootstrapScripts = jsvalue.Items(bootstrapScripts)
	}
	if !jsvalue.IsUndefined(bootstrapModules) {
		rs.hasBootstrapModules = true
		rs.bootstrapModules = jsvalue.Items(bootstrapModules)
	}
	return rs
}

// preambleState holds the <html>, <head> and <body> start-tag chunks; nil
// fields are JavaScript null.
type preambleState struct {
	htmlChunks, headChunks, bodyChunks *chunkList
}

type styleQueue struct {
	precedence string // escaped
	rules      []string
	hrefs      []string
	sheets     *orderedMap[string, *stylesheet]
}

type stylesheet struct {
	state int // 0 not flushed, 1 preloaded, 2 flushed, 3 flushed in a boundary's dependencies
	props *react.Object
}

type headersState struct {
	preconnects, fontPreloads, highImagePreloads string
	remainingCapacity                            int
}

type renderState struct {
	placeholderPrefix, segmentPrefix, boundaryPrefix string
	startInlineScript, startInlineStyle              string
	preamble                                         *preambleState
	bootstrapChunks                                  []string
	importMapChunks                                  []string
	onHeaders                                        func(headers map[string]string)
	headers                                          *headersState
	charsetChunks, viewportChunks, hoistableChunks   []string
	preconnects, fontPreloads, highImagePreloads     *orderedSet[*chunkList]
	styles                                           *orderedMap[string, *styleQueue]
	bootstrapScripts, scripts, bulkPreloads          *orderedSet[*chunkList]
	preloadImages, preloadStylesheets                map[string]*chunkList
	preloadScripts, preloadModuleScripts             map[string]*chunkList
	nonceScript, nonceStyle                          any // string or undefined
	stylesToHoist                                    bool
	generateStaticMarkup                             bool
}

type hoistableState struct {
	styles          *orderedSet[*styleQueue]
	stylesheets     *orderedSet[*stylesheet]
	suspenseyImages bool
}

func createHoistableState() *hoistableState {
	return &hoistableState{styles: newSet[*styleQueue](), stylesheets: newSet[*stylesheet]()}
}

func hoistHoistables(parent, child *hoistableState) {
	child.styles.forEach(func(q *styleQueue) { parent.styles.add(q) })
	child.stylesheets.forEach(func(s *stylesheet) { parent.stylesheets.add(s) })
	if child.suspenseyImages {
		parent.suspenseyImages = true
	}
}

func hasSuspenseyContent(h *hoistableState) bool {
	return h.stylesheets.size() > 0 || h.suspenseyImages
}

func newRenderStateBase(idPrefix string) *renderState {
	return &renderState{
		placeholderPrefix:    idPrefix + "P:",
		segmentPrefix:        idPrefix + "S:",
		boundaryPrefix:       idPrefix + "B:",
		startInlineScript:    "<script",
		startInlineStyle:     "<style",
		preamble:             &preambleState{},
		preconnects:          newSet[*chunkList](),
		fontPreloads:         newSet[*chunkList](),
		highImagePreloads:    newSet[*chunkList](),
		styles:               newMap[string, *styleQueue](),
		bootstrapScripts:     newSet[*chunkList](),
		scripts:              newSet[*chunkList](),
		bulkPreloads:         newSet[*chunkList](),
		preloadImages:        map[string]*chunkList{},
		preloadStylesheets:   map[string]*chunkList{},
		preloadScripts:       map[string]*chunkList{},
		preloadModuleScripts: map[string]*chunkList{},
		nonceScript:          jsvalue.Undefined{},
		nonceStyle:           jsvalue.Undefined{},
	}
}

// createRenderState builds the render state. nonce is the nonce option
// (string, {script, style} object, or undefined); importMap, onHeaders and
// maxHeadersLength are the streaming options (legacy passes undefined).
func createRenderState(rs *resumableState, nonce any, importMap any, onHeaders func(map[string]string), maxHeadersLength any, generateStaticMarkup bool) *renderState {
	var nonceScript, nonceStyle any = jsvalue.Undefined{}, jsvalue.Undefined{}
	if s, ok := nonce.(string); ok {
		nonceScript = s
	} else if jsvalue.Truthy(nonce) {
		nonceScript = getProp(nonce, "script")
		nonceStyle = getProp(nonce, "style")
	} else {
		nonceScript, nonceStyle = nonce, nonce
	}
	st := newRenderStateBase(rs.idPrefix)
	st.generateStaticMarkup = generateStaticMarkup
	st.nonceScript, st.nonceStyle = nonceScript, nonceStyle
	if !jsvalue.IsUndefined(nonceScript) {
		st.startInlineScript = "<script nonce=\"" + escapeTextForBrowser(nonceScript) + "\""
	}
	if !jsvalue.IsUndefined(nonceStyle) {
		st.startInlineStyle = "<style nonce=\"" + escapeTextForBrowser(nonceStyle) + "\""
	}
	if !jsvalue.IsUndefined(rs.bootstrapScriptContent) {
		st.bootstrapChunks = append(st.bootstrapChunks, st.startInlineScript)
		pushCompletedShellIdAttribute(&st.bootstrapChunks, rs)
		st.bootstrapChunks = append(st.bootstrapChunks, ">", escapeScript(jsvalue.ToString(rs.bootstrapScriptContent)), "</script>")
	}
	if !jsvalue.IsUndefined(importMap) {
		j, _ := jsvalue.JSONStringify(importMap)
		st.importMapChunks = append(st.importMapChunks, "<script type=\"importmap\">", escapeScript(j), "</script>")
	}
	if onHeaders != nil {
		capacity := 2000
		if jsvalue.IsNumber(maxHeadersLength) {
			capacity = int(jsvalue.Float(maxHeadersLength))
		}
		st.onHeaders = onHeaders
		st.headers = &headersState{remainingCapacity: 2 + capacity}
	}
	if rs.hasBootstrapScripts {
		for _, cfg := range rs.bootstrapScripts {
			var src, integrity, crossOrigin any = jsvalue.Undefined{}, jsvalue.Undefined{}, jsvalue.Undefined{}
			props := react.Obj("rel", "preload", "as", "script", "fetchPriority", "low", "nonce", nonce)
			if s, ok := cfg.(string); ok {
				src = s
				props.Set("href", s)
			} else {
				src = getProp(cfg, "src")
				props.Set("href", src)
				if s, ok := getProp(cfg, "integrity").(string); ok {
					integrity = s
				}
				props.Set("integrity", integrity)
				if co := getProp(cfg, "crossOrigin"); !jsvalue.IsNullish(co) {
					if co == "use-credentials" {
						crossOrigin = "use-credentials"
					} else {
						crossOrigin = ""
					}
				}
				props.Set("crossOrigin", crossOrigin)
			}
			key := jsvalue.ToString(src)
			rs.scriptResources[key] = nil
			rs.moduleScriptResources[key] = nil
			res := &chunkList{}
			pushLinkImpl(&res.chunks, props)
			st.bootstrapScripts.add(res)
			st.bootstrapChunks = append(st.bootstrapChunks, "<script src=\"", escapeTextForBrowser(src), "\"")
			if jsvalue.Truthy(nonceScript) {
				st.bootstrapChunks = append(st.bootstrapChunks, " nonce=\"", escapeTextForBrowser(nonceScript), "\"")
			}
			if s, ok := integrity.(string); ok {
				st.bootstrapChunks = append(st.bootstrapChunks, " integrity=\"", escapeTextForBrowser(s), "\"")
			}
			if s, ok := crossOrigin.(string); ok {
				st.bootstrapChunks = append(st.bootstrapChunks, " crossorigin=\"", escapeTextForBrowser(s), "\"")
			}
			pushCompletedShellIdAttribute(&st.bootstrapChunks, rs)
			st.bootstrapChunks = append(st.bootstrapChunks, " async=\"\"></script>")
		}
	}
	if rs.hasBootstrapModules {
		for _, cfg := range rs.bootstrapModules {
			var src, integrity, crossOrigin any = jsvalue.Undefined{}, jsvalue.Undefined{}, jsvalue.Undefined{}
			props := react.Obj("rel", "modulepreload", "fetchPriority", "low", "nonce", nonceScript)
			if s, ok := cfg.(string); ok {
				src = s
				props.Set("href", s)
			} else {
				src = getProp(cfg, "src")
				props.Set("href", src)
				if s, ok := getProp(cfg, "integrity").(string); ok {
					integrity = s
				}
				props.Set("integrity", integrity)
				if co := getProp(cfg, "crossOrigin"); !jsvalue.IsNullish(co) {
					if co == "use-credentials" {
						crossOrigin = "use-credentials"
					} else {
						crossOrigin = ""
					}
				}
				props.Set("crossOrigin", crossOrigin)
			}
			key := jsvalue.ToString(src)
			rs.scriptResources[key] = nil
			rs.moduleScriptResources[key] = nil
			res := &chunkList{}
			pushLinkImpl(&res.chunks, props)
			st.bootstrapScripts.add(res)
			st.bootstrapChunks = append(st.bootstrapChunks, "<script type=\"module\" src=\"", escapeTextForBrowser(src), "\"")
			if jsvalue.Truthy(nonceScript) {
				st.bootstrapChunks = append(st.bootstrapChunks, " nonce=\"", escapeTextForBrowser(nonceScript), "\"")
			}
			if s, ok := integrity.(string); ok {
				st.bootstrapChunks = append(st.bootstrapChunks, " integrity=\"", escapeTextForBrowser(s), "\"")
			}
			if s, ok := crossOrigin.(string); ok {
				st.bootstrapChunks = append(st.bootstrapChunks, " crossorigin=\"", escapeTextForBrowser(s), "\"")
			}
			pushCompletedShellIdAttribute(&st.bootstrapChunks, rs)
			st.bootstrapChunks = append(st.bootstrapChunks, " async=\"\"></script>")
		}
	}
	return st
}

// createLegacyRenderState is the renderToString / renderToStaticMarkup
// variant of createRenderState.
func createLegacyRenderState(rs *resumableState, generateStaticMarkup bool) *renderState {
	st := newRenderStateBase(rs.idPrefix)
	st.generateStaticMarkup = generateStaticMarkup
	return st
}

func pushCompletedShellIdAttribute(target *[]string, rs *resumableState) {
	if rs.instructions&32 == 0 {
		rs.instructions |= 32
		*target = append(*target, " id=\"", escapeTextForBrowser("_"+rs.idPrefix+"R_"), "\"")
	}
}

// getProp reads o[k] for the modelled objects (undefined when absent).
func getProp(o any, k string) any {
	if obj, ok := o.(*react.Object); ok && obj != nil {
		return obj.Get(k)
	}
	return jsvalue.Undefined{}
}

// formatContext is the HTML insertion context.
type formatContext struct {
	insertionMode int
	selectedValue any
	tagScope      int
}

const (
	rootHTMLMode     = 0
	htmlHTMLMode     = 1
	htmlMode         = 2
	htmlHeadMode     = 3
	svgMode          = 4
	mathMLMode       = 5
	htmlTableMode    = 6
	htmlTableBody    = 7
	htmlTableRowMode = 8
	htmlColgroupMode = 9
)

func createFormatContext(insertionMode int, selectedValue any, tagScope int) *formatContext {
	return &formatContext{insertionMode, selectedValue, tagScope}
}

func createRootFormatContext(namespaceURI any) *formatContext {
	mode := rootHTMLMode
	switch namespaceURI {
	case "http://www.w3.org/2000/svg":
		mode = svgMode
	case "http://www.w3.org/1998/Math/MathML":
		mode = mathMLMode
	}
	return createFormatContext(mode, nil, 0)
}

func getChildFormatContext(parent *formatContext, typ string, props *react.Object) *formatContext {
	subtreeScope := parent.tagScope & -25
	switch typ {
	case "noscript":
		return createFormatContext(htmlMode, nil, subtreeScope|1)
	case "select":
		v := props.Get("value")
		if jsvalue.IsNullish(v) {
			v = props.Get("defaultValue")
		}
		return createFormatContext(htmlMode, v, subtreeScope)
	case "svg":
		return createFormatContext(svgMode, nil, subtreeScope)
	case "picture":
		return createFormatContext(htmlMode, nil, subtreeScope|2)
	case "math":
		return createFormatContext(mathMLMode, nil, subtreeScope)
	case "foreignObject":
		return createFormatContext(htmlMode, nil, subtreeScope)
	case "table":
		return createFormatContext(htmlTableMode, nil, subtreeScope)
	case "thead", "tbody", "tfoot":
		return createFormatContext(htmlTableBody, nil, subtreeScope)
	case "colgroup":
		return createFormatContext(htmlColgroupMode, nil, subtreeScope)
	case "tr":
		return createFormatContext(htmlTableRowMode, nil, subtreeScope)
	case "head":
		if parent.insertionMode < htmlMode {
			return createFormatContext(htmlHeadMode, nil, subtreeScope)
		}
	case "html":
		if parent.insertionMode == rootHTMLMode {
			return createFormatContext(htmlHTMLMode, nil, subtreeScope)
		}
	}
	if parent.insertionMode >= htmlTableMode || parent.insertionMode < htmlMode {
		return createFormatContext(htmlMode, nil, subtreeScope)
	}
	if parent.tagScope != subtreeScope {
		return createFormatContext(parent.insertionMode, parent.selectedValue, subtreeScope)
	}
	return parent
}

func getSuspenseFallbackFormatContext(rs *resumableState, parent *formatContext) *formatContext {
	if parent.tagScope&32 != 0 {
		rs.instructions |= 128
	}
	return createFormatContext(parent.insertionMode, parent.selectedValue, parent.tagScope|12)
}

func getSuspenseContentFormatContext(rs *resumableState, parent *formatContext) *formatContext {
	return createFormatContext(parent.insertionMode, parent.selectedValue, parent.tagScope|16)
}
