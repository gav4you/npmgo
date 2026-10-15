package parityspec

import (
	"github.com/gav4you/npmgo/internal/jsvalue"
)

// CallFromOutside calls fn with args from outside the Babel ports (pkgs/babel/...): their deprecation warnings check
// whether the caller of a deprecated function, two frames up, is itself in a Babel package, and the oracle calls the
// originals from outside @babel. fn is a builder (func(...any) *jsvalue.Object), a validator (func(any, ...any) bool),
// an assert (func(any, ...any)) or a deprecation warning (func(string, string, ...string)).
//
//go:noinline
func CallFromOutside(fn any, args []any) any { return callFromOutside(fn, args) }

//go:noinline
func callFromOutside(fn any, args []any) any {
	arg := func(i int) any {
		if i < len(args) {
			return args[i]
		}
		return jsvalue.Undefined{}
	}
	rest := func() []any {
		if len(args) > 1 {
			return args[1:]
		}
		return nil
	}
	switch f := fn.(type) {
	case func(...any) *jsvalue.Object:
		return f(args...)
	case func(any, ...any) bool:
		return f(arg(0), rest()...)
	case func(any, ...any):
		f(arg(0), rest()...)
		return jsvalue.Undefined{}
	case func(string, string, ...string):
		strs := make([]string, len(args))
		for i, a := range args {
			strs[i] = jsvalue.ToString(a)
		}
		if len(strs) < 2 {
			strs = append(strs, make([]string, 2-len(strs))...)
		}
		f(strs[0], strs[1], strs[2:]...)
		return jsvalue.Undefined{}
	}
	panic("parityspec.CallFromOutside: unsupported function type")
}
