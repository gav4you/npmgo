# Licensing

This repository contains **translations** of third-party open-source code. There is no single licence
for the whole repository:

- Each directory under `pkgs/` is a translation of the npm package of the same name and is distributed
  under that package's original licence, which is included as `LICENSE` in that directory together with
  the original copyright notice.
- `internal/nodeutil/` is a translation of parts of Node.js (`lib/internal/util/inspect.js`,
  `lib/internal/url.js`, `src/node_i18n.cc`) and is distributed under the Node.js licence in
  `internal/nodeutil/LICENSE`. `width_table.go` is generated from Node's ICU data.
- Files written for this project (`parity/`, `tools/`, `internal/jsnum/`, `internal/parityspec/`, docs)
  are released under the MIT licence:

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
