// Builds real JS values from the shared JSON value-spec language used by parity tests.
'use strict';
function build(spec, refs = {}) {
  if (typeof spec === 'string') return spec.split('{CWD}').join(process.cwd());
  if (spec === null || typeof spec !== 'object') return spec;
  if (Array.isArray(spec)) return spec.map(s => build(s, refs));
  const B = s => build(s, refs);
  const named = (name, Base) => {
    if (name === undefined || name === 'Object') return null;
    const C = Base ? ({ [name]: class extends Base {} })[name] : ({ [name]: class {} })[name];
    return C;
  };
  const addProps = (o, props) => {
    for (const [k, v, f = {}] of props || []) {
      const key = f.sym ? Symbol.for(k) : k;
      if (f.getter || f.setter) {
        Object.defineProperty(o, key, {
          enumerable: !f.hidden, configurable: true,
          get: f.getter ? (f.throws ? () => { const e = new Error(f.throws); delete e.stack; throw e; } : () => B(v)) : undefined,
          set: f.setter ? () => {} : undefined,
        });
      } else if (f.hidden || key === '__proto__') {
        Object.defineProperty(o, key, { value: B(v), enumerable: !f.hidden, configurable: true, writable: true });
      } else o[key] = B(v);
    }
  };
  switch (spec.$) {
    case 'undef': return undefined;
    case 'num': return Number(spec.v);
    case 'big': return BigInt(spec.v);
    case 'sym': return spec.d === undefined ? Symbol() : Symbol(spec.d);
    case 'ref': return refs[spec.id];
    case 'obj': {
      let o;
      if (spec.ctor === null) o = Object.create(null);
      else { const C = named(spec.ctor); o = C ? new C() : {}; }
      if (spec.tag) { if (spec.ctor === undefined) o = Object.create(Object.create(Object.prototype)); Object.defineProperty(Object.getPrototypeOf(o), Symbol.toStringTag, { value: spec.tag, enumerable: false, configurable: true }); }
      if (spec.args) o = (function () { return arguments; })();
      if (spec.id) refs[spec.id] = o;
      addProps(o, spec.props);
      return o;
    }
    case 'arr': {
      const C = named(spec.ctor, Array);
      const a = C ? new C() : [];
      if (spec.id) refs[spec.id] = a;
      a.length = spec.items.length;
      spec.items.forEach((it, i) => { if (!(it && it.$ === 'hole')) a[i] = B(it); });
      addProps(a, spec.props);
      return a;
    }
    case 'map': { const m = new Map(); if (spec.id) refs[spec.id] = m; for (const [k, v] of spec.entries) m.set(B(k), B(v)); addProps(m, spec.props); return m; }
    case 'set': { const s = new Set(); if (spec.id) refs[spec.id] = s; for (const v of spec.items) s.add(B(v)); addProps(s, spec.props); return s; }
    case 'date': { const d = new Date(spec.ms === null ? NaN : spec.ms); addProps(d, spec.props); return d; }
    case 're': { const r = new RegExp(spec.src, spec.flags); if (spec.lastIndex !== undefined) r.lastIndex = spec.lastIndex; addProps(r, spec.props); return r; }
    case 'u8': return new Uint8Array(spec.items);
    case 'boxed': return Object(B(spec.v));
    case 'fn': {
      let f;
      if (spec.class && spec.extends) { const Base = ({ [spec.extends]: class {} })[spec.extends]; f = ({ [spec.name]: class extends Base {} })[spec.name]; } else if (spec.class) f = ({ [spec.name]: class {} })[spec.name];
      else if (spec.async && spec.gen) f = async function* () {};
      else if (spec.async) f = async function () {};
      else if (spec.gen) f = function* () {};
      else f = function () {};
      if (!spec.class) Object.defineProperty(f, 'name', { value: spec.name || '' });
      if (spec.id) refs[spec.id] = f;
      addProps(f, spec.props);
      return f;
    }
    case 'err': {
      const Ctor = spec.ctor && spec.ctor !== 'Error' ? (globalThis[spec.ctor] || named(spec.ctor, Error)) : Error;
      const e = spec.ctor === 'AggregateError' ? new Ctor([], spec.msg) : spec.cause !== undefined ? new Ctor(spec.msg, { cause: B(spec.cause) }) : new Ctor(spec.msg);
      if (spec.id) refs[spec.id] = e;
      if (spec.name !== undefined) e.name = spec.name;
      // stack: absent = delete the own property; a string is assigned as is;
      // any other spec (object, false, 0, null) is built and assigned.
      if (spec.stack === undefined) delete e.stack; else e.stack = typeof spec.stack === 'string' ? spec.stack : B(spec.stack);
      addProps(e, spec.props);
      if (spec.nullProto) Object.setPrototypeOf(e, null);
      return e;
    }
  }
  switch (spec.$) {
    case "proxy": { const t = B(spec.target), h = B(spec.handler); if (spec.revoked) { const r = Proxy.revocable(t, h); r.revoke(); return r.proxy; } return new Proxy(t, h); }
    case "promise": { if (spec.state === "pending") return new Promise(() => {}); if (spec.state === "rejected") { const p = Promise.reject(B(spec.v)); p.catch(() => {}); return p; } return Promise.resolve(B(spec.v)); }
    case "weakset": return new WeakSet(spec.items.map(B));
    case "weakmap": return new WeakMap(spec.entries.map(([k, v]) => [B(k), B(v)]));
    case "mapiter": { const m = new Map(spec.entries.map(([k, v]) => [B(k), B(v)])); const it = m[spec.kind](); addProps(it, spec.props); return it; }
    case "setiter": { const s = new Set(spec.items.map(B)); const it = s[spec.kind](); addProps(it, spec.props); return it; }
    case "wk": return globalThis[spec.name].prototype;
    case "chain": {
      // Object with an explicit prototype: proto is a spec (obj/chain/wk) or null.
      const proto = spec.proto === null ? null : B(spec.proto);
      const o = Object.create(proto);
      if (spec.id) refs[spec.id] = o;
      addProps(o, spec.props);
      return o;
    }
    case "ctorfn": {
      // A named function whose .prototype is a previously built object (by id).
      const f = ({ [spec.name]: function () {} })[spec.name];
      if (spec.protoRef) f.prototype = refs[spec.protoRef];
      return f;
    }
    case "ab": { const b = new (spec.shared ? SharedArrayBuffer : ArrayBuffer)(spec.bytes.length); new Uint8Array(b).set(spec.bytes); if (spec.detached) structuredClone(b, { transfer: [b] }); addProps(b, spec.props); return b; }
    case "dv": { const b = new ArrayBuffer(spec.bytes.length); new Uint8Array(b).set(spec.bytes); const d = new DataView(b, spec.offset || 0, spec.length); addProps(d, spec.props); return d; }
    case "typed": { const T = globalThis[spec.kind]; const items = spec.big ? spec.items.map(BigInt) : spec.items; const t = new T(items.length); t.set(items); addProps(t, spec.props); return t; }
    case "url": {
      const u = new URL(spec.parts.href);
      for (const [k, v] of Object.entries(spec.parts)) if (u[k] !== v) throw new Error(`url ${k}: node=${u[k]} spec=${v}`);
      const params = [...u.searchParams];
      if (JSON.stringify(params) !== JSON.stringify(spec.params || [])) throw new Error('url params mismatch ' + JSON.stringify(params));
      return u;
    }
    case "usp": return new URLSearchParams(spec.params);
    case "ns": {
      const ns = build.namespaces[spec.name];
      const keys = Reflect.ownKeys(ns).filter(k => typeof k === 'string');
      const want = spec.exports.map(e => e[0]);
      if (JSON.stringify(keys) !== JSON.stringify(want)) throw new Error('ns keys ' + JSON.stringify(keys));
      return ns;
    }
    case "custom": { const util = require("util"); return { [util.inspect.custom](depth, opts, inspect) { if (spec.mode === "depth") return "custom@" + depth + (opts.colors ? ":c" : ""); if (spec.mode === "self") return this; if (spec.mode === "inspect") return "<" + inspect(B(spec.ret), opts) + ">"; return B(spec.ret); } }; }
  }
  throw new Error("bad spec " + JSON.stringify(spec));
}
module.exports = { build };
