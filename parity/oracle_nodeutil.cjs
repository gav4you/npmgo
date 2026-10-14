// Runs Node's ORIGINAL util.inspect / util.format against the parity cases.
// Run: node --experimental-vm-modules parity/oracle_nodeutil.cjs parity/nodeutil/cases.json
'use strict';
const util = require('util');
const vm = require('vm');
const { build } = require('./build.cjs');
const c = require(require('path').resolve(process.argv[2]));
// Neutral working directory so recorded outputs don't contain the local machine path.
process.cwd = () => '/work/npmgo';
// Numeric options may be given as the strings 'Infinity' or 'NaN' (JSON has neither).
const opt = o => { if (!o) return undefined; const r = { ...o }; for (const k of ['depth', 'breakLength', 'maxArrayLength', 'maxStringLength']) { if (r[k] === 'Infinity') r[k] = Infinity; else if (r[k] === 'NaN') r[k] = NaN; } if (r.sorted === 'reverse') r.sorted = (a, b) => (a < b ? 1 : a > b ? -1 : 0); return r; };
(async () => {
  build.namespaces = {};
  for (const [name, def] of Object.entries(c.namespaces || {})) {
    const m = new vm.SourceTextModule(def.source);
    await m.link(() => { throw new Error('no imports'); });
    if (def.evaluate) await m.evaluate();
    build.namespaces[name] = m.namespace;
  }
  const inspect = c.inspect.map(k => { try { return util.inspect(build(k.v), opt(k.o)); } catch (e) { return 'THROW ' + e.message; } });
  const format = c.format.map(k => { try { return k.o ? util.formatWithOptions(opt(k.o), ...build(k.args)) : util.format(...build(k.args)); } catch (e) { return 'THROW ' + e.message; } });
  console.log(JSON.stringify({ node: process.version, cwd: process.cwd(), inspect, format }, null, 1));
})();
