package react

import (
	"iter"
	"runtime"
	"strconv"
	"strings"

	"github.com/gav4you/npmgo/internal/jsloop"
	"github.com/gav4you/npmgo/internal/jsvalue"
)

func isPanicNil(r any) bool {
	_, ok := r.(*runtime.PanicNilError)
	return ok
}

// Iterable is a JavaScript iterable object (something with
// [Symbol.iterator]); React renders and maps its values like an array.
type Iterable interface {
	Iterate() iter.Seq[any]
}

// Children is React.Children.
var Children childrenAPI

type childrenAPI struct{}

// escapeKey is React's escape(key): "$" + key with "=" -> "=0", ":" -> "=2".
func escapeKey(key string) string {
	var b strings.Builder
	b.WriteByte('$')
	for i := 0; i < len(key); i++ {
		switch key[i] {
		case '=':
			b.WriteString("=0")
		case ':':
			b.WriteString("=2")
		default:
			b.WriteByte(key[i])
		}
	}
	return b.String()
}

// escapeUserKey is key.replace(/\/+/g, "$&/").
func escapeUserKey(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] != '/' {
			b.WriteByte(s[i])
			i++
			continue
		}
		j := i
		for j < len(s) && s[j] == '/' {
			j++
		}
		b.WriteString(s[i:j])
		b.WriteByte('/')
		i = j
	}
	return b.String()
}

// keyOf returns the JavaScript value of v.key: kind 0 undefined, 1 null, 2 string.
func keyOf(v any) (kind int, key string) {
	switch x := v.(type) {
	case *Element:
		if x == nil {
			return 0, ""
		}
		if x.Key == nil {
			return 1, ""
		}
		return 2, *x.Key
	case *Portal:
		if x == nil {
			return 0, ""
		}
		if x.Key == nil {
			return 1, ""
		}
		return 2, *x.Key
	case *Object:
		if x == nil {
			return 0, ""
		}
		k, ok := x.Lookup("key")
		if !ok || jsvalue.IsUndefined(k) {
			return 0, ""
		}
		if jsvalue.IsNull(k) {
			return 1, ""
		}
		return 2, jsvalue.ToString(k)
	}
	return 0, ""
}

func getElementKey(element any, index int) string {
	if jsvalue.TypeOf(element) == "object" && !jsvalue.IsNull(element) {
		if kind, k := keyOf(element); kind == 2 {
			return escapeKey(k)
		}
	}
	return strconv.FormatInt(int64(index), 36)
}

// IteratorOf returns the values of v if it is iterable (an Iterable), the
// way getIteratorFn finds [Symbol.iterator].
func IteratorOf(v any) (iter.Seq[any], bool) {
	if it, ok := v.(Iterable); ok && !jsvalue.IsNull(v) {
		return it.Iterate(), true
	}
	return nil, false
}

// IsThenable reports typeof v.then === "function".
func IsThenable(v any) (Thenable, bool) {
	t, ok := v.(Thenable)
	if !ok || jsvalue.IsNull(v) {
		return nil, false
	}
	return t, true
}

// resolveThenable is react's resolveThenable (used by Children).
func resolveThenable(t Thenable) any {
	status, value, reason := t.ThenableStatus()
	switch status {
	case "fulfilled":
		return value
	case "rejected":
		panic(Throw(reason))
	}
	if status != "" {
		jsloop.Then(nil, t, noop, noop)
	} else {
		t.SetThenableStatus("pending", nil, nil)
		jsloop.Then(nil, t, func(v any) {
			if s, _, _ := t.ThenableStatus(); s == "pending" {
				t.SetThenableStatus("fulfilled", v, nil)
			}
		}, func(err any) {
			if s, _, _ := t.ThenableStatus(); s == "pending" {
				t.SetThenableStatus("rejected", nil, err)
			}
		})
	}
	status, value, reason = t.ThenableStatus()
	switch status {
	case "fulfilled":
		return value
	case "rejected":
		panic(Throw(reason))
	}
	panic(t)
}

// NotValidChildMessage is the message React throws for an object child.
func NotValidChildMessage(found string) string {
	return "Objects are not valid as a React child (found: " + found +
		"). If you meant to render a collection of children, use an array instead."
}

func mapIntoArray(children any, array *[]any, escapedPrefix, nameSoFar string, callback func(any) any) int {
	typ := jsvalue.TypeOf(children)
	if typ == "undefined" || typ == "boolean" {
		children = nil
	}
	invoke := false
	if jsvalue.IsNull(children) {
		invoke = true
	} else {
		switch typ {
		case "bigint", "string", "number":
			invoke = true
		case "object":
			switch c := children.(type) {
			case *Element, *Portal:
				invoke = true
			case *LazyType:
				return mapIntoArray(c.Init(nil), array, escapedPrefix, nameSoFar, callback)
			}
		}
	}
	if invoke {
		mapped := callback(children)
		childKey := nameSoFar
		if nameSoFar == "" {
			childKey = "." + getElementKey(children, 0)
		}
		if jsvalue.IsArray(mapped) {
			escapedChildKey := escapeUserKey(childKey) + "/"
			mapIntoArray(mapped, array, escapedChildKey, "", func(c any) any { return c })
		} else if !jsvalue.IsNullish(mapped) {
			if el, ok := mapped.(*Element); ok && el != nil {
				mid := ""
				mk, mkey := keyOf(el)
				ck, ckey := keyOf(children)
				sameKey := jsvalue.Truthy(children) && mk == ck && mkey == ckey
				if !(mk == 1 || sameKey) {
					mid = escapeUserKey(mkey) + "/"
				}
				newKey := escapedPrefix + mid + childKey
				mapped = newElement(el.Type, &newKey, el.Props)
			}
			*array = append(*array, mapped)
		}
		return 1
	}
	count := 0
	nextNamePrefix := "."
	if nameSoFar != "" {
		nextNamePrefix = nameSoFar + ":"
	}
	if jsvalue.IsArray(children) {
		for i, child := range jsvalue.Items(children) {
			count += mapIntoArray(child, array, escapedPrefix, nextNamePrefix+getElementKey(child, i), callback)
		}
	} else if seq, ok := IteratorOf(children); ok {
		i := 0
		for child := range seq {
			count += mapIntoArray(child, array, escapedPrefix, nextNamePrefix+getElementKey(child, i), callback)
			i++
		}
	} else if typ == "object" {
		if t, ok := IsThenable(children); ok {
			return mapIntoArray(resolveThenable(t), array, escapedPrefix, nameSoFar, callback)
		}
		s := jsvalue.String(children)
		found := s
		if s == "[object Object]" {
			found = "object with keys {" + strings.Join(jsvalue.ObjectKeys(children), ", ") + "}"
		}
		panic(newError(NotValidChildMessage(found)))
	}
	return count
}

func mapChildren(children any, fn func(child any, index int) any) any {
	if jsvalue.IsNullish(children) {
		return children
	}
	result := []any{}
	count := 0
	mapIntoArray(children, &result, "", "", func(child any) any {
		i := count
		count++
		return fn(child, i)
	})
	return result
}

// Map is React.Children.map(children, fn). It returns children itself when
// children is null or undefined, otherwise a []any.
func (childrenAPI) Map(children any, fn func(child any, index int) any) any {
	return mapChildren(children, fn)
}

// ForEach is React.Children.forEach(children, fn).
func (childrenAPI) ForEach(children any, fn func(child any, index int)) {
	mapChildren(children, func(child any, index int) any {
		fn(child, index)
		return Undefined{}
	})
}

// Count is React.Children.count(children).
func (childrenAPI) Count(children any) int {
	n := 0
	mapChildren(children, func(any, int) any {
		n++
		return Undefined{}
	})
	return n
}

// ToArray is React.Children.toArray(children).
func (childrenAPI) ToArray(children any) []any {
	r := mapChildren(children, func(child any, _ int) any { return child })
	if a, ok := r.([]any); ok {
		return a
	}
	return []any{}
}

// Only is React.Children.only(children).
func (childrenAPI) Only(children any) *Element {
	if !IsValidElement(children) {
		panic(newError("React.Children.only expected to receive a single React element child."))
	}
	return children.(*Element)
}
