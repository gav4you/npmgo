package server

import (
	"strings"

	"github.com/gav4you/npmgo/internal/jsvalue"
	"github.com/gav4you/npmgo/pkgs/react"
)

const textSeparator = "<!-- -->"

func pushLinkImpl(target *[]string, props *react.Object) any {
	push(target, startChunkForTag("link"))
	props.Range(func(k string, v any) bool {
		if jsvalue.IsNullish(v) {
			return true
		}
		switch k {
		case "children", "dangerouslySetInnerHTML":
			panic(newError("link is a self-closing tag and must neither have `children` nor use `dangerouslySetInnerHTML`."))
		default:
			pushAttribute(target, k, v)
		}
		return true
	})
	push(target, "/>")
	return nil
}

func pushSelfClosing(target *[]string, props *react.Object, tag string) any {
	push(target, startChunkForTag(tag))
	props.Range(func(k string, v any) bool {
		if jsvalue.IsNullish(v) {
			return true
		}
		switch k {
		case "children", "dangerouslySetInnerHTML":
			panic(newError(tag + " is a self-closing tag and must neither have `children` nor use `dangerouslySetInnerHTML`."))
		default:
			pushAttribute(target, k, v)
		}
		return true
	})
	push(target, "/>")
	return nil
}

// splitChildrenAndHTML pushes the attributes of every prop except children
// and dangerouslySetInnerHTML (and except the listed keys), returning those two.
func pushProps(target *[]string, props *react.Object, skip func(k string, v any) bool) (children, innerHTML any) {
	children, innerHTML = nil, nil
	props.Range(func(k string, v any) bool {
		if jsvalue.IsNullish(v) {
			return true
		}
		switch k {
		case "children":
			children = v
		case "dangerouslySetInnerHTML":
			innerHTML = v
		default:
			if skip != nil && skip(k, v) {
				return true
			}
			pushAttribute(target, k, v)
		}
		return true
	})
	return
}

// singleChild is `Array.isArray(c) ? (2 > c.length ? c[0] : null) : c`.
func singleChild(children any) any {
	if jsvalue.IsArray(children) {
		items := jsvalue.Items(children)
		if len(items) < 2 {
			if len(items) == 0 {
				return jsvalue.Undefined{}
			}
			return items[0]
		}
		return nil
	}
	return children
}

func pushTitleImpl(target *[]string, props *react.Object) any {
	push(target, startChunkForTag("title"))
	children, innerHTML := pushProps(target, props, nil)
	push(target, ">")
	child := singleChild(children)
	if !isFuncOrSymbol(child) && !jsvalue.IsNullish(child) {
		push(target, escapeTextForBrowser(jsvalue.ToString(child)))
	}
	pushInnerHTML(target, innerHTML, children)
	push(target, endChunkForTag("title"))
	return nil
}

func pushScriptImpl(target *[]string, props *react.Object) any {
	push(target, startChunkForTag("script"))
	children, innerHTML := pushProps(target, props, nil)
	push(target, ">")
	pushInnerHTML(target, innerHTML, children)
	if s, ok := children.(string); ok {
		push(target, escapeScript(s))
	}
	push(target, endChunkForTag("script"))
	return nil
}

func pushStartSingletonElement(target *[]string, props *react.Object, tag string) any {
	push(target, startChunkForTag(tag))
	children, innerHTML := pushProps(target, props, nil)
	push(target, ">")
	pushInnerHTML(target, innerHTML, children)
	return children
}

func pushStartGenericElement(target *[]string, props *react.Object, tag string) any {
	push(target, startChunkForTag(tag))
	children, innerHTML := pushProps(target, props, nil)
	push(target, ">")
	pushInnerHTML(target, innerHTML, children)
	if s, ok := children.(string); ok {
		push(target, escapeTextForBrowser(s))
		return nil
	}
	return children
}

func flattenOptionChildren(children any) string {
	var b strings.Builder
	react.Children.ForEach(children, func(child any, _ int) {
		if !jsvalue.IsNullish(child) {
			b.WriteString(jsvalue.ToString(child))
		}
	})
	return b.String()
}

func adoptPreloadCredentials(target *react.Object, creds []any) {
	if jsvalue.IsNullish(target.Get("crossOrigin")) {
		target.Set("crossOrigin", creds[0])
	}
	if jsvalue.IsNullish(target.Get("integrity")) {
		target.Set("integrity", creds[1])
	}
}

func isDataURL(v any) bool {
	s, ok := v.(string)
	if !ok || len(s) < 5 || s[4] != ':' {
		return false
	}
	return (s[0] == 'd' || s[0] == 'D') && (s[1] == 'a' || s[1] == 'A') && (s[2] == 't' || s[2] == 'T') && (s[3] == 'a' || s[3] == 'A')
}

// pushStartInstance writes an element's start tag (or routes it to a
// resource / the preamble) and returns the children left to render.
func (r *request) pushStartInstance(target *[]string, typ string, props *react.Object, preambleState *preambleState, hoistable *hoistableState, fc *formatContext, textEmbedded bool) any {
	rs, st := r.resumableState, r.renderState
	switch typ {
	case "div", "span", "svg", "path", "g", "p", "li",
		"annotation-xml", "color-profile", "font-face", "font-face-src", "font-face-uri", "font-face-format", "font-face-name", "missing-glyph":
		return pushStartGenericElement(target, props, typ)
	case "a":
		push(target, startChunkForTag("a"))
		var children, innerHTML any
		props.Range(func(k string, v any) bool {
			if jsvalue.IsNullish(v) {
				return true
			}
			switch k {
			case "children":
				children = v
			case "dangerouslySetInnerHTML":
				innerHTML = v
			case "href":
				if v == "" {
					pushStringAttribute(target, "href", "")
				} else {
					pushAttribute(target, k, v)
				}
			default:
				pushAttribute(target, k, v)
			}
			return true
		})
		push(target, ">")
		pushInnerHTML(target, innerHTML, children)
		if s, ok := children.(string); ok {
			push(target, escapeTextForBrowser(s))
			return nil
		}
		return children
	case "select":
		push(target, startChunkForTag("select"))
		children, innerHTML := pushProps(target, props, func(k string, _ any) bool { return k == "defaultValue" || k == "value" })
		push(target, ">")
		pushInnerHTML(target, innerHTML, children)
		return children
	case "option":
		selectedValue := fc.selectedValue
		push(target, startChunkForTag("option"))
		var children, value, selected, innerHTML any
		value = nil
		hasValue := false
		props.Range(func(k string, v any) bool {
			if jsvalue.IsNullish(v) {
				return true
			}
			switch k {
			case "children":
				children = v
			case "selected":
				selected = v
			case "dangerouslySetInnerHTML":
				innerHTML = v
			case "value":
				value, hasValue = v, true
				pushAttribute(target, k, v)
			default:
				pushAttribute(target, k, v)
			}
			return true
		})
		if !jsvalue.IsNullish(selectedValue) {
			var stringValue string
			if hasValue {
				stringValue = jsvalue.ToString(value)
			} else {
				stringValue = flattenOptionChildren(children)
			}
			if jsvalue.IsArray(selectedValue) {
				for _, sv := range jsvalue.Items(selectedValue) {
					if jsvalue.ToString(sv) == stringValue {
						push(target, " selected=\"\"")
						break
					}
				}
			} else if jsvalue.ToString(selectedValue) == stringValue {
				push(target, " selected=\"\"")
			}
		} else if jsvalue.Truthy(selected) {
			push(target, " selected=\"\"")
		}
		push(target, ">")
		pushInnerHTML(target, innerHTML, children)
		return children
	case "textarea":
		push(target, startChunkForTag("textarea"))
		var value, defaultValue, children any
		props.Range(func(k string, v any) bool {
			if jsvalue.IsNullish(v) {
				return true
			}
			switch k {
			case "children":
				children = v
			case "value":
				value = v
			case "defaultValue":
				defaultValue = v
			case "dangerouslySetInnerHTML":
				panic(newError("`dangerouslySetInnerHTML` does not make sense on <textarea>."))
			default:
				pushAttribute(target, k, v)
			}
			return true
		})
		if value == nil && defaultValue != nil {
			value = defaultValue
		}
		push(target, ">")
		if !jsvalue.IsNullish(children) {
			if !jsvalue.IsNullish(value) {
				panic(newError("If you supply `defaultValue` on a <textarea>, do not pass children."))
			}
			if jsvalue.IsArray(children) {
				items := jsvalue.Items(children)
				if len(items) > 1 {
					panic(newError("<textarea> can only have at most one child."))
				}
				if len(items) > 0 {
					_ = jsvalue.ToString(items[0])
				} else {
					_ = jsvalue.ToString(jsvalue.Undefined{})
				}
			}
			value = jsvalue.ToString(children)
		}
		if s, ok := value.(string); ok && len(s) > 0 && s[0] == '\n' {
			push(target, "\n")
		}
		if value != nil {
			push(target, escapeTextForBrowser(jsvalue.ToString(value)))
		}
		return nil
	case "input":
		push(target, startChunkForTag("input"))
		var name, formAction, formEncType, formMethod, formTarget, value, defaultValue, checked, defaultChecked any
		props.Range(func(k string, v any) bool {
			if jsvalue.IsNullish(v) {
				return true
			}
			switch k {
			case "children", "dangerouslySetInnerHTML":
				panic(newError("input is a self-closing tag and must neither have `children` nor use `dangerouslySetInnerHTML`."))
			case "name":
				name = v
			case "formAction":
				formAction = v
			case "formEncType":
				formEncType = v
			case "formMethod":
				formMethod = v
			case "formTarget":
				formTarget = v
			case "defaultChecked":
				defaultChecked = v
			case "defaultValue":
				defaultValue = v
			case "checked":
				checked = v
			case "value":
				value = v
			default:
				pushAttribute(target, k, v)
			}
			return true
		})
		formData := pushFormActionAttribute(target, rs, st, formAction, formEncType, formMethod, formTarget, name)
		if checked != nil {
			pushBooleanAttribute(target, "checked", checked)
		} else if defaultChecked != nil {
			pushBooleanAttribute(target, "checked", defaultChecked)
		}
		if value != nil {
			pushAttribute(target, "value", value)
		} else if defaultValue != nil {
			pushAttribute(target, "value", defaultValue)
		}
		push(target, "/>")
		for _, f := range formData {
			pushAdditionalFormField(target, f)
		}
		return nil
	case "button":
		push(target, startChunkForTag("button"))
		var children, innerHTML, name, formAction, formEncType, formMethod, formTarget any
		props.Range(func(k string, v any) bool {
			if jsvalue.IsNullish(v) {
				return true
			}
			switch k {
			case "children":
				children = v
			case "dangerouslySetInnerHTML":
				innerHTML = v
			case "name":
				name = v
			case "formAction":
				formAction = v
			case "formEncType":
				formEncType = v
			case "formMethod":
				formMethod = v
			case "formTarget":
				formTarget = v
			default:
				pushAttribute(target, k, v)
			}
			return true
		})
		formData := pushFormActionAttribute(target, rs, st, formAction, formEncType, formMethod, formTarget, name)
		push(target, ">")
		for _, f := range formData {
			pushAdditionalFormField(target, f)
		}
		pushInnerHTML(target, innerHTML, children)
		if s, ok := children.(string); ok {
			push(target, escapeTextForBrowser(s))
			return nil
		}
		return children
	case "form":
		push(target, startChunkForTag("form"))
		var children, innerHTML, formAction, formEncType, formMethod, formTarget any
		props.Range(func(k string, v any) bool {
			if jsvalue.IsNullish(v) {
				return true
			}
			switch k {
			case "children":
				children = v
			case "dangerouslySetInnerHTML":
				innerHTML = v
			case "action":
				formAction = v
			case "encType":
				formEncType = v
			case "method":
				formMethod = v
			case "target":
				formTarget = v
			default:
				pushAttribute(target, k, v)
			}
			return true
		})
		var formData []react.FormField
		var formActionName any
		hasFormActionName := false
		if jsvalue.IsFunction(formAction) {
			if f, ok := getCustomFormFields(rs, formAction); ok {
				formAction, formEncType, formMethod, formTarget = orEmpty(f.Action), f.EncType, f.Method, f.Target
				formData = f.Data
				formActionName, hasFormActionName = f.Name, true
			} else {
				push(target, " ", "action", "=\"", actionJavaScriptURL, "\"")
				formTarget, formMethod, formEncType, formAction = nil, nil, nil, nil
				injectFormReplayingRuntime(rs, st)
			}
		}
		if !jsvalue.IsNullish(formAction) {
			pushAttribute(target, "action", formAction)
		}
		if !jsvalue.IsNullish(formEncType) {
			pushAttribute(target, "encType", formEncType)
		}
		if !jsvalue.IsNullish(formMethod) {
			pushAttribute(target, "method", formMethod)
		}
		if !jsvalue.IsNullish(formTarget) {
			pushAttribute(target, "target", formTarget)
		}
		push(target, ">")
		if hasFormActionName && formActionName != nil {
			push(target, "<input type=\"hidden\"")
			pushStringAttribute(target, "name", formActionName)
			push(target, "/>")
			for _, f := range formData {
				pushAdditionalFormField(target, f)
			}
		}
		pushInnerHTML(target, innerHTML, children)
		if s, ok := children.(string); ok {
			push(target, escapeTextForBrowser(s))
			return nil
		}
		return children
	case "menuitem":
		push(target, startChunkForTag("menuitem"))
		props.Range(func(k string, v any) bool {
			if jsvalue.IsNullish(v) {
				return true
			}
			switch k {
			case "children", "dangerouslySetInnerHTML":
				panic(newError("menuitems cannot have `children` nor `dangerouslySetInnerHTML`."))
			default:
				pushAttribute(target, k, v)
			}
			return true
		})
		push(target, ">")
		return nil
	case "object":
		push(target, startChunkForTag("object"))
		var children, innerHTML any
		props.Range(func(k string, v any) bool {
			if jsvalue.IsNullish(v) {
				return true
			}
			switch k {
			case "children":
				children = v
			case "dangerouslySetInnerHTML":
				innerHTML = v
			case "data":
				sanitized := sanitizeURL(jsvalue.ToString(v))
				if sanitized == "" {
					return true
				}
				push(target, " ", "data", "=\"", escapeTextForBrowser(sanitized), "\"")
			default:
				pushAttribute(target, k, v)
			}
			return true
		})
		push(target, ">")
		pushInnerHTML(target, innerHTML, children)
		if s, ok := children.(string); ok {
			push(target, escapeTextForBrowser(s))
			return nil
		}
		return children
	case "title":
		noscript := fc.tagScope & 1
		isFallback := fc.tagScope & 4
		if fc.insertionMode == svgMode || noscript != 0 || !jsvalue.IsNullish(props.Get("itemProp")) {
			return pushTitleImpl(target, props)
		}
		if isFallback != 0 {
			return nil
		}
		pushTitleImpl(&st.hoistableChunks, props)
		return jsvalue.Undefined{}
	case "link":
		noscript := fc.tagScope & 1
		isFallback := fc.tagScope & 4
		rel, href, precedence := props.Get("rel"), props.Get("href"), props.Get("precedence")
		relS, relOK := rel.(string)
		hrefS, hrefOK := href.(string)
		_ = relS
		if fc.insertionMode == svgMode || noscript != 0 || !jsvalue.IsNullish(props.Get("itemProp")) || !relOK || !hrefOK || hrefS == "" {
			pushLinkImpl(target, props)
			return nil
		}
		if props.Get("rel") == "stylesheet" {
			precS, precOK := precedence.(string)
			if !precOK || !jsvalue.IsNullish(props.Get("disabled")) || jsvalue.Truthy(props.Get("onLoad")) || jsvalue.Truthy(props.Get("onError")) {
				return pushLinkImpl(target, props)
			}
			queue, hasQueue := st.styles.get(precS)
			resState, present := rs.styleResources.lookup(hrefS)
			if !present || resState != nil {
				rs.styleResources[hrefS] = nil
				if !hasQueue {
					queue = &styleQueue{precedence: escapeTextForBrowser(precS), sheets: newMap[string, *stylesheet]()}
					st.styles.set(precS, queue)
				}
				resProps := props.Clone()
				resProps.Set("data-precedence", props.Get("precedence"))
				resProps.Set("precedence", nil)
				res := &stylesheet{state: 0, props: resProps}
				if present && resState != nil {
					if len(resState.creds) == 2 {
						adoptPreloadCredentials(res.props, resState.creds)
					}
					if pr, ok := st.preloadStylesheets[hrefS]; ok && pr != nil && len(pr.chunks) > 0 {
						pr.chunks = pr.chunks[:0]
					} else {
						res.state = 1
					}
				}
				queue.sheets.set(hrefS, res)
				if hoistable != nil {
					hoistable.stylesheets.add(res)
				}
			} else if hasQueue {
				if sheet, ok := queue.sheets.get(hrefS); ok && sheet != nil && hoistable != nil {
					hoistable.stylesheets.add(sheet)
				}
			}
			if textEmbedded {
				push(target, textSeparator)
			}
			return nil
		}
		if jsvalue.Truthy(props.Get("onLoad")) || jsvalue.Truthy(props.Get("onError")) {
			return pushLinkImpl(target, props)
		}
		if textEmbedded {
			push(target, textSeparator)
		}
		if isFallback != 0 {
			return nil
		}
		return pushLinkImpl(&st.hoistableChunks, props)
	case "script":
		noscript := fc.tagScope & 1
		asyncProp := props.Get("async")
		srcS, srcOK := props.Get("src").(string)
		if !srcOK || srcS == "" || !jsvalue.Truthy(asyncProp) || isFuncOrSymbol(asyncProp) ||
			jsvalue.Truthy(props.Get("onLoad")) || jsvalue.Truthy(props.Get("onError")) ||
			fc.insertionMode == svgMode || noscript != 0 || !jsvalue.IsNullish(props.Get("itemProp")) {
			return pushScriptImpl(target, props)
		}
		key := srcS
		resources, preloads := rs.scriptResources, st.preloadScripts
		if props.Get("type") == "module" {
			resources, preloads = rs.moduleScriptResources, st.preloadModuleScripts
		}
		resState, present := resources.lookup(key)
		if !present || resState != nil {
			resources[key] = nil
			scriptProps := props
			if present && resState != nil {
				if len(resState.creds) == 2 {
					scriptProps = props.Clone()
					adoptPreloadCredentials(scriptProps, resState.creds)
				}
				if pr, ok := preloads[key]; ok && pr != nil {
					pr.chunks = pr.chunks[:0]
				}
			}
			res := &chunkList{}
			st.scripts.add(res)
			pushScriptImpl(&res.chunks, scriptProps)
		}
		if textEmbedded {
			push(target, textSeparator)
		}
		return nil
	case "style":
		noscript := fc.tagScope & 1
		precedence, href, nonce := props.Get("precedence"), props.Get("href"), props.Get("nonce")
		precS, precOK := precedence.(string)
		hrefS, hrefOK := href.(string)
		if fc.insertionMode == svgMode || noscript != 0 || !jsvalue.IsNullish(props.Get("itemProp")) || !precOK || !hrefOK || hrefS == "" {
			push(target, startChunkForTag("style"))
			children, innerHTML := pushProps(target, props, nil)
			push(target, ">")
			child := singleChild(children)
			if !isFuncOrSymbol(child) && !jsvalue.IsNullish(child) {
				push(target, escapeStyle(jsvalue.ToString(child)))
			}
			pushInnerHTML(target, innerHTML, children)
			push(target, endChunkForTag("style"))
			return nil
		}
		queue, hasQueue := st.styles.get(precS)
		resState, present := rs.styleResources.lookup(hrefS)
		if !present || resState != nil {
			rs.styleResources[hrefS] = nil
			if !hasQueue {
				queue = &styleQueue{precedence: escapeTextForBrowser(precS), sheets: newMap[string, *stylesheet]()}
				st.styles.set(precS, queue)
				hasQueue = true
			}
			nonceStyle := st.nonceStyle
			if !jsvalue.Truthy(nonceStyle) || jsvalue.StrictEquals(nonceStyle, nonce) {
				queue.hrefs = append(queue.hrefs, escapeTextForBrowser(hrefS))
				var children, innerHTML any
				props.Range(func(k string, v any) bool {
					if jsvalue.IsNullish(v) {
						return true
					}
					switch k {
					case "children":
						children = v
					case "dangerouslySetInnerHTML":
						innerHTML = v
					}
					return true
				})
				child := singleChild(children)
				if !isFuncOrSymbol(child) && !jsvalue.IsNullish(child) {
					queue.rules = append(queue.rules, escapeStyle(jsvalue.ToString(child)))
				}
				pushInnerHTML(&queue.rules, innerHTML, children)
			}
		}
		if hasQueue && hoistable != nil {
			hoistable.styles.add(queue)
		}
		if textEmbedded {
			push(target, textSeparator)
		}
		return jsvalue.Undefined{}
	case "meta":
		noscript := fc.tagScope & 1
		isFallback := fc.tagScope & 4
		if fc.insertionMode == svgMode || noscript != 0 || !jsvalue.IsNullish(props.Get("itemProp")) {
			return pushSelfClosing(target, props, "meta")
		}
		if textEmbedded {
			push(target, textSeparator)
		}
		if isFallback != 0 {
			return nil
		}
		if _, ok := props.Get("charSet").(string); ok {
			return pushSelfClosing(&st.charsetChunks, props, "meta")
		}
		if props.Get("name") == "viewport" {
			return pushSelfClosing(&st.viewportChunks, props, "meta")
		}
		return pushSelfClosing(&st.hoistableChunks, props, "meta")
	case "listing", "pre":
		push(target, startChunkForTag(typ))
		children, innerHTML := pushProps(target, props, nil)
		push(target, ">")
		if !jsvalue.IsNullish(innerHTML) {
			if !jsvalue.IsNullish(children) {
				panic(newError("Can only set one of `children` or `props.dangerouslySetInnerHTML`."))
			}
			if jsvalue.TypeOf(innerHTML) != "object" || !hasHTMLKey(innerHTML) {
				panic(newError("`props.dangerouslySetInnerHTML` must be in the form `{__html: ...}`. Please visit https://react.dev/link/dangerously-set-inner-html for more information."))
			}
			html := innerHTML.(*react.Object).Get("__html")
			if !jsvalue.IsNullish(html) {
				if s, ok := html.(string); ok && len(s) > 0 && s[0] == '\n' {
					push(target, "\n", s)
				} else {
					push(target, jsvalue.ToString(html))
				}
			}
		}
		if s, ok := children.(string); ok && len(s) > 0 && s[0] == '\n' {
			push(target, "\n")
		}
		return children
	case "img":
		pictureOrNoScript := fc.tagScope & 3
		src, srcSet := props.Get("src"), props.Get("srcSet")
		_, srcIsString := src.(string)
		_, srcSetIsString := srcSet.(string)
		if !(props.Get("loading") == "lazy" ||
			(!jsvalue.Truthy(src) && !jsvalue.Truthy(srcSet)) ||
			(!srcIsString && !jsvalue.IsNullish(src)) ||
			(!srcSetIsString && !jsvalue.IsNullish(srcSet)) ||
			props.Get("fetchPriority") == "low" ||
			pictureOrNoScript != 0) &&
			!isDataURL(src) && !isDataURL(srcSet) {
			if hoistable != nil && fc.tagScope&64 != 0 {
				hoistable.suspenseyImages = true
			}
			var sizes any = jsvalue.Undefined{}
			if s, ok := props.Get("sizes").(string); ok {
				sizes = s
			}
			var key string
			if jsvalue.Truthy(srcSet) {
				key = jsvalue.ToString(srcSet) + "\n"
				if jsvalue.Truthy(sizes) {
					key += jsvalue.ToString(sizes)
				}
			} else {
				key = jsvalue.ToString(src)
			}
			promotable := st.preloadImages
			if res, ok := promotable[key]; ok && res != nil {
				if props.Get("fetchPriority") == "high" || st.highImagePreloads.size() < 10 {
					delete(promotable, key)
					st.highImagePreloads.add(res)
				}
			} else if _, present := rs.imageResources[key]; !present {
				rs.imageResources[key] = preloadNoCreds
				var crossOrigin any = jsvalue.Undefined{}
				if s, ok := props.Get("crossOrigin").(string); ok {
					if s == "use-credentials" {
						crossOrigin = s
					} else {
						crossOrigin = ""
					}
				}
				headers := st.headers
				var header string
				useHeader := false
				if headers != nil && headers.remainingCapacity > 0 && !srcSetIsString &&
					(props.Get("fetchPriority") == "high" || len(headers.highImagePreloads) < 500) {
					header = getPreloadAsHeader(src, "image", react.Obj(
						"imageSrcSet", props.Get("srcSet"),
						"imageSizes", props.Get("sizes"),
						"crossOrigin", crossOrigin,
						"integrity", props.Get("integrity"),
						"nonce", props.Get("nonce"),
						"type", props.Get("type"),
						"fetchPriority", props.Get("fetchPriority"),
						"referrerPolicy", props.Get("refererPolicy"),
					))
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
					var href any = src
					if jsvalue.Truthy(srcSet) {
						href = jsvalue.Undefined{}
					}
					pushLinkImpl(&res.chunks, react.Obj(
						"rel", "preload",
						"as", "image",
						"href", href,
						"imageSrcSet", srcSet,
						"imageSizes", sizes,
						"crossOrigin", crossOrigin,
						"integrity", props.Get("integrity"),
						"type", props.Get("type"),
						"fetchPriority", props.Get("fetchPriority"),
						"referrerPolicy", props.Get("referrerPolicy"),
					))
					if props.Get("fetchPriority") == "high" || st.highImagePreloads.size() < 10 {
						st.highImagePreloads.add(res)
					} else {
						st.bulkPreloads.add(res)
						promotable[key] = res
					}
				}
			}
		}
		return pushSelfClosing(target, props, "img")
	case "base", "area", "br", "col", "embed", "hr", "keygen", "param", "source", "track", "wbr":
		return pushSelfClosing(target, props, typ)
	case "head":
		if fc.insertionMode < htmlMode {
			preamble := preambleState
			if preamble == nil {
				preamble = st.preamble
			}
			if preamble.headChunks != nil {
				panic(newError("The `<head>` tag may only be rendered once."))
			}
			if preambleState != nil {
				push(target, "<!--head-->")
			}
			preamble.headChunks = &chunkList{}
			return pushStartSingletonElement(&preamble.headChunks.chunks, props, "head")
		}
		return pushStartGenericElement(target, props, "head")
	case "body":
		if fc.insertionMode < htmlMode {
			preamble := preambleState
			if preamble == nil {
				preamble = st.preamble
			}
			if preamble.bodyChunks != nil {
				panic(newError("The `<body>` tag may only be rendered once."))
			}
			if preambleState != nil {
				push(target, "<!--body-->")
			}
			preamble.bodyChunks = &chunkList{}
			return pushStartSingletonElement(&preamble.bodyChunks.chunks, props, "body")
		}
		return pushStartGenericElement(target, props, "body")
	case "html":
		if fc.insertionMode == rootHTMLMode {
			preamble := preambleState
			if preamble == nil {
				preamble = st.preamble
			}
			if preamble.htmlChunks != nil {
				panic(newError("The `<html>` tag may only be rendered once."))
			}
			if preambleState != nil {
				push(target, "<!--html-->")
			}
			doctype := ""
			if r.flavor == streamFlavor {
				doctype = "<!DOCTYPE html>"
			}
			preamble.htmlChunks = &chunkList{chunks: []string{doctype}}
			return pushStartSingletonElement(&preamble.htmlChunks.chunks, props, "html")
		}
		return pushStartGenericElement(target, props, "html")
	}
	if strings.Contains(typ, "-") {
		push(target, startChunkForTag(typ))
		var children, innerHTML any
		props.Range(func(k string, v any) bool {
			if jsvalue.IsNullish(v) {
				return true
			}
			attributeName := k
			switch k {
			case "children":
				children = v
				return true
			case "dangerouslySetInnerHTML":
				innerHTML = v
				return true
			case "style":
				pushStyleAttribute(target, v)
				return true
			case "suppressContentEditableWarning", "suppressHydrationWarning", "ref":
				return true
			case "className":
				attributeName = "class"
			}
			if isAttributeNameSafe(k) && !isFuncOrSymbol(v) && v != false {
				if v == true {
					v = ""
				} else if jsvalue.TypeOf(v) == "object" {
					return true
				}
				push(target, " ", attributeName, "=\"", escapeTextForBrowser(v), "\"")
			}
			return true
		})
		push(target, ">")
		pushInnerHTML(target, innerHTML, children)
		return children
	}
	return pushStartGenericElement(target, props, typ)
}

// utf16Len is a JavaScript string's length.
func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		if r >= 0x10000 {
			n += 2
		} else {
			n++
		}
	}
	return n
}
