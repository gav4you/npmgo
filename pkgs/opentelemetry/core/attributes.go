// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
//
// Translated to Go by the npmgo contributors from @opentelemetry/core 2.11.0 (build/src/common/attributes.js and
// common/anchored-clock.js).

package core

import (
	"sort"
	"strconv"

	"github.com/gav4you/npmgo/internal/jsvalue"
	"github.com/gav4you/npmgo/pkgs/opentelemetry/api"
)

// SanitizeAttributes returns a new attributes object holding the valid attributes of attributes, in order: keys must
// be non-empty strings and values attribute values (see IsAttributeValue); arrays are copied. Each invalid key or
// value is logged as a warning to api.Diag. Anything but an object gives an empty object.
//
// attributes is a *jsvalue.Object (its own properties), a Go slice (an array: its indices) or a Go map with string
// keys (visited as Object.keys would list them; see api.DefaultTextMapGetter); any other object has no properties.
func SanitizeAttributes(attributes any) *jsvalue.Object {
	out := &jsvalue.Object{}
	if jsvalue.TypeOf(attributes) != "object" || jsvalue.IsNull(attributes) {
		return out
	}
	visit := func(key string, val any) {
		if !isAttributeKey(key) {
			api.Diag.Warn("Invalid attribute key: " + key)
			return
		}
		if !IsAttributeValue(val) {
			api.Diag.Warn("Invalid attribute value set for key: " + key)
			return
		}
		if jsvalue.IsArray(val) {
			out.Put(key, append([]any{}, jsvalue.Items(val)...))
		} else {
			out.Put(key, val)
		}
	}
	switch a := attributes.(type) {
	case *jsvalue.Object:
		a.Range(func(k string, v any) bool { visit(k, v); return true })
	case map[string]any:
		for _, k := range sortedKeys(a) {
			visit(k, a[k])
		}
	case map[string]string:
		for _, k := range sortedKeys(a) {
			visit(k, a[k])
		}
	default:
		if jsvalue.IsArray(attributes) {
			for i, v := range jsvalue.Items(attributes) {
				visit(strconv.Itoa(i), v)
			}
		}
	}
	return out
}

// sortedKeys lists a Go map's keys as Object.keys lists an object's: array-index keys ascending, then the others (Go
// keeps no insertion order; they are sorted by UTF-16 code units).
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, aIdx := jsvalue.ArrayIndex(keys[i])
		b, bIdx := jsvalue.ArrayIndex(keys[j])
		switch {
		case aIdx && bIdx:
			return a < b
		case aIdx != bIdx:
			return aIdx
		}
		return less16(keys[i], keys[j])
	})
	return keys
}

// less16 orders strings by UTF-16 code units, as JavaScript compares them.
func less16(a, b string) bool {
	ra, rb := []rune(a), []rune(b)
	for i := 0; i < len(ra) && i < len(rb); i++ {
		if ra[i] != rb[i] {
			return unit16(ra[i]) < unit16(rb[i])
		}
	}
	return len(ra) < len(rb)
}

// unit16 is the order of a code point's first UTF-16 code unit (a high surrogate for astral code points).
func unit16(r rune) float64 {
	if r > 0xFFFF {
		return 0xD800 + float64(r-0x10000)/0x400
	}
	return float64(r)
}

// isAttributeKey reports whether key is a non-empty string.
func isAttributeKey(key string) bool { return key != "" }

// IsAttributeValue reports whether val is an attribute value: null or undefined, a string, number or boolean, or an
// array whose elements, null and undefined aside, are all strings, all numbers or all booleans.
func IsAttributeValue(val any) bool {
	if jsvalue.IsNullish(val) {
		return true
	}
	if jsvalue.IsArray(val) {
		return isHomogeneousAttributeValueArray(jsvalue.Items(val))
	}
	return isValidPrimitiveAttributeValueType(jsvalue.TypeOf(val))
}

func isHomogeneousAttributeValueArray(arr []any) bool {
	typ := ""
	for _, element := range arr {
		if jsvalue.IsNullish(element) {
			continue
		}
		elementType := jsvalue.TypeOf(element)
		if elementType == typ {
			continue
		}
		if typ == "" {
			if isValidPrimitiveAttributeValueType(elementType) {
				typ = elementType
				continue
			}
			return false
		}
		return false
	}
	return true
}

func isValidPrimitiveAttributeValueType(valType string) bool {
	switch valType {
	case "number", "boolean", "string":
		return true
	}
	return false
}

// Clock is a source of milliseconds (Date, performance).
type Clock interface {
	Now() float64
}

// AnchoredClock returns wall times anchored at a point in time: the system clock's time when it was created plus
// the time a monotonic clock has advanced since, so that times taken from it never go backwards when the system
// clock is adjusted.
type AnchoredClock struct {
	monotonicClock    Clock
	epochMillis       float64
	performanceMillis float64
}

// NewAnchoredClock is new AnchoredClock(systemClock, monotonicClock): it reads the system clock, then the monotonic
// clock.
func NewAnchoredClock(systemClock, monotonicClock Clock) *AnchoredClock {
	c := &AnchoredClock{monotonicClock: monotonicClock}
	c.epochMillis = systemClock.Now()
	c.performanceMillis = monotonicClock.Now()
	return c
}

// Now returns the anchor's epoch time plus the monotonic clock's advance since the anchor.
func (c *AnchoredClock) Now() float64 {
	delta := c.monotonicClock.Now() - c.performanceMillis
	return c.epochMillis + delta
}
