// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
//
// Translated to Go by the npmgo contributors from @opentelemetry/api 1.9.0 (build/src/propagation/TextMapPropagator.js,
// propagation/NoopTextMapPropagator.js and api/propagation.js).

package api

import (
	"sort"

	"github.com/gav4you/npmgo/internal/jsvalue"
)

// TextMapGetter reads a carrier (such as incoming HTTP headers).
type TextMapGetter interface {
	// Keys returns the carrier's keys.
	Keys(carrier any) []string
	// Get returns the value under key: undefined (jsvalue.Undefined{}), a string or a list of strings, or whatever
	// else the carrier holds.
	Get(carrier any, key string) any
}

// TextMapSetter writes a carrier (such as outgoing HTTP headers).
type TextMapSetter interface {
	Set(carrier any, key string, value string)
}

// TextMapPropagator injects values from a context into a carrier and extracts them from one.
type TextMapPropagator interface {
	// Inject writes the propagated values of context into carrier through setter.
	Inject(context Context, carrier any, setter TextMapSetter)
	// Extract returns context with the values read from carrier through getter.
	Extract(context Context, carrier any, getter TextMapGetter) Context
	// Fields returns the keys the propagator uses.
	Fields() []string
}

// DefaultTextMapGetter reads carrier[key]: it supports *jsvalue.Object, map[string]string, map[string][]string and
// map[string]any carriers (anything else has no keys, nil is null). JavaScript's carrier[key] also finds inherited
// properties (carrier['toString']); Go carriers have none.
var DefaultTextMapGetter TextMapGetter = defaultTextMapGetter{}

// DefaultTextMapSetter assigns carrier[key] = value on the carriers DefaultTextMapGetter reads (a map[string][]string
// gets a one-element list); other carriers, and nil (null), are left alone.
var DefaultTextMapSetter TextMapSetter = defaultTextMapSetter{}

type defaultTextMapGetter struct{}

func (defaultTextMapGetter) Get(carrier any, key string) any {
	if jsvalue.IsNullish(carrier) {
		return jsvalue.Undefined{}
	}
	switch c := carrier.(type) {
	case *jsvalue.Object:
		return c.Get(key)
	case map[string]string:
		if v, ok := c[key]; ok {
			return v
		}
	case map[string][]string:
		if v, ok := c[key]; ok {
			return v
		}
	case map[string]any:
		if v, ok := c[key]; ok {
			return v
		}
	}
	return jsvalue.Undefined{}
}

func (defaultTextMapGetter) Keys(carrier any) []string {
	if jsvalue.IsNullish(carrier) {
		return []string{}
	}
	switch c := carrier.(type) {
	case *jsvalue.Object:
		return c.Keys()
	case map[string]string:
		return mapKeys(c)
	case map[string][]string:
		return mapKeys(c)
	case map[string]any:
		return mapKeys(c)
	}
	return []string{}
}

// mapKeys lists a Go map's keys as Object.keys would list those of an object with the same properties, assigned in
// an order Go does not keep: array-index keys ascending, then the others sorted by UTF-16 code units.
func mapKeys[V any](m map[string]V) []string {
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
		return lessUTF16(keys[i], keys[j])
	})
	return keys
}

// lessUTF16 orders strings by their UTF-16 code units, as JavaScript compares strings.
func lessUTF16(a, b string) bool {
	ra, rb := []rune(a), []rune(b)
	for i := 0; i < len(ra) && i < len(rb); i++ {
		if ra[i] != rb[i] {
			return utf16Key(ra[i]) < utf16Key(rb[i])
		}
	}
	return len(ra) < len(rb)
}

// utf16Key maps a code point to the order of its first UTF-16 code unit (astral code points start with a high
// surrogate, D800-DBFF, below U+E000).
func utf16Key(r rune) float64 {
	if r > 0xFFFF {
		return 0xD800 + float64(r-0x10000)/0x400
	}
	return float64(r)
}

type defaultTextMapSetter struct{}

func (defaultTextMapSetter) Set(carrier any, key string, value string) {
	if jsvalue.IsNullish(carrier) {
		return
	}
	switch c := carrier.(type) {
	case *jsvalue.Object:
		c.Put(key, value)
	case map[string]string:
		if key != "__proto__" {
			c[key] = value
		}
	case map[string][]string:
		if key != "__proto__" {
			c[key] = []string{value}
		}
	case map[string]any:
		if key != "__proto__" {
			c[key] = value
		}
	}
}

// NoopTextMapPropagator injects nothing and extracts nothing.
type NoopTextMapPropagator struct{}

func (NoopTextMapPropagator) Inject(Context, any, TextMapSetter) {}

func (NoopTextMapPropagator) Extract(context Context, _ any, _ TextMapGetter) Context { return context }

func (NoopTextMapPropagator) Fields() []string { return []string{} }

var noopTextMapPropagator TextMapPropagator = NoopTextMapPropagator{}

const propagationAPIName = "propagation"

// PropagationAPI is the propagation API (JavaScript's PropagationAPI class; its instance is Propagation).
type PropagationAPI struct{}

// SetGlobalPropagator registers the propagator and reports whether it was registered.
func (p *PropagationAPI) SetGlobalPropagator(propagator TextMapPropagator) bool {
	return registerGlobal(propagationAPIName, propagator, Diag, false)
}

// Inject injects context into carrier with the global propagator; a nil setter is DefaultTextMapSetter.
func (p *PropagationAPI) Inject(context Context, carrier any, setter TextMapSetter) {
	if setter == nil {
		setter = DefaultTextMapSetter
	}
	p.globalPropagator().Inject(context, carrier, setter)
}

// Extract extracts a context from carrier with the global propagator; a nil getter is DefaultTextMapGetter.
func (p *PropagationAPI) Extract(context Context, carrier any, getter TextMapGetter) Context {
	if getter == nil {
		getter = DefaultTextMapGetter
	}
	return p.globalPropagator().Extract(context, carrier, getter)
}

// Fields returns the fields of the global propagator.
func (p *PropagationAPI) Fields() []string { return p.globalPropagator().Fields() }

// Disable removes the global propagator.
func (p *PropagationAPI) Disable() { unregisterGlobal(propagationAPIName, Diag) }

func (p *PropagationAPI) globalPropagator() TextMapPropagator {
	if g, ok := getGlobal(propagationAPIName).(TextMapPropagator); ok && !jsvalue.IsNull(g) {
		return g
	}
	return noopTextMapPropagator
}

// CreateBaggage is propagation.createBaggage (see CreateBaggage).
func (p *PropagationAPI) CreateBaggage(entries ...BaggageEntryPair) Baggage {
	return CreateBaggage(entries...)
}

// GetBaggage returns the baggage of the context, or nil.
func (p *PropagationAPI) GetBaggage(context Context) Baggage { return getBaggage(context) }

// GetActiveBaggage returns the baggage of the active context, or nil.
func (p *PropagationAPI) GetActiveBaggage() Baggage { return getBaggage(Ctx.Active()) }

// SetBaggage returns a context holding the baggage.
func (p *PropagationAPI) SetBaggage(context Context, baggage Baggage) Context {
	return setBaggage(context, baggage)
}

// DeleteBaggage returns a context without baggage.
func (p *PropagationAPI) DeleteBaggage(context Context) Context { return deleteBaggage(context) }
