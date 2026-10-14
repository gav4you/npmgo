package nodeutil

import (
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// Cwd is process.cwd(); replaceable so stack colouring is testable.
var Cwd = os.Getwd

// BuiltinModules is require('module').builtinModules for Node v25.9.0.
var BuiltinModules = []string{"_http_agent", "_http_client", "_http_common", "_http_incoming",
	"_http_outgoing", "_http_server", "_stream_duplex", "_stream_passthrough", "_stream_readable",
	"_stream_transform", "_stream_wrap", "_stream_writable", "_tls_common", "_tls_wrap", "assert",
	"assert/strict", "async_hooks", "buffer", "child_process", "cluster", "console", "constants",
	"crypto", "dgram", "diagnostics_channel", "dns", "dns/promises", "domain", "events", "fs",
	"fs/promises", "http", "http2", "https", "inspector", "inspector/promises", "module", "net", "os",
	"path", "path/posix", "path/win32", "perf_hooks", "process", "punycode", "querystring", "readline",
	"readline/promises", "repl", "stream", "stream/consumers", "stream/promises", "stream/web",
	"string_decoder", "sys", "timers", "timers/promises", "tls", "trace_events", "tty", "url", "util",
	"util/types", "v8", "vm", "wasi", "worker_threads", "zlib", "sea", "sqlite", "test", "test/reporters"}

// isBuiltinModule ports BuiltinModule.exists: public ids plus Node's internal/* ids.
func isBuiltinModule(id string) bool {
	if strings.HasPrefix(id, "internal/") {
		return true
	}
	for _, m := range BuiltinModules {
		if m == id {
			return true
		}
	}
	return false
}

var coreModuleRe = regexp.MustCompile(`^ {4}at (?:[^/\\(]+ \(|)node:(.+):\d+:\d+\)?$`)

// pathToFileURLHref ports url.pathToFileURL(path).href.
func pathToFileURLHref(p string) string {
	if runtime.GOOS == "windows" {
		p = filepath.ToSlash(p)
		if !strings.HasPrefix(p, "/") {
			p = "/" + p
		}
	}
	u := url.URL{Scheme: "file", Path: p}
	return u.String()
}

// asError ports isError: a native error, or any object whose prototype chain
// reaches Error.prototype (adapted through errorView).
func asError(v any) (*Error, bool) {
	switch x := v.(type) {
	case *Error:
		return x, true
	case *Object:
		if usesChain(x) && inChain(x, ErrorPrototype) {
			return errorView(x), true
		}
	}
	return nil, false
}

func getStackFrames(c *ctx, e *Error, stack string) []string {
	frames := strings.Split(stack, "\n")

	if cause, ok := asError(e.Cause); ok && e.HasCause {
		causeStack := getStackString(c, cause)
		if start := strings.Index(causeStack, "\n    at"); start != -1 {
			causeFrames := strings.Split(causeStack[start+1:], "\n")
			l, offset := identicalSequenceRange(frames, causeFrames)
			if l > 0 {
				skipped := l - 2
				msg := "    ... " + itoa(skipped) + " lines matching cause stack trace ..."
				frames = splice(frames, offset+1, skipped, c.stylize(msg, "undefined"))
			}
		}
	}

	if len(frames) > 10 {
		ranges := getDuplicateErrorFrameRanges(frames)
		for i := len(ranges) - 3; i >= 0; i -= 3 {
			offset, length, dup := ranges[i], ranges[i+1], ranges[i+2]
			msg := "    ... collapsed " + itoa(length*dup) + " duplicate lines matching above "
			if dup > 1 {
				msg += itoa(length) + " lines " + itoa(dup) + " times..."
			} else {
				msg += "lines ..."
			}
			frames = splice(frames, offset, length*dup, c.stylize(msg, "undefined"))
		}
	}
	return frames
}

// getStackString ports getStackString: a truthy string stack is used as is; any
// other truthy value is formatted under the error's toString, with Node's
// missing recurseTimes (NaN) semantics; a falsy stack means no stack.
func getStackString(c *ctx, e *Error) string {
	if !jsTruthy(e.Stack) {
		return errorToString(e)
	}
	if s, ok := e.Stack.(string); ok {
		return s
	}
	c.seen = append(c.seen, identity(e))
	c.indentationLvl += 4
	saved := c.nanDepth
	c.nanDepth = true
	r := formatValue(c, e.Stack, 0, false)
	c.nanDepth = saved
	c.indentationLvl -= 4
	c.seen = c.seen[:len(c.seen)-1]
	return errorToString(e) + "\n    " + r
}

func splice(s []string, start, del int, insert string) []string {
	if start > len(s) {
		start = len(s)
	}
	end := start + del
	if end > len(s) {
		end = len(s)
	}
	out := append([]string{}, s[:start]...)
	out = append(out, insert)
	return append(out, s[end:]...)
}

func indexOf(s []string, v string) int {
	for i, x := range s {
		if x == v {
			return i
		}
	}
	return -1
}

func identicalSequenceRange(a, b []string) (int, int) {
	for i := 0; i < len(a)-3; i++ {
		pos := indexOf(b, a[i])
		if pos != -1 {
			rest := len(b) - pos
			if rest > 3 {
				l := 1
				maxL := len(a) - i
				if rest < maxL {
					maxL = rest
				}
				for maxL > l && a[i+l] == b[pos+l] {
					l++
				}
				if l > 3 {
					return l, i
				}
			}
		}
	}
	return 0, 0
}

func getDuplicateErrorFrameRanges(frames []string) []int {
	var result []int
	lineToPositions := map[string][]int{}
	for i, f := range frames {
		lineToPositions[f] = append(lineToPositions[f], i)
	}
	const minimumDuplicateRange = 3
	if len(frames)-len(lineToPositions) <= minimumDuplicateRange {
		return result
	}
	at := func(i int) (string, bool) {
		if i < 0 || i >= len(frames) {
			return "", false
		}
		return frames[i], true
	}
	for i := 0; i < len(frames)-minimumDuplicateRange; i++ {
		positions := lineToPositions[frames[i]]
		if len(positions) == 1 || positions[len(positions)-1] == i {
			continue
		}
		current := indexOfInt(positions, i) + 1
		if current == len(positions) {
			continue
		}
		rng := positions[len(positions)-1] - i
		if rng < minimumDuplicateRange {
			continue
		}
		var extraSteps []int
		haveExtra := false
		if current+1 < len(positions) {
			gcdRange := 0
			seen := map[int]bool{}
			var order []int
			for j := current; j < len(positions); j++ {
				distance := positions[j] - i
				for distance != 0 {
					remainder := gcdRange % distance
					if gcdRange != 0 {
						haveExtra = true
						if !seen[gcdRange] {
							seen[gcdRange] = true
							order = append(order, gcdRange)
						}
					}
					gcdRange = distance
					distance = remainder
				}
				if gcdRange == 1 {
					break
				}
			}
			rng = gcdRange
			if haveExtra {
				for _, v := range order {
					if v != rng {
						extraSteps = append(extraSteps, v)
					}
				}
			}
		}
		maxRange, maxDuplicates, duplicateRanges := rng, 0, 0
		for nextStart := i + rng; ; nextStart += rng {
			equal := 0
			for j := 0; j < rng; j++ {
				a, okA := at(i + j)
				b, okB := at(nextStart + j)
				if okA != okB || a != b {
					break
				}
				equal++
			}
			if equal != rng {
				if len(extraSteps) == 0 {
					break
				}
				if duplicateRanges != 0 && maxRange*maxDuplicates < rng*duplicateRanges {
					maxRange, maxDuplicates = rng, duplicateRanges
				}
				rng = extraSteps[len(extraSteps)-1]
				extraSteps = extraSteps[:len(extraSteps)-1]
				nextStart = i
				duplicateRanges = 0
				continue
			}
			duplicateRanges++
		}
		if maxDuplicates != 0 && maxRange*maxDuplicates >= rng*duplicateRanges {
			rng, duplicateRanges = maxRange, maxDuplicates
		}
		if duplicateRanges*rng >= 3 {
			result = append(result, i+rng, rng, duplicateRanges)
			i += rng*(duplicateRanges+1) - 1
		}
	}
	return result
}

func indexOfInt(s []int, v int) int {
	for i, x := range s {
		if x == v {
			return i
		}
	}
	return -1
}

func markNodeModules(c *ctx, line string) string {
	temp := ""
	lastPos, searchFrom := 0, 0
	for {
		rel := -1
		if searchFrom <= len(line) {
			rel = strings.Index(line[searchFrom:], "node_modules")
		}
		if rel == -1 {
			break
		}
		pos := searchFrom + rel
		var sep, after byte
		if pos > 0 {
			sep = line[pos-1]
		}
		if pos+12 < len(line) {
			after = line[pos+12]
		}
		if (after != '/' && after != '\\') || (sep != '/' && sep != '\\') {
			searchFrom = pos + 1
			continue
		}
		moduleStart := pos + 13
		temp += line[lastPos:moduleStart]
		moduleEnd := indexFrom(line, sep, moduleStart)
		if moduleStart < len(line) && line[moduleStart] == '@' {
			moduleEnd = indexFrom(line, sep, moduleEnd+1)
		}
		end := moduleEnd
		if end == -1 {
			end = len(line) - 1 // JS slice(start, -1)
		}
		if end < moduleStart {
			end = moduleStart
		}
		temp += c.stylize(line[moduleStart:end], "module")
		lastPos = moduleEnd
		searchFrom = moduleEnd
		if moduleEnd == -1 {
			// JS continues from index -1 → 0; avoid an infinite loop by
			// mirroring the observable result: remaining text is appended.
			lastPos = len(line) - 1
			break
		}
	}
	if lastPos != 0 {
		line = temp + line[lastPos:]
	}
	return line
}

func indexFrom(s string, b byte, from int) int {
	if from < 0 {
		from = 0
	}
	if from > len(s) {
		return -1
	}
	i := strings.IndexByte(s[from:], b)
	if i == -1 {
		return -1
	}
	return from + i
}

func markCwd(c *ctx, line, wd string) string {
	cwdStart := strings.Index(line, wd)
	if cwdStart == -1 {
		return line
	}
	cwdLength := len(wd)
	if cwdStart >= 7 && line[cwdStart-7:cwdStart] == "file://" {
		cwdLength += 7
		cwdStart -= 7
	}
	start := cwdStart
	if cwdStart > 0 && line[cwdStart-1] == '(' {
		start = cwdStart - 1
	}
	end := len(line)
	if start != cwdStart && strings.HasSuffix(line, ")") {
		end = -1
	}
	wdEnd := cwdStart + cwdLength + 1
	if wdEnd > len(line) {
		wdEnd = len(line)
	}
	temp := line[:start] + c.stylize(line[start:wdEnd], "undefined")
	if end == -1 {
		if wdEnd <= len(line)-1 {
			temp += line[wdEnd : len(line)-1]
		}
		temp += c.stylize(")", "undefined")
	} else {
		temp += line[wdEnd:end]
	}
	return temp
}
