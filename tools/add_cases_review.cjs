// Appends the review's parity cases: breakLength Infinity/NaN, integer-like key
// order, numericSeparator exponent quirk, negative/fractional maxStringLength and
// maxArrayLength, non-string and falsy error stacks, showHidden own properties
// ([length], [lastIndex], [stack]/[message], Symbol.toStringTag), iterator and
// namespace depth names, compact:true layout, circular references, and the
// %s/%o/%d/%i/%f/%j format edge cases.
// Run once: node tools/add_cases_review.cjs parity/nodeutil/cases.json
'use strict';
const fs = require('fs');
const p = process.argv[2];
const c = JSON.parse(fs.readFileSync(p, 'utf8'));
const O = (props, extra = {}) => ({ $: 'obj', props, ...extra });
const A = (items, extra = {}) => ({ $: 'arr', items, ...extra });
const num = v => ({ $: 'num', v: String(v) });
const err = (msg, extra = {}) => ({ $: 'err', msg, ...extra });
const wk = name => ({ $: 'wk', name });
const fn = (name, extra = {}) => ({ $: 'fn', name, ...extra });
const custom = (ret, extra = {}) => ({ $: 'custom', ret, ...extra });
const typed = (kind, items) => ({ $: 'typed', kind, items });
const hidden = { hidden: true };
const hole = { $: 'hole' };
const range = n => Array.from({ length: n }, (_, i) => i);
const longObj = O([['alpha', 'aaaaaaaaaaaaaaaaaaaa'], ['beta', 'bbbbbbbbbbbbbbbbbbbbbbbb'], ['gamma', 'cccccccccccccccccccccccc'], ['delta', 1]]);
const stackM = 'Error: m\n    at a (f.js:1:1)';
const urlB = {
  $: 'url',
  parts: { href: 'http://localhost/', origin: 'http://localhost', protocol: 'http:', username: '', password: '', host: 'localhost', hostname: 'localhost', port: '', pathname: '/', search: '', hash: '' },
  params: [],
  ctx: { href: 'http://localhost/', protocol_end: 5, username_end: 7, host_start: 7, host_end: 16, pathname_start: 16, search_start: 4294967295, hash_start: 4294967295, port: 4294967295, scheme_type: 0 },
};
const proxyA = { $: 'proxy', target: O([['a', 1]]), handler: O([]) };
const evald = { $: 'ns', name: 'evald', exports: [['a', O([['k', 1]])], ['b', 2], ['f', fn('f')]] };
const tdz = { $: 'ns', name: 'tdz', exports: [['x'], ['y'], ['z', { $: 'undef' }]], tdz: ['x', 'y'] };
// A namespace without a function export for the showHidden case: a function
// would print its own hidden properties, which the Go model does not have.
c.namespaces.plain = { source: 'export const b = 2; export let a = { k: 1 };', evaluate: true };
const plain = { $: 'ns', name: 'plain', exports: [['a', O([['k', 1]])], ['b', 2]] };

c.inspect.push(
  // --- breakLength Infinity / NaN (no int conversion) ---
  { v: longObj, o: { breakLength: 'Infinity' } },
  { v: longObj, o: { breakLength: 'Infinity', compact: true } },
  { v: A(range(30)), o: { breakLength: 'Infinity' } },
  { v: O([['a', O([['b', 'x'.repeat(100)]])]]), o: { breakLength: 'Infinity', compact: true } },
  { v: longObj, o: { breakLength: 'NaN' } },
  { v: 'line one\nline two '.repeat(5), o: { breakLength: 'Infinity' } },
  { v: O([['a', 1]]), o: { breakLength: 0 } },
  { v: 'x'.repeat(17), o: { breakLength: 0 } },
  { v: O([['a', O([['b', O([['c', O([['d', O([['e', 1]])]])]])]])]]), o: { depth: 'NaN' } },
  // --- integer-like own keys come first (OrdinaryOwnPropertyKeys) ---
  { v: O([['b', 1], ['1', 2], ['a', 3], ['0', 4], ['10', 5], ['-1', 6], ['01', 7], ['4294967294', 8], ['4294967295', 9], ['1000000000', 10]]) },
  { v: O([['b', 1], ['1', 2], ['h', 3, hidden]]), o: { showHidden: true } },
  { v: A([1, 2], { props: [['z', 1], ['4294967295', 'big'], ['y', 2]] }) },
  { v: { $: 'map', entries: [], props: [['z', 1], ['5', 2]] } },
  { v: err('x', { props: [['z', 1], ['5', 2]] }) },
  { v: fn('f', { props: [['z', 1], ['5', 2]] }) },
  { v: O([['b', 1], ['1', 2], ['a', 3]]), o: { sorted: true } },
  { v: O([['0', 1], ['1', 'a']], { args: true }) },
  // --- numericSeparator: Node's exponent quirk for non-integers without a '.' ---
  { v: num('1e-7'), o: { numericSeparator: true } },
  { v: num('-1e-7'), o: { numericSeparator: true } },
  { v: A([1.5e-7, 1.23e-18, 1e21, 1.5e21, num('-0'), num('NaN'), num('Infinity'), 1234567.1234567, -1234.5678, { $: 'big', v: '123456789' }]), o: { numericSeparator: true } },
  { v: typed('Float64Array', [1e-7, 1234.5]), o: { numericSeparator: true } },
  { v: num('1e-7'), o: { numericSeparator: true, colors: true } },
  // --- maxStringLength: negative (JS slice semantics) and fractional ---
  { v: 'abcdef', o: { maxStringLength: -1 } },
  { v: 'abcdef', o: { maxStringLength: -10 } },
  { v: 'abcdef', o: { maxStringLength: 2.5 } },
  { v: 'abcdef', o: { maxStringLength: 0, colors: true } },
  { v: 'ab\ncdef', o: { maxStringLength: 3 } },
  { v: 'abcdef', o: { maxStringLength: 'NaN' } },
  // --- maxArrayLength: fractional, zero, NaN, Infinity ---
  { v: A([1, 2, 3]), o: { maxArrayLength: 1.5 } },
  { v: { $: 'set', items: [1, 2, 3] }, o: { maxArrayLength: 1.5 } },
  { v: { $: 'map', entries: [[1, 1], [2, 2], [3, 3]] }, o: { maxArrayLength: 1.5 } },
  { v: A([1, hole, 3, 4]), o: { maxArrayLength: 1.5 } },
  { v: A([1, hole, 3, 4]), o: { maxArrayLength: 'Infinity' } },
  { v: { $: 'ab', bytes: [0, 0, 0] }, o: { maxArrayLength: 1.5 } },
  { v: { $: 'ab', bytes: [0, 0, 0] }, o: { maxArrayLength: 0 } },
  { v: { $: 'ab', bytes: [0, 0, 0] }, o: { maxArrayLength: 'Infinity' } },
  { v: typed('Uint8Array', [1, 2, 3]), o: { maxArrayLength: 0 } },
  { v: { $: 'set', items: [1, 2] }, o: { maxArrayLength: 0 } },
  { v: { $: 'weakset', items: [O([])] }, o: { maxArrayLength: 0, showHidden: true } },
  { v: A(range(101)), o: { maxArrayLength: 101 } },
  { v: A([1, 2, 3]), o: { maxArrayLength: 'NaN' } },
  { v: { $: 'set', items: [1, 2] }, o: { maxArrayLength: 'NaN' } },
  { v: A([1, 2, 3]), o: { maxArrayLength: -1 } },
  { v: A(Array.from({ length: 5 }, (_, i) => (i === 2 ? 'x' : hole))), o: { maxArrayLength: 1 } },
  // --- errors: non-string stacks are formatted with NaN recursion depth ---
  { v: err('m', { stack: O([['a', 1]]) }) },
  { v: err('m', { stack: O([['a', O([['b', O([['c', O([['d', 1]])]])]])], ['arr', A([1, 2])]]) }) },
  { v: err('m', { stack: O([['a', 1]]) }), o: { compact: true } },
  { v: err('m', { stack: O([['a', 1]]) }), o: { colors: true } },
  { v: err('m', { stack: A([1, 2]) }) },
  { v: err('m', { stack: O([['a', O([['b', O([['c', O([['d', 1]])]])]])]]) }), o: { depth: 1 } },
  { v: err('m', { stack: { $: 'proxy', target: O([['a', 1]]), handler: O([]) } }), o: { showProxy: true } },
  { v: err('m', { stack: custom(undefined, { mode: 'depth' }) }) },
  { v: err('m', { stack: stackM, cause: err('c', { stack: O([['x', 1]]) }) }) },
  { v: O([['e', err('m', { stack: O([['a', 1]]) })]]) },
  // --- errors: falsy stacks print as [name: message] ---
  { v: err('m', { stack: '' }) },
  { v: err('m', { stack: false }) },
  { v: err('m', { stack: 0 }) },
  { v: err('m', { stack: null }) },
  // --- errors with showHidden: own [stack] accessor, [message], [cause] order ---
  { v: err('m', { stack: stackM }), o: { showHidden: true } },
  { v: err('m'), o: { showHidden: true } },
  { v: err('m', { cause: 1, props: [['code', 'X']] }), o: { showHidden: true } },
  { v: err('m', { stack: stackM, cause: 'c', props: [['extra', 1]] }), o: { showHidden: true } },
  { v: err('many', { ctor: 'AggregateError', props: [['errors', A([err('one'), err('two', { ctor: 'TypeError' })]), hidden]] }), o: { showHidden: true } },
  { v: err('m', { props: [['s', 1, { sym: true }]] }) },
  { v: err('m', { stack: stackM, props: [['code', 'E']] }), o: { showHidden: true, colors: true } },
  // --- errors: stack text shapes and null prototypes ---
  { v: err('m', { name: 'Custom', stack: 'Custom: m\n    at a (f.js:1:1)' }) },
  { v: err('m', { stack: 'Custom: m\n    at a (f.js:1:1)', nullProto: true }) },
  { v: err('m', { stack: 'weird text', nullProto: true }) },
  { v: err('m', { stack: 'TypeError', nullProto: true }) },
  { v: err('m', { stack: stackM, nullProto: true }) },
  { v: err('m', { stack: 'Error: other\n    at a (f.js:1:1)' }) },
  { v: err('m', { ctor: 'TypeError', stack: stackM }) },
  { v: err('m', { name: 'MyError', stack: 'MyError: m\n    at a (f.js:1:1)' }) },
  { v: err('m', { ctor: 'MyErr', stack: stackM }) },
  { v: err('m', { cause: 'str' }) },
  { v: err('m', { props: [['errors', 'x']] }) },
  { v: err('m', { props: [['cause', 1]] }) },
  { v: err('r', { stack: 'Error: r\n' + '    at a (f.js:1:1)\n    at b (f.js:2:2)\n'.repeat(6) + '    at main (f.js:9:9)' }) },
  { v: err('r', { stack: 'Error: r\n' + '    at a (f.js:1:1)\n    at b (f.js:2:2)\n    at c (f.js:3:3)\n    at d (f.js:4:4)\n'.repeat(2) + '    at a (f.js:1:1)\n    at b (f.js:2:2)\n    at x (f.js:5:5)\n    at y (f.js:6:6)' }) },
  // cause whose [[Prototype]] chain reaches Error.prototype (not a native error): frames collapse
  { v: err('m', { stack: 'Error: m\n    at x (g.js:1:1)\n    at a (f.js:1:1)\n    at b (f.js:2:2)\n    at c (f.js:3:3)\n    at d (f.js:4:4)\n    at e (f.js:5:5)', cause: { $: 'chain', proto: wk('Error'), props: [['message', 'c'], ['stack', 'Error: c\n    at a (f.js:1:1)\n    at b (f.js:2:2)\n    at c (f.js:3:3)\n    at d (f.js:4:4)\n    at e (f.js:5:5)']] } }) },
  { v: err('m', { stack: stackM, cause: err('c', { stack: 'Error: c\n    at a (f.js:1:1)' }) }) },
  { v: { $: 'chain', proto: wk('TypeError'), props: [['message', 'hi']] } },
  { v: O([['e', err('x', { stack: 'Error: x\n    at a (f.js:1:1)' })]]) },
  // --- showHidden own properties of arrays, boxed strings and regexps ---
  { v: A([1, 2]), o: { showHidden: true } },
  { v: A([1, 2], { props: [['k', 'v']] }), o: { showHidden: true } },
  { v: A([3, 1], { props: [['z', 1], ['a', 2]] }), o: { showHidden: true, sorted: true } },
  { v: A([]), o: { showHidden: true } },
  { v: A([1, hole, 3]), o: { showHidden: true } },
  { v: O([['a', A([1])]]), o: { showHidden: true, depth: 0 } },
  { v: A([1, 2]), o: { showHidden: true, colors: true } },
  { v: { $: 'boxed', v: 'ab' }, o: { showHidden: true } },
  { v: { $: 'boxed', v: 'ab' }, o: { showHidden: true } },
  { v: { $: 'boxed', v: 1 }, o: { showHidden: true } },
  { v: { $: 're', src: 'x', flags: 'g' }, o: { showHidden: true } },
  { v: { $: 're', src: 'x', flags: 'g', lastIndex: 3 }, o: { showHidden: true } },
  { v: { $: 're', src: 'x', flags: 'g', props: [['k', 1]] }, o: { showHidden: true } },
  { v: { $: 'date', ms: 0 }, o: { showHidden: true } },
  { v: { $: 'map', entries: [[1, 2]] }, o: { showHidden: true } },
  { v: { $: 'promise', state: 'fulfilled', v: 1 }, o: { showHidden: true } },
  { v: { $: 'dv', bytes: [0], offset: 0 }, o: { showHidden: true } },
  // --- iterators: constructor name is Object; prototype toStringTag with showHidden ---
  { v: O([['a', { $: 'mapiter', kind: 'keys', entries: [[1, 2]] }]]), o: { depth: 0 } },
  { v: O([['a', { $: 'setiter', kind: 'values', items: [1] }]]), o: { depth: 0, sorted: true } },
  { v: { $: 'mapiter', kind: 'keys', entries: [[1, 2]] }, o: { showHidden: true } },
  { v: { $: 'setiter', kind: 'values', items: [1, 'two'] }, o: { showHidden: true, colors: true } },
  { v: { $: 'mapiter', kind: 'entries', entries: [[1, 2]] }, o: { showHidden: true } },
  { v: { $: 'mapiter', kind: 'keys', entries: [[1, 2]], props: [['extra', 1]] } },
  { v: { $: 'mapiter', kind: 'keys', entries: [['b', 1], ['a', 2]] }, o: { sorted: true } },
  // --- module namespaces: showHidden tag, depth name, colors ---
  { v: plain, o: { showHidden: true } },
  { v: O([['n', evald]]), o: { depth: 0 } },
  { v: tdz, o: { showHidden: true } },
  { v: tdz, o: { colors: true } },
  { v: evald, o: { sorted: true } },
  // --- depth-exceeded names of the other kinds ---
  { v: O([['p', { $: 'promise', state: 'fulfilled', v: 1 }], ['w', { $: 'weakmap', entries: [] }], ['d', { $: 'dv', bytes: [0], offset: 0 }], ['b', { $: 'ab', bytes: [0] }], ['s', { $: 'ab', bytes: [0], shared: true }], ['t', typed('Uint8Array', [1])], ['x', { $: 'boxed', v: 'x' }], ['dt', { $: 'date', ms: 0 }], ['dp', { $: 'date', ms: 0, props: [['x', 1]] }], ['r', { $: 're', src: 'x', flags: '', props: [['x', 1]] }], ['f', fn('f', { props: [['x', 1]] })], ['e', err('m', { props: [['x', 1]] })], ['ws', { $: 'weakset', items: [] }]]), o: { depth: 0 } },
  // --- compact: true layouts ---
  { v: O([['a', O([['b', O([['c', O([])]])]])], ['d', A([1, 2])]]), o: { compact: true, breakLength: 10 } },
  { v: O([['s', 'x'.repeat(100)]]), o: { compact: true } },
  { v: O([['g', 'x'.repeat(100), { getter: true }]]), o: { compact: true, getters: true } },
  { v: O([['s', 'x'.repeat(78)]]), o: { compact: true, colors: true } },
  { v: O([['s', '日'.repeat(45)]]), o: { compact: true } },
  { v: O([['s', '日'.repeat(37)]]), o: { compact: true } },
  { v: O([['s', '日'.repeat(36)]]), o: { compact: true } },
  { v: O([['é'.repeat(40), 1]]), o: { compact: true } },
  { v: A(range(30)), o: { compact: true } },
  { v: A([1, 2, 3, 4, 5, 6, 7, 8]), o: { compact: 1 } },
  { v: O([['a', A([1, 2])]]), o: { compact: 0 } },
  { v: O([['a', 1], ['self', { $: 'ref', id: 'r' }]], { id: 'r' }), o: { compact: true } },
  { v: fn('f', { id: 'r', props: [['self', { $: 'ref', id: 'r' }]] }), o: { compact: true } },
  // --- circular references in other containers ---
  { v: { $: 'map', id: 'm', entries: [[{ $: 'ref', id: 'm' }, 1]] } },
  { v: err('m', { id: 'e', props: [['self', { $: 'ref', id: 'e' }]] }) },
  { v: O([['b', O([['n', 'b'], ['a', O([['n', 'a'], ['b', { $: 'ref', id: 'b' }], ['self', { $: 'ref', id: 'a' }]], { id: 'a' })]], { id: 'b' })], ['a', { $: 'ref', id: 'a' }]]) },
  // --- array grouping ---
  { v: A([1, 2, 3, 4, 5, 6, 'a', 8]) },
  { v: A(range(10).map(i => ({ $: 'big', v: String(i * 100) }))) },
  { v: A([1, hole, 3, 4, 5, 6, 7, 8]) },
  { v: A(Array.from({ length: 8 }, () => '日本語')) },
  { v: A(Array.from({ length: 8 }, () => 'é')) },
  { v: A(Array.from({ length: 12 }, (_, i) => 'x'.repeat(i % 4 + 1))), o: { colors: true } },
  { v: A([1, 2, 3, 4, 5, 6, 7, { $: 'undef' }]) },
  // --- strings, symbols and keys ---
  { v: { $: 'sym', d: "a'b\n" } },
  { v: O([["it's\n", 1, { sym: true }]]) },
  { v: O([["it's", 1], ['a"b', 2], ['a\'b"c', 3], ['nl\n', 4], ['\u0085', 5], ['😀', 6]]) },
  { v: "it's " + 'x'.repeat(100) + '\n\u0085' },
  { v: "a'b\"c`d${e}" },
  { v: O([['x'.repeat(100), '']]) },
  { v: O([['a', O([['b', 'line one is long enough to wrap around the break length\nline two\n']])]]) },
  { v: A(['a\nb'.repeat(30)]) },
  { v: O([['k', 'x'.repeat(70)]]) },
  // --- custom inspect returns ---
  { v: custom(42) },
  { v: custom({ $: 'undef' }) },
  { v: O([['x', custom('a\nb')]]), o: { compact: false } },
  { v: custom(undefined, { mode: 'depth' }), o: { depth: 'Infinity' } },
  // --- constructor / tag combinations and getters on null prototypes ---
  { v: O([], { ctor: 'FooBar', tag: 'Foo' }) },
  { v: O([], { ctor: 'Foobar', tag: 'Foo' }) },
  { v: O([], { ctor: 'XFoo', tag: 'Foo' }) },
  { v: { $: 'chain', proto: null, props: [['g', 1, { getter: true }]] }, o: { getters: true } },
  { v: { $: 'chain', proto: wk('Object'), props: [['a', 1]] }, o: { showHidden: true } },
  { v: { $: 'weakset', items: [O([['b', 2]]), O([['a', 1]])] }, o: { showHidden: true, sorted: true } },
  { v: typed('Uint8Array', [1, 2]), o: { showHidden: true, colors: true } },
);

c.format.push(
  // --- %s: hasBuiltInToString decides between String(v) and inspect ---
  { args: ['%s', proxyA] },
  { args: ['%s', { $: 'proxy', target: O([]), handler: O([]), revoked: true }] },
  { args: ['%s', { $: 'proxy', target: A([1, 2]), handler: O([]) }] },
  { args: ['%s', { $: 'proxy', target: fn('f'), handler: O([]) }] },
  { args: ['%s', { $: 'proxy', target: proxyA, handler: O([]) }] },
  { args: ['%s', { $: 'promise', state: 'fulfilled', v: 42 }] },
  { args: ['%s', { $: 'weakmap', entries: [] }] },
  { args: ['%s', { $: 'ab', bytes: [0, 0] }] },
  { args: ['%s', { $: 'ab', bytes: [0, 0], shared: true }] },
  { args: ['%s', { $: 'dv', bytes: [0, 0], offset: 0 }] },
  { args: ['%s', { $: 'mapiter', kind: 'keys', entries: [[1, 2]] }] },
  { args: ['%s', urlB] },
  { args: ['%s', { $: 'usp', params: [['a', '1']] }] },
  { args: ['%s', { $: 'usp', params: [['a b', 'c&d=e'], ['ü', '😀'], ['*-._', '~!']] }] },
  { args: ['%s', custom('plain string')] },
  { args: ['%s', custom(O([['returned', A([1, 2])]]))] },
  { args: ['%s', typed('Int8Array', [1, -2])] },
  { args: ['%s', { $: 'typed', kind: 'BigInt64Array', items: ['1'], big: true }] },
  { args: ['%s', { $: 'u8', items: [1, 2] }] },
  { args: ['%s', typed('Float32Array', [])] },
  { args: ['%s', { $: 'boxed', v: { $: 'sym', d: 'q' } }] },
  { args: ['%s', { $: 'boxed', v: 'x' }] },
  { args: ['%s', { $: 'date', ms: null }] },
  { args: ['%s', evald] },
  { args: ['%s', { $: 'chain', proto: null, props: [['a', 1]] }] },
  { args: ['%s', err('x', { ctor: 'E' })] },
  { args: ['%s', { $: 'set', items: [1] }] },
  { args: ['%s', A([A([1, A([2])])])] },
  { args: ['%s', num('-0')] },
  { args: ['%s', 1e21] },
  { args: ['%s', 1234567], o: { numericSeparator: true } },
  { args: ['%s', O([], { ctor: 'Foo' })] },
  { args: ['%s', { $: 'obj', ctor: null, props: [['a', 1]] }] },
  // --- %o: showHidden + showProxy + depth 4 ---
  { args: ['%o', proxyA] },
  { args: ['%o', A([1, 2])] },
  { args: ['%o', O([['a', A([1, A([2])])]])] },
  { args: ['%o', { $: 'map', entries: [[1, A([2])]] }] },
  { args: ['%o', { $: 're', src: 'x', flags: 'g' }] },
  { args: ['%o', err('x', { stack: 'Error: x\n    at a (f.js:1:1)' })] },
  { args: ['%o', { $: 'boxed', v: 'ab' }] },
  { args: ['%o', O([['a', O([['b', O([['c', O([['d', O([['e', O([['f', 1]])]])]])]])]])]])] },
  // --- %d / %i / %f: ToNumber / parseInt / parseFloat edge cases ---
  { args: ['%d', { $: 'boxed', v: 5 }] },
  { args: ['%d', { $: 'boxed', v: '7' }] },
  { args: ['%d', { $: 'boxed', v: true }] },
  { args: ['%d', { $: 'boxed', v: { $: 'big', v: '5' } }] },
  { args: ['%d', '0x_1f'] },
  { args: ['%d', '0b1_1'] },
  { args: ['%d', '0xFFFFFFFFFFFFFFFFFFFF'] },
  { args: ['%d', '\u00855'] },
  { args: ['%d', '﻿5'] },
  { args: ['%d', ' 5 '] },
  { args: ['%d', ' 5 '] },
  { args: ['%d', '5\u0085'] },
  { args: ['%d', A([{ $: 'boxed', v: '7' }])] },
  { args: ['%d', A([A([5])])] },
  { args: ['%d', A([null])] },
  { args: ['%d', A([{ $: 'date', ms: 0 }])] },
  { args: ['%d', { $: 'date', ms: null }] },
  { args: ['%d', { $: 'date', ms: 5 }] },
  { args: ['%d', 1e-7], o: { numericSeparator: true } },
  { args: ['%d', '1e400'] },
  { args: ['%d', '.5'] },
  { args: ['%i', '0xg'] },
  { args: ['%i', '0x'] },
  { args: ['%i', '\u008512'] },
  { args: ['%i', '﻿12'] },
  { args: ['%i', { $: 'date', ms: 0 }] },
  { args: ['%i', { $: 'boxed', v: 12.9 }] },
  { args: ['%i', A([A([12])])] },
  { args: ['%i', '-0x1f'] },
  { args: ['%i', '0xFFFFFFFFFFFFFFFFFFFFFF'] },
  { args: ['%i', '12345678901234567890123'] },
  { args: ['%i', '-0'] },
  { args: ['%f', '\u00851.5'] },
  { args: ['%f', '﻿1.5'] },
  { args: ['%f', { $: 'date', ms: 0 }] },
  { args: ['%f', A([1.5])] },
  { args: ['%f', true] },
  { args: ['%f', '1.e5'] },
  // --- %j: JSON.stringify details ---
  { args: ['%j', '<>&  \b\f\u000b\u0001\u007f\u0080'] },
  { args: ['%j', O([['b', 1], ['1', 2], ['a', 3]])] },
  { args: ['%j', evald] },
  { args: ['%j', typed('Uint8Array', [1, 2])] },
  { args: ['%j', A([{ $: 'boxed', v: 's' }, { $: 'boxed', v: 1 }, { $: 'boxed', v: false }])] },
  { args: ['%j', { $: 'map', entries: [[1, 2]] }] },
  { args: ['%j', err('e')] },
  { args: ['%j', { $: 'date', ms: 0 }] },
  { args: ['%j', A([num('NaN'), num('Infinity'), num('-0')])] },
  { args: ['%j', O([['a', { $: 'undef' }], ['b', fn('b')], ['c', { $: 'sym', d: 'x' }], ['d', A([{ $: 'undef' }])]])] },
  { args: ['%j', O([['g', 1, { getter: true }], ['h', 2, hidden], ['s', 3, { sym: true }]])] },
  { args: ['%j', { $: 're', src: 'x', flags: '' }] },
  { args: ['%j', { $: 'chain', proto: null, props: [['a', 1]] }] },
  { args: ['%j', { $: 'date', ms: null }] },
  { args: ['%j', 'str'] },
  // --- argument handling ---
  { args: [1, 'a %s', 2] },
  { args: ['%s %s %s', 'a', 'b'] },
  { args: ['%%%s', 'x'] },
  { args: ['%s%', 'x'] },
  { args: ['a%cb', 'css', 'tail'] },
);
fs.writeFileSync(p, JSON.stringify(c));
console.log('inspect cases:', c.inspect.length, 'format cases:', c.format.length);
