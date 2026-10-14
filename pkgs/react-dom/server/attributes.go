package server

import (
	"strings"

	"github.com/gav4you/npmgo/internal/jsvalue"
	"github.com/gav4you/npmgo/pkgs/react"
)

func push(target *[]string, chunks ...string) { *target = append(*target, chunks...) }

// ownProps enumerates `for (key in obj) if (hasOwnProperty.call(obj, key))`
// for the modelled objects.
func ownProps(obj any, f func(k string, v any)) {
	switch o := obj.(type) {
	case *react.Object:
		o.Range(func(k string, v any) bool { f(k, v); return true })
		return
	}
	if jsvalue.IsArray(obj) {
		for i, v := range jsvalue.Items(obj) {
			f(itoa(i), v)
		}
	}
}

func pushStyleAttribute(target *[]string, style any) {
	if jsvalue.TypeOf(style) != "object" {
		panic(newError("The `style` prop expects a mapping from style properties to values, not a string. For example, style={{marginRight: spacing + 'em'}} when using JSX."))
	}
	isFirst := true
	ownProps(style, func(styleName string, styleValue any) {
		if jsvalue.IsNullish(styleValue) || jsvalue.TypeOf(styleValue) == "boolean" || styleValue == "" {
			return
		}
		var nameChunk, valueChunk string
		if strings.HasPrefix(styleName, "--") {
			nameChunk = escapeTextForBrowser(styleName)
			valueChunk = escapeTextForBrowser(jsvalue.Trim(jsvalue.ToString(styleValue)))
		} else {
			if c, ok := styleNameCache.Load(styleName); ok {
				nameChunk = c.(string)
			} else {
				nameChunk = escapeTextForBrowser(hyphenateStyleName(styleName))
				styleNameCache.Store(styleName, nameChunk)
			}
			if jsvalue.IsNumber(styleValue) {
				if jsvalue.Float(styleValue) == 0 || unitlessNumbers[styleName] {
					valueChunk = jsvalue.ToString(styleValue)
				} else {
					valueChunk = jsvalue.ToString(styleValue) + "px"
				}
			} else {
				valueChunk = escapeTextForBrowser(jsvalue.Trim(jsvalue.ToString(styleValue)))
			}
		}
		if isFirst {
			isFirst = false
			push(target, " style=\"", nameChunk, ":", valueChunk)
		} else {
			push(target, ";", nameChunk, ":", valueChunk)
		}
	})
	if !isFirst {
		push(target, "\"")
	}
}

func isFuncOrSymbol(v any) bool {
	t := jsvalue.TypeOf(v)
	return t == "function" || t == "symbol"
}

func pushBooleanAttribute(target *[]string, name string, value any) {
	if jsvalue.Truthy(value) && !isFuncOrSymbol(value) {
		push(target, " ", name, "=\"\"")
	}
}

func pushStringAttribute(target *[]string, name string, value any) {
	t := jsvalue.TypeOf(value)
	if t != "function" && t != "symbol" && t != "boolean" {
		push(target, " ", name, "=\"", escapeTextForBrowser(value), "\"")
	}
}

var actionJavaScriptURL = escapeString("javascript:throw new Error('React form unexpectedly submitted.')")

const formReplayingRuntimeScript = `addEventListener("submit",function(a){if(!a.defaultPrevented){var c=a.target,d=a.submitter,e=c.action,b=d;if(d){var f=d.getAttribute("formAction");null!=f&&(e=f,b=null)}"javascript:throw new Error('React form unexpectedly submitted.')"===e&&(a.preventDefault(),b?(a=document.createElement("input"),a.name=b.name,a.value=b.value,b.parentNode.insertBefore(a,b),b=new FormData(c),a.parentNode.removeChild(a)):b=new FormData(c),a=c.ownerDocument||c,(a.$$reactFormReplay=a.$$reactFormReplay||[]).push(c,d,b))}});`

func pushAdditionalFormField(target *[]string, f react.FormField) {
	push(target, "<input type=\"hidden\"")
	validateAdditionalFormField(f.Value)
	pushStringAttribute(target, "name", f.Name)
	pushStringAttribute(target, "value", f.Value)
	push(target, "/>")
}

func validateAdditionalFormField(value any) {
	if _, ok := value.(string); !ok {
		panic(newError("File/Blob fields are not yet supported in progressive forms. Will fallback to client hydration."))
	}
}

// formActionOf returns the action's $$FORM_ACTION, if it has one.
func formActionOf(action any) func(string) *react.FormActionFields {
	if a, ok := action.(*react.Action); ok && a != nil {
		return a.FormAction
	}
	return nil
}

// getCustomFormFields calls $$FORM_ACTION. ok is false for JavaScript null.
func getCustomFormFields(rs *resumableState, formAction any) (fields *react.FormActionFields, ok bool) {
	fa := formActionOf(formAction)
	if fa == nil {
		return nil, false
	}
	id := rs.nextFormID
	rs.nextFormID++
	prefix := rs.idPrefix + itoa(id)
	thrown, panicked := try(func() {
		fields = fa(prefix)
		if fields != nil {
			for _, d := range fields.Data {
				validateAdditionalFormField(d.Value)
			}
		}
	})
	if panicked {
		if _, isThenable := react.IsThenable(react.Thrown(thrown)); isThenable {
			panic(thrown)
		}
		return nil, false
	}
	if fields == nil {
		return nil, false
	}
	return fields, true
}

func orEmpty(v any) any {
	if jsvalue.Truthy(v) {
		return v
	}
	return ""
}

func pushFormActionAttribute(target *[]string, rs *resumableState, st *renderState, formAction, formEncType, formMethod, formTarget, name any) []react.FormField {
	var formData []react.FormField
	if jsvalue.IsFunction(formAction) {
		if f, ok := getCustomFormFields(rs, formAction); ok {
			name, formAction, formEncType, formMethod, formTarget = f.Name, orEmpty(f.Action), f.EncType, f.Method, f.Target
			formData = f.Data
		} else {
			push(target, " ", "formAction", "=\"", actionJavaScriptURL, "\"")
			formTarget, formMethod, formEncType, formAction, name = nil, nil, nil, nil, nil
			injectFormReplayingRuntime(rs, st)
		}
	}
	if !jsvalue.IsNullish(name) {
		pushAttribute(target, "name", name)
	}
	if !jsvalue.IsNullish(formAction) {
		pushAttribute(target, "formAction", formAction)
	}
	if !jsvalue.IsNullish(formEncType) {
		pushAttribute(target, "formEncType", formEncType)
	}
	if !jsvalue.IsNullish(formMethod) {
		pushAttribute(target, "formMethod", formMethod)
	}
	if !jsvalue.IsNullish(formTarget) {
		pushAttribute(target, "formTarget", formTarget)
	}
	return formData
}

func injectFormReplayingRuntime(rs *resumableState, st *renderState) {
	if rs.instructions&16 != 0 {
		return
	}
	rs.instructions |= 16
	p := st.preamble
	if (p.htmlChunks != nil || p.headChunks != nil) && len(st.bootstrapChunks) == 0 {
		st.bootstrapChunks = append(st.bootstrapChunks, st.startInlineScript)
		pushCompletedShellIdAttribute(&st.bootstrapChunks, rs)
		st.bootstrapChunks = append(st.bootstrapChunks, ">", formReplayingRuntimeScript, "</script>")
	} else {
		st.bootstrapChunks = append([]string{st.startInlineScript, ">", formReplayingRuntimeScript, "</script>"}, st.bootstrapChunks...)
	}
}

func pushAttribute(target *[]string, name string, value any) {
	switch name {
	case "className":
		pushStringAttribute(target, "class", value)
	case "tabIndex":
		pushStringAttribute(target, "tabindex", value)
	case "dir", "role", "viewBox", "width", "height":
		pushStringAttribute(target, name, value)
	case "style":
		pushStyleAttribute(target, value)
	case "src", "href", "action", "formAction":
		if (name == "src" || name == "href") && value == "" {
			return
		}
		if jsvalue.IsNullish(value) || isFuncOrSymbol(value) || jsvalue.TypeOf(value) == "boolean" {
			return
		}
		v := sanitizeURL(jsvalue.ToString(value))
		push(target, " ", name, "=\"", escapeTextForBrowser(v), "\"")
	case "defaultValue", "defaultChecked", "innerHTML", "suppressContentEditableWarning", "suppressHydrationWarning", "ref":
	case "autoFocus", "multiple", "muted":
		pushBooleanAttribute(target, strings.ToLower(name), value)
	case "xlinkHref":
		if isFuncOrSymbol(value) || jsvalue.TypeOf(value) == "boolean" {
			return
		}
		v := sanitizeURL(jsvalue.ToString(value))
		push(target, " ", "xlink:href", "=\"", escapeTextForBrowser(v), "\"")
	case "contentEditable", "spellCheck", "draggable", "value", "autoReverse", "externalResourcesRequired", "focusable", "preserveAlpha":
		if !isFuncOrSymbol(value) {
			push(target, " ", name, "=\"", escapeTextForBrowser(value), "\"")
		}
	case "inert", "allowFullScreen", "async", "autoPlay", "controls", "default", "defer", "disabled",
		"disablePictureInPicture", "disableRemotePlayback", "formNoValidate", "hidden", "loop", "noModule",
		"noValidate", "open", "playsInline", "readOnly", "required", "reversed", "scoped", "seamless", "itemScope":
		if jsvalue.Truthy(value) && !isFuncOrSymbol(value) {
			push(target, " ", name, "=\"\"")
		}
	case "capture", "download":
		if value == true {
			push(target, " ", name, "=\"\"")
		} else if value != false && !isFuncOrSymbol(value) {
			push(target, " ", name, "=\"", escapeTextForBrowser(value), "\"")
		}
	case "cols", "rows", "size", "span":
		if !isFuncOrSymbol(value) && !jsvalue.IsNaN(value) && 1 <= jsvalue.ToNumber(value) {
			push(target, " ", name, "=\"", escapeTextForBrowser(value), "\"")
		}
	case "rowSpan", "start":
		if !isFuncOrSymbol(value) && !jsvalue.IsNaN(value) {
			push(target, " ", name, "=\"", escapeTextForBrowser(value), "\"")
		}
	case "xlinkActuate":
		pushStringAttribute(target, "xlink:actuate", value)
	case "xlinkArcrole":
		pushStringAttribute(target, "xlink:arcrole", value)
	case "xlinkRole":
		pushStringAttribute(target, "xlink:role", value)
	case "xlinkShow":
		pushStringAttribute(target, "xlink:show", value)
	case "xlinkTitle":
		pushStringAttribute(target, "xlink:title", value)
	case "xlinkType":
		pushStringAttribute(target, "xlink:type", value)
	case "xmlBase":
		pushStringAttribute(target, "xml:base", value)
	case "xmlLang":
		pushStringAttribute(target, "xml:lang", value)
	case "xmlSpace":
		pushStringAttribute(target, "xml:space", value)
	default:
		if len(name) > 2 && (name[0] == 'o' || name[0] == 'O') && (name[1] == 'n' || name[1] == 'N') {
			return
		}
		if a, ok := aliases[name]; ok {
			name = a
		}
		if !isAttributeNameSafe(name) {
			return
		}
		switch jsvalue.TypeOf(value) {
		case "function", "symbol":
			return
		case "boolean":
			prefix := strings.ToLower(name)
			if !strings.HasPrefix(prefix, "data-") && !strings.HasPrefix(prefix, "aria-") {
				return
			}
		}
		push(target, " ", name, "=\"", escapeTextForBrowser(value), "\"")
	}
}

func hasHTMLKey(innerHTML any) bool {
	if o, ok := innerHTML.(*react.Object); ok && o != nil {
		return o.Has("__html")
	}
	return false
}

func pushInnerHTML(target *[]string, innerHTML, children any) {
	if jsvalue.IsNullish(innerHTML) {
		return
	}
	if !jsvalue.IsNullish(children) {
		panic(newError("Can only set one of `children` or `props.dangerouslySetInnerHTML`."))
	}
	if jsvalue.TypeOf(innerHTML) != "object" || !hasHTMLKey(innerHTML) {
		panic(newError("`props.dangerouslySetInnerHTML` must be in the form `{__html: ...}`. Please visit https://react.dev/link/dangerously-set-inner-html for more information."))
	}
	html := innerHTML.(*react.Object).Get("__html")
	if !jsvalue.IsNullish(html) {
		push(target, jsvalue.ToString(html))
	}
}
