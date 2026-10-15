# npmgo

Native Go translations of npm packages and the Node.js APIs they depend on, verified against the
originals. No Node runtime, no embedded JavaScript engine.

Every port is checked with a **differential parity test**: the original JavaScript runs in Node, the Go
port runs on the same inputs, and the outputs are diffed. A package counts as done only when they match.

```sh
go test ./...
```

## Status

| Package | Version | Parity checks |
|---|---|---|
| [`ms`](pkgs/ms) | 2.1.3 | 154 |
| [`debug`](pkgs/debug) | 4.4.3 | colours, enable/disable matrix, log lines, extend, formatters, in 3 env modes |
| [Node `util.inspect` / `util.format`](internal/nodeutil) | Node v25.9.0 | 350 inspect, 145 format, ICU string width |
| [`react`](pkgs/react) | 19.2.4 | verified through react-dom/server's cases |
| [`react-dom`](pkgs/react-dom) server rendering ([`server`](pkgs/react-dom/server)) | 19.2.4 | 359 cases, 1021 checks: `renderToString`, `renderToStaticMarkup`, `renderToPipeableStream` |
| [`chalk`](pkgs/chalk) | 4.1.2 | 1,304: styles and colour models at every level, templates, 121 pinned environments |
| [`ansi-styles`](pkgs/ansi-styles) | 4.3.0 | 1,836 |
| [`supports-color`](pkgs/supports-color) | 7.2.0 | 726 (121 pinned environments) |
| [`has-flag`](pkgs/has-flag) | 4.0.0 | 25 |
| [`color-convert`](pkgs/color-convert) | 2.0.1 | 210 routes, 8,484 conversions |
| [`color-name`](pkgs/color-name) | 1.1.4 | 148 |
| [`graceful-fs`](pkgs/graceful-fs) | 4.2.11 | 193 cases, 11,771 checks: full traces on a virtual clock over a scripted fs module |
| [`@opentelemetry/core`](pkgs/opentelemetry/core) | 2.11.0 | 676 cases, 1,898 trace lines: W3C trace-context and baggage propagation, TraceState, time, attributes, merge, environment, error handler, promise utilities on a virtual clock |
| [`@opentelemetry/api`](pkgs/opentelemetry/api) | 1.9.0 | 250 cases, 838 trace lines: contexts, the global registry, diag, baggage, propagation, tracers, trace state, metrics |
| [`@opentelemetry/semantic-conventions`](pkgs/opentelemetry/semantic-conventions) | 1.40.0 | every export: 601, and 2,353 of `incubating` |
| [`@babel/types`](pkgs/babel/types) | 7.29.0 | 15,933 traces, and 13,263 with `BABEL_TYPES_8_BREAKING`: every definition table, 257 builders with valid, invalid and missing arguments, 308 `is*` and 308 `assert*` over 297 values, the other 75 exports |
| [`@babel/helper-validator-identifier`](pkgs/babel/helper-validator-identifier) | 7.28.5 | 2,228,784: every code point, names, reserved words |
| [`@babel/helper-string-parser`](pkgs/babel/helper-string-parser) | 7.27.1 | 3,606 cases: string and template contents, integers, code points |

The full ledger is in [`PROGRESS.md`](PROGRESS.md) and the plan is in [`ROADMAP.md`](ROADMAP.md).
The pilot is the dependency tree of [Postiz](https://github.com/gitroomhq/postiz-app) (3,972 packages),
ported in order of how many other packages depend on each one.

## Layout

| Path | Contents |
|---|---|
| `pkgs/<name>/` | Go port + the original package's `LICENSE` (a scoped package `@scope/name` is in `pkgs/scope/name/`: Go import paths cannot contain `@`) |
| `internal/nodeutil/` | Node `util.inspect` / `util.format` port + Node.js `LICENSE` |
| `internal/jsvalue/`, `internal/jsloop/`, `internal/jspromise/` | JavaScript value semantics, a Node-like event loop (used by the React port), a virtual-clock loop with a microtask queue (used by the graceful-fs and OpenTelemetry parity tests) and ECMAScript promises on a microtask queue |
| `parity/` | value specs, Node oracle scripts, test cases, recorded Node outputs |
| `tools/` | generators that derive data tables from Node |

## Re-recording the originals' outputs

The `parity/oracle_*.cjs` scripts run the original JavaScript and write `parity/<pkg>/expected.json`.
Each script's header gives its command line. The original npm tarballs are not stored here; download
them from the npm registry.

## Licensing

See [`LICENSING.md`](LICENSING.md). Each translated package keeps its original licence.
