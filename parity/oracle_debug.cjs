// Runs the ORIGINAL debug@4.4.3 (Node build) against the parity cases in three env modes.
// Run: node parity/oracle_debug.cjs <debug package dir> parity/debug/cases.json > parity/debug/expected.json
// (<debug package dir> is the unpacked debug-4.4.3.tgz `package` directory; `ms` must be
// resolvable from it, e.g. in a node_modules directory beside it.)
'use strict';
const path = require('path');
const { build } = require('./build.cjs');
const [debugDir, casesPath] = process.argv.slice(2);
const c = require(path.resolve(casesPath));
const realNow = Date.now;
function load(env) {
  for (const k of Object.keys(process.env)) if (/^debug/i.test(k)) delete process.env[k];
  Object.assign(process.env, env);
  for (const k of Object.keys(require.cache)) if (k.startsWith(path.resolve(debugDir))) delete require.cache[k];
  return require(path.join(debugDir, 'src', 'node.js'));
}
// Captures what debug's default log sink writes to stderr: one entry per
// write, without the trailing newline.
function capture(fn) {
  const lines = [];
  const realWrite = process.stderr.write;
  process.stderr.write = (chunk) => { lines.push(String(chunk).replace(/\n$/, '')); return true; };
  try { fn(); } finally { process.stderr.write = realWrite; }
  return lines;
}
function run(env) {
  const debug = load(env);
  const out = {};
  out.colors = c.colors.map(ns => debug.selectColor(ns));
  out.enabled = c.patterns.map(p => { debug.enable(p); return c.names.map(n => debug.enabled(n)); });
  out.disable = c.patterns.map(p => { debug.enable(p); return debug.disable(); });
  debug.enable('*');
  // Each log case: create the instance (it snapshots inspectOpts), optionally
  // force useColors and change the global inspectOpts afterwards, then log.
  out.logs = c.logs.map(k => capture(() => {
    Date.now = () => 1700000000000; // fixed clock: diff = 0
    const d = debug(k.ns);
    if (k.useColors !== undefined) d.useColors = k.useColors;
    const saved = { ...debug.inspectOpts };
    if (k.globalOpts) Object.assign(debug.inspectOpts, k.globalOpts);
    try {
      d(...build(k.args));
    } finally {
      for (const key of Object.keys(debug.inspectOpts)) delete debug.inspectOpts[key];
      Object.assign(debug.inspectOpts, saved);
      Date.now = realNow;
    }
  }));
  const parent = debug('parent');
  out.extend = [parent.extend('child').namespace, parent.extend('child', '/').namespace, parent.extend('child', '').namespace];
  debug.formatters.h = v => Buffer.from(String(v)).toString('hex');
  const d = debug('custom');
  d.useColors = false;
  out.custom = capture(() => d('hex %h and %s', 'AB', 'tail'));
  delete debug.formatters.h;
  const d2 = debug('override'); debug.enable(''); d2.enabled = true; out.override = d2.enabled; debug.enable('*'); d2.enabled = false; out.override2 = d2.enabled;
  return out;
}
const plain = run({ DEBUG_COLORS: 'no', DEBUG_HIDE_DATE: 'yes' });
const colored = run({ DEBUG_COLORS: 'true', DEBUG_HIDE_DATE: 'yes' });
// Every DEBUG_* coercion: yes/no words, numbers, 'null', NaN, booleans where numbers are expected.
const env = run({ DEBUG_COLORS: 'off', DEBUG_HIDE_DATE: 'on', DEBUG_DEPTH: 'yes', DEBUG_SHOW_HIDDEN: '1', DEBUG_COMPACT: 'null', DEBUG_BREAK_LENGTH: 'abc', DEBUG_MAX_ARRAY_LENGTH: '2', DEBUG_SORTED: 'enabled', DEBUG_GETTERS: '1', DEBUG_NUMERIC_SEPARATOR: 'TRUE', DEBUG_SHOW_PROXY: 'true', DEBUG_MAX_STRING_LENGTH: '3' });
console.log(JSON.stringify({ plain, colored, env }, null, 1));
