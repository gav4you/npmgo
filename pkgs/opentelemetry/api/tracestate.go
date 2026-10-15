// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
//
// Translated to Go by the npmgo contributors from @opentelemetry/api 1.9.0 (build/src/trace/internal/utils.js,
// trace/internal/tracestate-impl.js and trace/internal/tracestate-validators.js).

package api

import (
	"regexp"
	"strings"

	"github.com/gav4you/npmgo/internal/jsvalue"
)

// TraceState is the W3C trace state of a span context: an immutable list of key-value pairs, the most recently set
// first. Set and Unset return a new TraceState.
type TraceState interface {
	Set(key, value string) TraceState
	Unset(key string) TraceState
	// Get returns the value under key (JavaScript returns undefined when there is none).
	Get(key string) (string, bool)
	Serialize() string
}

const (
	maxTraceStateItems         = 32
	maxTraceStateLen           = 512
	listMembersSeparator       = ","
	listMemberKeyValueSplitter = "="
)

var (
	// validKeyRe is new RegExp(`^(?:${VALID_KEY}|${VALID_VENDOR_KEY})$`).
	validKeyRe = regexp.MustCompile(`^(?:[a-z][_0-9a-z\-*/]{0,255}|[a-z0-9][_0-9a-z\-*/]{0,240}@[a-z][_0-9a-z\-*/]{0,13})$`)
	// validValueBaseRe is /^[ -~]{0,255}[!-~]$/.
	validValueBaseRe = regexp.MustCompile(`^[ -~]{0,255}[!-~]$`)
)

// validateKey is tracestate-validators.js's validateKey: a lowercase key of up to 256 characters, or a
// tenant@vendor key.
func validateKey(key string) bool { return validKeyRe.MatchString(key) }

// validateValue is validateValue: up to 256 printable ASCII characters, not ending in a space, without , or =.
func validateValue(value string) bool {
	return validValueBaseRe.MatchString(value) && !strings.ContainsAny(value, ",=")
}

// traceStateImpl is TraceStateImpl: the pairs in a Map whose insertion order is the reverse of the serialized order.
type traceStateImpl struct {
	keys   []string
	values map[string]string
}

// CreateTraceState is createTraceState(rawTraceState): a TraceState parsed from a tracestate header ("" for none).
func CreateTraceState(rawTraceState string) TraceState {
	t := &traceStateImpl{values: map[string]string{}}
	if rawTraceState != "" {
		t.parse(rawTraceState)
	}
	return t
}

func (t *traceStateImpl) clone() *traceStateImpl {
	c := &traceStateImpl{keys: append([]string(nil), t.keys...), values: make(map[string]string, len(t.values))}
	for k, v := range t.values {
		c.values[k] = v
	}
	return c
}

func (t *traceStateImpl) mapSet(key, value string) {
	if _, ok := t.values[key]; !ok {
		t.keys = append(t.keys, key)
	}
	t.values[key] = value
}

func (t *traceStateImpl) mapDelete(key string) {
	if _, ok := t.values[key]; !ok {
		return
	}
	delete(t.values, key)
	for i, k := range t.keys {
		if k == key {
			t.keys = append(t.keys[:i:i], t.keys[i+1:]...)
			return
		}
	}
}

// Set returns a copy with key set to value, moved to the front. Nothing is validated.
func (t *traceStateImpl) Set(key, value string) TraceState {
	c := t.clone()
	c.mapDelete(key)
	c.mapSet(key, value)
	return c
}

// Unset returns a copy without key.
func (t *traceStateImpl) Unset(key string) TraceState {
	c := t.clone()
	c.mapDelete(key)
	return c
}

func (t *traceStateImpl) Get(key string) (string, bool) {
	v, ok := t.values[key]
	return v, ok
}

// Serialize lists the pairs as key=value, most recent first, separated by commas.
func (t *traceStateImpl) Serialize() string {
	parts := make([]string, len(t.keys))
	for i := range t.keys {
		k := t.keys[len(t.keys)-1-i]
		parts[i] = k + listMemberKeyValueSplitter + t.values[k]
	}
	return strings.Join(parts, listMembersSeparator)
}

// parse is _parse: nothing for a header longer than 512 UTF-16 units; otherwise the valid members, read from the
// last one to the first (a repeated key keeps the place of its last occurrence and the value of its first), and at
// most 32 of them: beyond that the first 32 members are kept, in header order.
func (t *traceStateImpl) parse(raw string) {
	if jsvalue.Len16(raw) > maxTraceStateLen {
		return
	}
	parts := strings.Split(raw, listMembersSeparator)
	state := &traceStateImpl{values: map[string]string{}}
	for i := len(parts) - 1; i >= 0; i-- {
		listMember := jsvalue.Trim(parts[i])
		j := strings.Index(listMember, listMemberKeyValueSplitter)
		if j == -1 {
			continue
		}
		key, value := listMember[:j], listMember[j+1:]
		if validateKey(key) && validateValue(value) {
			state.mapSet(key, value)
		}
	}
	if len(state.keys) > maxTraceStateItems {
		// new Map(Array.from(entries).reverse().slice(0, 32))
		kept := &traceStateImpl{values: map[string]string{}}
		for i := len(state.keys) - 1; i >= 0 && len(kept.keys) < maxTraceStateItems; i-- {
			k := state.keys[i]
			kept.mapSet(k, state.values[k])
		}
		state = kept
	}
	t.keys, t.values = state.keys, state.values
}
