// Prints the names in inspect.js's private `builtInObjects` set (the
// capitalised globalThis names present when inspect.js was loaded), which
// internal/nodeutil/proto.go must mirror. The set is observed through
// util.format('%s'): an object whose prototype has its own toString and a
// constructor named <name> is inspected iff <name> is in the set, otherwise
// String(value) calls the toString.
// Run: node tools/builtin_objects.cjs
'use strict';
const util = require('util');
function inSet(name) {
  const C = { [name]: function () {} }[name];
  const proto = { toString() { return 'CUSTOM'; } };
  Object.defineProperty(proto, 'constructor', { value: C, writable: true, configurable: true });
  return util.format('%s', Object.create(proto)) !== 'CUSTOM';
}
const candidates = new Set(Object.getOwnPropertyNames(globalThis).filter(n => /^[A-Z][a-zA-Z0-9]+$/.test(n)));
for (const n of ['TypedArray', 'Iterator', 'Buffer', 'Float16Array', 'SharedArrayBuffer']) candidates.add(n);
const names = [...candidates].filter(inSet).sort();
console.log(process.version + ': ' + names.length + ' names');
console.log(names.join(' '));
