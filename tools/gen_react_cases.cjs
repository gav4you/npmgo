// Generates parity/react/cases.json, the React server-rendering parity cases
// (the case language is documented in internal/reactspec/reactspec.go).
// Run: node tools/gen_react_cases.cjs > parity/react/cases.json
'use strict';

// ---- case-language helpers ------------------------------------------------
const U = { $: 'undef' };
const num = v => ({ $: 'num', v: String(v) });
const big = v => ({ $: 'big', v: String(v) });
const sym = d => ({ $: 'sym', d });
const symfor = k => ({ $: 'symfor', k });
const o = obj => ({ $: 'obj', p: Object.entries(obj) });
const isExpr = x => x !== null && typeof x === 'object' && !Array.isArray(x) && '$' in x;
const el = (t, p, ...c) => ({ $: 'el', t, p: p === null || p === undefined ? null : isExpr(p) ? p : o(p), c });
const v = n => ({ $: 'var', n });
const prop = n => ({ $: 'prop', n });
const hook = (h, ...a) => ({ $: 'hook', h, a });
const api = (f, ...a) => ({ $: 'api', f, a });
const fn = (ps, ret, body) => (body ? { $: 'fn', ps, body, ret } : { $: 'fn', ps, ret });
const thunk = x => ({ $: 'thunk', v: x });
const call = (f, ...a) => ({ $: 'call', f, a });
const cat = (...a) => ({ $: 'cat', a });
const json = x => ({ $: 'json', v: x });
const str = x => ({ $: 'str', v: x });
const get = (obj, k) => ({ $: 'get', o: obj, k });
const add = (a, b) => ({ $: 'add', a, b });
const eq = (a, b) => ({ $: 'eq', a, b });
const lt = (a, b) => ({ $: 'lt', a, b });
const iff = (c, t, f) => ({ $: 'if', c, t, f });
const err = m => ({ $: 'error', m });
const promise = n => ({ $: 'promise', n });
const ctxv = n => ({ $: 'ctxv', n });
const compv = n => ({ $: 'compv', n });
const gget = n => ({ $: 'gget', n });
const gset = (n, x) => ({ $: 'gset', n, v: x });
const iter = (...items) => ({ $: 'iter', v: items });
const C = n => ({ $: 'comp', n });
const ctx = n => ({ $: 'ctx', n });
const provider = n => ({ $: 'provider', n });
const consumer = n => ({ $: 'consumer', n });
const T = { frag: { $: 'frag' }, suspense: { $: 'suspense' }, strict: { $: 'strict' }, profiler: { $: 'profiler' },
  activity: { $: 'activity' }, list: { $: 'suspenselist' }, vt: { $: 'viewtransition' }, scope: { $: 'scope' }, hidden: { $: 'legacyhidden' } };
const letv = (n, x) => ({ let: n, v: x });
const doo = x => ({ do: x });
const fc = (body, ret, extra = {}) => Object.assign({ kind: 'fn', body, ret }, extra);
const Sus = (fallback, ...c) => el(T.suspense, { fallback }, ...c);
const UseP = n => ({ kind: 'fn', name: 'Use_' + n, body: [letv('x', hook('use', promise(n)))], ret: v('x') });

const cases = [];
const add_ = (name, tree, extra = {}) => cases.push(Object.assign({ name, tree }, extra));

// ---- text and escaping -----------------------------------------------------
add_('text: plain string root', 'hello');
add_('text: escaping', el('div', null, `<script>alert("x" & 'y')</script>`));
add_('text: adjacent strings get separators', el('div', null, 'a', 'b', 'c'));
add_('text: text around elements', el('div', null, 'a', el('b', null, 'x'), 'c', 'd'));
add_('text: numbers', el('p', null, 0, ' ', num('-0'), ' ', 1.5, ' ', 1e21, ' ', num('NaN'), ' ', num('Infinity'), ' ', 123456789012345680000));
add_('text: bigint child', el('p', null, big('12345678901234567890')));
add_('text: null/boolean/undefined children', el('div', null, 'a', null, true, false, U, 'b'));
add_('text: empty strings keep separator state', el('div', null, 'a', '', 'b', '', ''));
add_('text: nested arrays', el('div', null, [['a', 'b'], ['c', ['d']]]));
add_('text: unicode and emoji', el('span', null, 'héllo ✓ 😀    '));
add_('text: newlines and tabs', el('pre', null, 'line1\nline2\ttab'));
add_('text: root array', ['x', el('i', null, 'y'), 'z']);
add_('text: root number', 42);
add_('text: root null', null);
add_('text: only text in fragment', el(T.frag, null, 'a', 'b'));

// ---- attributes --------------------------------------------------------------
add_('attr: className htmlFor tabIndex', el('label', { className: 'c', htmlFor: 'i', tabIndex: 3 }, 'L'));
add_('attr: class (not className) passes through', el('div', { class: 'raw' }));
add_('attr: boolean attributes', el('div', null,
  el('input', { disabled: true, readOnly: false, required: '', hidden: 'x', autoFocus: true, multiple: 1, muted: 0 }),
  el('details', { open: true }), el('video', { autoPlay: true, controls: true, loop: false, playsInline: true, muted: true }),
  el('div', { inert: true, itemScope: true })));
add_('attr: capture and download', el('div', null, el('input', { capture: true }), el('input', { capture: 'user' }), el('a', { download: true }), el('a', { download: false }), el('a', { download: 'file.txt' })));
add_('attr: numeric cols rows size span', el('div', null,
  el('textarea', { cols: 5, rows: '3' }), el('textarea', { cols: 0, rows: -1 }), el('input', { size: 'abc' }), el('input', { size: true }),
  el('col', { span: num('NaN') }), el('input', { size: [7] })));
add_('attr: rowSpan and start', el('div', null, el('td', { rowSpan: 0 }), el('td', { rowSpan: 'x' }), el('ol', { start: -3 }), el('ol', { start: '2' })));
add_('attr: style object', el('div', { style: o({ color: 'red', marginTop: 10, opacity: 0.5, zIndex: 3, flex: 1, lineHeight: 2, width: 0, height: '', top: null, left: U, display: true, fontSize: ' 12px ' }) }));
add_('attr: style vendor prefixes and custom properties', el('div', { style: o({ msTransform: 'x', WebkitTransition: 'y', MozBoxSizing: 'z', '--main-color': ' blue ', '--n': 5, WebkitLineClamp: 2, scale: 2 }) }));
add_('attr: style escaping', el('div', { style: o({ fontFamily: '"Open" & <Sans>', background: 'url("a.png")' }) }));
add_('attr: empty style object', el('div', { style: o({}) }, 'x'));
add_('attr: style with only nullish values', el('div', { style: o({ color: null, top: U }) }));
add_('attr: style string is an error', el('div', { style: 'color:red' }));
add_('attr: style number is an error', el('div', { style: 5 }));
add_('attr: style array (indices as names)', el('div', { style: ['a', 'b'] }));
add_('attr: data and aria attributes', el('div', { 'data-x': 'a', 'data-flag': true, 'data-off': false, 'aria-hidden': true, 'aria-label': 'L', 'data-n': 5, 'data-obj': o({}) }));
add_('attr: unknown attributes', el('div', { foo: 'bar', bar: 5, baz: true, qux: false, arr: [1, 2, U, null], obj: o({ a: 1 }), nul: null, und: U }));
add_('attr: event handlers and on* names dropped', el('button', { onClick: fn([], U), onfoo: 'x', on: 'kept', ON: 'k2', onX: 1 }, 'b'));
add_('attr: invalid attribute names dropped', el('div', { 'a b': 1, '<x': 2, 'x"y': 3, 'ok-name': 4, ':colon': 5, '_u': 6, '9n': 7 }));
add_('attr: aliases', el('div', null,
  el('form', { acceptCharset: 'utf-8' }), el('meta', { httpEquiv: 'refresh', content: '5' }),
  el('img', { crossOrigin: 'anonymous', src: 'x.png', loading: 'lazy' }), el('svg', { strokeWidth: 2, fillOpacity: 0.5, clipPath: 'url(#c)' })));
add_('attr: xlink and xml attributes', el('svg', null, el('use', { xlinkHref: '#a', xlinkTitle: 't', xlinkRole: 'r', xlinkActuate: 'onLoad', xlinkArcrole: 'ar', xlinkShow: 'new', xlinkType: 'simple', xmlBase: 'b', xmlLang: 'en', xmlSpace: 'preserve', xmlnsXlink: 'http://www.w3.org/1999/xlink' })));
add_('attr: dangerouslySetInnerHTML', el('div', { dangerouslySetInnerHTML: o({ __html: '<b>raw & "kept"</b>' }) }));
add_('attr: dangerouslySetInnerHTML null __html', el('div', { dangerouslySetInnerHTML: o({ __html: null }) }));
add_('attr: dangerouslySetInnerHTML with children is an error', el('div', { dangerouslySetInnerHTML: o({ __html: 'x' }) }, 'child'));
add_('attr: dangerouslySetInnerHTML wrong shape is an error', el('div', { dangerouslySetInnerHTML: o({ html: 'x' }) }));
add_('attr: dangerouslySetInnerHTML string is an error', el('div', { dangerouslySetInnerHTML: '<b>' }));
add_('attr: empty href and src', el('div', null, el('a', { href: '' }, 'a'), el('img', { src: '' }), el('link', { href: '', rel: 'x' }), el('area', { href: '' }), el('script', { src: '' })));
add_('attr: javascript: URLs sanitized', el('div', null,
  el('a', { href: 'javascript:alert(1)' }, 'a'), el('a', { href: '  \n JaVaScRiPt:void(0)' }, 'b'), el('a', { href: 'jav\tascript:x' }, 'c'),
  el('form', { action: 'javascript:x' }), el('iframe', { src: 'javascript:x' }), el('svg', null, el('a', { xlinkHref: 'javascript:x' })),
  el('button', { formAction: 'javascript:x' }), el('a', { href: 'https://ok.example/?q=javascript:x' }, 'ok')));
add_('attr: value on non-form elements', el('div', null, el('li', { value: 3 }), el('data', { value: 'x' }), el('param', { value: true }), el('meter', { value: 0.5 })));
add_('attr: string-valued enumerated attributes', el('div', { contentEditable: true, spellCheck: false, draggable: 'true' }));
add_('attr: dropped props', el('div', { suppressHydrationWarning: true, suppressContentEditableWarning: true, ref: o({}), defaultValue: 'x', defaultChecked: true, innerHTML: 'no' }, 'x'));
add_('attr: symbol and function values dropped', el('div', { id: sym('s'), title: fn([], 1), 'data-s': symfor('x'), className: sym('c') }));
add_('attr: attribute order follows prop order', el('div', { 'data-b': 1, id: 'i', 'data-a': 2, className: 'c' }));
add_('attr: integer-like prop keys come first', el('div', { b: 'x', 10: 'ten', 2: 'two', a: 'y' }));
add_('attr: width height viewBox role dir', el('svg', { width: 10, height: '20', viewBox: '0 0 10 20', role: 'img', dir: 'rtl' }));
add_('attr: boolean width is dropped', el('img', { width: true, height: false, src: 'x.png', loading: 'lazy' }));

// ---- elements --------------------------------------------------------------------
add_('el: nested elements', el('main', null, el('section', { id: 's' }, el('h1', null, 'T'), el('p', null, 'P'))));
add_('el: void elements', el('div', null, el('br'), el('hr', { className: 'x' }), el('input', { type: 'text', name: 'n' }), el('wbr'), el('img', { src: 'a.png', alt: 'A', loading: 'lazy' }), el('area'), el('base', { href: '/' }), el('col'), el('embed'), el('source'), el('track'), el('keygen'), el('param')));
add_('el: void element with children is an error', el('br', null, 'x'));
add_('el: input with children is an error', el('input', null, 'x'));
add_('el: img with dangerouslySetInnerHTML is an error', el('img', { dangerouslySetInnerHTML: o({ __html: 'x' }) }));
add_('el: menuitem', el('menu', null, el('menuitem', { label: 'm' })));
add_('el: menuitem with children is an error', el('menuitem', null, 'x'));
add_('el: custom elements', el('my-element', { className: 'c', foo: 'bar', flag: true, off: false, obj: o({ a: 1 }), num: 3, style: o({ color: 'red' }), onClick: fn([], 1), ref: o({}), htmlFor: 'f' }, 'child'));
add_('el: custom element innerHTML', el('x-y', { dangerouslySetInnerHTML: o({ __html: '<i>i</i>' }) }));
add_('el: invalid tag is an error', el('div!', null));
add_('el: uppercase and odd tag names', el('div', null, el('DIV', null, 'u'), el('a.b', null, 'dot'), el('x:y', null, 'colon')));
add_('el: object element', el('object', { data: 'movie.swf', type: 'application/x-shockwave-flash' }, el('param', { name: 'p', value: 'v' })));
add_('el: object data javascript and empty', el('div', null, el('object', { data: 'javascript:x' }), el('object', { data: '' })));
add_('el: a with string children escaped', el('a', { href: '/x?a=1&b=2' }, '<y>'));
add_('el: button with string children', el('button', { type: 'submit' }, 'Go & go'));
add_('el: table parts', el('table', null, el('thead', null, el('tr', null, el('th', null, 'h'))), el('tbody', null, el('tr', null, el('td', null, 'd'))), el('colgroup', null, el('col'))));

// ---- components ------------------------------------------------------------------
const Greeting = fc([], el('span', null, 'Hello, ', prop('name'), '!'), { name: 'Greeting' });
add_('comp: function component with props', el(C('Greeting'), { name: 'World' }), { comps: { Greeting } });
add_('comp: nested components and children prop', el(C('Box'), { title: 'T' }, el(C('Greeting'), { name: 'A' }), 'tail'), {
  comps: { Greeting, Box: fc([], el('div', { className: 'box' }, el('h2', null, prop('title')), prop('children')), { name: 'Box' }) } });
add_('comp: returns null undefined false', el('div', null, el(C('N')), el(C('Un')), el(C('F'))), {
  comps: { N: fc([], null), Un: fc([], U), F: fc([], false) } });
add_('comp: returns array and fragment', el('ul', null, el(C('Items'))), {
  comps: { Items: fc([], [el('li', { key: 'a' }, 'A'), el(T.frag, { key: 'f' }, el('li', null, 'B'), el('li', null, 'C'))]) } });
add_('comp: defaultProps via createElement', el('div', null, el(C('D')), el(C('D'), { a: 'given' }), el(C('D'), { a: U }), el(C('D'), { a: null })), {
  comps: { D: fc([], el('i', null, json(v('props'))), { defaultProps: o({ a: 'default', b: 2 }), body: [] }) } });
add_('comp: text from component next to text', el('div', null, 'a', el(C('T1')), 'b'), { comps: { T1: fc([], 'mid') } });
add_('comp: component returning number', el('div', null, el(C('N1')), el(C('N1'))), { comps: { N1: fc([], 7) } });
add_('comp: render prop child', el(C('RP'), { render: fn(['x'], el('b', null, v('x'))) }), { comps: { RP: fc([], call(prop('render'), 'rp')) } });
add_('comp: props children function', el(C('FC'), null, fn(['n'], cat('n=', v('n')))), { comps: { FC: fc([], call(prop('children'), 5)) } });

// ---- fragments, keys, arrays, Children -------------------------------------------
add_('frag: fragment and keyed fragment', el('div', null, el(T.frag, null, 'a', el('b', null, 'b')), el(T.frag, { key: 'k' }, 'c')));
add_('frag: strict mode and profiler pass through', el(T.strict, null, el(T.profiler, { id: 'p', onRender: fn([], U) }, el('i', null, 'x'))));
add_('frag: legacy hidden passes through', el(T.hidden, { mode: 'hidden' }, el('i', null, 'shown')));
add_('keys: Children.toArray keys', el('div', null, api('join', api('map', api('Children.toArray', [el('a', { key: 'x' }), el('b'), [el('c', { key: 'y' }), el('d')], 'text', null, el('e', { key: 'a/b=c:d' })]), fn(['c', 'i'], cat(get(v('c'), 'key'), ';'))), '')), {
  comps: {} });
add_('keys: Children.map keys and prefixes', el('div', null, api('join', api('map', api('Children.map', [el('a', { key: 'x' }), el('b')], fn(['c', 'i'], [v('c'), el('z', { key: 'z' })])), fn(['c'], get(v('c'), 'key'))), ' ')));
add_('keys: Children.map returning keyed clone', el('div', null, api('Children.map', [el('i', { key: 'k1' }, 'a'), el('i', null, 'b')], fn(['c', 'i'], api('cloneElement', v('c'), o({ key: cat('n', v('i')) }))))));
add_('keys: Children.count', el('p', null, api('Children.count', ['a', ['b', null], el('i'), U, true, 0])));
add_('keys: Children.count null', el('p', null, str(api('Children.count', null))));
add_('keys: Children.map null returns null', el('p', null, json(api('Children.map', null, fn(['c'], v('c'))))));
add_('keys: Children.forEach', el('p', null, el(C('CE'), null, 'a', 'b', el('i'))), { comps: { CE: fc([doo(gset('n', 0)), doo(api('Children.forEach', prop('children'), fn(['c', 'i'], gset('n', add(gget('n'), 1)))))], cat('count:', gget('n'))) } });
add_('keys: Children.only', el(C('Only'), null, el('b', null, 'one')), { comps: { Only: fc([], api('Children.only', prop('children'))) } });
add_('keys: Children.only error', el(C('Only'), null, 'a', 'b'), { comps: { Only: fc([], api('Children.only', prop('children'))) } });
add_('keys: Children with iterable', el('p', null, api('Children.count', iter('a', 'b', 'c'))));
add_('keys: Children of object throws', el('p', null, api('Children.count', o({ a: 1, b: 2 }))));
add_('keys: cloneElement props and children', el('div', null, api('cloneElement', el('a', { href: 'x', key: 'k' }, 'old'), o({ title: 't' }), 'new1', 'new2')));
add_('keys: cloneElement no config', el('div', null, api('cloneElement', el('b', { id: 'i' }, 'kept'))));
add_('keys: cloneElement undefined child', el('div', null, api('cloneElement', el('b', null, 'gone'), null, U)));
add_('keys: isValidElement', el('p', null, json([api('isValidElement', el('a')), api('isValidElement', 'a'), api('isValidElement', o({}))])));
add_('keys: element key from number and null', el('p', null, json([get(el('a', { key: 5 }), 'key'), get(el('a', { key: null }), 'key'), get(el('a', { key: U }), 'key')])));
add_('keys: iterable children', el('ul', null, iter(el('li', null, 'a'), el('li', null, 'b'))));
add_('keys: empty iterable', el('ul', null, iter()));

// ---- context -------------------------------------------------------------------------
const Theme = { default: 'light' };
add_('ctx: default value', el(C('Show')), { ctxs: { Theme }, comps: { Show: fc([letv('t', hook('useContext', ctxv('Theme')))], el('i', null, v('t'))) } });
add_('ctx: provider value', el(ctx('Theme'), { value: 'dark' }, el(C('Show'))), { ctxs: { Theme }, comps: { Show: fc([letv('t', hook('useContext', ctxv('Theme')))], el('i', null, v('t'))) } });
add_('ctx: nested providers', el(provider('Theme'), { value: 'a' }, el(C('Show')), el(ctx('Theme'), { value: 'b' }, el(C('Show'))), el(C('Show'))), { ctxs: { Theme }, comps: { Show: fc([letv('t', hook('useContext', ctxv('Theme')))], el('i', null, v('t'))) } });
add_('ctx: consumer render prop', el(ctx('Theme'), { value: 'c' }, el(consumer('Theme'), null, fn(['val'], el('b', null, v('val'))))), { ctxs: { Theme } });
add_('ctx: consumer non-function child is an error', el(consumer('Theme'), null, 'x'), { ctxs: { Theme } });
add_('ctx: provider without value prop', el(ctx('Theme'), null, el(C('Show'))), { ctxs: { Theme }, comps: { Show: fc([letv('t', hook('useContext', ctxv('Theme')))], el('i', null, str(v('t')))) } });
add_('ctx: use(Context)', el(ctx('Theme'), { value: 'used' }, el(C('U1'))), { ctxs: { Theme }, comps: { U1: fc([letv('t', hook('use', ctxv('Theme')))], v('t')) } });
add_('ctx: context object rendered as child', el(ctx('Theme'), { value: 'child-val' }, ctxv('Theme')), { ctxs: { Theme } });
add_('ctx: two contexts interleaved', el(ctx('A'), { value: 'a1' }, el(ctx('B'), { value: 'b1' }, el(ctx('A'), { value: 'a2' }, el(C('Both'))), el(C('Both')))), {
  ctxs: { A: { default: 'a0' }, B: { default: 'b0' } }, comps: { Both: fc([letv('a', hook('useContext', ctxv('A'))), letv('b', hook('useContext', ctxv('B')))], el('i', null, cat(v('a'), '/', v('b')))) } });
add_('ctx: object values', el(ctx('Theme'), { value: o({ color: 'red' }) }, el(C('Col'))), { ctxs: { Theme }, comps: { Col: fc([letv('t', hook('useContext', ctxv('Theme')))], el('i', { style: v('t') }, 'x')) } });

// ---- hooks -----------------------------------------------------------------------------
const Show = (body, ret, extra) => fc(body, ret, extra);
add_('hooks: useState initial value and lazy init', el('div', null, el(C('S1')), el(C('S2'))), { comps: {
  S1: Show([letv(['s', 'set'], hook('useState', 5))], el('i', null, v('s'))),
  S2: Show([letv(['s', 'set'], hook('useState', fn([], 'lazy')))], el('i', null, v('s'))) } });
add_('hooks: render-phase updates re-render', el(C('Count')), { comps: {
  Count: Show([letv(['n', 'set'], hook('useState', 0)), { if: lt(v('n'), 3), then: [doo(call(v('set'), add(v('n'), 1)))] }, doo(gset('renders', add(iff(gget('renders'), gget('renders'), 0), 1)))],
    el('b', null, cat('n=', v('n'), ' renders=', gget('renders')))) } });
add_('hooks: render-phase updater functions', el(C('Upd')), { comps: {
  Upd: Show([letv(['n', 'set'], hook('useState', 1)), { if: eq(v('n'), 1), then: [doo(call(v('set'), fn(['p'], add(v('p'), 10)))), doo(call(v('set'), fn(['p'], add(v('p'), 100))))] }], el('b', null, v('n'))) } });
add_('hooks: too many re-renders', el(C('Loop')), { comps: { Loop: Show([letv(['n', 'set'], hook('useState', 0)), doo(call(v('set'), add(v('n'), 1)))], v('n')) } });
add_('hooks: more hooks during re-render is an error', el(C('More')), { comps: {
  More: Show([letv(['n', 'set'], hook('useState', 0)), { if: eq(v('n'), 0), then: [doo(call(v('set'), 1))], else: [letv('r', hook('useRef', 1))] }], v('n')) } });
add_('hooks: useReducer with init', el(C('Red')), { comps: {
  Red: Show([letv(['st', 'dispatch'], hook('useReducer', fn(['s', 'a'], add(v('s'), v('a'))), 2, fn(['x'], add(v('x'), 40)))),
    { if: lt(v('st'), 50), then: [doo(call(v('dispatch'), 5))] }], el('i', null, v('st'))) } });
add_('hooks: useReducer without init', el(C('Red')), { comps: { Red: Show([letv(['st', 'd'], hook('useReducer', fn(['s', 'a'], v('a')), 'init'))], v('st')) } });
add_('hooks: useMemo and useCallback', el(C('Memo')), { comps: {
  Memo: Show([letv('m', hook('useMemo', thunk(cat('computed', '!')), [1])), letv('cb', hook('useCallback', fn([], 'cb-result'), [])), letv('m2', hook('useMemo', thunk(5)))],
    el('i', null, v('m'), '|', call(v('cb')), '|', v('m2'))) } });
add_('hooks: useMemo reuses value across render-phase updates', el(C('MemoRe')), { comps: {
  MemoRe: Show([letv(['n', 'set'], hook('useState', 0)), letv('m', hook('useMemo', thunk(gset('calls', add(iff(gget('calls'), gget('calls'), 0), 1))), [1])),
    { if: lt(v('n'), 2), then: [doo(call(v('set'), add(v('n'), 1)))] }], cat('calls=', v('m'), ' n=', v('n'))) } });
add_('hooks: useMemo deps change recomputes', el(C('MemoDeps')), { comps: {
  MemoDeps: Show([letv(['n', 'set'], hook('useState', 0)), letv('m', hook('useMemo', thunk(gset('calls', add(iff(gget('calls'), gget('calls'), 0), 1))), [v('n')])),
    { if: lt(v('n'), 2), then: [doo(call(v('set'), add(v('n'), 1)))] }], cat('calls=', v('m'))) } });
add_('hooks: useRef', el(C('Ref')), { comps: { Ref: Show([letv('r', hook('useRef', 'initial'))], el('i', null, get(v('r'), 'current'))) } });
add_('hooks: useSyncExternalStore server snapshot', el(C('Store')), { comps: {
  Store: Show([letv('s', hook('useSyncExternalStore', fn(['cb'], fn([], U)), thunk('client'), thunk('server')))], el('i', null, v('s'))) } });
add_('hooks: useSyncExternalStore without server snapshot', el(C('Store')), { comps: {
  Store: Show([letv('s', hook('useSyncExternalStore', fn(['cb'], fn([], U)), thunk('client')))], v('s')) } });
add_('hooks: useDeferredValue', el(C('Def')), { comps: { Def: Show([letv('a', hook('useDeferredValue', 'value')), letv('b', hook('useDeferredValue', 'value', 'initial'))], cat(v('a'), ',', v('b'))) } });
add_('hooks: useTransition', el(C('Tr')), { comps: { Tr: Show([letv(['p', 'start'], hook('useTransition'))], json(v('p'))) } });
add_('hooks: startTransition during render throws', el(C('Tr')), { comps: { Tr: Show([letv(['p', 'start'], hook('useTransition')), doo(call(v('start'), fn([], U)))], 'x') } });
add_('hooks: useOptimistic', el(C('Opt')), { comps: { Opt: Show([letv(['s', 'set'], hook('useOptimistic', 'passthrough', fn(['a', 'b'], v('b'))))], v('s')) } });
add_('hooks: optimistic setter throws during render', el(C('Opt')), { comps: { Opt: Show([letv(['s', 'set'], hook('useOptimistic', 'p')), doo(call(v('set'), 'x'))], v('s')) } });
add_('hooks: useActionState with plain function', el(C('Act')), { comps: { Act: Show([letv(['s', 'act', 'pending'], hook('useActionState', fn(['prev', 'payload'], v('payload')), 'init'))], el('form', { action: v('act') }, cat(v('s'), ' ', json(v('pending'))))) } });
add_('hooks: useFormState alias', el(C('FS')), { comps: { FS: Show([letv(['s', 'act'], hook('useFormState', fn(['p'], 1), 'state'))], v('s')) } });
add_('hooks: useFormStatus', el(C('Status')), { comps: { Status: Show([letv('st', hook('useFormStatus'))], el('i', null, json(get(v('st'), 'pending')), json(get(v('st'), 'data')))) } });
add_('hooks: effects are not run on the server', el(C('Eff')), { comps: {
  Eff: Show([doo(hook('useEffect', fn([], gset('ran', 'effect')), [])), doo(hook('useLayoutEffect', fn([], gset('ran', 'layout')))), doo(hook('useInsertionEffect', fn([], gset('ran', 'insertion')))),
    doo(hook('useImperativeHandle', o({}), fn([], gset('ran', 'imperative')))), doo(hook('useDebugValue', 'dbg'))], str(gget('ran'))) } });
add_('hooks: useEffectEvent callback throws during render', el(C('EE')), { comps: { EE: Show([letv('f', hook('useEffectEvent', fn([], 1))), doo(call(v('f')))], 'x') } });
add_('hooks: useEffectEvent returns a function', el(C('EE')), { comps: { EE: Show([letv('f', hook('useEffectEvent', fn([], 1)))], { $: 'typeof', v: v('f') }) } });
add_('hooks: compiler memo cache', el(C('MC')), { comps: { MC: Show([letv('c', hook('useMemoCache', 3))], cat(get(v('c'), 'length'), ' ', str(get(v('c'), 0)))) } });
add_('hooks: cache refresh throws when called', el(C('CR')), { comps: { CR: Show([letv('r', hook('useCacheRefresh')), doo(call(v('r')))], 'x') } });
add_('hooks: setState from a previous component is ignored', el('div', null, el(C('Owner')), el(C('Caller'))), { comps: {
  Owner: Show([letv(['s', 'set'], hook('useState', 'o')), doo(gset('setter', v('set')))], v('s')),
  Caller: Show([doo(call(gget('setter'), 'changed'))], 'called') } });

// ---- useId -----------------------------------------------------------------------------
const Id = Show([letv('id', hook('useId'))], el('span', { id: v('id') }, v('id')));
const Id2 = Show([letv('a', hook('useId')), letv('b', hook('useId')), letv('c', hook('useId'))], el('span', null, v('a'), ' ', v('b'), ' ', v('c')));
add_('useId: single', el(C('Id')), { comps: { Id } });
add_('useId: siblings', el('div', null, el(C('Id')), el(C('Id')), el(C('Id'))), { comps: { Id } });
add_('useId: several in one component', el('div', null, el(C('Id2'))), { comps: { Id2 } });
add_('useId: nested components', el(C('Outer')), { comps: { Id, Outer: Show([letv('id', hook('useId'))], el('div', { id: v('id') }, el(C('Id')), el(C('Id')))) } });
add_('useId: identifierPrefix', el('div', null, el(C('Id')), el(C('Id'))), { comps: { Id }, opts: { identifierPrefix: 'app-' }, stream: { opts: { identifierPrefix: 'app-' } } });
add_('useId: in arrays and fragments', el('div', null, [el(C('Id'), { key: 'a' }), el(T.frag, { key: 'b' }, el(C('Id')), el(C('Id')))], el(C('Id'))), { comps: { Id } });
add_('useId: inside suspense and context', el(Sus('f', el(ctx('Theme'), { value: 'x' }, el(C('Id'), null), el(C('Id'))))), { comps: { Id }, ctxs: { Theme } });
const deep = (n) => { let t = el(C('Id')); for (let i = 0; i < n; i++) t = el('div', null, 'x', t, 'y'); return t; };
add_('useId: deep tree overflow', deep(20), { comps: { Id } });
add_('useId: wide tree', el('div', null, ...Array.from({ length: 40 }, (_, i) => el(C('Id'), { key: String(i) }))), { comps: { Id } });
add_('useId: deep and wide overflow mix', (() => { let t = el(C('Id2')); for (let i = 0; i < 9; i++) t = el('section', null, ...Array.from({ length: 5 }, (_, j) => (j === 3 ? t : el('i', { key: String(j) })))); return t; })(), { comps: { Id2 } });
add_('useId: component without useId does not advance tree id', el('div', null, el(C('Plain')), el(C('Id'))), { comps: { Id, Plain: Show([], el(C('Id'))) } });

// ---- use() and thenables -------------------------------------------------------------
add_('use: tagged fulfilled promise reads synchronously', el(Sus('F', el(C('Use_p')))), { comps: { Use_p: UseP('p') }, promises: [{ n: 'p', kind: 'fulfilled', value: 'tagged' }] });
add_('use: untracked resolved promise suspends once', el('div', null, Sus('F', el(C('Use_p')))), { comps: { Use_p: UseP('p') }, promises: [{ n: 'p', kind: 'resolved', value: 'resolved' }] });
add_('use: resolved promise is cached for the next render', el(Sus('F', el(C('Use_p')))), { twice: true, modes: ['string', 'static'], comps: { Use_p: UseP('p') }, promises: [{ n: 'p', kind: 'resolved', value: 'cached' }] });
add_('use: pending promise resolved before second render', el(Sus('F', el(C('Use_p')))), { twice: true, modes: ['string', 'static'], comps: { Use_p: UseP('p') }, promises: [{ n: 'p', kind: 'pending', value: 'later', step: 1 }] });
add_('use: synchronous thenable', el(Sus('F', el(C('Use_p')))), { comps: { Use_p: UseP('p') }, promises: [{ n: 'p', kind: 'sync', value: 'sync-value' }] });
add_('use: synchronous rejecting thenable', el(Sus('F', el(C('Use_p')))), { comps: { Use_p: UseP('p') }, promises: [{ n: 'p', kind: 'syncReject', value: err('sync boom') }] });
add_('use: tagged rejected promise throws', el('div', null, el(C('Use_p'))), { comps: { Use_p: UseP('p') }, promises: [{ n: 'p', kind: 'rejectedTagged', value: err('tagged boom') }] });
add_('use: tagged rejected inside suspense', el(Sus('F', el(C('Use_p')))), { comps: { Use_p: UseP('p') }, promises: [{ n: 'p', kind: 'rejectedTagged', value: err('in boundary') }] });
add_('use: suspending outside a boundary', el('div', null, el(C('Use_p'))), { comps: { Use_p: UseP('p') }, promises: [{ n: 'p', kind: 'pending', value: 'v', step: 1 }] });
add_('use: unsupported argument', el(C('Bad')), { comps: { Bad: Show([letv('x', hook('use', 5))], 'x') } });
add_('use: unsupported object argument', el(C('Bad')), { comps: { Bad: Show([letv('x', hook('use', o({ a: 1 })))], 'x') } });
add_('use: thenable child', el('div', null, promise('p')), { promises: [{ n: 'p', kind: 'fulfilled', value: el('b', null, 'from promise') }] });
add_('use: pending thenable child in suspense', el(Sus('wait', promise('p'))), { promises: [{ n: 'p', kind: 'pending', value: 'resolved child', step: 1 }] });
add_('use: two uses in one component', el(Sus('F', el(C('Two')))), { comps: { Two: Show([letv('a', hook('use', promise('a'))), letv('b', hook('use', promise('b')))], cat(v('a'), '+', v('b'))) },
  promises: [{ n: 'a', kind: 'pending', value: 'A', step: 1 }, { n: 'b', kind: 'pending', value: 'B', step: 2 }] });
add_('use: try/catch around use rethrows', el(Sus('F', el(C('Catch')))), { comps: { Catch: Show([{ try: [letv('x', hook('use', promise('p')))], catch: 'e', handle: [{ throw: v('e') }] }], 'unreached') }, promises: [{ n: 'p', kind: 'pending', value: 'v', step: 1 }] });

// ---- Suspense ------------------------------------------------------------------------
add_('suspense: no suspension', el('div', null, Sus('F', el('p', null, 'content'))));
add_('suspense: text inside boundary', el('div', null, 'before', Sus('F', 'inside'), 'after'));
add_('suspense: nested boundaries', el(Sus('outer', el('div', null, Sus('inner', el(C('Use_p')))))), { comps: { Use_p: UseP('p') }, promises: [{ n: 'p', kind: 'pending', value: 'deep', step: 1 }] });
add_('suspense: fallback element', el('div', null, Sus(el('span', { className: 'spinner' }, 'Loading…'), el(C('Use_p')))), { comps: { Use_p: UseP('p') }, promises: [{ n: 'p', kind: 'pending', value: 'loaded', step: 1 }] });
add_('suspense: two boundaries resolve in order', el('div', null, Sus('f1', el(C('A'))), Sus('f2', el(C('B')))), { comps: { A: UseP('a'), B: UseP('b') },
  promises: [{ n: 'a', kind: 'pending', value: 'first', step: 2 }, { n: 'b', kind: 'pending', value: 'second', step: 1 }] });
add_('suspense: error inside boundary', el('div', null, Sus('fallback', el(C('Boom')))), { comps: { Boom: Show([{ throw: err('kaboom') }], 'x') } });
add_('suspense: error with digest', el('div', null, Sus('fallback', el(C('Boom')))), { comps: { Boom: Show([{ throw: err('digest me') }], 'x') }, stream: { digest: 'msg' } });
add_('suspense: error in fallback', el(Sus(el(C('Boom')), el(C('Use_p')))), { comps: { Use_p: UseP('p'), Boom: Show([{ throw: err('fallback broke') }], 'x') }, promises: [{ n: 'p', kind: 'pending', value: 'v', step: 1 }] });
add_('suspense: rejection after shell', el('div', null, Sus('F', el(C('Use_p')))), { comps: { Use_p: UseP('p') }, promises: [{ n: 'p', kind: 'pending', value: err('late failure'), reject: true, step: 1 }] });
add_('suspense: rejection after shell with digest', el('div', null, Sus('F', el(C('Use_p')))), { comps: { Use_p: UseP('p') }, promises: [{ n: 'p', kind: 'pending', value: err('late'), reject: true, step: 1 }], stream: { digest: 'msg' } });
add_('suspense: undefined fallback', el('div', null, el(T.suspense, null, el(C('Use_p')))), { comps: { Use_p: UseP('p') }, promises: [{ n: 'p', kind: 'pending', value: 'x', step: 1 }] });
add_('suspense: text separators around boundary content', el('div', null, 'a', Sus('F', 'b', el(C('Use_p')), 'c'), 'd'), { comps: { Use_p: UseP('p') }, promises: [{ n: 'p', kind: 'pending', value: 'mid', step: 1 }] });
add_('suspense: suspended text at boundary edge', el('p', null, 'x', Sus('F', el(C('Use_p'))), 'y'), { comps: { Use_p: UseP('p') }, promises: [{ n: 'p', kind: 'pending', value: 'text', step: 1 }] });
add_('suspense: abort with reason', el('div', null, Sus('F', el(C('Use_p')))), { modes: ['stream'], comps: { Use_p: UseP('p') }, promises: [{ n: 'p', kind: 'pending', value: 'never' }], stream: { abort: { step: 1, reason: err('timed out') } } });
add_('suspense: abort without reason', el('div', null, Sus('F', el(C('Use_p')))), { modes: ['stream'], comps: { Use_p: UseP('p') }, promises: [{ n: 'p', kind: 'pending', value: 'never' }], stream: { abort: { step: 1 } } });
add_('suspense: abort with string reason and digest', el('div', null, Sus('F', el(C('Use_p')))), { modes: ['stream'], comps: { Use_p: UseP('p') }, promises: [{ n: 'p', kind: 'pending', value: 'never' }], stream: { digest: 'msg', abort: { step: 1, reason: 'stop' } } });
add_('suspense: abort the shell', el('div', null, el(C('Use_p'))), { modes: ['stream'], comps: { Use_p: UseP('p') }, promises: [{ n: 'p', kind: 'pending', value: 'never' }], stream: { abort: { step: 1, reason: err('shell aborted') } } });
add_('suspense: pipe in onShellReady', el('div', null, Sus('F', el(C('Use_p')))), { modes: ['stream'], comps: { Use_p: UseP('p') }, promises: [{ n: 'p', kind: 'pending', value: 'shell-piped', step: 1 }], stream: { pipe: 'shell' } });
add_('suspense: pipe in onAllReady inlines content', el('div', null, Sus('F', el(C('Use_p')))), { modes: ['stream'], comps: { Use_p: UseP('p') }, promises: [{ n: 'p', kind: 'pending', value: 'all-piped', step: 1 }], stream: { pipe: 'all' } });
add_('suspense: resolved promise piped in onAllReady', el('div', null, Sus('F', el(C('Use_p')))), { modes: ['stream'], comps: { Use_p: UseP('p') }, promises: [{ n: 'p', kind: 'resolved', value: 'quick' }], stream: { pipe: 'all' } });
add_('suspense: nested boundary completes before outer', el(Sus('outer', el(C('A')), Sus('inner', el(C('B'))))), { comps: { A: UseP('a'), B: UseP('b') },
  promises: [{ n: 'a', kind: 'pending', value: 'A', step: 2 }, { n: 'b', kind: 'pending', value: 'B', step: 1 }] });
add_('suspense: large boundary is outlined', el('div', null, Sus('F', el('p', null, 'x'.repeat(600)))), { modes: ['stream'], stream: { opts: { progressiveChunkSize: 100 } } });
add_('suspense: large boundary inlined when piped late', el('div', null, Sus('F', el('p', null, 'y'.repeat(600)))), { modes: ['stream'], stream: { pipe: 'all', opts: { progressiveChunkSize: 100 } } });
add_('suspense: segment inside resolved boundary (partial)', el(Sus('outer', el('div', null, el(C('A')), el(C('B'))))), { comps: { A: UseP('a'), B: UseP('b') },
  promises: [{ n: 'a', kind: 'pending', value: 'A', step: 1 }, { n: 'b', kind: 'pending', value: 'B', step: 2 }] });
add_('suspense: table insertion mode segments', el('table', null, el('tbody', null, Sus(el('tr', null, el('td', null, 'loading')), el(C('Row'))))), { comps: { Row: Show([letv('x', hook('use', promise('p')))], el('tr', null, el('td', null, v('x')))) },
  promises: [{ n: 'p', kind: 'pending', value: 'row', step: 1 }] });
add_('suspense: svg insertion mode segments', el('svg', null, Sus(el('circle'), el(C('Shape')))), { comps: { Shape: Show([letv('x', hook('use', promise('p')))], el('rect', { width: v('x') })) },
  promises: [{ n: 'p', kind: 'pending', value: 5, step: 1 }] });

// ---- lazy --------------------------------------------------------------------------------
const Hello = Show([], el('b', null, 'lazy:', prop('who')), { name: 'Hello' });
add_('lazy: synchronous thenable module', el(C('L'), { who: 'sync' }), { comps: { Hello, L: { kind: 'lazy', p: 'mod' } }, promises: [{ n: 'mod', kind: 'sync', value: o({ default: compv('Hello') }) }] });
add_('lazy: pending module in suspense', el(Sus('F', el(C('L'), { who: 'async' }))), { comps: { Hello, L: { kind: 'lazy', p: 'mod' } }, promises: [{ n: 'mod', kind: 'pending', value: o({ default: compv('Hello') }), step: 1 }] });
add_('lazy: resolved promise module suspends once', el(Sus('F', el(C('L'), { who: 'resolved' }))), { comps: { Hello, L: { kind: 'lazy', p: 'mod' } }, promises: [{ n: 'mod', kind: 'resolved', value: o({ default: compv('Hello') }) }] });
add_('lazy: module without default', el(C('L')), { comps: { L: { kind: 'lazy', p: 'mod' } }, promises: [{ n: 'mod', kind: 'sync', value: o({ other: 1 }) }] });
add_('lazy: loader throws', el(C('L')), { comps: { L: { kind: 'lazy', throws: err('loader failed') } } });
add_('lazy: rejected module', el(Sus('F', el(C('L')))), { comps: { L: { kind: 'lazy', p: 'mod' } }, promises: [{ n: 'mod', kind: 'syncReject', value: err('module failed') }] });
add_('lazy: as a child node', el('div', null, compv('L')), { comps: { L: { kind: 'lazy', p: 'mod' } }, promises: [{ n: 'mod', kind: 'sync', value: o({ default: 'lazy text' }) }] });

// ---- forwardRef and memo ------------------------------------------------------------------
add_('forwardRef: renders and receives ref', el(C('Fwd'), { ref: o({ current: null }), label: 'L' }), { comps: { Fwd: { kind: 'forwardRef', name: 'Inner', body: [], ret: el('i', null, prop('label'), ' ref:', { $: 'typeof', v: v('ref') }, ' hasRefProp:', json({ $: 'eq', a: prop('ref'), b: U })) } } });
add_('forwardRef: hooks inside', el(C('Fwd')), { comps: { Fwd: { kind: 'forwardRef', body: [letv(['s'], hook('useState', 'fs')), letv('id', hook('useId'))], ret: cat(v('s'), v('id')) } } });
add_('memo: function component', el(C('M'), { x: 1 }), { comps: { Hello, M: { kind: 'memo', of: C('Inner') }, Inner: Show([], cat('memo:', prop('x'))) } });
add_('memo: of forwardRef', el(C('M'), { ref: 'r' }), { comps: { M: { kind: 'memo', of: C('Fwd') }, Fwd: { kind: 'forwardRef', body: [], ret: str(v('ref')) } } });
add_('memo: with compare (ignored on the server)', el(C('M')), { comps: { M: { kind: 'memo', of: C('Inner'), compare: fn(['a', 'b'], true) }, Inner: Show([], 'rendered') } });
add_('memo: of host element', el(C('M'), { id: 'm' }, 'child'), { comps: { M: { kind: 'memo', of: 'section' } } });

// ---- class components ------------------------------------------------------------------
const cls = (extra) => Object.assign({ kind: 'class', name: 'K', body: [], ret: el('i', null, json(v('state'))) }, extra);
add_('class: render with props and state', el(C('K'), { a: 1 }), { comps: { K: cls({ state: o({ s: 'x' }), ret: el('i', null, json(v('props')), json(v('state'))) }) } });
add_('class: no state is null', el(C('K')), { comps: { K: cls({}) } });
add_('class: defaultProps', el('div', null, el(C('K')), el(C('K'), { a: 'given' })), { comps: { K: cls({ defaultProps: o({ a: 'def', b: 2 }), ret: json(v('props')) }) } });
add_('class: contextType', el(ctx('Theme'), { value: 'ctx-val' }, el(C('K'))), { ctxs: { Theme }, comps: { K: cls({ contextType: 'Theme', ret: str(v('context')) }) } });
add_('class: no contextType gives empty object', el(C('K')), { comps: { K: cls({ ret: json(v('context')) }) } });
add_('class: getDerivedStateFromProps', el(C('K'), { p: 3 }), { comps: { K: cls({ state: o({ a: 1, b: 2 }), gdsfp: fn(['props', 'state'], o({ b: get(v('props'), 'p'), c: 'new' })) }) } });
add_('class: getDerivedStateFromProps returning null', el(C('K')), { comps: { K: cls({ state: o({ a: 1 }), gdsfp: fn(['p', 's'], null) }) } });
add_('class: componentWillMount setState merges', el(C('K')), { comps: { K: cls({ state: o({ a: 1, b: 1 }), cwm: [doo({ $: 'setState', v: o({ b: 2 }) }), doo({ $: 'setState', v: fn(['prev', 'props'], o({ c: add(get(v('prev'), 'b'), 10) })) })] }) } });
add_('class: UNSAFE_componentWillMount', el(C('K')), { comps: { K: cls({ state: o({ a: 1 }), ucwm: [doo({ $: 'setState', v: o({ u: 'unsafe' }) })] }) } });
add_('class: both will-mount methods', el(C('K')), { comps: { K: cls({ state: o({ n: 0 }), cwm: [doo({ $: 'setState', v: o({ cwm: true }) })], ucwm: [doo({ $: 'setState', v: o({ ucwm: true }) })] }) } });
add_('class: getSnapshotBeforeUpdate disables componentWillMount', el(C('K')), { comps: { K: cls({ state: o({ a: 1 }), snapshot: true, cwm: [doo({ $: 'setState', v: o({ ignored: true }) })] }) } });
add_('class: gdsfp disables componentWillMount', el(C('K')), { comps: { K: cls({ state: o({ a: 1 }), gdsfp: fn(['p', 's'], null), cwm: [doo({ $: 'setState', v: o({ ignored: true }) })] }) } });
add_('class: setState with null and invalid argument', el(C('K')), { comps: { K: cls({ state: o({ a: 1 }), cwm: [doo({ $: 'setState', v: null }), doo({ $: 'setState', v: 'bad' })] }) } });
add_('class: PureComponent', el(C('K')), { comps: { K: cls({ pure: true, state: o({ p: 'pure' }) }) } });
add_('class: render throws', el(C('K')), { comps: { K: cls({ body: [{ throw: err('class render failed') }] }) } });
add_('class: ref prop stripped from props', el(C('K'), { ref: 'r', x: 1 }), { comps: { K: cls({ ret: json(v('props')) }) } });

// ---- select / option / textarea -----------------------------------------------------------
add_('form: select value marks option', el('select', { value: 'b', onChange: fn([], U) }, el('option', { value: 'a' }, 'A'), el('option', { value: 'b' }, 'B')));
add_('form: select defaultValue', el('select', { defaultValue: 'a' }, el('option', { value: 'a' }, 'A'), el('option', { value: 'b' }, 'B')));
add_('form: select multiple array value', el('select', { multiple: true, value: ['a', 'c'] }, el('option', { value: 'a' }, 'A'), el('option', { value: 'b' }, 'B'), el('option', { value: 'c' }, 'C')));
add_('form: option value from children', el('select', { value: 'Two' }, el('option', null, 'One'), el('option', null, 'Two'), el('option', null, 'T', 'wo')));
add_('form: option numeric value matches string', el('select', { value: 2 }, el('option', { value: 1 }, 'one'), el('option', { value: '2' }, 'two')));
add_('form: option selected without select value', el('div', null, el('option', { selected: true }, 'x'), el('option', { selected: false }, 'y')));
add_('form: optgroup inside select', el('select', { value: 'y' }, el('optgroup', { label: 'G' }, el('option', { value: 'x' }, 'X'), el('option', { value: 'y' }, 'Y'))));
add_('form: option with element children', el('select', { value: '[object Object]' }, el('option', null, el('b', null, 'bold'))));
add_('form: option dangerouslySetInnerHTML', el('option', { dangerouslySetInnerHTML: o({ __html: '<i>x</i>' }) }));
add_('form: textarea value', el('textarea', { value: 'hello <&>', onChange: fn([], U) }));
add_('form: textarea defaultValue', el('textarea', { defaultValue: 'def' }));
add_('form: textarea children', el('textarea', null, 'child text'));
add_('form: textarea array child', el('textarea', null, ['only']));
add_('form: textarea leading newline', el('div', null, el('textarea', { value: '\nfirst line' }), el('textarea', { defaultValue: 'no newline' })));
add_('form: textarea value and children is an error', el('textarea', { value: 'v' }, 'c'));
add_('form: textarea two children is an error', el('textarea', null, 'a', 'b'));
add_('form: textarea dangerouslySetInnerHTML is an error', el('textarea', { dangerouslySetInnerHTML: o({ __html: 'x' }) }));
add_('form: input value checked and defaults', el('div', null, el('input', { value: 'v', defaultValue: 'd', checked: true, defaultChecked: false, type: 'checkbox' }), el('input', { defaultValue: 'only-default', defaultChecked: true })));
add_('form: input value types', el('div', null, el('input', { value: 0 }), el('input', { value: false }), el('input', { value: '' }), el('input', { value: null })));
add_('form: pre and listing leading newline', el('div', null, el('pre', null, '\ncode'), el('listing', null, '\nlisting'), el('pre', { dangerouslySetInnerHTML: o({ __html: '\nraw' }) }), el('pre', null, 'plain')));
add_('form: form action function', el('form', { action: fn(['fd'], U) }, el('button', { formAction: fn([], U) }, 'b')));
add_('form: form action string', el('form', { action: '/submit', method: 'post', encType: 'multipart/form-data', target: '_self' }, 'f'));
add_('form: button and input formAction strings', el('div', null, el('button', { formAction: '/b', formMethod: 'post', formEncType: 'x', formTarget: 't', name: 'n' }, 'b'), el('input', { type: 'submit', formAction: '/i', name: 'n2' })));
add_('form: input formAction function', el('form', null, el('input', { type: 'submit', formAction: fn([], U), name: 'ignored' })));
add_('form: server action with $$FORM_ACTION', el('form', { action: { $: 'action', fn: fn([], U), formAction: fn(['prefix'], o({ name: cat('$ACTION_', v('prefix')), action: '/act', method: 'POST', data: [['a', '1'], ['b', 'two']] })) } }, 'f'));
add_('form: button with $$FORM_ACTION', el('form', null, el('button', { formAction: { $: 'action', fn: fn([], U), formAction: fn(['prefix'], o({ name: 'btn', action: '', encType: 'multipart/form-data', data: [['x', 'y']] })) } }, 'go')));
add_('form: $$FORM_ACTION field values become strings', el('form', { action: { $: 'action', fn: fn([], U), formAction: fn(['p'], o({ name: 'n', data: [['f', 5]] })) } }, 'x'));
add_('form: $$FORM_ACTION with a Blob field falls back', el('form', { action: { $: 'action', fn: fn([], U), formAction: fn(['p'], o({ name: 'n', data: [['f', { $: 'blob' }]] })) } }, 'x'));
add_('form: $$FORM_ACTION without name', el('form', { action: { $: 'action', fn: fn([], U), formAction: fn(['p'], o({ action: '/no-name' })) } }, 'x'));

// ---- SVG and MathML -------------------------------------------------------------------
add_('svg: attributes and casing', el('svg', { viewBox: '0 0 10 10', xmlns: 'http://www.w3.org/2000/svg' }, el('path', { d: 'M0 0', strokeWidth: 2, strokeLinecap: 'round', fillRule: 'evenodd', className: 'p' }), el('circle', { cx: 5, cy: 5, r: 4, fillOpacity: 0.5 }), el('text', { textAnchor: 'middle', dominantBaseline: 'central' }, 'T')));
add_('svg: foreignObject switches back to HTML', el('svg', null, el('foreignObject', null, el('div', null, el('title', null, 'not hoisted?')))));
add_('svg: title inside svg is not hoisted', el('div', null, el('svg', null, el('title', null, 'SVG title'), el('g'))));
add_('svg: script and style inside svg', el('svg', null, el('style', null, 'circle{}'), el('script', { src: 'x.js', async: true })));
add_('svg: link and meta inside svg', el('svg', null, el('link', { rel: 'stylesheet', href: 's.css', precedence: 'p' }), el('meta', { name: 'm' })));
add_('math: elements', el('math', null, el('mi', null, 'x'), el('mo', null, '='), el('mn', null, 2), el('annotation-xml', { encoding: 'text/html' }, el('b', null, 'b'))));
add_('svg: root namespaceURI option', el('circle', { r: 2 }), { modes: ['stream'], stream: { opts: { namespaceURI: 'http://www.w3.org/2000/svg' } } });

// ---- script and style ---------------------------------------------------------------
add_('script: inline content escaping', el('div', null, el('script', null, 'var s = "</script><script>alert(1)</SCRIPT>"; if (a<b) {}')));
add_('script: non-string children not rendered', el('div', null, el('script', null, 5), el('script', null, ['a', 'b'])));
add_('script: dangerouslySetInnerHTML', el('script', { dangerouslySetInnerHTML: o({ __html: 'var x = "</script>";' }) }));
add_('script: async src is hoisted as resource', el('div', null, 'text', el('script', { async: true, src: 'a.js' }), el('script', { async: true, src: 'a.js' })));
add_('script: module async src', el('div', null, el('script', { async: true, src: 'm.js', type: 'module' })));
add_('script: non-async src stays in place', el('div', null, el('script', { src: 'b.js' }), el('script', { src: 'c.js', async: true, onLoad: fn([], U) })));
add_('style: inline content escaping', el('div', null, el('style', null, 'a::after{content:"</style><b>"} </STYLE')));
add_('style: array child', el('div', null, el('style', null, ['only']), el('style', null, ['a', 'b'])));
add_('style: precedence and href hoisted', el('div', null, 'x', el('style', { href: 'h1', precedence: 'low' }, '.a{}'), el('style', { href: 'h2', precedence: 'low' }, '.b{}'), el('style', { href: 'h1', precedence: 'low' }, 'dup')));
add_('style: dangerouslySetInnerHTML in hoisted style', el('div', null, el('style', { href: 'h', precedence: 'p', dangerouslySetInnerHTML: o({ __html: '.raw{}' }) })));

// ---- hoisting and resources ---------------------------------------------------------
add_('hoist: title meta and link in body', el('div', null, 'a', el('title', null, 'Page'), el('meta', { name: 'description', content: 'd' }), el('link', { rel: 'canonical', href: '/c' }), 'b'));
add_('hoist: meta charset and viewport first', el('div', null, el('meta', { name: 'x', content: '1' }), el('meta', { name: 'viewport', content: 'w' }), el('meta', { charSet: 'utf-8' })));
add_('hoist: title with array children', el('div', null, el('title', null, ['single']), el('title', null, 'a', 'b'), el('title', null, 5)));
add_('hoist: title in noscript and with itemProp stays', el('div', null, el('noscript', null, el('title', null, 'ns')), el('title', { itemProp: 'name' }, 'ip')));
add_('hoist: stylesheet with precedence', el('div', null, el('link', { rel: 'stylesheet', href: 'a.css', precedence: 'high' }), el('link', { rel: 'stylesheet', href: 'b.css', precedence: 'low' }), el('link', { rel: 'stylesheet', href: 'c.css', precedence: 'high' })));
add_('hoist: stylesheet without precedence stays', el('div', null, el('link', { rel: 'stylesheet', href: 'plain.css' })));
add_('hoist: stylesheet with onLoad or disabled stays', el('div', null, el('link', { rel: 'stylesheet', href: 'x.css', precedence: 'p', disabled: true }), el('link', { rel: 'stylesheet', href: 'y.css', precedence: 'p', onError: fn([], U) })));
add_('hoist: link with children is an error', el('link', { rel: 'icon', href: 'i.png' }, 'x'));
add_('hoist: images get preload links', el('div', null, el('img', { src: 'hero.jpg' }), el('img', { src: 'b.jpg', fetchPriority: 'high' }), el('img', { srcSet: 'a.png 1x, b.png 2x', sizes: '100vw' }), el('img', { src: 'lazy.jpg', loading: 'lazy' }), el('img', { src: 'data:image/png;base64,xx' }), el('img', { src: 'low.jpg', fetchPriority: 'low' })));
add_('hoist: many images promote only ten', el('div', null, ...Array.from({ length: 13 }, (_, i) => el('img', { src: `i${i}.png`, key: String(i) }))));
add_('hoist: image in picture is not preloaded', el('picture', null, el('source', { srcSet: 's.webp' }), el('img', { src: 'p.jpg' })));
add_('hoist: image crossOrigin and integrity', el('div', null, el('img', { src: 'c.jpg', crossOrigin: 'use-credentials', integrity: 'sha' }), el('img', { src: 'd.jpg', crossOrigin: '' })));
add_('resources: preload preconnect prefetchDNS in a component', el('div', null, el(C('R'))), { comps: { R: Show([
  doo(api('preload', 'font.woff2', o({ as: 'font', type: 'font/woff2' }))), doo(api('preload', 'img.png', o({ as: 'image', fetchPriority: 'high' }))),
  doo(api('preload', 'style.css', o({ as: 'style', crossOrigin: 'use-credentials', integrity: 'sha-1' }))), doo(api('preload', 'script.js', o({ as: 'script' }))),
  doo(api('preload', 'data.json', o({ as: 'fetch' }))), doo(api('preconnect', 'https://cdn.example', o({ crossOrigin: 'anonymous' }))), doo(api('preconnect', 'https://plain.example')),
  doo(api('prefetchDNS', 'https://dns.example')), doo(api('preload', 'font.woff2', o({ as: 'font' })))], 'r') } });
add_('resources: preinit style and script', el('div', null, el(C('R'))), { comps: { R: Show([
  doo(api('preinit', 'p.css', o({ as: 'style', precedence: 'high' }))), doo(api('preinit', 'q.css', o({ as: 'style' }))),
  doo(api('preinit', 'p.js', o({ as: 'script', nonce: 'n1', integrity: 'i' }))), doo(api('preinit', 'bad', o({ as: 'other' })))], 'r') } });
add_('resources: preloadModule and preinitModule', el('div', null, el(C('R'))), { comps: { R: Show([
  doo(api('preloadModule', 'm.js')), doo(api('preloadModule', 'w.js', o({ as: 'worker' }))), doo(api('preinitModule', 'pm.js')), doo(api('preinitModule', 'pm2.js', o({ crossOrigin: 'use-credentials' })))], 'r') } });
add_('resources: preload then matching stylesheet adopts credentials', el('div', null, el(C('R')), el('link', { rel: 'stylesheet', href: 'adopt.css', precedence: 'p' })), { comps: { R: Show([doo(api('preload', 'adopt.css', o({ as: 'style', crossOrigin: 'anonymous', integrity: 'sha-2' })))], 'r') } });
add_('resources: preload then async script adopts credentials', el('div', null, el(C('R')), el('script', { async: true, src: 'adopt.js' })), { comps: { R: Show([doo(api('preload', 'adopt.js', o({ as: 'script', integrity: 'sha-3' })))], 'r') } });
add_('resources: invalid calls are ignored', el('div', null, el(C('R'))), { comps: { R: Show([doo(api('preload', 5, o({ as: 'font' }))), doo(api('preload', 'x')), doo(api('preinit', 'y')), doo(api('prefetchDNS', '')), doo(api('preconnect', 7))], 'r') } });
add_('resources: early Link header', el('div', null, el(C('R')), el('img', { src: 'hdr.png' })), { modes: ['stream'], comps: { R: Show([doo(api('preconnect', 'https://h.example')), doo(api('preload', 'f.woff', o({ as: 'font' }))), doo(api('prefetchDNS', 'https://d.example'))], 'r') }, stream: { opts: { onHeaders: true } } });
add_('resources: header capacity exhausted', el('div', null, el(C('R'))), { modes: ['stream'], comps: { R: Show([doo(api('preconnect', 'https://aaaaaaaaaaaaaaaaaaaa.example')), doo(api('preconnect', 'https://bbbbbbbbbbbbbbbbbbbb.example'))], 'r') }, stream: { opts: { onHeaders: true, maxHeadersLength: 50 } } });

// ---- documents --------------------------------------------------------------------
add_('doc: html head body', el('html', { lang: 'en' }, el('head', null, el('meta', { charSet: 'utf-8' }), el('title', null, 'Doc')), el('body', { className: 'b' }, el('h1', null, 'Hi'))));
add_('doc: html without head', el('html', null, el('body', null, 'x')));
add_('doc: head and body twice is an error', el('html', null, el('head'), el('head')));
add_('doc: body without html', el('body', null, 'only body'));
add_('doc: head without html', el('div', null, el('head', null, el('title', null, 't'))));
add_('doc: html with suspense in body', el('html', null, el('head'), el('body', null, Sus('F', el(C('Use_p'))))), { comps: { Use_p: UseP('p') }, promises: [{ n: 'p', kind: 'pending', value: 'body content', step: 1 }] });
add_('doc: suspense around html', el(Sus('F', el('html', null, el('body', null, 'x')))));
add_('doc: stylesheet with document', el('html', null, el('head'), el('body', null, el('link', { rel: 'stylesheet', href: 'main.css', precedence: 'default' }), 'x')));
add_('doc: hoistables inside suspense boundary', el('div', null, Sus('F', el('title', null, 'In boundary'), el('meta', { name: 'b' }), el(C('Use_p')))), { comps: { Use_p: UseP('p') }, promises: [{ n: 'p', kind: 'pending', value: 'later', step: 1 }] });
add_('doc: stylesheet inside late boundary', el('div', null, Sus('F', el(C('Use_p')), el('link', { rel: 'stylesheet', href: 'late.css', precedence: 'default' }))), { comps: { Use_p: UseP('p') }, promises: [{ n: 'p', kind: 'pending', value: 'styled', step: 1 }] });
add_('doc: stylesheet only inside late boundary', el('div', null, Sus('F', el(C('Gate')))), { comps: { Gate: Show([letv('x', hook('use', promise('p')))], el(T.frag, null, el('link', { rel: 'stylesheet', href: 'only-late.css', precedence: 'default', 'data-x': 'y' }), v('x'))) },
  promises: [{ n: 'p', kind: 'pending', value: 'styled', step: 1 }] });
add_('doc: style tag inside late boundary', el('div', null, Sus('F', el(C('Gate')))), { comps: { Gate: Show([letv('x', hook('use', promise('p')))], el(T.frag, null, el('style', { href: 'late-style', precedence: 'default' }, '.late{}'), v('x'))) },
  promises: [{ n: 'p', kind: 'pending', value: 'styled', step: 1 }] });

// ---- stream options ------------------------------------------------------------------
add_('stream: bootstrap scripts and modules', el('div', null, 'app'), { modes: ['stream'], stream: { opts: { bootstrapScriptContent: 'window.x = "</script>";', bootstrapScripts: ['/a.js', o({ src: '/b.js', integrity: 'sha', crossOrigin: 'use-credentials' })], bootstrapModules: ['/m.mjs', o({ src: '/n.mjs', crossOrigin: 'anonymous' })] } } });
add_('stream: nonce string', el('div', null, Sus('F', el(C('Use_p')))), { modes: ['stream'], comps: { Use_p: UseP('p') }, promises: [{ n: 'p', kind: 'pending', value: 'n', step: 1 }], stream: { opts: { nonce: 'abc', bootstrapScriptContent: 'go()', bootstrapScripts: ['/x.js'] } } });
add_('stream: nonce object', el('div', null, el('style', { href: 's', precedence: 'p', nonce: 'st' }, '.a{}'), el('style', { href: 't', precedence: 'p', nonce: 'other' }, '.b{}')), { modes: ['stream'], stream: { opts: { nonce: o({ script: 'sc', style: 'st' }), bootstrapScriptContent: 'go()' } } });
add_('stream: importMap', el('div', null, 'x'), { modes: ['stream'], stream: { opts: { importMap: o({ imports: o({ react: '/react.js', '<x>': '&' }) }) } } });
add_('stream: identifierPrefix in ids and boundaries', el('div', null, el(C('Id')), Sus('F', el(C('Use_p')))), { modes: ['stream'], comps: { Id, Use_p: UseP('p') }, promises: [{ n: 'p', kind: 'pending', value: 'v', step: 1 }], stream: { opts: { identifierPrefix: 'pre_' } } });
add_('stream: shell error without pipe', el(C('Boom')), { modes: ['stream'], comps: { Boom: Show([{ throw: err('shell failed') }], 'x') }, stream: { pipe: 'all' } });
add_('stream: component throws a string', el(C('Boom')), { comps: { Boom: Show([{ throw: 'a string' }], 'x') } });
add_('stream: component throws null', el('div', null, Sus('F', el(C('Boom')))), { comps: { Boom: Show([{ throw: null }], 'x') } });
add_('stream: formState markers', el(C('Act')), { modes: ['stream'], comps: { Act: Show([letv(['s', 'a'], hook('useActionState', fn(['p'], 1), 'st')), letv(['s2', 'a2'], hook('useActionState', fn(['p'], 2), 'st2'))], cat(v('s'), v('s2'))) }, stream: { opts: { formState: ['state', 'key', 'ref', 0] } } });
add_('stream: formState matching action', el(C('Act')), { modes: ['stream'], comps: { Act: Show([letv(['s', 'a'], hook('useActionState', { $: 'action', fn: fn(['p'], 1), formAction: fn(['p'], o({ name: 'n', data: [] })), sigEqual: fn(['x', 'y'], true) }, 'initial', '/perma'))], el('form', { action: v('a') }, v('s'))) },
  stream: { opts: { formState: ['restored', 'p/perma', 'ref', 0] } } });
const keyedAction = { $: 'action', fn: fn(['p'], 1), bindable: true, formAction: fn(['p'], o({ name: cat('n_', v('p')), data: [['field', 'v']] })) };
const ActForm = Show([letv(['s', 'a'], hook('useActionState', keyedAction, 'init'))], el('form', { action: v('a') }, v('s')), { name: 'ActForm' });
add_('form: useActionState postback key (murmurhash / md5)', el('section', null, el('div', { key: 'k' }, el(C('ActForm'))), el(C('ActForm'))), { comps: { ActForm } });
add_('form: useActionState postback key with permalink', el(C('PF')), { comps: { PF: Show([letv(['s', 'a'], hook('useActionState', keyedAction, 'init', '/perma'))], el('form', { action: v('a') }, v('s')), { name: 'PF' }) } });
add_('form: useActionState with formState signature match', el(C('ActForm')), { modes: ['stream'], comps: { ActForm: Show([letv(['s', 'a'], hook('useActionState', Object.assign({}, keyedAction, { sigEqual: fn(['x', 'y'], true) }), 'init'))], el('form', { action: v('a') }, v('s')), { name: 'ActForm' }) },
  stream: { opts: { formState: ['restored', 'k-unmatched', 'ref', 0] } } });
add_('resources: Link header when piped in onShellReady', el('div', null, el(C('R')), el('img', { src: 'hdr2.png' })), { modes: ['stream'], comps: { R: Show([doo(api('preconnect', 'https://h2.example', o({ crossOrigin: 'use-credentials' }))), doo(api('preload', 'f2.woff', o({ as: 'font' }))), doo(api('preload', 'big.png', o({ as: 'image', fetchPriority: 'high' })))], 'r') }, stream: { pipe: 'shell', opts: { onHeaders: true } } });
add_('resources: Link header includes stylesheets when shell pending', el('div', null, el('link', { rel: 'stylesheet', href: 'early.css', precedence: 'p', crossOrigin: 'anonymous' }), el(C('Use_p'))), { modes: ['stream'], comps: { Use_p: UseP('p') }, promises: [{ n: 'p', kind: 'pending', value: 'late', step: 1 }], stream: { pipe: 'shell', opts: { onHeaders: true } } });
add_('form: textarea and select edge values', el('div', null, el('textarea', { value: 0 }), el('textarea', { value: null, defaultValue: 'dv' }), el('select', { value: null, defaultValue: 'b' }, el('option', { value: 'a' }), el('option', { value: 'b' }))));
add_('stream: progressive chunk boundaries flush', el('div', null, Sus('F1', el(C('A'))), Sus('F2', el(C('B')))), { modes: ['stream'], comps: { A: UseP('a'), B: UseP('b') },
  promises: [{ n: 'a', kind: 'pending', value: 'z'.repeat(700), step: 1 }, { n: 'b', kind: 'pending', value: 'small', step: 1 }], stream: { opts: { progressiveChunkSize: 300 } } });

// ---- Activity and SuspenseList ----------------------------------------------------------
add_('activity: visible', el('div', null, 'a', el(T.activity, { mode: 'visible' }, 'b'), 'c'));
add_('activity: hidden', el('div', null, 'a', el(T.activity, { mode: 'hidden' }, el('i', null, 'hidden')), 'c'));
add_('activity: default mode', el(T.activity, null, el('p', null, 'x')));
add_('suspenselist: forwards', el(T.list, { revealOrder: 'forwards' }, Sus('f1', el(C('A'))), Sus('f2', el(C('B')))), { comps: { A: UseP('a'), B: UseP('b') },
  promises: [{ n: 'a', kind: 'pending', value: 'A', step: 2 }, { n: 'b', kind: 'pending', value: 'B', step: 1 }] });
add_('suspenselist: backwards', el(T.list, { revealOrder: 'backwards' }, Sus('f1', el(C('A'))), Sus('f2', el(C('B'))), 'tail'), { comps: { A: UseP('a'), B: UseP('b') },
  promises: [{ n: 'a', kind: 'pending', value: 'A', step: 1 }, { n: 'b', kind: 'pending', value: 'B', step: 2 }] });
add_('suspenselist: together', el(T.list, { revealOrder: 'together' }, Sus('f1', el(C('A'))), Sus('f2', el(C('B')))), { comps: { A: UseP('a'), B: UseP('b') },
  promises: [{ n: 'a', kind: 'pending', value: 'A', step: 1 }, { n: 'b', kind: 'pending', value: 'B', step: 2 }] });
add_('suspenselist: no revealOrder', el(T.list, null, Sus('f1', 'a'), 'b'));
add_('suspenselist: forwards without suspension', el(T.list, { revealOrder: 'forwards' }, Sus('f', 'one'), Sus('f', 'two')));
add_('suspenselist: iterable children', el(T.list, { revealOrder: 'forwards' }, iter(Sus('f', 'one'), 'two')));

// ---- errors -------------------------------------------------------------------------------
add_('error: undefined element type', el(U, null));
add_('error: null element type', el(null, null));
add_('error: number element type', el(5, null));
add_('error: object element type', el(o({ a: 1 }), null));
add_('error: boolean element type', el(true, null));
add_('error: plain object child', el('div', null, o({ a: 1, b: 2 })));
add_('error: integer-keyed object child', el('div', null, o({ z: 1, 1: 2, 0: 3 })));
add_('error: error object child', el('div', null, err('as child')));
add_('error: portal is not supported', el('div', null, api('createPortal', el('i'), o({ nodeType: 1 }))));
add_('error: createPortal without container', el('div', null, api('createPortal', el('i'), null)));
add_('error: component throws Error', el('div', null, el(C('Boom'))), { comps: { Boom: Show([{ throw: err('boom') }], 'x') } });
add_('error: component throws TypeError', el('div', null, el(C('Boom'))), { comps: { Boom: Show([{ throw: { $: 'typeerror', m: 'bad type' } }], 'x') } });
add_('error: component throws object', el('div', null, el(C('Boom'))), { comps: { Boom: Show([{ throw: o({ code: 1 }) }], 'x') } });
add_('error: view transitions not supported', el(T.vt, null, 'x'));
add_('error: scope not supported', el(T.scope, null, 'x'));
add_('error: symbol children ignored', el('div', null, 'a', sym('s'), 'b'));
add_('error: function children ignored', el('div', null, 'a', fn([], 'never'), 'b'));
add_('error: requestFormReset on the server', el(C('RF')), { comps: { RF: Show([doo(api('requestFormReset', null))], 'x') } });
add_('error: flushSync returns callback result', el('p', null, api('flushSync', fn([], 'flushed'))));
add_('error: hook outside render via stale setter', el('div', null, el(C('Owner'))), { comps: { Owner: Show([letv(['s', 'set'], hook('useState', 1))], v('s')) } });
add_('error: nested renderToString inside a component', el('div', null, el(C('Nested'))), { comps: { Nested: Show([], api('renderToString', el('span', null, 'inner', 'text'))) } });
add_('error: version', el('p', null, api('version')));
add_('error: cache passthrough', el('p', null, call(api('cache', fn(['x'], cat('c:', v('x')))), 1)));
add_('error: startTransition runs scope', el(C('ST')), { comps: { ST: Show([doo(api('startTransition', fn([], gset('ran', 'yes'))))], str(gget('ran'))) } });

process.stdout.write(JSON.stringify({ cases }, null, 1) + '\n');
