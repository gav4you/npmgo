// Mutation check of the OpenTelemetry parity harness: applies each deliberate change below to the Go ports (and to the
// runtime helpers they run on), runs the parity tests of the package it targets, and restores the file with
// `git checkout -- <file>`. Every mutation should make the parity tests fail ("caught").
//
// Run from the repository root, on a clean tree (it refuses otherwise, since it restores files from git):
//   node tools/mutate_opentelemetry.cjs
// It exits with status 1 when a mutation survives, does not apply or does not build.
'use strict';
const fs = require('fs');
const path = require('path');
const { spawnSync } = require('child_process');

const root = process.cwd();
const status = spawnSync('git', ['status', '--porcelain'], { cwd: root, encoding: 'utf8' });
if (status.status !== 0) throw new Error('git status failed: ' + status.stderr);
if (status.stdout.trim() !== '') throw new Error('the working tree is not clean: commit or set aside your changes first');

const C = 'pkgs/opentelemetry/core', A = 'pkgs/opentelemetry/api', S = 'pkgs/opentelemetry/semantic-conventions';
const core = './' + C + '/', api = './' + A + '/', sem = './' + S + '/', inc = './' + S + '/incubating/';
// [description, file, text to replace (its first occurrence), replacement, package whose parity test must fail]
const M = [
  ['core: inject keeps pairs of exactly 4096 units', C + '/baggage.go', 'if jsvalue.Len16(pair) <= baggageMaxPerNameValuePairs {', 'if jsvalue.Len16(pair) < baggageMaxPerNameValuePairs {', core],
  ['core: extract skips entries of exactly 4096 units', C + '/baggage.go', 'if entryLength <= baggageMaxPerNameValuePairs {', 'if entryLength < baggageMaxPerNameValuePairs {', core],
  ['core: extract total limit > becomes >=', C + '/baggage.go', 'if totalSize+entrySize > baggageMaxTotalLength {', 'if totalSize+entrySize >= baggageMaxTotalLength {', core],
  ['core: serializeKeyPairs counts bytes, not UTF-16 units', C + '/baggage.go', 'if jsvalue.Len16(value) <= baggageMaxTotalLength {', 'if len(value) <= baggageMaxTotalLength {', core],
  ['core: ParseKeyPairsIntoRecord keeps __proto__', C + '/baggage.go', 'result.Put(kp.key, kp.value)', 'result.Set(kp.key, kp.value)', core],
  ['core: TraceState.set counts one separator too few', C + '/tracestate.go', 'newLength += 2', 'newLength += 1', core],
  ['core: TraceState.unset keeps the separator', C + '/tracestate.go', 'newLength--', 'newLength -= 0', core],
  ['core: TraceState parse holds 33 keys', C + '/tracestate.go', 'if len(vendorEntries.keys) >= maxTraceStateItems {', 'if len(vendorEntries.keys) > maxTraceStateItems {', core],
  ['core: version 00 accepts trailing fields', C + '/tracecontext.go', 'if version == "00" && hasMore {', 'if version == "01" && hasMore {', core],
  ['core: traceparent flags in decimal', C + '/tracecontext.go', 'jsnum.StringRadix(flags, 16)', 'jsnum.StringRadix(flags, 10)', core],
  ['core: extracted span context not remote', C + '/tracecontext.go', 'spanContext.IsRemote = true', 'spanContext.IsRemote = false', core],
  ['core: merge level cap 21', C + '/merge.go', 'const maxMergeLevel = 20', 'const maxMergeLevel = 21', core],
  ['core: merge reads no inherited Object.prototype methods', C + '/merge.go', 'if objectPrototypeMethods[key] {\n\t\treturn builtinFunction{key}', 'if false {\n\t\treturn builtinFunction{key}', core],
  ['core: flattenException takes Object.prototype names', C + '/errorhandler.go', '|| objectPrototypeMethods[name] ||', '||', core],
  ['core: millisToHrTime rounds half away from zero', C + '/time.go', 'nanos := jsnum.Round(', 'nanos := math.Round(', core],
  ['core: composite extract failure returns ROOT_CONTEXT', C + '/composite.go', 'out = ctx', 'out = api.ROOT_CONTEXT', core],
  ['core: getBooleanFromEnv does not lower-case', C + '/environment.go', 'raw = jsvalue.ToLowerCase(jsvalue.Trim(raw))', 'raw = jsvalue.Trim(raw)', core],
  ['core: env inherits no Object.prototype names', C + '/environment.go', 'if objectPrototypeMethods[key] || key == "__proto__" {', 'if false {', core],
  ['core: urlMatches substring for strings', C + '/environment.go', 'return url == m', 'return strings.Contains(url, m)', core],
  ['core: sanitizeAttributes shares arrays', C + '/attributes.go', 'out.Put(key, append([]any{}, jsvalue.Items(val)...))', 'out.Put(key, val)', core],
  ['core: callWithTimeout keeps the timer on success', C + '/async.go', 'host.ClearTimeout(timeoutHandle)\n\t\t\treturn result', '_ = timeoutHandle\n\t\t\treturn result', core],
  ['core: BindOnceFuture calls every time', C + '/async.go', 'if !b.isCalled {', 'if true {', core],
  ['core: _export does not suppress tracing', C + '/async.go', 'api.Ctx.With(SuppressTracing(api.Ctx.Active()), func', 'api.Ctx.With(api.Ctx.Active(), func', core],
  ['api: duplicate registration allowed', A + '/globals.go', 'if !allowOverride && jsvalue.Truthy(global.entries[typ]) {', 'if false && jsvalue.Truthy(global.entries[typ]) {', api],
  ['api: semver minor must be strictly lower', A + '/globals.go', 'if own.minor <= g.minor {', 'if own.minor < g.minor {', api],
  ['api: NaN log level lets everything through', A + '/diag.go', 'if present && maxLevel >= level {', 'if present && !(maxLevel < level) {', api],
  ['api: suppressOverrideMessage ignored', A + '/diag.go', 'if oldLogger != nil && !jsvalue.IsNull(oldLogger) && !suppress {', 'if oldLogger != nil && !jsvalue.IsNull(oldLogger) && (!suppress || true) {', api],
  ['api: DiagConsoleLogger verbose to console.debug', A + '/diag.go', 'func(c *ConsoleMethods) func(...any) { return c.Trace }', 'func(c *ConsoleMethods) func(...any) { return c.Debug }', api],
  ['api: TraceState truncation keeps the last 32', A + '/tracestate.go', 'for i := len(state.keys) - 1; i >= 0 && len(kept.keys) < maxTraceStateItems; i-- {', 'for i := 0; i < len(state.keys) && len(kept.keys) < maxTraceStateItems; i++ {', api],
  ['api: createBaggage keeps __proto__', A + '/baggage.go', 'record.Put(e.Key, e.Entry)', 'record.Set(e.Key, e.Entry)', api],
  ['api: NoopTracer ignores root', A + '/trace.go', 'if options != nil && options.Root {', 'if false {', api],
  ['api: ProxyTracer does not keep its delegate', A + '/trace.go', 't.delegate = tracer', '_ = tracer', api],
  ['api: isValidTraceId accepts all zeros', A + '/trace.go', 'return validTraceIdRe.MatchString(traceId) && traceId != INVALID_TRACEID', 'return validTraceIdRe.MatchString(traceId)', api],
  ['api: createContextKey makes a new symbol each time', A + '/context.go', 'return jsvalue.SymbolFor(description)', 'return jsvalue.NewSymbol(description)', api],
  ['semconv: one stable value', S + '/stable_attributes.go', 'const ATTR_HTTP_REQUEST_METHOD = "http.request.method"', 'const ATTR_HTTP_REQUEST_METHOD = "http.request.methods"', sem],
  ['semconv: createConstMap keeps dots', S + '/semanticconventions.go', 'strings.NewReplacer("-", "_", ".", "_")', 'strings.NewReplacer("-", "_")', sem],
  ['semconv: one incubating template', S + '/incubating/experimental_attributes.go', 'return "container.label." + key', 'return "container.label" + key', inc],
  ['jspromise: adopting a promise without the thenable job', 'internal/jspromise/promise.go', 'p.q.QueueMicrotask(func() {\n\t\t\tresolve, reject := p.resolvingFunctions()\n\t\t\tr.performThen', 'func(f func()) { f() }(func() {\n\t\t\tresolve, reject := p.resolvingFunctions()\n\t\t\tr.performThen', core],
  ['jspromise: reactions in reverse order', 'internal/jspromise/promise.go', 'for _, r := range reactions {', 'for i := range reactions {\n\t\tr := reactions[len(reactions)-1-i]', core],
  ['jsloop: microtasks never drained', 'internal/jsloop/virtual.go', '\t\tv.drainMicrotasks()\n', '\t\t_ = v.micro\n', core],
  ['jsvalue: decodeURIComponent accepts malformed UTF-8', 'internal/jsvalue/strings.go', 'if _, size := utf8.DecodeRune(seq); size != n {', 'if _, size := utf8.DecodeRune(seq); false && size != n {', core],
];

let failures = 0;
let caught = 0;
for (const [name, file, from, to, pkg] of M) {
  const full = path.join(root, file);
  const src = fs.readFileSync(full, 'utf8').replace(/\r\n/g, '\n');
  const at = src.indexOf(from);
  let verdict;
  if (at < 0) {
    verdict = 'NOT APPLIED (text not found)';
  } else {
    fs.writeFileSync(full, src.slice(0, at) + to + src.slice(at + from.length));
    try {
      const r = spawnSync('go', ['test', pkg, '-run', 'TestParity', '-count=1'], { cwd: root, encoding: 'utf8' });
      const out = r.stdout + r.stderr;
      if (/\[build failed\]|\[setup failed\]|cannot use|undefined:|declared and not used/.test(out)) verdict = 'BUILD ERROR';
      else if (r.status === 0) verdict = 'SURVIVED';
      else verdict = `caught (${(out.match(/^\s+\S+_test\.go:\d+: /gm) || []).length} failure reports)`;
    } finally {
      spawnSync('git', ['checkout', '--', file], { cwd: root });
    }
  }
  if (verdict.startsWith('caught')) caught++; else failures++;
  console.log(`${verdict.padEnd(32)} ${name}`);
}
const after = spawnSync('git', ['status', '--porcelain'], { cwd: root, encoding: 'utf8' });
console.log('working tree after: ' + (after.stdout.trim() || 'clean'));
console.log(`${caught} of ${M.length} mutations caught`);
process.exitCode = failures ? 1 : 0;
