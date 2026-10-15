// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
//
// Translated to Go by the npmgo contributors from @opentelemetry/semantic-conventions 1.40.0
// (build/src/index.js and build/src/internal/utils.js).

// Package semanticconventions is a translation of @opentelemetry/semantic-conventions 1.40.0's main entry point,
// require('@opentelemetry/semantic-conventions'): the stable attribute (ATTR_*), metric (METRIC_*) and event
// (EVENT_*) names and their enum values, and the deprecated SEMATTRS_* / SEMRESATTRS_* constants with their namespace
// objects (SemanticAttributes, DbSystemValues, …). The incubating entry point is package incubating.
//
// The declarations are generated from the original module by tools/gen_semconv.cjs, under the JavaScript names; the
// documentation is the original's. Template functions such as ATTR_HTTP_REQUEST_HEADER are Go functions of a string,
// and the namespace objects are *ConstMap values.
package semanticconventions

import (
	"strings"

	"github.com/gav4you/npmgo/internal/jsvalue"
)

// ConstMap is one of the deprecated namespace objects (SemanticAttributes, DbSystemValues, …): a JavaScript object of
// constants, its keys in JavaScript property order. It is read-only.
type ConstMap[V string | float64] struct {
	obj *jsvalue.Object
}

// Get returns obj[key] for an own property key.
func (m *ConstMap[V]) Get(key string) (value V, ok bool) {
	v, ok := m.obj.Lookup(key)
	if !ok {
		return value, false
	}
	return v.(V), true
}

// Keys returns the keys in order (Object.keys).
func (m *ConstMap[V]) Keys() []string { return m.obj.Keys() }

// Len is the number of keys.
func (m *ConstMap[V]) Len() int { return m.obj.Len() }

// newConstMap builds an object literal from alternating keys and values.
func newConstMap[V string | float64](kv ...any) *ConstMap[V] {
	m := &ConstMap[V]{obj: &jsvalue.Object{}}
	for i := 0; i+1 < len(kv); i += 2 {
		var v V
		switch x := kv[i+1].(type) {
		case string:
			v = any(x).(V)
		case int:
			v = any(float64(x)).(V)
		}
		m.obj.Set(kv[i].(string), v)
	}
	return m
}

var keyReplacer = strings.NewReplacer("-", "_", ".", "_")

// createConstMap is internal/utils.js's createConstMap: each truthy value under
// String(val).toUpperCase().replace(/[-.]/g, '_') (a later value with the same key replaces the earlier one in its
// place).
func createConstMap(values ...string) *ConstMap[string] {
	m := &ConstMap[string]{obj: &jsvalue.Object{}}
	for _, val := range values {
		if val != "" {
			m.obj.Set(keyReplacer.Replace(jsvalue.ToUpperCase(val)), val)
		}
	}
	return m
}
