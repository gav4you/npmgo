// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
//
// Translated to Go by the npmgo contributors from @opentelemetry/api 1.9.0 (build/src/baggage/utils.js,
// baggage/internal/baggage-impl.js, baggage/internal/symbol.js and baggage/context-helpers.js).

package api

import (
	"github.com/gav4you/npmgo/internal/jsvalue"
)

// BaggageEntryMetadata is opaque metadata of a baggage entry; ToString is its toString().
type BaggageEntryMetadata interface {
	ToString() string
}

// BaggageEntry is a baggage value with optional metadata (nil: undefined).
type BaggageEntry struct {
	Value    string
	Metadata BaggageEntryMetadata
}

// BaggageEntryPair is one [key, entry] pair of Baggage.GetAllEntries, and one property of the record CreateBaggage
// takes.
type BaggageEntryPair struct {
	Key   string
	Entry BaggageEntry
}

// Baggage is an immutable set of entries: the setters return a new Baggage.
type Baggage interface {
	// GetEntry returns a copy of the entry under key, or nil.
	GetEntry(key string) *BaggageEntry
	// GetAllEntries returns the entries in insertion order (replacing an entry keeps its place).
	GetAllEntries() []BaggageEntryPair
	// SetEntry returns a new Baggage with entry under key.
	SetEntry(key string, entry BaggageEntry) Baggage
	// RemoveEntry returns a new Baggage without key.
	RemoveEntry(key string) Baggage
	// RemoveEntries returns a new Baggage without the keys.
	RemoveEntries(keys ...string) Baggage
	// Clear returns an empty Baggage.
	Clear() Baggage
}

// baggageImpl is BaggageImpl: a Map from key to entry, in insertion order.
type baggageImpl struct {
	keys    []string
	entries map[string]BaggageEntry
}

func newBaggageImpl(from *baggageImpl) *baggageImpl {
	b := &baggageImpl{entries: map[string]BaggageEntry{}}
	if from != nil {
		b.keys = append([]string(nil), from.keys...)
		for k, v := range from.entries {
			b.entries[k] = v
		}
	}
	return b
}

// set is Map.prototype.set: a new key goes last, an existing one keeps its place.
func (b *baggageImpl) set(key string, entry BaggageEntry) {
	if _, ok := b.entries[key]; !ok {
		b.keys = append(b.keys, key)
	}
	b.entries[key] = entry
}

func (b *baggageImpl) delete(key string) {
	if _, ok := b.entries[key]; !ok {
		return
	}
	delete(b.entries, key)
	for i, k := range b.keys {
		if k == key {
			b.keys = append(b.keys[:i:i], b.keys[i+1:]...)
			break
		}
	}
}

func (b *baggageImpl) GetEntry(key string) *BaggageEntry {
	e, ok := b.entries[key]
	if !ok {
		return nil
	}
	return &e // Object.assign({}, entry): a copy
}

func (b *baggageImpl) GetAllEntries() []BaggageEntryPair {
	out := make([]BaggageEntryPair, len(b.keys))
	for i, k := range b.keys {
		out[i] = BaggageEntryPair{k, b.entries[k]}
	}
	return out
}

func (b *baggageImpl) SetEntry(key string, entry BaggageEntry) Baggage {
	n := newBaggageImpl(b)
	n.set(key, entry)
	return n
}

func (b *baggageImpl) RemoveEntry(key string) Baggage {
	n := newBaggageImpl(b)
	n.delete(key)
	return n
}

func (b *baggageImpl) RemoveEntries(keys ...string) Baggage {
	n := newBaggageImpl(b)
	for _, k := range keys {
		n.delete(k)
	}
	return n
}

func (b *baggageImpl) Clear() Baggage { return newBaggageImpl(nil) }

// CreateBaggage is createBaggage(record): a Baggage of the record's entries, in Object.entries order. The record is
// given as its property assignments in order (record[key] = entry), so its keys are ordered as a JavaScript object
// orders them: array-index keys first, ascending, then the others by first assignment; a repeated key keeps its
// place with its last entry, and an assignment to "__proto__" creates no property.
func CreateBaggage(entries ...BaggageEntryPair) Baggage {
	record := &jsvalue.Object{}
	for _, e := range entries {
		record.Put(e.Key, e.Entry)
	}
	b := newBaggageImpl(nil)
	record.Range(func(k string, v any) bool {
		b.set(k, v.(BaggageEntry))
		return true
	})
	return b
}

// baggageEntryMetadata is the object baggageEntryMetadataFromString returns (its __TYPE__ brand is the type).
type baggageEntryMetadata struct{ s string }

func (m baggageEntryMetadata) ToString() string { return m.s }

// BaggageEntryMetadataFromString returns metadata whose toString() is str. A str that is not a string is logged as an
// error to Diag ("Cannot create baggage metadata from unknown type: <typeof>") and replaced by "".
func BaggageEntryMetadataFromString(str any) BaggageEntryMetadata {
	s, ok := str.(string)
	if !ok {
		Diag.Error("Cannot create baggage metadata from unknown type: " + jsvalue.TypeOf(str))
		s = ""
	}
	return baggageEntryMetadata{s}
}

// baggageKey is the context key of the baggage.
var baggageKey = CreateContextKey("OpenTelemetry Baggage Key")

// getBaggage is getBaggage(context): the Baggage stored in the context, or nil.
func getBaggage(context Context) Baggage {
	b, _ := context.GetValue(baggageKey).(Baggage)
	if jsvalue.IsNull(b) {
		return nil
	}
	return b
}

func setBaggage(context Context, baggage Baggage) Context {
	return context.SetValue(baggageKey, baggage)
}

func deleteBaggage(context Context) Context { return context.DeleteValue(baggageKey) }
