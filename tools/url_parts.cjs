// Prints the WHATWG URL components and Node URLContext for the given hrefs
// (used to write URL parity specs). Run: node --expose-internals tools/url_parts.cjs <href>...
'use strict';
const util = require('util');
for (const href of process.argv.slice(2)) {
  const u = new URL(href);
  const parts = {};
  for (const k of ['href', 'origin', 'protocol', 'username', 'password', 'host', 'hostname', 'port', 'pathname', 'search', 'hash']) parts[k] = u[k];
  const params = [...u.searchParams];
  // URLContext is only reachable through inspect(showHidden); parse it back out.
  const shown = util.inspect(u, { showHidden: true, depth: 2, breakLength: Infinity });
  const m = /\[?Symbol\(context\)\]?: URLContext \{([^}]*)\}/.exec(shown);
  const ctx = {};
  if (m) for (const [, k, v] of m[1].matchAll(/(\w+): ('(?:[^'\\]|\\.)*'|[-\d.]+|true|false)/g)) {
    ctx[k] = v[0] === "'" ? v.slice(1, -1) : v === 'true' ? true : v === 'false' ? false : Number(v);
  }
  console.log(JSON.stringify({ parts, params, ctx, shown }));
}
