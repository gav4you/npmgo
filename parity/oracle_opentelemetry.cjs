// Runs the ORIGINAL @opentelemetry/api@1.9.0, @opentelemetry/core@2.11.0 and @opentelemetry/semantic-conventions@1.40.0
// against the parity cases and prints what happened in each, as JSON.
//
// Run: node parity/oracle_opentelemetry.cjs <root> parity/opentelemetry/cases.json > parity/opentelemetry/expected.json
//
// <root> holds a node_modules layout of the three original tarballs. Treat the tarballs as untrusted data: extract each
// into its own new, empty directory and run nothing from them except through this oracle:
//   for p in api-1.9.0 core-2.11.0 semantic-conventions-1.40.0; do
//     mkdir -p <root>/node_modules/@opentelemetry/${p%-*} &&
//       tar -xzf @opentelemetry__$p.tgz --strip-components=1 -C <root>/node_modules/@opentelemetry/${p%-*}
//   done
// (GNU tar on Windows needs --force-local for paths like C:/...). core finds api and semantic-conventions there, as it
// would in an application. Recorded with Node v25.9.0 on win32-x64; nothing in the recording depends on the machine.
//
// Every case records a trace: the results of the calls it makes, encoded as strings (see enc), and everything it can
// observe meanwhile: the messages that reach the diag logger (a recorder registered at level ALL before each case),
// every get/set of the text-map getters and setters, console calls, and, for the promise utilities, timers and
// settlements on a virtual clock. The global state is reset before each case. The only normalisation: the V8 stack
// frames ("\n    at ...") in messages that carry an error's stack are cut off (the Go port has no V8 frames).
'use strict';
const path = require('path');
const { build } = require('./build.cjs');

const [rootArg, casesPath] = process.argv.slice(2);
const root = path.resolve(rootArg);
const mod = name => path.join(root, 'node_modules', '@opentelemetry', name);
const cases = require(path.resolve(casesPath));

const api = require(mod('api'));
const core = require(mod('core'));
const semconvMain = require(mod('semantic-conventions'));
const semconvIncubating = require(path.join(mod('semantic-conventions'), 'build', 'src', 'index-incubating.js'));
const semver = require(path.join(mod('api'), 'build', 'src', 'internal', 'semver.js'));
const GLOBAL_KEY = Symbol.for('opentelemetry.js.api.1');

// --- Encoding.
function enc(v) {
  if (v === undefined) return 'undefined';
  if (v === null) return 'null';
  switch (typeof v) {
    case 'number': return Object.is(v, -0) ? '-0' : String(v);
    case 'string': return JSON.stringify(v);
    case 'boolean': return String(v);
    case 'bigint': return v + 'n';
    case 'symbol': return v.toString();
    case 'function': return 'fn';
  }
  if (Array.isArray(v)) return '[' + Array.from(v, enc).join(',') + ']';
  if (v instanceof Date) return 'Date(' + enc(v.getTime()) + ')';
  if (v instanceof RegExp) return 'RegExp(' + String(v) + ')';
  if (v instanceof core.TimeoutError) return 'TimeoutError(' + v.name + ': ' + v.message + ')';
  if (v instanceof Error) return 'Error(' + v.name + ': ' + v.message + ')';
  const proto = Object.getPrototypeOf(v);
  if (proto === Object.prototype || proto === null) return '{' + Object.keys(v).map(k => JSON.stringify(k) + ':' + enc(v[k])).join(',') + '}';
  return 'object ' + (v.constructor && v.constructor.name);
}
const stripStack = s => (typeof s === 'string' ? s.replace(/\n {4}at [\s\S]*$/, '') : s);
const thrown = e => 'throws ' + (e instanceof Error ? e.name + ': ' + e.message : enc(e));

// encSC renders a span context; isRemote is read as a boolean (Go's SpanContext.IsRemote is false where JavaScript
// leaves it undefined).
function encSC(sc) {
  if (sc === undefined || sc === null) return enc(sc);
  return `sc(${enc(sc.traceId)},${enc(sc.spanId)},${enc(sc.traceFlags)},remote=${!!sc.isRemote},state=${sc.traceState ? enc(sc.traceState.serialize()) : 'none'})`;
}
function encEntry(e) {
  if (e === undefined) return 'undefined';
  return enc(e.value) + (e.metadata !== undefined ? ';' + enc(e.metadata.toString()) : '');
}
function encBaggage(b) {
  if (b === undefined) return 'undefined';
  return '[' + b.getAllEntries().map(([k, e]) => enc(k) + '=' + encEntry(e)).join(',') + ']';
}

// --- Recording.
let sink = null; // the trace of the running case
const rec = line => { if (sink) sink.push(line); };
function makeRecorder(label) {
  const r = {};
  for (const level of ['error', 'warn', 'info', 'debug', 'verbose']) {
    r[level] = (...args) => rec(`${label}.${level}(${args.map(a => enc(stripStack(a))).join(', ')})`);
  }
  return r;
}
const diagRecorder = makeRecorder('diag');
const recGetter = () => ({
  get(carrier, key) { const v = api.defaultTextMapGetter.get(carrier, key); rec(`get ${key} -> ${enc(v)}`); return v; },
  keys(carrier) { const k = api.defaultTextMapGetter.keys(carrier); rec(`keys -> ${enc(k)}`); return k; },
});
const recSetter = () => ({ set(carrier, key, value) { rec(`set ${key} = ${enc(value)}`); api.defaultTextMapSetter.set(carrier, key, value); } });

// A context manager keeping a stack of active contexts (registered by some cases so that context.with takes effect).
class StackContextManager {
  constructor() { this.stack = [api.ROOT_CONTEXT]; }
  active() { return this.stack[this.stack.length - 1]; }
  with(context, fn, thisArg, ...args) { this.stack.push(context); try { return fn.call(thisArg, ...args); } finally { this.stack.pop(); } }
  bind(context, target) { return target; }
  enable() { return this; }
  disable() { this.stack = [api.ROOT_CONTEXT]; return this; }
}

function reset() {
  sink = null;
  delete globalThis[GLOBAL_KEY];
  api.trace.disable();
  api.context.disable();
  api.propagation.disable();
  api.metrics.disable();
  api.diag.setLogger(diagRecorder, { logLevel: api.DiagLogLevel.ALL, suppressOverrideMessage: true });
  core.setGlobalErrorHandler(core.loggingErrorHandler());
}
function runCase(fn, input) {
  reset();
  const trace = [];
  sink = trace;
  try { fn(input); } catch (e) { rec('case ' + thrown(e)); }
  sink = null;
  return trace;
}
async function runCaseAsync(fn, input) {
  reset();
  const trace = [];
  sink = trace;
  try { await fn(input); } catch (e) { rec('case ' + thrown(e)); }
  sink = null;
  return trace;
}

const TID = '4bf92f3577b34da6a3ce929d0e0e4736', SID = '00f067aa0ba902b7';
const validSC = () => ({ traceId: TID, spanId: SID, traceFlags: 1 });

// buildContext makes a context from {sc, baggage: [[key, value, metadata?]], suppressed}.
function buildContext(c) {
  let ctx = api.ROOT_CONTEXT;
  if (c.sc) {
    const sc = { traceId: c.sc.traceId, spanId: c.sc.spanId };
    if ('traceFlags' in c.sc) sc.traceFlags = build(c.sc.traceFlags);
    if ('traceState' in c.sc) sc.traceState = new core.TraceState(c.sc.traceState);
    ctx = api.trace.setSpanContext(ctx, sc);
  }
  if (c.baggage) {
    const record = {};
    for (const [k, v, m] of c.baggage) record[k] = m === undefined ? { value: v } : { value: v, metadata: api.baggageEntryMetadataFromString(m) };
    ctx = api.propagation.setBaggage(ctx, api.propagation.createBaggage(record));
  }
  if (c.suppressed) ctx = core.suppressTracing(ctx);
  return ctx;
}
const noFieldsKey = api.createContextKey('noFields');
function encContext(ctx) {
  if (ctx === api.ROOT_CONTEXT) return 'same context';
  return `span ${encSC(api.trace.getSpanContext(ctx))} baggage ${encBaggage(api.propagation.getBaggage(ctx))} noFields=${enc(ctx.getValue(noFieldsKey))}`;
}

// --- The virtual clock of the promise cases: timers (setTimeout / clearTimeout, clamped as in Node) and host work
// (`after`, unclamped), by due time then scheduling order. run() yields to V8 between tasks (through a real
// setImmediate), so every microtask a task queues runs before the next task, as in Node; internal/jsloop's Virtual
// drains its microtask queue at the same points.
class VirtualClock {
  constructor() { this.now = 0; this.seq = 0; this.tasks = []; }
  setTimeout(fn, ms) {
    ms = Number(ms);
    if (!Number.isInteger(ms)) throw new Error('virtual setTimeout: delay ' + ms + ' is not an integer');
    if (!(ms >= 1 && ms <= 2147483647)) ms = 1;
    rec(`t=${this.now} setTimeout(${ms})`);
    const t = { due: this.now + ms, seq: this.seq++, fn: () => { rec(`t=${this.now} timer fires`); fn(); }, timer: true, queued: true };
    this.tasks.push(t);
    return t;
  }
  clearTimeout(t) {
    rec(`t=${this.now} clearTimeout(${t && t.timer ? (t.queued ? 'pending' : 'done') : 'other'})`);
    if (!t || !t.timer || !t.queued) return;
    t.queued = false;
    this.tasks.splice(this.tasks.indexOf(t), 1);
  }
  after(ms, fn) { this.tasks.push({ due: this.now + ms, seq: this.seq++, fn, timer: false, queued: true }); }
  next() {
    let b = 0;
    for (let i = 1; i < this.tasks.length; i++) {
      const t = this.tasks[i], u = this.tasks[b];
      if (t.due < u.due || (t.due === u.due && t.seq < u.seq)) b = i;
    }
    return b;
  }
  async run() {
    for (let n = 0; ; n++) {
      if (n > 10000) throw new Error('virtual clock ran too many tasks');
      await new Promise(resolve => setImmediate(resolve));
      if (!this.tasks.length) return;
      const t = this.tasks.splice(this.next(), 1)[0];
      t.queued = false;
      if (t.due > this.now) this.now = t.due;
      t.fn();
    }
  }
}
async function withVirtualClock(fn) {
  const V = new VirtualClock();
  const saved = { setTimeout: global.setTimeout, clearTimeout: global.clearTimeout };
  global.setTimeout = (fn, ms) => V.setTimeout(fn, ms);
  global.clearTimeout = t => V.clearTimeout(t);
  try {
    fn(V);
    await V.run();
  } finally {
    global.setTimeout = saved.setTimeout;
    global.clearTimeout = saved.clearTimeout;
  }
  rec(`end t=${V.now}`);
}

const out = { node: process.version, apiVersion: require(path.join(mod('api'), 'package.json')).version };

// ================================================================== semantic-conventions
{
  const keys = cases.semconv.sampleKeys;
  const encExport = v => {
    if (typeof v === 'function') return 'fn(' + keys.map(k => JSON.stringify(v(k))).join(',') + ')';
    if (v !== null && typeof v === 'object') return '{' + Object.keys(v).map(k => JSON.stringify(k) + ':' + enc(v[k])).join(',') + '}';
    return enc(v);
  };
  out.semconv = {
    main: Object.keys(semconvMain).map(k => [k, encExport(semconvMain[k])]),
    incubating: Object.keys(semconvIncubating).map(k => [k, encExport(semconvIncubating[k])]),
  };
}

// ================================================================== core
const C = cases.core;
const coreOut = {};
coreOut.traceparent = C.traceparent.map(s => runCase(s => rec(encSC(core.parseTraceParent(s))), s));

coreOut.extract = C.extract.map(c => runCase(c => {
  const carrier = {};
  for (const k of ['traceparent', 'tracestate']) if (k in c) carrier[k] = build(c[k]);
  const ctx = new core.W3CTraceContextPropagator().extract(api.ROOT_CONTEXT, carrier, recGetter());
  rec(ctx === api.ROOT_CONTEXT ? 'same context' : `span ${encSC(api.trace.getSpanContext(ctx))} recording=${api.trace.getSpan(ctx).isRecording()}`);
}, c));

coreOut.inject = C.inject.map(c => runCase(c => {
  const ctx = buildContext({ sc: c.noSpan ? undefined : c.sc, suppressed: c.suppressed });
  const carrier = {};
  new core.W3CTraceContextPropagator().inject(ctx, carrier, recSetter());
  rec('carrier ' + enc(carrier));
}, c));

function traceStateOps(make) {
  return c => {
    let ts = make(build(c.raw));
    for (const [op, a, b] of c.ops) {
      switch (op) {
        case 'serialize': rec('serialize -> ' + enc(ts.serialize())); break;
        case 'get': rec(`get ${a} -> ${enc(ts.get(a))}`); break;
        case 'set': { const n = ts.set(a, b); rec(`set ${a} ${enc(b)} -> ${n === ts ? 'same' : 'new'}`); ts = n; break; }
        case 'unset': { const n = ts.unset(a); rec(`unset ${a} -> ${n === ts ? 'same' : 'new'}`); ts = n; break; }
        default: throw new Error('bad op ' + op);
      }
    }
  };
}
coreOut.traceState = C.traceState.map(c => runCase(traceStateOps(raw => new core.TraceState(raw)), c));

coreOut.baggageExtract = C.baggageExtract.map(c => runCase(c => {
  const carrier = c.absent ? {} : { baggage: build(c.header) };
  const ctx = new core.W3CBaggagePropagator().extract(api.ROOT_CONTEXT, carrier, recGetter());
  rec(ctx === api.ROOT_CONTEXT ? 'same context' : 'baggage ' + encBaggage(api.propagation.getBaggage(ctx)));
}, c));

coreOut.baggageInject = C.baggageInject.map(c => runCase(c => {
  let ctx = api.ROOT_CONTEXT;
  if (!c.noBaggage) {
    const entry = ([, v, m]) => (m === undefined ? { value: v } : { value: v, metadata: api.baggageEntryMetadataFromString(m) });
    let b;
    if (c.via === 'record') {
      const record = {};
      for (const e of c.entries) record[e[0]] = entry(e);
      b = api.propagation.createBaggage(record);
    } else {
      b = api.propagation.createBaggage();
      for (const e of c.entries) b = b.setEntry(e[0], entry(e));
    }
    ctx = api.propagation.setBaggage(ctx, b);
  }
  if (c.suppressed) ctx = core.suppressTracing(ctx);
  const carrier = {};
  new core.W3CBaggagePropagator().inject(ctx, carrier, recSetter());
  rec('carrier ' + enc(carrier));
}, c));

coreOut.parseRecord = C.parseRecord.map(v => runCase(v => rec(enc(core.parseKeyPairsIntoRecord(build(v)))), v));

{
  class ThrowingPropagator {
    inject() { throw new Error('inject failed'); }
    extract() { throw new Error('extract failed'); }
    fields() { return ['x-throw']; }
  }
  class StringThrower {
    inject() { throw 'just a string'; }
    extract() { throw 'just a string'; }
    fields() { return ['traceparent']; }
  }
  const noFields = {
    inject(ctx, carrier, setter) { setter.set(carrier, 'x-nofields', '1'); },
    extract(ctx) { return ctx.setValue(noFieldsKey, true); },
  };
  class DupFields {
    inject() {}
    extract(ctx) { return ctx; }
    fields() { return ['traceparent', 'baggage', 'traceparent', 'x-dup']; }
  }
  class SetsTraceparent {
    inject(ctx, carrier, setter) { setter.set(carrier, 'traceparent', 'overridden'); }
    extract(ctx) { return api.trace.setSpanContext(ctx, { traceId: '1'.repeat(32), spanId: '2'.repeat(16), traceFlags: 0 }); }
    fields() { return ['traceparent']; }
  }
  const make = name => ({
    trace: () => new core.W3CTraceContextPropagator(), baggage: () => new core.W3CBaggagePropagator(),
    throwing: () => new ThrowingPropagator(), stringThrower: () => new StringThrower(), noFields: () => noFields,
    dupFields: () => new DupFields(), setsTraceparent: () => new SetsTraceparent(),
  })[name]();
  coreOut.composite = C.composite.map(c => runCase(c => {
    const comp = c.noConfig ? new core.CompositePropagator() : new core.CompositePropagator({ propagators: c.propagators.map(make) });
    switch (c.op) {
      case 'fields': {
        const f = comp.fields();
        rec('fields ' + enc(f));
        f.push('mutated');
        rec('fields again ' + enc(comp.fields()));
        break;
      }
      case 'inject': {
        const carrier = {};
        comp.inject(buildContext(c.ctx), carrier, recSetter());
        rec('carrier ' + enc(carrier));
        break;
      }
      case 'extract':
        rec(encContext(comp.extract(api.ROOT_CONTEXT, Object.assign({}, c.carrier), recGetter())));
        break;
    }
  }, c));
}

coreOut.sanitize = C.sanitize.map(s => runCase(s => {
  const attrs = build(s);
  const result = core.sanitizeAttributes(attrs);
  rec('result ' + enc(result));
  if (attrs !== null && typeof attrs === 'object') {
    for (const k of Object.keys(result)) {
      if (Array.isArray(result[k]) && result[k].length > 0) rec(`array ${k} ${result[k] === attrs[k] ? 'shared' : 'copied'}`);
    }
  }
}, s));
coreOut.isAttributeValue = C.isAttributeValue.map(s => runCase(s => rec(String(core.isAttributeValue(build(s)))), s));

// merge: the result is encoded with the identity of every object: an input object (or array, function, Date, RegExp)
// is "=in<n>" (numbered depth-first through the arguments), an object met again in the result "#<n>". Empty arrays
// are not given an identity (Go's empty slices cannot keep one).
function encGraph(rootValue, inputs) {
  const seen = new Map();
  const go = v => {
    if (v === null || (typeof v !== 'object' && typeof v !== 'function')) return enc(v);
    if (Array.isArray(v) && v.length === 0) return '[]';
    if (inputs && inputs.has(v)) return '=' + inputs.get(v);
    if (typeof v === 'function') return 'fn';
    if (v instanceof Date || v instanceof RegExp) return enc(v);
    if (seen.has(v)) return '#' + seen.get(v);
    seen.set(v, seen.size);
    if (Array.isArray(v)) return '[' + Array.from(v, go).join(',') + ']';
    const proto = Object.getPrototypeOf(v);
    const name = proto === Object.prototype || proto === null ? '' : (v.constructor && v.constructor.name) || '?';
    return name + '{' + Object.keys(v).map(k => JSON.stringify(k) + ':' + go(v[k])).join(',') + '}';
  };
  return go(rootValue);
}
coreOut.merge = C.merge.map(specs => runCase(specs => {
  const refs = {};
  const args = specs.map(s => build(s, refs));
  const inputs = new Map();
  const walk = v => {
    if (v === null || (typeof v !== 'object' && typeof v !== 'function')) return;
    if (Array.isArray(v) && v.length === 0) return;
    if (inputs.has(v)) return;
    inputs.set(v, 'in' + inputs.size);
    if (typeof v === 'function' || v instanceof Date || v instanceof RegExp) return;
    if (Array.isArray(v)) v.forEach(walk); else Object.keys(v).forEach(k => walk(v[k]));
  };
  args.forEach(walk);
  const before = args.map(a => encGraph(a, null)).join(' | ');
  rec('result ' + encGraph(core.merge(...args), inputs));
  const after = args.map(a => encGraph(a, null)).join(' | ');
  rec(after === before ? 'inputs unchanged' : 'inputs changed: ' + after);
}, specs));

coreOut.time = C.time.map(c => runCase(c => {
  const perf = globalThis.performance;
  const realNow = perf.now;
  let i = 0;
  Object.defineProperty(perf, 'timeOrigin', { configurable: true, get: () => build(c.perf.timeOrigin) });
  perf.now = () => {
    if (i >= c.perf.now.length) throw new Error('now() called too often');
    return build(c.perf.now[i++]);
  };
  try {
    for (const [fn, ...args] of c.calls) {
      try { rec(`${fn} -> ${enc(core[fn](...args.map(a => build(a))))}`); } catch (e) { rec(`${fn} ${thrown(e)}`); }
    }
  } finally {
    delete perf.timeOrigin;
    delete perf.now;
    if (perf.now !== realNow) throw new Error('performance.now not restored');
  }
}, c));

coreOut.env = C.env.map(c => runCase(c => {
  const saved = process.env;
  process.env = Object.assign({}, c.env);
  try {
    for (const [fn, key] of c.calls) {
      try { rec(`${fn} ${key} -> ${enc(core[fn](key))}`); } catch (e) { rec(`${fn} ${key} ${thrown(e)}`); }
    }
  } finally {
    process.env = saved;
  }
}, c));

coreOut.diagLevel = C.diagLevel.map(v => runCase(v => rec(enc(core.diagLogLevelFromString(build(v)))), v));

const matcher = m => (m !== null && typeof m === 'object' && 're' in m ? new RegExp(m.re, m.flags) : build(m));
coreOut.urlMatches = C.urlMatches.map(c => runCase(([url, m]) => rec(String(core.urlMatches(url, matcher(m)))), c));
coreOut.urlIgnored = C.urlIgnored.map(c => runCase(([url, list]) => {
  rec(String(core.isUrlIgnored(url, Array.isArray(list) ? list.map(matcher) : build(list))));
}, c));

function buildException(spec) {
  if (spec !== null && typeof spec === 'object' && spec.$ === 'timeoutError') {
    const e = new core.TimeoutError(spec.msg);
    delete e.stack;
    return e;
  }
  return build(spec);
}
coreOut.errorHandler = C.errorHandler.map(c => runCase(c => {
  if (c.handler === 'recording') core.setGlobalErrorHandler(ex => rec('handler ' + enc(ex)));
  if (c.handler === 'throwing') core.setGlobalErrorHandler(() => { rec('handler throws'); throw new Error('handler failed'); });
  c.exceptions.forEach((spec, i) => {
    rec('exception ' + i);
    const ex = buildException(spec);
    if (c.handler === 'direct') {
      try { core.loggingErrorHandler()(ex); rec('returned'); } catch (e) { rec(thrown(e)); }
    } else {
      core.globalErrorHandler(ex);
      rec('returned');
    }
  });
}, c));

coreOut.anchoredClock = C.anchoredClock.map(c => runCase(c => {
  const clock = (name, list) => {
    let i = 0;
    return { now() { rec(name + '.now'); return build(list[i++]); } };
  };
  const ac = new core.AnchoredClock(clock('system', c.system), clock('monotonic', c.monotonic));
  for (let i = 0; i < c.calls; i++) rec('now -> ' + enc(ac.now()));
}, c));

coreOut.rpc = C.rpc.map(c => runCase(ops => {
  let ctx = api.ROOT_CONTEXT;
  const span = api.trace.wrapSpanContext(validSC());
  // route: Go's RPCMetadata.Route is "" where JavaScript has none, so an empty route reads as none.
  const encRpc = m => (m === undefined ? 'undefined' : `rpc(type=${enc(m.type)},route=${m.route ? enc(m.route) : 'none'},span=${m.span ? encSC(m.span.spanContext()) : 'none'})`);
  for (const [op, arg] of ops) {
    switch (op) {
      case 'get': rec('get -> ' + encRpc(core.getRPCMetadata(ctx))); break;
      case 'set': {
        const meta = { type: core.RPCType.HTTP, span };
        if ('route' in arg) meta.route = arg.route;
        ctx = core.setRPCMetadata(ctx, meta);
        rec('set');
        break;
      }
      case 'setRoute': core.getRPCMetadata(ctx).route = arg; rec('route set'); break;
      case 'delete': ctx = core.deleteRPCMetadata(ctx); rec('delete'); break;
      default: throw new Error('bad op ' + op);
    }
  }
}, c));

coreOut.suppress = C.suppress.map(c => runCase(ops => {
  let ctx = api.ROOT_CONTEXT;
  const key = api.createContextKey('OpenTelemetry SDK Context Key SUPPRESS_TRACING');
  for (const [op, v] of ops) {
    switch (op) {
      case 'is': rec('is ' + core.isTracingSuppressed(ctx)); break;
      case 'suppress': ctx = core.suppressTracing(ctx); rec('suppress'); break;
      case 'unsuppress': ctx = core.unsuppressTracing(ctx); rec('unsuppress'); break;
      case 'raw': ctx = ctx.setValue(key, build(v)); rec('raw ' + enc(build(v))); break;
      default: throw new Error('bad op ' + op);
    }
  }
}, c));

coreOut.unref = C.unref.map(s => runCase(s => {
  const timer = s === 'withUnref' ? { unref() { rec('unref called'); } } : s === 'withoutUnref' ? {} : build(s);
  try { core.unrefTimer(timer); rec('returned'); } catch (e) { rec(thrown(e)); }
}, s));

{
  const e = new core.TimeoutError('msg');
  coreOut.static = {
    sdkInfo: enc(core.SDK_INFO),
    exportResultCode: enc(Object.entries(core.ExportResultCode)),
    rpcType: enc(Object.entries(core.RPCType)),
    headers: [core.TRACE_PARENT_HEADER, core.TRACE_STATE_HEADER],
    traceFields: new core.W3CTraceContextPropagator().fields(),
    baggageFields: new core.W3CBaggagePropagator().fields(),
    timeoutError: [e.name, e.message, e instanceof Error, e instanceof core.TimeoutError, String(e), Object.keys(e)],
    exports: Object.keys(core),
    internal: Object.keys(core.internal),
  };
}

// ================================================================== api
const A = cases.api;
const apiOut = {};

apiOut.context = A.context.map(c => runCase(ops => {
  let ctx = api.ROOT_CONTEXT;
  for (const [op, key, spec] of ops) {
    const k = api.createContextKey(key);
    switch (op) {
      case 'set': {
        const prev = ctx;
        const v = build(spec);
        ctx = ctx.setValue(k, v);
        rec(`set ${key} ${enc(v)} -> ${ctx === prev ? 'same' : 'new'}; before -> ${enc(prev.getValue(k))}`);
        break;
      }
      case 'get': rec(`get ${key} -> ${enc(ctx.getValue(k))}`); break;
      case 'delete': { const prev = ctx; ctx = ctx.deleteValue(k); rec(`delete ${key} -> ${ctx === prev ? 'same' : 'new'}`); break; }
      case 'rootGet': rec(`root get ${key} -> ${enc(api.ROOT_CONTEXT.getValue(k))}`); break;
      default: throw new Error('bad op ' + op);
    }
  }
}, c));

apiOut.keys = A.keys.map(c => runCase(([a, b]) => {
  const ka = api.createContextKey(a), kb = api.createContextKey(b);
  rec(`${typeof ka} ${String(ka)} same=${ka === kb}`);
}, c));

{
  // Providers and propagators that record how they are called.
  const tracerProvider = name => ({
    name,
    getTracer(n, v, o) { rec(`${name}.getTracer(${enc(n)}, ${enc(v)}, ${enc(o)})`); return { label: name + ' tracer' }; },
  });
  const meterProvider = name => ({
    name,
    getMeter(n, v, o) { rec(`${name}.getMeter(${enc(n)}, ${enc(v)}, ${enc(o)})`); return { label: name + ' meter' }; },
  });
  const propagator = name => ({
    inject() {},
    extract(ctx) { return ctx; },
    fields() { return [name + '-field']; },
  });
  const describeProvider = p => (p instanceof api.ProxyTracerProvider ? 'proxy delegate=' + (p.getDelegate().name || p.getDelegate().constructor.name) : 'other');
  const describeTracer = t => (t instanceof api.ProxyTracer ? `proxy(${enc(t.name)}, ${enc(t.version)})` : t.label || t.constructor.name);
  const rawRecorder = makeRecorder('raw');
  apiOut.registry = A.registry.map(c => runCase(ops => {
    for (const [op, a, b] of ops) {
      switch (op) {
        case 'setTracer': rec('setGlobalTracerProvider -> ' + api.trace.setGlobalTracerProvider(tracerProvider(a))); break;
        case 'getTracerProvider': rec('getTracerProvider -> ' + describeProvider(api.trace.getTracerProvider())); break;
        case 'getTracer': rec('getTracer -> ' + describeTracer(api.trace.getTracer(a, b || undefined))); break;
        case 'disableTrace': api.trace.disable(); rec('trace disabled'); break;
        case 'setPropagator': rec('setGlobalPropagator -> ' + api.propagation.setGlobalPropagator(propagator(a))); break;
        case 'propagationFields': rec('fields -> ' + enc(api.propagation.fields())); break;
        case 'disablePropagation': api.propagation.disable(); rec('propagation disabled'); break;
        case 'setMeter': rec('setGlobalMeterProvider -> ' + api.metrics.setGlobalMeterProvider(meterProvider(a))); break;
        case 'getMeter': {
          const m = api.metrics.getMeter(a, b || undefined);
          rec('getMeter -> ' + (m === api.createNoopMeter() ? 'noop meter' : m.label));
          break;
        }
        case 'disableMetrics': api.metrics.disable(); rec('metrics disabled'); break;
        case 'setManager': rec('setGlobalContextManager -> ' + api.context.setGlobalContextManager(new StackContextManager())); break;
        case 'active': rec('active -> ' + (api.context.active() === api.ROOT_CONTEXT ? 'root' : 'other')); break;
        case 'disableContext': api.context.disable(); rec('context disabled'); break;
        case 'tamper':
          globalThis[GLOBAL_KEY] = b ? { version: a, diag: rawRecorder } : { version: a };
          rec(`registry replaced: version ${enc(a)}${b ? ' with a diag logger' : ''}`);
          break;
        case 'diagError': api.diag.error('direct error'); rec('diag.error called'); break;
        default: throw new Error('bad op ' + op);
      }
    }
  }, c));
}

apiOut.semver = A.semver.map(c => runCase(([own, g]) => {
  const check = semver._makeCompatibilityCheck(own);
  rec(`${enc(own)} ${enc(g)} -> ${check(g)} ${check(g)}`);
}, c));

{
  const consoleMethods = ['error', 'warn', 'info', 'debug', 'trace', 'log'];
  const consoleVariants = {
    full: consoleMethods,
    noDebug: ['error', 'warn', 'info', 'trace', 'log'],
    logOnly: ['log'],
    none: [],
  };
  apiOut.diag = A.diag.map(c => runCase(c => {
    const L = makeRecorder('L');
    const saved = {};
    for (const m of consoleMethods) saved[m] = console[m];
    try {
      if (c.console) {
        for (const m of consoleMethods) {
          console[m] = consoleVariants[c.console].includes(m) ? (...args) => rec(`console.${m}(${args.map(a => enc(stripStack(a))).join(', ')})`) : undefined;
        }
      }
      for (const [op, kind, arg] of c.setup) {
        if (op === 'disable') { api.diag.disable(); rec('disable'); continue; }
        let logger = L, args;
        switch (kind) {
          case 'level': args = [api.DiagLogLevel[arg]]; break;
          case 'number': args = [build(arg)]; break;
          case 'options': {
            const o = {};
            if ('logLevel' in arg) o.logLevel = api.DiagLogLevel[arg.logLevel];
            if ('suppressOverrideMessage' in arg) o.suppressOverrideMessage = arg.suppressOverrideMessage;
            args = [o];
            break;
          }
          case 'default': args = []; break;
          case 'null': args = [null]; break;
          case 'self': logger = api.diag; args = []; break;
          case 'nullLogger': logger = null; args = [api.DiagLogLevel[arg]]; break;
          case 'console': logger = new api.DiagConsoleLogger(); args = [api.DiagLogLevel[arg]]; break;
          default: throw new Error('bad kind ' + kind);
        }
        try { rec(`setLogger(${kind}) -> ${api.diag.setLogger(logger, ...args)}`); } catch (e) { rec(`setLogger(${kind}) ${thrown(e)}`); }
      }
      if (c.logs) for (const level of ['verbose', 'debug', 'info', 'warn', 'error']) api.diag[level](level + ' message', 1, { a: 1 });
      for (const ns of c.component || []) {
        const cl = api.diag.createComponentLogger({ namespace: ns });
        for (const level of ['verbose', 'debug', 'info', 'warn', 'error']) cl[level](level + ' from component', 2);
      }
    } finally {
      for (const m of consoleMethods) console[m] = saved[m];
    }
  }, c));
}

apiOut.baggage = A.baggage.map(c => runCase(ops => {
  let b, prev;
  const entry = (v, m) => (m === undefined ? { value: v } : { value: v, metadata: api.baggageEntryMetadataFromString(m) });
  const encEntries = entries => '[' + entries.map(([k, e]) => enc(k) + '=' + encEntry(e)).join(',') + ']';
  for (const [op, a, b2] of ops) {
    switch (op) {
      case 'create': {
        const record = {};
        for (const [k, v, m] of a) record[k] = entry(v, m);
        b = api.propagation.createBaggage(record);
        rec('created');
        break;
      }
      case 'all': rec('all -> ' + encEntries(b.getAllEntries())); break;
      case 'get': rec(`get ${a} -> ${encEntry(b.getEntry(a))}`); break;
      case 'mutateEntry': b.getEntry(a).value = 'mutated'; rec('mutated a copy'); break;
      case 'set': prev = b; b = b.setEntry(a, entry(b2)); rec(`set -> ${b === prev ? 'same' : 'new'}`); break;
      case 'prev': rec('previous all -> ' + encEntries(prev.getAllEntries())); break;
      case 'remove': prev = b; b = b.removeEntry(a); rec(`remove -> ${b === prev ? 'same' : 'new'}`); break;
      case 'removeMany': prev = b; b = b.removeEntries(...a); rec(`removeMany -> ${b === prev ? 'same' : 'new'}`); break;
      case 'clear': prev = b; b = b.clear(); rec(`clear -> ${b === prev ? 'same' : 'new'}`); break;
      case 'meta': {
        const m = api.baggageEntryMetadataFromString(build(a));
        rec(`meta -> ${enc(m.toString())} ${typeof m.__TYPE__ === 'symbol' ? 'branded' : 'unbranded'}`);
        break;
      }
      case 'context': {
        const bb = api.propagation.createBaggage({ a: { value: '1' } });
        let ctx = api.propagation.setBaggage(api.ROOT_CONTEXT, bb);
        rec('get -> ' + (api.propagation.getBaggage(ctx) === bb ? 'the baggage' : 'other'));
        rec('root -> ' + encBaggage(api.propagation.getBaggage(api.ROOT_CONTEXT)));
        ctx = api.propagation.deleteBaggage(ctx);
        rec('deleted -> ' + encBaggage(api.propagation.getBaggage(ctx)));
        rec('active -> ' + encBaggage(api.propagation.getActiveBaggage()));
        api.context.setGlobalContextManager(new StackContextManager());
        api.context.with(api.propagation.setBaggage(api.ROOT_CONTEXT, bb), () => rec('active inside -> ' + encBaggage(api.propagation.getActiveBaggage())));
        for (const falsy of [0, '', false, null]) {
          const fctx = api.ROOT_CONTEXT.setValue(api.createContextKey('OpenTelemetry Baggage Key'), falsy);
          rec(`falsy ${enc(falsy)} -> ${encBaggage(api.propagation.getBaggage(fctx))}`);
        }
        break;
      }
      default: throw new Error('bad op ' + op);
    }
  }
}, c));

apiOut.propagation = A.propagation.map(c => runCase(c => {
  const carrier = build(c.carrier);
  const g = api.defaultTextMapGetter, s = api.defaultTextMapSetter;
  rec('keys -> ' + enc(g.keys(carrier)));
  for (const k of ['a', 'b', '2', 'missing']) rec(`get ${k} -> ${enc(g.get(carrier, k))}`);
  s.set(carrier, 'new', 'v');
  s.set(carrier, 'a', 'replaced');
  rec('after set -> ' + enc(carrier));
  const key = api.createContextKey('extracted');
  if (c.global) {
    rec('setGlobalPropagator -> ' + api.propagation.setGlobalPropagator({
      inject(ctx, carrier, setter) { rec('propagator inject'); setter.set(carrier, 'injected', '1'); },
      extract(ctx, carrier, getter) { rec('propagator extract ' + enc(getter.get(carrier, 'traceparent'))); return ctx.setValue(key, 'yes'); },
      fields() { return ['f1', 'f2']; },
    }));
  }
  rec('fields -> ' + enc(api.propagation.fields()));
  const ctx = api.propagation.extract(api.ROOT_CONTEXT, carrier);
  rec('extract -> ' + (ctx === api.ROOT_CONTEXT ? 'same' : 'value ' + enc(ctx.getValue(key))));
  api.propagation.inject(api.ROOT_CONTEXT, carrier);
  rec('after inject -> ' + enc(carrier));
}, c));

apiOut.traceIds = A.traceIds.map(id => runCase(id => rec(`traceId ${api.isValidTraceId(id)} spanId ${api.isValidSpanId(id)}`), id));

{
  const recordingTracerProvider = name => ({
    name,
    getTracer(n, v, o) {
      rec(`${name}.getTracer(${enc(n)}, ${enc(v)}, ${enc(o)})`);
      if (name === 'empty') return undefined;
      return {
        label: name + ' tracer',
        startSpan(sn, opts, ctx) { rec(`${name} tracer startSpan ${enc(sn)}`); return api.trace.wrapSpanContext({ traceId: '3'.repeat(32), spanId: '4'.repeat(16), traceFlags: 1 }); },
        startActiveSpan(sn, ...rest) { rec(`${name} tracer startActiveSpan ${enc(sn)} with ${rest.length} more arguments`); return 'delegated'; },
      };
    },
  });
  const describeTracer = t => (t instanceof api.ProxyTracer ? `proxy(${enc(t.name)}, ${enc(t.version)}, ${enc(t.options)})` : t.label || t.constructor.name);
  const invalidSC = { traceId: '0'.repeat(32), spanId: SID, traceFlags: 1 };
  const programs = {
    'startSpan root'() {
      const s = api.trace.getTracer('t').startSpan('x', { root: true }, api.trace.setSpanContext(api.ROOT_CONTEXT, validSC()));
      rec(`span ${encSC(s.spanContext())} recording=${s.isRecording()}`);
    },
    'startSpan valid parent'() {
      const t = api.trace.getTracer('t');
      rec('span ' + encSC(t.startSpan('x', {}, api.trace.setSpanContext(api.ROOT_CONTEXT, validSC())).spanContext()));
      rec('span ' + encSC(t.startSpan('x', undefined, api.trace.setSpanContext(api.ROOT_CONTEXT, { ...validSC(), traceState: api.createTraceState('a=1') })).spanContext()));
    },
    'startSpan invalid parent'() {
      const t = api.trace.getTracer('t');
      rec('span ' + encSC(t.startSpan('x', {}, api.trace.setSpanContext(api.ROOT_CONTEXT, invalidSC)).spanContext()));
      rec('span ' + encSC(t.startSpan('x', {}, api.ROOT_CONTEXT).spanContext()));
    },
    'startSpan no context'() {
      const t = api.trace.getTracer('t');
      rec('span ' + encSC(t.startSpan('x').spanContext()));
      api.context.setGlobalContextManager(new StackContextManager());
      api.context.with(api.trace.setSpanContext(api.ROOT_CONTEXT, validSC()), () => rec('span ' + encSC(t.startSpan('x').spanContext())));
    },
    'startActiveSpan arities'() {
      const t = api.trace.getTracer('t');
      const parent = api.trace.setSpanContext(api.ROOT_CONTEXT, validSC());
      const fn = label => span => { rec(`${label} fn ${encSC(span.spanContext())}`); return label + ' result'; };
      rec('no fn -> ' + enc(t.startActiveSpan('a')));
      rec('fn -> ' + enc(t.startActiveSpan('a', fn('2 args'))));
      rec('options -> ' + enc(t.startActiveSpan('a', { root: true }, fn('3 args'))));
      rec('context -> ' + enc(t.startActiveSpan('a', undefined, parent, fn('4 args'))));
      rec('undefined context -> ' + enc(t.startActiveSpan('a', {}, undefined, fn('4 args, no context'))));
    },
    'proxy before delegate'() {
      const pp = new api.ProxyTracerProvider();
      const t = pp.getTracer('n', 'v', { schemaUrl: 's' });
      rec('tracer ' + describeTracer(t));
      rec('delegate ' + pp.getDelegate().constructor.name);
      rec('span ' + encSC(t.startSpan('x').spanContext()));
      pp.setDelegate(recordingTracerProvider('late'));
      rec('span ' + encSC(t.startSpan('y').spanContext()));
      rec('span again ' + encSC(t.startSpan('z').spanContext()));
      rec('active ' + enc(t.startActiveSpan('w', () => 'unused')));
      rec('new tracer ' + describeTracer(pp.getTracer('m')));
      rec('delegate ' + pp.getDelegate().name);
    },
    'proxy after delegate'() {
      const pp = new api.ProxyTracerProvider();
      pp.setDelegate(recordingTracerProvider('early'));
      rec('tracer ' + describeTracer(pp.getTracer('n', 'v')));
      rec('delegate tracer ' + describeTracer(pp.getDelegateTracer('d', 'e', { schemaUrl: 'f' })));
    },
    'proxy delegate returns nothing'() {
      const pp = new api.ProxyTracerProvider();
      pp.setDelegate(recordingTracerProvider('empty'));
      const t = pp.getTracer('n');
      rec('tracer ' + describeTracer(t));
      rec('span ' + encSC(t.startSpan('x').spanContext()));
      rec('span ' + encSC(t.startSpan('y').spanContext()));
    },
    'wrap and helpers'() {
      const sc = validSC();
      const span = api.trace.wrapSpanContext(sc);
      rec(`wrapped ${encSC(span.spanContext())} recording=${span.isRecording()}`);
      let ctx = api.trace.setSpan(api.ROOT_CONTEXT, span);
      rec('getSpan same -> ' + (api.trace.getSpan(ctx) === span));
      rec('getSpanContext -> ' + encSC(api.trace.getSpanContext(ctx)));
      ctx = api.trace.deleteSpan(ctx);
      rec('after delete -> ' + enc(api.trace.getSpan(ctx)) + ' ' + encSC(api.trace.getSpanContext(ctx)));
      rec('active span -> ' + enc(api.trace.getActiveSpan()));
      rec('valid -> ' + api.trace.isSpanContextValid(sc) + ' ' + api.isSpanContextValid(invalidSC) + ' ' + api.isSpanContextValid({ ...sc, spanId: '0'.repeat(16) }));
      const ctx2 = api.trace.setSpanContext(api.ROOT_CONTEXT, sc);
      rec('setSpanContext -> ' + encSC(api.trace.getSpanContext(ctx2)) + ' recording=' + api.trace.getSpan(ctx2).isRecording());
      for (const falsy of [0, '', false, null]) {
        const fctx = api.ROOT_CONTEXT.setValue(api.createContextKey('OpenTelemetry Context Key SPAN'), falsy);
        rec(`falsy ${enc(falsy)} -> ${enc(api.trace.getSpan(fctx))}`);
      }
      rec('invalid constant ' + encSC(api.INVALID_SPAN_CONTEXT) + ' ' + enc(api.INVALID_SPANID) + ' ' + enc(api.INVALID_TRACEID));
    },
    'span methods'() {
      const span = api.trace.wrapSpanContext(validSC());
      const same = v => (v === span ? 'this' : enc(v));
      rec('setAttribute ' + same(span.setAttribute('a', 1)));
      rec('setAttributes ' + same(span.setAttributes({ a: 1 })));
      rec('addEvent ' + same(span.addEvent('e', { a: 1 }, 5)));
      rec('addLink ' + same(span.addLink({ context: validSC() })));
      rec('addLinks ' + same(span.addLinks([])));
      rec('setStatus ' + same(span.setStatus({ code: api.SpanStatusCode.ERROR, message: 'm' })));
      rec('updateName ' + same(span.updateName('n')));
      rec('end ' + enc(span.end(5)));
      rec('recordException ' + enc(span.recordException(new Error('x'))));
      rec('isRecording ' + span.isRecording());
      rec('noop span ' + encSC(api.trace.getTracer('t').startSpan('x').spanContext()));
    },
    'startActiveSpan with manager'() {
      api.context.setGlobalContextManager(new StackContextManager());
      const t = api.trace.getTracer('t');
      const parent = api.trace.setSpanContext(api.ROOT_CONTEXT, validSC());
      const r = t.startActiveSpan('a', {}, parent, span => {
        rec('active span is the span -> ' + (api.trace.getActiveSpan() === span));
        rec('active span context ' + encSC(api.trace.getSpanContext(api.context.active())));
        return 'done';
      });
      rec('result ' + enc(r) + ' active after -> ' + enc(api.trace.getActiveSpan()));
    },
    'startActiveSpan bad fn'() {
      const t = api.trace.getTracer('t');
      for (const args of [[{}, undefined, undefined], [5], [{}, null]]) {
        try { rec(`args ${args.length + 1} -> ${enc(t.startActiveSpan('a', ...args))}`); } catch (e) { rec(`args ${args.length + 1} ${thrown(e)}`); }
      }
    },
    'global tracer'() {
      rec('register -> ' + api.trace.setGlobalTracerProvider(recordingTracerProvider('global')));
      rec('provider ' + (api.trace.getTracerProvider() instanceof api.ProxyTracerProvider ? 'proxy' : 'other'));
      rec('tracer ' + describeTracer(api.trace.getTracer('n', 'v')));
      api.trace.disable();
      rec('tracer after disable ' + describeTracer(api.trace.getTracer('n', 'v')));
      rec('register again -> ' + api.trace.setGlobalTracerProvider(recordingTracerProvider('second')));
      rec('tracer ' + describeTracer(api.trace.getTracer('n')));
    },
  };
  apiOut.trace = A.trace.map(([name]) => runCase(() => programs[name](), null));
}

apiOut.traceState = A.traceState.filter(c => typeof c.raw === 'string' || (c.raw && c.raw.$ === 'undef'))
  .map(c => runCase(traceStateOps(raw => api.createTraceState(raw)), c));

{
  const programs = {
    noop() {
      const m = api.createNoopMeter();
      rec('same meter -> ' + (m === api.createNoopMeter()));
      const kinds = ['createCounter', 'createUpDownCounter', 'createGauge', 'createHistogram', 'createObservableCounter', 'createObservableGauge', 'createObservableUpDownCounter'];
      for (const k of kinds) {
        const a = m[k]('a'), b = m[k]('b', { description: 'd' });
        rec(`${k} -> shared=${a === b} ${a.constructor.name}`);
      }
      rec('counter vs up-down -> ' + (m.createCounter('a') === m.createUpDownCounter('a')));
      rec('add -> ' + enc(m.createCounter('a').add(1, { k: 'v' })));
      rec('record -> ' + enc(m.createHistogram('a').record(1)));
      rec('addCallback -> ' + enc(m.createObservableGauge('a').addCallback(() => {})));
      rec('addBatchObservableCallback -> ' + enc(m.addBatchObservableCallback(() => {}, [])));
      rec('provider meter -> ' + (api.metrics.getMeterProvider().getMeter('x') === m) + ' ' + (api.metrics.getMeter('y', 'v') === m));
    },
    global() {
      const provider = {
        getMeter(n, v, o) { rec(`provider.getMeter(${enc(n)}, ${enc(v)}, ${enc(o)})`); return { label: 'provided' }; },
      };
      rec('register -> ' + api.metrics.setGlobalMeterProvider(provider));
      rec('provider is it -> ' + (api.metrics.getMeterProvider() === provider));
      rec('meter -> ' + api.metrics.getMeter('n', 'v', { schemaUrl: 's' }).label);
      rec('register again -> ' + api.metrics.setGlobalMeterProvider(provider));
      api.metrics.disable();
      rec('meter after disable -> ' + (api.metrics.getMeter('n') === api.createNoopMeter() ? 'noop' : 'other'));
    },
  };
  apiOut.metrics = A.metrics.map(([name]) => runCase(() => programs[name](), null));
}

apiOut.static = {
  enums: Object.fromEntries(['SpanKind', 'SpanStatusCode', 'TraceFlags', 'SamplingDecision', 'DiagLogLevel', 'ValueType'].map(n => [n, enc(Object.entries(api[n]))])),
  invalid: [api.INVALID_SPANID, api.INVALID_TRACEID, encSC(api.INVALID_SPAN_CONTEXT)],
  exports: Object.keys(api),
  defaultExport: Object.keys(api.default),
};

// ================================================================== async (core's promise utilities, virtual time)
(async () => {
  coreOut.async = [];
  for (const c of C.async) {
    coreOut.async.push(await runCaseAsync(async c => {
      await withVirtualClock(V => {
        switch (c.kind) {
          case 'cwt': {
            const s = c.settle;
            let inner;
            if (s === 'never') inner = new Promise(() => {});
            else if ('plain' in s) inner = build(s.plain);
            else if ('now' in s) inner = s.now === 'ok' ? Promise.resolve(build(s.value)) : Promise.reject(build(s.value));
            else {
              inner = new Promise((resolve, reject) => V.after(s.at, () => {
                rec(`t=${V.now} settling the promise`);
                if ('ok' in s) resolve(s.adopt ? Promise.resolve(build(s.ok)) : build(s.ok)); else reject(build(s.err));
              }));
            }
            const p = core.callWithTimeout(inner, c.timeout);
            p.then(v => rec(`t=${V.now} result ok ${enc(v)}`), e => rec(`t=${V.now} result err ${enc(e)}`));
            if (inner instanceof Promise) inner.then(v => rec(`t=${V.now} promise ok ${enc(v)}`), e => rec(`t=${V.now} promise err ${enc(e)}`));
            break;
          }
          case 'bof': {
            const cb = name => (...args) => {
              rec(`t=${V.now} callback(${args.map(enc).join(', ')})`);
              switch (name) {
                case 'value': return 'result';
                case 'throw': throw new Error('callback failed');
                case 'throwNull': throw null;
                case 'resolveLater': return new Promise(resolve => V.after(3, () => { rec(`t=${V.now} resolving`); resolve('later'); }));
                case 'rejectLater': return new Promise((_, reject) => V.after(3, () => { rec(`t=${V.now} rejecting`); reject(new Error('later failure')); }));
                case 'thenable': return { then(resolve) { rec(`t=${V.now} then called`); resolve('from thenable'); } };
                case 'undefined': return undefined;
              }
              throw new Error('bad callback');
            };
            const f = new core.BindOnceFuture(cb(c.callback), null);
            rec('isCalled ' + f.isCalled);
            f.promise.then(v => rec(`t=${V.now} promise ok ${enc(v)}`), e => rec(`t=${V.now} promise err ${enc(e)}`));
            for (const call of c.calls) {
              V.after(call.at, () => {
                const p = f.call(...call.args.map(a => build(a)));
                rec(`t=${V.now} call -> ${p === f.promise ? 'the promise' : 'another promise'}, isCalled ${f.isCalled}`);
              });
            }
            break;
          }
          case 'export': {
            if (c.manager) rec('setGlobalContextManager -> ' + api.context.setGlobalContextManager(new StackContextManager()));
            const exporter = {
              export(arg, done) {
                rec(`t=${V.now} export(${enc(arg)}) suppressed=${core.isTracingSuppressed(api.context.active())}`);
                switch (c.exporter) {
                  case 'sync': done({ code: core.ExportResultCode.SUCCESS }); break;
                  case 'later': V.after(4, () => { rec(`t=${V.now} reporting`); done({ code: core.ExportResultCode.FAILED, error: new Error('export failed') }); }); break;
                  case 'twice': done({ code: core.ExportResultCode.SUCCESS }); done({ code: core.ExportResultCode.FAILED }); break;
                  case 'throws': throw new Error('exporter threw');
                  case 'never': break;
                }
              },
            };
            const p = core.internal._export(exporter, build(c.arg));
            p.then(r => rec(`t=${V.now} result code=${enc(r.code)} error=${enc(r.error)}`), e => rec(`t=${V.now} rejected ${enc(e)}`));
            rec('active suppressed=' + core.isTracingSuppressed(api.context.active()));
            break;
          }
          default: throw new Error('bad kind ' + c.kind);
        }
      });
    }, c));
  }
  out.api = apiOut;
  out.core = coreOut;
  process.stdout.write(JSON.stringify(out, null, 1) + '\n');
})().catch(e => { console.error(e); process.exitCode = 1; });
