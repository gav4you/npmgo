# Licensing

This repository contains **translations** of third-party open-source code. There is no single licence
for the whole repository:

- Each directory under `pkgs/` is a translation of the npm package of the same name and is distributed
  under that package's original licence, which is included as `LICENSE` in that directory together with
  the original copyright notice. A scoped package `@scope/name` is in `pkgs/scope/name` (Go import paths
  cannot contain `@`); subpackages for other entry points of a package (such as `react-dom/server` or
  `opentelemetry/semantic-conventions/incubating`) carry a copy of the same `LICENSE`.
- `pkgs/opentelemetry/api`, `core` and `semantic-conventions` (with `incubating`) translate
  @opentelemetry/api 1.9.0, @opentelemetry/core 2.11.0 and @opentelemetry/semantic-conventions 1.40.0,
  which are under the Apache License 2.0 (Copyright The OpenTelemetry Authors). None of the three tarballs
  ships a `NOTICE` file. As Apache-2.0 §4(b) requires for modified files, every Go file of these packages
  carries the original copyright line and SPDX identifier and states that it was translated to Go by the
  npmgo contributors, from which original files; the generated semantic-conventions files say they were
  generated from the original module by `tools/gen_semconv.cjs`, and their documentation comments are the
  original's.
- `pkgs/opentelemetry/core/merge.go`'s `isPlainObject` is translated from core's `utils/lodash.merge.js`,
  which is based on lodash (MIT, Copyright OpenJS Foundation and other contributors); lodash's notice is
  reproduced at the top of that file.
- `internal/nodeutil/` is a translation of parts of Node.js (`lib/internal/util/inspect.js`,
  `lib/internal/url.js`, `src/node_i18n.cc`) and is distributed under the Node.js licence in
  `internal/nodeutil/LICENSE`. `width_table.go` is generated from Node's ICU data.
- `pkgs/react-dom/server/murmurhash.go` is translated from the MurmurHash3 implementation bundled in
  react-dom (MIT, Copyright (c) 2011 Gary Court); its notice is reproduced at the top of that file.
- `pkgs/graceful-fs/errcode_windows.go` translates libuv's `uv_translate_sys_error` (libuv 1.51.0,
  `src/win/error.c`, MIT, Copyright Joyent, Inc. and other Node contributors); its notice is reproduced at the
  top of that file.
- `internal/jsnum/fdlibm.go` is translated from fdlibm, the math library V8 uses for `Math.atan2`,
  `Math.sin` and `Math.cos` (Copyright (C) 1993 Sun Microsystems, Inc.; use, copying, modification
  and distribution freely granted provided the notice is preserved); its notice is reproduced at
  the top of that file.
- Files written for this project (`parity/`, `tools/`, `internal/jsnum/`, `internal/jsvalue/`,
  `internal/jsloop/`, `internal/jspromise/`, `internal/parityspec/`, `internal/reactspec/`, docs) are
  released under the MIT licence:

  > Copyright (c) 2026 npmgo contributors
  >
  > Permission is hereby granted, free of charge, to any person obtaining a copy of this software and
  > associated documentation files (the "Software"), to deal in the Software without restriction,
  > including without limitation the rights to use, copy, modify, merge, publish, distribute,
  > sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is
  > furnished to do so, subject to the following conditions:
  >
  > The above copyright notice and this permission notice shall be included in all copies or
  > substantial portions of the Software.
  >
  > THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT
  > NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND
  > NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES
  > OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
  > CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

Packages whose licence does not allow redistribution of modified versions (no licence, `UNLICENSED`,
proprietary) are **not** translated or published here; see the licence gate in `ROADMAP.md`.
