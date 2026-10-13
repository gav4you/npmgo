// Appends parity cases for prototype chains, buffers, typed arrays, URL,
// module namespaces, AggregateError, getters modes and sorted comparators.
// Run once: node tools/add_cases_batch2.cjs parity/nodeutil/cases.json
'use strict';
const fs = require('fs');
const p = process.argv[2];
const c = JSON.parse(fs.readFileSync(p, 'utf8'));
const O = (props, extra = {}) => ({ $: 'obj', props, ...extra });
const wk = name => ({ $: 'wk', name });
const fnc = (name, protoRef) => ({ $: 'ctorfn', name, protoRef });
const hidden = { hidden: true };

const urlA = {
  $: 'url',
  parts: { href: 'https://user:pw@example.com:8080/a/b?x=1&y=two#frag', origin: 'https://example.com:8080', protocol: 'https:', username: 'user', password: 'pw', host: 'example.com:8080', hostname: 'example.com', port: '8080', pathname: '/a/b', search: '?x=1&y=two', hash: '#frag' },
  params: [['x', '1'], ['y', 'two']],
  ctx: { href: 'https://user:pw@example.com:8080/a/b?x=1&y=two#frag', protocol_end: 6, username_end: 12, host_start: 15, host_end: 27, pathname_start: 32, search_start: 36, hash_start: 46, port: 8080, scheme_type: 2 },
};
const urlB = {
  $: 'url',
  parts: { href: 'http://localhost/', origin: 'http://localhost', protocol: 'http:', username: '', password: '', host: 'localhost', hostname: 'localhost', port: '', pathname: '/', search: '', hash: '' },
  params: [],
  ctx: { href: 'http://localhost/', protocol_end: 5, username_end: 7, host_start: 7, host_end: 16, pathname_start: 16, search_start: 4294967295, hash_start: 4294967295, port: 4294967295, scheme_type: 0 },
};

c.namespaces = {
  evald: { source: 'export const b = 2; export let a = { k: 1 }; export function f() {}', evaluate: true },
  tdz: { source: 'export let x = 1; export const y = 2; export var z = 3;', evaluate: false },
};

c.inspect.push(
  // --- prototype chains ---
  { v: { $: 'chain', proto: O([], { ctor: null }), props: [['a', 1]] } },
  { v: { $: 'chain', proto: null, props: [['a', 1]] } },
  { v: { $: 'chain', proto: { $: 'chain', proto: null, props: [['inherited', 1]] }, props: [['own', 2]] } },
  { v: { $: 'chain', proto: { $: 'chain', proto: null, props: [['inherited', 1]] }, props: [['own', 2]] }, o: { showHidden: true } },
  { v: { $: 'chain', proto: { $: 'chain', id: 'P', proto: wk('Object'), props: [['constructor', fnc('Foo', 'P'), hidden], ['shared', 'x'], ['method', { $: 'fn', name: 'method' }]] }, props: [['own', 1]] } },
  { v: { $: 'chain', proto: { $: 'chain', id: 'P', proto: wk('Object'), props: [['constructor', fnc('Foo', 'P'), hidden], ['shared', 'x'], ['method', { $: 'fn', name: 'method' }]] }, props: [['own', 1]] }, o: { showHidden: true } },
  { v: { $: 'chain', proto: { $: 'chain', id: 'P', proto: wk('Object'), props: [['constructor', fnc('Foo', 'P'), hidden], ['shared', 'x']] }, props: [['own', 1]] }, o: { showHidden: true, colors: true } },
  { v: { $: 'chain', proto: { $: 'chain', id: 'Q', proto: { $: 'chain', id: 'P', proto: wk('Object'), props: [['constructor', fnc('Base', 'P'), hidden], ['baseProp', 1]] }, props: [['constructor', fnc('Child', 'Q'), hidden], ['childProp', 2]] }, props: [['own', 3]] }, o: { showHidden: true } },
  { v: { $: 'chain', proto: { $: 'chain', proto: { $: 'chain', proto: null, props: [] }, props: [] }, props: [['deep', true]] } },
  { v: { $: 'chain', proto: { $: 'chain', proto: { $: 'chain', proto: null, props: [] }, props: [] }, props: [['deep', true]] }, o: { depth: -1 } },
  { v: { $: 'chain', proto: wk('Array'), props: [['x', 1]] } },
  { v: { $: 'chain', proto: wk('Map'), props: [] } },
  { v: { $: 'chain', proto: wk('Error'), props: [['message', 'hi']] } },
  { v: O([['nested', { $: 'chain', proto: { $: 'chain', proto: null, props: [] }, props: [['a', 1]] }]]), o: { depth: 0 } },
  // --- buffers / views / typed arrays ---
  { v: { $: 'ab', bytes: [] } },
  { v: { $: 'ab', bytes: [1, 2, 255, 16] } },
  { v: { $: 'ab', bytes: Array.from({ length: 120 }, (_, i) => i) } },
  { v: { $: 'ab', bytes: [1, 2, 3, 4, 5] }, o: { maxArrayLength: 2 } },
  { v: { $: 'ab', bytes: [9, 8], shared: true } },
  { v: { $: 'ab', bytes: [1, 2, 3], detached: true } },
  { v: { $: 'ab', bytes: [1, 2], props: [['tag', 'x']] } },
  { v: { $: 'dv', bytes: [1, 2, 3, 4], offset: 1, length: 2 } },
  { v: { $: 'typed', kind: 'Int8Array', items: [-1, 0, 127] } },
  { v: { $: 'typed', kind: 'Uint16Array', items: [1, 65535] } },
  { v: { $: 'typed', kind: 'Int32Array', items: [-5, 5] } },
  { v: { $: 'typed', kind: 'Float32Array', items: [1.5, -0.25] } },
  { v: { $: 'typed', kind: 'Float64Array', items: [3.141592653589793, -0] } },
  { v: { $: 'typed', kind: 'Uint8ClampedArray', items: [0, 255] } },
  { v: { $: 'typed', kind: 'BigInt64Array', items: ['-9', '9007199254740993'], big: true } },
  { v: { $: 'typed', kind: 'BigUint64Array', items: ['18446744073709551615'], big: true } },
  { v: { $: 'typed', kind: 'Float64Array', items: [] } },
  { v: { $: 'typed', kind: 'Uint8Array', items: [1, 2] }, o: { showHidden: true } },
  { v: { $: 'typed', kind: 'Int16Array', items: [-2, 300] }, o: { showHidden: true } },
  { v: { $: 'typed', kind: 'Float64Array', items: [] }, o: { showHidden: true } },
  { v: { $: 'typed', kind: 'BigInt64Array', items: ['-1'], big: true }, o: { showHidden: true } },
  { v: { $: 'typed', kind: 'Int32Array', items: [1234567, -7654321] }, o: { numericSeparator: true } },
  // --- URL / URLSearchParams ---
  { v: urlA }, { v: urlB },
  { v: urlA, o: { showHidden: true } },
  { v: urlA, o: { customInspect: false } },
  { v: O([['link', urlB]]), o: { depth: 0 } },
  { v: { $: 'usp', params: [] } },
  { v: { $: 'usp', params: [['a', '1'], ['b', 'two words']] } },
  { v: { $: 'usp', params: [['aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb'], ['c', 'dddddddddddddddd']] } },
  { v: O([['p', { $: 'usp', params: [['k', 'v']] }]]), o: { depth: 0 } },
  // --- module namespaces ---
  { v: { $: 'ns', name: 'evald', exports: [['a', O([['k', 1]])], ['b', 2], ['f', { $: 'fn', name: 'f' }]] } },
  { v: { $: 'ns', name: 'tdz', exports: [['x'], ['y'], ['z', { $: 'undef' }]], tdz: ['x', 'y'] } },
  // --- AggregateError ---
  { v: { $: 'err', ctor: 'AggregateError', msg: 'many', props: [['errors', { $: 'arr', items: [{ $: 'err', msg: 'one' }, { $: 'err', ctor: 'TypeError', msg: 'two' }] }, hidden]] } },
  // --- getters modes / sorted comparator ---
  { v: O([['ro', 1, { getter: true }], ['rw', 2, { getter: true, setter: true }]]), o: { getters: 'get' } },
  { v: O([['ro', 1, { getter: true }], ['rw', 2, { getter: true, setter: true }]]), o: { getters: 'set' } },
  { v: O([['a', 1], ['c', 3], ['b', 2]]), o: { sorted: 'reverse' } },
  { v: { $: 'arr', items: [3, 1, 2], props: [['z', 1], ['y', 2]] }, o: { sorted: 'reverse' } },
);
fs.writeFileSync(p, JSON.stringify(c));
console.log('inspect cases:', c.inspect.length);
