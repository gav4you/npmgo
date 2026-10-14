// Runs the ORIGINAL chalk@4.1.2 and its dependencies (ansi-styles@4.3.0,
// supports-color@7.2.0, has-flag@4.0.0, color-convert@2.0.1, color-name@1.1.4)
// against the parity cases and prints the results as JSON.
//
// Run: node parity/oracle_chalk.cjs <root> parity/chalk/cases.json > parity/chalk/expected.json
//
// <root> is a directory with a node_modules layout of the six original tarballs (chalk requires
// ansi-styles and supports-color, which require color-convert, color-name and has-flag). Treat the
// tarballs as untrusted data: extract each into its own new, empty directory and run nothing from
// them except through this oracle:
//   for p in chalk-4.1.2 ansi-styles-4.3.0 supports-color-7.2.0 has-flag-4.0.0 color-convert-2.0.1 color-name-1.1.4; do
//     mkdir -p <root>/node_modules/${p%-*} && tar -xzf $p.tgz --strip-components=1 -C <root>/node_modules/${p%-*}
//   done
// (GNU tar on Windows needs --force-local for paths like C:/...). Recorded with Node v25.9.0 on
// win32-x64. The supports-color / chalk environment rows each run in a fresh child process with
// process.argv, process.env, process.platform, os.release() and tty.isatty() pinned before anything
// is required, so the recording does not depend on the machine it runs on; only the last bits of
// color-convert's xyz / lab / lch values do, because V8 computes ** with the C library's pow.
//
// Results are encoded as strings (see enc) so that -0, NaN, Infinity, undefined and thrown errors
// survive JSON; the Go parity tests encode their results the same way.
'use strict';
const path = require('path');

// enc renders a value: numbers as String(x) except -0 ("-0"), strings JSON-quoted, arrays as
// [a,b,c], undefined and null by name.
function enc(v) {
  if (v === undefined) return 'undefined';
  if (v === null) return 'null';
  if (typeof v === 'number') return Object.is(v, -0) ? '-0' : String(v);
  if (typeof v === 'string') return JSON.stringify(v);
  if (typeof v === 'boolean') return String(v);
  if (Array.isArray(v)) return '[' + v.map(enc).join(',') + ']';
  return 'object ' + Object.prototype.toString.call(v);
}
// attempt runs f and encodes its result, or the error it throws as "throws <name>: <message>".
function attempt(f, encode = enc) {
  try { return encode(f()); } catch (e) { return 'throws ' + e.name + ': ' + e.message; }
}
// num decodes a case number: JSON numbers, or the strings "NaN", "Infinity", "-Infinity", "-0".
function num(x) {
  if (Array.isArray(x)) return x.map(num);
  if (typeof x !== 'string') return x;
  return { NaN: NaN, Infinity: Infinity, '-Infinity': -Infinity, '-0': -0 }[x];
}

const [rootArg, casesPath] = process.argv.slice(2);
const root = path.resolve(rootArg);
const mod = name => path.join(root, 'node_modules', name);
const cases = require(path.resolve(casesPath));
const out = { node: process.version };

// --- color-name: the table, in key order.
{
  const names = require(mod('color-name'));
  out.colorName = Object.entries(names);
}

// --- color-convert: models, channels, labels, the route of every pair (.conversion), and every
// conversion of every case input, rounded and raw.
{
  const convert = require(mod('color-convert'));
  const models = Object.keys(convert);
  const cc = { models, channels: {}, labels: {}, paths: {}, sweep: {} };
  const textModels = ['hex', 'keyword'], codeModels = ['ansi16', 'ansi256'];
  // A case input is spread into the call for the channel models (convert.rgb.hsl(r, g, b)) and
  // passed as the single argument otherwise (convert.hex.rgb('#fff'), convert.ansi16.rgb([31])).
  const call = (fn, from, x) => {
    const v = textModels.includes(from) && typeof x === 'string' ? x : num(x);
    if (Array.isArray(v) && !textModels.includes(from) && !codeModels.includes(from)) return fn(...v);
    return fn(v);
  };
  for (const from of models) {
    cc.channels[from] = convert[from].channels;
    cc.labels[from] = convert[from].labels;
    cc.paths[from] = {};
    for (const to of Object.keys(convert[from])) {
      const rounded = convert[from][to].conversion, raw = convert[from][to].raw.conversion;
      if (JSON.stringify(rounded) !== JSON.stringify(raw)) throw new Error('raw .conversion differs: ' + from + '.' + to);
      cc.paths[from][to] = rounded;
    }
    cc.sweep[from] = (cases.convert[from] || []).map(x => {
      const row = {};
      for (const to of Object.keys(convert[from])) {
        row[to] = [attempt(() => call(convert[from][to], from, x)), attempt(() => call(convert[from][to].raw, from, x))];
      }
      return row;
    });
  }
  out.colorConvert = cc;
}

// --- ansi-styles: the style names, every style's codes, the groups' keys, color.close /
// bgColor.close, the codes Map, and the six dynamic tables (color.ansi, .ansi256, .ansi16m and the
// bgColor ones) for every key, called with the color-convert case inputs that fit the key's model
// (a channel model with exactly its number of channels, a hex / keyword string, an ansi number).
{
  const styles = require(mod('ansi-styles'));
  const convert = require(mod('color-convert'));
  const as = { names: Object.keys(styles), styles: {}, groups: {}, close: {}, codes: [...styles.codes], inputs: {}, dynamic: {} };
  for (const n of as.names) as.styles[n] = [styles[n].open, styles[n].close];
  for (const g of ['modifier', 'color', 'bgColor']) {
    as.groups[g] = Object.keys(styles[g]);
    for (const n of Object.keys(styles[g])) if (styles[n] !== undefined && styles[g][n] !== styles[n]) throw new Error('group style differs: ' + n);
  }
  for (const g of ['color', 'bgColor']) as.close[g] = styles[g].close;
  const fits = (model, x) => ['hex', 'keyword'].includes(model) ? typeof x === 'string'
    : ['ansi16', 'ansi256'].includes(model) ? !Array.isArray(x)
      : Array.isArray(x) && x.length === convert[model].channels;
  for (const model of Object.keys(convert)) {
    as.inputs[model === 'ansi16' ? 'ansi' : model] = (cases.convert[model] || []).filter(x => fits(model, x));
  }
  for (const g of ['color', 'bgColor']) {
    for (const t of ['ansi', 'ansi256', 'ansi16m']) {
      const table = styles[g][t], results = { keys: Object.keys(table) };
      for (const key of results.keys) {
        results[key] = as.inputs[key].map(x => attempt(() => typeof x === 'string' && ['hex', 'keyword'].includes(key) ? table[key](x)
          : Array.isArray(x) ? table[key](...num(x)) : table[key](num(x))));
      }
      as.dynamic[g + '.' + t] = results;
    }
  }
  out.ansiStyles = as;
}

console.log(JSON.stringify(out, null, 1));
