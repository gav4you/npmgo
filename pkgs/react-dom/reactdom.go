// Package reactdom is a Go translation of the server-relevant part of the
// npm package react-dom@19.2.4's main entry (https://github.com/facebook/react,
// packages/react-dom; behaviour taken from cjs/react-dom.production.js): the
// resource hints (preload, preinit, preconnect, prefetchDNS, preloadModule,
// preinitModule), the form hooks (useFormStatus, useFormState), flushSync,
// createPortal, requestFormReset and unstable_batchedUpdates. The original
// copyright notice and MIT license are preserved in LICENSE.
//
// In JavaScript the resource hints find the active render through a global
// dispatcher. Go has no implicit current render, so each hint takes the
// react.Hooks the component received; with a nil Hooks (or outside a
// render) a hint does nothing, like the server's default dispatcher.
//
// The server renderer is the subpackage server. The client renderer
// (react-dom/client) is not ported.
package reactdom

import (
	"net/url"

	"github.com/gav4you/npmgo/internal/jsvalue"
	"github.com/gav4you/npmgo/pkgs/react"
)

// Version is the react-dom version this package translates.
const Version = "19.2.4"

// ResourceDispatcher receives resource hints (ReactDOMSharedInternals.d).
// The Hooks of a react-dom/server render implement it.
type ResourceDispatcher interface {
	PrefetchDNS(href string)
	Preconnect(href string, crossOrigin any)
	Preload(href, as string, options *react.Object)
	PreloadModule(href string, options *react.Object)
	PreinitScript(src string, options *react.Object)
	PreinitStyle(href string, precedence any, options *react.Object)
	PreinitModuleScript(src string, options *react.Object)
}

func dispatcherOf(h react.Hooks) ResourceDispatcher {
	if d, ok := h.(ResourceDispatcher); ok && !jsvalue.IsNull(h) {
		return d
	}
	return nil
}

func formatProdErrorMessage(code string, args ...string) string {
	u := "https://react.dev/errors/" + code
	for i, a := range args {
		if i == 0 {
			u += "?args[]=" + url.QueryEscape(a)
		} else {
			u += "&args[]=" + url.QueryEscape(a)
		}
	}
	return "Minified React error #" + code + "; visit " + u + " for the full message or use the non-minified dev environment for full errors and additional helpful warnings."
}

func getCrossOriginStringAs(as, input any) any {
	if as == "font" {
		return ""
	}
	if s, ok := input.(string); ok {
		if s == "use-credentials" {
			return s
		}
		return ""
	}
	return jsvalue.Undefined{}
}

func stringOrUndefined(v any) any {
	if s, ok := v.(string); ok {
		return s
	}
	return jsvalue.Undefined{}
}

func get(o *react.Object, k string) any {
	if o == nil {
		return jsvalue.Undefined{}
	}
	return o.Get(k)
}

// Preconnect is ReactDOM.preconnect(href, options).
func Preconnect(h react.Hooks, href any, options *react.Object) {
	s, ok := href.(string)
	if !ok {
		return
	}
	var crossOrigin any
	if options != nil {
		crossOrigin = getCrossOriginStringAs(nil, options.Get("crossOrigin"))
	}
	if d := dispatcherOf(h); d != nil {
		d.Preconnect(s, crossOrigin)
	}
}

// PrefetchDNS is ReactDOM.prefetchDNS(href).
func PrefetchDNS(h react.Hooks, href any) {
	if s, ok := href.(string); ok {
		if d := dispatcherOf(h); d != nil {
			d.PrefetchDNS(s)
		}
	}
}

// Preinit is ReactDOM.preinit(href, options).
func Preinit(h react.Hooks, href any, options *react.Object) {
	s, ok := href.(string)
	if !ok || options == nil {
		return
	}
	as, ok := options.Get("as").(string)
	if !ok {
		return
	}
	crossOrigin := getCrossOriginStringAs(as, options.Get("crossOrigin"))
	integrity := stringOrUndefined(options.Get("integrity"))
	fetchPriority := stringOrUndefined(options.Get("fetchPriority"))
	d := dispatcherOf(h)
	if d == nil {
		return
	}
	switch as {
	case "style":
		d.PreinitStyle(s, stringOrUndefined(options.Get("precedence")), react.Obj(
			"crossOrigin", crossOrigin, "integrity", integrity, "fetchPriority", fetchPriority))
	case "script":
		d.PreinitScript(s, react.Obj(
			"crossOrigin", crossOrigin, "integrity", integrity, "fetchPriority", fetchPriority,
			"nonce", stringOrUndefined(options.Get("nonce"))))
	}
}

// PreinitModule is ReactDOM.preinitModule(href, options); options is an
// *react.Object, nil (null) or Undefined{}.
func PreinitModule(h react.Hooks, href any, options any) {
	s, ok := href.(string)
	if !ok {
		return
	}
	d := dispatcherOf(h)
	if o, ok := options.(*react.Object); ok && o != nil {
		as := o.Get("as")
		if jsvalue.IsNullish(as) || as == "script" {
			crossOrigin := getCrossOriginStringAs(as, o.Get("crossOrigin"))
			if d != nil {
				d.PreinitModuleScript(s, react.Obj(
					"crossOrigin", crossOrigin,
					"integrity", stringOrUndefined(o.Get("integrity")),
					"nonce", stringOrUndefined(o.Get("nonce"))))
			}
		}
		return
	}
	if jsvalue.IsNullish(options) && d != nil {
		d.PreinitModuleScript(s, nil)
	}
}

// Preload is ReactDOM.preload(href, options).
func Preload(h react.Hooks, href any, options *react.Object) {
	s, ok := href.(string)
	if !ok || options == nil {
		return
	}
	as, ok := options.Get("as").(string)
	if !ok {
		return
	}
	crossOrigin := getCrossOriginStringAs(as, options.Get("crossOrigin"))
	if d := dispatcherOf(h); d != nil {
		d.Preload(s, as, react.Obj(
			"crossOrigin", crossOrigin,
			"integrity", stringOrUndefined(options.Get("integrity")),
			"nonce", stringOrUndefined(options.Get("nonce")),
			"type", stringOrUndefined(options.Get("type")),
			"fetchPriority", stringOrUndefined(options.Get("fetchPriority")),
			"referrerPolicy", stringOrUndefined(options.Get("referrerPolicy")),
			"imageSrcSet", stringOrUndefined(options.Get("imageSrcSet")),
			"imageSizes", stringOrUndefined(options.Get("imageSizes")),
			"media", stringOrUndefined(options.Get("media")),
		))
	}
}

// PreloadModule is ReactDOM.preloadModule(href, options).
func PreloadModule(h react.Hooks, href any, options *react.Object) {
	s, ok := href.(string)
	if !ok {
		return
	}
	d := dispatcherOf(h)
	if d == nil {
		return
	}
	if options != nil {
		as := options.Get("as")
		crossOrigin := getCrossOriginStringAs(as, options.Get("crossOrigin"))
		var asOpt any = jsvalue.Undefined{}
		if a, ok := as.(string); ok && a != "script" {
			asOpt = a
		}
		d.PreloadModule(s, react.Obj("as", asOpt, "crossOrigin", crossOrigin, "integrity", stringOrUndefined(options.Get("integrity"))))
		return
	}
	d.PreloadModule(s, nil)
}

// UseFormStatus is ReactDOM.useFormStatus().
func UseFormStatus(h react.Hooks) *react.FormStatus { return h.UseHostTransitionStatus() }

// UseFormState is ReactDOM.useFormState(action, initialState, permalink).
func UseFormState(h react.Hooks, action, initialState any, permalink ...string) (any, *react.Action, bool) {
	return h.UseFormState(action, initialState, permalink...)
}

// FlushSync is ReactDOM.flushSync(fn): on the server it calls fn and
// returns its result.
func FlushSync(fn func() any) any {
	if fn == nil {
		return react.Undefined{}
	}
	return fn()
}

// CreatePortal is ReactDOM.createPortal(children, container, key). container
// must look like a DOM element, document or fragment: an *react.Object whose
// nodeType is 1, 9 or 11.
func CreatePortal(children, container any, key ...any) *react.Portal {
	nt := 0.0
	if o, ok := container.(*react.Object); ok && o != nil {
		nt = jsvalue.ToNumber(o.Get("nodeType"))
	}
	if !jsvalue.Truthy(container) || (nt != 1 && nt != 9 && nt != 11) {
		panic(&react.Error{Message: formatProdErrorMessage("299")})
	}
	p := &react.Portal{Children: children, ContainerInfo: container}
	if len(key) > 0 && !jsvalue.IsNullish(key[0]) {
		k := jsvalue.ToString(key[0])
		p.Key = &k
	}
	return p
}

// RequestFormReset is ReactDOM.requestFormReset(form); the server's
// dispatcher throws (minified error #522).
func RequestFormReset(form any) {
	panic(&react.Error{Message: formatProdErrorMessage("522")})
}

// UnstableBatchedUpdates is ReactDOM.unstable_batchedUpdates(fn, a).
func UnstableBatchedUpdates(fn func(any) any, a any) any { return fn(a) }
