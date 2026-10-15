// Copyright (c) 2014-present Sebastian McKenzie and other contributors. MIT License (see LICENSE).
// Translated to Go by the npmgo contributors from @babel/types 7.29.0 (lib/utils/deprecationWarning.js).

package types

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
)

// ConsoleWarn is console.warn(message), where deprecation warnings and the BABEL_TYPES_8_BREAKING notice go: the
// message and a newline on standard error. Replace it to capture them (before the package is used from several
// goroutines).
var ConsoleWarn = func(message string) { fmt.Fprintln(os.Stderr, message) }

var warnings struct {
	sync.Mutex
	seen map[string]bool
}

// babelPackages is the Go counterpart of the /@babel/ in the file name deprecationWarning tests: the packages
// translated from @babel.
const babelPackages = "github.com/gav4you/npmgo/pkgs/babel/"

// InternalDeprecationWarning is __internal__deprecationWarning(oldName, newName, prefix = "", cacheKey = oldName):
// the first time for a cacheKey, it warns that oldName is deprecated in favour of newName, with two lines of stack
// (the caller and its caller), unless that caller is itself in a Babel package. Pass prefix, then cacheKey.
func InternalDeprecationWarning(oldName, newName string, prefixAndCacheKey ...string) {
	warnDeprecated(oldName, newName, prefixAndCacheKey)
}

func deprecationWarning(oldName, newName string, prefixAndCacheKey ...string) {
	warnDeprecated(oldName, newName, prefixAndCacheKey)
}

func warnDeprecated(oldName, newName string, opt []string) {
	prefix, cacheKey := "", oldName
	if len(opt) > 0 {
		prefix = opt[0]
	}
	if len(opt) > 1 {
		cacheKey = opt[1]
	}
	warnings.Lock()
	if warnings.seen == nil {
		warnings.seen = map[string]bool{}
	}
	if warnings.seen[cacheKey] {
		warnings.Unlock()
		return
	}
	warnings.seen[cacheKey] = true
	warnings.Unlock()
	internal, trace := captureShortStackTrace(2)
	if internal {
		return
	}
	ConsoleWarn(prefix + "`" + oldName + "` has been deprecated, please migrate to `" + newName + "`\n" + trace)
}

// captureShortStackTrace returns the frames of the deprecated function (the caller of deprecationWarning) and of its
// caller, as JavaScript's captureShortStackTrace(1, 2) does, and whether that caller is in a Babel package.
func captureShortStackTrace(length int) (internal bool, trace string) {
	pcs := make([]uintptr, 32)
	n := runtime.Callers(1, pcs)
	frames := runtime.CallersFrames(pcs[:n])
	var all []runtime.Frame
	for {
		f, more := frames.Next()
		all = append(all, f)
		if !more {
			break
		}
	}
	// all[0] is captureShortStackTrace, all[1] warnDeprecated, all[2] (Internal)deprecationWarning.
	const skip = 3
	if len(all) < skip+length {
		return false, ""
	}
	short := all[skip : skip+length]
	lines := make([]string, len(short))
	for i, f := range short {
		lines[i] = fmt.Sprintf("    at %s (%s:%d)", f.Function, f.File, f.Line)
	}
	return strings.HasPrefix(short[1].Function, babelPackages), strings.Join(lines, "\n")
}

// resetDeprecationWarnings forgets the warnings given (for tests, which replay the original's sequence).
func resetDeprecationWarnings() {
	warnings.Lock()
	warnings.seen = nil
	warnings.Unlock()
}
