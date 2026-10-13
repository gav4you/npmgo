package nodeutil

// Adapter types: Go stand-ins for state that Node reads out of V8 internals
// (getProxyDetails, previewEntries, getPromiseDetails). Callers construct
// them with the same information V8 would expose.

// Proxy is a JS Proxy. Revoked proxies print as <Revoked Proxy>.
type Proxy struct {
	Target  any
	Handler any
	Revoked bool
}

// PromiseState mirrors V8's kPending / kFulfilled / kRejected.
type PromiseState int

const (
	Pending PromiseState = iota
	Fulfilled
	Rejected
)

// Promise is a JS Promise with its settled state and result.
type Promise struct {
	Proto
	State  PromiseState
	Result any
	Props  []Prop
}

// WeakSet holds the entries V8's previewEntries would return.
type WeakSet struct {
	Proto
	Items []any
	Props []Prop
}

// WeakMap holds the entries V8's previewEntries would return.
type WeakMap struct {
	Proto
	Keys   []any
	Values []any
	Props  []Prop
}

// MapIterator is a Map iterator (map.keys(), .values() or .entries()).
// For Entries iterators, Items is flattened [k1, v1, k2, v2, ...].
type MapIterator struct {
	Tag     string // defaults to "Map Iterator"
	Items   []any
	Entries bool
	Props   []Prop
}

// SetIterator is a Set iterator (set.values() or .entries()).
type SetIterator struct {
	Tag     string // defaults to "Set Iterator"
	Items   []any
	Entries bool
	Props   []Prop
}

// CustomOptions is what a custom inspector receives, like the options
// object passed to [util.inspect.custom](depth, options, inspect).
type CustomOptions struct {
	Options
	Stylize func(str, styleType string) string
}

// CustomInspector is the Go form of [util.inspect.custom]. Returning a
// string prints it verbatim (re-indented); any other value is inspected.
// Returning the receiver itself falls back to normal formatting.
type CustomInspector interface {
	InspectCustom(depth float64, opts CustomOptions, inspect func(v any, o Options) string) any
}

// InspectCustomDescription is the description of util.inspect.custom
// (Symbol.for('nodejs.util.inspect.custom')).
const InspectCustomDescription = "nodejs.util.inspect.custom"

// JSModeler lets a value (typically a CustomInspector) describe itself as a
// plain modelled JS value, used when its custom hook is not applied
// (customInspect: false, or the hook returns the receiver).
type JSModeler interface {
	JSModel() any
}

// ArrayBuffer is a JS ArrayBuffer (or SharedArrayBuffer when Shared).
type ArrayBuffer struct {
	Proto
	Bytes    []byte
	Shared   bool
	Detached bool
	Props    []Prop
}

// DataView is a JS DataView over Buffer.
type DataView struct {
	Proto
	Buffer     *ArrayBuffer
	ByteOffset int
	ByteLength int
	Props      []Prop
}

// URL is a WHATWG URL object (only Href is needed when customInspect is off;
// the full record drives URL's own [util.inspect.custom]).
type URL struct {
	Href, Origin, Protocol, Username, Password   string
	Host, Hostname, Port, Pathname, Search, Hash string
	SearchParams                                 *URLSearchParams
	Context                                      *URLContext // shown under Symbol(context) with showHidden
	Props                                        []Prop
}

// URLSearchParams is a WHATWG URLSearchParams list.
type URLSearchParams struct {
	Names, Values []string
}

// ModuleNamespace is an ES module namespace object. Uninitialized lists
// bindings in the temporal dead zone (accessing them throws ReferenceError).
type ModuleNamespace struct {
	Props         []Prop
	Uninitialized map[string]bool
}

// External is a V8 External wrapping a native pointer address.
type External struct {
	Address uint64
}
