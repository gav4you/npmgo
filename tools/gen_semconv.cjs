// Generates the Go translation of @opentelemetry/semantic-conventions@1.40.0's constants:
// pkgs/opentelemetry/semantic-conventions (the main entry point, require('@opentelemetry/semantic-conventions')) and
// its incubating package (require('@opentelemetry/semantic-conventions/incubating')), plus each package's
// exports_test.go, the table the parity test checks against the recorded module.
//
// Run: node tools/gen_semconv.cjs <semantic-conventions package dir> pkgs/opentelemetry/semantic-conventions
//
// <package dir> is the `package` directory of opentelemetry__semantic-conventions-1.40.0.tgz, extracted into its own
// new, empty directory (treat the tarball as untrusted data). The script loads the two entry points (only constants
// and template functions), reads build/src/*.js as text to see how each export is written (a literal, a TMP_
// constant, a createConstMap([...]) call, an object literal or a `(key) => \`prefix${key}\`` function) and takes the
// documentation from the .d.ts files. Every value it writes is checked against the loaded module first. It runs
// gofmt -w on what it writes (gofmt must be on PATH).
'use strict';
const fs = require('fs');
const path = require('path');
const { execFileSync } = require('child_process');

const [pkgArg, outArg] = process.argv.slice(2);
if (!pkgArg || !outArg) throw new Error('usage: node tools/gen_semconv.cjs <package dir> <output dir>');
const pkgDir = path.resolve(pkgArg);
const outDir = path.resolve(outArg);
const src = path.join(pkgDir, 'build', 'src');
const version = JSON.parse(fs.readFileSync(path.join(pkgDir, 'package.json'), 'utf8')).version;
if (version !== '1.40.0') throw new Error('expected semantic-conventions 1.40.0, got ' + version);

const mainEntry = require(path.join(src, 'index.js'));
const incubatingEntry = require(path.join(src, 'index-incubating.js'));

// The source files in the order the entry points re-export them (__exportStar: the first definition of a name wins).
const MAIN_FILES = ['trace/SemanticAttributes', 'resource/SemanticResourceAttributes', 'stable_attributes', 'stable_metrics', 'stable_events'];
const STABLE_FILES = ['stable_attributes', 'stable_metrics', 'stable_events'];
const EXPERIMENTAL_FILES = ['experimental_attributes', 'experimental_metrics', 'experimental_events'];
const GO_FILE = {
  'trace/SemanticAttributes': 'trace.go', 'resource/SemanticResourceAttributes': 'resource.go',
  stable_attributes: 'stable_attributes.go', stable_metrics: 'stable_metrics.go', stable_events: 'stable_events.go',
  experimental_attributes: 'experimental_attributes.go', experimental_metrics: 'experimental_metrics.go',
  experimental_events: 'experimental_events.go',
};

// docs reads the JSDoc comment written right before each `export declare const NAME` in a .d.ts file.
function docs(file) {
  const text = fs.readFileSync(path.join(src, file + '.d.ts'), 'utf8');
  const out = new Map();
  const re = /\/\*\*((?:(?!\*\/)[\s\S])*)\*\/\s*export declare const (\w+)/g;
  let m;
  while ((m = re.exec(text))) out.set(m[2], m[1]);
  return out;
}

// parse classifies each `exports.NAME = ...` statement of a .js file (the `exports.A = exports.B = void 0`
// declarations aside), in source order.
function parse(file) {
  const text = fs.readFileSync(path.join(src, file + '.js'), 'utf8');
  const tmps = new Map(); // TMP_ name -> literal text
  for (const m of text.matchAll(/^const (TMP_\w+) = (.*);$/gm)) tmps.set(m[1], m[2]);
  const fns = new Map(); // template functions: name -> prefix
  for (const m of text.matchAll(/^const (\w+) = \(key\) => `([^`$\\]*)\$\{key\}`;$/gm)) fns.set(m[1], m[2]);
  const items = [];
  const re = /^exports\.(\w+) = ([\s\S]*?);$/gm;
  let m;
  while ((m = re.exec(text))) {
    const [, name, rhs] = m;
    if (/^(exports\.\w+ = )*void 0$/.test(rhs) || /^exports\.\w+ = /.test(rhs)) continue;
    let item;
    if (/^TMP_\w+$/.test(rhs)) item = { kind: 'value', tmp: rhs };
    else if (/^('[^'\\]*'|"[^"\\]*"|-?\d+)$/.test(rhs)) item = { kind: 'value' };
    else if (rhs === name && fns.has(name)) item = { kind: 'fn', prefix: fns.get(name) };
    else if (/^\s*\/\*#__PURE__\*\/ \(0, utils_1\.createConstMap\)\(\[([\s\S]*)\]\)$/.test(rhs)) {
      const list = RegExp.$1.split(',').map(s => s.trim()).filter(Boolean);
      for (const t of list) if (!tmps.has(t)) throw new Error(`${file}: ${name}: unknown ${t}`);
      item = { kind: 'constmap', tmps: list };
    } else if (/^\{([\s\S]*)\}$/.test(rhs)) {
      const entries = RegExp.$1.split(',').map(s => s.trim()).filter(Boolean).map(e => {
        const mm = /^(\w+): (TMP_\w+)$/.exec(e);
        if (!mm || !tmps.has(mm[2])) throw new Error(`${file}: ${name}: bad entry ${e}`);
        return [mm[1], mm[2]];
      });
      item = { kind: 'object', entries };
    } else throw new Error(`${file}: ${name}: unrecognised form ${rhs.slice(0, 80)}`);
    item.name = name;
    items.push(item);
  }
  return { items, tmps };
}

// goString writes a JavaScript string as a Go string literal (both are checked to round-trip).
function goString(s) {
  const lit = JSON.stringify(s);
  if (JSON.parse(lit) !== s || /[\u2028\u2029]/.test(s)) throw new Error('unexpected string ' + lit);
  return lit;
}

function goNumber(x) {
  if (!Number.isInteger(x) || Object.is(x, -0)) throw new Error('unexpected number ' + x);
  return String(x);
}

// goDoc turns a JSDoc comment into Go comment lines; `@deprecated text` becomes Go's `Deprecated: text` paragraph.
function goDoc(doc) {
  let lines = doc.split('\n').map(l => l.replace(/^\s*\* ?/, '').replace(/^\s*$/, '').replace(/\s+$/, ''));
  while (lines.length && lines[0] === '') lines.shift();
  while (lines.length && lines[lines.length - 1] === '') lines.pop();
  const out = [];
  for (const l of lines) {
    if (l.startsWith('@deprecated')) {
      if (out.length && out[out.length - 1] !== '') out.push('');
      out.push('Deprecated:' + l.slice('@deprecated'.length));
    } else out.push(l);
  }
  return out.map(l => (l ? '// ' + l : '//')).join('\n') + '\n';
}

function header(pkg, sources) {
  return `// Code generated by tools/gen_semconv.cjs from @opentelemetry/semantic-conventions ${version}; DO NOT EDIT.
//
// Translated to Go by the npmgo contributors from ${sources}.
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ${pkg}
`;
}

// emitFile writes the Go declarations of one JavaScript source file, skipping names an earlier file of the same entry
// point already defined (as __exportStar does), and returns the names it defined.
function emitFile(file, pkg, dir, defined, entry) {
  const { items, tmps } = parse(file);
  const mod = require(path.join(src, file + '.js'));
  const doc = docs(file);
  // TMP_ constants resolve to the exported constant that holds them, when there is one.
  const tmpExport = new Map();
  for (const it of items) if (it.kind === 'value' && it.tmp && !tmpExport.has(it.tmp)) tmpExport.set(it.tmp, it.name);
  const tmpValue = t => {
    const lit = tmps.get(t);
    if (/^'[^'\\]*'$/.test(lit) || /^"[^"\\]*"$/.test(lit)) return lit.slice(1, -1);
    if (/^-?\d+$/.test(lit)) return Number(lit);
    throw new Error(`${file}: ${t}: unexpected literal ${lit}`);
  };
  const tmpRef = t => tmpExport.get(t) || (typeof tmpValue(t) === 'number' ? goNumber(tmpValue(t)) : goString(tmpValue(t)));
  let out = header(pkg, `build/src/${file}.js (documentation from ${file}.d.ts)`) + '\n';
  const names = [];
  let skipped = 0;
  for (const it of items) {
    const name = it.name;
    const v = mod[name];
    if (defined.has(name)) {
      if (entry[name] !== defined.get(name)) throw new Error(`${name}: shadowed export differs`);
      skipped++;
      continue;
    }
    if (entry[name] !== v) throw new Error(`${file}: ${name}: the entry point exports a different value`);
    const d = doc.has(name) ? goDoc(doc.get(name)) : '';
    switch (it.kind) {
      case 'value': {
        if (it.tmp && tmpValue(it.tmp) !== v) throw new Error(`${name}: TMP value mismatch`);
        if (typeof v === 'string') out += `${d}const ${name} = ${goString(v)}\n\n`;
        else if (typeof v === 'number') out += `${d}const ${name} = ${goNumber(v)}\n\n`;
        else throw new Error(`${name}: unexpected ${typeof v}`);
        break;
      }
      case 'fn': {
        if (typeof v !== 'function' || v.length !== 1) throw new Error(`${name}: not a one-argument function`);
        for (const k of ['', 'x', 'content-type']) if (v(k) !== it.prefix + k) throw new Error(`${name}: template mismatch`);
        out += `${d}func ${name}(key string) string { return ${goString(it.prefix)} + key }\n\n`;
        break;
      }
      case 'constmap': {
        // Check the translation of createConstMap against the object the module built.
        const want = {};
        for (const t of it.tmps) { const val = tmpValue(t); if (val) want[String(val).toUpperCase().replace(/[-.]/g, '_')] = val; }
        if (JSON.stringify(Object.entries(want)) !== JSON.stringify(Object.entries(v))) throw new Error(`${name}: createConstMap mismatch`);
        if (it.tmps.some(t => typeof tmpValue(t) !== 'string')) throw new Error(`${name}: non-string createConstMap value`);
        out += `${d}var ${name} = createConstMap(\n${it.tmps.map(t => '\t' + tmpRef(t) + ',\n').join('')})\n\n`;
        break;
      }
      case 'object': {
        const want = Object.fromEntries(it.entries.map(([k, t]) => [k, tmpValue(t)]));
        if (JSON.stringify(Object.entries(want)) !== JSON.stringify(Object.entries(v))) throw new Error(`${name}: object mismatch`);
        const numeric = it.entries.every(([, t]) => typeof tmpValue(t) === 'number');
        if (!numeric && !it.entries.every(([, t]) => typeof tmpValue(t) === 'string')) throw new Error(`${name}: mixed object`);
        out += `${d}var ${name} = newConstMap[${numeric ? 'float64' : 'string'}](\n${it.entries.map(([k, t]) => `\t${goString(k)}, ${tmpRef(t)},\n`).join('')})\n\n`;
        break;
      }
    }
    defined.set(name, v);
    names.push(name);
  }
  fs.writeFileSync(path.join(dir, GO_FILE[file]), out);
  return { names, skipped };
}

// exportsTable writes exports_test.go: every JavaScript export name and the Go declaration translating it.
function exportsTable(pkg, dir, names, varName) {
  let out = header(pkg, 'the export list of the entry point') + '\n';
  out += `// ${varName} maps each export of the JavaScript entry point to its Go translation.\nvar ${varName} = map[string]any{\n`;
  for (const n of names) out += `\t${goString(n)}: ${n},\n`;
  out += '}\n';
  fs.writeFileSync(path.join(dir, 'exports_test.go'), out);
}

// --- The main entry point.
const mainDefined = new Map();
const report = {};
for (const f of MAIN_FILES) report[f] = emitFile(f, 'semanticconventions', outDir, mainDefined, mainEntry);
const mainNames = Object.keys(mainEntry);
if (mainNames.length !== mainDefined.size || mainNames.some(n => !mainDefined.has(n))) throw new Error('main entry: export list mismatch');
exportsTable('semanticconventions', outDir, [...mainDefined.keys()], 'jsExports');

// --- The incubating entry point: the stable files' exports are re-exports of the main package's.
const incDir = path.join(outDir, 'incubating');
fs.mkdirSync(incDir, { recursive: true });
const incDefined = new Map();
let stable = header('incubating', 'build/src/index-incubating.js, which re-exports stable_attributes.js, stable_metrics.js and stable_events.js') + `
import semanticconventions "github.com/gav4you/npmgo/pkgs/opentelemetry/semantic-conventions"

`;
for (const f of STABLE_FILES) {
  for (const it of parse(f).items) {
    const v = incubatingEntry[it.name];
    if (incDefined.has(it.name)) continue;
    if (v !== mainEntry[it.name]) throw new Error(`incubating ${it.name}: not the stable export`);
    if (it.kind === 'fn') stable += `// ${it.name} is [semanticconventions.${it.name}].\nfunc ${it.name}(key string) string { return semanticconventions.${it.name}(key) }\n\n`;
    else if (it.kind === 'value') stable += `// ${it.name} is [semanticconventions.${it.name}].\nconst ${it.name} = semanticconventions.${it.name}\n\n`;
    else throw new Error(`incubating ${it.name}: unexpected ${it.kind}`);
    incDefined.set(it.name, v);
  }
}
fs.writeFileSync(path.join(incDir, 'stable.go'), stable);
for (const f of EXPERIMENTAL_FILES) report[f] = emitFile(f, 'incubating', incDir, incDefined, incubatingEntry);
const incNames = Object.keys(incubatingEntry);
if (incNames.length !== incDefined.size || incNames.some(n => !incDefined.has(n))) throw new Error('incubating entry: export list mismatch');
exportsTable('incubating', incDir, [...incDefined.keys()], 'jsExports');

const files = [...MAIN_FILES.map(f => path.join(outDir, GO_FILE[f])), path.join(outDir, 'exports_test.go'),
  path.join(incDir, 'stable.go'), ...EXPERIMENTAL_FILES.map(f => path.join(incDir, GO_FILE[f])), path.join(incDir, 'exports_test.go')];
execFileSync('gofmt', ['-w', ...files]);
for (const [f, r] of Object.entries(report)) console.log(`${f}: ${r.names.length} declarations${r.skipped ? `, ${r.skipped} shadowed` : ''}`);
console.log(`main entry: ${mainDefined.size} exports; incubating entry: ${incDefined.size} exports`);
