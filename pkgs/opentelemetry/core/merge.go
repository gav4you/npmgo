// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
//
// Translated to Go by the npmgo contributors from @opentelemetry/core 2.11.0 (build/src/utils/merge.js and
// utils/lodash.merge.js).
//
// isPlainObject is translated from utils/lodash.merge.js, which the original describes as "based on lodash in order
// to support esm builds without esModuleInterop. lodash is using MIT License." lodash (4.17.21) carries this notice:
//
//	Copyright OpenJS Foundation and other contributors <https://openjsf.org/>
//
//	Based on Underscore.js, copyright Jeremy Ashkenas,
//	DocumentCloud and Investigative Reporters & Editors <http://underscorejs.org/>
//
//	This software consists of voluntary contributions made by many
//	individuals. For exact contribution history, see the revision history
//	available at https://github.com/lodash/lodash
//
//	Permission is hereby granted, free of charge, to any person obtaining
//	a copy of this software and associated documentation files (the
//	"Software"), to deal in the Software without restriction, including
//	without limitation the rights to use, copy, modify, merge, publish,
//	distribute, sublicense, and/or sell copies of the Software, and to
//	permit persons to whom the Software is furnished to do so, subject to
//	the following conditions:
//
//	The above copyright notice and this permission notice shall be
//	included in all copies or substantial portions of the Software.
//
//	THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
//	EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
//	MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND
//	NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE
//	LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION
//	OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION
//	WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

package core

import (
	"reflect"
	"regexp"
	"time"

	"github.com/gav4you/npmgo/internal/jsvalue"
)

const maxMergeLevel = 20

// Merge deep-merges its arguments from left to right and returns the result (undefined without arguments; the first
// argument itself when it is the only one). In the jsvalue model: plain objects are *jsvalue.Object, arrays are Go
// slices (results are []any), functions are Go funcs, a Date is a time.Time and a RegExp a *regexp.Regexp; any other
// Go value is an object that is not plain (a class instance).
//
// As in JavaScript: a primitive, Date, RegExp or function on either side gives the right-hand value (arrays copied);
// an array on the left is copied and the right-hand array appended (or the right-hand object's array-index
// properties assigned); two plain objects merge key by key (undefined deletes the key; __proto__, constructor and
// prototype are skipped; a key whose value was already merged at the same place of an object being merged, a cycle,
// is deleted) and anything else gives the right-hand value itself. Nesting deeper than 20 levels gives undefined.
// Like the original, a key of the right-hand object named after an Object.prototype method (toString, valueOf, …)
// whose value is an object or array merges with the inherited method and ends up undefined.
//
// Not modelled: named (non-index) properties of arrays, which merging a plain object into an array assigns, and the
// prototype that copying an own __proto__ property sets.
func Merge(args ...any) any {
	if len(args) == 0 {
		return jsvalue.Undefined{}
	}
	result := args[0]
	objects := &weakRefs{m: map[any][]mergeRef{}}
	for _, a := range args[1:] {
		result = mergeTwoObjects(result, a, 0, objects)
	}
	return result
}

// mergeRef is one {obj, key} record of the WeakMap merge keeps.
type mergeRef struct {
	obj any
	key string
}

// weakRefs is the WeakMap from an object to where it was met.
type weakRefs struct {
	m map[any][]mergeRef
}

// identity is a map key standing for a value's object identity (nil for values that are not objects).
func identity(v any) any {
	if v == nil {
		return nil
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Pointer, reflect.Map, reflect.Slice, reflect.Func, reflect.Chan, reflect.UnsafePointer:
		return struct {
			t reflect.Type
			p uintptr
		}{rv.Type(), rv.Pointer()}
	}
	if rv.Type().Comparable() {
		return v
	}
	return nil
}

func (w *weakRefs) get(v any) []mergeRef {
	if id := identity(v); id != nil {
		return w.m[id]
	}
	return nil
}

func (w *weakRefs) add(v any, r mergeRef) {
	if id := identity(v); id != nil {
		w.m[id] = append(w.m[id], r)
	}
}

// objectPrototypeMethods are the Object.prototype properties whose values are functions: what obj[key] reads on a
// plain object without an own key of that name.
var objectPrototypeMethods = map[string]bool{
	"constructor": true, "__defineGetter__": true, "__defineSetter__": true, "hasOwnProperty": true,
	"__lookupGetter__": true, "__lookupSetter__": true, "isPrototypeOf": true, "propertyIsEnumerable": true,
	"toString": true, "valueOf": true, "toLocaleString": true,
}

// builtinFunction stands for a built-in function read through the prototype chain.
type builtinFunction struct{ name string }

func (builtinFunction) JSTypeOf() string { return "function" }

// get is obj[key] for a plain object: an own property, an inherited Object.prototype method, or undefined.
func get(obj *jsvalue.Object, key string) any {
	if v, ok := obj.Lookup(key); ok {
		return v
	}
	if objectPrototypeMethods[key] {
		return builtinFunction{key}
	}
	return jsvalue.Undefined{}
}

// getAny is obj[key] where obj is one of merge's operands.
func getAny(obj any, key string) any {
	if o, ok := obj.(*jsvalue.Object); ok && o != nil {
		return get(o, key)
	}
	return jsvalue.Undefined{}
}

func takeValue(value any) any {
	if isArray(value) {
		return append([]any{}, jsvalue.Items(value)...)
	}
	return value
}

func mergeTwoObjects(one, two any, level int, objects *weakRefs) any {
	var result any = jsvalue.Undefined{}
	if level > maxMergeLevel {
		return jsvalue.Undefined{}
	}
	level++
	switch {
	case isPrimitive(one) || isPrimitive(two) || isFunction(two):
		result = takeValue(two)
	case isArray(one):
		r := append([]any{}, jsvalue.Items(one)...)
		if isArray(two) {
			for _, v := range jsvalue.Items(two) {
				r = append(r, takeValue(v))
			}
		} else if isObject(two) {
			if o, ok := two.(*jsvalue.Object); ok {
				for _, key := range o.Keys() {
					if key == "__proto__" || key == "constructor" || key == "prototype" {
						continue
					}
					// result[key] = takeValue(two[key]); only array-index keys are modelled.
					if idx, ok := jsvalue.ArrayIndex(key); ok && idx < 1<<31 {
						for uint64(len(r)) <= idx {
							r = append(r, jsvalue.Undefined{})
						}
						r[idx] = takeValue(o.Get(key))
					}
				}
			}
		}
		result = r
	case isObject(one):
		if !isObject(two) {
			result = two
			break
		}
		if !shouldMerge(one, two) {
			return two
		}
		oneObj, twoObj := one.(*jsvalue.Object), two.(*jsvalue.Object)
		res := &jsvalue.Object{} // Object.assign({}, one)
		oneObj.Range(func(k string, v any) bool { res.Put(k, v); return true })
		for _, key := range twoObj.Keys() {
			if key == "__proto__" || key == "constructor" || key == "prototype" {
				continue
			}
			twoValue := twoObj.Get(key)
			if isPrimitive(twoValue) {
				if jsvalue.IsUndefined(twoValue) {
					res.Delete(key)
				} else {
					res.Put(key, twoValue)
				}
				continue
			}
			obj1 := get(res, key)
			obj2 := twoValue
			if wasObjectReferenced(one, key, objects) || wasObjectReferenced(two, key, objects) {
				res.Delete(key)
				continue
			}
			if isObject(obj1) && isObject(obj2) {
				objects.add(obj1, mergeRef{one, key})
				objects.add(obj2, mergeRef{two, key})
			}
			res.Put(key, mergeTwoObjects(get(res, key), twoValue, level, objects))
		}
		result = res
	}
	return result
}

// wasObjectReferenced reports whether obj[key] was already met as obj's key.
func wasObjectReferenced(obj any, key string, objects *weakRefs) bool {
	for _, info := range objects.get(getAny(obj, key)) {
		if info.key == key && identity(info.obj) == identity(obj) {
			return true
		}
	}
	return false
}

func isArray(value any) bool { return jsvalue.IsArray(value) }

func isFunction(value any) bool { return jsvalue.TypeOf(value) == "function" }

func isObject(value any) bool {
	return !isPrimitive(value) && !isArray(value) && !isFunction(value) && jsvalue.TypeOf(value) == "object"
}

// isPrimitive is merge.js's isPrimitive: strings, numbers, booleans, undefined and null, and Dates and RegExps.
func isPrimitive(value any) bool {
	switch value.(type) {
	case string, bool, jsvalue.Undefined, time.Time, *time.Time, *regexp.Regexp:
		return true
	}
	return jsvalue.IsNumber(value) || jsvalue.IsNull(value)
}

func shouldMerge(one, two any) bool { return isPlainObject(one) && isPlainObject(two) }

// isPlainObject is lodash's isPlainObject: an object made by the Object constructor or with a null prototype. In the
// jsvalue model those are the *jsvalue.Object values.
func isPlainObject(value any) bool {
	o, ok := value.(*jsvalue.Object)
	return ok && o != nil
}
