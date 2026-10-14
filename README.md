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

The full ledger is in [`PROGRESS.md`](PROGRESS.md) and the plan is in [`ROADMAP.md`](ROADMAP.md).
The pilot is the dependency tree of [Postiz](https://github.com/gitroomhq/postiz-app) (3,972 packages),
ported in order of how many other packages depend on each one.

## Layout

| Path | Contents |
|---|---|
| `pkgs/<name>/` | Go port + the original package's `LICENSE` |
| `internal/nodeutil/` | Node `util.inspect` / `util.format` port + Node.js `LICENSE` |
| `internal/jsvalue/`, `internal/jsloop/` | JavaScript value semantics and a Node-like event loop, used by the React port |
| `parity/` | value specs, Node oracle scripts, test cases, recorded Node outputs |
| `tools/` | generators that derive data tables from Node |

## Re-recording the originals' outputs

The `parity/oracle_*.cjs` scripts run the original JavaScript and write `parity/<pkg>/expected.json`.
Each script's header gives its command line. The original npm tarballs are not stored here; download
them from the npm registry.

## Licensing

See [`LICENSING.md`](LICENSING.md). Each translated package keeps its original licence.
