// Runs the ORIGINAL graceful-fs@4.2.11 against scripted cases and prints what happened in each, as JSON.
//
// Run: node parity/oracle_gracefulfs.cjs <graceful-fs package dir> parity/gracefulfs/cases.json > parity/gracefulfs/expected.json
//
// <graceful-fs package dir> is the `package` directory of graceful-fs-4.2.11.tgz, extracted into its own new,
// empty directory (treat the tarball as untrusted data). Recorded with Node v25.9.0 on win32-x64; nothing in the
// recording depends on the machine: graceful-fs never touches the real file system or clock here.
//
// Each case loads graceful-fs afresh (a clean require.cache) over a scripted fake fs module, injected with a
// Module._load hook because graceful-fs captures fs's functions when it loads. Date.now, setTimeout, clearTimeout
// and process.nextTick are replaced by one virtual scheduler (class Virtual, the twin of internal/jsloop's
// Virtual): ticks run before tasks and after every task, tasks run by due time then scheduling order, and
// setTimeout clamps delays outside 1..2^31-1 to 1 ms as Node does. The fake fs completes each asynchronous call
// as a task `ms` milliseconds later (default 0, never synchronously) with the next scripted outcome for that
// function. The trace lists every call graceful-fs makes to the fake fs (its arguments, trailing undefineds
// dropped, and the virtual time relative to the start), every user callback with its arguments, the return value
// or exception of every step, uncaught exceptions and stream events. Timer fires are summarised as a count and an
// FNV-1a hash of their times, since an entry in 100 ms backoff fires the retry timer every millisecond.
//
// The queue graceful-fs publishes on global[Symbol.for('graceful-fs.queue')] cannot be removed, so every case
// shares it: the oracle checks it is empty before a case, records its length after, and empties it.
'use strict';
const path = require('path');
const Module = require('module');

const [gfsArg, casesPath] = process.argv.slice(2);
const gfsDir = path.resolve(gfsArg);
const cases = require(path.resolve(casesPath));
if (/\bgfs4\b/i.test(process.env.NODE_DEBUG || '')) throw new Error('run without NODE_DEBUG=gfs4');
if (process.env.TEST_GRACEFUL_FS_GLOBAL_PATCH) throw new Error('run without TEST_GRACEFUL_FS_GLOBAL_PATCH');
const queueSymbol = Symbol.for('graceful-fs.queue');

// --- The virtual scheduler (internal/jsloop/virtual.go).
class VTask {
  constructor(due, seq, fn, timer) { this.due = due; this.seq = seq; this.fn = fn; this.timer = timer; this.queued = true; }
}
class Virtual {
  constructor(now) { this.now = now; this.seq = 0; this.ticks = []; this.tasks = []; this.ran = 0; this.uncaught = null; this.maxTasks = 0; }
  nextTick(fn) { this.ticks.push(fn); }
  setTimeout(fn, ms) {
    if (!Number.isInteger(ms)) throw new Error('virtual setTimeout: delay ' + ms + ' is not an integer');
    if (!(ms >= 1 && ms <= 2147483647)) ms = 1;
    return this.schedule(ms, fn, true);
  }
  clearTimeout(t) {
    if (!(t instanceof VTask) || !t.timer || !t.queued) return;
    t.queued = false;
    this.tasks.splice(this.tasks.indexOf(t), 1);
  }
  after(ms, fn) { this.schedule(ms < 0 ? 0 : ms, fn, false); }
  schedule(ms, fn, timer) { const t = new VTask(this.now + ms, this.seq++, fn, timer); this.tasks.push(t); return t; }
  call(fn) {
    if (!this.uncaught) return fn();
    try { fn(); } catch (e) { this.uncaught(e); }
  }
  run() {
    for (;;) {
      while (this.ticks.length) this.call(this.ticks.shift());
      if (!this.tasks.length) return;
      if (this.maxTasks > 0 && this.ran >= this.maxTasks) throw new Error('virtual loop ran more than ' + this.maxTasks + ' tasks');
      this.ran++;
      let b = 0;
      for (let i = 1; i < this.tasks.length; i++) {
        const t = this.tasks[i], u = this.tasks[b];
        if (t.due < u.due || (t.due === u.due && t.seq < u.seq)) b = i;
      }
      const t = this.tasks.splice(b, 1)[0];
      t.queued = false;
      if (t.due > this.now) this.now = t.due;
      this.call(t.fn);
    }
  }
}

// --- Values the fake fs produces, and their encoding in the trace.
class FakeError extends Error {
  constructor(code, id) { super(code + ' #' + id); this.code = code; this.id = id; }
}
// OracleError reports a broken case (a script that runs out, an unscripted call). It is also remembered on the
// case, because graceful-fs swallows some exceptions itself (lutimesSync's closeSync).
class OracleError extends Error {}
function broken(ctx, msg) {
  const e = new OracleError(`case ${ctx.name}: ${msg}`);
  ctx.broken = ctx.broken || e;
  return e;
}
class FakeDirent { constructor(name) { this.name = name; } }
class FakeStats { constructor(uid, gid) { this.uid = uid; this.gid = gid; } }
class FakeStream {}

function enc(v) {
  if (v === undefined) return 'undefined';
  if (v === null) return 'null';
  if (typeof v === 'boolean') return String(v);
  if (typeof v === 'number') return Object.is(v, -0) ? '-0' : String(v);
  if (typeof v === 'string') return JSON.stringify(v);
  if (typeof v === 'function') return 'fn';
  if (Buffer.isBuffer(v)) return 'buf:' + v.toString('hex');
  if (Array.isArray(v)) return '[' + v.map(enc).join(',') + ']';
  if (v instanceof FakeError) return 'err:' + v.code + '#' + v.id;
  if (v instanceof FakeDirent) return 'dirent:' + JSON.stringify(v.name);
  if (v instanceof FakeStats) return 'stats:' + v.uid + ',' + v.gid;
  if (v instanceof FakeStream) return 'stream' + v.sid;
  if (Object.getPrototypeOf(v) === Object.prototype) return JSON.stringify(v);
  throw new Error('enc: unexpected value ' + Object.prototype.toString.call(v));
}
// thrown encodes an exception: a fake error, or the name of a built-in error (a TypeError from calling a missing
// function). An OracleError is re-thrown.
function thrown(e) {
  if (e instanceof OracleError) throw e;
  if (e instanceof FakeError) return enc(e);
  if (e instanceof Error) return e.name;
  return 'value ' + enc(e);
}
// val builds a scripted result: JSON values, {"buf": hex}, {"dirent": name}, {"stats": [uid, gid]}, {"undef": true}.
function val(x) {
  if (Array.isArray(x)) return x.map(val);
  if (x !== null && typeof x === 'object') {
    if ('buf' in x) return Buffer.from(x.buf, 'hex');
    if ('dirent' in x) return new FakeDirent(x.dirent);
    if ('stats' in x) return new FakeStats(x.stats[0], x.stats[1]);
    if (x.undef) return undefined;
    throw new Error('bad value ' + JSON.stringify(x));
  }
  return x;
}

// Kinds of the fake fs's functions: 'void' completes with (err) or (null), 'val' with (err) or (null, value),
// 'read' with (err) or (null, bytesRead, buffer). Asynchronous functions take their callback at cbPos (stat, fstat
// and lstat: last), and throw ERR_INVALID_ARG_TYPE, as Node does, when it is not a function.
const ASYNC = {
  open: ['val', 3], close: ['void', 1], readFile: ['val', 2], writeFile: ['void', 3], appendFile: ['void', 3],
  copyFile: ['void', 3], readdir: ['val', 2], read: ['read', 5], rename: ['void', 2],
  chown: ['void', 3], fchown: ['void', 3], lchown: ['void', 3], chmod: ['void', 2], fchmod: ['void', 2], lchmod: ['void', 2],
  stat: ['val', -1], fstat: ['val', -1], lstat: ['val', -1], futimes: ['void', 3], lutimes: ['void', 3],
};
const SYNC = {
  openSync: 'val', closeSync: 'void', readSync: 'val', chownSync: 'void', fchownSync: 'void', lchownSync: 'void',
  chmodSync: 'void', fchmodSync: 'void', lchmodSync: 'void', statSync: 'val', fstatSync: 'val', lstatSync: 'val',
  futimesSync: 'void', lutimesSync: 'void',
};
// The arity of each operation's callback as recorded: (err, value) for the 'val' ones, (err, bytesRead, buffer)
// for read, (err) otherwise.
const CB_ARITY = { open: 2, readFile: 2, readdir: 2, stat: 2, fstat: 2, lstat: 2, read: 3 };
// The functions whose presence on the loaded module is recorded.
const HAS = ['open', 'openSync', 'close', 'closeSync', 'readFile', 'writeFile', 'appendFile', 'copyFile', 'readdir',
  'read', 'readSync', 'rename', 'chown', 'fchown', 'lchown', 'chownSync', 'fchownSync', 'lchownSync',
  'chmod', 'fchmod', 'lchmod', 'chmodSync', 'fchmodSync', 'lchmodSync', 'stat', 'fstat', 'lstat',
  'statSync', 'fstatSync', 'lstatSync', 'futimes', 'futimesSync', 'lutimes', 'lutimesSync',
  'ReadStream', 'WriteStream', 'createReadStream', 'createWriteStream'];

// makeFake builds fake fs module number idx from its spec: {missing: [names], script: {key: [outcomes]}}. A
// call takes the next outcome under "name:arg0" (arg0 a string or number), else under "name"; an outcome is
// {err: code} or {ok: value} (or both, for a 'val' function), with optional ms (completion delay), times (repeat
// count) and throw (an error code thrown synchronously). {} is a success. An unscripted 'void' function
// succeeds; other unscripted calls, and a script that runs out, break the case. Synchronous 'void' functions
// return undefined, as Node's do.
function makeFake(ctx, idx, spec) {
  const fsName = idx === 0 ? 'fs' : 'fs' + idx;
  const missing = new Set(spec.missing || []);
  const script = spec.script || {};
  const cursors = {};
  const next = (name, args, kind) => {
    let key = name;
    if ((typeof args[0] === 'string' || typeof args[0] === 'number') && (name + ':' + args[0]) in script) key = name + ':' + args[0];
    const list = script[key];
    if (!list) {
      if (kind !== 'void') throw broken(ctx, 'unscripted ' + name);
      return {};
    }
    const c = cursors[key] || (cursors[key] = { i: 0, used: 0 });
    if (c.i >= list.length) throw broken(ctx, `script ${key} exhausted`);
    const o = list[c.i];
    if (++c.used >= (o.times || 1)) { c.i++; c.used = 0; }
    if ('ok' in o && kind === 'void') throw broken(ctx, `${key}: a 'void' function has no result`);
    if (!('ok' in o) && !o.err && !o.throw && kind !== 'void') throw broken(ctx, `${key}: no result`);
    return o;
  };
  const record = (name, args) => {
    const id = ++ctx.calls;
    let n = args.length;
    while (n > 0 && args[n - 1] === undefined) n--;
    ctx.trace(`${fsName}.${name}(${args.slice(0, n).map(enc).join(',')})`);
    return id;
  };
  const fake = {};
  for (const [name, [kind, cbPos]] of Object.entries(ASYNC)) {
    if (missing.has(name)) continue;
    fake[name] = function (...args) {
      const id = record(name, args);
      const cb = args[cbPos < 0 ? args.length - 1 : cbPos];
      if (typeof cb !== 'function') throw new FakeError('ERR_INVALID_ARG_TYPE', id);
      const o = next(name, args, kind);
      if (o.throw) throw new FakeError(o.throw, id);
      ctx.V.after(o.ms || 0, () => {
        const err = o.err ? new FakeError(o.err, id) : null;
        if (kind === 'read') return err ? cb(err) : cb(null, val(o.ok), args[1]);
        if (kind === 'void') return cb(err);
        if (err) return 'ok' in o ? cb(err, val(o.ok)) : cb(err);
        return cb(null, val(o.ok));
      });
    };
  }
  for (const [name, kind] of Object.entries(SYNC)) {
    if (missing.has(name)) continue;
    fake[name] = function (...args) {
      const id = record(name, args);
      const o = next(name, args, kind);
      if (o.err) throw new FakeError(o.err, id);
      return kind === 'void' ? undefined : val(o.ok);
    };
  }
  for (const kind of ['Read', 'Write']) {
    if (missing.has(kind + 'Stream')) continue;
    // A stream class modelled on Node's: the constructor takes flags, mode, autoClose and fd from the options
    // and calls this.open() when there is no fd, which runs a subclass's open (graceful-fs's).
    function S(p, options) {
      options = options || {};
      this.sid = ++ctx.streams;
      this.path = p;
      this.flags = options.flags === undefined ? (kind === 'Read' ? 'r' : 'w') : options.flags;
      this.mode = options.mode === undefined ? 0o666 : options.mode;
      this.autoClose = options.autoClose === undefined ? true : options.autoClose;
      this.fd = options.fd === undefined ? null : options.fd;
      ctx.trace(`stream${this.sid} = new ${kind}Stream(${enc(p)})`);
      if (this.fd === null) this.open();
    }
    S.prototype = Object.create(FakeStream.prototype);
    S.prototype.open = function () { ctx.trace(`stream${this.sid}.open()`); };
    S.prototype.emit = function (ev, arg) { ctx.trace(`stream${this.sid}.emit(${JSON.stringify(ev)},${enc(arg)})`); return true; };
    S.prototype.destroy = function () { ctx.trace(`stream${this.sid}.destroy()`); return this; };
    S.prototype.read = function () { ctx.trace(`stream${this.sid}.read()`); return null; };
    fake[kind + 'Stream'] = S;
  }
  return fake;
}

// --- Steps: calls on a loaded copy ("g0", "g1") or on a fake fs module ("b0", "b1"), at a virtual time or from
// a callback ("then"). Arguments are JSON values, {"undef": true}, {"buf": hex}, {"opts": object} and
// {"cb": n, "then": [steps], "throws": code}, a recording callback that runs its steps, then throws a fake error
// with that code (id "cb<n>") if asked to.
function arg(ctx, a, op) {
  if (a !== null && typeof a === 'object' && !Array.isArray(a)) {
    if (a.undef) return undefined;
    if ('buf' in a) return Buffer.from(a.buf, 'hex');
    if ('opts' in a) return JSON.parse(JSON.stringify(a.opts));
    if ('cb' in a) {
      const arity = CB_ARITY[op] || 1;
      return function () {
        const vals = [arguments[0] == null ? 'null' : enc(arguments[0])];
        for (let i = 1; i < arity; i++) vals.push(enc(arguments[i]));
        ctx.trace(`cb${a.cb}(${vals.join(',')})`);
        (a.then || []).forEach((s, j) => runStep(ctx, s, `cb${a.cb}.${j}`));
        if (a.throws) throw new FakeError(a.throws, 'cb' + a.cb);
      };
    }
    throw new Error('bad argument ' + JSON.stringify(a));
  }
  return a;
}
function runStep(ctx, step, label) {
  const on = step.on || 'g0';
  const target = on[0] === 'g' ? ctx.instances[+on.slice(1)] : ctx.fakes[+on.slice(1)];
  const args = (step.args || []).map(a => arg(ctx, a, step.call));
  try {
    const r = target[step.call].apply(target, args);
    if (/Sync$|^create.*Stream$/.test(step.call)) ctx.trace(`${label} = ${enc(r)}`);
  } catch (e) {
    ctx.trace(`${label} throws ${thrown(e)}`);
  }
}

// --- Loading graceful-fs over the fakes.
let loadFs = null, loadConstants = null;
const realLoad = Module._load;
Module._load = function (request, parent) {
  if (parent && parent.filename && parent.filename.startsWith(gfsDir + path.sep)) {
    if (request === 'fs') return loadFs;
    if (request === 'constants') return loadConstants;
  }
  return realLoad.apply(this, arguments);
};
const clearCache = () => { for (const k of Object.keys(require.cache)) if (k.startsWith(gfsDir + path.sep)) delete require.cache[k]; };

function fnv(h, s) {
  for (let i = 0; i < s.length; i++) { h ^= s.charCodeAt(i); h = Math.imul(h, 16777619) >>> 0; }
  return h;
}

function runCase(c) {
  const env = Object.assign({ platform: 'linux', uid: 1000, osymlink: null, clock: 1700000000000 }, c.env);
  const start = env.clock;
  const V = new Virtual(start);
  V.maxTasks = 1000000;
  const out = [];
  const ctx = { name: c.name, V, calls: 0, streams: 0, instances: [], fakes: [], trace: line => out.push((V.now - start) + ' ' + line) };
  const bases = c.bases || [{}];
  if (c.missing || c.script) bases[0] = Object.assign({}, bases[0], c.missing && { missing: c.missing }, c.script && { script: c.script });
  ctx.fakes = bases.map((b, i) => makeFake(ctx, i, b));

  const q0 = global[queueSymbol];
  if (q0 && q0.length) throw new Error(`case ${c.name}: the shared queue is not empty`);
  const saved = {
    now: Date.now, setTimeout: global.setTimeout, clearTimeout: global.clearTimeout, nextTick: process.nextTick,
    cwd: process.cwd, chdir: process.chdir, getuid: Object.getOwnPropertyDescriptor(process, 'getuid'),
    platform: Object.getOwnPropertyDescriptor(process, 'platform'), gfsPlatform: process.env.GRACEFUL_FS_PLATFORM,
  };
  let fired = 0, hash = 2166136261;
  try {
    Object.defineProperty(process, 'platform', { value: env.platform, configurable: true, enumerable: true, writable: false });
    if (env.gfsPlatform === undefined) delete process.env.GRACEFUL_FS_PLATFORM; else process.env.GRACEFUL_FS_PLATFORM = env.gfsPlatform;
    if (env.uid === null) delete process.getuid; else process.getuid = () => env.uid;
    Date.now = () => V.now;
    global.setTimeout = (fn, ms) => V.setTimeout(() => { fired++; hash = fnv(hash, (V.now - start) + ';'); fn(); }, ms);
    global.clearTimeout = t => V.clearTimeout(t);
    process.nextTick = fn => V.nextTick(fn);
    V.uncaught = e => ctx.trace('uncaught ' + thrown(e));

    for (const bi of c.load || [0]) {
      clearCache();
      loadFs = ctx.fakes[bi];
      loadConstants = env.osymlink === null ? {} : { O_SYMLINK: env.osymlink };
      ctx.instances.push(require(path.join(gfsDir, 'graceful-fs.js')));
    }
    clearCache();
    (c.steps || []).forEach((s, i) => V.after(s.at || 0, () => runStep(ctx, s, 's' + i)));
    V.run();
    if (ctx.broken) throw ctx.broken;
  } finally {
    Date.now = saved.now;
    global.setTimeout = saved.setTimeout;
    global.clearTimeout = saved.clearTimeout;
    process.nextTick = saved.nextTick;
    process.cwd = saved.cwd;
    process.chdir = saved.chdir;
    if (saved.getuid) Object.defineProperty(process, 'getuid', saved.getuid); else delete process.getuid;
    Object.defineProperty(process, 'platform', saved.platform);
    if (saved.gfsPlatform === undefined) delete process.env.GRACEFUL_FS_PLATFORM; else process.env.GRACEFUL_FS_PLATFORM = saved.gfsPlatform;
  }
  const q = global[queueSymbol];
  const left = q.length;
  q.length = 0;
  return {
    trace: out, end: V.now - start, timers: fired, timerHash: hash.toString(16).padStart(8, '0'), queue: left,
    has: ctx.instances.map(g => HAS.filter(n => typeof g[n] === 'function')),
  };
}

// --- The scheduler on its own: programs of timers ({timeout: ms, id}), host work ({after: ms, id}) and ticks
// ({tick: id}), each logging "<time> <id>" when it runs, then throwing its id if it has panic: true, or else
// running its "then" ops; {clear: id} clears a timer.
function runProgram(p) {
  const V = new Virtual(p.start || 0);
  const log = [];
  V.uncaught = e => log.push(V.now + ' uncaught ' + e);
  const timers = {};
  const fire = op => {
    const id = 'tick' in op ? op.tick : op.id;
    log.push(V.now + ' ' + id);
    if (op.panic) throw id;
    (op.then || []).forEach(exec);
  };
  const exec = op => {
    if ('timeout' in op) timers[op.id] = V.setTimeout(() => fire(op), op.timeout);
    else if ('after' in op) V.after(op.after, () => fire(op));
    else if ('tick' in op) V.nextTick(() => fire(op));
    else if ('clear' in op) V.clearTimeout(timers[op.clear]);
    else throw new Error('bad op ' + JSON.stringify(op));
  };
  p.ops.forEach(exec);
  V.run();
  return log;
}

const names = new Set();
for (const c of cases.cases) {
  if (names.has(c.name)) throw new Error('duplicate case ' + c.name);
  names.add(c.name);
}
const result = {
  node: process.version,
  scheduler: cases.scheduler.map(runProgram),
  cases: cases.cases.map(runCase),
};
console.log(JSON.stringify(result, null, 1));
