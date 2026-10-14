package server

import (
	"regexp"
	"strings"
	"sync"

	"github.com/gav4you/npmgo/internal/jsvalue"
)

// escapeTextForBrowser escapes ", &, ', <, > (numbers, booleans and bigints
// are stringified without escaping).
func escapeTextForBrowser(text any) string {
	switch jsvalue.TypeOf(text) {
	case "boolean", "number", "bigint":
		return jsvalue.ToString(text)
	}
	return escapeString(jsvalue.ToString(text))
}

func escapeString(s string) string {
	i := strings.IndexAny(s, "\"&'<>")
	if i < 0 {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 16)
	b.WriteString(s[:i])
	for ; i < len(s); i++ {
		switch s[i] {
		case '"':
			b.WriteString("&quot;")
		case '&':
			b.WriteString("&amp;")
		case '\'':
			b.WriteString("&#x27;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

var validAttributeNameRegex = regexp.MustCompile("^[:A-Z_a-z\\x{00C0}-\\x{00D6}\\x{00D8}-\\x{00F6}\\x{00F8}-\\x{02FF}\\x{0370}-\\x{037D}\\x{037F}-\\x{1FFF}\\x{200C}-\\x{200D}\\x{2070}-\\x{218F}\\x{2C00}-\\x{2FEF}\\x{3001}-\\x{D7FF}\\x{F900}-\\x{FDCF}\\x{FDF0}-\\x{FFFD}][:A-Z_a-z\\x{00C0}-\\x{00D6}\\x{00D8}-\\x{00F6}\\x{00F8}-\\x{02FF}\\x{0370}-\\x{037D}\\x{037F}-\\x{1FFF}\\x{200C}-\\x{200D}\\x{2070}-\\x{218F}\\x{2C00}-\\x{2FEF}\\x{3001}-\\x{D7FF}\\x{F900}-\\x{FDCF}\\x{FDF0}-\\x{FFFD}\\-.0-9\\x{00B7}\\x{0300}-\\x{036F}\\x{203F}-\\x{2040}]*$")

var attributeNameCache sync.Map // name -> bool

func isAttributeNameSafe(name string) bool {
	if v, ok := attributeNameCache.Load(name); ok {
		return v.(bool)
	}
	ok := validAttributeNameRegex.MatchString(name)
	attributeNameCache.Store(name, ok)
	return ok
}

var unitlessNumbers = func() map[string]bool {
	m := map[string]bool{}
	for _, k := range strings.Split("animationIterationCount aspectRatio borderImageOutset borderImageSlice borderImageWidth boxFlex boxFlexGroup boxOrdinalGroup columnCount columns flex flexGrow flexPositive flexShrink flexNegative flexOrder gridArea gridRow gridRowEnd gridRowSpan gridRowStart gridColumn gridColumnEnd gridColumnSpan gridColumnStart fontWeight lineClamp lineHeight opacity order orphans scale tabSize widows zIndex zoom fillOpacity floodOpacity stopOpacity strokeDasharray strokeDashoffset strokeMiterlimit strokeOpacity strokeWidth MozAnimationIterationCount MozBoxFlex MozBoxFlexGroup MozLineClamp msAnimationIterationCount msFlex msZoom msFlexGrow msFlexNegative msFlexOrder msFlexPositive msFlexShrink msGridColumn msGridColumnSpan msGridRow msGridRowSpan WebkitAnimationIterationCount WebkitBoxFlex WebKitBoxFlexGroup WebkitBoxOrdinalGroup WebkitColumnCount WebkitColumns WebkitFlex WebkitFlexGrow WebkitFlexPositive WebkitFlexShrink WebkitLineClamp", " ") {
		m[k] = true
	}
	return m
}()

var aliases = map[string]string{
	"acceptCharset": "accept-charset", "htmlFor": "for", "httpEquiv": "http-equiv", "crossOrigin": "crossorigin",
	"accentHeight": "accent-height", "alignmentBaseline": "alignment-baseline", "arabicForm": "arabic-form",
	"baselineShift": "baseline-shift", "capHeight": "cap-height", "clipPath": "clip-path", "clipRule": "clip-rule",
	"colorInterpolation": "color-interpolation", "colorInterpolationFilters": "color-interpolation-filters",
	"colorProfile": "color-profile", "colorRendering": "color-rendering", "dominantBaseline": "dominant-baseline",
	"enableBackground": "enable-background", "fillOpacity": "fill-opacity", "fillRule": "fill-rule",
	"floodColor": "flood-color", "floodOpacity": "flood-opacity", "fontFamily": "font-family", "fontSize": "font-size",
	"fontSizeAdjust": "font-size-adjust", "fontStretch": "font-stretch", "fontStyle": "font-style",
	"fontVariant": "font-variant", "fontWeight": "font-weight", "glyphName": "glyph-name",
	"glyphOrientationHorizontal": "glyph-orientation-horizontal", "glyphOrientationVertical": "glyph-orientation-vertical",
	"horizAdvX": "horiz-adv-x", "horizOriginX": "horiz-origin-x", "imageRendering": "image-rendering",
	"letterSpacing": "letter-spacing", "lightingColor": "lighting-color", "markerEnd": "marker-end",
	"markerMid": "marker-mid", "markerStart": "marker-start", "overlinePosition": "overline-position",
	"overlineThickness": "overline-thickness", "paintOrder": "paint-order", "panose-1": "panose-1",
	"pointerEvents": "pointer-events", "renderingIntent": "rendering-intent", "shapeRendering": "shape-rendering",
	"stopColor": "stop-color", "stopOpacity": "stop-opacity", "strikethroughPosition": "strikethrough-position",
	"strikethroughThickness": "strikethrough-thickness", "strokeDasharray": "stroke-dasharray",
	"strokeDashoffset": "stroke-dashoffset", "strokeLinecap": "stroke-linecap", "strokeLinejoin": "stroke-linejoin",
	"strokeMiterlimit": "stroke-miterlimit", "strokeOpacity": "stroke-opacity", "strokeWidth": "stroke-width",
	"textAnchor": "text-anchor", "textDecoration": "text-decoration", "textRendering": "text-rendering",
	"transformOrigin": "transform-origin", "underlinePosition": "underline-position",
	"underlineThickness": "underline-thickness", "unicodeBidi": "unicode-bidi", "unicodeRange": "unicode-range",
	"unitsPerEm": "units-per-em", "vAlphabetic": "v-alphabetic", "vHanging": "v-hanging",
	"vIdeographic": "v-ideographic", "vMathematical": "v-mathematical", "vectorEffect": "vector-effect",
	"vertAdvY": "vert-adv-y", "vertOriginX": "vert-origin-x", "vertOriginY": "vert-origin-y",
	"wordSpacing": "word-spacing", "writingMode": "writing-mode", "xmlnsXlink": "xmlns:xlink", "xHeight": "x-height",
}

// isJavaScriptProtocol is /^[\u0000-\u001F ]*j[\r\n\t]*a...t[\r\n\t]*:/i
// with ASCII-only case folding (JavaScript's non-unicode /i).
var isJavaScriptProtocol = regexp.MustCompile("^[\\x00-\\x1F ]*[jJ][\\r\\n\\t]*[aA][\\r\\n\\t]*[vV][\\r\\n\\t]*[aA][\\r\\n\\t]*[sS][\\r\\n\\t]*[cC][\\r\\n\\t]*[rR][\\r\\n\\t]*[iI][\\r\\n\\t]*[pP][\\r\\n\\t]*[tT][\\r\\n\\t]*:")

const blockedJavaScriptURL = "javascript:throw new Error('React has blocked a javascript: URL as a security precaution.')"

// sanitizeURL replaces javascript: URLs.
func sanitizeURL(url any) any {
	if isJavaScriptProtocol.MatchString(jsvalue.ToString(url)) {
		return blockedJavaScriptURL
	}
	return url
}

var styleNameCache sync.Map // styleName -> hyphenated, escaped name

// hyphenateStyleName is styleName.replace(/([A-Z])/g, "-$1").toLowerCase().replace(/^ms-/, "-ms-").
func hyphenateStyleName(name string) string {
	var b strings.Builder
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c >= 'A' && c <= 'Z' {
			b.WriteByte('-')
		}
		b.WriteByte(c)
	}
	s := strings.ToLower(b.String())
	if strings.HasPrefix(s, "ms-") {
		s = "-" + s
	}
	return s
}

// replaceScriptOrStyle implements str.replace(/(<\/|<)(s)(cript)/gi, ...) and
// the style equivalent, with ASCII-only case folding.
func replaceTagOpen(s, rest, lower, upper string) string {
	if !strings.Contains(s, "<") {
		return s
	}
	var b strings.Builder
	n := len(rest)
	for i := 0; i < len(s); {
		if s[i] != '<' {
			b.WriteByte(s[i])
			i++
			continue
		}
		j := i + 1
		prefix := "<"
		if j < len(s) && s[j] == '/' {
			prefix = "</"
			j++
		}
		if j+1+n <= len(s) && (s[j] == 's' || s[j] == 'S') && strings.EqualFold(s[j+1:j+1+n], rest) && isASCII(s[j+1:j+1+n]) {
			b.WriteString(prefix)
			if s[j] == 's' {
				b.WriteString(lower)
			} else {
				b.WriteString(upper)
			}
			b.WriteString(s[j+1 : j+1+n])
			i = j + 1 + n
			continue
		}
		b.WriteByte('<')
		i++
	}
	return b.String()
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

func escapeScript(s string) string { return replaceTagOpen(s, "cript", "\\u0073", "\\u0053") }
func escapeStyle(s string) string  { return replaceTagOpen(s, "tyle", "\\73 ", "\\53 ") }

// escapeJSStringsForInstructionScripts is JSON.stringify(s) with <, U+2028,
// U+2029 escaped.
func escapeJSStringsForInstructionScripts(s string) string {
	q := jsvalue.JSONQuote(s)
	r := strings.NewReplacer("<", "\\u003c", " ", "\\u2028", " ", "\\u2029")
	return r.Replace(q)
}

// escapeJSObjectForInstructionScripts is JSON.stringify(v) with &, >, <,
// U+2028, U+2029 escaped.
func escapeJSObjectForInstructionScripts(s string) string {
	q := jsvalue.JSONQuote(s)
	r := strings.NewReplacer("&", "\\u0026", ">", "\\u003e", "<", "\\u003c", " ", "\\u2028", " ", "\\u2029")
	return r.Replace(q)
}

// escapeHrefForLinkHeader is href.replace(/[<>\r\n]/g, ...).
func escapeHrefForLinkHeader(s string) string {
	return strings.NewReplacer("<", "%3C", ">", "%3E", "\n", "%0A", "\r", "%0D").Replace(s)
}

// escapeStringForLinkHeaderQuotedParam is s.replace(/["';,\r\n]/g, ...).
func escapeStringForLinkHeaderQuotedParam(s string) string {
	return strings.NewReplacer("\"", "%22", "'", "%27", ";", "%3B", ",", "%2C", "\n", "%0A", "\r", "%0D").Replace(s)
}

var validTagRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z:_\.\-\d]*$`)

var validatedTagCache sync.Map

func startChunkForTag(tag string) string {
	if v, ok := validatedTagCache.Load(tag); ok {
		return v.(string)
	}
	if !validTagRegex.MatchString(tag) {
		panic(newError("Invalid tag: " + tag))
	}
	chunk := "<" + tag
	validatedTagCache.Store(tag, chunk)
	return chunk
}

func endChunkForTag(tag string) string { return "</" + tag + ">" }
