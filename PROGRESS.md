# Postiz npm -> Go port: progress

Order: `parity/criticality-ranking.tsv` (number of Postiz packages that depend on each one).
Method: translate the original source (its license kept in the package folder), then run the
ORIGINAL code with Node on the same cases (`parity/oracle_*.cjs` -> `parity/<pkg>/expected.json`)
and diff the Go output against it (`go test ./...`). Test values are written once as JSON specs and
built as real JS values (`parity/build.cjs`) and as Go values (`internal/parityspec`).

Run all checks: `go test ./...`  ·  Re-record a package's originals: see the header of its `parity/oracle_*.cjs`.

| Pkg | Version | License | Status | Parity checks |
|---|---|---|---|---|
| ms | 2.1.3 | MIT | ✅ match | 39 parse, 33 short, 33 long, 2 errors |
| Node `util.inspect` / `util.format` (dependency of debug) | Node v25.9.0 | MIT (Node.js) | ✅ match | 194 inspect, 46 format, ICU string-width suite (proxies, custom inspect, Promise, WeakMap/WeakSet, iterators, coloured stack traces, RegExp highlighting, prototype chains, ArrayBuffer/SharedArrayBuffer/DataView, all typed-array kinds, URL/URLSearchParams, module namespaces incl. TDZ, AggregateError, getters modes, custom sort) |
| debug | 4.4.3 | MIT | ✅ match | ×2 env modes (plain, colored): 14 colors, 13×10 enable matrix, 13 disable, 18 log lines, extend, custom formatter, enabled override |
| tslib | 2.8.1 | 0BSD | N/A: TypeScript compiler helpers (`__extends`, `__awaiter`…); Go has these natively | - |
| react | 19.2.4 | MIT | next | |

## Adapters (Go stand-ins for V8 internals) — `internal/nodeutil/adapters.go`
- `Proxy{Target, Handler, Revoked}` — what getProxyDetails returns.
- `Promise{State, Result}`, `WeakSet{Items}`, `WeakMap{Keys, Values}`, `MapIterator`, `SetIterator` — what
  getPromiseDetails / previewEntries return.
- `CustomInspector` interface = `[util.inspect.custom]`; optional `JSModeler` gives the plain-object form used
  when the hook is off or returns itself.
- `Cwd` and `BuiltinModules` feed stack-trace colouring.

## Remaining gaps in nodeutil
- External values are translated but cannot be verified against Node without a native addon.
- Cross-context (vm) objects: the stripped options object passed to their custom hooks.
- "Maximum call stack size exceeded" interruption (Go recursion depth differs from V8).
- Mutating `inspect.defaultOptions` / `inspect.styles` / `inspect.colors`, and the legacy positional
  `inspect(obj, showHidden, depth, colors)` form.
- Lone UTF-16 surrogates in strings (Go strings cannot hold them).

## debug
- The optional `supports-color` 256-colour palette isn't used (Postiz doesn't install it); the deprecated
  `destroy()` stub is omitted.
