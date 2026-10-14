package nodeutil

import "math/big"

// Go model of the JavaScript values util.inspect understands.
//
//	JS             Go
//	undefined      Undefined{}
//	null           nil
//	boolean        bool
//	number         float64 (int is accepted and converted)
//	bigint         *big.Int
//	string         string
//	symbol         Symbol
//	object/array…  the pointer types below

// Undefined represents JavaScript undefined.
type Undefined struct{}

// Symbol represents a JavaScript symbol primitive.
type Symbol struct{ Description *string }

func (s Symbol) String() string {
	if s.Description == nil {
		return "Symbol()"
	}
	return "Symbol(" + *s.Description + ")"
}

// Hole marks an empty slot in a sparse array.
type Hole struct{}

// Prop is an own property. Key is a string or a Symbol.
type Prop struct {
	Key        any
	Value      any
	Getter     func() (any, error) // non-nil for accessor properties
	HasSetter  bool
	Hidden     bool // non-enumerable
	IsAccessor bool
}

// Proto describes an object's prototype for constructor-name purposes.
// Constructor "" with Null true is a null-prototype object.
type Proto struct {
	Constructor string
	Null        bool
	// Prototype, when set, is the explicit [[Prototype]] object and the
	// constructor name is found by walking the chain (getConstructorName).
	// Use the well-known sentinels (ObjectPrototype, ArrayPrototype, ...) to
	// end a chain at a built-in prototype.
	Prototype *Object
}

// Object is a plain or class-instance object. Zero Proto means "Object".
type Object struct {
	Proto
	// wellKnown names a built-in prototype sentinel (e.g. "Object").
	wellKnown string
	Tag       string // Symbol.toStringTag (inherited / non-enumerable)
	Props     []Prop
	// Arguments marks an `arguments` object.
	Arguments bool
}

// Array is a JS Array. Use Hole{} for empty slots.
type Array struct {
	Proto
	Tag   string
	Items []any
	Props []Prop // non-index properties
}

// Map is a JS Map with insertion-ordered entries.
type Map struct {
	Proto
	Tag    string
	Keys   []any
	Values []any
	Props  []Prop
}

// Set is a JS Set with insertion-ordered values.
type Set struct {
	Proto
	Tag   string
	Items []any
	Props []Prop
}

// Date is a JS Date. Ms is NaN for an invalid date.
type Date struct {
	Proto
	Ms    float64
	Props []Prop
}

// RegExp is a JS regular expression literal. LastIndex is the own
// non-enumerable lastIndex property (shown with showHidden).
type RegExp struct {
	Proto
	Source    string
	Flags     string
	LastIndex float64
	Props     []Prop
}

// Error is a JS Error. Stack nil means the stack property is absent.
type Error struct {
	Proto    // Constructor e.g. "Error", "TypeError"
	Name     string
	Message  string
	Stack    any // string, or any value; nil = no stack
	Cause    any // only used when HasCause
	HasCause bool
	origin   any // set when adapted from another object (errorView)
	Props    []Prop
}

// Function is a JS function or class.
type Function struct {
	Proto     // Constructor defaults to "Function"
	Name      string
	Async     bool
	Generator bool
	Class     bool
	Extends   string // superclass name for classes
	Source    string // Function.prototype.toString() text, used by String(fn)
	// PrototypeObject is fn.prototype, used for instanceof checks.
	PrototypeObject *Object
	Props           []Prop
}

// Boxed is a boxed primitive (new Number(1), new String('a'), ...).
type Boxed struct {
	Proto
	Tag   string
	Value any // float64, string, bool, *big.Int or Symbol
	Props []Prop
}

// TypedArray is a numeric typed array such as Uint8Array.
type TypedArray struct {
	Proto // Constructor e.g. "Uint8Array"
	// Kind is the V8 element type name, e.g. "Uint8Array", "Float64Array",
	// "BigInt64Array". Defaults to "Uint8Array".
	Kind string
	// Items holds numeric elements; BigItems holds BigInt64/BigUint64 elements.
	Items    []float64
	BigItems []*big.Int
	// Buffer, ByteOffset back the showHidden extra keys; nil = own buffer.
	Buffer     *ArrayBuffer
	ByteOffset int
	Props      []Prop
}

func ctorOr(p Proto, def string) *string {
	if p.Null {
		return nil
	}
	if p.Constructor == "" {
		s := def
		return &s
	}
	s := p.Constructor
	return &s
}

var _ = big.NewInt
