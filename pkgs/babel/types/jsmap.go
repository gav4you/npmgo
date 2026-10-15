// Copyright (c) 2014-present Sebastian McKenzie and other contributors. MIT License (see LICENSE).
// Written for the Go translation of @babel/types 7.29.0 by the npmgo contributors.

package types

import (
	"math"
	"math/big"
	"reflect"

	"github.com/gav4you/npmgo/internal/jsvalue"
)

// jsMap is a JavaScript Map (or, with values ignored, a Set): keys compared with SameValueZero, iterated in insertion
// order.
type jsMap struct {
	keys  []any
	vals  []any
	index map[any]int
}

type nanKey struct{}

type sliceKey struct {
	typ reflect.Type
	ptr uintptr
	n   int
}

type refKey struct {
	typ reflect.Type
	ptr uintptr
}

// mapKey is a Go map key with SameValueZero equality: numbers by value (NaN equal to NaN, -0 to +0), strings,
// booleans and symbols by value, objects, arrays and functions by identity.
func mapKey(v any) any {
	if jsvalue.IsNumber(v) {
		f := jsvalue.Float(v)
		if math.IsNaN(f) {
			return nanKey{}
		}
		if f == 0 {
			return 0.0
		}
		return f
	}
	switch x := v.(type) {
	case nil, jsvalue.Undefined, bool, string, jsvalue.Symbol, *jsvalue.Object, *jsvalue.Array, *Validator, builtin, *objectPrototypeValue:
		return v
	case *big.Int:
		if x == nil {
			return nil
		}
		return "bigint:" + x.String()
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Slice:
		return sliceKey{rv.Type(), rv.Pointer(), rv.Len()}
	case reflect.Pointer, reflect.Map, reflect.Func, reflect.Chan, reflect.UnsafePointer:
		return refKey{rv.Type(), rv.Pointer()}
	}
	if rv.Type().Comparable() {
		return v
	}
	return refKey{rv.Type(), 0}
}

func newJSMap() *jsMap { return &jsMap{index: map[any]int{}} }

func (m *jsMap) get(k any) (any, bool) {
	if i, ok := m.index[mapKey(k)]; ok {
		return m.vals[i], true
	}
	return jsvalue.Undefined{}, false
}

func (m *jsMap) has(k any) bool {
	_, ok := m.index[mapKey(k)]
	return ok
}

// set is map.set(k, v): an existing key keeps its place.
func (m *jsMap) set(k, v any) {
	key := mapKey(k)
	if i, ok := m.index[key]; ok {
		m.vals[i] = v
		return
	}
	m.index[key] = len(m.keys)
	m.keys = append(m.keys, k)
	m.vals = append(m.vals, v)
}
