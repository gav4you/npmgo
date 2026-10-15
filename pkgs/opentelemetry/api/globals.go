// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
//
// Translated to Go by the npmgo contributors from @opentelemetry/api 1.9.0 (build/src/internal/global-utils.js and
// internal/semver.js).

package api

import (
	"regexp"
	"sync"

	"github.com/gav4you/npmgo/internal/jsnum"
	"github.com/gav4you/npmgo/internal/jsvalue"
)

// globalRecord is the object JavaScript keeps at globalThis[Symbol.for('opentelemetry.js.api.1')], which every copy of
// the API in a process shares: the version of the copy that created it and the registered globals by type (trace,
// context, propagation, diag, metrics). Go has one copy of the package, so the record is package state; a record of
// another version can only be made by this package's tests.
type globalRecord struct {
	version string
	entries map[string]any
}

var (
	globalMu sync.RWMutex
	global   *globalRecord // nil until the first registration
)

// registerGlobal stores instance as the global of its type and reports whether it did. Without allowOverride an
// existing global of the type is kept; a record of another API version refuses every registration. Both refusals are
// logged to diag as errors, a registration as a debug message.
//
// JavaScript logs err.stack (the message and the V8 stack frames); Go logs the first line, "Error: <message>".
func registerGlobal(typ string, instance any, diag DiagLogger, allowOverride bool) bool {
	globalMu.Lock()
	if global == nil {
		global = &globalRecord{version: VERSION, entries: map[string]any{}}
	}
	if !allowOverride && jsvalue.Truthy(global.entries[typ]) {
		globalMu.Unlock()
		diag.Error("Error: @opentelemetry/api: Attempted duplicate registration of API: " + typ)
		return false
	}
	if global.version != VERSION {
		version := global.version
		globalMu.Unlock()
		diag.Error("Error: @opentelemetry/api: Registration of version v" + version + " for " + typ +
			" does not match previously registered API v" + VERSION)
		return false
	}
	global.entries[typ] = instance
	globalMu.Unlock()
	diag.Debug("@opentelemetry/api: Registered a global for " + typ + " v" + VERSION + ".")
	return true
}

// getGlobal returns the global of the type, or nil (undefined) when there is none or the record's version is not
// compatible with this API.
func getGlobal(typ string) any {
	globalMu.RLock()
	defer globalMu.RUnlock()
	if global == nil || global.version == "" || !isCompatible(global.version) {
		return nil
	}
	return global.entries[typ]
}

// unregisterGlobal removes the global of the type.
func unregisterGlobal(typ string, diag DiagLogger) {
	diag.Debug("@opentelemetry/api: Unregistering a global for " + typ + " v" + VERSION + ".")
	globalMu.Lock()
	defer globalMu.Unlock()
	if global != nil {
		delete(global.entries, typ)
	}
}

// semverRe is /^(\d+)\.(\d+)\.(\d+)(-(.+))?$/ (JavaScript's . matches no line terminator).
var semverRe = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)(-([^\n\r\x{2028}\x{2029}]+))?$`)

type parsedVersion struct {
	major, minor, patch float64
	prerelease          bool
}

func parseVersion(v string) (parsedVersion, bool) {
	m := semverRe.FindStringSubmatch(v)
	if m == nil {
		return parsedVersion{}, false
	}
	return parsedVersion{jsnum.Number(m[1]), jsnum.Number(m[2]), jsnum.Number(m[3]), m[4] != ""}, true
}

// makeCompatibilityCheck is _makeCompatibilityCheck: a test of whether a global API of the given version can serve an
// API of ownVersion. An exact match is compatible; otherwise neither may be a prerelease, the majors must match and
// ownVersion's minor must not be above the global's (for major 0: the minors must match and ownVersion's patch must
// not be above the global's). Versions that do not parse are incompatible. Results are cached, as in JavaScript.
func makeCompatibilityCheck(ownVersion string) func(globalVersion string) bool {
	var mu sync.Mutex
	accepted := map[string]bool{ownVersion: true}
	rejected := map[string]bool{}
	own, ok := parseVersion(ownVersion)
	if !ok {
		return func(string) bool { return false }
	}
	if own.prerelease {
		return func(globalVersion string) bool { return globalVersion == ownVersion }
	}
	return func(globalVersion string) bool {
		mu.Lock()
		defer mu.Unlock()
		if accepted[globalVersion] {
			return true
		}
		if rejected[globalVersion] {
			return false
		}
		reject := func() bool { rejected[globalVersion] = true; return false }
		accept := func() bool { accepted[globalVersion] = true; return true }
		g, ok := parseVersion(globalVersion)
		if !ok || g.prerelease || own.major != g.major {
			return reject()
		}
		if own.major == 0 {
			if own.minor == g.minor && own.patch <= g.patch {
				return accept()
			}
			return reject()
		}
		if own.minor <= g.minor {
			return accept()
		}
		return reject()
	}
}

// isCompatible tests a global API version against this API's (semver.js's isCompatible).
var isCompatible = makeCompatibilityCheck(VERSION)
