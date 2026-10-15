// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
//
// Translated to Go by the npmgo contributors from @opentelemetry/core 2.11.0 (build/src/trace/TraceState.js and
// internal/validators.js).

package core

import (
	"regexp"
	"strings"
	"sync"

	"github.com/gav4you/npmgo/internal/jsvalue"
	"github.com/gav4you/npmgo/pkgs/opentelemetry/api"
)

const (
	maxTraceStateItems         = 32
	maxTraceStateLen           = 512
	listMembersSeparator       = ","
	listMemberKeyValueSplitter = "="
)

var (
	// validKeyRe is validators.js's VALID_KEY_REGEX.
	validKeyRe = regexp.MustCompile(`^(?:[a-z][_0-9a-z\-*/]{0,255}|[a-z0-9][_0-9a-z\-*/]{0,240}@[a-z][_0-9a-z\-*/]{0,13})$`)
	// validValueBaseRe is VALID_VALUE_BASE_REGEX, /^[ -~]{0,255}[!-~]$/.
	validValueBaseRe = regexp.MustCompile(`^[ -~]{0,255}[!-~]$`)
)

// validateKey: a lowercase key of up to 256 characters (letters, digits, _ - * /, starting with a letter), or
// tenant@vendor (https://www.w3.org/TR/trace-context/#key).
func validateKey(key string) bool { return validKeyRe.MatchString(key) }

// validateValue: up to 256 printable ASCII characters, not ending in a space, without , or =.
func validateValue(value string) bool {
	return validValueBaseRe.MatchString(value) && !strings.ContainsAny(value, ",=")
}

// orderedState is the Map of a TraceState: insertion order is the reverse of the serialized order.
type orderedState struct {
	keys   []string
	values map[string]string
}

func newOrderedState() *orderedState { return &orderedState{values: map[string]string{}} }

func (s *orderedState) clone() *orderedState {
	c := &orderedState{keys: append([]string(nil), s.keys...), values: make(map[string]string, len(s.values))}
	for k, v := range s.values {
		c.values[k] = v
	}
	return c
}

func (s *orderedState) get(key string) (string, bool) {
	v, ok := s.values[key]
	return v, ok
}

// set is Map.prototype.set: an existing key keeps its place.
func (s *orderedState) set(key, value string) {
	if _, ok := s.values[key]; !ok {
		s.keys = append(s.keys, key)
	}
	s.values[key] = value
}

func (s *orderedState) delete(key string) {
	if _, ok := s.values[key]; !ok {
		return
	}
	delete(s.values, key)
	for i, k := range s.keys {
		if k == key {
			s.keys = append(s.keys[:i:i], s.keys[i+1:]...)
			return
		}
	}
}

// TraceState is the W3C tracestate list (https://www.w3.org/TR/trace-context/#tracestate-field): new and updated
// keys go first. It is immutable (Set and Unset return a new TraceState) and parses its header on first use; it is
// safe for concurrent use.
type TraceState struct {
	rawTraceState string

	mu     sync.Mutex
	length int           // the serialized length, in UTF-16 units
	state  *orderedState // nil until parsed
}

// NewTraceState is new TraceState(rawTraceState): a TraceState of the tracestate header ("" for none; JavaScript
// treats any non-string the same way).
func NewTraceState(rawTraceState string) *TraceState {
	return &TraceState{rawTraceState: rawTraceState, length: jsvalue.Len16(rawTraceState)}
}

// getState is _getState: the parsed list. Members are read from the first; invalid ones and ones that would take the
// length above 512 are skipped (a repeated key keeps its first place, takes the later value and adds its length
// again), and reading stops once 32 keys are held.
func (t *TraceState) getState() (*orderedState, int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.state != nil {
		return t.state, t.length
	}
	vendorEntries := newOrderedState()
	currentLength := 0
	for _, member := range strings.Split(t.rawTraceState, listMembersSeparator) {
		m := jsvalue.Trim(member)
		idx := strings.Index(m, listMemberKeyValueSplitter)
		if idx == -1 {
			continue
		}
		key, value := m[:idx], m[idx+1:]
		if !validateKey(key) || !validateValue(value) {
			continue
		}
		// m is ASCII now: its length in bytes is its length in UTF-16 units.
		futureLength := currentLength + len(m)
		if len(vendorEntries.keys) > 0 {
			futureLength++
		}
		if futureLength > maxTraceStateLen {
			continue
		}
		vendorEntries.set(key, value)
		currentLength = futureLength
		if len(vendorEntries.keys) >= maxTraceStateItems {
			break
		}
	}
	// The Map in the order of the list reversed.
	state := newOrderedState()
	for i := len(vendorEntries.keys) - 1; i >= 0; i-- {
		k := vendorEntries.keys[i]
		state.set(k, vendorEntries.values[k])
	}
	t.length, t.state = currentLength, state
	return state, currentLength
}

func fromState(state *orderedState, length int) *TraceState {
	return &TraceState{length: length, state: state}
}

// Set returns a TraceState with key set to value, first in the list. An invalid key or value, or a result longer
// than 512 characters, returns t itself.
func (t *TraceState) Set(key, value string) api.TraceState { return t.set(key, value) }

func (t *TraceState) set(key, value string) *TraceState {
	if !validateKey(key) || !validateValue(value) {
		return t
	}
	currState, length := t.getState()
	newLength := length
	if currValue, ok := currState.get(key); ok {
		newLength += len(value) - len(currValue)
	} else {
		newLength += len(key) + len(value)
		if len(currState.keys) > 0 {
			newLength += 2
		} else {
			newLength++
		}
	}
	if newLength > maxTraceStateLen {
		return t
	}
	newState := currState.clone()
	newState.delete(key)
	newState.set(key, value)
	return fromState(newState, newLength)
}

// Unset returns a TraceState without key, or t itself when it has no such key.
func (t *TraceState) Unset(key string) api.TraceState { return t.unset(key) }

func (t *TraceState) unset(key string) *TraceState {
	currState, length := t.getState()
	currValue, ok := currState.get(key)
	if !ok {
		return t
	}
	// key, value and validated members are ASCII: their byte lengths are their lengths.
	newLength := length - (jsvalue.Len16(key) + len(currValue) + 1)
	if len(currState.keys) > 1 {
		newLength--
	}
	newState := currState.clone()
	newState.delete(key)
	return fromState(newState, newLength)
}

// Get returns the value under key.
func (t *TraceState) Get(key string) (string, bool) {
	s, _ := t.getState()
	return s.get(key)
}

// Serialize renders the list: key=value pairs separated by commas, the most recently set first.
func (t *TraceState) Serialize() string {
	s, _ := t.getState()
	serialized := ""
	for i, k := range s.keys {
		if i > 0 {
			serialized = listMembersSeparator + serialized
		}
		serialized = k + listMemberKeyValueSplitter + s.values[k] + serialized
	}
	return serialized
}
