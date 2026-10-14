package server

import (
	"strings"

	"github.com/gav4you/npmgo/internal/jsvalue"
	"github.com/gav4you/npmgo/pkgs/react"
)

func getPreloadAsHeader(href any, as string, params *react.Object) string {
	h := escapeHrefForLinkHeader(jsvalue.ToString(href))
	a := escapeStringForLinkHeaderQuotedParam(as)
	out := "<" + h + ">; rel=preload; as=\"" + a + "\""
	params.Range(func(name string, v any) bool {
		if s, ok := v.(string); ok {
			out += "; " + strings.ToLower(name) + "=\"" + escapeStringForLinkHeaderQuotedParam(s) + "\""
		}
		return true
	})
	return out
}

// The functions below are react-dom/server's implementation of
// ReactDOMSharedInternals.d (prefetchDNS, preconnect, preload, preloadModule,
// preinitStyle, preinitScript, preinitModuleScript). r is the request the
// call belongs to; nil means "no request" and the call does nothing (the
// previous dispatcher on the server is a no-op).

func (r *request) prefetchDNS(href any) {
	if r == nil {
		return
	}
	rs, st := r.resumableState, r.renderState
	hs, ok := href.(string)
	if !ok || hs == "" {
		return
	}
	if _, present := rs.dnsResources[hs]; !present {
		rs.dnsResources[hs] = nil
		headers := st.headers
		var header string
		useHeader := false
		if headers != nil && headers.remainingCapacity > 0 {
			header = "<" + escapeHrefForLinkHeader(hs) + ">; rel=dns-prefetch"
			headers.remainingCapacity -= utf16Len(header) + 2
			useHeader = headers.remainingCapacity >= 0
		}
		if useHeader {
			if headers.preconnects != "" {
				headers.preconnects += ", "
			}
			headers.preconnects += header
		} else {
			res := &chunkList{}
			pushLinkImpl(&res.chunks, react.Obj("href", hs, "rel", "dns-prefetch"))
			st.preconnects.add(res)
		}
	}
	r.enqueueFlush()
}

func (r *request) preconnect(href, crossOrigin any) {
	if r == nil {
		return
	}
	rs, st := r.resumableState, r.renderState
	hs, ok := href.(string)
	if !ok || hs == "" {
		return
	}
	bucket := "default"
	if crossOrigin == "use-credentials" {
		bucket = "credentials"
	} else if _, ok := crossOrigin.(string); ok {
		bucket = "anonymous"
	}
	if _, present := rs.connectResources[bucket][hs]; !present {
		rs.connectResources[bucket][hs] = nil
		headers := st.headers
		var header string
		useHeader := false
		if headers != nil && headers.remainingCapacity > 0 {
			header = "<" + escapeHrefForLinkHeader(hs) + ">; rel=preconnect"
			if s, ok := crossOrigin.(string); ok {
				header += "; crossorigin=\"" + escapeStringForLinkHeaderQuotedParam(s) + "\""
			}
			headers.remainingCapacity -= utf16Len(header) + 2
			useHeader = headers.remainingCapacity >= 0
		}
		if useHeader {
			if headers.preconnects != "" {
				headers.preconnects += ", "
			}
			headers.preconnects += header
		} else {
			res := &chunkList{}
			pushLinkImpl(&res.chunks, react.Obj("rel", "preconnect", "href", hs, "crossOrigin", crossOrigin))
			st.preconnects.add(res)
		}
	}
	r.enqueueFlush()
}

func credsOf(options *react.Object) *resourceState {
	if options == nil {
		return preloadNoCreds
	}
	_, a := options.Get("crossOrigin").(string)
	_, b := options.Get("integrity").(string)
	if !a && !b {
		return preloadNoCreds
	}
	return &resourceState{creds: []any{options.Get("crossOrigin"), options.Get("integrity")}}
}

func assignInto(base *react.Object, options *react.Object) *react.Object {
	if options != nil {
		base.Assign(options)
	}
	return base
}

func (r *request) preload(href any, as any, options *react.Object) {
	if r == nil {
		return
	}
	rs, st := r.resumableState, r.renderState
	if !jsvalue.Truthy(as) || !jsvalue.Truthy(href) {
		return
	}
	hs := jsvalue.ToString(href)
	asS := jsvalue.ToString(as)
	switch asS {
	case "image":
		var imageSrcSet, imageSizes, fetchPriority any = jsvalue.Undefined{}, jsvalue.Undefined{}, jsvalue.Undefined{}
		if options != nil {
			imageSrcSet, imageSizes, fetchPriority = options.Get("imageSrcSet"), options.Get("imageSizes"), options.Get("fetchPriority")
		}
		key := hs
		if jsvalue.Truthy(imageSrcSet) {
			key = jsvalue.ToString(imageSrcSet) + "\n"
			if jsvalue.Truthy(imageSizes) {
				key += jsvalue.ToString(imageSizes)
			}
		}
		if _, present := rs.imageResources[key]; present {
			return
		}
		rs.imageResources[key] = preloadNoCreds
		headers := st.headers
		var header string
		useHeader := false
		_, srcSetString := imageSrcSet.(string)
		if headers != nil && headers.remainingCapacity > 0 && !srcSetString && fetchPriority == "high" {
			header = getPreloadAsHeader(hs, asS, optionsOrEmpty(options))
			headers.remainingCapacity -= utf16Len(header) + 2
			useHeader = headers.remainingCapacity >= 0
		}
		if useHeader {
			if headers.highImagePreloads != "" {
				headers.highImagePreloads += ", "
			}
			headers.highImagePreloads += header
		} else {
			res := &chunkList{}
			var h any = hs
			if jsvalue.Truthy(imageSrcSet) {
				h = jsvalue.Undefined{}
			}
			pushLinkImpl(&res.chunks, assignInto(react.Obj("rel", "preload", "href", h, "as", asS), options))
			if fetchPriority == "high" {
				st.highImagePreloads.add(res)
			} else {
				st.bulkPreloads.add(res)
				st.preloadImages[key] = res
			}
		}
	case "style":
		if _, present := rs.styleResources[hs]; present {
			return
		}
		res := &chunkList{}
		pushLinkImpl(&res.chunks, assignInto(react.Obj("rel", "preload", "href", hs, "as", asS), options))
		rs.styleResources[hs] = credsOf(options)
		st.preloadStylesheets[hs] = res
		st.bulkPreloads.add(res)
	case "script":
		if _, present := rs.scriptResources[hs]; present {
			return
		}
		res := &chunkList{}
		st.preloadScripts[hs] = res
		st.bulkPreloads.add(res)
		pushLinkImpl(&res.chunks, assignInto(react.Obj("rel", "preload", "href", hs, "as", asS), options))
		rs.scriptResources[hs] = credsOf(options)
	default:
		resources, ok := rs.unknownResources[asS]
		if ok {
			if _, present := resources[hs]; present {
				return
			}
		} else {
			resources = resourceMap{}
			rs.unknownResources[asS] = resources
		}
		resources[hs] = preloadNoCreds
		headers := st.headers
		if headers != nil && headers.remainingCapacity > 0 && asS == "font" {
			header := getPreloadAsHeader(hs, asS, optionsOrEmpty(options))
			headers.remainingCapacity -= utf16Len(header) + 2
			if headers.remainingCapacity >= 0 {
				if headers.fontPreloads != "" {
					headers.fontPreloads += ", "
				}
				headers.fontPreloads += header
				break
			}
		}
		res := &chunkList{}
		pushLinkImpl(&res.chunks, assignInto(react.Obj("rel", "preload", "href", hs, "as", asS), options))
		if asS == "font" {
			st.fontPreloads.add(res)
		} else {
			st.bulkPreloads.add(res)
		}
	}
	r.enqueueFlush()
}

func optionsOrEmpty(o *react.Object) *react.Object {
	if o == nil {
		return &react.Object{}
	}
	return o
}

func (r *request) preloadModule(href any, options *react.Object) {
	if r == nil {
		return
	}
	rs, st := r.resumableState, r.renderState
	if !jsvalue.Truthy(href) {
		return
	}
	hs := jsvalue.ToString(href)
	as := "script"
	if options != nil {
		if s, ok := options.Get("as").(string); ok {
			as = s
		}
	}
	res := &chunkList{}
	switch as {
	case "script":
		if _, present := rs.moduleScriptResources[hs]; present {
			return
		}
		rs.moduleScriptResources[hs] = credsOf(options)
		st.preloadModuleScripts[hs] = res
	default:
		resources, ok := rs.moduleUnknownResources[as]
		if ok {
			// React reads resumableState.unknownResources here (a bug in the
			// original that throws when that bucket is missing); mirror it.
			ur, ok2 := rs.unknownResources[as]
			if !ok2 {
				panic(&jsvalue.TypeError{Message: "Cannot read properties of undefined (reading 'hasOwnProperty')"})
			}
			if _, present := ur[hs]; present {
				return
			}
			resources = ur
		} else {
			resources = resourceMap{}
			rs.moduleUnknownResources[as] = resources
		}
		resources[hs] = preloadNoCreds
	}
	pushLinkImpl(&res.chunks, assignInto(react.Obj("rel", "modulepreload", "href", hs), options))
	st.bulkPreloads.add(res)
	r.enqueueFlush()
}

func (r *request) preinitStyle(href any, precedence any, options *react.Object) {
	if r == nil {
		return
	}
	rs, st := r.resumableState, r.renderState
	if !jsvalue.Truthy(href) {
		return
	}
	hs := jsvalue.ToString(href)
	if !jsvalue.Truthy(precedence) {
		precedence = "default"
	}
	prec := jsvalue.ToString(precedence)
	queue, hasQueue := st.styles.get(prec)
	resState, present := rs.styleResources.lookup(hs)
	if present && resState == nil {
		return
	}
	rs.styleResources[hs] = nil
	if !hasQueue {
		queue = &styleQueue{precedence: escapeTextForBrowser(prec), sheets: newMap[string, *stylesheet]()}
		st.styles.set(prec, queue)
	}
	sheet := &stylesheet{state: 0, props: assignInto(react.Obj("rel", "stylesheet", "href", hs, "data-precedence", prec), options)}
	if present && resState != nil {
		if len(resState.creds) == 2 {
			adoptPreloadCredentials(sheet.props, resState.creds)
		}
		if pr, ok := st.preloadStylesheets[hs]; ok && pr != nil && len(pr.chunks) > 0 {
			pr.chunks = pr.chunks[:0]
		} else {
			sheet.state = 1
		}
	}
	queue.sheets.set(hs, sheet)
	r.enqueueFlush()
}

func (r *request) preinitScript(src any, options *react.Object) {
	if r == nil {
		return
	}
	rs, st := r.resumableState, r.renderState
	if !jsvalue.Truthy(src) {
		return
	}
	ss := jsvalue.ToString(src)
	resState, present := rs.scriptResources.lookup(ss)
	if present && resState == nil {
		return
	}
	rs.scriptResources[ss] = nil
	props := assignInto(react.Obj("src", ss, "async", true), options)
	if present && resState != nil {
		if len(resState.creds) == 2 {
			adoptPreloadCredentials(props, resState.creds)
		}
		if pr, ok := st.preloadScripts[ss]; ok && pr != nil {
			pr.chunks = pr.chunks[:0]
		}
	}
	res := &chunkList{}
	st.scripts.add(res)
	pushScriptImpl(&res.chunks, props)
	r.enqueueFlush()
}

func (r *request) preinitModuleScript(src any, options *react.Object) {
	if r == nil {
		return
	}
	rs, st := r.resumableState, r.renderState
	if !jsvalue.Truthy(src) {
		return
	}
	ss := jsvalue.ToString(src)
	resState, present := rs.moduleScriptResources.lookup(ss)
	if present && resState == nil {
		return
	}
	rs.moduleScriptResources[ss] = nil
	props := assignInto(react.Obj("src", ss, "type", "module", "async", true), options)
	if present && resState != nil {
		if len(resState.creds) == 2 {
			adoptPreloadCredentials(props, resState.creds)
		}
		if pr, ok := st.preloadModuleScripts[ss]; ok && pr != nil {
			pr.chunks = pr.chunks[:0]
		}
	}
	res := &chunkList{}
	st.scripts.add(res)
	pushScriptImpl(&res.chunks, props)
	r.enqueueFlush()
}
