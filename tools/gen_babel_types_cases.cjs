// Generates parity/babel-types/cases.json: the cases parity/oracle_babel_types.cjs records for @babel/types 7.29.0,
// @babel/helper-string-parser 7.27.1 and @babel/helper-validator-identifier 7.28.5.
//
// Run from the repository root: node tools/gen_babel_types_cases.cjs <root> > parity/babel-types/cases.json
// <root> holds the node_modules layout of the original tarballs (see the oracle). The builder cases are derived from
// the original's field definitions: for each builder a valid argument list (checked by calling the original builder),
// every argument replaced in turn by a list of invalid values, and every shorter argument list. Everything is
// deterministic (no clock, a seeded generator), so re-running reproduces the file byte for byte.
'use strict';
const fs = require('fs');
const path = require('path');
const { build } = require('../parity/babel_spec.cjs');

const root = path.resolve(process.argv[2]);
const lib = path.join(root, 'node_modules', '@babel', 'types', 'lib');
delete process.env.BABEL_TYPES_8_BREAKING;
const t = require(path.join(root, 'node_modules', '@babel', 'types'));

const U = { $: 'undef' };
const num = v => ({ $: 'num', v });
const big = v => ({ $: 'big', v: String(v) });
const ref = id => ({ $: 'ref', id });
const obj = (id, props, syms) => (syms ? { $: 'obj', id, props, syms } : { $: 'obj', id, props });
const arr = (id, items) => ({ $: 'arr', id, items });
const sym = (d, id) => (id ? { $: 'sym', d, id } : { $: 'sym', d });
const symfor = k => ({ $: 'symfor', k });
const N = (type, props = {}) => ({ type, ...props });

// A small deterministic generator (mulberry32).
function rng(seed) {
  return () => {
    seed |= 0;
    seed = (seed + 0x6d2b79f5) | 0;
    let x = Math.imul(seed ^ (seed >>> 15), 1 | seed);
    x = (x + Math.imul(x ^ (x >>> 7), 61 | x)) ^ x;
    return ((x ^ (x >>> 14)) >>> 0) / 4294967296;
  };
}

// ================================================================== helper-validator-identifier
const identifier = {
  special: [-1, 0, 36, 36.5, 48, 57.5, 65, 65.5, 90.5, 95, 97, 122.5, 169.5, 170, 170.5, 0xd800, 0xdfff, 0xfeff, 0xffff,
    65535.5, 0x10000, 0x10000 + 0.5, 0x1d400, 0x2fa1d, 0x2fa1e, 0x10ffff, 0x110000, 1e21,
    num('NaN'), num('Infinity'), num('-Infinity'), num('-0')],
  names: ['', 'a', 'A', '$', '_', '0', 'a0', '0a', 'a-b', 'abc', 'café', 'ünïcödé', 'Ωmega', 'x‌', 'x‍', '‌x',
    'ℹ', '℘', '゛', 'x゛', '𐐀', '𐐀x', 'x𐐀', '😀', 'a😀', '\u{1d400}', '\u{2fa1d}', '\u{e0100}', 'a\u{e0100}', 'class',
    'let', 'yield', 'await', 'ª', '·', 'a·', '̀', 'à', '　', 'a b', 'a\tb', '﻿', 'ab ',
    '٣', 'a٣', 'ⅼ', 'x\u{11000}', '\u{11000}'],
  words: ['', 'break', 'case', 'catch', 'continue', 'debugger', 'default', 'do', 'else', 'finally', 'for', 'function', 'if',
    'return', 'switch', 'throw', 'try', 'var', 'const', 'while', 'with', 'new', 'this', 'super', 'class', 'extends',
    'export', 'import', 'null', 'true', 'false', 'in', 'instanceof', 'typeof', 'void', 'delete', 'implements',
    'interface', 'let', 'package', 'private', 'protected', 'public', 'static', 'yield', 'eval', 'arguments', 'await',
    'enum', 'async', 'of', 'get', 'set', 'undefined', 'NaN', 'Break', 'constructor', 'toString', '__proto__', 'hasOwnProperty'],
};

// ================================================================== helper-string-parser
// Cases: ["c", type, input, pos, lineStart, curLine, throwing] for readStringContents, ["i", input, pos, radix, len,
// forceLen, allowNumSeparator, bailOnError, invalidDigit's answer, throwing] for readInt (lineStart 0, curLine 1), and
// ["p", input, pos, throwOnInvalid] for readCodePoint. The errors object records every call; with throwing set, every
// handler but unterminated throws, as @babel/parser's do. unterminated throws on its second call (a newline in a
// string literal calls it without moving on).
const stringParser = [];
{
  const inputs = [
    '', 'abc', 'abc"', "abc'", 'abc`', 'a${b}`', 'a$b`', 'a$`', '$', '${', 'a\nb`', 'a\rb`', 'a\r\nb`', 'a b`',
    'a b"', 'a\nb"', "a\rb'", '\\n\\r\\t\\b\\v\\f`', '\\x41`', '\\x4`', '\\x`', '\\xg1`', '\\x_1`', '\\u0041`',
    '\\u004`', '\\u{41}`', '\\u{1F600}`', '\\u{110000}`', '\\u{}`', '\\u{41`', '\\u{4_1}`', '\\u00_41`', '\\uD83D\\uDE00`',
    '\\uD83D`', '\\uDE00x`', '\\0`', '\\00`', '\\01`', '\\08`', '\\09`', '\\1`', '\\12`', '\\377`', '\\400`', '\\777`',
    '\\8`', '\\9`', '\\a`', '\\"`', "\\'`", '\\\\`', '\\\n`', '\\\r\n`', '\\\r`', '\\ `', '\\', 'a\\', '\\x', '\\u',
    '\\u{', '\\0"', '\\08"', '\\1"', '\\8"', '\\x4"', '\\u{110000}"', '\\u{41}"', 'tail\\u0062`', '\\x41\\x42`',
    'a\\u0041\\x`', 'x\\8\\9`', 'é\\u00e9`', '😀\\u{1F600}`', '\\u{0000000041}`', '\\u{FFFFFFFFFFFFFFFFFFFF}`',
    '\\u{10FFFF}`', 'ab${', 'a\\${`', 'a\\`b`',
  ];
  const types = ['template', 'double', 'single'];
  for (const input of inputs) {
    for (const type of types) {
      stringParser.push(['c', type, input, 0, 0, 1, 0]);
    }
  }
  // Handlers that throw (as @babel/parser's do), and invalidDigit answering true.
  for (const input of ['\\x4"', '\\u{110000}"', '\\08"', '\\1"', '\\8"', '\\u{4_1}"', '\\x_1"', '\\u00_41"', 'a\nb"']) {
    stringParser.push(['c', 'double', input, 0, 0, 1, 1]);
  }
  // Start positions and line numbers.
  for (const [input, pos] of [['ab"cd"', 3], ['a\nb\nc`', 2], ['xx\\ny`', 2], ['x`', 5], ['abc`', -1]]) {
    stringParser.push(['c', 'template', input, pos, 1, 7, 0]);
  }
  // A fuzz of short strings over the characters that matter.
  const alphabet = ['\\', 'x', 'u', '{', '}', '0', '1', '4', '7', '8', '9', 'a', 'F', 'g', '_', '"', "'", '`', '$', '\n', '\r', ' ', 'é', '😀', 'D'];
  const r = rng(20261015);
  for (let n = 0; n < 1500; n++) {
    const len = 1 + Math.floor(r() * 8);
    let s = '';
    for (let i = 0; i < len; i++) s += alphabet[Math.floor(r() * alphabet.length)];
    stringParser.push(['c', types[n % 3], s, 0, 0, 1, 0]);
  }
  // readInt.
  const intInputs = ['123', '1_000', '1__0', '_1', '1_', '0x', 'ff', 'FF', 'fg', '777', '778', '1012', '9', 'zz', '1_.0', '1_e5', '0b1_0', 'abc', ''];
  for (const input of intInputs) {
    for (const radix of [2, 8, 10, 16]) {
      for (const [len, forceLen] of [[U, false], [2, false], [2, true], [0, false]]) {
        for (const allow of [false, true, 'bail']) {
          for (const bail of [false, true]) {
            stringParser.push(['i', input, 0, radix, len, forceLen, allow, bail, radix === 8, 0]);
          }
        }
      }
    }
  }
  stringParser.push(['i', '12345678901234567890', 0, 10, U, false, false, false, false, 0]);
  stringParser.push(['i', 'ffffffffffffffffffff', 0, 16, U, false, false, false, false, 0]);
  stringParser.push(['i', '1_2', 0, 10, U, false, false, false, false, 1]);
  // readCodePoint.
  for (const input of ['0041', '004', '{41}', '{1F600}', '{110000}', '{}', '{41', '{4_1}', '00_4', 'D83D', 'zzzz', '', '{0}', '{10FFFF}']) {
    for (const throwOnInvalid of [false, true]) {
      stringParser.push(['p', input, 0, throwOnInvalid]);
    }
  }
}

// ================================================================== @babel/types
const types = {};

// --- the sample values of the is*/assert* matrices
const samples = [];
const nodeTypes = Object.keys(t.VISITOR_KEYS);
for (const type of nodeTypes) samples.push(N(type));
for (const type of Object.keys(t.DEPRECATED_KEYS)) samples.push(N(type));
for (const type of ['Expression', 'Statement', 'ModuleDeclaration', 'Flow']) samples.push(N(type));
for (const expected of [...t.PLACEHOLDERS, 'Foo', 'toString', null, 1, ['Identifier']]) samples.push(N('Placeholder', { expectedNode: expected }));
const weirdSamples = [null, U, false, 0, '', 'Identifier', 1, true, {}, N(null), N(1), N('toString'), N('constructor'),
  N('__proto__'), N(['Identifier']), [], [N('Identifier')], N(sym('t')), N('Identifier', { name: 'x' }), N('VariableDeclaration', { kind: 'let' }),
  N('Identifier', { name: 'undefined' }), N('ImportDefaultSpecifier'), N('Placeholder')];
for (const w of weirdSamples) samples.push(w);
types.samples = samples;
// The samples whose assert messages are recorded for every assert (the others record their outcome only): one node
// type, a deprecated type, a placeholder, a placeholder of an Object.prototype name, and the odd values.
const sampleIndex = s => samples.findIndex(x => JSON.stringify(x) === JSON.stringify(s));
types.assertMessageSamples = [nodeTypes.indexOf('Identifier'), sampleIndex(N('NumberLiteral')),
  sampleIndex(N('Placeholder', { expectedNode: 'Identifier' })), sampleIndex(N('Placeholder', { expectedNode: 'toString' })),
  ...[null, U, 0, '', 'Identifier', {}, N(null), N('toString'), N(['Identifier']), [], N(sym('t'))].map(sampleIndex)];
if (types.assertMessageSamples.includes(-1)) throw new Error('assert message sample not found');
// is*/assert* with options.
const optsSpecs = [U, null, {}, { name: 'x' }, { name: 'y' }, { type: 'Identifier' }, { zz: U }, '', 'ab', [], ['I'], 0, false];
types.optsSamples = [N('Identifier', { name: 'x' }), N('Placeholder', { expectedNode: 'Identifier', name: 'x' }), N('StringLiteral', { value: 'x' }), N('Identifier')];
types.optsSpecs = optsSpecs;
types.optsValidators = ['isIdentifier', 'isExpression', 'isLVal', 'isStringLiteral', 'isPlaceholder', 'isStandardized',
  'isNumberLiteral', 'isModuleDeclaration', 'isScopable'];
types.optsAsserts = ['assertIdentifier', 'assertExpression', 'assertStringLiteral', 'assertPlaceholder'];

// --- builders
const lowercaseSrc = fs.readFileSync(path.join(lib, 'builders', 'generated', 'lowercase.js'), 'utf8');
const builderFns = [];
{
  const re = /^function (\w+)\(([^)]*)\) \{\n([\s\S]*?)\n\}$/gm;
  let m;
  while ((m = re.exec(lowercaseSrc))) {
    const params = m[2] ? m[2].split(', ').map(p => /^(\w+)/.exec(p)[1]) : [];
    const typeM = /type: "(\w+)"/.exec(m[3]);
    const props = [...m[3].matchAll(/^    (\w+)(?:: (\w+))?,?$/gm)].map(x => [x[1], x[2] || x[1]]);
    const deprecated = /_deprecationWarning/.test(m[3]);
    // A deprecated builder calls the builder of its replacement with its own parameters.
    const replacement = { NumberLiteral: 'NumericLiteral', RegexLiteral: 'RegExpLiteral', RestProperty: 'RestElement', SpreadProperty: 'SpreadElement' }[m[1]];
    builderFns.push({ name: m[1], params, type: typeM ? typeM[1] : replacement || null, props, deprecated });
  }
}
const exportsOfFn = new Map();
for (const name of Object.keys(t)) {
  if (typeof t[name] !== 'function') continue;
  const f = t[name];
  if (!exportsOfFn.has(f)) exportsOfFn.set(f, []);
  exportsOfFn.get(f).push(name);
}
const lowercaseMod = require(path.join(lib, 'builders', 'generated', 'lowercase.js'));
// The local function of each export of lowercase.js ("exports.super = _super;").
const exportOfLocal = new Map();
for (const line of lowercaseSrc.split('\n')) {
  const m = /^exports\.(\w+)((?: = exports\.\w+)*) = (\w+);$/.exec(line);
  if (m && m[3] !== 'void' && !exportOfLocal.has(m[3])) exportOfLocal.set(m[3], m[1]);
}

// nodeStub is a node of one of the types a validator names (aliases expanded), different for each choice.
function stubsOf(typeNames) {
  const out = [];
  for (const name of typeNames) {
    if (t.VISITOR_KEYS[name]) out.push(N(name));
    else if (t.FLIPPED_ALIAS_KEYS[name]) for (const m of t.FLIPPED_ALIAS_KEYS[name].slice(0, 2)) out.push(N(m));
  }
  return out;
}
// candidates proposes valid values for a field validator, best first.
function candidates(v, depth = 0) {
  const out = [];
  if (!v || depth > 4) return [U];
  if (typeof v === 'function') {
    if (v.chainOf) {
      for (const link of v.chainOf) for (const c of candidates(link, depth + 1)) out.push(c);
    }
    if (v.type) {
      out.push(...({ string: ['x', 'y'], boolean: [true, false], number: [1, 0], array: [[]], object: [{}], any: ['any'] }[v.type] || []));
    }
    if (v.oneOf) out.push(...v.oneOf);
    if (v.oneOfNodeTypes) out.push(...stubsOf(v.oneOfNodeTypes));
    if (v.oneOfNodeOrValueTypes) {
      for (const n of v.oneOfNodeOrValueTypes) if (n === 'null') out.push(null);
      out.push(...stubsOf(v.oneOfNodeOrValueTypes.filter(n => n !== 'null')));
    }
    if (v.each) {
      const each = candidates(v.each, depth + 1).filter(c => c !== U);
      out.push([]);
      if (each.length) out.push([each[0]]);
    }
    if (v.shapeOf) {
      const o = {};
      for (const k of Object.keys(v.shapeOf)) o[k] = candidates(v.shapeOf[k].validate, depth + 1)[0];
      out.push(o);
    }
  }
  if (!out.length) out.push('x', 1, true, null, [], {});
  return out;
}
const tryCall = (fn, args) => {
  try {
    fn(...build(args));
    return null;
  } catch (e) {
    return e;
  }
};
// Valid argument lists the derivation above does not find.
const MANUAL_VALID = {
  templateLiteral: [[N('TemplateElement', { value: { raw: 'a' } })], []],
  tsTemplateLiteralType: [[N('TemplateElement', { value: { raw: 'a' } })], []],
  templateElement: [{ raw: 'a\\nb', cooked: 'x' }, false],
};
types.builders = [];
for (const b of builderFns) {
  const fn = lowercaseMod[exportOfLocal.get(b.name)];
  const names = exportsOfFn.get(fn) || [];
  if (!names.length) throw new Error('builder not exported: ' + b.name);
  const fields = b.type ? t.NODE_FIELDS[b.type] : {};
  // The field each parameter fills.
  const fieldOf = b.params.map(p => {
    const prop = b.props.find(([, param]) => param === p);
    return prop ? prop[0] : p.replace(/^_/, '');
  });
  let valid;
  if (MANUAL_VALID[b.name]) valid = MANUAL_VALID[b.name];
  else {
    const cands = fieldOf.map(f => (fields && fields[f] ? candidates(fields[f].validate) : ['x']));
    const choice = cands.map(() => 0);
    for (let attempt = 0; attempt < 60; attempt++) {
      valid = choice.map((c, i) => cands[i][c]);
      const e = tryCall(fn, valid);
      if (!e) break;
      const m = /^Property (\w+)/.exec(e.message);
      const idx = m ? fieldOf.indexOf(m[1]) : -1;
      if (idx < 0 || choice[idx] + 1 >= cands[idx].length) throw new Error(`builder ${b.name}: no valid arguments (${e.message})`);
      choice[idx]++;
    }
  }
  if (tryCall(fn, valid)) throw new Error('builder ' + b.name + ': manual arguments fail');
  // Invalid values for each parameter.
  const bad = fieldOf.map((f, i) => {
    const v = fields && fields[f] && fields[f].validate;
    const list = [U, null, 'x', 1, true, [], {}, N(1), N('toString'), N('__proto__'), N('Placeholder', { expectedNode: 'Identifier' }),
      N('Placeholder', { expectedNode: 'Statement' }), N('V8IntrinsicIdentifier'), N('JSXClosingFragment'), big(1), N(big(2))];
    if (v && (v.chainOf || v.each)) {
      const elem = candidates(v)[1];
      list.push([null], [N('JSXClosingFragment')], [elem !== undefined && !Array.isArray(elem) ? elem : N('Identifier'), N('V8IntrinsicIdentifier')], [U]);
    }
    if (v && v.oneOf) list.push('bogus', { a: [1, 'b'] });
    return list;
  });
  types.builders.push({ fn: b.name, names, valid, bad });
}

// --- calls of the other exports
const calls = [];
const call = (fn, ...args) => calls.push({ fn, args });
const callWith = (fn, extra, ...args) => calls.push({ fn, args, ...extra });
const id = (name, props = {}) => N('Identifier', { name, ...props });
const str = value => N('StringLiteral', { value });
const numLit = value => N('NumericLiteral', { value });
const exprStmt = expression => N('ExpressionStatement', { expression });
const block = body => N('BlockStatement', { body, directives: [] });
const member = (object, property, computed = false) => N('MemberExpression', { object, property, computed });
const comment = (value, idOpt) => (idOpt ? obj(idOpt, [['type', 'CommentBlock'], ['value', value]]) : { type: 'CommentLine', value });

// cloneNode and friends.
{
  const loc = { start: { line: 1, column: 0 }, end: { line: 1, column: 5 } };
  const shared = obj('c', [['type', 'CommentBlock'], ['value', 'shared'], ['loc', loc]]);
  const tree = obj('root', [['type', 'BinaryExpression'], ['operator', '+'], ['left', id('a', { loc, leadingComments: [shared] })],
    ['right', N('CallExpression', { callee: id('f'), arguments: [numLit(1), str('s'), null], trailingComments: [ref('c')] })],
    ['loc', loc], ['extra', obj('ex', [['parenthesized', true]], [[symfor('sym'), 1]])], ['_private', 1], ['start', 0]]);
  for (const fn of ['cloneNode', 'clone', 'cloneDeep', 'cloneDeepWithoutLoc', 'cloneWithoutLoc']) call(fn, tree);
  for (const [deep, withoutLoc] of [[U], [true], [false], [true, true], [false, true], [true, false]]) call('cloneNode', tree, deep, ...(withoutLoc === undefined ? [] : [withoutLoc]));
  const ident = id('x', { optional: true, typeAnnotation: N('TypeAnnotation', { typeAnnotation: N('AnyTypeAnnotation') }), decorators: [N('Decorator', { expression: id('d') })], loc });
  call('cloneNode', ident);
  call('cloneNode', ident, false);
  call('cloneNode', id('y', { optional: 'yes', extra: 'ab', innerComments: [comment('i')] }));
  call('cloneNode', id('y', { optional: false, extra: [1, 2] }));
  call('cloneNode', N('File', { program: N('Program', { body: [], directives: [], sourceType: 'module' }), comments: [comment('a'), comment('b')], tokens: [{ type: 'name' }] }));
  call('cloneNode', N('File', { program: N('Program', { body: [], directives: [] }), comments: null }));
  for (const v of [null, U, 0, '', false]) call('cloneNode', v);
  for (const v of [N('Foo'), N(U), 'abc', [1], N('toString'), N('constructor'), N(sym('x')), N(1)]) call('cloneNode', v);
  call('cloneNode', N('Identifier', { name: 'z', leadingComments: [null] }));
  call('cloneNode', N('Identifier', { name: 'z', trailingComments: [U] }));
  call('cloneNode', N('Identifier', { name: 'z', innerComments: [num('NaN'), num('NaN'), 0, num('-0')] }));
  call('cloneNode', N('Identifier', { name: 'z', innerComments: [big(1), big(1), sym('c'), symfor('c'), symfor('c'), true] }));
  call('inheritsComments', id('c', { leadingComments: [big(1), num('NaN'), 'x', 2] }), id('p', { leadingComments: [big(1), 'x', 2, num('-0'), symfor('k'), symfor('k')] }));
  call('cloneNode', N('Identifier', { name: 'z', leadingComments: 'str' }));
  call('cloneNode', N('Identifier', { name: 'z', leadingComments: [1, 'c', true] }));
  call('cloneNode', N('Placeholder', { expectedNode: 'Statement', name: id('p') }));
  call('cloneNode', N('ArrayExpression', { elements: [null, U, id('a'), [id('nested')], { notNode: true }, { type: 5 }] }));
  call('cloneNode', N('ObjectProperty', { key: id('k'), value: id('v'), computed: false, shorthand: false, decorators: null, extraField: 1 }));
  call('cloneNode', N('TemplateElement', { value: { raw: 'r', cooked: 'c' }, tail: true }));
  call('cloneNode', obj('cyc', [['type', 'ExpressionStatement'], ['expression', id('e')], ['leadingComments', [shared, ref('c')]]]));
}

// Comments.
{
  for (const type of ['leading', 'trailing', 'inner']) {
    call('addComment', id('a'), type, 'text');
    call('addComment', id('a'), type, 'text', true);
    call('addComment', id('a', { [type + 'Comments']: [comment('old')] }), type, 'new');
    call('addComments', id('a'), type, [comment('x'), comment('y')]);
    call('addComments', id('a', { [type + 'Comments']: [comment('old')] }), type, [comment('x')]);
    call('addComments', obj('n', [['type', 'Identifier'], ['name', 'n'], [type + 'Comments', arr('existing', [comment('old')])]]), type, [comment('x')]);
    call('addComments', id('a', { [type + 'Comments']: 'str' }), type, [comment('x')]);
    call('addComments', id('a', { [type + 'Comments']: [comment('old')] }), type, 'xy');
    call('addComments', id('a', { [type + 'Comments']: [comment('old')] }), type, 5);
  }
  call('addComments', null, 'leading', [comment('x')]);
  call('addComments', id('a'), 'leading', null);
  call('addComments', id('a'), 'leading', U);
  call('addComments', 5, 'leading', [comment('x')]);
  call('addComments', id('a'), 'other', [comment('x')]);
  const c1 = obj('c1', [['type', 'CommentLine'], ['value', 'one']]);
  for (const fn of ['inheritInnerComments', 'inheritLeadingComments', 'inheritTrailingComments', 'inheritsComments']) {
    call(fn, id('child', { leadingComments: [c1, comment('c')], trailingComments: null, innerComments: [null, 0, ''] }),
      id('parent', { leadingComments: [ref('c1'), comment('p')], trailingComments: [comment('t')], innerComments: comment('single') }));
    call(fn, id('child'), id('parent'));
    call(fn, null, id('parent', { leadingComments: [comment('p')] }));
    call(fn, id('child', { leadingComments: [comment('x')] }), null);
  }
  call('removeComments', id('a', { leadingComments: [comment('x')], other: 1 }));
  for (const v of [null, U, 'str', true, 5, big(3), sym('s'), [1]]) call('removeComments', v);
}

// Converters.
{
  const fnExpr = N('FunctionExpression', { id: null, params: [], body: block([]), generator: false, async: false });
  call('ensureBlock', N('IfStatement', { test: id('t'), consequent: exprStmt(id('c')), body: exprStmt(id('b')) }));
  call('ensureBlock', N('IfStatement', { test: id('t'), consequent: exprStmt(id('c')) }), 'consequent');
  call('ensureBlock', N('ArrowFunctionExpression', { params: [], body: id('x'), async: false }));
  call('ensureBlock', N('WhileStatement', { test: id('t'), body: N('EmptyStatement') }));
  call('ensureBlock', N('WhileStatement', { test: id('t'), body: block([exprStmt(id('k'))]) }));
  call('ensureBlock', N('WhileStatement', { test: id('t'), body: N('VariableDeclaration', { kind: 'var', declarations: [] }) }));
  call('ensureBlock', N('WhileStatement', { test: id('t'), body: N('ClassMethod') }));
  call('ensureBlock', null);
  call('ensureBlock', obj(null, [['type', 'WhileStatement'], ['test', id('t')], ['__proto__', exprStmt(id('x'))]]), '__proto__');
  call('ensureBlock', N('WhileStatement', { test: id('t'), body: exprStmt(id('x')) }), '__proto__');
  call('toBlock', id('x'), fnExpr);
  call('toBlock', id('x'), null);
  call('toBlock', N('EmptyStatement'), null);
  call('toBlock', block([]), null);
  call('toBlock', exprStmt(id('e')), fnExpr);
  call('toBlock', N('JSXClosingFragment'), null);
  call('toBlock', null, null);
  call('toBlock', U, fnExpr);
  const idInputs = ['', 'a b', 'hello world', '1abc', '-foo-bar', 'eval', 'arguments', 'class', 'hello-world', 'ä ü', 'ß x',
    'x ß', '😀 smile', 'a 𐐨', 'a b', 'x\ty', '--', 'a--b', '_', '$', 'a-1', '1-a', 'await', 'yield', 'let', 'enum',
    'constructor', 'toString', 'a.b.c', 'foo bar baz', 'Ab-Cd', 'ŉ x', 'x ǆ', 'x ﬃ', 'snake_case_name', 'x y',
    'x﻿y', 'a‌b', '   ', '123', 'π r²', 'aé bé', 'x 1', 'x -1', 'default', 'null', 'true', 'undefined', 'NaN'];
  for (const s of idInputs) {
    call('toIdentifier', s);
    call('toBindingIdentifierName', s);
  }
  for (const v of [5, 1.5, num('-0'), null, U, true, {}, [1, 2], big(7), num('NaN')]) {
    call('toIdentifier', v);
    call('toBindingIdentifierName', v);
  }
  call('toIdentifier', sym('s'));
  call('toComputedKey', N('ObjectProperty', { key: id('k'), computed: false }));
  call('toComputedKey', N('ObjectProperty', { key: id('k'), computed: true }));
  call('toComputedKey', N('ObjectProperty', { key: str('s'), computed: false }));
  call('toComputedKey', member(id('o'), id('p')));
  call('toComputedKey', member(id('o'), id('p'), true));
  call('toComputedKey', N('ObjectProperty', { key: null, property: id('q') }));
  call('toComputedKey', N('ObjectProperty', { key: id('k'), computed: false }), id('explicit'));
  call('toComputedKey', N('ObjectProperty', { key: id('k'), computed: false }), U);
  call('toComputedKey', {});
  call('toComputedKey', N('ObjectProperty', { key: id(5) }));
  call('toExpression', exprStmt(id('e')));
  call('toExpression', id('e'));
  call('toExpression', N('ClassDeclaration', { id: id('C'), superClass: null, body: N('ClassBody', { body: [] }), abstract: true }));
  call('toExpression', N('FunctionDeclaration', { id: id('f'), params: [], body: block([]) }));
  call('toExpression', N('ReturnStatement', { argument: null }));
  call('toExpression', exprStmt(null));
  call('toExpression', exprStmt(N('ClassDeclaration', { id: null })));
  call('toExpression', N('ObjectMethod', { kind: 'method' }));
  call('toExpression', null);
  for (const ignore of [U, true, false]) {
    call('toStatement', N('ClassExpression', { id: id('C'), body: N('ClassBody', { body: [] }) }), ignore);
    call('toStatement', N('ClassExpression', { id: null }), ignore);
    call('toStatement', N('FunctionExpression', { id: id('f') }), ignore);
    call('toStatement', N('ArrowFunctionExpression', { params: [], body: id('x') }), ignore);
    call('toStatement', N('AssignmentExpression', { operator: '=', left: id('a'), right: id('b') }), ignore);
    call('toStatement', id('x'), ignore);
    call('toStatement', exprStmt(id('x')), ignore);
  }
  call('toStatement', N('AssignmentExpression', { operator: '=', left: id('a'), right: N('Noop') }));
  call('toStatement', null);
  // toKeyAlias (the method counter starts at 0 in both recordings).
  call('toKeyAlias', N('ObjectMethod', { kind: 'method', key: id('m') }));
  call('toKeyAlias', N('ObjectMethod', { kind: 'method', key: id('m') }));
  call('toKeyAlias', N('ObjectProperty', { key: id('k') }));
  call('toKeyAlias', N('ObjectProperty', { key: str('s"q') }));
  call('toKeyAlias', N('ObjectProperty', { key: numLit(5), computed: true }));
  call('toKeyAlias', N('ClassProperty', { key: id('k'), static: true }));
  call('toKeyAlias', N('ClassProperty', { key: id('k'), static: true, computed: true }));
  call('toKeyAlias', N('ClassProperty', { key: member(id('a'), id('b'), false), computed: true, loc: { x: 1 }, start: 3 }));
  call('toKeyAlias', N('ObjectProperty', { key: id(5) }));
  call('toKeyAlias', N('ObjectProperty', { key: str(U) }));
  call('toKeyAlias', N('ObjectProperty', { key: U }));
  call('toKeyAlias', N('ObjectProperty', { key: U, computed: true }));
  call('toKeyAlias', N('ObjectProperty', { key: id('k') }), str('explicit'));
  call('toKeyAlias', N('ObjectMethod', { kind: 'get', key: id('g') }));
  call('toKeyAlias', N('ObjectProperty', { key: N('BigIntLiteral', { value: '1' }), _x: 1, leadingComments: [comment('c')] }));
  call('toKeyAlias', null);
  callWith('toKeyAlias', { setUid: 9007199254740990 }, N('ObjectMethod', { kind: 'method', key: id('m') }));
  call('toKeyAlias', N('ObjectMethod', { kind: 'method', key: id('m') }));
  call('toKeyAlias', N('ObjectMethod', { kind: 'method', key: id('m') }));
  // toSequenceExpression with a scope array.
  const varDecl = (kind, decls) => N('VariableDeclaration', { kind, declarations: decls });
  const declarator = (idNode, init) => N('VariableDeclarator', { id: idNode, init });
  const seqInputs = [
    [exprStmt(id('a')), exprStmt(id('b'))],
    [exprStmt(id('a'))],
    [id('x'), numLit(1)],
    [varDecl('var', [declarator(id('v'), numLit(1)), declarator(id('w'), null)])],
    [varDecl('var', [declarator(N('ObjectPattern', { properties: [N('ObjectProperty', { key: id('p'), value: id('q') })] }), id('o'))])],
    [varDecl('let', [declarator(id('l'), numLit(1))])],
    [N('IfStatement', { test: id('t'), consequent: exprStmt(id('c')), alternate: null })],
    [N('IfStatement', { test: id('t'), consequent: exprStmt(id('c')), alternate: exprStmt(id('d')) })],
    [N('IfStatement', { test: id('t'), consequent: N('ReturnStatement'), alternate: null })],
    [block([exprStmt(id('a')), exprStmt(id('b'))])],
    [block([])],
    [N('EmptyStatement'), exprStmt(id('a'))],
    [exprStmt(id('a')), N('EmptyStatement')],
    [N('EmptyStatement')],
    [N('ReturnStatement', { argument: id('r') })],
    [N('FunctionDeclaration', { id: id('f') })],
    [varDecl('var', [])],
    [exprStmt(id('a')), varDecl('var', [declarator(id('z'), null)])],
    [N('IfStatement', { test: id('t'), consequent: null, alternate: exprStmt(id('d')) })],
    [N('IfStatement', { test: id('t'), consequent: exprStmt(id('c')), alternate: N('ReturnStatement') })],
    [block([N('ReturnStatement')])],
    [varDecl('var', 'ab')],
  ];
  for (const nodes of seqInputs) callWith('toSequenceExpression', { scope: true }, nodes);
  callWith('toSequenceExpression', { scope: true }, []);
  callWith('toSequenceExpression', { scope: true }, null);
  callWith('toSequenceExpression', { scope: false }, [varDecl('var', [declarator(id('v'), numLit(1))])]);
  callWith('toSequenceExpression', { scope: false }, [exprStmt(id('a'))]);
  // valueToNode.
  const values = [U, true, false, null, '', 'str', 'quote"\n', 0, 1, 1.5, -1, -1.5, num('-0'), num('NaN'), num('Infinity'), num('-Infinity'),
    1e21, 5e-324, big(0), big(10), big(-10), big('123456789012345678901234567890'),
    { $: 're', source: 'a', flags: '' }, { $: 're', source: 'a+b', flags: 'gimsuy' }, { $: 're', source: '\\/', flags: 'g' }, { $: 're', source: '(?:)', flags: '' },
    [], [1, 'a', null, [true]], { a: 1, 'b-c': 2, class: 3, 1: 4, __proto__x: 5, _: 6 }, obj(null, [['__proto__', 1]]), {}, { nested: { deep: [{ x: U }] } },
    { $: 'date', ms: 0 }, { $: 'inst', ctor: 'Foo' }, { $: 'map' }, { $: 'fn', name: 'f' }, sym('s'), symfor('k'),
    obj(null, [['k', 1]], [[sym('hidden'), 2]]), { 'π': 1, '': 2, ' ': 3, 'a b': 4, '0x': 5, '10': 6, '01': 7 }];
  for (const v of values) call('valueToNode', v);
  // createTypeAnnotationBasedOnTypeof.
  for (const v of ['string', 'number', 'undefined', 'boolean', 'function', 'object', 'symbol', 'bigint', 'other', U, null, 5]) call('createTypeAnnotationBasedOnTypeof', v);
  call('buildUndefinedNode');
}

// Modifications.
{
  call('appendToMemberExpression', member(id('a'), id('b')), id('c'));
  call('appendToMemberExpression', member(id('a'), id('b')), id('c'), true);
  call('appendToMemberExpression', member(id('a'), id('b'), true), str('c'), false);
  call('appendToMemberExpression', member(id('a'), N('Noop')), id('c'));
  call('appendToMemberExpression', null, id('c'));
  call('prependToMemberExpression', member(id('a'), id('b')), id('p'));
  call('prependToMemberExpression', member(N('Super'), id('b')), id('p'));
  call('prependToMemberExpression', member(id('a'), id('b')), N('JSXClosingFragment'));
  call('prependToMemberExpression', {}, id('p'));
  const child = id('child', { typeAnnotation: null, returnType: N('Noop'), _kept: 1, start: 9 });
  const parent = id('parent', { typeAnnotation: N('TypeAnnotation'), typeParameters: N('TypeParameterDeclaration'), returnType: N('Noop'),
    _a: 1, __clone: 2, __b: 3, start: 0, end: 5, loc: { line: 1 }, leadingComments: [comment('l')], _undef: U });
  call('inherits', child, parent);
  call('inherits', id('c'), id('p'));
  call('inherits', null, parent);
  call('inherits', child, null);
  call('inherits', child, U);
  const props = N('Identifier', { name: 'n', tokens: [1], start: 0, end: 1, loc: { a: 1 }, raw: 'r', rawValue: 'v', leadingComments: [comment('c')],
    comments: [], _x: 1, _y: null, __z: 0, keep: 1 });
  call('removeProperties', props);
  call('removeProperties', props, { preserveComments: true });
  call('removeProperties', props, {});
  call('removeProperties', props, null);
  call('removeProperties', obj('s', [['type', 'Identifier'], ['start', null], ['end', U]], [[sym('a'), 1], [symfor('b'), 2]]));
  call('removeProperties', null);
  call('removeProperties', 'str');
  // An own "__proto__" property (as JSON.parse makes) is assigned like any other; without one, the assignment goes to
  // Object.prototype's accessor, which ignores a primitive.
  call('removeProperties', obj(null, [['type', 'Identifier'], ['__proto__', 5], ['_q', 1]]));
  call('removeProperties', obj(null, [['type', 'Identifier'], ['__proto__', null], ['_q', 1]]));
  call('inherits', id('c'), obj(null, [['type', 'Identifier'], ['__proto__', 7], ['_k', 1]]));
  call('inherits', obj(null, [['type', 'Identifier'], ['__proto__', 1]]), obj(null, [['type', 'Identifier'], ['__proto__', 7]]));
  const deep = N('BinaryExpression', { operator: '+', left: id('a', { start: 0, _p: 1 }), right: N('CallExpression', { callee: id('f', { loc: 1 }), arguments: [numLit(1)], end: 3 }), start: 0 });
  call('removePropertiesDeep', deep);
  call('removePropertiesDeep', deep, { preserveComments: true });
  call('removePropertiesDeep', null);
  // removeTypeDuplicates and the union builders.
  const any = N('AnyTypeAnnotation');
  const gen = (name, params) => N('GenericTypeAnnotation', { id: id(name), typeParameters: params ? N('TypeParameterInstantiation', { params }) : null });
  const qual = N('GenericTypeAnnotation', { id: N('QualifiedTypeIdentifier', { id: id('B'), qualification: N('QualifiedTypeIdentifier', { id: id('A'), qualification: id('Q') }) }) });
  const sharedTypes = arr('st', [N('StringTypeAnnotation'), N('NumberTypeAnnotation')]);
  const flowLists = [
    [N('StringTypeAnnotation'), N('NumberTypeAnnotation'), N('StringTypeAnnotation')],
    [N('StringTypeAnnotation'), any, N('NumberTypeAnnotation')],
    [N('UnionTypeAnnotation', { types: [N('StringTypeAnnotation'), N('BooleanTypeAnnotation')] }), N('NullLiteralTypeAnnotation')],
    [N('UnionTypeAnnotation', { types: sharedTypes }), N('UnionTypeAnnotation', { types: ref('st') })],
    [gen('T'), gen('T'), gen('U', [N('StringTypeAnnotation')]), gen('U', [N('NumberTypeAnnotation'), N('StringTypeAnnotation')])],
    [gen('V', [N('StringTypeAnnotation')]), gen('V')],
    [gen('W'), gen('W', [N('StringTypeAnnotation')])],
    [qual, qual],
    [null, U, 0, N('StringTypeAnnotation')],
    [obj('dup', [['type', 'ArrayTypeAnnotation'], ['elementType', N('AnyTypeAnnotation')]]), ref('dup'), N('ArrayTypeAnnotation')],
    [N('StringTypeAnnotation')],
    [],
  ];
  for (const list of flowLists) {
    call('removeTypeDuplicates', list);
    call('createFlowUnionType', list);
    call('createUnionTypeAnnotation', list);
  }
  call('removeTypeDuplicates', 'ab');
  call('removeTypeDuplicates', null);
  call('removeTypeDuplicates', { length: 2, 0: N('StringTypeAnnotation') });
  call('removeTypeDuplicates', [N('UnionTypeAnnotation', { types: null })]);
  call('removeTypeDuplicates', [N('UnionTypeAnnotation', { types: 5 })]);
  call('removeTypeDuplicates', [gen('P', [N('StringTypeAnnotation')]), N('GenericTypeAnnotation', { id: id('P'), typeParameters: { params: null } })]);
  call('removeTypeDuplicates', [N('GenericTypeAnnotation', { id: id('Q'), typeParameters: { params: null } }), gen('Q', [N('StringTypeAnnotation')])]);
  call('removeTypeDuplicates', [N('GenericTypeAnnotation', { id: id('R'), typeParameters: { params: { 0: 1 } } }), gen('R', [N('StringTypeAnnotation')])]);
  call('removeTypeDuplicates', [gen('X', [N('StringTypeAnnotation')]), N('GenericTypeAnnotation', { id: id('X'), typeParameters: N('TypeParameterInstantiation', { params: 5 }) })]);
  const tsRef = (name, params) => N('TSTypeReference', { typeName: typeof name === 'string' ? id(name) : name, typeParameters: params ? N('TSTypeParameterInstantiation', { params }) : null });
  const tsLists = [
    [N('TSStringKeyword'), N('TSNumberKeyword'), N('TSStringKeyword')],
    [N('TSStringKeyword'), N('TSAnyKeyword')],
    [N('TSUnionType', { types: [N('TSStringKeyword'), N('TSBooleanKeyword')] }), N('TSNullKeyword')],
    [tsRef('T'), tsRef('T'), tsRef('U', [N('TSStringKeyword')]), tsRef('U', [N('TSNumberKeyword')])],
    [tsRef(N('TSQualifiedName', { left: id('A'), right: id('B') }), [N('TSStringKeyword')]), tsRef(N('TSQualifiedName', { left: id('A'), right: id('B') }), [N('TSStringKeyword')])],
    [N('TSTypeAnnotation', { typeAnnotation: N('TSStringKeyword') }), N('TSStringKeyword'), N('TSLiteralType', { literal: numLit(1) })],
    [tsRef(N('ThisExpression'), [N('TSAnyKeyword')]), tsRef(N('ThisExpression'), [N('TSNeverKeyword')])],
    [N('TSTypeAnnotation', { typeAnnotation: N('TSUnknownKeyword') })],
    [],
  ];
  for (const list of tsLists) call('createTSUnionType', list);
  call('createTSUnionType', [tsRef('S', [N('TSStringKeyword')]), N('TSTypeReference', { typeName: id('S'), typeParameters: { params: 'xy' } })]);
  call('createTSUnionType', [N('TSTypeReference', { typeName: id('S'), typeParameters: { params: null } }), tsRef('S', [N('TSStringKeyword')])]);
  call('createTSUnionType', null);
  call('createTSUnionType', 'x');
}

// Retrievers.
{
  const pattern = N('ObjectPattern', { properties: [N('ObjectProperty', { key: id('k'), value: id('a') }),
    N('RestElement', { argument: id('rest') })] });
  const arrayPattern = N('ArrayPattern', { elements: [id('x'), null, N('AssignmentPattern', { left: id('y'), right: numLit(1) }), N('ArrayPattern', { elements: [id('z')] })] });
  const bindingNodes = [
    id('a'),
    [id('a'), id('b'), id('a')],
    N('VariableDeclaration', { kind: 'var', declarations: [N('VariableDeclarator', { id: pattern, init: null }), N('VariableDeclarator', { id: arrayPattern })] }),
    N('FunctionDeclaration', { id: id('f'), params: [id('p1'), N('RestElement', { argument: id('p2') })], body: block([]) }),
    N('FunctionExpression', { id: id('fe'), params: [id('q')] }),
    N('ArrowFunctionExpression', { params: [id('arrow')] }),
    N('ClassDeclaration', { id: id('C') }),
    N('ImportDeclaration', { specifiers: [N('ImportSpecifier', { local: id('l1'), imported: id('i1') }), N('ImportDefaultSpecifier', { local: id('def') }), N('ImportNamespaceSpecifier', { local: id('ns') })] }),
    N('ExportNamedDeclaration', { declaration: N('VariableDeclaration', { kind: 'const', declarations: [N('VariableDeclarator', { id: id('ex') })] }) }),
    N('ExportNamedDeclaration', { declaration: null, specifiers: [N('ExportSpecifier', { local: id('a'), exported: id('b') })] }),
    N('ExportDefaultDeclaration', { declaration: N('FunctionDeclaration', { id: id('dflt') }) }),
    N('ExportAllDeclaration', { source: str('m') }),
    N('AssignmentExpression', { operator: '=', left: id('assigned'), right: id('r') }),
    N('UnaryExpression', { operator: 'delete', argument: id('del') }),
    N('UpdateExpression', { operator: '++', argument: id('upd') }),
    N('CatchClause', { param: id('err') }),
    N('LabeledStatement', { label: id('lbl') }),
    N('ForInStatement', { left: id('fi') }),
    N('ForOfStatement', { left: N('VariableDeclaration', { kind: 'let', declarations: [N('VariableDeclarator', { id: id('fo') })] }) }),
    N('TSImportEqualsDeclaration', { id: id('tsi') }),
    N('DeclareClass', { id: id('dc') }),
    N('ObjectMethod', { params: [id('om')] }),
    N('ClassMethod', { params: [N('TSParameterProperty', { parameter: id('pp') })] }),
    [id('constructor'), id('__proto__'), id('toString'), id('__proto__')],
    N('Identifier', { name: sym('symname') }),
    N('Foo', { id: id('ignored') }),
    N('toString', { length: id('weird') }),
    N('hasOwnProperty', { undefined: id('undef') }),
    [null, 0, '', id('kept')],
    N('VariableDeclaration', { kind: 'var', declarations: 'ab' }),
  ];
  for (const n of bindingNodes) {
    call('getBindingIdentifiers', n);
    call('getBindingIdentifiers', n, true);
    call('getBindingIdentifiers', n, false, true);
    call('getBindingIdentifiers', n, true, true, true);
    call('getOuterBindingIdentifiers', n);
    call('getOuterBindingIdentifiers', n, true);
    call('getAssignmentIdentifiers', n);
  }
  call('getBindingIdentifiers', null);
  call('getAssignmentIdentifiers', N('ArrayPattern', { elements: null }));
  call('getAssignmentIdentifiers', N('ArrayPattern'));
  call('getAssignmentIdentifiers', N('ArrayPattern', { elements: 'ab' }));
  call('getAssignmentIdentifiers', N('ObjectPattern', { properties: big(1) }));
  call('getAssignmentIdentifiers', N('ObjectPattern', { properties: 5 }));
  call('getAssignmentIdentifiers', N('AssignmentPattern', { left: N('ObjectPattern', { properties: [N('ObjectProperty', { value: id('deep') })] }) }));
  const fn = (props = {}) => N('FunctionExpression', { id: null, params: [], body: block([]), ...props });
  const fnWithID = (i, props = {}) => obj(i, Object.entries(fn(props)));
  const named = [
    [fn({ id: id('own') })],
    [fn({ id: id(5) })],
    [fnWithID('f1'), N('ObjectProperty', { key: id('k'), value: ref('f1'), computed: false })],
    [fnWithID('f2'), N('ObjectProperty', { key: str('s'), value: ref('f2'), computed: true })],
    [fnWithID('f3'), N('ObjectProperty', { key: id('c'), value: ref('f3'), computed: true })],
    [fnWithID('f4'), N('VariableDeclarator', { id: id('v'), init: ref('f4') })],
    [fnWithID('f5'), N('AssignmentExpression', { operator: '=', left: id('l'), right: ref('f5') })],
    [fnWithID('f6'), N('AssignmentExpression', { operator: '+=', left: id('l'), right: ref('f6') })],
    [fnWithID('f7'), N('VariableDeclarator', { id: N('ArrayPattern'), init: ref('f7') })],
    [fnWithID('f8'), N('ObjectProperty', { key: N('PrivateName', { id: id('pn') }), value: ref('f8') })],
    [fn(), N('VariableDeclarator', { id: id('v') })],
    [N('ObjectMethod', { kind: 'get', key: id('g'), computed: false })],
    [N('ObjectMethod', { kind: 'set', key: numLit(1), computed: true })],
    [N('ClassMethod', { kind: 'method', key: N('PrivateName', { id: id('priv') }) })],
    [N('ClassMethod', { kind: 'method', key: id('c'), computed: true })],
    [N('ClassMethod', { kind: 'method', key: N('NullLiteral') })],
    [N('ClassMethod', { kind: 'method', key: N('RegExpLiteral', { pattern: 'a', flags: 'g' }) })],
    [N('ClassMethod', { kind: 'method', key: N('TemplateLiteral', { quasis: [N('TemplateElement', { value: { raw: 'x' } }), N('TemplateElement', { value: { raw: null } })], expressions: [] }) })],
    [N('ClassMethod', { kind: 'method', key: N('BigIntLiteral', { value: '5' }) })],
    [N('ClassMethod', { kind: 'method', key: N('BooleanLiteral', { value: true }) })],
    [N('ClassMethod', { kind: 'method', key: N('StringLiteral') })],
    [N('ClassMethod', { kind: 'method', key: N('NumericLiteral', { value: sym('n') }) })],
    [N('ClassMethod', { kind: 'set', key: N('TemplateLiteral', { quasis: 5 }) })],
    [fn(), null],
    [fn(), N('ObjectProperty', { key: id('k'), value: id('other') })],
    [N('ClassDeclaration', { id: null }), N('VariableDeclarator', { id: N('ArrayPattern') })],
    ['str', null],
    [null],
    [U],
    [sym('f')],
    [5],
    [[]],
    [{ $: 'fn', name: 'f' }],
  ];
  for (const [node, parent] of named) call('getFunctionName', node, ...(parent === undefined ? [] : [parent]));
}

// Traversal (the harness records each handler call).
{
  const tree = N('Program', { body: [exprStmt(N('BinaryExpression', { operator: '+', left: id('a'), right: numLit(1) })), null,
    N('IfStatement', { test: id('t'), consequent: block([exprStmt(id('c'))]), alternate: null })], directives: [] });
  for (const handlers of ['function', 'enter', 'exit', 'both', 'object']) callWith('traverse', { handlers }, tree, { state: 1 });
  callWith('traverse', { handlers: 'both' }, N('Unknown', { body: [id('x')] }));
  callWith('traverse', { handlers: 'both' }, N('toString'));
  callWith('traverse', { handlers: 'both' }, N('ArrayExpression', { elements: [id('a'), [id('nested')], 0, { type: 'Identifier', name: 'b' }] }));
  callWith('traverse', { handlers: 'null' }, tree);
  callWith('traverse', { handlers: 'undefined' }, tree);
  callWith('traverse', { handlers: 'badEnter' }, tree);
  callWith('traverse', { handlers: 'badExit' }, N('Identifier'));
  callWith('traverse', { handlers: 'both' }, null);
  for (const mode of ['all', 'skip:IfStatement', 'stop:Identifier', 'skip:Program', 'stop:BlockStatement']) callWith('traverseFast', { enter: mode }, tree);
  callWith('traverseFast', { enter: 'all' }, null);
  callWith('traverseFast', { enter: 'all' }, N('toString'));
  callWith('traverseFast', { enter: 'all', opts: { o: 1 } }, tree);
}

// Validators (the utilities).
{
  const placeholder = expected => N('Placeholder', { expectedNode: expected });
  const isTypes = ['Identifier', 'Expression', 'Statement', 'Pattern', 'LVal', 'Standardized', 'Declaration', 'Flow', 'ModuleDeclaration',
    'ImportOrExportDeclaration', 'Placeholder', 'Foo', 'toString', 'constructor', '__proto__', 'hasOwnProperty'];
  const isNodes = [id('x'), placeholder('Identifier'), placeholder('Statement'), placeholder('Declaration'), placeholder('Pattern'),
    placeholder('toString'), placeholder('Foo'), N('ExportNamedDeclaration'), N('FunctionDeclaration'), null, N('toString'), N('__proto__')];
  for (const type of isTypes) {
    for (const node of isNodes) {
      call('is', type, node);
      call('is', type, node, { name: 'x' });
      call('is', type, node, null);
    }
  }
  call('is', sym('t'), id('x'));
  call('is', 5, N('5'));
  for (const [a, b] of [['Identifier', 'Identifier'], ['Identifier', 'Expression'], ['Identifier', 'Statement'], [null, 'Expression'],
    [U, U], ['Placeholder', 'Expression'], ['Identifier', 'toString'], ['Identifier', '__proto__'], ['toString', 'toString'],
    ['ExportNamedDeclaration', 'ModuleDeclaration'], ['Identifier', 'Pattern'], [5, 'Expression'], ['Expression', 'Expression']]) call('isType', a, b);
  for (const [a, b] of [['Identifier', 'Identifier'], ['Identifier', 'Expression'], ['Pattern', 'LVal'], ['Declaration', 'Statement'],
    ['Foo', 'Expression'], ['toString', 'Expression'], ['__proto__', 'x'], [null, 'Expression'], [['Identifier'], 'Expression'],
    ['Identifier', 'Foo'], ['BlockStatement', 'Scopable']]) call('isPlaceholderType', a, b);
  for (const v of [id('x'), N('Foo'), N('toString'), N('__proto__'), null, U, 'Identifier', {}, N(['Identifier']), 0]) {
    call('isNode', v);
    call('assertNode', v);
  }
  call('assertNode', obj('loop', [['self', ref('loop')]]));
  for (const [a, b] of [[{ a: 1 }, { a: 1 }], [{ a: 1, b: 2 }, { a: 1 }], [{ a: 1 }, { a: 1, b: 2 }], [{ a: [] }, { a: [] }],
    [obj('o', [['a', 1]]), {}], ['ab', 'ab'], [{ 0: 'a', 1: 'b' }, 'ab'], [{}, ''], [{ length: 2 }, [1]], [{ toString: 1 }, { toString: 1 }]]) call('shallowEqual', a, b);
  // A validator read through NODE_FIELDS: its name, length and properties read as on any function. The harness puts
  // NODE_FIELDS[type][field].validate in the place of the argument validatorAt names.
  for (const [type, field, expected] of [['Identifier', 'name', { type: 'string' }], ['Identifier', 'name', { name: 'validate', length: 3 }],
    ['Identifier', 'name', { type: 'number' }], ['ArrayExpression', 'elements', { length: 0, name: 'validate' }],
    ['TemplateElement', 'value', { length: 0 }], ['MemberExpression', 'property', { name: 'validator', length: 3 }],
    ['File', 'comments', { length: 0, name: '' }], ['TryStatement', 'block', { length: 3 }], ['OptionalMemberExpression', 'optional', { length: 3 }]]) {
    callWith('shallowEqual', { validatorAt: [type, field, 0] }, null, expected);
    callWith('shallowEqual', { validatorAt: [type, field, 1] }, expected, null);
  }
  call('shallowEqual', null, { a: 1 });
  call('shallowEqual', {}, U);
  call('shallowEqual', 'abc', 'abcdef');
  call('shallowEqual', [1, 2], [1, 2]);
  call('shallowEqual', 5, { toFixed: 1 });
  call('shallowEqual', { a: 1 }, null);
  const decl = id('d');
  const bindingCases = [
    [obj('b1', [['type', 'Identifier'], ['name', 'x']]), N('VariableDeclarator', { id: ref('b1') })],
    [id('y'), N('VariableDeclarator', { id: id('y') })],
    [obj('b2', [['type', 'Identifier']]), N('FunctionDeclaration', { params: [ref('b2')] })],
    [obj('b3', [['type', 'Identifier']]), N('ObjectProperty', { value: ref('b3') }), N('ObjectExpression')],
    [obj('b4', [['type', 'Identifier']]), N('ObjectProperty', { value: ref('b4') }), N('ObjectPattern')],
    [decl, N('Foo')],
    [decl, N('toString')],
    [decl, N('hasOwnProperty')],
  ];
  for (const [node, parent, gp] of bindingCases) call('isBinding', node, parent, ...(gp ? [gp] : []));
  call('isBinding', obj('dd', [['type', 'Identifier']]), N('__defineGetter__', { undefined: ref('dd') }));
  for (const v of [N('FunctionDeclaration'), N('ClassDeclaration'), N('VariableDeclaration', { kind: 'let' }), N('VariableDeclaration', { kind: 'var' }),
    obj('vb', [['type', 'VariableDeclaration'], ['kind', 'var']], [[symfor('var used to be block scoped'), true]]),
    obj('vs', [['type', 'VariableDeclaration'], ['kind', 'var']], [[symfor('var used to be block scoped'), 'yes']]), id('x'), null]) {
    call('isBlockScoped', v);
    call('isLet', v);
    call('isVar', v);
  }
  for (const v of [str('s'), id('undefined'), id('x'), N('JSXText'), N('Identifier'), N('TemplateLiteral'), N('Immutable')]) call('isImmutable', v);
  call('isImmutable', null);
  const eqPairs = [
    [id('a'), id('a')], [id('a'), id('b')], [numLit(1), numLit(1)], [N('ArrayExpression', { elements: [id('a'), null] }), N('ArrayExpression', { elements: [id('a'), null] })],
    [N('ArrayExpression', { elements: [id('a')] }), N('ArrayExpression', { elements: [id('a'), id('b')] })],
    [N('TemplateElement', { value: { raw: 'a', cooked: 'a' } }), N('TemplateElement', { value: { raw: 'a', cooked: 'a' } })],
    [N('TemplateElement', { value: { raw: 'a', cooked: 'a' } }), N('TemplateElement', { value: { raw: 'a', cooked: 'b' } })],
    [N('Foo', { 0: 'x' }), N('Foo', { 0: 'x' })], [N('Foo', { 0: 'x' }), N('Foo', { 0: 'y' })], [N('toString'), N('toString')],
    [id('a'), N('StringLiteral')], [null, null], [1, 1], [{}, {}], [N(U), N(U)], [id('a', { name: U }), id('a', { name: null })],
    [N('MemberExpression', { object: id('a'), property: id('b'), computed: false }), N('MemberExpression', { object: id('a'), property: id('b'), computed: false })],
    [N('ArrayExpression', { elements: [] }), N('ArrayExpression', { elements: {} })],
    [N('ArrayExpression', { elements: [id('a')] }), N('ArrayExpression', { elements: [id('b')] })],
    [id('a', { typeAnnotation: null }), id('a', { typeAnnotation: N('TypeAnnotation') })],
    [id('a', { typeAnnotation: N('TypeAnnotation') }), id('a', { typeAnnotation: U })],
    [N('Foo'), N('Foo')], [N(U), N(null)], [N('5'), N('5')], [N(5), N(5)],
  ];
  for (const [a, b] of eqPairs) call('isNodesEquivalent', a, b);
  const refCases = [];
  const refNode = obj('rn', [['type', 'Identifier'], ['name', 'r']]);
  const parents = ['MemberExpression', 'OptionalMemberExpression', 'JSXMemberExpression', 'VariableDeclarator', 'ArrowFunctionExpression',
    'PrivateName', 'ClassMethod', 'ClassPrivateMethod', 'ObjectMethod', 'ObjectProperty', 'ClassProperty', 'ClassAccessorProperty',
    'ClassPrivateProperty', 'ClassDeclaration', 'ClassExpression', 'AssignmentExpression', 'AssignmentPattern', 'LabeledStatement',
    'CatchClause', 'RestElement', 'BreakStatement', 'ContinueStatement', 'FunctionDeclaration', 'FunctionExpression',
    'ExportNamespaceSpecifier', 'ExportDefaultSpecifier', 'ExportSpecifier', 'ImportDefaultSpecifier', 'ImportNamespaceSpecifier',
    'ImportSpecifier', 'ImportAttribute', 'JSXAttribute', 'ObjectPattern', 'ArrayPattern', 'MetaProperty', 'ObjectTypeProperty',
    'TSEnumMember', 'TSPropertySignature', 'CallExpression'];
  const slots = ['property', 'object', 'init', 'body', 'key', 'superClass', 'right', 'local', 'id', 'value', 'other'];
  for (const p of parents) {
    for (const slot of slots) {
      refCases.push([p, slot, false]);
    }
    refCases.push([p, 'key', true]);
    refCases.push([p, 'property', true]);
  }
  for (const [p, slot, computed] of refCases) {
    for (const gp of [U, N('ObjectPattern'), N('ExportNamedDeclaration', { source: str('m') })]) {
      if (gp !== U && !['ObjectProperty', 'ExportSpecifier'].includes(p)) continue;
      call('isReferenced', refNode, obj('rp', [['type', p], [slot, ref('rn')], ['computed', computed]]), ...(gp === U ? [] : [gp]));
    }
  }
  call('isReferenced', id('x'), N(5));
  call('isReferenced', id('x'), null);
  for (const [node, parent] of [[block([]), N('FunctionDeclaration')], [block([]), N('CatchClause')], [block([]), N('IfStatement')],
    [N('ObjectPattern'), N('ArrowFunctionExpression')], [N('ObjectPattern'), N('CatchClause')], [N('ObjectPattern'), N('Program')],
    [N('Program'), null], [id('x'), null], [N('ClassDeclaration'), null], [N('Placeholder', { expectedNode: 'BlockStatement' }), N('Program')]]) call('isScope', node, parent);
  for (const v of [N('ImportDefaultSpecifier'), N('ImportSpecifier', { imported: id('default') }), N('ImportSpecifier', { imported: id('x') }),
    N('ExportSpecifier', { exported: id('default') }), N('ExportSpecifier', { exported: str('default') }), N('ImportSpecifier', { imported: null, exported: id('default') }), {}]) call('isSpecifierDefault', v);
  call('isSpecifierDefault', null);
  for (const v of [...identifier.words, 'a', '1a', 'a b', 'π', '𐐀', 'x‌', '', 5, null, U, ['a'], 'yield', 'await', 'abstract', 'goto', 'volatile']) {
    call('isValidIdentifier', v);
    call('isValidIdentifier', v, false);
    call('isValidIdentifier', v, true);
    call('isValidES3Identifier', v);
  }
  // matchesPattern and buildMatchMemberExpression.
  const thisMember = member(N('ThisExpression'), id('a'));
  const deepMember = member(member(id('a'), id('b')), id('c'));
  const patterns = ['a', 'a.b', 'a.b.c', 'a.b.c.d', 'b.c', 'this.a', 'a.*', '', ['a', 'b', 'c'], ['a', 'b'], ['x']];
  const members = [deepMember, thisMember, member(member(id('a'), str('b'), true), id('c')), member(N('Super'), id('s')),
    member(id('a'), N('PrivateName', { id: id('p') })), N('MetaProperty', { meta: id('import'), property: id('meta') }),
    member(N('MetaProperty', { meta: id('new'), property: id('target') }), id('x')), member(id('a'), numLit(1), true), id('a'), null,
    member(null, id('m'))];
  for (const m of members) {
    for (const p of patterns) {
      call('matchesPattern', m, p);
      call('matchesPattern', m, p, true);
    }
  }
  call('matchesPattern', deepMember, 5);
  for (const m of members) {
    for (const p of ['a.b.c', 'a.b', 'React.Component']) {
      callWith('buildMatchMemberExpression', { member: m }, p);
      callWith('buildMatchMemberExpression', { member: m }, p, true);
    }
  }
  for (const m of [member(id('React'), id('Component')), member(member(id('React'), id('Component')), id('x')), id('React'), null]) callWith('react.isReactComponent', {}, m);
  for (const v of ['div', 'Div', 'my-el', '', 'a', '1a', U, null, 5, ['x'], 'éa', 'ab']) callWith('react.isCompatTag', {}, v);
  const text = value => N('JSXText', { value });
  for (const children of [
    [text('  hello  \n   world  '), N('JSXExpressionContainer', { expression: id('x') }), N('JSXExpressionContainer', { expression: N('JSXEmptyExpression') }), N('JSXElement')],
    [text('\n\n   '), text('a\r\nb\rc'), text('\ttab\t'), text('line\n\n'), text('  only')],
    [text('x', { start: 1, leadingComments: [comment('c')], _y: 2 })],
    [null, 0, 'str'],
    [text(5)],
  ]) callWith('react.buildChildren', {}, N('JSXElement', { children }));
  callWith('react.buildChildren', {}, N('JSXElement'));
  callWith('react.buildChildren', {}, N('JSXElement', { children: [text(null)] }));
  // validate.
  const validateCases = [
    [N('Identifier', { name: 'x' }), 'name', 'y'], [N('Identifier'), 'name', 5], [N('Identifier'), 'optional', 'no'],
    [N('Identifier'), 'typeAnnotation', null], [N('Identifier'), 'typeAnnotation', id('t')], [N('Identifier'), 'decorators', [id('d')]],
    [N('BlockStatement'), 'body', [exprStmt(id('a')), id('b')]], [N('BlockStatement'), 'directives', 'x'], [N('Foo'), 'x', 1],
    [N('toString'), 'name', 1], [N('Identifier'), 'constructor', 1], [N('Identifier'), '__proto__', 1], [N('Identifier'), 'toString', 1],
    [null, 'name', 1], [N('MemberExpression', { computed: true }), 'property', id('p')], [N('MemberExpression', { computed: false }), 'property', str('p')],
    [N('TemplateLiteral', { quasis: [1, 2] }), 'expressions', [id('e')]], [N('TemplateLiteral', { quasis: [1] }), 'expressions', [id('e')]],
    [N('TemplateLiteral'), 'expressions', []], [N('TemplateElement', { value: { raw: 'a' } }), 'value', { raw: 'b' }],
    [N('TemplateElement'), 'value', { raw: 'b' }], [N('TemplateElement', { value: { raw: 5 } }), 'value', { raw: 'b' }],
    [N('TemplateElement', { value: { raw: [] } }), 'value', { raw: 'b' }], [N('TemplateElement', { value: { raw: { length: 0 } } }), 'value', { raw: 'b' }],
    [N('TemplateElement', { value: { raw: null } }), 'value', { raw: 'b' }], [N('TemplateElement', { value: { raw: U } }), 'value', { raw: 'b' }], [N('TemplateElement'), 'value', null],
    [N('TemplateElement'), 'value', { raw: 1, cooked: 2 }], [N('BinaryExpression', { operator: 'in' }), 'left', N('PrivateName')],
    [N('BinaryExpression', { operator: '+' }), 'left', N('PrivateName')], [N('TSLiteralType'), 'literal', N('UnaryExpression', { operator: '-', argument: numLit(1) })],
    [N('TSLiteralType'), 'literal', N('UnaryExpression', { operator: '+', argument: numLit(1) })], [N('TSLiteralType'), 'literal', N('UnaryExpression', { operator: '-', argument: str('s') })],
    [N('File'), 'comments', 'anything'], [N('File'), 'tokens', [1, 'x']], [N('BindExpression'), 'object', null], [N('BindExpression'), 'object', N('__proto__')],
    [N('BindExpression'), 'object', N('__defineGetter__')], [N('BindExpression'), 'object', N('__lookupGetter__')], [N('BindExpression'), 'callee', N('constructor')],
    [N('Identifier'), sym('k'), 1], [N('ObjectProperty', { computed: true }), 'key', N('PrivateName')], [N('ObjectProperty', { computed: false }), 'key', N('PrivateName')],
    [N('ClassMethod', { computed: false }), 'key', N('PrivateName')], [N('ClassAccessorProperty', { computed: false }), 'key', N('PrivateName')],
    [N('OptionalMemberExpression', { computed: false }), 'property', N('PrivateName')], [N('NumericLiteral'), 'value', -1],
    [N('ArrayExpression'), 'elements', [null, U]], [N('Program'), 'sourceType', 'bogus'], [N('TSMappedType'), 'readonly', '+'], [N('TSMappedType'), 'readonly', 'x'],
    [N('ObjectTypeAnnotation'), 'exact', 'x'], [N('ImportDeclaration'), 'phase', 'source'], [N('ImportDeclaration'), 'phase', null],
  ];
  for (const [node, key, val] of validateCases) call('validate', node, key, val);
  // The validators that differ with BABEL_TYPES_8_BREAKING (these calls run in both environments).
  const pat = N('ObjectPattern', { properties: [] });
  call('validate', N('AssignmentExpression', { left: pat }), 'operator', '+=');
  call('validate', N('AssignmentExpression', { left: pat }), 'operator', '=');
  call('validate', N('AssignmentExpression', { left: id('x') }), 'operator', '**=');
  call('validate', N('AssignmentExpression', { left: id('x') }), 'operator', 'bogus');
  for (const name of ['valid', '1abc', 'class', 'a-b', '', 'π']) call('validate', N('Identifier'), 'name', name);
  for (const flags of ['gimsuy', 'dv', 'gx', 'G', '', 'g😀']) call('validate', N('RegExpLiteral'), 'flags', flags);
  for (const [node, val] of [[N('ObjectProperty', { computed: true, key: id('k') }), true], [N('ObjectProperty', { computed: false, key: str('k') }), true],
    [N('ObjectProperty', { computed: false, key: id('k') }), true], [N('ObjectProperty', { computed: true, key: id('k') }), false]]) call('validate', node, 'shorthand', val);
  call('validate', N('TryStatement', { handler: null, finalizer: null }), 'block', block([]));
  call('validate', N('TryStatement', { handler: N('CatchClause'), finalizer: null }), 'block', block([]));
  const varDeclNode = N('VariableDeclaration', { kind: 'var', declarations: [] });
  call('validate', N('ExportNamedDeclaration', { specifiers: [N('ExportSpecifier')], source: null }), 'declaration', varDeclNode);
  call('validate', N('ExportNamedDeclaration', { specifiers: [], source: str('m') }), 'declaration', varDeclNode);
  call('validate', N('ExportNamedDeclaration', { specifiers: [], source: null }), 'declaration', varDeclNode);
  call('validate', N('ExportNamedDeclaration', { specifiers: [], source: null }), 'declaration', null);
  for (const source of [null, str('m')]) {
    for (const spec of ['ExportSpecifier', 'ExportDefaultSpecifier', 'ExportNamespaceSpecifier']) {
      call('validate', N('ExportNamedDeclaration', { source }), 'specifiers', [N(spec)]);
    }
  }
  for (const left of [varDeclNode, id('x'), member(id('a'), id('b')), N('ObjectExpression'), N('ArrayPattern')]) call('validate', N('ForOfStatement'), 'left', left);
  for (const [meta, property] of [['new', 'target'], ['import', 'meta'], ['function', 'sent'], ['new', 'meta'], ['other', 'x']]) {
    call('validate', N('MetaProperty', { property: id(property) }), 'meta', id(meta));
  }
  call('validate', N('MetaProperty', { property: null }), 'meta', id('other'));
  call('validate', N('YieldExpression', { argument: null }), 'delegate', true);
  call('validate', N('YieldExpression', { argument: id('a') }), 'delegate', true);
  call('validate', N('YieldExpression', { argument: null }), 'delegate', false);
  // The optional-chain check of OptionalMemberExpression / OptionalCallExpression .optional.
  const ome = (optional, object) => N('OptionalMemberExpression', { optional, object, property: id('p'), computed: false });
  const oce = (optional, callee) => N('OptionalCallExpression', { optional, callee, arguments: [] });
  for (const node of [ome(true, id('a')), ome(false, ome(true, id('a'))), ome(false, oce(false, ome(true, id('a')))), ome(false, id('a')),
    ome(false, oce(false, id('f'))), ome(false, ome(false, null)), ome(false, U), oce(false, oce(true, id('f'))), oce(false, member(id('a'), id('b')))]) {
    call('validate', node, 'optional', node.optional);
  }
  call('validate', oce(false, id('f')), 'optional', false);
  // Parent validations (BABEL_TYPES_8_BREAKING), through validate's validateChild.
  for (const key of ['x.property', 'x.key', 'x.exported', 'x.imported', 'x.meta', 'x.other', 'plain']) {
    for (const name of ['class', 'this', 'enum', 'await', 'ok', 'let']) {
      const pv = () => obj('pv', [['type', 'Identifier'], ['name', name]]);
      for (const parent of [member(id('a'), id('b')), N('OptionalMemberExpression', { computed: false }), N('OptionalMemberExpression', { computed: true }),
        N('ObjectProperty', { computed: false }), N('ClassMethod', { computed: false }), N('ExportSpecifier'), N('Program')]) {
        call('validate', parent, key, pv());
      }
      // The parent holds the identifier itself.
      call('validate', N('ImportSpecifier', { imported: pv() }), key, ref('pv'));
      call('validate', N('MetaProperty', { meta: pv() }), key, ref('pv'));
      call('validate', N('ImportSpecifier', { imported: id(name) }), key, pv());
    }
  }
  for (const [parent, key] of [[N('ArrayPattern', { elements: [1, 2] }), 'elements[0]'], [N('ArrayPattern', { elements: [1, 2] }), 'elements[1]'],
    [N('ArrayPattern', { elements: [1] }), 'elements'], [N('ObjectPattern', { properties: [1, 2, 3] }), 'x.properties[1]'], [N('ArrayPattern'), 'other[0]'],
    [N('ArrayPattern'), 'x'], [N('ArrayPattern', { list: 'abc' }), 'list[1]']]) {
    call('validate', parent, key, N('RestElement', { argument: id('r') }));
  }
  const decl1 = N('VariableDeclarator', { id: id('a'), init: null });
  const declInit = N('VariableDeclarator', { id: N('ObjectPattern', { properties: [] }), init: id('i') });
  const declPattern = N('VariableDeclarator', { id: N('ArrayPattern', { elements: [] }), init: null });
  for (const kind of ['var', 'let', 'const', 'using', 'await using']) {
    for (const declarations of [[decl1], [declInit], [declPattern], [decl1, decl1], [N('VariableDeclarator', { id: N('VoidPattern') })], 'ab']) {
      call('validate', N('Program'), 'body[0]', N('VariableDeclaration', { kind, declarations }));
      call('validate', N('ForOfStatement', { left: obj('vd', [['type', 'VariableDeclaration'], ['kind', kind], ['declarations', declarations]]) }), 'left', ref('vd'));
    }
  }
  for (const parent of [N('ObjectPattern'), N('ObjectExpression')]) {
    for (const value of [id('v'), N('ObjectPattern', { properties: [] }), N('CallExpression'), N('AssignmentPattern')]) {
      call('validate', parent, 'properties[0]', N('ObjectProperty', { key: id('k'), value }));
    }
  }
  for (const type of ['FunctionDeclaration', 'ClassDeclaration']) {
    for (const parent of [N('Program'), N('ExportDefaultDeclaration')]) {
      for (const idNode of [null, id('f'), str('s')]) call('validate', parent, 'x', N(type, { id: idNode }));
    }
  }
  // Builders whose children are validated against their parent with BABEL_TYPES_8_BREAKING.
  call('arrayPattern', [N('RestElement', { argument: id('r') }), id('x')]);
  call('arrayPattern', [id('x'), N('RestElement', { argument: id('r') })]);
  call('objectPattern', [N('ObjectProperty', { key: id('k'), value: N('CallExpression'), computed: false, shorthand: false })]);
  call('objectExpression', [N('ObjectProperty', { key: id('k'), value: N('ObjectPattern', { properties: [] }), computed: false, shorthand: false })]);
  call('program', [N('FunctionDeclaration', { id: null }), N('ClassDeclaration', { id: null })]);
  call('exportDefaultDeclaration', N('FunctionDeclaration', { id: null }));
  call('forOfStatement', N('VariableDeclaration', { kind: 'const', declarations: [decl1, decl1] }), id('xs'), block([]));
  call('memberExpression', id('o'), id('class'));
  call('variableDeclaration', 'let', [N('VariableDeclarator', { id: N('ArrayPattern', { elements: [] }), init: null })]);
  // File.tokens' element validator checks nothing, so only BABEL_TYPES_8_BREAKING's assertEach validates each token
  // against its parent (the others' node-type validators already do).
  call('file', N('Program', { body: [], directives: [] }), null, [N('RestElement', { argument: id('r') }), 1]);
  call('file', N('Program', { body: [], directives: [] }), null, [1, N('RestElement', { argument: id('r') })]);
  call('file', N('Program', { body: [], directives: [] }), null, [N('VariableDeclaration', { kind: 'let', declarations: [N('VariableDeclarator', { id: N('ArrayPattern', { elements: [] }), init: null })] })]);
  call('optionalMemberExpression', id('a'), id('b'), false, false);
  call('optionalMemberExpression', ome(true, id('a')), id('b'), false, false);
  call('optionalCallExpression', id('f'), [], false);
  call('templateLiteral', [N('TemplateElement', { value: { raw: 'a' } })], [id('x')]);
  call('tsTemplateLiteralType', [N('TemplateElement', { value: { raw: 'a' } }), N('TemplateElement', { value: { raw: 'b' } })], [N('TSStringKeyword')]);
  // Template elements: invalid escapes give a null cooked value, a backtick or ${ is not raw text.
  for (const raw of ['plain', 'a\\nb', '\\u{', '\\x', '\\01', '\\8', '\\u{110000}', '\\u{41}\\u{', '\\x4 \\u', 'a`b', 'a${b', 'a$b', '\r\n', '\\\r\nx', 'a\\`b', '😀\\uD83D\\uDE00', '\\uD83D']) {
    call('templateElement', { raw, cooked: 'old' });
    call('templateElement', { raw });
  }
  call('templateElement', 5);
  call('templateElement', null);
  call('templateElement', { cooked: 1 });
  call('validate', N('Identifier'), 'name', { $: 'obj', id: 'cyc', props: [['type', ref('cyc')]] });
  call('validate', N('ExpressionStatement'), 'expression', obj('cyc2', [['type', obj('cyc3', [['self', ref('cyc3')]])]]));
  call('validate', N('ExpressionStatement'), 'expression', N(big(5)));
  call('validate', N('Program'), 'sourceType', big(5));
}

// Deprecation warnings: the first call of each name warns (once per process); "internal" callers (a Babel package)
// are not warned.
const deprecations = [];
{
  const dep = (fn, via, ...args) => deprecations.push({ fn, via, args });
  dep('__internal__deprecationWarning', 'internal', 'old1', 'new1');
  dep('__internal__deprecationWarning', 'external', 'old1', 'new1');
  dep('__internal__deprecationWarning', 'external', 'old2', 'new2');
  dep('__internal__deprecationWarning', 'external', 'old2', 'new2');
  dep('__internal__deprecationWarning', 'external', 'old3', 'new3', 'The thing ');
  dep('__internal__deprecationWarning', 'external', 'old4', 'new4', '', 'key4');
  dep('__internal__deprecationWarning', 'external', 'old5', 'new5', 'P ', 'key4');
  dep('isNumberLiteral', 'internal', N('NumberLiteral'));
  dep('isNumberLiteral', 'external', N('NumberLiteral'));
  dep('isRegexLiteral', 'external', N('RegexLiteral'));
  dep('isRegexLiteral', 'external', N('RegexLiteral'));
  dep('isRestProperty', 'external', null);
  dep('isSpreadProperty', 'internal', null);
  dep('isSpreadProperty', 'external', null);
  dep('isModuleDeclaration', 'external', N('ImportDeclaration'));
  dep('assertNumberLiteral', 'external', N('NumberLiteral'));
  dep('assertRegexLiteral', 'internal', N('Identifier'));
  dep('assertRestProperty', 'external', N('RestProperty'));
  dep('assertSpreadProperty', 'external', N('SpreadProperty'));
  dep('assertModuleDeclaration', 'external', N('ImportDeclaration'));
  dep('NumberLiteral', 'external', 1);
  dep('numberLiteral', 'external', 2);
  dep('RegexLiteral', 'internal', 'a');
  dep('regexLiteral', 'external', 'b', 'g');
  dep('RestProperty', 'external', id('r'));
  dep('SpreadProperty', 'external', id('s'));
  dep('spreadProperty', 'external', id('s'));
}

types.calls = calls;
types.deprecations = deprecations;

process.stdout.write(JSON.stringify({ identifier, stringParser, types }) + '\n');
