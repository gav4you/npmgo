package server

import (
	"strconv"

	"github.com/gav4you/npmgo/internal/jsvalue"
)

// destination is where flushed chunks go: the string being built by
// renderToString, or a Writable (io.Writer) for the streaming renderer.
type destination interface {
	// write writes a chunk and reports whether the destination still has
	// capacity (writeChunkAndReturn / destination.push).
	write(chunk string) bool
	// beginWriting / completeWriting bracket a flush (stream buffering).
	beginWriting()
	completeWriting()
	flushBuffered()
	end()
	destroy(err any)
}

const (
	completeSegmentFunction            = `$RS=function(a,b){a=document.getElementById(a);b=document.getElementById(b);for(a.parentNode.removeChild(a);a.firstChild;)b.parentNode.insertBefore(a.firstChild,b);b.parentNode.removeChild(b)};`
	completeBoundaryFunction           = "$RB=[];$RV=function(a){$RT=performance.now();for(var b=0;b<a.length;b+=2){var c=a[b],e=a[b+1];null!==e.parentNode&&e.parentNode.removeChild(e);var f=c.parentNode;if(f){var g=c.previousSibling,h=0;do{if(c&&8===c.nodeType){var d=c.data;if(\"/$\"===d||\"/&\"===d)if(0===h)break;else h--;else\"$\"!==d&&\"$?\"!==d&&\"$~\"!==d&&\"$!\"!==d&&\"&\"!==d||h++}d=c.nextSibling;f.removeChild(c);c=d}while(c);for(;e.firstChild;)f.insertBefore(e.firstChild,c);g.data=\"$\";g._reactRetry&&requestAnimationFrame(g._reactRetry)}}a.length=0};\n$RC=function(a,b){if(b=document.getElementById(b))(a=document.getElementById(a))?(a.previousSibling.data=\"$~\",$RB.push(a,b),2===$RB.length&&(\"number\"!==typeof $RT?requestAnimationFrame($RV.bind(null,$RB)):(a=performance.now(),setTimeout($RV.bind(null,$RB),2300>a&&2E3<a?2300-a:$RT+300-a)))):b.parentNode.removeChild(b)};"
	completeBoundaryWithStylesFunction = "$RM=new Map;$RR=function(n,w,p){function u(q){this._p=null;q()}for(var r=new Map,t=document,h,b,e=t.querySelectorAll(\"link[data-precedence],style[data-precedence]\"),v=[],k=0;b=e[k++];)\"not all\"===b.getAttribute(\"media\")?v.push(b):(\"LINK\"===b.tagName&&$RM.set(b.getAttribute(\"href\"),b),r.set(b.dataset.precedence,h=b));e=0;b=[];var l,a;for(k=!0;;){if(k){var f=p[e++];if(!f){k=!1;e=0;continue}var c=!1,m=0;var d=f[m++];if(a=$RM.get(d)){var g=a._p;c=!0}else{a=t.createElement(\"link\");a.href=d;a.rel=\n\"stylesheet\";for(a.dataset.precedence=l=f[m++];g=f[m++];)a.setAttribute(g,f[m++]);g=a._p=new Promise(function(q,x){a.onload=u.bind(a,q);a.onerror=u.bind(a,x)});$RM.set(d,a)}d=a.getAttribute(\"media\");!g||d&&!matchMedia(d).matches||b.push(g);if(c)continue}else{a=v[e++];if(!a)break;l=a.getAttribute(\"data-precedence\");a.removeAttribute(\"media\")}c=r.get(l)||h;c===h&&(h=a);r.set(l,a);c?c.parentNode.insertBefore(a,c.nextSibling):(c=t.head,c.insertBefore(a,c.firstChild))}if(p=document.getElementById(n))p.previousSibling.data=\n\"$~\";Promise.all(b).then($RC.bind(null,n,w),$RX.bind(null,n,\"CSS failed to load\"))};"
	clientRenderFunction               = `$RX=function(b,c,d,e,f){var a=document.getElementById(b);a&&(b=a.previousSibling,b.data="$!",a=a.dataset,c&&(a.dgst=c),d&&(a.msg=d),e&&(a.stck=e),f&&(a.cstck=f),b._reactRetry&&b._reactRetry())};`
	shellTimeRuntimeScript             = "requestAnimationFrame(function(){$RT=performance.now()});"
)

func toHex(id int) string { return strconv.FormatInt(int64(id), 16) }

func itoa(i int) string { return strconv.Itoa(i) }

func writeBootstrap(d destination, st *renderState) bool {
	chunks := st.bootstrapChunks
	i := 0
	for ; i < len(chunks)-1; i++ {
		d.write(chunks[i])
	}
	if i < len(chunks) {
		last := chunks[i]
		st.bootstrapChunks = st.bootstrapChunks[:0]
		return d.write(last)
	}
	return true
}

func writeStartPendingSuspenseBoundary(d destination, st *renderState, id int) bool {
	d.write("<!--$?--><template id=\"")
	if id == -1 {
		panic(newError("An ID must have been assigned before we can complete the boundary."))
	}
	d.write(st.boundaryPrefix)
	d.write(toHex(id))
	return d.write("\"></template>")
}

func writeStartSegment(d destination, st *renderState, fc *formatContext, id int) bool {
	var open string
	switch fc.insertionMode {
	case rootHTMLMode, htmlHTMLMode, htmlHeadMode, htmlMode:
		open = "<div hidden id=\""
	case svgMode:
		open = "<svg aria-hidden=\"true\" style=\"display:none\" id=\""
	case mathMLMode:
		open = "<math aria-hidden=\"true\" style=\"display:none\" id=\""
	case htmlTableMode:
		open = "<table hidden id=\""
	case htmlTableBody:
		open = "<table hidden><tbody id=\""
	case htmlTableRowMode:
		open = "<table hidden><tr id=\""
	case htmlColgroupMode:
		open = "<table hidden><colgroup id=\""
	default:
		panic(newError("Unknown insertion mode. This is a bug in React."))
	}
	d.write(open)
	d.write(st.segmentPrefix)
	d.write(toHex(id))
	return d.write("\">")
}

func writeEndSegment(d destination, fc *formatContext) bool {
	switch fc.insertionMode {
	case rootHTMLMode, htmlHTMLMode, htmlHeadMode, htmlMode:
		return d.write("</div>")
	case svgMode:
		return d.write("</svg>")
	case mathMLMode:
		return d.write("</math>")
	case htmlTableMode:
		return d.write("</table>")
	case htmlTableBody:
		return d.write("</tbody></table>")
	case htmlTableRowMode:
		return d.write("</tr></table>")
	case htmlColgroupMode:
		return d.write("</colgroup></table>")
	}
	panic(newError("Unknown insertion mode. This is a bug in React."))
}

func (r *request) flushStyleTagsLateForBoundary(d destination, q *styleQueue) {
	rules, hrefs := q.rules, q.hrefs
	if len(hrefs) == 0 {
		return
	}
	d.write(r.renderState.startInlineStyle)
	d.write(" media=\"not all\" data-precedence=\"")
	d.write(q.precedence)
	d.write("\" data-href=\"")
	i := 0
	for ; i < len(hrefs)-1; i++ {
		d.write(hrefs[i])
		d.write(" ")
	}
	d.write(hrefs[i])
	d.write("\">")
	for _, rule := range rules {
		d.write(rule)
	}
	r.destinationHasCapacity = d.write("</style>")
	r.currentlyRenderingBoundaryHasStylesToHoist = true
	q.rules = q.rules[:0]
	q.hrefs = q.hrefs[:0]
}

func (r *request) writeHoistablesForBoundary(d destination, h *hoistableState, st *renderState) bool {
	r.currentlyRenderingBoundaryHasStylesToHoist = false
	r.destinationHasCapacity = true
	h.styles.forEach(func(q *styleQueue) { r.flushStyleTagsLateForBoundary(d, q) })
	h.stylesheets.forEach(func(s *stylesheet) {
		if s.state != 2 {
			r.currentlyRenderingBoundaryHasStylesToHoist = true
		}
	})
	if r.currentlyRenderingBoundaryHasStylesToHoist {
		st.stylesToHoist = true
	}
	return r.destinationHasCapacity
}

func flushResource(d destination, res *chunkList) {
	for _, c := range res.chunks {
		d.write(c)
	}
	res.chunks = res.chunks[:0]
}

func flushStyleInPreamble(d destination, s *stylesheet) {
	var q []string
	pushLinkImpl(&q, s.props)
	for _, c := range q {
		d.write(c)
	}
	s.state = 2
}

func flushStylesInPreamble(d destination, st *renderState, q *styleQueue) {
	hasStylesheets := q.sheets.size() > 0
	q.sheets.forEach(func(_ string, s *stylesheet) { flushStyleInPreamble(d, s) })
	q.sheets.clear()
	rules, hrefs := q.rules, q.hrefs
	if !hasStylesheets || len(hrefs) > 0 {
		d.write(st.startInlineStyle)
		d.write(" data-precedence=\"")
		d.write(q.precedence)
		if len(hrefs) > 0 {
			d.write("\" data-href=\"")
			i := 0
			for ; i < len(hrefs)-1; i++ {
				d.write(hrefs[i])
				d.write(" ")
			}
			d.write(hrefs[i])
		}
		d.write("\">")
		for _, rule := range rules {
			d.write(rule)
		}
		d.write("</style>")
		q.rules = q.rules[:0]
		q.hrefs = q.hrefs[:0]
	}
}

func preloadLateStyle(d destination, s *stylesheet) {
	if s.state != 0 {
		return
	}
	s.state = 1
	p := s.props
	var q []string
	pushLinkImpl(&q, newObj(
		"rel", "preload",
		"as", "style",
		"href", p.Get("href"),
		"crossOrigin", p.Get("crossOrigin"),
		"fetchPriority", p.Get("fetchPriority"),
		"integrity", p.Get("integrity"),
		"media", p.Get("media"),
		"hrefLang", p.Get("hrefLang"),
		"referrerPolicy", p.Get("referrerPolicy"),
	))
	for _, c := range q {
		d.write(c)
	}
}

func preloadLateStyles(d destination, q *styleQueue) {
	q.sheets.forEach(func(_ string, s *stylesheet) { preloadLateStyle(d, s) })
	q.sheets.clear()
}

func writeStyleResourceDependenciesInJS(d destination, h *hoistableState) {
	d.write("[")
	next := "["
	h.stylesheets.forEach(func(res *stylesheet) {
		if res.state == 2 {
			return
		}
		if res.state == 3 {
			d.write(next)
			d.write(escapeJSObjectForInstructionScripts(jsvalue.ToString(res.props.Get("href"))))
			d.write("]")
			next = ",["
			return
		}
		d.write(next)
		precedence := res.props.Get("data-precedence")
		props := res.props
		coercedHref := jsvalue.ToString(sanitizeURL(jsvalue.ToString(props.Get("href"))))
		d.write(escapeJSObjectForInstructionScripts(coercedHref))
		d.write(",")
		d.write(escapeJSObjectForInstructionScripts(jsvalue.ToString(precedence)))
		props.Range(func(k string, v any) bool {
			if jsvalue.IsNullish(v) {
				return true
			}
			switch k {
			case "href", "rel", "precedence", "data-precedence":
			case "children", "dangerouslySetInnerHTML":
				panic(newError("link is a self-closing tag and must neither have `children` nor use `dangerouslySetInnerHTML`."))
			default:
				writeStyleResourceAttributeInJS(d, k, v)
			}
			return true
		})
		d.write("]")
		next = ",["
		res.state = 3
	})
	d.write("]")
}

func writeStyleResourceAttributeInJS(d destination, name string, value any) {
	attributeName := lower(name)
	switch jsvalue.TypeOf(value) {
	case "function", "symbol":
		return
	}
	var str string
	switch name {
	case "innerHTML", "dangerouslySetInnerHTML", "suppressContentEditableWarning", "suppressHydrationWarning", "style", "ref":
		return
	case "className":
		attributeName = "class"
		str = jsvalue.ToString(value)
	case "hidden":
		if value == false {
			return
		}
		str = ""
	case "src", "href":
		str = jsvalue.ToString(sanitizeURL(value))
	default:
		if (len(name) > 2 && (name[0] == 'o' || name[0] == 'O') && (name[1] == 'n' || name[1] == 'N')) || !isAttributeNameSafe(name) {
			return
		}
		str = jsvalue.ToString(value)
	}
	d.write(",")
	d.write(escapeJSObjectForInstructionScripts(attributeName))
	d.write(",")
	d.write(escapeJSObjectForInstructionScripts(str))
}

// flushSubtree writes a segment's chunks with its children interleaved.
func (r *request) flushSubtree(d destination, seg *segment, h *hoistableState) bool {
	seg.parentFlushed = true
	switch seg.status {
	case segPending:
		seg.id = r.nextSegmentID
		r.nextSegmentID++
		fallthrough
	case segPostponed:
		id := seg.id
		seg.lastPushedText = false
		seg.textEmbedded = false
		d.write("<template id=\"")
		d.write(r.renderState.placeholderPrefix)
		d.write(toHex(id))
		return d.write("\"></template>")
	case segCompleted:
		seg.status = segFlushed
		ok := true
		chunks := seg.chunks
		chunkIdx := 0
		for _, child := range seg.children {
			for ; chunkIdx < child.index; chunkIdx++ {
				d.write(chunks[chunkIdx])
			}
			ok = r.flushSegment(d, child, h)
		}
		for ; chunkIdx < len(chunks)-1; chunkIdx++ {
			d.write(chunks[chunkIdx])
		}
		if chunkIdx < len(chunks) {
			ok = d.write(chunks[chunkIdx])
		}
		return ok
	case segAborted:
		return true
	}
	panic(newError("Aborted, errored or already flushed boundaries should not be flushed again. This is a bug in React."))
}

func (r *request) flushSegment(d destination, seg *segment, h *hoistableState) bool {
	b := seg.boundary
	if b == nil {
		return r.flushSubtree(d, seg, h)
	}
	b.parentFlushed = true
	st := r.renderState
	if b.status == boundaryClientRendered {
		if row := b.row; row != nil {
			row.pendingTasks--
			if row.pendingTasks == 0 {
				r.finishSuspenseListRow(row)
			}
		}
		if !st.generateStaticMarkup {
			d.write("<!--$!-->")
			d.write("<template")
			if jsvalue.Truthy(b.errorDigest) {
				d.write(" data-dgst=\"")
				d.write(escapeTextForBrowser(b.errorDigest))
				d.write("\"")
			}
			d.write("></template>")
		}
		r.flushSubtree(d, seg, h)
		if st.generateStaticMarkup {
			return true
		}
		return d.write("<!--/$-->")
	}
	if b.status != boundaryCompleted {
		if b.status == boundaryPending {
			b.rootSegmentID = r.nextSegmentID
			r.nextSegmentID++
		}
		if len(b.completedSegments) > 0 {
			r.partialBoundaries = append(r.partialBoundaries, b)
		}
		writeStartPendingSuspenseBoundary(d, st, b.rootSegmentID)
		if h != nil {
			hoistHoistables(h, b.fallbackState)
		}
		r.flushSubtree(d, seg, h)
		return d.write("<!--/$-->")
	}
	if !r.flushingPartialBoundaries && r.isEligibleForOutlining(b) &&
		(float64(r.flushedByteSize+b.byteSize) > r.progressiveChunkSize ||
			(r.flavor == streamFlavor && hasSuspenseyContent(b.contentState))) {
		b.rootSegmentID = r.nextSegmentID
		r.nextSegmentID++
		r.completedBoundaries = append(r.completedBoundaries, b)
		writeStartPendingSuspenseBoundary(d, st, b.rootSegmentID)
		r.flushSubtree(d, seg, h)
		return d.write("<!--/$-->")
	}
	r.flushedByteSize += b.byteSize
	if h != nil {
		hoistHoistables(h, b.contentState)
	}
	if row := b.row; row != nil && r.isEligibleForOutlining(b) {
		row.pendingTasks--
		if row.pendingTasks == 0 {
			r.finishSuspenseListRow(row)
		}
	}
	if !st.generateStaticMarkup {
		d.write("<!--$-->")
	}
	if len(b.completedSegments) != 1 {
		panic(newError("A previously unvisited boundary must have exactly one root segment. This is a bug in React."))
	}
	r.flushSegment(d, b.completedSegments[0], h)
	if st.generateStaticMarkup {
		return true
	}
	return d.write("<!--/$-->")
}

func (r *request) flushSegmentContainer(d destination, seg *segment, h *hoistableState) bool {
	writeStartSegment(d, r.renderState, seg.parentFormatContext, seg.id)
	r.flushSegment(d, seg, h)
	return writeEndSegment(d, seg.parentFormatContext)
}

func (r *request) flushCompletedBoundary(d destination, b *boundary) bool {
	r.flushedByteSize = b.byteSize
	for _, seg := range b.completedSegments {
		r.flushPartiallyCompletedSegment(d, b, seg)
	}
	b.completedSegments = b.completedSegments[:0]
	if row := b.row; row != nil && r.isEligibleForOutlining(b) {
		row.pendingTasks--
		if row.pendingTasks == 0 {
			r.finishSuspenseListRow(row)
		}
	}
	r.writeHoistablesForBoundary(d, b.contentState, r.renderState)
	rs, st := r.resumableState, r.renderState
	id := b.rootSegmentID
	h := b.contentState
	requiresStyleInsertion := st.stylesToHoist
	st.stylesToHoist = false
	d.write(st.startInlineScript)
	d.write(">")
	if requiresStyleInsertion {
		if rs.instructions&4 == 0 {
			rs.instructions |= 4
			d.write(clientRenderFunction)
		}
		if rs.instructions&2 == 0 {
			rs.instructions |= 2
			d.write(completeBoundaryFunction)
		}
		if rs.instructions&8 == 0 {
			rs.instructions |= 8
			d.write(completeBoundaryWithStylesFunction + "$RR(\"")
		} else {
			d.write("$RR(\"")
		}
	} else {
		if rs.instructions&2 == 0 {
			rs.instructions |= 2
			d.write(completeBoundaryFunction)
		}
		d.write("$RC(\"")
	}
	idHex := toHex(id)
	d.write(st.boundaryPrefix)
	d.write(idHex)
	d.write("\",\"")
	d.write(st.segmentPrefix)
	d.write(idHex)
	if requiresStyleInsertion {
		d.write("\",")
		writeStyleResourceDependenciesInJS(d, h)
	} else {
		d.write("\"")
	}
	ok := d.write(")</script>")
	return writeBootstrap(d, st) && ok
}

func (r *request) flushPartiallyCompletedSegment(d destination, b *boundary, seg *segment) bool {
	if seg.status == segFlushed {
		return true
	}
	h := b.contentState
	segmentID := seg.id
	if segmentID == -1 {
		seg.id = b.rootSegmentID
		if seg.id == -1 {
			panic(newError("A root segment ID must have been assigned by now. This is a bug in React."))
		}
		return r.flushSegmentContainer(d, seg, h)
	}
	if segmentID == b.rootSegmentID {
		return r.flushSegmentContainer(d, seg, h)
	}
	r.flushSegmentContainer(d, seg, h)
	rs, st := r.resumableState, r.renderState
	d.write(st.startInlineScript)
	d.write(">")
	if rs.instructions&1 == 0 {
		rs.instructions |= 1
		d.write(completeSegmentFunction + "$RS(\"")
	} else {
		d.write("$RS(\"")
	}
	d.write(st.segmentPrefix)
	idHex := toHex(segmentID)
	d.write(idHex)
	d.write("\",\"")
	d.write(st.placeholderPrefix)
	d.write(idHex)
	return d.write("\")</script>")
}

// flushCompletedQueues writes everything that is ready to the destination.
func (r *request) flushCompletedQueues(d destination) {
	defer func() {
		r.flushingPartialBoundaries = false
		if r.allPendingTasks == 0 && len(r.clientRenderedBoundaries) == 0 && len(r.completedBoundaries) == 0 {
			r.flushScheduled = false
			rs := r.resumableState
			if rs.hasBody {
				d.write(endChunkForTag("body"))
			}
			if rs.hasHtml {
				d.write(endChunkForTag("html"))
			}
			d.completeWriting()
			d.flushBuffered()
			r.status = statusClosed
			d.end()
			r.destination = nil
		} else {
			d.completeWriting()
			d.flushBuffered()
		}
	}()
	d.beginWriting()
	if r.pendingRootTasks > 0 {
		return
	}
	completedRoot := r.completedRootSegment
	if completedRoot != nil {
		if completedRoot.status == segPostponed {
			return
		}
		completedPreambleSegments := r.completedPreambleSegments
		if completedPreambleSegments == nil {
			return
		}
		r.flushedByteSize = r.byteSize
		rs, st := r.resumableState, r.renderState
		preamble := st.preamble
		htmlChunks, headChunks := preamble.htmlChunks, preamble.headChunks
		if htmlChunks != nil {
			for _, c := range htmlChunks.chunks {
				d.write(c)
			}
			if headChunks != nil {
				for _, c := range headChunks.chunks {
					d.write(c)
				}
			} else {
				d.write(startChunkForTag("head"))
				d.write(">")
			}
		} else if headChunks != nil {
			for _, c := range headChunks.chunks {
				d.write(c)
			}
		}
		for _, c := range st.charsetChunks {
			d.write(c)
		}
		st.charsetChunks = st.charsetChunks[:0]
		st.preconnects.forEach(func(c *chunkList) { flushResource(d, c) })
		st.preconnects.clear()
		for _, c := range st.viewportChunks {
			d.write(c)
		}
		st.viewportChunks = st.viewportChunks[:0]
		st.fontPreloads.forEach(func(c *chunkList) { flushResource(d, c) })
		st.fontPreloads.clear()
		st.highImagePreloads.forEach(func(c *chunkList) { flushResource(d, c) })
		st.highImagePreloads.clear()
		st.styles.forEach(func(_ string, q *styleQueue) { flushStylesInPreamble(d, st, q) })
		for _, c := range st.importMapChunks {
			d.write(c)
		}
		st.importMapChunks = st.importMapChunks[:0]
		st.bootstrapScripts.forEach(func(c *chunkList) { flushResource(d, c) })
		st.scripts.forEach(func(c *chunkList) { flushResource(d, c) })
		st.scripts.clear()
		st.bulkPreloads.forEach(func(c *chunkList) { flushResource(d, c) })
		st.bulkPreloads.clear()
		if r.flavor == legacyFlavor || (htmlChunks == nil && headChunks == nil) {
			rs.instructions |= 32
		}
		for _, c := range st.hoistableChunks {
			d.write(c)
		}
		st.hoistableChunks = st.hoistableChunks[:0]
		for _, segs := range completedPreambleSegments {
			for _, s := range segs {
				r.flushSegment(d, s, nil)
			}
		}
		if preamble.htmlChunks != nil || preamble.headChunks != nil {
			d.write(endChunkForTag("head"))
		}
		if preamble.bodyChunks != nil {
			for _, c := range preamble.bodyChunks.chunks {
				d.write(c)
			}
		}
		r.flushSegment(d, completedRoot, nil)
		r.completedRootSegment = nil
		if r.allPendingTasks != 0 || len(r.clientRenderedBoundaries) != 0 || len(r.completedBoundaries) != 0 {
			if rs.instructions&64 == 0 {
				rs.instructions |= 64
				d.write(st.startInlineScript)
				if rs.instructions&32 == 0 {
					rs.instructions |= 32
					d.write(" id=\"")
					d.write(escapeTextForBrowser("_" + rs.idPrefix + "R_"))
					d.write("\"")
				}
				d.write(">")
				d.write(shellTimeRuntimeScript)
				d.write("</script>")
			}
		}
		writeBootstrap(d, st)
	}
	st := r.renderState
	for _, c := range st.viewportChunks {
		d.write(c)
	}
	st.viewportChunks = st.viewportChunks[:0]
	st.preconnects.forEach(func(c *chunkList) { flushResource(d, c) })
	st.preconnects.clear()
	st.fontPreloads.forEach(func(c *chunkList) { flushResource(d, c) })
	st.fontPreloads.clear()
	st.highImagePreloads.forEach(func(c *chunkList) { flushResource(d, c) })
	st.highImagePreloads.clear()
	st.styles.forEach(func(_ string, q *styleQueue) { preloadLateStyles(d, q) })
	st.scripts.forEach(func(c *chunkList) { flushResource(d, c) })
	st.scripts.clear()
	st.bulkPreloads.forEach(func(c *chunkList) { flushResource(d, c) })
	st.bulkPreloads.clear()
	for _, c := range st.hoistableChunks {
		d.write(c)
	}
	st.hoistableChunks = st.hoistableChunks[:0]

	i := 0
	for ; i < len(r.clientRenderedBoundaries); i++ {
		b := r.clientRenderedBoundaries[i]
		rs := r.resumableState
		d.write(st.startInlineScript)
		d.write(">")
		if rs.instructions&4 == 0 {
			rs.instructions |= 4
			d.write(clientRenderFunction + ";$RX(\"")
		} else {
			d.write("$RX(\"")
		}
		d.write(st.boundaryPrefix)
		d.write(toHex(b.rootSegmentID))
		d.write("\"")
		if jsvalue.Truthy(b.errorDigest) {
			d.write(",")
			d.write(escapeJSStringsForInstructionScripts(jsvalue.ToString(b.errorDigest)))
		}
		if !d.write(")</script>") {
			r.destination = nil
			i++
			r.clientRenderedBoundaries = splice(r.clientRenderedBoundaries, i)
			return
		}
	}
	r.clientRenderedBoundaries = splice(r.clientRenderedBoundaries, i)

	for i = 0; i < len(r.completedBoundaries); i++ {
		if !r.flushCompletedBoundary(d, r.completedBoundaries[i]) {
			r.destination = nil
			i++
			r.completedBoundaries = splice(r.completedBoundaries, i)
			return
		}
	}
	r.completedBoundaries = splice(r.completedBoundaries, i)

	d.completeWriting()
	d.beginWriting()
	r.flushingPartialBoundaries = true
	r.destinationHasCapacity = true
	for i = 0; i < len(r.partialBoundaries); i++ {
		b := r.partialBoundaries[i]
		ok := func() bool {
			r.flushedByteSize = b.byteSize
			j := 0
			for ; j < len(b.completedSegments); j++ {
				if !r.flushPartiallyCompletedSegment(d, b, b.completedSegments[j]) {
					j++
					b.completedSegments = splice(b.completedSegments, j)
					return false
				}
			}
			b.completedSegments = splice(b.completedSegments, j)
			if row := b.row; row != nil && row.together && b.pendingTasks == 1 {
				if row.pendingTasks == 1 {
					r.unblockSuspenseListRow(row, row.hoistables)
				} else {
					row.pendingTasks--
				}
			}
			return r.writeHoistablesForBoundary(d, b.contentState, r.renderState)
		}()
		if !ok {
			r.destination = nil
			i++
			r.partialBoundaries = splice(r.partialBoundaries, i)
			return
		}
	}
	r.partialBoundaries = splice(r.partialBoundaries, i)
	r.flushingPartialBoundaries = false

	for i = 0; i < len(r.completedBoundaries); i++ {
		if !r.flushCompletedBoundary(d, r.completedBoundaries[i]) {
			r.destination = nil
			i++
			r.completedBoundaries = splice(r.completedBoundaries, i)
			return
		}
	}
	r.completedBoundaries = splice(r.completedBoundaries, i)
}

// splice is arr.splice(0, n): it drops the first n items.
func splice[T any](s []T, n int) []T {
	if n >= len(s) {
		return s[:0:0]
	}
	return append([]T(nil), s[n:]...)
}
