// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
//
// Translated to Go by the npmgo contributors from @opentelemetry/core 2.11.0 (build/src/platform/node/environment.js,
// utils/configuration.js, utils/url.js and common/timer-util.js).

package core

import (
	"os"
	"regexp"
	"strings"

	"github.com/gav4you/npmgo/internal/jsvalue"
	"github.com/gav4you/npmgo/internal/nodeutil"
	"github.com/gav4you/npmgo/pkgs/opentelemetry/api"
)

// ProcessEnv looks a variable up in the environment, as process.env[key] does: os.LookupEnv (case-insensitive on
// Windows, like Node's process.env there). Tests and embedders may replace it before use.
var ProcessEnv func(key string) (string, bool) = os.LookupEnv

// envValue is process.env[key]: the variable, or, for a key named like an Object.prototype property (process.env
// inherits them), the inherited function or object, or undefined.
func envValue(key string) any {
	if v, ok := ProcessEnv(key); ok {
		return v
	}
	if objectPrototypeMethods[key] || key == "__proto__" {
		return builtinFunction{key}
	}
	return jsvalue.Undefined{}
}

// envString is process.env[key] read as JavaScript reads `raw.trim()`: a string, or undefined; an inherited value has
// no trim method (TypeError, with V8's message for the expression).
func envString(key, expr string) (string, bool) {
	switch v := envValue(key).(type) {
	case string:
		return v, true
	case jsvalue.Undefined:
		return "", false
	}
	panic(&jsvalue.TypeError{Message: expr + " is not a function"})
}

// GetNumberFromEnv returns the variable as a number (Number(raw)); an unset, empty or whitespace-only variable is
// undefined, and so is one that is not a number, which is logged as a warning to api.Diag.
func GetNumberFromEnv(key string) (float64, bool) {
	raw, ok := envString(key, "raw.trim")
	if !ok || jsvalue.Trim(raw) == "" {
		return 0, false
	}
	value := jsvalue.StringToNumber(raw)
	if value != value { // isNaN
		api.Diag.Warn("Unknown value " + nodeutil.Inspect(raw) + " for " + key + ", expected a number, using defaults")
		return 0, false
	}
	return value, true
}

// GetStringFromEnv returns the variable; an unset, empty or whitespace-only variable is undefined. The value is not
// trimmed.
func GetStringFromEnv(key string) (string, bool) {
	raw, ok := envString(key, "raw.trim")
	if !ok || jsvalue.Trim(raw) == "" {
		return "", false
	}
	return raw, true
}

// GetBooleanFromEnv returns whether the variable, trimmed and lower-cased, is "true"; anything but "true", "false"
// and an unset or blank variable is logged as a warning to api.Diag.
func GetBooleanFromEnv(key string) bool {
	raw, ok := envString(key, "process.env[key]?.trim")
	if !ok {
		return false
	}
	raw = jsvalue.ToLowerCase(jsvalue.Trim(raw))
	switch raw {
	case "":
		return false
	case "true":
		return true
	case "false":
		return false
	}
	api.Diag.Warn("Unknown value " + nodeutil.Inspect(raw) + " for " + key + ", expected 'true' or 'false', falling back to 'false' (default)")
	return false
}

// GetStringListFromEnv returns the variable split at commas, each entry trimmed and the empty ones left out (so an
// all-blank list is empty); an unset, empty or whitespace-only variable is undefined.
func GetStringListFromEnv(key string) ([]string, bool) {
	s, ok := GetStringFromEnv(key)
	if !ok {
		return nil, false
	}
	out := []string{}
	for _, v := range strings.Split(s, ",") {
		if t := jsvalue.Trim(v); t != "" {
			out = append(out, t)
		}
	}
	return out, true
}

// logLevelMap is configuration.js's logLevelMap, in its key order.
var logLevelMap = []struct {
	name  string
	level api.DiagLogLevel
}{
	{"ALL", api.DiagLogLevel_ALL},
	{"VERBOSE", api.DiagLogLevel_VERBOSE},
	{"DEBUG", api.DiagLogLevel_DEBUG},
	{"INFO", api.DiagLogLevel_INFO},
	{"WARN", api.DiagLogLevel_WARN},
	{"ERROR", api.DiagLogLevel_ERROR},
	{"NONE", api.DiagLogLevel_NONE},
}

// DiagLogLevelFromString converts a log level name (in any case: value.toUpperCase()) to a DiagLogLevel. nil
// (undefined or null) gives undefined (false); an unknown name is logged as a warning to api.Diag and gives INFO.
func DiagLogLevelFromString(value *string) (api.DiagLogLevel, bool) {
	if value == nil {
		return 0, false
	}
	upper := jsvalue.ToUpperCase(*value)
	for _, l := range logLevelMap {
		if l.name == upper {
			return l.level, true
		}
	}
	names := make([]string, len(logLevelMap))
	for i, l := range logLevelMap {
		names[i] = l.name
	}
	api.Diag.Warn(`Unknown log level "` + *value + `", expected one of ` + strings.Join(names, ",") + `, using default`)
	return api.DiagLogLevel_INFO, true
}

// UrlMatches reports whether url is urlToMatch (a string) or matches it (a *regexp.Regexp, Go's regular expression
// syntax standing in for a JavaScript RegExp). Any other value is turned into a pattern as new RegExp(value) would
// be: undefined is the empty pattern, anything else its string (null is "null").
func UrlMatches(url string, urlToMatch any) bool {
	switch m := urlToMatch.(type) {
	case string:
		return url == m
	case *regexp.Regexp:
		return m.MatchString(url)
	case jsvalue.Undefined:
		return true
	}
	return regexp.MustCompile(jsvalue.ToString(urlToMatch)).MatchString(url)
}

// IsUrlIgnored reports whether url matches one of ignoredUrls (see UrlMatches); nil ignores nothing.
func IsUrlIgnored(url string, ignoredUrls []any) bool {
	for _, ignoreUrl := range ignoredUrls {
		if UrlMatches(url, ignoreUrl) {
			return true
		}
	}
	return false
}

// Unrefer is a timer that can be unreferenced (a Node Timeout).
type Unrefer interface {
	Unref()
}

// UnrefTimer calls timer.unref() unless timer is a number (a browser timer id). Deprecated in the original. A timer
// without an Unref method panics as JavaScript throws (a TypeError).
func UnrefTimer(timer any) {
	if jsvalue.IsNumber(timer) {
		return
	}
	switch t := timer.(type) {
	case nil:
		panic(&jsvalue.TypeError{Message: "Cannot read properties of null (reading 'unref')"})
	case jsvalue.Undefined:
		panic(&jsvalue.TypeError{Message: "Cannot read properties of undefined (reading 'unref')"})
	case Unrefer:
		t.Unref()
		return
	}
	panic(&jsvalue.TypeError{Message: "timer.unref is not a function"})
}
