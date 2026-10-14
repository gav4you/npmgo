package jsvalue

import (
	"reflect"
)

// Caller is implemented by Go values that stand for JavaScript functions
// and know how to be called.
type Caller interface {
	CallJS(args ...any) any
}

var anyType = reflect.TypeOf((*any)(nil)).Elem()

// Call calls fn the way JavaScript calls a function: missing arguments are
// undefined, extra arguments are dropped. fn may be any Go func; results
// beyond the first are ignored, a missing result is undefined. ok is false
// when fn is not callable.
func Call(fn any, args ...any) (result any, ok bool) {
	switch f := fn.(type) {
	case func():
		f()
		return Undefined{}, true
	case func() any:
		return f(), true
	case func(any):
		f(arg(args, 0))
		return Undefined{}, true
	case func(any) any:
		return f(arg(args, 0)), true
	case func(any, any) any:
		return f(arg(args, 0), arg(args, 1)), true
	case func(...any) any:
		return f(args...), true
	case func(...any):
		f(args...)
		return Undefined{}, true
	case Caller:
		if IsNull(fn) {
			return nil, false
		}
		return f.CallJS(args...), true
	}
	rv := reflect.ValueOf(fn)
	if rv.Kind() != reflect.Func || rv.IsNil() {
		return nil, false
	}
	t := rv.Type()
	n := t.NumIn()
	in := make([]reflect.Value, 0, n)
	for i := 0; i < n; i++ {
		pt := t.In(i)
		if t.IsVariadic() && i == n-1 {
			et := pt.Elem()
			for j := i; j < len(args); j++ {
				in = append(in, convert(args[j], et))
			}
			break
		}
		in = append(in, convert(arg(args, i), pt))
	}
	var out []reflect.Value
	if t.IsVariadic() {
		out = rv.Call(in)
	} else {
		out = rv.Call(in)
	}
	if len(out) == 0 {
		return Undefined{}, true
	}
	return out[0].Interface(), true
}

func arg(args []any, i int) any {
	if i < len(args) {
		return args[i]
	}
	return Undefined{}
}

func convert(v any, t reflect.Type) reflect.Value {
	if v == nil {
		return reflect.Zero(t)
	}
	rv := reflect.ValueOf(v)
	if rv.Type().AssignableTo(t) {
		return rv
	}
	if t == anyType {
		return rv
	}
	if IsNumber(v) && (t.Kind() >= reflect.Int && t.Kind() <= reflect.Float64) {
		return reflect.ValueOf(Float(v)).Convert(t)
	}
	if rv.Type().ConvertibleTo(t) {
		return rv.Convert(t)
	}
	return reflect.Zero(t)
}
