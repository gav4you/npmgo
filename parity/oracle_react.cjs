// Renders the React parity cases with the ORIGINAL react@19.2.4 and
// react-dom@19.2.4 server renderers (production CJS builds) and prints the
// results as JSON.
//
// Run: node parity/oracle_react.cjs <root> parity/react/cases.json > parity/react/expected.json
//
// <root> is a directory with a node_modules layout of the original tarballs (react-dom's server
// builds require("react") and require("react-dom")). Treat the tarballs as untrusted data: extract
// each into its own new, empty directory and run nothing from them except through this oracle:
//   mkdir -p <root>/node_modules/react <root>/node_modules/react-dom <root>/node_modules/scheduler
//   tar -xzf react-19.2.4.tgz      --strip-components=1 -C <root>/node_modules/react
//   tar -xzf react-dom-19.2.4.tgz  --strip-components=1 -C <root>/node_modules/react-dom
//   tar -xzf scheduler-0.27.0.tgz  --strip-components=1 -C <root>/node_modules/scheduler
// (GNU tar on Windows needs --force-local for paths like C:/...; scheduler is not loaded by the
// server builds but completes react-dom's dependency layout.) Recorded with Node v25.9.0.
//
// The case language (shared with internal/reactspec, which builds the same
// trees in Go) is documented in internal/reactspec/reactspec.go.
'use strict';
process.env.NODE_ENV = 'production';
const path = require('path');
const root = path.resolve(process.argv[2]);
const React = require(path.join(root, 'node_modules', 'react'));
const ReactDOM = require(path.join(root, 'node_modules', 'react-dom'));
const Server = require(path.join(root, 'node_modules', 'react-dom', 'server.node.js'));
const cases = require(path.resolve(process.argv[3])).cases;
// Neutral working directory so nothing machine-specific can leak into the output.
process.cwd = () => '/work/npmgo';

const tick = () => new Promise(r => setImmediate(r));
async function settle() { for (let i = 0; i < 40; i++) await tick(); }

function msg(e) { return e instanceof Error ? e.message : 'THROWN ' + String(e); }

class Throw { constructor(v) { this.v = v; } }

// setup builds one fresh instance of a case: components, contexts, promises.
function setup(c) {
  const env = { comps: {}, ctxs: {}, promises: {}, resolvers: {}, globals: {} };
  for (const [name, def] of Object.entries(c.ctxs || {})) {
    const ctx = React.createContext(evalExpr(def.default === undefined ? { $: 'undef' } : def.default, {}, env));
    if (def.displayName !== undefined) ctx.displayName = def.displayName;
    env.ctxs[name] = ctx;
  }
  // memo wraps another component, so memos are created after everything else.
  const comps = Object.entries(c.comps || {});
  for (const [name, def] of comps) if (def.kind !== 'memo') env.comps[name] = makeComp(name, def, env);
  for (const [name, def] of comps) if (def.kind === 'memo') env.comps[name] = makeComp(name, def, env);
  env.resolvers = [];
  for (const def of c.promises || []) {
    const name = def.n;
    const value = () => evalExpr(def.value === undefined ? { $: 'undef' } : def.value, {}, env);
    let p;
    switch (def.kind) {
      case 'pending': {
        let res, rej;
        p = new Promise((a, b) => { res = a; rej = b; });
        if (def.step !== undefined) env.resolvers.push({ step: def.step, run: () => (def.reject ? rej(value()) : res(value())) });
        if (def.reject) p.catch(() => {});
        break;
      }
      case 'resolved': p = Promise.resolve(value()); break;
      case 'rejected': p = Promise.reject(value()); p.catch(() => {}); break;
      case 'fulfilled': { const v = value(); p = Promise.resolve(v); p.status = 'fulfilled'; p.value = v; break; }
      case 'rejectedTagged': { const v = value(); p = Promise.reject(v); p.catch(() => {}); p.status = 'rejected'; p.reason = v; break; }
      case 'sync': p = { then(res) { res(value()); } }; break;
      case 'syncReject': p = { then(res, rej) { rej(value()); } }; break;
      default: throw new Error('bad promise kind ' + def.kind);
    }
    env.promises[name] = p;
  }
  return env;
}

function runResolvers(env, step) {
  for (const r of env.resolvers) if (r.step === step) r.run();
}

function maxStep(env) {
  let m = 0;
  for (const r of env.resolvers) if (r.step > m) m = r.step;
  return m;
}

function defineName(fn, name) {
  if (name !== undefined) Object.defineProperty(fn, 'name', { value: name });
  return fn;
}

function makeComp(name, def, env) {
  let C;
  switch (def.kind) {
    case 'fn': {
      C = defineName(function (props) {
        return runBody(def, { props }, env);
      }, def.name === undefined ? '' : def.name);
      break;
    }
    case 'class': {
      const Base = def.pure ? React.PureComponent : React.Component;
      C = ({ [def.name || '']: class extends Base {
        constructor(props, context) {
          super(props, context);
          if (def.state !== undefined) this.state = evalExpr(def.state, { props, context, self: this }, env);
        }
        render() { return runBody(def, { props: this.props, state: this.state, context: this.context, self: this }, env); }
      } })[def.name || ''];
      if (def.cwm) C.prototype.componentWillMount = function () { runStmts(def.cwm, { props: this.props, state: this.state, context: this.context, self: this }, env); };
      if (def.ucwm) C.prototype.UNSAFE_componentWillMount = function () { runStmts(def.ucwm, { props: this.props, state: this.state, context: this.context, self: this }, env); };
      if (def.snapshot) C.prototype.getSnapshotBeforeUpdate = function () { return null; };
      if (def.contextType !== undefined) C.contextType = env.ctxs[def.contextType];
      if (def.gdsfp !== undefined) C.getDerivedStateFromProps = (props, state) => callFn(evalExpr(def.gdsfp, {}, env), [props, state]);
      break;
    }
    case 'forwardRef': {
      const render = defineName(function (props, ref) { return runBody(def, { props, ref }, env); }, def.name === undefined ? '' : def.name);
      C = React.forwardRef(render);
      break;
    }
    case 'memo': C = React.memo(evalType(def.of, env), def.compare === undefined ? undefined : (a, b) => callFn(evalExpr(def.compare, {}, env), [a, b])); break;
    case 'lazy': C = React.lazy(() => {
      if (def.throws !== undefined) throw evalExpr(def.throws, {}, env);
      return env.promises[def.p];
    }); break;
    default: throw new Error('bad component kind ' + def.kind);
  }
  if (def.displayName !== undefined) C.displayName = def.displayName;
  if (def.defaultProps !== undefined) C.defaultProps = evalExpr(def.defaultProps, {}, env);
  return C;
}

function runBody(def, scope, env) {
  const r = runStmts(def.body || [], scope, env);
  if (r && r.ret) return r.value;
  return evalExpr(def.ret === undefined ? null : def.ret, scope, env);
}

function runStmts(stmts, scope, env) {
  for (const s of stmts) {
    if (s.let !== undefined) {
      const v = evalExpr(s.v, scope, env);
      if (Array.isArray(s.let)) s.let.forEach((n, i) => { scope[n] = v[i]; });
      else scope[s.let] = v;
    } else if (s.do !== undefined) {
      evalExpr(s.do, scope, env);
    } else if (s.if !== undefined) {
      const r = runStmts(evalExpr(s.if, scope, env) ? s.then || [] : s.else || [], scope, env);
      if (r && r.ret) return r;
    } else if (s.throw !== undefined) {
      throw evalExpr(s.throw, scope, env);
    } else if (s.return !== undefined) {
      return { ret: true, value: evalExpr(s.return, scope, env) };
    } else if (s.try !== undefined) {
      try { runStmts(s.try, scope, env); } catch (e) { scope[s.catch] = e; runStmts(s.handle || [], scope, env); }
    } else throw new Error('bad statement ' + JSON.stringify(s));
  }
  return null;
}

function callFn(f, args) { return f.apply(null, args); }

function evalType(t, env) {
  if (typeof t === 'string') return t;
  if (t && typeof t === 'object' && !Array.isArray(t)) {
    switch (t.$) {
      case 'frag': return React.Fragment;
      case 'suspense': return React.Suspense;
      case 'strict': return React.StrictMode;
      case 'profiler': return React.Profiler;
      case 'activity': return React.Activity;
      case 'suspenselist': return Symbol.for('react.suspense_list');
      case 'viewtransition': return Symbol.for('react.view_transition');
      case 'scope': return Symbol.for('react.scope');
      case 'legacyhidden': return Symbol.for('react.legacy_hidden');
      case 'comp': return env.comps[t.n];
      case 'ctx': case 'provider': return t.$ === 'ctx' ? env.ctxs[t.n] : env.ctxs[t.n].Provider;
      case 'consumer': return env.ctxs[t.n].Consumer;
    }
  }
  return evalExpr(t, {}, env);
}

function get(o, k) {
  if (o === null || o === undefined) throw new TypeError(`Cannot read properties of ${o} (reading '${String(k)}')`);
  return o[k];
}

function evalExpr(e, scope, env) {
  if (e === null || typeof e !== 'object') return e;
  if (Array.isArray(e)) return e.map(x => evalExpr(x, scope, env));
  const E = x => evalExpr(x, scope, env);
  switch (e.$) {
    case 'undef': return undefined;
    case 'num': return Number(e.v);
    case 'big': return BigInt(e.v);
    case 'sym': return Symbol(e.d);
    case 'symfor': return Symbol.for(e.k);
    case 'obj': { const o = {}; for (const [k, v] of e.p || []) o[k] = E(v); return o; }
    case 'el': {
      const args = [evalType(e.t, env), e.p === undefined || e.p === null ? null : E(e.p)];
      if (e.c !== undefined) for (const c of e.c) args.push(E(c));
      return React.createElement.apply(null, args);
    }
    case 'type': return evalType(e.t, env);
    case 'var': if (!(e.n in scope)) throw new Error('unbound ' + e.n); return scope[e.n];
    case 'prop': return scope.props[e.n];
    case 'get': return get(E(e.o), E(e.k));
    case 'cat': return e.a.map(E).reduce((a, b) => a + b, '');
    case 'add': return E(e.a) + E(e.b);
    case 'json': return String(JSON.stringify(E(e.v)));
    case 'str': return String(E(e.v));
    case 'typeof': return typeof E(e.v);
    case 'eq': return E(e.a) === E(e.b);
    case 'lt': return E(e.a) < E(e.b);
    case 'not': return !E(e.v);
    case 'if': return E(e.c) ? E(e.t) : E(e.f);
    case 'fn': {
      const ps = e.ps || [];
      return function () {
        const s = Object.assign({}, scope);
        ps.forEach((p, i) => { s[p] = arguments[i]; });
        const r = runStmts(e.body || [], s, env);
        if (r && r.ret) return r.value;
        return evalExpr(e.ret === undefined ? { $: 'undef' } : e.ret, s, env);
      };
    }
    case 'thunk': return () => E(e.v);
    case 'call': return callFn(E(e.f), (e.a || []).map(E));
    case 'hook': return callHook(e.h, (e.a || []).map(E));
    case 'api': return callApi(e.f, (e.a || []).map(E));
    case 'promise': return env.promises[e.n];
    case 'ctxv': return env.ctxs[e.n];
    case 'compv': return env.comps[e.n];
    case 'error': return new Error(e.m);
    case 'typeerror': return new TypeError(e.m);
    case 'blob': return new Blob(['x']);
    case 'gget': return env.globals[e.n];
    case 'gset': return (env.globals[e.n] = E(e.v));
    case 'iter': { const items = e.v.map(E); return { [Symbol.iterator]() { return items[Symbol.iterator](); } }; }
    case 'setState': return scope.self.setState(E(e.v));
    case 'action': {
      const fn = E(e.fn);
      const a = function () { return fn.apply(null, arguments); };
      if (e.formAction !== undefined) {
        const fa = E(e.formAction);
        a.$$FORM_ACTION = function (prefix) {
          const r = fa(prefix);
          if (r && r.data !== undefined) {
            const fd = new FormData();
            for (const [k, v] of r.data) fd.append(k, v);
            return Object.assign({}, r, { data: fd });
          }
          return r;
        };
      }
      if (e.sigEqual !== undefined) { const se = E(e.sigEqual); a.$$IS_SIGNATURE_EQUAL = function (x, y) { return se(x, y); }; }
      // A server reference's bind keeps $$FORM_ACTION; model it as returning the action itself.
      if (e.bindable) a.bind = function () { return a; };
      return a;
    }
  }
  throw new Error('bad expr ' + JSON.stringify(e));
}

function callHook(h, a) {
  switch (h) {
    case 'useState': return React.useState(a[0]);
    case 'useReducer': return a.length > 2 ? React.useReducer(a[0], a[1], a[2]) : React.useReducer(a[0], a[1]);
    case 'useMemo': return React.useMemo(a[0], a[1]);
    case 'useCallback': return React.useCallback(a[0], a[1]);
    case 'useRef': return React.useRef(a[0]);
    case 'useContext': return React.useContext(a[0]);
    case 'useId': return React.useId();
    case 'useSyncExternalStore': return React.useSyncExternalStore(a[0], a[1], a[2]);
    case 'use': return React.use(a[0]);
    case 'useDeferredValue': return a.length > 1 ? React.useDeferredValue(a[0], a[1]) : React.useDeferredValue(a[0]);
    case 'useTransition': return React.useTransition();
    case 'useOptimistic': return React.useOptimistic(a[0], a[1]);
    case 'useActionState': return a.length > 2 ? React.useActionState(a[0], a[1], a[2]) : React.useActionState(a[0], a[1]);
    case 'useFormState': return a.length > 2 ? ReactDOM.useFormState(a[0], a[1], a[2]) : ReactDOM.useFormState(a[0], a[1]);
    case 'useFormStatus': return ReactDOM.useFormStatus();
    case 'useEffect': return React.useEffect(a[0], a[1]);
    case 'useLayoutEffect': return React.useLayoutEffect(a[0], a[1]);
    case 'useInsertionEffect': return React.useInsertionEffect(a[0], a[1]);
    case 'useImperativeHandle': return React.useImperativeHandle(a[0], a[1], a[2]);
    case 'useDebugValue': return React.useDebugValue(a[0]);
    case 'useEffectEvent': return React.useEffectEvent(a[0]);
    case 'useMemoCache': return React.__COMPILER_RUNTIME.c(a[0]);
    case 'useCacheRefresh': return React.unstable_useCacheRefresh();
  }
  throw new Error('bad hook ' + h);
}

function callApi(f, a) {
  switch (f) {
    case 'Children.map': return React.Children.map(a[0], a[1]);
    case 'Children.forEach': return React.Children.forEach(a[0], a[1]);
    case 'Children.count': return React.Children.count(a[0]);
    case 'Children.toArray': return React.Children.toArray(a[0]);
    case 'Children.only': return React.Children.only(a[0]);
    case 'cloneElement': return React.cloneElement.apply(null, a);
    case 'isValidElement': return React.isValidElement(a[0]);
    case 'createRef': return React.createRef();
    case 'version': return React.version;
    case 'startTransition': return React.startTransition(a[0]);
    case 'cache': return React.cache(a[0]);
    case 'join': return a[0].join(a[1]);
    case 'map': return a[0].map((x, i) => a[1](x, i));
    case 'keys': return Object.keys(a[0]);
    case 'preload': return ReactDOM.preload(a[0], a[1]);
    case 'preloadModule': return a.length > 1 ? ReactDOM.preloadModule(a[0], a[1]) : ReactDOM.preloadModule(a[0]);
    case 'preinit': return ReactDOM.preinit(a[0], a[1]);
    case 'preinitModule': return a.length > 1 ? ReactDOM.preinitModule(a[0], a[1]) : ReactDOM.preinitModule(a[0]);
    case 'preconnect': return ReactDOM.preconnect(a[0], a[1]);
    case 'prefetchDNS': return ReactDOM.prefetchDNS(a[0]);
    case 'flushSync': return ReactDOM.flushSync(a[0]);
    case 'createPortal': return ReactDOM.createPortal(a[0], a[1]);
    case 'requestFormReset': return ReactDOM.requestFormReset(a[0]);
    case 'renderToString': return Server.renderToString(a[0]);
  }
  throw new Error('bad api ' + f);
}

function legacy(c, fn) {
  const env = setup(c);
  const out = {};
  const opts = c.opts || undefined;
  try { out.html = fn(evalExpr(c.tree, {}, env), opts); } catch (e) { out.error = msg(e); }
  return { out, env };
}

async function legacyMode(c, fn) {
  const { out, env } = legacy(c, fn);
  if (c.twice) {
    await settle();
    runResolvers(env, 1);
    await settle();
    try { out.html2 = fn(evalExpr(c.tree, {}, env), c.opts || undefined); } catch (e) { out.error2 = msg(e); }
  }
  return out;
}

function streamOpts(c, env, ev, onHeaders) {
  const so = (c.stream && c.stream.opts) || {};
  const o = {};
  for (const [k, v] of Object.entries(so)) {
    if (k === 'onHeaders') continue;
    o[k] = evalExpr(v, {}, env);
  }
  if (so.onHeaders) o.onHeaders = onHeaders;
  return o;
}

function stream(c) {
  return new Promise(async resolve => {
    const env = setup(c);
    const ev = [];
    const chunks = [];
    const sc = c.stream || {};
    const pipeWhen = sc.pipe || 'now';
    let finished = false;
    const finish = () => { if (!finished) { finished = true; } };
    const dest = {
      write(chunk) { chunks.push(Buffer.from(chunk).toString('utf8')); return true; },
      end() { ev.push('end'); finish(); },
      destroy(e) { ev.push('destroy:' + msg(e)); finish(); },
      on() {},
    };
    let s;
    const opts = Object.assign(streamOpts(c, env, ev, h => { ev.push('headers:' + JSON.stringify(h)); }), {
      onShellReady() { ev.push('shellReady'); if (pipeWhen === 'shell') s.pipe(dest); },
      onShellError(e) { ev.push('shellError:' + msg(e)); },
      onAllReady() { ev.push('allReady'); if (pipeWhen === 'all') s.pipe(dest); },
      onError(e) { ev.push('error:' + msg(e)); return sc.digest === 'msg' ? 'D:' + msg(e) : undefined; },
    });
    let tree;
    try { tree = evalExpr(c.tree, {}, env); } catch (e) { resolve({ setupError: msg(e) }); return; }
    try { s = Server.renderToPipeableStream(tree, opts); } catch (e) { resolve({ error: msg(e) }); return; }
    if (pipeWhen === 'now') s.pipe(dest);
    await settle();
    const last = Math.max(maxStep(env), sc.abort ? sc.abort.step : 0);
    for (let step = 1; step <= last; step++) {
      if (sc.abort && sc.abort.step === step) s.abort(sc.abort.reason === undefined ? undefined : evalExpr(sc.abort.reason, {}, env));
      runResolvers(env, step);
      await settle();
    }
    resolve({ html: chunks.join(''), events: ev });
  });
}

(async () => {
  const results = [];
  for (const c of cases) {
    const modes = c.modes || ['string', 'static', 'stream'];
    const r = { name: c.name };
    if (modes.includes('string')) r.string = await legacyMode(c, Server.renderToString);
    if (modes.includes('static')) r.static = await legacyMode(c, Server.renderToStaticMarkup);
    if (modes.includes('stream')) r.stream = await stream(c);
    results.push(r);
  }
  console.log(JSON.stringify({ node: process.version, react: React.version, reactDOM: Server.version, results }, null, 1));
})();
