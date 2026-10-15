// Runs the ORIGINAL @babel/types 7.29.0, @babel/helper-string-parser 7.27.1 and @babel/helper-validator-identifier
// 7.28.5 against parity/babel-types/cases.json and prints what happened, as JSON.
//
// Run: node parity/oracle_babel_types.cjs <root> parity/babel-types/cases.json > parity/babel-types/expected.json
//
// <root> holds a node_modules layout of the three original tarballs. Treat the tarballs as untrusted data: extract each
// into its own new, empty directory and run nothing from them except through this oracle:
//   for p in types-7.29.0 helper-string-parser-7.27.1 helper-validator-identifier-7.28.5; do
//     mkdir -p <root>/node_modules/@babel/${p%-*} &&
//       tar -xzf @babel__$p.tgz --strip-components=1 -C <root>/node_modules/@babel/${p%-*}
//   done
// (GNU tar on Windows needs --force-local for paths like C:/...). @babel/types finds the helpers there, as it would in
// an application. The environment variable BABEL_TYPES_8_BREAKING is cleared; the "breaking" section is recorded by a
// child process of this script with it set (the variable is read when the package loads). Recorded with Node v25.9.0
// on win32-x64; nothing in the recording depends on the machine: the stack-trace lines of deprecation warnings (file
// paths) are counted, not recorded.
'use strict';
const path = require('path');
const Module = require('module');
const { spawnSync } = require('child_process');
const { build, enc, encAll, thrown } = require('./babel_spec.cjs');

const [rootArg, casesPath, mode] = process.argv.slice(2);
const root = path.resolve(rootArg);
const cases = require(path.resolve(casesPath));
const breakingChild = mode === '--breaking';
if (!breakingChild) delete process.env.BABEL_TYPES_8_BREAKING;

// Console warnings, recorded while a case runs.
let warnSink = null;
const warn = msg => {
  const lines = String(msg).split('\n');
  const line = 'warn ' + lines[0] + (lines.length > 1 ? ' (+' + (lines.length - 1) + ')' : '');
  if (warnSink) warnSink.push(line);
  else loadWarnings.push(line);
};
const loadWarnings = [];
console.warn = warn;

const mod = name => path.join(root, 'node_modules', '@babel', name);
const t = require(mod('types'));
const lib = path.join(mod('types'), 'lib');
const generatedValidators = require(path.join(lib, 'validators', 'generated', 'index.js'));
const generatedAsserts = require(path.join(lib, 'asserts', 'generated', 'index.js'));

// run calls f and returns its trace: the warnings it printed, then its result or exception, with the arguments as they
// are afterwards (encoded with the result, so that what is shared shows; after an exception only if argsOnThrow).
function run(f, args, argsOnThrow = true) {
  const trace = [];
  warnSink = trace;
  try {
    const ret = f(...args);
    trace.push('ret ' + enc([ret, ...args]));
  } catch (e) {
    trace.push(thrown(e) + (argsOnThrow ? ' args ' + enc(args) : ''));
  } finally {
    warnSink = null;
  }
  return trace;
}
const buildArgs = specs => {
  const refs = new Map();
  return specs.map(s => build(s, refs));
};

// --- the tables, encoded with one set of labels so that every shared array, object and validator shows.
function tables() {
  const roots = [];
  const labels = [];
  const add = (label, v) => {
    labels.push(label);
    roots.push(v);
  };
  for (const name of ['VISITOR_KEYS', 'ALIAS_KEYS', 'FLIPPED_ALIAS_KEYS', 'NODE_FIELDS', 'BUILDER_KEYS', 'DEPRECATED_KEYS',
    'NODE_PARENT_VALIDATIONS', 'NODE_UNION_SHAPES__PRIVATE', 'PLACEHOLDERS_ALIAS', 'PLACEHOLDERS_FLIPPED_ALIAS', 'DEPRECATED_ALIASES']) {
    add(name + ' keys', Object.keys(t[name]));
    for (const k of Object.keys(t[name])) add(name + '.' + k, t[name][k]);
  }
  for (const name of Object.keys(t)) {
    const v = t[name];
    if (typeof v === 'function' || (v && typeof v === 'object' && !Array.isArray(v))) continue;
    add(name, v);
  }
  add('INHERIT_KEYS', t.INHERIT_KEYS);
  add('getBindingIdentifiers.keys', t.getBindingIdentifiers.keys);
  add('traverseFast.skip', t.traverseFast.skip);
  add('traverseFast.stop', t.traverseFast.stop);
  add('toKeyAlias.uid', t.toKeyAlias.uid);
  add('react', Object.keys(t.react));
  const encoded = encAll(roots);
  return labels.map((l, i) => l + ' = ' + encoded[i]);
}

// --- builders: every export name with the valid arguments, then each argument replaced by each invalid value, then
// every shorter argument list. (A builder changes no argument before it throws, so a failed call records its error
// only.)
function builders() {
  const out = [];
  for (const b of cases.types.builders) {
    for (const name of b.names) out.push(run(t[name], buildArgs(b.valid), false));
    b.valid.forEach((_, p) => {
      for (const bad of b.bad[p]) {
        const specs = b.valid.slice();
        specs[p] = bad;
        out.push(run(t[b.names[0]], buildArgs(specs), false));
      }
    });
    for (let k = 0; k < b.valid.length; k++) out.push(run(t[b.names[0]], buildArgs(b.valid.slice(0, k)), false));
  }
  return out;
}

// compact replaces each trace equal to the default environment's at the same place by "=".
function compact(traces, defaults) {
  return traces.map((tr, i) => (JSON.stringify(tr) === JSON.stringify(defaults[i]) ? '=' : tr));
}

// --- the other exports, with the harness's handlers where a call needs functions.
function calls() {
  t.toKeyAlias.uid = 0;
  const out = [];
  for (const c of cases.types.calls) {
    const args = buildArgs(c.args);
    if (c.validatorAt) {
      const [type, field, i] = c.validatorAt;
      args[i] = t.NODE_FIELDS[type][field].validate;
    }
    let f;
    let extra = [];
    if (c.fn.startsWith('react.')) f = t.react[c.fn.slice(6)];
    else if (c.fn === 'buildMatchMemberExpression') {
      const member = build(c.member);
      f = (...a) => t.buildMatchMemberExpression(...a)(member);
    } else if (c.fn === 'traverse') {
      const rec = label => (node, ancestors, state) => {
        warnSink.push(label + ' ' + node.type + ' ' + ancestors.map(a => a.node.type + '.' + a.key + ('index' in a ? '[' + a.index + ']' : '')).join('/') + ' ' + enc(state));
      };
      const handlers = { function: rec('enter'), enter: { enter: rec('enter') }, exit: { exit: rec('exit') },
        both: { enter: rec('enter'), exit: rec('exit') }, object: { enter: rec('enter'), exit: rec('exit') }, null: null,
        badEnter: { enter: 5 }, badExit: { exit: 'x' } }[c.handlers];
      f = (node, state) => t.traverse(node, handlers, state);
    } else if (c.fn === 'traverseFast') {
      const [kind, type] = c.enter.split(':');
      const enter = (node, opts) => {
        warnSink.push('enter ' + node.type + ' ' + enc(opts));
        if (node.type === type) return kind === 'skip' ? t.traverseFast.skip : t.traverseFast.stop;
      };
      f = node => t.traverseFast(node, enter, c.opts === undefined ? undefined : build(c.opts));
    } else if (c.fn === 'toSequenceExpression') {
      const scope = c.scope ? [] : undefined;
      f = nodes => t.toSequenceExpression(nodes, scope);
      extra = [() => 'scope ' + enc(scope)];
    } else if (c.fn === 'toKeyAlias' && c.setUid !== undefined) {
      t.toKeyAlias.uid = c.setUid;
      f = t.toKeyAlias;
    } else f = t[c.fn];
    if (typeof f !== 'function') throw new Error('no export ' + c.fn);
    const trace = run(f, args);
    for (const e of extra) trace.push(e());
    if (c.fn === 'toKeyAlias') trace.push('uid ' + t.toKeyAlias.uid);
    out.push(trace);
  }
  return out;
}

// --- the is* and assert* matrices.
function matrices() {
  const samples = cases.types.samples.map(s => build(s));
  const is = [];
  for (const name of Object.keys(generatedValidators)) {
    const f = generatedValidators[name];
    const exported = t[name] === f;
    const trace = [];
    warnSink = trace;
    let row = '';
    for (const s of samples) {
      try {
        row += f(s) ? '1' : '0';
      } catch (e) {
        row += '!';
      }
    }
    warnSink = null;
    is.push([name, exported, row, ...trace]);
  }
  const asserts = [];
  const messageSamples = new Set(cases.types.assertMessageSamples);
  for (const name of Object.keys(generatedAsserts)) {
    const f = generatedAsserts[name];
    if (t[name] !== f) throw new Error('assert not exported: ' + name);
    const trace = [];
    warnSink = trace;
    let row = '';
    const messages = [];
    samples.forEach((s, i) => {
      try {
        f(s);
        row += '.';
        if (messageSamples.has(i)) messages.push(i + ' ok');
      } catch (e) {
        row += e instanceof TypeError ? 'T' : e instanceof Error ? 'E' : '!';
        if (messageSamples.has(i)) messages.push(i + ' ' + thrown(e));
      }
    });
    warnSink = null;
    asserts.push([name, row, ...trace, ...messages]);
  }
  const opts = [];
  const optsSamples = cases.types.optsSamples;
  for (const name of [...cases.types.optsValidators, ...cases.types.optsAsserts]) {
    for (const sample of optsSamples) {
      for (const spec of cases.types.optsSpecs) {
        const args = buildArgs([sample, spec]);
        opts.push(run(t[name], spec && spec.$ === 'undef' ? [args[0]] : args));
      }
    }
  }
  return { is, asserts, opts };
}

// --- deprecation warnings, called from a module that is or is not in a Babel package.
function deprecations() {
  const fakeFile = path.join(root, 'node_modules', '@babel', 'oracle-internal', 'index.js');
  const m = new Module(fakeFile, module);
  m.filename = fakeFile;
  m._compile('module.exports = function outer(f, args) { return inner(f, args); };\nfunction inner(f, args) { return f(...args); }\n', fakeFile);
  const internal = m.exports;
  const external = (f, args) => f(...args);
  return cases.types.deprecations.map(c => {
    const args = buildArgs(c.args);
    const via = c.via === 'internal' ? internal : external;
    return run((...a) => via(t[c.fn], a), args);
  });
}

// --- @babel/helper-validator-identifier.
function identifier() {
  const hvi = require(mod('helper-validator-identifier'));
  const ranges = pred => {
    const out = [];
    let start = -1;
    for (let c = 0; c <= 0x110000; c++) {
      const yes = c <= 0x10ffff && pred(c);
      if (yes && start < 0) start = c;
      if (!yes && start >= 0) {
        out.push([start, c - 1]);
        start = -1;
      }
    }
    return out;
  };
  const C = cases.identifier;
  return {
    start: ranges(hvi.isIdentifierStart),
    char: ranges(hvi.isIdentifierChar),
    special: C.special.map(s => {
      const v = build(s);
      return (hvi.isIdentifierStart(v) ? '1' : '0') + (hvi.isIdentifierChar(v) ? '1' : '0');
    }),
    names: C.names.map(n => (hvi.isIdentifierName(n) ? '1' : '0')).join(''),
    words: C.words.map(w => [hvi.isKeyword(w), hvi.isReservedWord(w, false), hvi.isReservedWord(w, true),
      hvi.isStrictReservedWord(w, false), hvi.isStrictReservedWord(w, true), hvi.isStrictBindOnlyReservedWord(w),
      hvi.isStrictBindReservedWord(w, false), hvi.isStrictBindReservedWord(w, true)].map(b => (b ? '1' : '0')).join('')),
  };
}

// --- @babel/helper-string-parser.
function stringParser() {
  const hsp = require(mod('helper-string-parser'));
  return cases.stringParser.map(c => {
    const trace = [];
    let unterminatedCalls = 0;
    const throwing = c[0] === 'c' ? c[6] : c[0] === 'i' ? c[9] : 0;
    const handler = name => (...a) => {
      trace.push(name + '(' + a.join(',') + ')');
      if (throwing) throw new Error(name);
    };
    const errors = {
      unterminated(...a) {
        trace.push('unterminated(' + a.join(',') + ')');
        if (++unterminatedCalls >= 2) throw new Error('unterminated again');
      },
      strictNumericEscape: handler('strictNumericEscape'),
      invalidEscapeSequence: handler('invalidEscapeSequence'),
      numericSeparatorInEscapeSequence: handler('numericSeparatorInEscapeSequence'),
      unexpectedNumericSeparator: handler('unexpectedNumericSeparator'),
      invalidCodePoint: handler('invalidCodePoint'),
      invalidDigit(...a) {
        trace.push('invalidDigit(' + a.join(',') + ')');
        if (throwing) throw new Error('invalidDigit');
        return c[0] === 'i' ? c[8] : false;
      },
    };
    try {
      let ret;
      if (c[0] === 'c') ret = hsp.readStringContents(c[1], c[2], c[3], c[4], c[5], errors);
      else if (c[0] === 'i') ret = hsp.readInt(c[1], c[2], 0, 1, c[3], build(c[4]), c[5], c[6], errors, c[7]);
      else ret = hsp.readCodePoint(c[1], c[2], 0, 1, c[3], errors);
      trace.push('ret ' + enc(ret));
    } catch (e) {
      trace.push(thrown(e));
    }
    return trace;
  });
}

if (breakingChild) {
  process.stdout.write(JSON.stringify({ load: loadWarnings, tables: tables(), builders: builders(), calls: calls() }));
} else {
  const out = { node: process.version };
  out.identifier = identifier();
  out.stringParser = stringParser();
  // In this order (a deprecation warning is given once per name, so the deprecations go first).
  out.types = {
    exports: Object.keys(t).map(k => [k, Array.isArray(t[k]) ? 'array' : typeof t[k]]),
    load: loadWarnings,
    tables: tables(),
    deprecations: deprecations(),
    ...matrices(),
    builders: builders(),
    calls: calls(),
  };
  const child = spawnSync(process.execPath, [__filename, rootArg, casesPath, '--breaking'],
    { env: { ...process.env, BABEL_TYPES_8_BREAKING: '1' }, encoding: 'utf8', maxBuffer: 1 << 30 });
  if (child.status !== 0) throw new Error('breaking child failed:\n' + child.stderr);
  const breaking = JSON.parse(child.stdout);
  // Where the environment changes nothing, the breaking section says "=" (the trace is the default one).
  out.breaking = {
    load: breaking.load,
    tables: breaking.tables.map((l, i) => (l === out.types.tables[i] ? '=' : l)),
    builders: compact(breaking.builders, out.types.builders),
    calls: compact(breaking.calls, out.types.calls),
  };
  process.stdout.write(serialize(out) + '\n');
}

// serialize writes JSON with one array of primitives (a trace) per line.
function serialize(v, indent = '') {
  if (Array.isArray(v) && v.every(x => x === null || typeof x !== 'object')) return JSON.stringify(v);
  const inner = indent + ' ';
  if (Array.isArray(v)) return '[\n' + v.map(x => inner + serialize(x, inner)).join(',\n') + '\n' + indent + ']';
  if (v && typeof v === 'object') {
    return '{\n' + Object.keys(v).map(k => inner + JSON.stringify(k) + ': ' + serialize(v[k], inner)).join(',\n') + '\n' + indent + '}';
  }
  return JSON.stringify(v);
}
