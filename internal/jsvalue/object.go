package jsvalue

import "strconv"

// Object is an ordered JavaScript plain object: its own enumerable
// string-keyed properties, iterated in JavaScript's own-key order
// (array-index keys ascending, then other keys in insertion order). A nil
// *Object reads as an empty object.
type Object struct {
	entries []entry
	index   map[string]int // built lazily for larger objects
}

type entry struct {
	key string
	val any
}

const indexThreshold = 12

// NewObject builds an object from alternating key, value arguments.
func NewObject(kv ...any) *Object {
	o := &Object{}
	for i := 0; i+1 < len(kv); i += 2 {
		k, ok := kv[i].(string)
		if !ok {
			k = ToString(kv[i])
		}
		o.Set(k, kv[i+1])
	}
	return o
}

// arrayIndex reports whether k is a canonical array index ("0", "1", ...,
// below 2^32-1), which JavaScript enumerates before other keys.
func arrayIndex(k string) (uint64, bool) {
	if k == "" || len(k) > 10 || (len(k) > 1 && k[0] == '0') {
		return 0, false
	}
	n, err := strconv.ParseUint(k, 10, 64)
	if err != nil || n >= 1<<32-1 {
		return 0, false
	}
	return n, true
}

func (o *Object) find(k string) int {
	if o == nil {
		return -1
	}
	if o.index != nil {
		if i, ok := o.index[k]; ok {
			return i
		}
		return -1
	}
	for i := range o.entries {
		if o.entries[i].key == k {
			return i
		}
	}
	return -1
}

func (o *Object) reindex() {
	if len(o.entries) < indexThreshold {
		o.index = nil
		return
	}
	o.index = make(map[string]int, len(o.entries))
	for i, e := range o.entries {
		o.index[e.key] = i
	}
}

// Lookup returns the value of own property k and whether it exists.
func (o *Object) Lookup(k string) (any, bool) {
	if i := o.find(k); i >= 0 {
		return o.entries[i].val, true
	}
	return nil, false
}

// Get returns own property k, or Undefined{} when it is absent.
func (o *Object) Get(k string) any {
	if v, ok := o.Lookup(k); ok {
		return v
	}
	return Undefined{}
}

// Has is hasOwnProperty(k).
func (o *Object) Has(k string) bool { return o.find(k) >= 0 }

// Set assigns property k, keeping the position of an existing key.
func (o *Object) Set(k string, v any) {
	if i := o.find(k); i >= 0 {
		o.entries[i].val = v
		return
	}
	pos := len(o.entries)
	if n, ok := arrayIndex(k); ok {
		pos = 0
		for pos < len(o.entries) {
			m, isIdx := arrayIndex(o.entries[pos].key)
			if !isIdx || m > n {
				break
			}
			pos++
		}
	}
	o.entries = append(o.entries, entry{})
	copy(o.entries[pos+1:], o.entries[pos:])
	o.entries[pos] = entry{k, v}
	if o.index != nil || len(o.entries) >= indexThreshold {
		o.reindex()
	}
}

// Delete removes property k and reports whether it existed.
func (o *Object) Delete(k string) bool {
	i := o.find(k)
	if i < 0 {
		return false
	}
	o.entries = append(o.entries[:i], o.entries[i+1:]...)
	if o.index != nil {
		o.reindex()
	}
	return true
}

// Keys returns the own keys in JavaScript order (Object.keys).
func (o *Object) Keys() []string {
	if o == nil {
		return nil
	}
	keys := make([]string, len(o.entries))
	for i, e := range o.entries {
		keys[i] = e.key
	}
	return keys
}

// Len is the number of own properties.
func (o *Object) Len() int {
	if o == nil {
		return 0
	}
	return len(o.entries)
}

// Range calls f for each property in order until f returns false.
func (o *Object) Range(f func(k string, v any) bool) {
	if o == nil {
		return
	}
	for _, e := range o.entries {
		if !f(e.key, e.val) {
			return
		}
	}
}

// Clone returns a shallow copy ({...o}).
func (o *Object) Clone() *Object {
	c := &Object{}
	if o != nil {
		c.entries = append([]entry(nil), o.entries...)
		c.reindex()
	}
	return c
}

// Assign copies the own properties of each source onto o, like
// Object.assign(o, ...sources). Non-object sources contribute nothing.
func (o *Object) Assign(sources ...any) *Object {
	for _, s := range sources {
		if src, ok := s.(*Object); ok && src != nil {
			for _, e := range src.entries {
				o.Set(e.key, e.val)
			}
		}
	}
	return o
}
