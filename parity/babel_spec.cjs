// Value specs and the value encoding of the @babel/types parity cases, shared by tools/gen_babel_types_cases.cjs and
// parity/oracle_babel_types.cjs. pkgs/babel/types/harness_test.go is the Go twin of both.
//
// A spec is JSON. Strings, numbers, booleans and null are themselves; a JSON array is a new array; a JSON object
// without a "$" key is a new plain object with those keys in order ("__proto__" included, as an own property). An
// object with a "$" key is special:
//   {"$":"undef"}                       undefined
//   {"$":"num","v":"NaN"}               a number JSON cannot write: "NaN", "Infinity", "-Infinity", "-0"
//   {"$":"big","v":"12"}                a BigInt
//   {"$":"sym","d":"x","id":"s"}        a new symbol (Symbol("x")); id names it for {"$":"ref"}
//   {"$":"symfor","k":"x"}              Symbol.for("x")
//   {"$":"obj","id":"a","props":[[k,v],...],"syms":[[symSpec,v],...]}   an object that can be referred to
//   {"$":"arr","id":"a","items":[...]}  an array that can be referred to
//   {"$":"ref","id":"a"}                the object, array or symbol built with that id (it may be an enclosing one)
//   {"$":"re","source":"a","flags":"g"} a RegExp (the source must be canonical: new RegExp(source).source)
//   {"$":"date","ms":0}                 a Date
//   {"$":"inst","ctor":"Foo"}           an instance of a class Foo
//   {"$":"map"}                         an empty Map
//   {"$":"fn","name":"f"}               a function named f
'use strict';

function define(o, k, v) {
  if (k === '__proto__') Object.defineProperty(o, k, { value: v, enumerable: true, writable: true, configurable: true });
  else o[k] = v;
}

function build(spec, refs = new Map()) {
  if (spec === null || typeof spec !== 'object') return spec;
  if (Array.isArray(spec)) return spec.map(s => build(s, refs));
  if (!Object.prototype.hasOwnProperty.call(spec, '$')) {
    const o = {};
    for (const k of Object.keys(spec)) define(o, k, build(spec[k], refs));
    return o;
  }
  switch (spec.$) {
    case 'undef':
      return undefined;
    case 'num':
      if (!['NaN', 'Infinity', '-Infinity', '-0'].includes(spec.v)) throw new Error('bad num spec ' + spec.v);
      return spec.v === '-0' ? -0 : Number(spec.v);
    case 'big':
      return BigInt(spec.v);
    case 'sym': {
      const s = Symbol(spec.d);
      if (spec.id) refs.set(spec.id, s);
      return s;
    }
    case 'symfor':
      return Symbol.for(spec.k);
    case 'ref':
      if (!refs.has(spec.id)) throw new Error('unknown ref ' + spec.id);
      return refs.get(spec.id);
    case 'obj': {
      const o = {};
      if (spec.id) refs.set(spec.id, o);
      for (const [k, v] of spec.props || []) define(o, k, build(v, refs));
      for (const [k, v] of spec.syms || []) o[build(k, refs)] = build(v, refs);
      return o;
    }
    case 'arr': {
      const a = [];
      if (spec.id) refs.set(spec.id, a);
      for (const it of spec.items) a.push(build(it, refs));
      return a;
    }
    case 're': {
      const r = new RegExp(spec.source, spec.flags);
      if (r.source !== spec.source) throw new Error('non-canonical RegExp source ' + spec.source);
      return r;
    }
    case 'date':
      return new Date(spec.ms);
    case 'inst':
      return new ({ [spec.ctor]: class {} })[spec.ctor]();
    case 'map':
      return new Map();
    case 'fn':
      return ({ [spec.name]: function () {} })[spec.name];
  }
  throw new Error('bad spec ' + JSON.stringify(spec));
}

const isPlainish = v => {
  const p = Object.getPrototypeOf(v);
  return p === Object.prototype || p === null;
};

// children lists what enc descends into, in order.
function children(v) {
  if (Array.isArray(v)) return Array.from(v);
  if (typeof v === 'function') return Object.keys(v).map(k => v[k]);
  if (isPlainish(v)) return [...Object.keys(v).map(k => v[k]), ...Object.getOwnPropertySymbols(v).map(s => v[s])];
  return [];
}

function encSymbol(s) {
  const key = Symbol.keyFor(s);
  if (key !== undefined) return 'Symbol.for(' + JSON.stringify(key) + ')';
  return 'Symbol(' + (s.description === undefined ? '' : s.description) + ')';
}

function encNumber(x) {
  if (Object.is(x, -0)) return '-0';
  return String(x);
}

// enc renders a value: primitives as JavaScript writes them (strings JSON-quoted, -0, 12n), arrays and objects with
// their keys in order (symbol keys after the strings), functions as fn "name"{their own enumerable properties}, Dates,
// RegExps and Maps by kind, other objects by constructor. An object or array met more than once is labelled the first
// time (#n=) and referred to after (#n), so that sharing and cycles show.
function enc(root) {
  return encAll([root])[0];
}

// encAll renders several values with one set of labels (an object shared between them is labelled once).
function encAll(roots) {
  const counts = new Map();
  const isRef = v => (typeof v === 'object' && v !== null) || typeof v === 'function';
  const visit = v => {
    if (!isRef(v)) return;
    const c = counts.get(v) || 0;
    counts.set(v, c + 1);
    if (c > 0) return;
    for (const child of children(v)) visit(child);
  };
  roots.forEach(visit);
  const labels = new Map();
  let next = 0;
  const out = v => {
    if (v === undefined) return 'undefined';
    if (v === null) return 'null';
    switch (typeof v) {
      case 'number': return encNumber(v);
      case 'string': return JSON.stringify(v);
      case 'boolean': return String(v);
      case 'bigint': return v + 'n';
      case 'symbol': return encSymbol(v);
    }
    if (labels.has(v)) return '#' + labels.get(v);
    let prefix = '';
    if (counts.get(v) > 1) {
      labels.set(v, next++);
      prefix = '#' + labels.get(v) + '=';
    }
    if (Array.isArray(v)) return prefix + '[' + Array.from(v, out).join(',') + ']';
    if (typeof v === 'function') return prefix + 'fn ' + JSON.stringify(v.name) + '{' + Object.keys(v).map(k => JSON.stringify(k) + ':' + out(v[k])).join(',') + '}';
    if (v instanceof Date) return prefix + 'Date(' + encNumber(v.getTime()) + ')';
    if (v instanceof RegExp) return prefix + 'RegExp(' + String(v) + ')';
    if (v instanceof Map) return prefix + 'object Map';
    if (isPlainish(v)) {
      const parts = Object.keys(v).map(k => JSON.stringify(k) + ':' + out(v[k]));
      for (const s of Object.getOwnPropertySymbols(v)) parts.push('[' + encSymbol(s) + ']:' + out(v[s]));
      return prefix + '{' + parts.join(',') + '}';
    }
    return prefix + 'object ' + (v.constructor && v.constructor.name);
  };
  return roots.map(out);
}

// thrown renders an exception: its class and message.
function thrown(e) {
  if (e instanceof Error) return 'throws ' + e.name + ': ' + e.message;
  return 'throws ' + enc(e);
}

module.exports = { build, enc, encAll, thrown };
