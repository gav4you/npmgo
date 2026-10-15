// Generates parity/opentelemetry/cases.json, the inputs parity/oracle_opentelemetry.cjs runs through
// @opentelemetry/api@1.9.0, @opentelemetry/core@2.11.0 and @opentelemetry/semantic-conventions@1.40.0.
//
// Run: node tools/gen_opentelemetry_cases.cjs > parity/opentelemetry/cases.json
//
// Values that are not plain JSON use the value-spec language of parity/build.cjs ({"$": "undef"}, {"$": "num", "v":
// "NaN"}, {"$": "obj", "props": [...]}, {"$": "arr", "items": [...]}, {"$": "date", "ms": ...}, ...).
'use strict';

const undef = { $: 'undef' };
const num = v => ({ $: 'num', v: String(v) });
const obj = (...props) => ({ $: 'obj', props });
const arr = (...items) => ({ $: 'arr', items });

const TID = '4bf92f3577b34da6a3ce929d0e0e4736';
const SID = '00f067aa0ba902b7';
const Z32 = '0'.repeat(32), Z16 = '0'.repeat(16);

// --- traceparent strings
const traceparents = [];
{
  const ok = `00-${TID}-${SID}-01`;
  traceparents.push(ok, `00-${TID}-${SID}-00`, `00-${TID}-${SID}-ff`, `00-${TID}-${SID}-0f`, `00-${TID}-${SID}-10`,
    `00-${TID}-${SID}-9a`, `01-${TID}-${SID}-01`, `fe-${TID}-${SID}-01`, `ff-${TID}-${SID}-01`, `0f-${TID}-${SID}-01`,
    `00-${Z32}-${SID}-01`, `00-${TID}-${Z16}-01`, `00-${Z32}-${Z16}-01`, `00-${TID.toUpperCase()}-${SID}-01`,
    `00-${TID}-${SID.toUpperCase()}-01`, `00-${TID}-${SID}-0F`, `0A-${TID}-${SID}-01`, `00-${TID.slice(1)}-${SID}-01`,
    `00-${TID}0-${SID}-01`, `00-${TID}-${SID.slice(1)}-01`, `00-${TID}-${SID}0-01`, `00-${TID}-${SID}-1`, `00-${TID}-${SID}-011`,
    `00_${TID}-${SID}-01`, `00-${TID}_${SID}-01`, `00-${TID}-${SID}_01`, `000-${TID}-${SID}-01`, `0-${TID}-${SID}-01`,
    `00-${TID.replace('f', 'g')}-${SID}-01`, `00-${TID}-${SID.replace('a', 'z')}-01`, `00-${TID}-${SID}-0g`, `g0-${TID}-${SID}-01`,
    '', '-', '00', `00-${TID}`, `00-${TID}-${SID}`, `00-${TID}-${SID}-`, `${TID}-${SID}-01`, 'not a traceparent',
    `00-${'0'.repeat(31)}1-${SID}-01`, `00-${TID}-${'0'.repeat(15)}1-01`, `00-1${'0'.repeat(31)}-${SID}-01`);
  // Trailing fields: forbidden for version 00, allowed (any characters but line terminators) for later versions.
  for (const v of ['00', '01', 'cc', 'fe']) {
    for (const tail of ['-', '-x', '-01', '-abc-def', '- ', '-\t', '-a b', '-é€', '-\u{1F600}', '-x\n', '-x\r', '-x ',
      '-x ', '-x\n\n', '-x\ny', '-\n', '-x\u0085', '-x ', '-x \n', 'x', '--', '-x\t ', ' -x', '-x﻿']) {
      traceparents.push(`${v}-${TID}-${SID}-01${tail}`);
    }
  }
  // Whitespace around the header: one character either side is allowed.
  for (const ws of [' ', '\t', '\n', '\r', '\v', '\f', ' ', '﻿', ' ', ' ', '　', ' ', ' ',
    ' ', ' ', ' ', '\u0085', '᠎', '​', '  ', 'x']) {
    traceparents.push(ws + ok, ok + ws, ws + ok + ws, ws + `cc-${TID}-${SID}-01-x` + ws);
  }
}

// --- carriers for the trace-context extract
const extract = [];
{
  const tp = `00-${TID}-${SID}-01`;
  const tps = [tp, `00-${TID}-${SID}-00`, `00-${TID}-${SID}-ff`, `cc-${TID}-${SID}-09-extra`, 'bad', '', ` ${tp} `];
  for (const p of tps) extract.push({ traceparent: p });
  for (const st of ['a=1', 'a=1,b=2', ' a = 1 , b=2 ', 'foo=bar,foo=baz', 'k=v,invalid,=x,y=,Z=1', 'tenant@vendor=v,1a@b=c,a@1=x',
    'a=1'.repeat(1) + ',b=' + 'x'.repeat(250), 'a=' + 'x'.repeat(255) + '!,b=2', '', ' ', ',,,', 'a=1,b=2,c=3,a=4',
    Array.from({ length: 40 }, (_, i) => `k${i}=v${i}`).join(','), 'a=x y,b= z,c=w ', 'k=vé', 'a=1\t,b=2']) {
    extract.push({ traceparent: tp, tracestate: st });
  }
  extract.push({ traceparent: tp, tracestate: arr('a=1', 'b=2') });
  extract.push({ traceparent: tp, tracestate: arr('a=1', null, undef, 'b=2') });
  extract.push({ traceparent: tp, tracestate: arr() });
  extract.push({ traceparent: tp, tracestate: arr(arr('a=1', 'b=2'), 'c=3') });
  extract.push({ traceparent: tp, tracestate: 5 });
  extract.push({ traceparent: tp, tracestate: obj(['a', '1']) });
  extract.push({ traceparent: tp, tracestate: true });
  extract.push({ traceparent: tp, tracestate: 0 });
  extract.push({ traceparent: arr(tp, `00-${TID}-${'1'.repeat(16)}-00`) });
  extract.push({ traceparent: arr('bad', tp) });
  extract.push({ traceparent: arr() });
  extract.push({ traceparent: arr(5) });
  extract.push({ traceparent: 5 });
  extract.push({ traceparent: obj(['x', 1]) });
  extract.push({ traceparent: null });
  extract.push({ traceparent: undef });
  extract.push({ traceparent: false });
  extract.push({});
  extract.push({ tracestate: 'a=1' });
}

// --- span contexts for the trace-context inject
const inject = [];
{
  for (const f of [undefined, 0, 1, 2, 15, 16, 255, 256, 4096, -1, -16, 1.5, 0.1, num('NaN'), num('Infinity'), num('-Infinity'), num('-0'), 1e21, 2 ** 53]) {
    inject.push({ sc: f === undefined ? { traceId: TID, spanId: SID } : { traceId: TID, spanId: SID, traceFlags: f } });
  }
  inject.push({ sc: { traceId: TID, spanId: SID, traceFlags: 1, traceState: 'a=1,b=2' } });
  inject.push({ sc: { traceId: TID, spanId: SID, traceFlags: 1, traceState: '' } });
  inject.push({ sc: { traceId: TID, spanId: SID, traceFlags: 1, traceState: 'bad' } });
  inject.push({ sc: { traceId: TID, spanId: SID, traceFlags: 1 }, suppressed: true });
  inject.push({ sc: { traceId: Z32, spanId: SID, traceFlags: 1 } });
  inject.push({ sc: { traceId: TID, spanId: Z16, traceFlags: 1 } });
  inject.push({ sc: { traceId: TID.toUpperCase(), spanId: SID.toUpperCase(), traceFlags: 1 } });
  inject.push({ sc: { traceId: 'short', spanId: SID, traceFlags: 1 } });
  inject.push({ noSpan: true });
}

// --- TraceState operations (core and api)
const traceStates = [];
{
  const ops = (...o) => o;
  const raws = ['', 'a=1', 'a=1,b=2,c=3', 'b=2,a=1', ' a = 1 ', 'a=1,a=2', 'a=1,,b=2', 'k=v,invalid,=x,Y=1', 'tenant@vendor=1',
    Array.from({ length: 40 }, (_, i) => `k${i}=v${i}`).join(','), Array.from({ length: 33 }, (_, i) => `k${i}=v`).join(','),
    'a=' + 'x'.repeat(255) + 'y,b=' + 'x'.repeat(240), 'a=' + 'x'.repeat(256), 'x'.repeat(600), ' a=1﻿',
    Array.from({ length: 20 }, (_, i) => `key${i}=` + 'v'.repeat(30)).join(',')];
  for (const raw of raws) traceStates.push({ raw, ops: ops(['serialize'], ['get', 'a'], ['get', 'k0'], ['get', 'nope']) });
  traceStates.push({ raw: undef, ops: ops(['serialize'], ['set', 'a', '1'], ['serialize']) });
  traceStates.push({ raw: 5, ops: ops(['serialize']) });
  traceStates.push({
    raw: 'a=1,b=2', ops: ops(['set', 'c', '3'], ['serialize'], ['set', 'a', 'x'], ['serialize'], ['unset', 'b'], ['serialize'],
      ['unset', 'nope'], ['get', 'a'], ['set', 'b', '2'], ['serialize'], ['unset', 'a'], ['unset', 'b'], ['unset', 'c'], ['serialize']),
  });
  traceStates.push({
    raw: '', ops: ops(['set', 'Bad', '1'], ['set', 'ok', 'bad,value'], ['set', 'ok', 'bad=value'], ['set', 'ok', ''], ['set', 'ok', 'trailing '],
      ['set', '', 'x'], ['set', 'a@b', 'v'], ['set', 'a/b*c-d_e', 'v'], ['serialize'], ['set', 'ok', 'é'], ['set', 'ok', 'x'.repeat(256)],
      ['set', 'ok', 'x'.repeat(257)], ['serialize']),
  });
  // Length accounting: grow towards 512 characters.
  const grow = [];
  for (let i = 0; i < 12; i++) grow.push(['set', `key${i}`, 'v'.repeat(40)]);
  grow.push(['serialize'], ['set', 'key0', 'w'], ['serialize'], ['set', 'last', 'v'.repeat(100)], ['serialize']);
  traceStates.push({ raw: '', ops: grow });
  const shrink = [['set', 'big', 'x'.repeat(200)], ['set', 'big2', 'y'.repeat(200)], ['set', 'big3', 'z'.repeat(100)], ['serialize'],
    ['unset', 'big2'], ['set', 'big3', 'z'.repeat(150)], ['serialize'], ['unset', 'big'], ['unset', 'big3'], ['set', 'only', 'o'.repeat(500)], ['serialize']];
  traceStates.push({ raw: '', ops: shrink });
  traceStates.push({ raw: 'a=1,a=2,a=3', ops: ops(['serialize'], ['unset', 'a'], ['serialize'], ['set', 'b', '1'], ['serialize']) });
  // The 512-character limit exactly (values are at most 256 characters): a=x{256} is 258, ",b=y{240}" makes 501,
  // ",c=z{9}" would make 513, ",c=z{8}" makes 512.
  traceStates.push({
    raw: '', ops: ops(['set', 'a', 'x'.repeat(256)], ['set', 'b', 'y'.repeat(240)], ['set', 'c', 'z'.repeat(9)], ['set', 'c', 'z'.repeat(8)],
      ['serialize'], ['set', 'b', 'y'.repeat(241)], ['set', 'b', 'y'.repeat(240)]),
  });
  // unset takes the separator off: 258 + ",b=y{200}" is 461, unsetting b gives 258 again, and ",c=z{251}" makes 512.
  traceStates.push({
    raw: '', ops: ops(['set', 'a', 'x'.repeat(256)], ['set', 'b', 'y'.repeat(200)], ['unset', 'b'], ['set', 'c', 'z'.repeat(252)],
      ['set', 'c', 'z'.repeat(251)], ['serialize'], ['unset', 'a'], ['unset', 'c'], ['set', 'd', 'w'.repeat(256)], ['serialize']),
  });
  // A header of exactly 512 characters (253 + "," + 258), and of 513.
  traceStates.push({ raw: 'a=' + 'x'.repeat(251) + ',b=' + 'y'.repeat(256), ops: ops(['serialize'], ['set', 'c', '1'], ['serialize']) });
  traceStates.push({ raw: 'a=' + 'x'.repeat(252) + ',b=' + 'y'.repeat(256), ops: ops(['serialize'], ['get', 'a']) });
  traceStates.push({ raw: Array.from({ length: 33 }, (_, i) => `k${i}=v`).join(','), ops: ops(['set', 'new', 'v'], ['serialize'], ['get', 'k32']) });
}

// --- baggage headers for the baggage extract
const baggageExtract = [];
{
  for (const h of ['a=1', 'a=1,b=2', ' a = 1 , b = 2 ', 'key1=value1;metadata1,key2=value2;m=2;n', 'a=1;', 'a=1; ;x', 'a=%20b%3Bc',
    'a%3Db=1', '%E2%82%AC=%F0%9F%98%80', 'a=%', 'a=%zz,b=2', '%C0%80=1,b=2', '%EF%BF%BD=%EF%BF%BD,b=2', 'a=b=c', '=1', 'a=', ' =1', 'a= ', ',,a=1,,',
    'a', '', ' ', 'b=1,a=2,2=x,1=y,b=3', '__proto__=1', '%5F%5Fproto%5F%5F=1,b=2', 'constructor=1,toString=2', 'a=1,a=2,a=3',
    'ké=v€', 'a=x;é', 'a=1\t', ' a=1', 'a=1,' + 'b=' + 'x'.repeat(4094), 'a=1,' + 'b=' + 'x'.repeat(4093),
    'x='.concat('y'.repeat(4096)), 'a=1;' + 'm'.repeat(4100),
    Array.from({ length: 200 }, (_, i) => `k${i}=v${i}`).join(','),
    Array.from({ length: 179 }, (_, i) => `k${i}=v`).join(',') + ',bad,last=1,after=2',
    Array.from({ length: 4 }, (_, i) => `k${i}=` + 'x'.repeat(2045)).join(','),
    Array.from({ length: 4 }, (_, i) => `k${i}=` + 'x'.repeat(2044)).join(','),
    Array.from({ length: 5 }, (_, i) => `k${i}=` + 'x'.repeat(2000)).join(',') + ',z=1',
    // Exactly 8192 units in all (2048 + 3 × (1 + 2047)), and one unit more.
    ['k0=' + 'x'.repeat(2045), ...[1, 2, 3].map(i => `k${i}=` + 'x'.repeat(2044))].join(','),
    ['k0=' + 'x'.repeat(2045), ...[1, 2].map(i => `k${i}=` + 'x'.repeat(2044)), 'k3=' + 'x'.repeat(2045)].join(','),
    'a=\u{1F600}'.repeat(1) + ',b=' + '\u{1F600}'.repeat(2046)]) {
    baggageExtract.push({ header: h });
  }
  baggageExtract.push({ header: arr('a=1', 'b=2') });
  baggageExtract.push({ header: arr('a=1,b=2', 'a=3') });
  baggageExtract.push({ header: arr('k=' + 'x'.repeat(4000), 'j=' + 'y'.repeat(4000), 'i=' + 'z'.repeat(300), 'h=1') });
  baggageExtract.push({ header: arr(Array.from({ length: 100 }, (_, i) => `a${i}=1`).join(','), Array.from({ length: 100 }, (_, i) => `b${i}=1`).join(',')) });
  baggageExtract.push({ header: arr() });
  baggageExtract.push({ header: arr('') });
  baggageExtract.push({ header: arr(5, 'a=1') });
  baggageExtract.push({ header: arr('a=1', null) });
  baggageExtract.push({ header: arr('a=1', undef) });
  baggageExtract.push({ header: arr(arr(), 'a=1') });
  baggageExtract.push({ header: arr(arr('a=1')) });
  baggageExtract.push({ header: arr(obj(['x', 1]), 'a=1') });
  baggageExtract.push({ header: 5 });
  baggageExtract.push({ header: true });
  baggageExtract.push({ header: obj(['a', '1']) });
  baggageExtract.push({ header: null });
  baggageExtract.push({ header: undef });
  baggageExtract.push({ absent: true });
}

// --- baggage entries for the baggage inject: [key, value, metadata?]
const baggageInject = [];
{
  const e = (...entries) => entries;
  const sets = [
    e(['a', '1']), e(['a', '1'], ['b', '2']), e(['a b', 'c;d,e=f'], ['é', '€\u{1F600}']), e(['a', '1', 'meta']),
    e(['a', '1', ''], ['b', '2', 'x;y=z'], ['c', '3', 'é €']), e(['b', '1'], ['a', '2'], ['2', 'x'], ['1', 'y'], ['b', '3']),
    e(['__proto__', 'x'], ['a', '1']), e(['k', '']), e(['', 'v']), e(['!~*\'()', '-_.']),
    e(['a', 'x'.repeat(4094)]), e(['a', 'x'.repeat(4095)]), e(['a', '1', 'm'.repeat(4092)]), e(['a', '1', 'm'.repeat(4093)]),
    e(['a', 'x'.repeat(4000)], ['b', 'y'.repeat(4000)], ['c', 'z'.repeat(300)], ['d', '1']),
    e(['a', '\u{1F600}'.repeat(1000)]), e(['a', '1', '\u{1F600}'.repeat(2046)]), e(['a', '1', '\u{1F600}'.repeat(2047)]),
    // Unencoded metadata: 8009 UTF-16 units in all (the limit counts them), 16009 UTF-8 bytes.
    e(['a', '1', '\u{1F600}'.repeat(2000)], ['b', '2', 'é'.repeat(4000)]),
    Array.from({ length: 200 }, (_, i) => [`k${i}`, `v${i}`]),
    [['big', 'x'.repeat(5000)], ...Array.from({ length: 185 }, (_, i) => [`k${i}`, 'v'])],
    [],
  ];
  for (const s of sets) {
    baggageInject.push({ entries: s, via: 'record' });
    baggageInject.push({ entries: s, via: 'set' });
  }
  baggageInject.push({ entries: e(['a', '1']), via: 'set', suppressed: true });
  baggageInject.push({ noBaggage: true });
}

// --- parseKeyPairsIntoRecord
const parseRecord = ['a=1', 'a=1,b=2', ' a = 1 ;meta, b=2', 'b=1,a=2,2=x,1=y,b=3', '__proto__=1,a=2', 'a=%E2%82%AC', 'a=%EF%BF%BD', 'a=%zz,b=2', 'a=',
  '=1', '', ',', 'a', 'ké=v', 'constructor=1', 5, null, undef, arr('a=1'), obj(['a', '1'])];

// --- composite propagators
const composite = [];
{
  const ctxSpec = { sc: { traceId: TID, spanId: SID, traceFlags: 1, traceState: 'a=1' }, baggage: [['k', 'v']] };
  const carrier = { traceparent: `00-${TID}-${SID}-01`, tracestate: 'x=1', baggage: 'k=v,j=w' };
  const lists = [[], ['trace'], ['baggage'], ['trace', 'baggage'], ['baggage', 'trace'], ['throwing'], ['trace', 'throwing', 'baggage'],
    ['stringThrower', 'trace'], ['noFields', 'trace'], ['dupFields', 'trace', 'baggage'], ['trace', 'trace'], ['setsTraceparent', 'trace'],
    ['trace', 'setsTraceparent']];
  for (const p of lists) {
    composite.push({ propagators: p, op: 'fields' });
    composite.push({ propagators: p, op: 'inject', ctx: ctxSpec });
    composite.push({ propagators: p, op: 'extract', carrier });
  }
  composite.push({ propagators: ['trace', 'baggage'], op: 'inject', ctx: { sc: ctxSpec.sc, baggage: ctxSpec.baggage, suppressed: true } });
  composite.push({ noConfig: true, op: 'fields' });
  composite.push({ noConfig: true, op: 'inject', ctx: ctxSpec });
  composite.push({ noConfig: true, op: 'extract', carrier });
}

// --- attributes
const sanitize = [
  obj(['a', 1], ['b', 'x'], ['c', true]),
  obj(['', 1], ['ok', 1]),
  obj(['n', null], ['u', undef], ['arr', arr(1, 2, 3)], ['mixed', arr(1, 'a')], ['withNull', arr(null, 'a', undef, 'b')]),
  obj(['obj', obj(['x', 1])], ['fn', { $: 'fn', name: 'f' }], ['sym', { $: 'sym', d: 'd' }], ['big', { $: 'big', v: '1' }], ['date', { $: 'date', ms: 0 }]),
  obj(['nested', arr(arr(1))], ['objs', arr(obj())], ['bools', arr(true, false, null)], ['empty', arr()], ['allNull', arr(null, undef)]),
  obj(['b', 1], ['a', 2], ['2', 3], ['1', 4], ['__proto__', 5]),
  obj(['num', num('NaN')], ['inf', num('Infinity')], ['negz', num('-0')], ['nums', arr(1, num('NaN'), num('-0'))]),
  obj(['strs', arr('a', 1)], ['bad', arr({ $: 'sym', d: 'x' })], ['fns', arr({ $: 'fn', name: 'g' })]),
  arr('x', 1, arr(2)),
  null, undef, 5, 'string', true, { $: 'fn', name: 'f' }, obj(),
];
const isAttributeValue = [null, undef, 1, 'x', true, false, 0, '', num('NaN'), arr(), arr(1, 2), arr('a', 'b'), arr(true), arr(1, 'a'),
  arr(null, 1), arr(undef, 'a', null), arr(null), arr(arr()), arr(obj()), obj(), { $: 'fn', name: 'f' }, { $: 'sym', d: 'x' },
  { $: 'big', v: '5' }, { $: 'date', ms: 1 }, arr(1, num('NaN'), num('Infinity')), arr(true, 1), arr({ $: 'big', v: '5' })];

// --- merge
const ref = id => ({ $: 'ref', id });
const merge = [
  [obj(['a', 1]), obj(['b', 2])],
  [obj(['a', 1], ['b', obj(['c', 1])]), obj(['b', obj(['d', 2])], ['e', 3])],
  [obj(['a', arr(1, 2)]), obj(['a', arr(3)])],
  [arr(1, 2), arr(3, 4)],
  [arr(1, arr(2)), obj(['0', 'x'], ['5', 'y'])],
  [obj(['a', 1]), undef],
  [obj(['a', 1]), null],
  [obj(['a', 1]), 5],
  [5, obj(['a', 1])],
  [undef, obj(['a', obj(['b', 1])])],
  [obj(['a', 1], ['b', 2]), obj(['a', undef])],
  [obj(['a', obj(['x', 1])]), obj(['a', null])],
  [obj(['a', obj(['x', 1])]), obj(['a', 'str'])],
  [obj(['a', 'str']), obj(['a', obj(['x', 1])])],
  [obj(['a', obj(['x', 1])]), obj(['a', arr(1)])],
  [obj(['a', arr(1)]), obj(['a', obj(['x', 1])])],
  [obj(['a', { $: 'date', ms: 5 }]), obj(['a', { $: 'date', ms: 6 }])],
  [obj(['a', obj(['x', 1])]), obj(['a', { $: 're', src: 'ab+c', flags: '' }])],
  [obj(['f', { $: 'fn', name: 'f1' }]), obj(['f', { $: 'fn', name: 'f2' }])],
  [obj(['f', obj(['x', 1])]), obj(['f', { $: 'fn', name: 'f2' }])],
  [{ $: 'fn', name: 'f' }, obj(['a', 1])],
  [obj(['a', 1]), { $: 'fn', name: 'f' }],
  [{ $: 'obj', ctor: 'Foo', props: [['a', 1]] }, obj(['b', 2])],
  [obj(['b', 2]), { $: 'obj', ctor: 'Foo', props: [['a', 1]] }],
  [{ $: 'obj', ctor: null, props: [['a', 1]] }, obj(['b', 2])],
  [obj(['a', { $: 'obj', ctor: 'Foo', props: [] }]), obj(['a', obj(['x', 1])])],
  [obj(['__proto__', 1], ['constructor', 2], ['prototype', 3], ['ok', 4]), obj(['__proto__', obj(['x', 1])], ['constructor', 5], ['prototype', 6], ['ok2', 7])],
  [obj(), obj(['toString', obj(['a', 1])], ['valueOf', arr(1)], ['hasOwnProperty', 5], ['isPrototypeOf', { $: 'fn', name: 'g' }])],
  [obj(['toString', obj(['x', 1])]), obj(['toString', obj(['a', 1])])],
  [{ $: 'obj', id: 'cyc', props: [['self', ref('cyc')], ['v', 1]] }, obj(['self', obj(['w', 2])])],
  [obj(['a', obj(['b', 1])]), { $: 'obj', id: 'c2', props: [['a', obj(['me', ref('c2')])]] }],
  [{ $: 'obj', id: 's', props: [['x', 1]] }, obj(['p', ref('s')], ['q', ref('s')])],
  [obj(['p', { $: 'obj', id: 's2', props: [['x', 1]] }], ['q', ref('s2')]), obj(['p', obj(['y', 2])], ['q', obj(['z', 3])])],
  [obj(['a', 1]), obj(['b', 2]), obj(['c', 3]), obj(['a', 4])],
  [obj(['a', obj(['b', 1])]), obj(['a', obj(['c', 2])]), obj(['a', obj(['b', 3])])],
  [obj(['big', { $: 'big', v: '5' }]), obj(['big', obj(['x', 1])])],
  [obj(['s', { $: 'sym', d: 's' }]), obj(['s', obj(['x', 1])])],
  [{ $: 'big', v: '1' }, obj(['a', 1])],
  [obj(['a', 1])],
  [],
  [obj(['n', num('NaN')], ['z', num('-0')]), obj(['n', num('-0')], ['m', num('Infinity')])],
  [obj(['b', 1], ['a', 2]), obj(['2', 'x'], ['1', 'y'], ['c', 3])],
];
// 25 levels of nesting on both sides (deeper than MAX_LEVEL = 20).
{
  let a = obj(['leaf', 1]), b = obj(['leaf2', 2]);
  for (let i = 0; i < 25; i++) { a = obj(['n', a]); b = obj(['n', b]); }
  merge.push([a, b]);
  let c = obj(['leaf', 1]), d = obj(['leaf2', 2]);
  for (let i = 0; i < 19; i++) { c = obj(['n', c]); d = obj(['n', d]); }
  merge.push([c, d]);
}

// --- time
const hr = (a, b) => arr(a, b);
const timeCases = [
  { perf: { timeOrigin: 1700000000000.5, now: [123.456] }, calls: [['getTimeOrigin'], ['hrTime'], ['hrTime', 1.5], ['hrTime', 0], ['hrTime', 999.9999995]] },
  { perf: { timeOrigin: 1700000000123.4567, now: [0.0001, 1e9] }, calls: [['hrTime'], ['hrTime']] },
  {
    perf: { timeOrigin: 1700000000000, now: [] }, calls: [
      ['millisToHrTime', 0], ['millisToHrTime', 1], ['millisToHrTime', 1500], ['millisToHrTime', 1500.0000004], ['millisToHrTime', 1500.0000005],
      ['millisToHrTime', -1500], ['millisToHrTime', -0.5], ['millisToHrTime', num('-0')], ['millisToHrTime', num('NaN')],
      ['millisToHrTime', num('Infinity')], ['millisToHrTime', 1700000000123.4567], ['millisToHrTime', 2 ** 53], ['millisToHrTime', 1e21],
      ['millisToHrTime', 0.0000004], ['millisToHrTime', 0.0000005], ['millisToHrTime', 999.9999996], ['millisToHrTime', -999.9999996],
      // 2^-7 ms is exactly 7812.5 ns: Math.round rounds halves up, also below zero.
      ['millisToHrTime', 0.0078125], ['millisToHrTime', -0.0078125], ['millisToHrTime', -1000.0078125], ['millisToHrTime', 1000.0234375],
    ],
  },
  { perf: { timeOrigin: 1700000000000, now: [-0.0078125] }, calls: [['hrTime'], ['hrTime', 0.0234375], ['hrTime', -0.0234375]] },
  {
    perf: { timeOrigin: 1700000000000, now: [1000] }, calls: [
      ['timeInputToHrTime', hr(1, 2)], ['timeInputToHrTime', 1000], ['timeInputToHrTime', 849999999999.9], ['timeInputToHrTime', 850000000000],
      ['timeInputToHrTime', 1700000000000], ['timeInputToHrTime', -5], ['timeInputToHrTime', { $: 'date', ms: 1700000000123 }],
      ['timeInputToHrTime', { $: 'date', ms: -1 }], ['timeInputToHrTime', 'x'], ['timeInputToHrTime', hr(1, 'x')], ['timeInputToHrTime', arr(1, 2, 3)],
      ['timeInputToHrTime', null], ['timeInputToHrTime', undef], ['timeInputToHrTime', obj()], ['timeInputToHrTime', num('NaN')],
    ],
  },
  {
    perf: { timeOrigin: 1700000000000, now: [] }, calls: [
      ['hrTimeDuration', hr(1, 500), hr(2, 400)], ['hrTimeDuration', hr(1, 0), hr(1, 0)], ['hrTimeDuration', hr(5, 999999999), hr(1, 0)],
      ['hrTimeDuration', hr(0, 0), hr(0, -1)], ['addHrTimes', hr(1, 600000000), hr(2, 400000000)], ['addHrTimes', hr(1, 600000000), hr(2, 399999999)],
      ['addHrTimes', hr(1, 999999999), hr(0, 999999999)], ['addHrTimes', hr(-1, 0), hr(0, -5)], ['addHrTimes', hr(0.5, 0.5), hr(0.5, 1e9)],
      ['hrTimeToNanoseconds', hr(1, 1)], ['hrTimeToNanoseconds', hr(1700000000, 123456789)], ['hrTimeToMicroseconds', hr(1700000000, 123456789)],
      ['hrTimeToMilliseconds', hr(1700000000, 123456789)], ['hrTimeToSeconds', hr(1700000000, 123456789)], ['hrTimeToSeconds', hr(0, 1)],
      ['hrTimeToMilliseconds', hr(-1, 5)], ['hrTimeToMicroseconds', hr(0.1, 0.1)], ['hrTimeToNanoseconds', hr(9007199, 254740993)],
    ],
  },
  {
    perf: { timeOrigin: 1700000000000, now: [] }, calls: [
      ['hrTimeToTimeStamp', hr(0, 0)], ['hrTimeToTimeStamp', hr(1557853200, 123456)], ['hrTimeToTimeStamp', hr(1557853200, 999999999)],
      ['hrTimeToTimeStamp', hr(1557853200, 1e9)], ['hrTimeToTimeStamp', hr(1557853200, 12345678901)], ['hrTimeToTimeStamp', hr(-1, 5)],
      ['hrTimeToTimeStamp', hr(1.5, 7)], ['hrTimeToTimeStamp', hr(1.0005, 7)], ['hrTimeToTimeStamp', hr(1557853200, -5)],
      ['hrTimeToTimeStamp', hr(1557853200, 1.5)], ['hrTimeToTimeStamp', hr(1557853200, num('NaN'))], ['hrTimeToTimeStamp', hr(253402300799, 1)],
      ['hrTimeToTimeStamp', hr(253402300800, 1)], ['hrTimeToTimeStamp', hr(-62135596800, 1)], ['hrTimeToTimeStamp', hr(-62135596801, 1)],
      ['hrTimeToTimeStamp', hr(-62198755200, 0)], ['hrTimeToTimeStamp', hr(8.64e12, 0)], ['hrTimeToTimeStamp', hr(-8.64e12, 0)],
      ['hrTimeToTimeStamp', hr(8.64e12 + 1, 0)], ['hrTimeToTimeStamp', hr(num('NaN'), 0)], ['hrTimeToTimeStamp', hr(num('-0'), 0)],
      ['hrTimeToTimeStamp', hr(1e21, 0)], ['hrTimeToTimeStamp', hr(-0.0004, 0)],
    ],
  },
  {
    perf: { timeOrigin: 1700000000000, now: [] }, calls: [
      ['isTimeInputHrTime', hr(1, 2)], ['isTimeInputHrTime', arr(1)], ['isTimeInputHrTime', arr(1, 2, 3)], ['isTimeInputHrTime', hr('1', 2)],
      ['isTimeInputHrTime', 5], ['isTimeInputHrTime', null], ['isTimeInputHrTime', hr(num('NaN'), num('Infinity'))],
      ['isTimeInput', hr(1, 2)], ['isTimeInput', 5], ['isTimeInput', num('NaN')], ['isTimeInput', { $: 'date', ms: 0 }], ['isTimeInput', '5'],
      ['isTimeInput', null], ['isTimeInput', undef], ['isTimeInput', arr(1)], ['isTimeInput', obj()],
    ],
  },
];

// --- environment
const envCases = [
  {
    env: { NUM: '42', NEG: '-1.5', HEX: '0x10', EXP: '1e3', SPACES: '  7  ', EMPTY: '', BLANK: '   ', BAD: 'abc', INF: 'Infinity', NAN: 'NaN', TAB: '\t5\n', NBSP: ' 5', NEL: '\u00855', BIN: '0b11', OCT: '0o17', DOT: '.5', QUOTE: "it's" },
    calls: ['NUM', 'NEG', 'HEX', 'EXP', 'SPACES', 'EMPTY', 'BLANK', 'BAD', 'INF', 'NAN', 'TAB', 'NBSP', 'NEL', 'BIN', 'OCT', 'DOT', 'QUOTE', 'MISSING'].map(k => ['getNumberFromEnv', k]),
  },
  {
    env: { S: 'value', SP: '  padded  ', EMPTY: '', BLANK: ' \t ', UNI: 'é€', NBSP: ' ' },
    calls: ['S', 'SP', 'EMPTY', 'BLANK', 'UNI', 'NBSP', 'MISSING'].map(k => ['getStringFromEnv', k]),
  },
  {
    env: { T: 'true', F: 'false', TU: 'TRUE', TS: '  True  ', ONE: '1', YES: 'yes', EMPTY: '', BLANK: '  ', DOTTED: 'İ', SIGMA: 'TRUEΣ', Q: "o'k", GARAY: 'TRUE\u{10D50}', BERIA: 'tRuE\u{16EA0}', NBSP: ' true ' },
    // GARAY and BERIA end in capital letters of scripts added in Unicode 16 and 17 (lower-cased in the warning).
    calls: ['T', 'F', 'TU', 'TS', 'ONE', 'YES', 'EMPTY', 'BLANK', 'DOTTED', 'SIGMA', 'Q', 'NBSP', 'GARAY', 'BERIA', 'MISSING'].map(k => ['getBooleanFromEnv', k]),
  },
  {
    env: { L: 'a,b,c', SP: ' a , b ,, c ', EMPTY: '', COMMAS: ',,,', BLANKS: ' , , ', ONE: 'one', NBSP: ' a ,b' },
    calls: ['L', 'SP', 'EMPTY', 'COMMAS', 'BLANKS', 'ONE', 'NBSP', 'MISSING'].map(k => ['getStringListFromEnv', k]),
  },
  {
    env: {},
    calls: ['toString', 'constructor', '__proto__', 'hasOwnProperty'].flatMap(k => ['getNumberFromEnv', 'getStringFromEnv', 'getBooleanFromEnv', 'getStringListFromEnv'].map(f => [f, k])),
  },
  { env: { toString: '5', valueOf: 'true' }, calls: [['getNumberFromEnv', 'toString'], ['getStringFromEnv', 'toString'], ['getBooleanFromEnv', 'valueOf']] },
];

// --- diagLogLevelFromString
const diagLevel = ['ALL', 'all', 'Verbose', 'debug', 'INFO', 'warn', 'error', 'none', 'NONE', 'bogus', '', ' info', 'ınfo', 'INFO ', null, undef];

// --- url matching
const re = (src, flags = '') => ({ re: src, flags });
const urlMatches = [
  ['http://a/b', 'http://a/b'], ['http://a/b', 'http://a/'], ['http://a/b', re('a/b')], ['http://a/b', re('^http://a/b$')],
  ['http://A/b', re('a', 'i')], ['http://A/b', re('a')], ['http://a/b', re('^b')], ['http://a/b', undef], ['', re('^$')], ['', ''],
  ['http://a/health', re('/health$')], ['http://a/healthz', re('/health$')],
];
const urlIgnored = [
  ['http://a/b', [re('/c$'), 'http://a/b']], ['http://a/b', [re('/c$'), 'http://a/']], ['http://a/b', []], ['http://a/b', undef],
  ['http://a/b', [re('x'), re('b$')]],
];

// --- error handler
const errSpec = (msg, ctor) => (ctor ? { $: 'err', ctor, msg } : { $: 'err', msg });
const exceptions = ['plain string', '', errSpec('boom'), errSpec(''), errSpec('bad type', 'TypeError'), errSpec('range', 'RangeError'),
  obj(['message', 'm'], ['code', 42], ['name', 'Custom']), obj(['code', 0], ['message', ''], ['flag', true], ['nested', obj(['x', 1])]),
  obj(['arr', arr(1, 2)], ['n', null], ['u', undef], ['f', false]), obj(['toString', 'x'], ['valueOf', 'y'], ['constructor', 'z'], ['hasOwnProperty', 'w'], ['ok', 'v']),
  obj(['b', 'x'], ['2', 'y'], ['a', 'z'], ['1', 'w']), obj(['quote', 'say "hi"\n\\'], ['uni', 'é€\u{1F600}'], ['ctl', '\u0001\u001f\u007f']),
  obj(), 42, 0, num('NaN'), true, false, null, undef, { $: 'timeoutError', msg: 'timed out' }];
const errorHandler = [
  { handler: 'default', exceptions },
  { handler: 'recording', exceptions: ['s', errSpec('e'), obj(['a', 1]), null, undef] },
  { handler: 'throwing', exceptions: ['s', errSpec('e')] },
  { handler: 'direct', exceptions },
];

// --- anchored clock
const anchoredClock = [
  { system: [1000], monotonic: [50, 60, 75.5, 40], calls: 3 },
  { system: [1700000000000.25], monotonic: [0.001, 1e9, num('NaN')], calls: 2 },
  { system: [num('NaN')], monotonic: [1, 2], calls: 1 },
  { system: [-5], monotonic: [10, 10], calls: 1 },
];

// --- async programs on the virtual clock
const asyncPrograms = [
  // callWithTimeout
  { kind: 'cwt', settle: { at: 5, ok: 'v' }, timeout: 10 },
  { kind: 'cwt', settle: { at: 15, ok: 'v' }, timeout: 10 },
  { kind: 'cwt', settle: { at: 10, ok: 'v' }, timeout: 10 },
  { kind: 'cwt', settle: { at: 5, err: 'e' }, timeout: 10 },
  { kind: 'cwt', settle: { at: 5, err: null }, timeout: 10 },
  { kind: 'cwt', settle: { at: 5, ok: undef }, timeout: 10 },
  { kind: 'cwt', settle: 'never', timeout: 10 },
  { kind: 'cwt', settle: 'never', timeout: 0 },
  { kind: 'cwt', settle: 'never', timeout: -5 },
  { kind: 'cwt', settle: { now: 'ok', value: 1 }, timeout: 10 },
  { kind: 'cwt', settle: { now: 'err', value: 1 }, timeout: 10 },
  { kind: 'cwt', settle: { plain: 7 }, timeout: 10 },
  { kind: 'cwt', settle: { at: 0, ok: 'zero' }, timeout: 1 },
  { kind: 'cwt', settle: { at: 1, ok: 'one' }, timeout: 1 },
  { kind: 'cwt', settle: { at: 3, ok: 'adopted', adopt: true }, timeout: 10 },
  // BindOnceFuture
  { kind: 'bof', callback: 'value', calls: [{ at: 0, args: ['a', 1] }, { at: 0, args: ['b'] }, { at: 5, args: [] }] },
  { kind: 'bof', callback: 'throw', calls: [{ at: 0, args: [] }, { at: 1, args: [] }] },
  { kind: 'bof', callback: 'throwNull', calls: [{ at: 0, args: [] }] },
  { kind: 'bof', callback: 'resolveLater', calls: [{ at: 0, args: ['x'] }, { at: 2, args: [] }] },
  { kind: 'bof', callback: 'rejectLater', calls: [{ at: 0, args: [] }] },
  { kind: 'bof', callback: 'thenable', calls: [{ at: 0, args: [] }] },
  { kind: 'bof', callback: 'undefined', calls: [{ at: 3, args: [] }] },
  { kind: 'bof', callback: 'value', calls: [] },
  // internal._export
  { kind: 'export', exporter: 'sync', arg: 'spans' },
  { kind: 'export', exporter: 'later', arg: arr(1, 2) },
  { kind: 'export', exporter: 'twice', arg: 'x' },
  { kind: 'export', exporter: 'throws', arg: 'x' },
  { kind: 'export', exporter: 'sync', arg: 'with manager', manager: true },
  { kind: 'export', exporter: 'later', arg: 'with manager', manager: true },
  { kind: 'export', exporter: 'never', arg: 'x' },
];

// --- rpc metadata, suppress tracing, unrefTimer
const rpc = [
  [['get'], ['set', { route: '/a' }], ['get'], ['setRoute', '/b'], ['get'], ['delete'], ['get']],
  [['set', {}], ['get'], ['set', { route: '' }], ['get']],
];
const suppress = [
  [['is'], ['suppress'], ['is'], ['unsuppress'], ['is'], ['suppress'], ['suppress'], ['is']],
  [['raw', true], ['is'], ['raw', 1], ['is'], ['raw', 'true'], ['is'], ['raw', false], ['is'], ['raw', null], ['is']],
];
const unref = [5, 0, num('NaN'), 'withUnref', 'withoutUnref', null, undef, 'str', true];

// --- api cases
const api = {};
api.context = [
  [['set', 'a', 1], ['get', 'a'], ['get', 'b'], ['set', 'b', undef], ['get', 'b'], ['set', 'a', 2], ['get', 'a'], ['delete', 'a'], ['get', 'a'], ['rootGet', 'a']],
  [['set', 'n', null], ['get', 'n'], ['set', 'z', 0], ['get', 'z'], ['delete', 'missing'], ['get', 'n'], ['set', 'o', obj(['x', 1])], ['get', 'o']],
];
api.keys = [['a', 'a'], ['a', 'b'], ['OpenTelemetry Context Key SPAN', 'OpenTelemetry Context Key SPAN'], ['', '']];
api.registry = [
  [['setTracer', 'p1'], ['setTracer', 'p2'], ['getTracerProvider'], ['getTracer', 'n', 'v'], ['disableTrace'], ['getTracerProvider'], ['setTracer', 'p2'], ['getTracer', 'n', '']],
  [['setPropagator', 'r1'], ['setPropagator', 'r2'], ['propagationFields'], ['disablePropagation'], ['propagationFields']],
  [['setMeter', 'm1'], ['setMeter', 'm2'], ['getMeter', 'n', 'v'], ['disableMetrics'], ['getMeter', 'n', 'v']],
  [['setManager'], ['setManager'], ['active'], ['disableContext'], ['active']],
  [['tamper', '1.3.0', false], ['setTracer', 'p1'], ['setPropagator', 'r1'], ['getTracerProvider'], ['propagationFields']],
  [['tamper', '1.10.0', true], ['setTracer', 'p1'], ['setMeter', 'm1'], ['getTracerProvider']],
  [['tamper', '1.9.0', true], ['setTracer', 'p1'], ['getTracerProvider']],
  [['tamper', '2.0.0', true], ['setTracer', 'p1'], ['diagError']],
  [['tamper', '1.9.0-rc.1', true], ['setPropagator', 'r1'], ['diagError']],
  [['tamper', '', true], ['setPropagator', 'r1'], ['diagError']],
];
api.semver = [];
for (const own of ['1.9.0', '1.0.0', '0.3.1', '1.9.0-rc.1', 'not.a.version', '01.09.00', '1.9.0\n']) {
  for (const g of ['1.9.0', '1.9.1', '1.10.0', '1.8.9', '2.0.0', '0.3.1', '0.3.2', '0.3.0', '0.4.1', '1.9.0-rc.1', '1.9.0-rc.2', 'x', '1.9', '01.09.00',
    '1.9.0\n', '1.9.0- ', '1.9.0-a\rb', '1.10.0-alpha', '']) api.semver.push([own, g]);
}
api.diag = [
  { setup: [['setLogger', 'level', 'ALL']], logs: true },
  ...['NONE', 'ERROR', 'WARN', 'INFO', 'DEBUG', 'VERBOSE'].map(l => ({ setup: [['setLogger', 'level', l]], logs: true })),
  ...[-1, 10000, 55, num('NaN'), num('Infinity'), 69.9, 70].map(n => ({ setup: [['setLogger', 'number', n]], logs: true })),
  { setup: [['setLogger', 'options', {}]], logs: true },
  { setup: [['setLogger', 'options', { logLevel: 'DEBUG' }]], logs: true },
  { setup: [['setLogger', 'options', { suppressOverrideMessage: true }]], logs: true },
  { setup: [['setLogger', 'default']], logs: true },
  { setup: [['setLogger', 'null']], logs: false },
  { setup: [['setLogger', 'level', 'ALL'], ['setLogger', 'level', 'WARN']], logs: true },
  { setup: [['setLogger', 'level', 'ALL'], ['setLogger', 'options', { logLevel: 'ALL', suppressOverrideMessage: true }]], logs: true },
  { setup: [['setLogger', 'level', 'ALL'], ['setLogger', 'self']], logs: true },
  { setup: [['setLogger', 'self']], logs: true },
  { setup: [['setLogger', 'level', 'ALL'], ['disable']], logs: true },
  { setup: [['setLogger', 'nullLogger', 'ALL']], logs: true },
  { setup: [['setLogger', 'level', 'ALL']], component: ['@opentelemetry/x', ''] },
  { setup: [['disable']], component: ['ns'] },
  { setup: [['setLogger', 'console', 'ALL']], console: 'full', logs: true },
  { setup: [['setLogger', 'console', 'ALL']], console: 'noDebug', logs: true },
  { setup: [['setLogger', 'console', 'ALL']], console: 'logOnly', logs: true },
  { setup: [['setLogger', 'console', 'ALL']], console: 'none', logs: true },
];
api.baggage = [
  [['create', [['a', '1'], ['b', '2', 'm']]], ['all'], ['get', 'a'], ['get', 'zz'], ['mutateEntry', 'a'], ['get', 'a'], ['set', 'c', '3'], ['all'], ['prev'],
    ['set', 'a', 'x'], ['all'], ['remove', 'b'], ['all'], ['removeMany', ['a', 'zz', 'c']], ['all'], ['clear'], ['all']],
  [['create', [['b', '1'], ['a', '2'], ['2', 'x'], ['1', 'y'], ['b', '3'], ['__proto__', 'p']]], ['all']],
  [['create', []], ['all'], ['set', '2', 'x'], ['set', '1', 'y'], ['all']],
  [['meta', 'm;x=1'], ['meta', ''], ['meta', 5], ['meta', null], ['meta', undef], ['meta', obj()], ['meta', true]],
  [['context']],
];
api.propagation = [
  { carrier: obj(['a', '1'], ['b', arr('x', 'y')], ['2', 'z']) },
  { carrier: null },
  { carrier: undef },
  { carrier: obj() },
  { global: true, carrier: obj(['traceparent', 'x']) },
  // The setter assigns: __proto__ on an ordinary carrier reaches Object.prototype's accessor (a string changes nothing),
  // and an own __proto__ property (as JSON.parse makes) is overwritten.
  { carrier: obj(['a', '1']), set: [['__proto__', 'x'], ['b', '2']] },
  { carrier: obj(['__proto__', 'own'], ['a', '1']), set: [['__proto__', 'x']] },
];
api.traceIds = ['4bf92f3577b34da6a3ce929d0e0e4736', '4BF92F3577B34DA6A3CE929D0E0E4736', Z32, 'x'.repeat(32), TID.slice(1), TID + '0', '', TID + '\n',
  '00f067aa0ba902b7', '00F067AA0BA902B7', Z16, SID.slice(1), SID + '0', 'g'.repeat(16), SID + '\n', '٠'.repeat(16)];
api.trace = [
  ['startSpan root'], ['startSpan valid parent'], ['startSpan invalid parent'], ['startSpan no context'], ['startActiveSpan arities'],
  ['proxy before delegate'], ['proxy after delegate'], ['proxy delegate returns nothing'], ['wrap and helpers'], ['span methods'],
  ['startActiveSpan with manager'], ['startActiveSpan bad fn'], ['global tracer'],
];
api.traceState = traceStates;
api.metrics = [['noop'], ['global']];

const cases = {
  semconv: { sampleKeys: ['', 'content-type', 'x-forwarded-for', 'a.b', 'é'] },
  api,
  core: {
    traceparent: traceparents, extract, inject, traceState: traceStates, baggageExtract, baggageInject, parseRecord, composite,
    sanitize, isAttributeValue, merge, time: timeCases, env: envCases, diagLevel, urlMatches, urlIgnored, errorHandler, anchoredClock,
    async: asyncPrograms, rpc, suppress, unref,
  },
};
process.stdout.write(JSON.stringify(cases, null, 1) + '\n');
