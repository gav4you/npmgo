# npmgo Roadmap

**Goal:** take the functionality in the npm ecosystem and regenerate it as verified, native Go:
no Node runtime, no embedded JavaScript engine, no compatibility VM. The output is ordinary Go
modules (`go get`) and, eventually, whole applications compiled to a single binary.

Status as of 2026-10-15: Phase 1 is underway (`ms`, `debug`, Node's `util.inspect`/`util.format`,
React 19's server renderer — `react` plus `react-dom/server` — `chalk` 4 with its dependency tree and
`graceful-fs` 4 are ported and verified; `@opentelemetry/core` 2.11.0 with `@opentelemetry/api` 1.9.0 and
`@opentelemetry/semantic-conventions` 1.40.0 are ported and verified, pending review; see `PROGRESS.md`).

---

## Principles (apply to every phase)

1. **Behaviour is the contract.** A port is only done when a differential test shows that the
   original JS and the Go version produce the same outputs, side effects and errors on the same inputs.
2. **Licences are a hard gate.** Nothing is published unless its licence allows redistribution of a
   modified/translated version (see "Licence gate" below). The original `LICENSE` and copyright notice
   travel with every port.
3. **No hidden runtime.** Where JS depends on engine/runtime internals, use Go adapter types
   (as done for Proxy, Promise, WeakMap, …), never an embedded JS engine.
4. **Source-language neutral core.** npm is the first front-end, not the core abstraction.

---

## Phase 1: Translate the npm library (current)

Faithful translation of each package's original source, verified one package at a time.

**1a. Postiz dependency tree (3,972 packages)**: the pilot, in criticality order
(`parity/criticality-ranking.tsv`).

**1b. Node built-in APIs, on demand.** Packages call `fs`, `http`, `crypto`, streams, `Buffer`,
`events`, timers, the URL parser, etc. Each API gets a Go implementation tested against Node the same
way, added when the first package needs it. `util.inspect`/`util.format` was the first.

**1c. Wider npm**: extend in order of download count / dependents, reusing 1b.

Per-package pipeline:

```text
fetch tarball → licence gate → classify → translate → record Node outputs → diff Go vs Node → publish
```

Classification outcomes:
- **Port**: has runtime behaviour.
- **N/A**: types-only (`@types/*`), build tooling, compiler helpers (e.g. `tslib`), and JS-engine intrinsic
  shims and re-exports of built-ins (e.g. `get-intrinsic`, `call-bind`, `hasown`, `es-errors`, `safe-buffer`): Go
  callers use the native operation directly, and a re-exported Node API (`safe-buffer`'s `buffer`) is covered by its
  1b port. The reason is recorded.
- **Blocked**: licence does not permit redistribution, or needs a capability Go cannot provide; listed
  with the reason.

### Licence gate (required before anything goes public)
| Licence | Action |
|---|---|
| MIT, ISC, BSD-2/3, Apache-2.0, 0BSD, Unlicense, CC0, BlueOak | Port; keep `LICENSE` + notice (Apache: also `NOTICE`, state changes). |
| MPL-2.0 | Port; the translated files stay MPL-2.0. |
| GPL/LGPL/AGPL | Port only in a separately licensed area under the same licence; never mixed into permissive modules. |
| No licence, `UNLICENSED`, `SEE LICENSE IN …`, proprietary | **Do not translate or publish.** All rights reserved by default. |

### Exit criteria
- Every in-scope package is Ported, N/A or Blocked, with the reason recorded.
- `go test ./...` green, with a recorded Node version for every parity suite.

---

## Phase 2: Recast, a new, separate native-Go library

Phase 2 is **not** an edit of the Phase 1 translations. It is a new library, invented fresh, in its own
folder and Go module, separate from this one:

```text
NPM Project/
├── golang invention/      Phase 1: faithful translations (this module, `npmgo`)
└── recast/                Phase 2: Recast, the new native-Go library (own go.mod, repo: gav4you/recast)
```

- **Phase 1 stays frozen as the reference.** The Phase 1 ports and their parity suites become the
  behaviour oracle. The new library is tested against them (and, through them, against Node).
- **Designed for Go, not shaped by JS:** goroutines and `context.Context` instead of promise chains,
  errors as values, typed APIs instead of the JS value model and adapters.
- **Free to invent:** merge overlapping npm packages into one coherent API, drop JS-only workarounds,
  add capabilities the originals never had. Behaviour that callers depend on still has to match.
- **New code, own licence:** the new library is original work and can carry its own licence. Where a
  design is directly derived from a Phase 1 port, that package's original notice is kept.
- Benchmarks per package (CPU, memory, startup) vs the Node original. Measure, don't assume.
- Published as its own Go module(s) under one import path, versioned independently of npm.
- **Open question for Phase 2 (user decision 2026-10-15): API shape.** Phase 1 ports deliberately keep a
  JS-shaped Go API (`any` for values the package doesn't inspect, Node-style callbacks, explicit event loop /
  virtual clock, e.g. `pkgs/graceful-fs`) because Phase 1's goal is to copy behaviour as closely as possible.
  Whether Recast keeps any of that shape or goes fully idiomatic (blocking calls, `context.Context`, typed
  options, `error` values) is decided at the start of Phase 2, not before. Phase 1 ports must not be reshaped
  for Go idiom in the meantime.

---

## Phase 3: Convert Node applications

`npmgo convert my-node-app` → `go.mod`, `cmd/`, `internal/`, `generated/`, with no Node at runtime.

- Map `import x from 'pkg'` to the Phase 2 library (falling back to Phase 1 ports where Phase 2 has no equivalent yet).
- Translate application code (Express/Fastify/Nest-style servers, workers, queues, DB access).
- Differential verification at the HTTP/API level: replay recorded traffic against both versions.
- **First target: Postiz's backend** (`apps/backend`, `apps/workers`, `apps/cron` in gitroomhq/postiz-app).

---

## Phase 4: Framework conversion (Next.js and React)

- **Server side → Go:** Route Handlers, middleware, server actions, auth, caching, data access.
- **SSR:** a Go renderer whose HTML output matches React's server renderer (verified against real React).
- **Client side:** browsers don't run Go source. Options to evaluate: keep the client bundle as JS,
  compile client logic to WebAssembly, or generate JS from Go. Decision deferred to this phase.

---

## Phase 5: Semantic transpilation platform

Generalise the pipeline into an "AI software recompiler":

```text
source package (npm / PyPI / RubyGems / …)
  → semantic analysis (what does it do?)
  → behavioural spec + API contract + generated tests
  → native Go generator
  → differential verifier (original ↔ Go)
```

npm becomes one front-end; PyPI, RubyGems, Java and PHP are candidates for later ones.

---

## Repository layout

| Path | Contents |
|---|---|
| `pkgs/<name>/` | Go port + original `LICENSE` (scoped `@scope/name` in `pkgs/scope/name/`: Go import paths cannot contain `@`) |
| `internal/nodeutil/`, future `internal/node*/` | Node built-in API ports |
| `parity/` | shared value specs, Node oracle scripts, cases, recorded expected outputs |
| `tools/` | generators (tables from Node, case builders) |
| `PROGRESS.md` | per-package status ledger |
| `ROADMAP.md` | this file |

Original npm tarballs are **not** committed (they're re-downloadable from the registry and would bloat the repo).

## Known constraints
- **Scale:** the full npm registry has 3M+ packages. Phase 1c needs prioritisation, automation and
  many sessions of work; the pipeline must be resumable.
- **Unrepresentable JS details** (lone UTF-16 surrogates, V8 stack-depth limits, native addons) are
  documented per package, never silently dropped.
- **Malicious packages** exist on npm; tarballs are treated as untrusted data and never executed outside
  the parity sandbox.
