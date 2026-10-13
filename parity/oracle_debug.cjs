// Runs the ORIGINAL debug@4.4.3 (Node build) against the parity cases in two env modes.
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
function run(env) {
  const debug = load(env);
  const out = {};
  out.colors = c.colors.map(ns => debug.selectColor(ns));
  out.enabled = c.patterns.map(p => { debug.enable(p); return c.names.map(n => debug.enabled(n)); });
  out.disable = c.patterns.map(p => { debug.enable(p); return debug.disable(); });
  debug.enable('*');
  out.logs = c.logs.map(k => {
    const lines = [];
    Date.now = () => 1700000000000; // fixed clock: diff = 0
    const d = debug(k.ns);
    d.log = (...a) => lines.push(require('util').formatWithOptions(d.inspectOpts || debug.inspectOpts, ...a));
    d(...build(k.args));
    Date.now = realNow;
    return lines;
  });
  const parent = debug('parent');
  out.extend = [parent.extend('child').namespace, parent.extend('child', '/').namespace, parent.extend('child', '').namespace];
  debug.formatters.h = v => Buffer.from(String(v)).toString('hex');
  const lines = [];
  const d = debug('custom'); d.log = (...a) => lines.push(require('util').format(...a));
  d.useColors = false;
  const savedHide = debug.inspectOpts.hideDate; debug.inspectOpts.hideDate = true;
  d('hex %h and %s', 'AB', 'tail');
  debug.inspectOpts.hideDate = savedHide;
  out.custom = lines;
  delete debug.formatters.h;
  const d2 = debug('override'); debug.enable(''); d2.enabled = true; out.override = d2.enabled; debug.enable('*'); d2.enabled = false; out.override2 = d2.enabled;
  return out;
}
const plain = run({ DEBUG_COLORS: 'no', DEBUG_HIDE_DATE: 'yes' });
const colored = run({ DEBUG_COLORS: 'true' });
console.log(JSON.stringify({ plain, colored }, null, 1));
