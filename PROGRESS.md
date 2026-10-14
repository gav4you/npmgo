# Postiz npm -> Go port: progress

Order: `parity/criticality-ranking.tsv` (number of Postiz packages that depend on each one).
Method: translate the original source (its license kept in the package folder), then run the
ORIGINAL code with Node on the same cases (`parity/oracle_*.cjs` -> `parity/<pkg>/expected.json`)
and diff the Go output against it (`go test ./...`). Test values are written once as JSON specs and
built as real JS values (`parity/build.cjs`) and as Go values (`internal/parityspec`).

Run all checks: `go test ./...`  ·  Re-record a package's originals: see the header of its `parity/oracle_*.cjs`.

| Pkg | Version | License | Status | Parity checks |
|---|---|---|---|---|
| ms | 2.1.3 | MIT | ✅ match (reviewed) | 65 parse (+1 over-length), 43 short, 43 long, 2 errors |
| Node `util.inspect` / `util.format` (dependency of debug) | Node v25.9.0 | MIT (Node.js) | ✅ match (reviewed) | 350 inspect, 145 format, ICU string-width suite (58 strings ×2 modes, 22 code points): proxies, custom inspect, Promise, WeakMap/WeakSet, iterators, coloured stack traces, RegExp highlighting, prototype chains, ArrayBuffer/SharedArrayBuffer/DataView, all typed-array kinds, URL/URLSearchParams, module namespaces incl. TDZ, AggregateError, getters modes, custom sort, every numeric option at Infinity / NaN / negative / fractional values, non-string error stacks, showHidden own properties, %s/%d/%i/%f/%j coercions |
| debug | 4.4.3 | MIT | ✅ match (reviewed) | ×3 env modes (plain, colored, every DEBUG_* coercion): 14 colors, 18×16 enable matrix, 18 disable, 36 log lines (incl. per-instance inspectOpts snapshot vs global), extend, custom formatter, enabled override |
| tslib | 2.8.1 | 0BSD | N/A: TypeScript compiler helpers (`__extends`, `__awaiter`…); Go has these natively | - |
| react | 19.2.4 | MIT | ✅ match (server rendering) | verified through react-dom/server below: elements, `Children`, `cloneElement`, context, every hook React serves on the server, `forwardRef`, `memo`, `lazy`, class components, promises / thenables |
| react-dom (server: `renderToString`, `renderToStaticMarkup`, `renderToPipeableStream`; resource hints, form hooks, `flushSync`, `createPortal`) | 19.2.4 | MIT | ✅ match (server stage; client renderer not started) | 359 cases → 1021 checks: 332 `renderToString`, 332 `renderToStaticMarkup`, 357 `renderToPipeableStream` (HTML + callback order, with scripted promise resolution, pipe timing and aborts); 181 error results. 310 of the stream cases are also rendered through the public `Pipe` / `Wait` API. Plus Go API tests (`api_test.go`, clean under `-race`) |
| scheduler | 0.27.0 | MIT | N/A for this stage: only `react-dom/client` requires it (neither server build does); deferred to the client-renderer stage | - |
| chalk | 4.1.2 | MIT | ✅ match (reviewed) | 1,304 checks: `require('chalk')` in 121 pinned environments (level, supportsColor, stderr, `new Instance()`), level option, 44 styles + visible × 4 levels, 8 colour models × fg/bg × 4 levels over edge values (736), 87 calls (long chains, nesting with embedded codes, LF / CRLF, empty strings, argument coercion and joining, the level setter), 150 templates (nesting, `~`, arguments, escapes, every error), util.js |
| ansi-styles | 4.3.0 | MIT | ✅ match (reviewed) | 1,836 checks: every style's codes, groups, `codes`, the six dynamic tables × 15 models over the color-convert inputs |
| supports-color | 7.2.0 | MIT | ✅ match (reviewed) | 121 pinned environments × stdout, stderr and `supportsColor(stream)` for 4 streams = 726 checks |
| has-flag | 4.0.0 | MIT | ✅ match (reviewed) | 25 cases |
| color-convert | 2.0.1 | MIT | ✅ match (reviewed; `**` routes within 1e-9) | 210 routes (`.conversion`), 8,484 conversions (303 inputs through every route, rounded and raw): 3,894 input/route pairs exact, 348 on `**` routes within 1e-9 (largest deviation 1.8e-12) |
| color-name | 1.1.4 | MIT | ✅ match (reviewed) | 148 colours in key order |
| graceful-fs | 4.2.11 | ISC | ✅ match (pending review) | 193 cases, 11,753 checks: the whole trace of each case on a virtual clock (10,746 events: every call graceful-fs makes to a scripted fs module, every callback, step result, exception and stream event), end time, timer fires (count and hash of their times), the queue left over and the functions of the loaded module (965), and 6 scheduler programs (42 events) |
| es-errors | 1.3.0 | MIT | N/A: re-export of the built-in Error constructors (`Error`, `TypeError`, …); Go returns its own error values | - |
| call-bind | 1.0.8 | MIT | N/A: intrinsic shim, `Function.prototype.call.bind(fn)` with a corrected `length`; Go calls functions and methods directly | - |
| call-bound | 1.0.4 | MIT | N/A: intrinsic shim, call-bound built-ins by name (`%String.prototype.indexOf%` …); Go calls the operation directly | - |
| call-bind-apply-helpers | 1.0.2 | MIT | N/A: intrinsic shim, the `call` / `apply` / `bind` helpers behind call-bind | - |
| get-intrinsic | 1.3.0 | MIT | N/A: intrinsic shim, JS built-ins cached by name against later monkey-patching; Go has no mutable built-ins | - |
| es-object-atoms | 1.1.1 | MIT | N/A: re-export of `Object`, `ToObject`, `RequireObjectCoercible` | - |
| gopd | 1.2.0 | MIT | N/A: intrinsic shim, `Object.getOwnPropertyDescriptor` (IE 8 workaround) | - |
| hasown | 2.0.2 | MIT | N/A: intrinsic shim, call-bound `Object.prototype.hasOwnProperty`; Go uses map lookups and struct fields | - |
| hasown | 2.0.4 | MIT | N/A: as 2.0.2 (same `index.js`) | - |
| has-symbols | 1.1.0 | MIT | N/A: engine probe, whether `Symbol` exists (always on supported Node) | - |
| has-tostringtag | 1.0.2 | MIT | N/A: engine probe, whether `Symbol.toStringTag` exists | - |
| get-proto | 1.0.1 | MIT | N/A: intrinsic shim, `[[Prototype]]` through `Reflect.getPrototypeOf` / `__proto__`; Go types have no prototype chain | - |
| function-bind | 1.1.2 | MIT | N/A: intrinsic shim, `Function.prototype.bind` or its polyfill; Go uses closures and method values | - |
| has-property-descriptors | 1.0.2 | MIT | N/A: engine probe, whether `Object.defineProperty` works (IE 8 bug check) | - |
| es-set-tostringtag | 2.1.0 | MIT | N/A: intrinsic shim, defines `Symbol.toStringTag` on an object | - |
| es-define-property | 1.0.1 | MIT | N/A: intrinsic shim, `Object.defineProperty` (IE 8 workaround) | - |
| define-data-property | 1.1.4 | MIT | N/A: intrinsic shim, `Object.defineProperty` for a data property, falling back to assignment | - |
| math-intrinsics | 1.1.0 | MIT | N/A: intrinsic shim, cached `Math` / `Number` built-ins (`abs`, `floor`, `isNaN`, `MAX_SAFE_INTEGER` …) and one-line helpers on them; Go uses `math` | - |
| dunder-proto | 1.0.1 | MIT | N/A: intrinsic shim, call-bound `__proto__` getter and setter | - |
| set-function-length | 1.2.2 | MIT | N/A: intrinsic shim, sets a function's `length` property; Go functions have none | - |
| safe-buffer | 5.2.1 | MIT | N/A: re-export of Node's `buffer` (a polyfill on old Node); covered by a future Phase 1b `Buffer` port (not started) | - |
| safe-buffer | 5.1.2 | MIT | N/A: as 5.2.1 (re-export of `buffer`); covered by the future Phase 1b `Buffer` port (not started) | - |

Licences of the N/A rows were checked in each tarball (`LICENSE` with the MIT text and copyright, and the
`package.json` licence field). Other versions of the chalk family in the ranking are not yet ported: chalk 5.6.2,
ansi-styles 5.2.0 and 6.2.3, supports-color 5.5.0 and 8.1.1, has-flag 3.0.0.

## Adapters (Go stand-ins for V8 internals) — `internal/nodeutil/adapters.go`
- `Proxy{Target, Handler, Revoked}` — what getProxyDetails returns.
- `Promise{State, Result}`, `WeakSet{Items}`, `WeakMap{Keys, Values}`, `MapIterator`, `SetIterator` — what
  getPromiseDetails / previewEntries return.
- `CustomInspector` interface = `[util.inspect.custom]`; optional `JSModeler` gives the plain-object form used
  when the hook is off or returns itself.
- `Cwd` and `BuiltinModules` feed stack-trace colouring.

## Remaining gaps in nodeutil
Not modelled (the value model has no field for it):
- Functions' own hidden properties (`length`, `name`, `prototype`, and the `Symbol.toStringTag` of
  async / generator functions) and arguments objects' `callee` / `Symbol.iterator`, so `showHidden`
  and `%o` print `[Function: f]` where Node adds `{ [length]: 0, [name]: 'f', [prototype]: … }`.
- The prototype getters Node lists with `showHidden` for `Float16Array` and `SharedArrayBuffer`
  (their constructors are not in `builtInObjects`, see below).
- V8's internal constructor name for a null-prototype object: `Object.setPrototypeOf(new Foo(), null)`
  prints `[Foo: null prototype]` in Node and `[Object: null prototype]` here; an own *enumerable*
  `Symbol.toStringTag` likewise names the prefix in Node. Built-in prototype objects themselves
  (`Array.prototype` → `Object(0) []`, `Map.prototype` → `Object [Map] {}`).
- `Date` / `RegExp` / `Error` values whose constructor name is `Object` take Node's plain-object
  branch; the Go types always print as their kind.
- User `toString` / `Symbol.toPrimitive` methods cannot run: `%s` on such an object prints
  `[object Object]`; `String(date)` uses Go's zone abbreviation, not ICU's long zone name (only the
  NaN-ness matters for `%i` / `%f`).
- `depth: null` cannot be passed to a custom inspector (Go has no null; it receives `+Inf`); an error's
  non-string `stack` is formatted with Node's NaN recursion depth (`ctx.nanDepth`).
- A getter that returns a function: Node's `formatPrimitive` throws and prints
  `<Inspection threw (TypeError … with a V8 stack)>`; Go prints the function.
Deliberate differences:
- A `node_modules/<name>` stack frame with no separator after the module name makes Node loop
  forever in `markNodeModules` (verified: `at f (/a/node_modules/foo)` hangs); the Go port returns the
  line with the module name underlined.
- A fractional or NaN `maxArrayLength` on typed arrays, iterators and weak collections throws
  `RangeError: Invalid array length` in Node (`new Array(maxLength)`); Go prints `floor(maxLength)`
  entries. Arrays, Sets, Maps, ArrayBuffers and strings follow Node exactly (`… 1.5 more items`).
- `highlightRegExp` with an empty or invalid `RegExpColors` palette throws in Node and panics here.
- A `%j` of a BigInt throws in Node and panics here (`TypeError: Do not know how to serialize a BigInt`).
Cannot be verified / reproduced:
- External values are translated but cannot be verified against Node without a native addon.
- Cross-context (vm) objects: the stripped options object passed to their custom hooks.
- "Maximum call stack size exceeded" interruption (Go recursion depth differs from V8).
- Mutating `inspect.defaultOptions` / `inspect.styles` / `inspect.colors`, and the legacy positional
  `inspect(obj, showHidden, depth, colors)` form.
- Lone UTF-16 surrogates in strings (Go strings cannot hold them).
- NFC normalisation before the ICU width uses golang.org/x/text (Unicode 15.0); Node's ICU 78 is
  Unicode 17.0, so a string containing a composition added after Unicode 15 may differ in width.
  The per-code-point width table itself is generated from Node's ICU.
Review notes:
- `builtInObjects` (proto.go) is the set inspect.js captures when it loads, before Node installs
  Buffer, the web globals, SharedArrayBuffer, Float16Array and WebAssembly; `tools/builtin_objects.cjs`
  measures it through `util.format('%s')`. It decides `%s` (inspect vs `String()`), when prototype
  properties are listed, and where the prototype walk stops.
- Numeric options in `parity/nodeutil/cases.json` may be the strings `"Infinity"` / `"NaN"`; an
  error spec's `stack` may be any spec (object, `false`, `0`, `null`); `nullProto`, `lastIndex`,
  iterator `props` and `id` on `fn` / `err` specs are supported by both builders.

## debug
- Each Debugger copies `InspectOpts` when created (`createDebug.init`); `%o` / `%O` format with that
  copy after setting its `colors` to the instance's `UseColors`, while the final
  `util.formatWithOptions` call and `hideDate` read the live global, exactly as in `node.js`. The
  parity "snap" / "mix" / "nocolor" log lines pin this down.
- `DEBUG_*` coercions follow `node.js` + `util.inspect`'s comparisons: booleans count as 0 / 1 where a
  number is expected, `null` is Infinity for depth / maxArrayLength / maxStringLength but 0 for
  breakLength, `null` or NaN compact disables the compact layout, `getters` only for `true`. A
  non-integer `compact` is truncated.
- The optional `supports-color` 256-colour palette isn't used (Postiz doesn't install it); the deprecated
  `destroy()` stub is omitted. `Debugger.Log` receives the formatted line (Node's `log(...args)`
  receives the raw arguments and formats them itself).
- Go: `Enable`, `Disable`, `Enabled` and a Debugger's methods are safe for concurrent use (checked
  under `-race`); `Formatters`, `InspectOpts`, `Log` and `Now` are process-wide settings to configure
  before logging from several goroutines. A `Formatter` runs with the Debugger locked, so it must not
  log through that Debugger. `Print` copies its argument slice (Node splices its own array).

## ms
- Matches on every case, including case-folding (`1 S`, `1 ſ`), over-length and non-ASCII input.
  `Parse` counts the 100-character limit in UTF-16 code units like `String.prototype.length`.

## chalk
chalk 4.1.2 and its dependency tree: `pkgs/chalk`, `pkgs/ansi-styles`, `pkgs/supports-color`, `pkgs/has-flag`,
`pkgs/color-convert`, `pkgs/color-name`.

Verification: `parity/oracle_chalk.cjs` loads the six original packages from a `node_modules` layout and records
`parity/chalk/expected.json` (Node v25.9.0, win32-x64); each package's `parity_test.go` diffs its section. Results are
encoded so that `-0`, `NaN`, `undefined` and thrown errors (name and V8 message) are compared as well. supports-color's
and chalk's load-time state is recorded in a fresh child process per environment row, with `process.argv`,
`process.env` (a plain, case-sensitive object), `process.platform`, `os.release()` and `tty.isatty()` pinned before
anything is required, so nothing of the recording machine reaches the expected output; every other chalk case creates
its instance with an explicit level.

Go API: getters are methods (`c.Red().Bold()`, `c.RGB(…)`, `c.BgHex(…)`, `c.Visible()`), calling is `Sprint`, the tagged
template is `Tag(literals, values...)` or `Template(text)`, `new chalk.Instance(options)` is `c.Instance(…)` / `New(…)`,
`require('chalk')` is `Default` (`Load(supportscolor.Stdout, supportscolor.Stderr)`). `Instance()` without options
detects the level (JavaScript's `undefined` or `{}`); `Instance(Options{})` is `{level: 0}`. supports-color's inputs are
an explicit `Env`: `Load(env)` is the module body as a pure function and `ProcessEnv()` reads the real process. Where
JavaScript throws, Go panics with the same message (the level option, colour models that cannot convert or whose level
is outside 0 to 3); templates return their errors.

Numbers follow V8 through `internal/jsnum`: `Math.round`, `%`, the bitwise operators (`ToInt32`), `parseInt(s, 10)`
(`FORCE_COLOR=0x3` is 0), `Number()` and number-to-string. `Math.atan2`, `Math.sin` and `Math.cos` (lab ↔ lch) are a
translation of fdlibm, which V8 uses on every platform; it matched V8 bit for bit on 193,468 inputs across the double
range, huge arguments included. Products that feed an addition are wrapped in `float64()`, and the arm64, ppc64le,
s390x and riscv64 assembly of `internal/jsnum` and `pkgs/color-convert` contains no fused multiply-add, so the exact
routes hold on those architectures too.

Limits and deviations:
- `**` (color-convert's rgb.xyz, rgb.lab, xyz.rgb, xyz.lab and lab.xyz, and the routes through them): V8 computes it
  with the C library's pow (its default `--use-std-math-pow`; the UCRT on Windows, glibc on Linux; on an 80,000-input
  probe these agreed with each other in 99.86% of cases and with fdlibm in 91%), so Node's own last bits depend on the
  platform. Go uses `math.Pow`. 348 of the 4,242 input/route pairs on these routes differ in the last bits (largest
  1.8e-12 relative to max(1, |x|)) and are compared within 1e-9; their rounded results are equal, and so are the
  ansi-styles codes computed from xyz, lab and lch. All 348 are pow's rounding: with a correctly rounded pow (math/big,
  tried in review) every pair matches bit for bit, so the UCRT's results on these inputs are the correctly rounded
  ones. A correctly rounded pow is not adopted because the C libraries are not correctly rounded either (on a
  79,000-input probe the UCRT missed the correctly rounded result in 0.02-0.6% of cases, so a tolerance would stay) and
  it would be far slower; the deviation grows with the input (2.8e-10 at channels of 1e21), hence 1e-9. On these
  routes the hue of an achromatic colour (chroma or saturation below 1e-6) is the angle of rounding noise and is not
  compared (none differed in this recording). Re-recording on another platform may move these last bits.
- The nearest-keyword distance squares with `** 2`, which V8's std-pow path evaluates as `x * x` (it special-cases the
  exponents 2 and 0.5 before calling pow); Go's `d*d` is the same operation.
- Colour-model values are typed. JavaScript's loose arguments are not modelled: a string given to a numeric model
  (read character by character), a one-element array argument (a one-element `[]float64` reads as its number here),
  missing channels (which leave `undefined` in raw results; here they read as NaN).
- color-name's object also answers to `Object.prototype` members (`constructor`, `toString`, `__proto__` …), so a
  JavaScript keyword lookup of such a name finds a function or an object and yields NaN codes or other TypeErrors; in
  Go they are unknown keywords. `keyword → rgb` returns a copy (JavaScript returns the shared table array).
- color-convert's labels of rgb … lch are strings in JavaScript; `Labels` splits them, one per channel.
- ansi-styles assembles a fresh object on every `require`; the Go tables are shared and read-only. `Codes` is a Go map
  (JavaScript's `Map` also keeps insertion order). The dynamic tables take typed values (JavaScript's
  `color.ansi.ansi('31')` concatenates to `\x1b[310m`, `color.ansi16m.rgb(1)` writes `undefined`).
- supports-color: `Env.Env` is case-sensitive (Node's `process.env` is not on Windows, so `ProcessEnv` upper-cases
  the names there); `ProcessEnv` fills `OSRelease` only on Windows (it is read only for win32), takes `os.Args` for
  `process.argv` (which starts with node and the script) and `golang.org/x/term` for `tty.isatty`. A level beyond the
  int range (a FORCE_COLOR of 19 or more digits) saturates. `browser.js` is not ported. has-flag's `Has` reads `os.Args`.
- chalk levels are `int`. JavaScript accepts any value through its truthiness check and the unchecked setter
  (`{level: NaN}`, `chalk.level = 1.5`, which styles but breaks the colour models).
- Argument coercion (`'' + arg`, `join(' ')`) follows `jsvalue.ToString`: numbers as JavaScript prints them, `nil` as
  `null`, slices joined with commas, errors as `Error: …`, `fmt.Stringer`s through `String`, anything else
  `[object Object]`. A JavaScript object's own `toString` / `valueOf` cannot run, and symbols cannot be passed.
- Templates: a style name that is another property of a chalk object (`level`, `constructor`, `template`, `Instance`,
  `supportsColor`, `stderr`, `name`, `length`, `call`, `_styler` …) reports `Unknown Chalk style` here, where
  JavaScript reads the property (a TypeError, the deprecation error or, on a builder, the `Function` constructor). A
  colour model given arguments that JavaScript passes on unchecked (fewer than three numbers for rgb / hsl / hsv /
  hwb, a string first argument for a numeric model) returns an error instead of codes such as
  `38;2;0;undefined;undefined`. Escapes that produce a lone surrogate give U+FFFD in the returned string; halves that
  meet are joined, as in JavaScript.
- A colour model used without arguments in a template (`{rgb text}`) throws in JavaScript, with a message the port
  reproduces: the `_styler` read, or the conversion's TypeError for an unknown keyword and for an hsv text whose first
  character is not a digit or whitespace (the keyword caveat above applies).
- `Instance` is also a method of builders (JavaScript builders have none); `SupportsColor` and `Stderr` are fields of
  every `*Chalk` but are set only on `Load`'s results; `chalk.template` (the instance itself) and the deprecated
  `chalk.constructor()` are not ported. JavaScript caches each getter's builder on its object; Go builds a new one per
  call, which the output cannot show.
- `Default` is initialised from the real process. Where `require('chalk')` throws (FORCE_COLOR=-1 with nothing else
  detected makes stderr's level −1), importing the package panics.
- Go: a `*Chalk` is safe for concurrent use (the level is atomic, builders are immutable; checked under `-race`).

Cannot be verified: the real-process conveniences (`supportscolor.ProcessEnv`, `Stdout`, `Stderr`, `chalk.Default`,
`hasflag.Has`) depend on the machine and are covered by Go API tests only.

Review notes:
- Reviewed line by line against the originals and with further cases outside the recording, run through the same
  originals: 14 has-flag calls (terminators, multi-unit flags); 76 environments (FORCE_COLOR and TERM_PROGRAM_VERSION
  coercions, the TEAMCITY and TERM expressions, win32 release strings, flag precedence); 52 calls (nested reopen
  chains, CRLF runs, number, array, object and symbol arguments); 118 templates (key order of numeric style names, `~`
  on colour models, whitespace and escape edge cases, 60 spellings of numeric arguments, model errors at every level);
  5,488 color-convert checks on boundary, huge, subnormal and -0 inputs; jsnum's Number, parseInt, String, round and
  ToInt32, and 20,899 atan2 / sin / cos calls. Only the documented deviations differed (FORCE_COLOR beyond the int
  range, string arguments to a numeric model in a template, Object.prototype names as keywords).

## graceful-fs
graceful-fs 4.2.11 in `pkgs/graceful-fs` (Go package `gracefulfs`, named like the other ports), translated from
`graceful-fs.js`, `polyfills.js` and `clone.js`.

Verification: a deterministic trace diff, which checks graceful-fs's own logic against a scripted fs module and a
virtual clock rather than the real disk. `parity/oracle_gracefulfs.cjs` loads the original afresh for every case (a
clean `require.cache`) over a fake fs module injected with a `Module._load` hook (graceful-fs captures fs's functions
when it loads), with `Date.now`, `setTimeout`, `clearTimeout` and `process.nextTick` replaced by one virtual scheduler;
`parity_test.go` runs the same cases through the Go port over the same fake on `internal/jsloop`'s `Virtual`, the Go
twin of that scheduler (ticks before tasks and after every task, tasks by due time then scheduling order, Node's
setTimeout clamp: 0 ms runs after 1 ms). The fake completes each asynchronous call as a task `ms` later (never
synchronously) with the next scripted outcome for that function, and each error carries the number of the call that
produced it, so the trace shows which error reached a callback. Compared per case: every call to the fs module (its
arguments and virtual time), every user callback and its arguments, step results and exceptions, uncaught exceptions,
stream events, the end time, the queue left over, the functions the loaded module has, and the timer fires as a count
and an FNV-1a hash of their times (an entry in 100 ms backoff fires the retry timer every millisecond, so they are not
listed). The scheduler models are also diffed on their own (6 programs). Cases come from
`tools/gen_gracefulfs_cases.cjs`; recorded with Node v25.9.0 on win32-x64 (nothing in the recording depends on the
machine). The oracle refuses a case whose script runs out, even where graceful-fs swallows the exception itself
(lutimesSync's closeSync).

Covered: N EMFILE / ENFILE then success through open, readFile, writeFile, appendFile, copyFile, readdir and the
streams; several entries rotating through the queue under backoff; the 60 s cutoff (59999 retries, 60000 fails with
the latest error, failures delivered after 60 s, the cutoff counted from the first failure, not the first call); a
clock at 0, where `startTime || Date.now()` restarts the window; a successful close, closeSync, close or closeSync
through the underlying module, and lutimes's close resetting the queue (the next attempt still waits 2 ms), failed or
throwing ones not; a reset while the call is in flight; re-entrancy (a timeout callback calling closeSync); a retried
call or a timeout callback that throws, which stops the retry timer until the next reset; two loaded copies over one
fs module and over two (one queue, two retry timers, only the first copy patches close); every argument shape of the
optional options / mode / flags / callback; readdir sorting by UTF-16 code units (astral against U+E000–U+FFFF),
Buffers by their WHATWG UTF-8 decoding (maximal subparts), Dirents never reordered, mixed values, errors that come
with names; chown, fchown, lchown, chmod, fchmod, lchmod and their Sync forms as root, non-root and without getuid
over EPERM, EINVAL, ENOSYS and other errors; the no-op lchmod, lchown and lutimes (and when they stay absent); the
O_SYMLINK lutimes and lutimesSync (open through the queue, error precedence); statFix on stat, fstat, lstat and the
Sync forms, with the two- or three-argument call decided by the truthiness of options; win32 rename (backoff 1, 10, …
100 ms, the stat of the destination, the 59999 / 60000 window, a missing callback throwing, GRACEFUL_FS_PLATFORM
overriding process.platform, empty or not); read and readSync after 10 and 11 EAGAINs, and the short form `read(fd,
cb)`, which graceful-fs does not retry. A mutation check of the harness (39 deliberate changes to the port: `>` for
`>=`, factor 1.25, cap 99, every copy patching close, an unstable sort, root counted as non-root …) failed the parity
test for all but three: `setTimeout(retry, 1)` and a control, which change nothing (the clamp makes 0 and 1 equal),
and sort keys taken from Go's decoding of a string, which differs only for strings that are not valid UTF-8 (not
expressible in the JSON cases) and fails `TestSortInvalidUTF8`.

Go API: `(*Runtime).Load(base)` is `require('graceful-fs')` where `require('fs')` is `base`, an `FS` (a struct of the
functions graceful-fs reads, nil where the module has none), on a `Host` (the clock, timers and nextTick) with
`Runtime.Platform`, `Getuid` and `OSymlink` for process.platform / GRACEFUL_FS_PLATFORM, process.getuid and
constants.O_SYMLINK. Each `Load` is a freshly loaded copy with its own retry timer; the first copy over a base patches
its `Close` / `CloseSync` in place (once), so a close made anywhere through the module resets the queue, and every copy
uses that base's queue. The translation keeps JavaScript's shapes: values graceful-fs does not inspect are `any` (null
is nil, undefined `jsvalue.Undefined{}`), a callback passed in the place of an optional argument is moved as in
JavaScript (recognised by the operation's callback type), and a Runtime with the FS values loaded on it is
single-threaded like JavaScript. For ordinary Go code the blocking functions (`Open`, `Close`, `ReadFile`, `WriteFile`,
`AppendFile`, `ReadDir`, `ReadDirEntries`, `Stat`, `Lstat`, `Fstat`, `Chmod`, `Fchmod`, `Lchmod`, `Chown`, `Fchown`,
`Lchown`, `Rename`, `Read`, `CreateReadStream`, `CreateWriteStream`) run graceful-fs over package os on one internal
event-loop goroutine and may be called from any goroutine.

Not ported (unreachable on supported Node, or with no Go counterpart):
- `legacy-streams.js` (Node 0.8), the `lchmod` back-port of polyfills.js (Node 0.5 and 0.6.0 to 0.6.2), readdir
  without options (Node 0 to 5), and queue entries without a startTime, which only older graceful-fs versions sharing
  the queue push (`retry` would run them at once).
- `gracefulify(fs)` and `TEST_GRACEFUL_FS_GLOBAL_PATCH`, which patch an fs module in place: Go has no global fs module
  to patch, and `Load` returns a patched copy (patching the base's close in place, as graceful-fs patches
  `require('fs')`). Copies of other graceful-fs versions sharing the queue are not modelled.
- process.cwd caching: loading polyfills.js replaces `process.cwd` by a version that caches its first result and
  `process.chdir` by one that clears the cache. graceful-fs never calls `process.cwd` itself, so nothing it does depends
  on this; it changes a process-wide Node API Go does not have (`os.Getwd` is Go's own). The oracle restores both
  after every case.
- The `gfs4` debug log (NODE_DEBUG=gfs4: ENQUEUE / RETRY / TIMEOUT messages and an exit-time assertion that the queue
  is empty); the oracle refuses to run with it set.
- Node's ReadStream / WriteStream internals: only the open method graceful-fs installs is ported (`FS.ReadStream` and
  `FS.WriteStream` take the stream's open method, the `Stream` interface is the part of a stream it uses). The JS class
  mechanics (the ReadStream / WriteStream getters and setters, the FileReadStream / FileWriteStream aliases, calling
  without `new`, `instanceof`) are not modelled.

Deviations in the value model (none is reachable through Node's own fs):
- A nil error stands for null, undefined and a missing error argument (the no-op lchmod, lchown and lutimes call back
  with no argument, real functions with null); the trace records them alike. Callbacks receive their operation's
  values; JavaScript's `cb.apply(this, arguments)` also passes `this` and any extra arguments the fs module gave.
- Synchronous functions return an error instead of throwing, and the ones without a result return nothing (JavaScript
  returns whatever the fs module returned). An fs function that throws synchronously (Node's argument checks) is a
  panic from the backend, which propagates as the exception does (a retried call that throws stops the retry timer
  in both).
- `Stats.Uid` / `Gid` are int64; Node's bigint stats, where graceful-fs's `stats.uid += 0x100000000` throws on a
  negative BigInt, are not modelled.
- readdir's sort compares strings and `[]byte` (Buffers) by their UTF-16 forms, nil as "null", numbers and booleans by
  their JavaScript string forms, anything else as "[object Object]"; `jsvalue.Undefined{}` goes last. Values with their
  own `toString` are not modelled, and lone surrogates cannot be represented. A Go string that is not valid UTF-8 (a
  raw Unix file name) sorts as Node would have decoded it (Node returns the replaced name itself).
- A function passed in the place of an optional argument is moved only when it has the operation's callback type;
  JavaScript moves any function.

The real file system (blocking functions):
- The fs module is package os where it matches Node's, and libuv's behaviour where package os differs: names in
  strcmp order on Unix (libuv's scandir sorts them; Windows keeps directory order, and graceful-fs leaves Dirents in
  it), chown / fchown / lchown succeed without doing anything on Windows, a read at the end of a file is 0 bytes (the
  blocking `Read` returns `io.EOF`), uid and gid are 0 on Windows. Flags are `os.O_*` values and modes
  `os.FileMode`; Node's flag strings and option objects are not interpreted. There is no copyFile, lchmod (so
  graceful-fs's no-op applies, also on macOS where Node has one), futimes or lutimes. `Fchmod` on Windows needs a
  handle opened for writing (Node reopens the handle for attribute access); on a read-only handle the EPERM is
  ignored, as graceful-fs ignores it for every non-root process. A file closed directly (`f.Close()`) does not reset
  the queue.
- err.code of a real error: on Unix the errno's name (the nine codes graceful-fs reads); on Windows libuv 1.51.0's
  `uv_translate_sys_error` (all 100 mappings, through x/sys/windows's constants; Go's invented errno values come out
  as UNKNOWN). libuv's fs-layer special cases (such as EISDIR for ERROR_FILE_EXISTS on an O_CREAT open) are not
  reproduced.
- Machine-dependent, so covered by Go API tests rather than parity: real files and directories, a real Windows sharing
  violation retried by rename until the handle closes, and a real-time EMFILE limit with 40 goroutines. They ran with
  `-race` on Windows and on Linux (the golang:1.26.5 image, as root and as uid 1000, where chown to root fails with
  EPERM and graceful-fs ignores it); macOS and the other targets were only compiled (`go vet` for 13 GOOS).

## React (`pkgs/react`, `pkgs/react-dom`, `pkgs/react-dom/server`)
Translated from the production CJS builds: `react.production.js`, `react-dom.production.js`,
`react-dom-server-legacy.node.production.js` (renderToString / renderToStaticMarkup) and
`react-dom-server.node.production.js` (renderToPipeableStream). The two server builds share one Go
renderer with a legacy / stream flavour switch for every place they differ (static markup, doctype,
outlining, preamble state, md5 vs MurmurHash3 action keys, scheduling).

Verification: `parity/oracle_react.cjs` renders each case in `parity/react/cases.json` (generated by
`tools/gen_react_cases.cjs`) with the original packages, loaded from a `node_modules` layout of the
three tarballs with `NODE_ENV=production`; `internal/reactspec` builds the same trees in Go from the
same JSON; `pkgs/react-dom/server/parity_test.go` diffs HTML, errors and stream callback order.
Streaming cases resolve promises in numbered steps applied when the event loop is idle, so both sides
see the same interleaving. `TestReactParity` drives every stream through an internal test driver
(attach a recording destination, run the event loop to idle, apply the step, repeat; aborts are
applied synchronously, as the oracle calls `abort()`). `TestReactParityPublicStream` additionally
renders each stream case without scripted steps or aborts (301 of 339) through the public
`RenderToPipeableStream(...).Pipe(w)` / `Wait()` path into a `bytes.Buffer`; there the destination's
end / destroy is inferred from what `Pipe` / `Wait` return. Cases with steps or aborts cannot be
timed deterministically through the public API, so they are checked through the internal driver
only, and the public `Abort` (asynchronous, see below) is covered by the Go API tests, not by parity.
Review note: a case whose expected output is an error that the case did not mean to provoke is a
void case (both sides fail the same way). The generator now refuses an element expression used as
an element type, the mistake that had voided 18 Suspense cases; `parity/react/expected.json`
should be scanned for unintended errors when cases are added.

Go API: components receive a `react.Hooks` value (`h.UseState(0)`, `h.UseId()`, `h.Use(p)`…) instead
of reaching a global dispatcher, and all render state is per request, so renders on different
goroutines are independent. `react.Promise` models a JavaScript promise; the streaming renderer runs a
small Node-like event loop (`internal/jsloop`) on the goroutine that calls `Pipe` / `Wait`.
JavaScript value semantics (typeof, ToString, ToNumber, Object.is, ordered objects…) live in
`internal/jsvalue`.

### Known limits
Not ported (out of scope for this stage, or unreachable from the three entry points):
- The client renderer (`react-dom/client`, reconciliation, hydration) and therefore `scheduler`.
- `prerender`, `prerenderToNodeStream`, `resume`, `resumeAndPrerender`, `resumeToPipeableStream` and the
  replay / postpone machinery behind them (`trackedPostpones`, replay tasks, `getPostponedState`);
  `renderToReadableStream` (web streams); `react-dom/static`; the React Server Components builds
  (`*.react-server.js`); the jsx-runtime entry points (use `CreateElement`); `test-utils`; profiling
  builds; development-only warnings and checks; `unstable_externalRuntimeSrc`.
- Node stream integration: backpressure (`drain`) is not modelled (Go writers block); the
  destination's `close` event (client went away) has no `io.Writer` equivalent, so callers abort
  (`PipeableStream.Abort`, e.g. on request-context cancellation). A write error aborts the render
  with React's "The destination stream errored while writing data." and `Pipe` returns it.
- The "Maximum call stack size exceeded" recovery path (React re-schedules the task on a fresh stack):
  Go stacks grow, and a runaway recursion is fatal.

Behaviour that differs, by design or because Go cannot reproduce it (the parity cases do not exercise
these differences):
- Hooks are methods of the `react.Hooks` handle. A stale handle used after its component returned
  throws React's "Invalid hook call" error; in JavaScript the call goes to whatever render is active.
  Context values are stored per request rather than on the shared context object, and a
  `RenderToString` nested inside a component does not reset the outer component's hook state (in
  JavaScript it does).
- `RenderToPipeableStream` starts rendering when the stream is driven by `Pipe` or `Wait` (Node: in a
  microtask). Output equals Node's pattern of piping right after creating the stream.
- The public `Abort` is asynchronous: it queues the abort on the stream's event loop (so it is safe
  from any goroutine) and takes effect after the current task; Node's `abort()` runs synchronously.
  The parity tests apply aborts through the internal synchronous `abort`.
- `ErrorInfo.ComponentStack` lists component names only (`\n    at Name (<anonymous>)`); V8 source
  locations cannot be reproduced. Plain Go funcs have no name: wrap them in `react.FunctionComponent`.
  It is built for every error (JavaScript builds it when `componentStack` is read), so a lazy type
  in the stack is described as the component it resolved to, or as `Lazy`; building the stack never
  runs a loader. Component stacks are not parity-checked.
- `react.CloneElement(nil)` reports "you passed null." (JavaScript distinguishes `undefined`).
- Functions have no source text: where JavaScript would stringify a function (an `<option>` /
  `<textarea>` function child, `String(fn)` in the `use()` error) Go prints a placeholder.
- Strings cannot hold lone UTF-16 surrogates; `toLowerCase` special cases (e.g. U+0130) differ in the
  `data-` / `aria-` prefix check of non-ASCII attribute names.
- `Object.is` on Go closures, slices and maps (e.g. `useMemo` deps) compares code / data pointers.
- `react.Promise` reactions registered by user code with `Then` run on the settling goroutine (or, for
  an already-settled promise, when a render next waits on it), not as microtasks; promise adoption
  (`resolve(otherPromise)`) is not modelled. React's own reactions are delivered on the render's
  event loop exactly as in Node.
- Errors reported globally (`startTransition`) go to `react.ReportGlobalError`; the streaming
  renderer's default `onError` prints with `fmt` (`server.ConsoleError`), not `console.error`'s format.
- `StreamOptions` zero values mean "not passed" (so `progressiveChunkSize: 0` cannot be expressed).
- Byte boundaries of individual writes to the destination are not verified (the HTML is).
