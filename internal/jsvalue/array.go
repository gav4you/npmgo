package jsvalue

// Array is a JavaScript array held by reference: every holder of an *Array sees a change made through any of them
// (Push, Unshift, Pop, Shift, SetAt), and two arrays are the same array only when they are the same *Array, as in
// JavaScript. Go slices ([]any and other slice types) stay accepted wherever an array is read (IsArray, Items); they
// behave as arrays that nothing else holds. Holes are not modelled: an absent element reads as undefined. A nil
// *Array is null.
type Array struct{ items []any }

// NewArray returns a new array of the given elements (copied), like the literal [a, b, c].
func NewArray(items ...any) *Array { return &Array{items: append([]any(nil), items...)} }

// Len is the array's length.
func (a *Array) Len() int {
	if a == nil {
		return 0
	}
	return len(a.items)
}

// At is a[i]: undefined outside the array.
func (a *Array) At(i int) any {
	if a == nil || i < 0 || i >= len(a.items) {
		return Undefined{}
	}
	return a.items[i]
}

// SetAt is the assignment a[i] = v; an index past the end extends the array with undefined elements up to i.
func (a *Array) SetAt(i int, v any) {
	if i < 0 {
		panic("jsvalue: negative array index")
	}
	for len(a.items) <= i {
		a.items = append(a.items, Undefined{})
	}
	a.items[i] = v
}

// Items returns the elements. The slice is the array's own storage: assigning an element through it is a[i] = v,
// appending to it is not a push (use Push).
func (a *Array) Items() []any {
	if a == nil {
		return nil
	}
	return a.items
}

// Push is a.push(...v): it appends the values and returns the new length.
func (a *Array) Push(v ...any) int {
	a.items = append(a.items, v...)
	return len(a.items)
}

// Unshift is a.unshift(...v): it inserts the values at the front, in order, and returns the new length.
func (a *Array) Unshift(v ...any) int {
	if len(v) == 0 {
		return len(a.items)
	}
	items := make([]any, 0, len(v)+len(a.items))
	items = append(items, v...)
	a.items = append(items, a.items...)
	return len(a.items)
}

// Pop is a.pop(): it removes and returns the last element (undefined for an empty array).
func (a *Array) Pop() any {
	if len(a.items) == 0 {
		return Undefined{}
	}
	v := a.items[len(a.items)-1]
	a.items[len(a.items)-1] = nil
	a.items = a.items[:len(a.items)-1]
	return v
}

// Shift is a.shift(): it removes and returns the first element (undefined for an empty array).
func (a *Array) Shift() any {
	if len(a.items) == 0 {
		return Undefined{}
	}
	v := a.items[0]
	a.items = append([]any(nil), a.items[1:]...)
	return v
}

// Includes is a.includes(v): SameValueZero, so NaN is found.
func (a *Array) Includes(v any) bool {
	for _, it := range a.Items() {
		if SameValueZero(it, v) {
			return true
		}
	}
	return false
}

// IndexOf is a.indexOf(v): strict equality, so NaN is never found; -1 when absent.
func (a *Array) IndexOf(v any) int {
	for i, it := range a.Items() {
		if StrictEquals(it, v) {
			return i
		}
	}
	return -1
}
