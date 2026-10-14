// Generates parity/gracefulfs/cases.json, the cases parity/oracle_gracefulfs.cjs runs through the original
// graceful-fs and pkgs/graceful-fs/parity_test.go through the Go port.
//
// Run: node tools/gen_gracefulfs_cases.cjs > parity/gracefulfs/cases.json
//
// A case: {name, env, missing, script, bases, load, steps} (see the oracle's header and makeFake):
//   env     {platform, gfsPlatform, uid (null: no process.getuid), osymlink (null: none), clock}
//   script  {"open": [outcome…], "open:a": [outcome…], …}; outcome {err, ok, ms, times, throw}
//   steps   {at, on ("g0"/"g1" loaded copies, "b0"/"b1" fake fs modules), call, args}
// Step arguments: JSON values, {undef: true}, {buf: hex}, {opts: object}, {cb: n, then: [steps]}.
'use strict';

const cb = (n, then) => (then ? { cb: n, then } : { cb: n });
const U = { undef: true };
const buf = s => ({ buf: Buffer.from(s, 'utf8').toString('hex') });
const hex = h => ({ buf: h });
const opts = o => ({ opts: o });
const step = (at, call, args, on) => (on ? { at, on, call, args } : { at, call, args });
const err = (code, extra) => Object.assign({ err: code }, extra);
const ok = (v, extra) => Object.assign({ ok: v }, extra);
const done = extra => Object.assign({}, extra); // the success of a function without a result
const forever = code => ({ err: code, times: 1000000 });

const cases = [];
const add = c => cases.push(c);

// ---------------------------------------------------------------- the queue, through open
add({ name: 'open: success', script: { open: [ok(7)] }, steps: [step(0, 'open', ['a', 'r', cb(1)])] });
add({ name: 'open: EMFILE 3 times, then success', script: { open: [err('EMFILE', { times: 3 }), ok(7)] },
  steps: [step(0, 'open', ['a', 'r', cb(1)])] });
add({ name: 'open: ENFILE 4 times, then success', script: { open: [err('ENFILE', { times: 4 }), ok(7)] },
  steps: [step(0, 'open', ['a', 'r', cb(1)])] });
add({ name: 'open: EMFILE and ENFILE alternating', script: { open: [err('EMFILE'), err('ENFILE'), err('EMFILE'), err('ENFILE'), ok(3)] },
  steps: [step(0, 'open', ['a', 'r', cb(1)])] });
add({ name: 'open: ENOENT is not retried', script: { open: [err('ENOENT')] }, steps: [step(0, 'open', ['a', 'r', cb(1)])] });
add({ name: 'open: EMFILE, then ENOENT ends the retries', script: { open: [err('EMFILE', { times: 2 }), err('ENOENT')] },
  steps: [step(0, 'open', ['a', 'r', cb(1)])] });
add({ name: 'open: EMFILE with completion delays', script: { open: [err('EMFILE', { ms: 3 }), err('EMFILE', { ms: 7 }), err('EMFILE', { ms: 1 }), ok(4, { ms: 2 })] },
  steps: [step(0, 'open', ['a', 'r', cb(1)])] });
add({ name: 'open: EMFILE until the 60 s cutoff', script: { open: [forever('EMFILE')] }, steps: [step(0, 'open', ['a', 'r', cb(1)])] });
add({ name: 'open: ENFILE until the 60 s cutoff, delivered late', script: { open: [{ err: 'ENFILE', ms: 37, times: 1000000 }] },
  steps: [step(0, 'open', ['a', 'r', cb(1)])] });
add({ name: 'open: a retry at 59999 ms still runs', script: { open: [err('EMFILE'), err('EMFILE', { ms: 59897 }), ok(9)] },
  steps: [step(0, 'open', ['a', 'r', cb(1)])] });
add({ name: 'open: at 60000 ms the cutoff wins, with the latest error', script: { open: [err('EMFILE'), err('ENFILE', { ms: 59898 }), ok(9)] },
  steps: [step(0, 'open', ['a', 'r', cb(1)])] });
add({ name: 'open: the cutoff checks the time of the retry, not of the failure', script: { open: [err('EMFILE'), err('EMFILE', { ms: 59997 }), ok(9)] },
  steps: [step(0, 'open', ['a', 'r', cb(1)])] });
add({ name: 'open: a failure delivered after 60 s times out at once', script: { open: [err('EMFILE'), err('EMFILE', { ms: 60000 }), ok(9)] },
  steps: [step(0, 'open', ['a', 'r', cb(1)])] });
add({ name: 'open: the cutoff without a callback', script: { open: [err('EMFILE'), err('EMFILE', { ms: 60000 })] },
  steps: [step(0, 'open', ['a', 'r'])] });
add({ name: 'open: a clock at 0 makes startTime 0, which || replaces', env: { clock: 0 },
  script: { open: [err('EMFILE'), err('EMFILE', { ms: 59998 }), ok(9)] }, steps: [step(0, 'open', ['a', 'r', cb(1)])] });
add({ name: 'open: the same script on a real clock times out', script: { open: [err('EMFILE'), err('EMFILE', { ms: 59998 }), ok(9)] },
  steps: [step(0, 'open', ['a', 'r', cb(1)])] });
add({ name: 'open: a clock at 0, EMFILE until the cutoff', env: { clock: 0 }, script: { open: [forever('EMFILE')] },
  steps: [step(0, 'open', ['a', 'r', cb(1)])] });
add({ name: 'open: a negative clock', env: { clock: -5000 }, script: { open: [err('EMFILE', { times: 5 }), ok(2)] },
  steps: [step(0, 'open', ['a', 'r', cb(1)])] });
add({ name: 'open: three entries interleave under backoff',
  script: { 'open:a': [err('EMFILE', { times: 4 }), ok(10)], 'open:b': [err('EMFILE', { times: 2 }), ok(11)], 'open:c': [err('ENFILE', { times: 6 }), ok(12)] },
  steps: [step(0, 'open', ['a', 'r', cb(1)]), step(0, 'open', ['b', 'r', cb(2)]), step(0, 'open', ['c', 'r', cb(3)])] });
add({ name: 'open: entries queued at different times',
  script: { 'open:a': [err('EMFILE', { times: 5 }), ok(10)], 'open:b': [err('EMFILE', { times: 2, ms: 2 }), ok(11)], 'open:c': [err('ENFILE', { times: 3 }), ok(12)] },
  steps: [step(0, 'open', ['a', 'r', cb(1)]), step(7, 'open', ['b', 'w', cb(2)]), step(13, 'open', ['c', 'a', cb(3)])] });
add({ name: 'open: many entries in long backoff',
  script: { 'open:a': [err('EMFILE', { times: 12 }), ok(10)], 'open:b': [err('EMFILE', { times: 9 }), ok(11)], 'open:c': [err('EMFILE', { times: 11 }), ok(12)],
    'open:d': [err('EMFILE', { times: 3 }), ok(13)], 'open:e': [err('EMFILE', { times: 10 }), ok(14)] },
  steps: ['a', 'b', 'c', 'd', 'e'].map((p, i) => step(i * 3, 'open', [p, 'r', cb(i + 1)])) });
add({ name: 'open: one entry times out while another succeeds',
  script: { 'open:a': [forever('EMFILE')], 'open:b': [err('EMFILE', { times: 2 }), ok(5, { ms: 30000 }), ok(6)] },
  steps: [step(0, 'open', ['a', 'r', cb(1)]), step(10, 'open', ['b', 'r', cb(2)])] });

// ---------------------------------------------------------------- close and the queue
const backoff = { 'open:a': [err('EMFILE', { times: 8 }), ok(7)] };
add({ name: 'close: a successful close mid-backoff resets the delay', script: backoff,
  steps: [step(0, 'open', ['a', 'r', cb(1)]), step(60, 'close', [5, cb(2)])] });
add({ name: 'close: a failed close does not reset', script: Object.assign({ close: [err('EBADF')] }, backoff),
  steps: [step(0, 'open', ['a', 'r', cb(1)]), step(60, 'close', [5, cb(2)])] });
add({ name: 'close: completion delayed, reset when it completes', script: Object.assign({ close: [done({ ms: 20 })] }, backoff),
  steps: [step(0, 'open', ['a', 'r', cb(1)]), step(40, 'close', [5, cb(2)])] });
add({ name: 'close: without a callback', script: backoff, steps: [step(0, 'open', ['a', 'r', cb(1)]), step(60, 'close', [5])] });
add({ name: 'close: several entries are all reset',
  script: { 'open:a': [err('EMFILE', { times: 8 }), ok(7)], 'open:b': [err('ENFILE', { times: 6 }), ok(8)], 'readFile:c': [err('EMFILE', { times: 5 }), ok('c')] },
  steps: [step(0, 'open', ['a', 'r', cb(1)]), step(1, 'open', ['b', 'r', cb(2)]), step(2, 'readFile', ['c', cb(3)]), step(70, 'close', [5, cb(4)])] });
add({ name: 'closeSync: a successful closeSync resets', script: backoff,
  steps: [step(0, 'open', ['a', 'r', cb(1)]), step(60, 'closeSync', [5])] });
add({ name: 'closeSync: a closeSync that throws does not reset', script: Object.assign({ closeSync: [err('EBADF')] }, backoff),
  steps: [step(0, 'open', ['a', 'r', cb(1)]), step(60, 'closeSync', [5])] });
add({ name: 'close through the underlying fs module resets the queue', script: backoff,
  steps: [step(0, 'open', ['a', 'r', cb(1)]), step(60, 'close', [5, cb(2)], 'b0')] });
add({ name: 'closeSync through the underlying fs module resets the queue', script: backoff,
  steps: [step(0, 'open', ['a', 'r', cb(1)]), step(60, 'closeSync', [5], 'b0')] });
add({ name: 'close: a reset restarts the 60 s cutoff', script: { open: [forever('EMFILE')] },
  steps: [step(0, 'open', ['a', 'r', cb(1)]), step(30000, 'closeSync', [5])] });
add({ name: 'close: a reset while the call is in flight keeps its startTime', script: { open: [err('EMFILE'), err('EMFILE', { ms: 59000 }), forever('EMFILE')] },
  steps: [step(0, 'open', ['a', 'r', cb(1)]), step(100, 'closeSync', [5])] });
add({ name: 'close: an empty queue', steps: [step(0, 'close', [5, cb(1)]), step(1, 'closeSync', [6])] });

// ---------------------------------------------------------------- the other queued functions
add({ name: 'readFile: EMFILE twice, then data', script: { readFile: [err('EMFILE', { times: 2 }), ok('data')] },
  steps: [step(0, 'readFile', ['f', cb(1)])] });
add({ name: 'readFile: argument shapes',
  script: { readFile: [ok('a'), ok('b'), ok('c'), ok(hex('6869')), ok('e')] },
  steps: [step(0, 'readFile', ['f', 'utf8', cb(1)]), step(1, 'readFile', ['f', opts({ encoding: 'utf8', flag: 'r' }), cb(2)]),
    step(2, 'readFile', ['f', U, cb(3)]), step(3, 'readFile', ['f', null, cb(4)]), step(4, 'readFile', ['f'])] });
add({ name: 'readFile: ENFILE until the cutoff', script: { readFile: [forever('ENFILE')] }, steps: [step(0, 'readFile', ['f', 'utf8', cb(1)])] });
add({ name: 'writeFile: EMFILE, then success; argument shapes',
  script: { 'writeFile:f': [err('EMFILE'), done()] },
  steps: [step(0, 'writeFile', ['f', 'x', cb(1)]), step(0, 'writeFile', ['g', hex('6162'), opts({ mode: 420 }), cb(2)]),
    step(0, 'writeFile', ['h', 'y', U, cb(3)]), step(1, 'writeFile', ['h', 'z'])] });
add({ name: 'writeFile: ENOENT', script: { writeFile: [err('ENOENT')] }, steps: [step(0, 'writeFile', ['f', 'x', 'utf8', cb(1)])] });
add({ name: 'appendFile: ENFILE, then success; argument shapes',
  script: { 'appendFile:f': [err('ENFILE', { times: 3 }), done()] },
  steps: [step(0, 'appendFile', ['f', 'x', cb(1)]), step(0, 'appendFile', ['g', 'y', opts({ flag: 'a' }), cb(2)]), step(1, 'appendFile', ['g', 'z'])] });
add({ name: 'appendFile: absent from the module stays absent', missing: ['appendFile'], steps: [step(0, 'appendFile', ['f', 'x', cb(1)])] });
add({ name: 'copyFile: argument shapes; flags default to 0',
  steps: [step(0, 'copyFile', ['a', 'b', cb(1)]), step(1, 'copyFile', ['a', 'b', 1, cb(2)]), step(2, 'copyFile', ['a', 'b', null, cb(3)]),
    step(3, 'copyFile', ['a', 'b', U, cb(4)]), step(4, 'copyFile', ['a', 'b'])] });
add({ name: 'copyFile: EMFILE, then success', script: { copyFile: [err('EMFILE', { times: 3 }), done()] },
  steps: [step(0, 'copyFile', ['a', 'b', cb(1)])] });
add({ name: 'copyFile: EEXIST is not retried', script: { copyFile: [err('EEXIST')] }, steps: [step(0, 'copyFile', ['a', 'b', 1, cb(1)])] });
add({ name: 'copyFile: absent from the module stays absent', missing: ['copyFile'], steps: [step(0, 'copyFile', ['a', 'b', cb(1)])] });
add({ name: 'open: argument shapes',
  script: { open: [ok(1), ok(2), ok(3), ok(4), ok(5), ok(6)] },
  steps: [step(0, 'open', ['a', 'r', cb(1)]), step(1, 'open', ['a', 'r', 438, cb(2)]), step(2, 'open', ['a', 'r', U, cb(3)]),
    step(3, 'open', ['a', cb(4)]), step(4, 'open', ['a', 'r']), step(5, 'open', ['a', 'r', null, cb(6)])] });
add({ name: 'the queue holds calls of every kind',
  script: { open: [err('EMFILE', { times: 3 }), ok(3)], readFile: [err('ENFILE', { times: 2 }), ok('r')], writeFile: [err('EMFILE', { times: 4 }), done()],
    appendFile: [err('EMFILE'), done()], copyFile: [err('ENFILE', { times: 2 }), done()], readdir: [err('EMFILE', { times: 3 }), ok(['b', 'a'])] },
  steps: [step(0, 'open', ['o', 'r', cb(1)]), step(0, 'readFile', ['r', cb(2)]), step(1, 'writeFile', ['w', 'x', cb(3)]),
    step(1, 'appendFile', ['p', 'y', cb(4)]), step(2, 'copyFile', ['s', 'd', cb(5)]), step(2, 'readdir', ['dir', cb(6)])] });

// ---------------------------------------------------------------- readdir
add({ name: 'readdir: EMFILE, then sorted names', script: { readdir: [err('EMFILE', { times: 2 }), ok(['c', 'a', 'b'])] },
  steps: [step(0, 'readdir', ['d', cb(1)])] });
add({ name: 'readdir: argument shapes',
  script: { readdir: [ok(['b', 'a']), ok([hex('62'), hex('61')]), ok([{ dirent: 'b' }, { dirent: 'a' }]), ok(['d', 'c']), ok(['f', 'e'])] },
  steps: [step(0, 'readdir', ['d', cb(1)]), step(1, 'readdir', ['d', 'buffer', cb(2)]), step(2, 'readdir', ['d', opts({ withFileTypes: true }), cb(3)]),
    step(3, 'readdir', ['d', U, cb(4)]), step(4, 'readdir', ['d'])] });
add({ name: 'readdir: names sort by UTF-16 code units',
  script: { readdir: [ok(['b', 'B', 'a', '_', '~', '10', '9', 'é', 'é', '😀', '', '～', '�', '􏿿',
    '𐀀', '日本', '', 'a\u0000', ' ', 'Z', 'aa', 'a-b', 'a.b', 'ÿ', 'Ā'])] },
  steps: [step(0, 'readdir', ['d', cb(1)])] });
add({ name: 'readdir: Buffers sort by their UTF-8 decoding',
  script: { readdir: [ok([hex('62'), hex('61'), hex('f09f98'), hex('ff'), hex('e282'), hex('ffffff'), hex('61f09f62'), hex('f09f9880'),
    hex('eebfbf'), hex('efbfbd'), hex('c080'), hex(''), hex('eda080'), hex('f4908080'), hex('e08080'), hex('f08080'), hex('7a'),
    hex('c3a9'), hex('65cc81'), hex('efbdbe'), hex('ee8080'), hex('f0908080'), hex('c2'), hex('e0a0'), hex('f48fbfbf')])] },
  steps: [step(0, 'readdir', ['d', 'buffer', cb(1)])] });
add({ name: 'readdir: Dirents keep their order',
  script: { readdir: [ok([{ dirent: 'b' }, { dirent: 'a' }, { dirent: 'c' }, { dirent: '😀' }, { dirent: '' }, { dirent: 'A' }])] },
  steps: [step(0, 'readdir', ['d', opts({ withFileTypes: true }), cb(1)])] });
add({ name: 'readdir: values of every kind',
  script: { readdir: [ok(['b', null, 10, 9, true, U, 'a', hex('6e756c6b'), { dirent: 'x' }, false, 'nulk', 1.5, -1, U, 'undefined'])] },
  steps: [step(0, 'readdir', ['d', cb(1)])] });
add({ name: 'readdir: an error that comes with names still sorts them', script: { readdir: [err('EIO', { ok: ['b', 'a'] })] },
  steps: [step(0, 'readdir', ['d', cb(1)])] });
add({ name: 'readdir: EMFILE with names is retried', script: { readdir: [err('EMFILE', { ok: ['b', 'a'] }), ok(['d', 'c'])] },
  steps: [step(0, 'readdir', ['d', cb(1)])] });
add({ name: 'readdir: an empty directory', script: { readdir: [ok([])] }, steps: [step(0, 'readdir', ['d', cb(1)])] });
add({ name: 'readdir: ENOENT', script: { readdir: [err('ENOENT')] }, steps: [step(0, 'readdir', ['d', cb(1)])] });
add({ name: 'readdir: ENFILE until the cutoff', script: { readdir: [forever('ENFILE')] }, steps: [step(0, 'readdir', ['d', cb(1)])] });

// ---------------------------------------------------------------- chown, chmod
const ownership = [
  ['chown', ['p', 1, 2]], ['fchown', [3, 1, 2]], ['lchown', ['p', 1, 2]],
  ['chmod', ['p', 420]], ['fchmod', [3, 420]], ['lchmod', ['p', 420]],
];
for (const [uidName, uid] of [['nonroot', 1000], ['root', 0], ['no getuid', null]]) {
  for (const [fn, args] of ownership) {
    add({ name: `${fn}: errors as ${uidName}`, env: { uid },
      script: { [fn]: ['EPERM', 'EINVAL', 'ENOSYS', 'EACCES', 'ENOENT'].map(c => err(c)).concat([done()]) },
      steps: [0, 1, 2, 3, 4, 5].map(i => step(i, fn, args.concat([cb(i + 1)]))) });
    add({ name: `${fn}Sync: errors as ${uidName}`, env: { uid },
      script: { [fn + 'Sync']: ['EPERM', 'EINVAL', 'ENOSYS', 'EACCES', 'ENOENT'].map(c => err(c)).concat([done()]) },
      steps: [0, 1, 2, 3, 4, 5].map(i => step(i, fn + 'Sync', args)) });
  }
}
add({ name: 'chown: a uid that is neither 0 nor 1000', env: { uid: -1 }, script: { chown: [err('EPERM')] }, steps: [step(0, 'chown', ['p', 1, 2, cb(1)])] });
add({ name: 'chown and chmod: no callback', script: { chown: [err('EACCES')], chmod: [err('EPERM')] },
  steps: [step(0, 'chown', ['p', 1, 2]), step(0, 'chmod', ['p', 420])] });
add({ name: 'chown and chmod: completion delays', script: { chown: [err('EPERM', { ms: 5 })], fchmod: [done({ ms: 3 })] },
  steps: [step(0, 'chown', ['p', 1, 2, cb(1)]), step(0, 'fchmod', [3, 420, cb(2)])] });
add({ name: 'chown and chmod: EMFILE is not retried', script: { chown: [err('EMFILE')], chmod: [err('ENFILE')] },
  steps: [step(0, 'chown', ['p', 1, 2, cb(1)]), step(0, 'chmod', ['p', 420, cb(2)])] });

// ---------------------------------------------------------------- no-op fallbacks
add({ name: 'lchmod: a no-op where the module lacks it', missing: ['lchmod', 'lchmodSync'],
  steps: [step(0, 'lchmod', ['p', 420, cb(1)]), step(0, 'lchmodSync', ['p', 420]), step(1, 'lchmod', ['p', 420]), step(2, 'chmod', ['p', 420, cb(2)])] });
add({ name: 'lchmod: missing lchmod also replaces an existing lchmodSync', missing: ['lchmod'], script: { lchmodSync: [err('EACCES')] },
  steps: [step(0, 'lchmodSync', ['p', 420]), step(0, 'lchmod', ['p', 420, cb(1)])] });
add({ name: 'lchmod: stays absent without chmod', missing: ['chmod', 'lchmod', 'lchmodSync'], steps: [step(0, 'lchmod', ['p', 420, cb(1)])] });
add({ name: 'lchown: a no-op where the module lacks it', missing: ['lchown', 'lchownSync'],
  steps: [step(0, 'lchown', ['p', 1, 2, cb(1)]), step(0, 'lchownSync', ['p', 1, 2]), step(1, 'lchown', ['p', 1, 2])] });
add({ name: 'lchown: missing lchown also replaces an existing lchownSync', missing: ['lchown'], script: { lchownSync: [err('EACCES')] },
  steps: [step(0, 'lchownSync', ['p', 1, 2])] });
add({ name: 'lchown: stays absent without chown', missing: ['chown', 'lchown', 'lchownSync'], steps: [step(0, 'lchown', ['p', 1, 2, cb(1)])] });
add({ name: 'the no-op callbacks run before a timer or completion', missing: ['lchmod', 'lchown'], script: { open: [ok(3)] },
  steps: [step(0, 'open', ['a', 'r', cb(1)]), step(0, 'lchown', ['p', 1, 2, cb(2)]), step(0, 'lchmod', ['p', 420, cb(3)])] });
add({ name: 'lutimes: the module\'s own is used', script: { lutimes: [err('ENOENT'), done()] },
  steps: [step(0, 'lutimes', ['p', 1, 2, cb(1)]), step(1, 'lutimes', ['p', 1, 2, cb(2)]), step(2, 'lutimesSync', ['p', 1, 2])] });
add({ name: 'lutimes: a no-op where the module lacks it but has futimes', missing: ['lutimes', 'lutimesSync'],
  steps: [step(0, 'lutimes', ['p', 1, 2, cb(1)]), step(0, 'lutimesSync', ['p', 1, 2]), step(1, 'lutimes', ['p', 1, 2])] });
add({ name: 'lutimes: stays absent without futimes', missing: ['lutimes', 'lutimesSync', 'futimes'], steps: [step(0, 'lutimes', ['p', 1, 2, cb(1)])] });
add({ name: 'lutimes: missing lutimes with futimesSync but no futimes', missing: ['lutimes', 'futimes'], steps: [step(0, 'lutimesSync', ['p', 1, 2])] });
const symlink = { osymlink: 2097152 };
add({ name: 'lutimes: through open(O_SYMLINK), futimes and close', env: symlink, missing: ['lutimes', 'lutimesSync'], script: { open: [ok(9)] },
  steps: [step(0, 'lutimes', ['p', 1, 2, cb(1)])] });
add({ name: 'lutimes: O_SYMLINK, the open fails', env: symlink, missing: ['lutimes', 'lutimesSync'], script: { open: [err('ELOOP', { times: 2 })] },
  steps: [step(0, 'lutimes', ['p', 1, 2, cb(1)]), step(1, 'lutimes', ['p', 1, 2])] });
add({ name: 'lutimes: O_SYMLINK, futimes fails and close succeeds', env: symlink, missing: ['lutimes', 'lutimesSync'],
  script: { open: [ok(9)], futimes: [err('EPERM')] }, steps: [step(0, 'lutimes', ['p', 1, 2, cb(1)])] });
add({ name: 'lutimes: O_SYMLINK, futimes succeeds and close fails', env: symlink, missing: ['lutimes', 'lutimesSync'],
  script: { open: [ok(9)], close: [err('EIO')] }, steps: [step(0, 'lutimes', ['p', 1, 2, cb(1)])] });
add({ name: 'lutimes: O_SYMLINK, both fail', env: symlink, missing: ['lutimes', 'lutimesSync'],
  script: { open: [ok(9)], futimes: [err('EPERM')], close: [err('EIO')] }, steps: [step(0, 'lutimes', ['p', 1, 2, cb(1)])] });
add({ name: 'lutimes: O_SYMLINK, without a callback', env: symlink, missing: ['lutimes', 'lutimesSync'], script: { open: [ok(9)] },
  steps: [step(0, 'lutimes', ['p', 1, 2])] });
add({ name: 'lutimes: O_SYMLINK, the open goes through the queue', env: symlink, missing: ['lutimes', 'lutimesSync'],
  script: { open: [err('EMFILE', { times: 3 }), ok(9)] }, steps: [step(0, 'lutimes', ['p', 1, 2, cb(1)])] });
add({ name: 'lutimes: O_SYMLINK, its close resets the queue', env: symlink, missing: ['lutimes', 'lutimesSync'],
  script: { 'open:a': [err('EMFILE', { times: 8 }), ok(7)], 'open:p': [ok(9)] },
  steps: [step(0, 'open', ['a', 'r', cb(1)]), step(60, 'lutimes', ['p', 1, 2, cb(2)])] });
add({ name: 'lutimesSync: O_SYMLINK', env: symlink, missing: ['lutimes', 'lutimesSync'],
  script: { openSync: [ok(9), err('ELOOP'), ok(10), ok(11), ok(12)], futimesSync: [done(), err('EPERM'), done(), err('EPERM')], closeSync: [done(), err('EIO'), err('EIO'), done()] },
  steps: [step(0, 'lutimesSync', ['p', 1, 2]), step(1, 'lutimesSync', ['q', 1, 2]), step(2, 'lutimesSync', ['r', 1, 2]),
    step(3, 'lutimesSync', ['s', 1, 2]), step(4, 'lutimesSync', ['t', 1, 2])] });
add({ name: 'lutimesSync: O_SYMLINK, its closeSync resets the queue', env: symlink, missing: ['lutimes', 'lutimesSync'],
  script: { 'open:a': [err('EMFILE', { times: 8 }), ok(7)], openSync: [ok(9)] },
  steps: [step(0, 'open', ['a', 'r', cb(1)]), step(60, 'lutimesSync', ['p', 1, 2])] });
add({ name: 'lutimes: O_SYMLINK without futimes is a missing lutimes', env: symlink, missing: ['lutimes', 'lutimesSync', 'futimes'],
  steps: [step(0, 'lutimes', ['p', 1, 2, cb(1)])] });

// ---------------------------------------------------------------- stat
const statPairs = [[-2, -1], [0, 0], [1000, 1000], [-4294967296, -4294967297], [-1, 5], [5, -1], [4294967295, 4294967296]];
for (const fn of ['stat', 'fstat', 'lstat']) {
  const target = fn === 'fstat' ? 3 : 'p';
  add({ name: `${fn}: uid and gid corrections`, script: { [fn]: statPairs.map(p => ok({ stats: p })) },
    steps: statPairs.map((_, i) => step(i, fn, [target, cb(i + 1)])) });
  add({ name: `${fn}Sync: uid and gid corrections`, script: { [fn + 'Sync']: statPairs.map(p => ok({ stats: p })) },
    steps: statPairs.map((_, i) => step(i, fn + 'Sync', [target])) });
}
add({ name: 'stat: options decide how many arguments the module gets',
  script: { stat: [1, 2, 3, 4, 5, 6, 7, 8, 9].map(i => ok({ stats: [-i, i] })) },
  steps: [step(0, 'stat', ['p', cb(1)]), step(1, 'stat', ['p', opts({ bigint: false }), cb(2)]), step(2, 'stat', ['p', null, cb(3)]),
    step(3, 'stat', ['p', 0, cb(4)]), step(4, 'stat', ['p', '', cb(5)]), step(5, 'stat', ['p', U, cb(6)]), step(6, 'stat', ['p']),
    step(7, 'stat', ['p', 'x', cb(8)]), step(8, 'stat', ['p', 1, cb(9)])] });
add({ name: 'statSync: options decide how many arguments the module gets',
  script: { statSync: [1, 2, 3, 4, 5].map(i => ok({ stats: [-i, i] })) },
  steps: [step(0, 'statSync', ['p']), step(1, 'statSync', ['p', opts({ throwIfNoEntry: false })]), step(2, 'statSync', ['p', null]),
    step(3, 'statSync', ['p', 0]), step(4, 'statSync', ['p', true])] });
add({ name: 'stat: errors are not corrected', script: { stat: [err('ENOENT')], lstat: [err('EMFILE')], statSync: [err('ENOENT'), ok(U)] },
  steps: [step(0, 'stat', ['p', cb(1)]), step(0, 'lstat', ['p', cb(2)]), step(0, 'statSync', ['p']), step(0, 'statSync', ['p', opts({ throwIfNoEntry: false })])] });
add({ name: 'stat: absent from the module stays absent', missing: ['stat', 'statSync', 'fstat'],
  steps: [step(0, 'stat', ['p', cb(1)]), step(0, 'statSync', ['p'])] });

// ---------------------------------------------------------------- rename on win32
const win = { platform: 'win32', uid: null };
add({ name: 'rename: not patched on linux', script: { rename: [err('EPERM')] }, steps: [step(0, 'rename', ['a', 'b', cb(1)])] });
add({ name: 'rename: win32, success', env: win, steps: [step(0, 'rename', ['a', 'b', cb(1)])] });
add({ name: 'rename: win32, EPERM while the destination is missing is retried', env: win,
  script: { rename: [err('EPERM'), done()], stat: [err('ENOENT')] }, steps: [step(0, 'rename', ['a', 'b', cb(1)])] });
add({ name: 'rename: win32, EACCES and EBUSY are retried too', env: win,
  script: { rename: [err('EACCES'), err('EBUSY'), err('EACCES'), done()], stat: [err('ENOENT', { times: 3 })] }, steps: [step(0, 'rename', ['a', 'b', cb(1)])] });
add({ name: 'rename: win32, the destination exists: the rename error', env: win,
  script: { rename: [err('EACCES')], stat: [ok({ stats: [0, 0] })] }, steps: [step(0, 'rename', ['a', 'b', cb(1)])] });
add({ name: 'rename: win32, stat fails otherwise: the rename error', env: win,
  script: { rename: [err('EBUSY'), err('EPERM')], stat: [err('ENOENT'), err('EACCES')] }, steps: [step(0, 'rename', ['a', 'b', cb(1)])] });
add({ name: 'rename: win32, backoff 0, 10, … 100, 100', env: win,
  script: { rename: [err('EPERM', { times: 14 }), done()], stat: [err('ENOENT', { times: 14 })] }, steps: [step(0, 'rename', ['a', 'b', cb(1)])] });
add({ name: 'rename: win32, slow stat and rename', env: win,
  script: { rename: [err('EPERM', { ms: 7, times: 3 }), done({ ms: 4 })], stat: [err('ENOENT', { ms: 5, times: 3 })] }, steps: [step(0, 'rename', ['a', 'b', cb(1)])] });
add({ name: 'rename: win32, EPERM until the 60 s window closes', env: win,
  script: { rename: [forever('EPERM')], stat: [forever('ENOENT')] }, steps: [step(0, 'rename', ['a', 'b', cb(1)])] });
add({ name: 'rename: win32, an error at 59999 ms is retried, at 60000 it is final', env: win,
  script: { rename: [err('EPERM', { ms: 59999 }), err('EPERM'), done()], stat: [err('ENOENT', { times: 2 })] }, steps: [step(0, 'rename', ['a', 'b', cb(1)])] });
add({ name: 'rename: win32, an error at 60000 ms is final', env: win,
  script: { rename: [err('EPERM', { ms: 60000 }), done()], stat: [err('ENOENT')] }, steps: [step(0, 'rename', ['a', 'b', cb(1)])] });
add({ name: 'rename: win32, ENOENT and EXDEV are not retried', env: win,
  script: { 'rename:a': [err('ENOENT')], 'rename:c': [err('EXDEV')] }, steps: [step(0, 'rename', ['a', 'b', cb(1)]), step(0, 'rename', ['c', 'd', cb(2)])] });
add({ name: 'rename: win32, the stat is the corrected stat with a callback only', env: win,
  script: { rename: [err('EPERM'), done()], stat: [err('ENOENT')] }, steps: [step(0, 'rename', ['a', 'b', cb(1)])] });
add({ name: 'rename: win32, no callback, success', env: win, steps: [step(0, 'rename', ['a', 'b'])] });
add({ name: 'rename: win32, no callback, the destination exists', env: win,
  script: { rename: [err('EPERM')], stat: [ok({ stats: [0, 0] })] }, steps: [step(0, 'rename', ['a', 'b'])] });
add({ name: 'rename: win32, no callback, the retry succeeds', env: win,
  script: { rename: [err('EPERM'), done()], stat: [err('ENOENT')] }, steps: [step(0, 'rename', ['a', 'b'])] });
add({ name: 'rename: win32, two renames at once', env: win,
  script: { 'rename:a': [err('EPERM', { times: 3 }), done()], 'rename:c': [err('EBUSY'), done()], stat: [err('ENOENT', { times: 4 })] },
  steps: [step(0, 'rename', ['a', 'b', cb(1)]), step(5, 'rename', ['c', 'd', cb(2)])] });
add({ name: 'rename: GRACEFUL_FS_PLATFORM=win32 on linux', env: { platform: 'linux', gfsPlatform: 'win32' },
  script: { rename: [err('EPERM'), done()], stat: [err('ENOENT')] }, steps: [step(0, 'rename', ['a', 'b', cb(1)])] });
add({ name: 'rename: GRACEFUL_FS_PLATFORM=linux on win32', env: { platform: 'win32', gfsPlatform: 'linux', uid: null },
  script: { rename: [err('EPERM')] }, steps: [step(0, 'rename', ['a', 'b', cb(1)])] });
add({ name: 'rename: an empty GRACEFUL_FS_PLATFORM falls back to process.platform', env: { platform: 'win32', gfsPlatform: '', uid: null },
  script: { rename: [err('EPERM'), done()], stat: [err('ENOENT')] }, steps: [step(0, 'rename', ['a', 'b', cb(1)])] });
add({ name: 'rename: win32, absent from the module stays absent', env: win, missing: ['rename'], steps: [step(0, 'rename', ['a', 'b', cb(1)])] });
add({ name: 'rename: win32 without stat throws when it retries', env: win, missing: ['stat'],
  script: { rename: [err('EPERM')] }, steps: [step(0, 'rename', ['a', 'b', cb(1)])] });

// ---------------------------------------------------------------- read
const rbuf = hex('00000000000000');
add({ name: 'read: EAGAIN 10 times, then success', script: { read: [err('EAGAIN', { times: 10 }), ok(5)] },
  steps: [step(0, 'read', [3, rbuf, 0, 7, null, cb(1)])] });
add({ name: 'read: EAGAIN 11 times', script: { read: [err('EAGAIN', { times: 11 }), ok(5)] }, steps: [step(0, 'read', [3, rbuf, 0, 7, null, cb(1)])] });
add({ name: 'read: EAGAIN with completion delays', script: { read: [err('EAGAIN', { ms: 3, times: 4 }), ok(2, { ms: 1 })] },
  steps: [step(0, 'read', [3, rbuf, 1, 2, 10, cb(1)])] });
add({ name: 'read: other errors are not retried', script: { read: [err('EBADF'), err('EMFILE'), ok(0)] },
  steps: [step(0, 'read', [3, rbuf, 0, 7, null, cb(1)]), step(1, 'read', [3, rbuf, 0, 7, null, cb(2)]), step(2, 'read', [3, rbuf, 0, 7, -1, cb(3)])] });
add({ name: 'read: each call counts its own retries', script: { read: [err('EAGAIN', { times: 6 }), ok(1), err('EAGAIN', { times: 10 }), ok(2)] },
  steps: [step(0, 'read', [3, rbuf, 0, 7, null, cb(1)]), step(1, 'read', [3, rbuf, 0, 7, null, cb(2)])] });
add({ name: 'read: without a callback the module gets none', script: { read: [ok(1)] }, steps: [step(0, 'read', [3, rbuf, 0, 7, null])] });
add({ name: 'read: the short form read(fd, cb) is not retried', script: { read: [ok(1)] }, steps: [step(0, 'read', [3, cb(1)])] });
add({ name: 'readSync: EAGAIN 10 times, then success', script: { readSync: [err('EAGAIN', { times: 10 }), ok(5)] },
  steps: [step(0, 'readSync', [3, rbuf, 0, 7, null])] });
add({ name: 'readSync: EAGAIN 11 times', script: { readSync: [err('EAGAIN', { times: 11 }), ok(5)] }, steps: [step(0, 'readSync', [3, rbuf, 0, 7, null])] });
add({ name: 'readSync: other errors', script: { readSync: [err('EAGAIN', { times: 2 }), err('EBADF'), ok(0)] },
  steps: [step(0, 'readSync', [3, rbuf, 0, 7, null]), step(1, 'readSync', [3, rbuf, 0, 7, 0])] });
add({ name: 'read and readSync: absent from the module stay absent', missing: ['read', 'readSync'],
  steps: [step(0, 'read', [3, rbuf, 0, 7, null, cb(1)]), step(0, 'readSync', [3, rbuf, 0, 7, null])] });

// ---------------------------------------------------------------- streams
add({ name: 'createReadStream: the open goes through the queue', script: { open: [err('EMFILE', { times: 2 }), ok(7)] },
  steps: [step(0, 'createReadStream', ['p'])] });
add({ name: 'createReadStream: options', script: { open: [ok(7), ok(8)] },
  steps: [step(0, 'createReadStream', ['p', opts({ flags: 'r+', mode: 384 })]), step(1, 'createReadStream', ['q', 'utf8'])] });
add({ name: 'createReadStream: an error destroys the stream (autoClose)', script: { open: [err('ENOENT')] }, steps: [step(0, 'createReadStream', ['p'])] });
add({ name: 'createReadStream: an error with autoClose false', script: { open: [err('EACCES')] },
  steps: [step(0, 'createReadStream', ['p', opts({ autoClose: false })])] });
add({ name: 'createReadStream: an fd skips the open', steps: [step(0, 'createReadStream', ['p', opts({ fd: 5 })])] });
add({ name: 'createReadStream: ENFILE until the cutoff', script: { open: [forever('ENFILE')] }, steps: [step(0, 'createReadStream', ['p'])] });
add({ name: 'createWriteStream: the open goes through the queue', script: { open: [err('ENFILE', { times: 3 }), ok(8)] },
  steps: [step(0, 'createWriteStream', ['p'])] });
add({ name: 'createWriteStream: an error always destroys', script: { open: [err('EROFS'), err('EISDIR')] },
  steps: [step(0, 'createWriteStream', ['p']), step(1, 'createWriteStream', ['q', opts({ autoClose: false, flags: 'a' })])] });
add({ name: 'createReadStream: the module has no ReadStream', missing: ['ReadStream', 'WriteStream'],
  steps: [step(0, 'createReadStream', ['p']), step(0, 'createWriteStream', ['p'])] });
add({ name: 'streams: a close resets the queue their opens wait on', script: { 'open:a': [err('EMFILE', { times: 8 }), ok(7)], 'open:b': [err('EMFILE', { times: 8 }), ok(8)] },
  steps: [step(0, 'createReadStream', ['a']), step(1, 'createWriteStream', ['b']), step(70, 'close', [4, cb(1)])] });

// ---------------------------------------------------------------- two loaded copies
add({ name: 'two copies over one module share the queue, each with its own timer', load: [0, 0],
  script: { 'open:a': [err('EMFILE', { times: 6 }), ok(10)], 'open:b': [err('EMFILE', { times: 6 }), ok(11)] },
  steps: [step(0, 'open', ['a', 'r', cb(1)]), step(0, 'open', ['b', 'r', cb(2)], 'g1')] });
add({ name: 'two copies over one module: only the first patches close', load: [0, 0],
  script: { 'open:a': [err('EMFILE', { times: 8 }), ok(10)], 'open:b': [err('EMFILE', { times: 8 }), ok(11)] },
  steps: [step(0, 'open', ['a', 'r', cb(1)]), step(1, 'open', ['b', 'r', cb(2)], 'g1'), step(60, 'close', [5, cb(3)], 'g1'), step(200, 'closeSync', [6], 'b0')] });
add({ name: 'two copies over two modules share the queue', load: [0, 1],
  bases: [{ script: { open: [err('EMFILE', { times: 8 }), ok(10)] } }, { script: { open: [err('ENFILE', { times: 8 }), ok(11)] } }],
  steps: [step(0, 'open', ['a', 'r', cb(1)]), step(0, 'open', ['b', 'r', cb(2)], 'g1'), step(60, 'close', [5, cb(3)], 'b1')] });
add({ name: 'two copies over two modules: the second module\'s closeSync', load: [0, 1],
  bases: [{ script: { open: [err('EMFILE', { times: 8 }), ok(10)] } }, { missing: ['appendFile'] }],
  steps: [step(0, 'open', ['a', 'r', cb(1)]), step(60, 'closeSync', [5], 'g1'), step(61, 'appendFile', ['x', 'y', cb(2)], 'g1')] });
add({ name: 'two copies: a timeout callback in one runs the other\'s closeSync', load: [0, 0],
  script: { 'open:a': [err('EMFILE'), err('EMFILE', { ms: 59990 })], 'open:b': [forever('EMFILE')] },
  steps: [step(0, 'open', ['a', 'r', cb(1, [{ on: 'g1', call: 'closeSync', args: [5] }])]), step(20, 'open', ['b', 'r', cb(2)])] });

// ---------------------------------------------------------------- re-entrancy and sync exceptions
add({ name: 'a timeout callback that calls closeSync', script: { 'open:a': [err('EMFILE'), err('EMFILE', { ms: 59990 })], 'open:b': [forever('EMFILE')] },
  steps: [step(0, 'open', ['a', 'r', cb(1, [{ call: 'closeSync', args: [5] }])]), step(20, 'open', ['b', 'r', cb(2)])] });
add({ name: 'a callback that opens again', script: { open: [err('EMFILE', { times: 2 }), ok(3), err('EMFILE', { times: 2 }), ok(4)] },
  steps: [step(0, 'open', ['a', 'r', cb(1, [{ call: 'open', args: ['b', 'r', cb(2)] }])])] });
add({ name: 'a callback that closes synchronously while others wait',
  script: { 'open:a': [err('EMFILE', { times: 2 }), ok(3)], 'open:b': [err('EMFILE', { times: 9 }), ok(4)] },
  steps: [step(0, 'open', ['b', 'r', cb(2)]), step(1, 'open', ['a', 'r', cb(1, [{ call: 'closeSync', args: [3] }])])] });
add({ name: 'a retried call that throws stops the retry timer',
  script: { 'open:a': [err('EMFILE'), { throw: 'ERR_BOOM' }], 'open:b': [err('EMFILE', { times: 3 }), ok(4)] },
  steps: [step(0, 'open', ['a', 'r', cb(1)]), step(0, 'open', ['b', 'r', cb(2)]), step(500, 'closeSync', [9])] });
add({ name: 'a first call that throws reaches the caller', script: { open: [{ throw: 'ERR_INVALID_ARG_VALUE' }] },
  steps: [step(0, 'open', ['a', 'bad', cb(1)])] });
add({ name: 'a step in a callback that throws is caught there', script: { open: [ok(3)] },
  steps: [step(0, 'open', ['a', 'r', cb(1, [{ call: 'appendFile', args: ['x', 'y', cb(2)] }])])], missing: ['appendFile'] });
const throwing = (n, then) => Object.assign(cb(n, then), { throws: 'ERR_CB' });
add({ name: 'a callback that throws: uncaught, from the completion', script: { open: [ok(3)], readdir: [ok(['b', 'a'])] },
  steps: [step(0, 'open', ['a', 'r', throwing(1)]), step(1, 'readdir', ['d', throwing(2)]), step(2, 'chmod', ['p', 420, throwing(3)])] });
add({ name: 'a timeout callback that throws stops the retry timer',
  script: { 'open:a': [err('EMFILE'), err('EMFILE', { ms: 59990 })], 'open:b': [err('EMFILE', { times: 605 }), ok(4)] },
  steps: [step(0, 'open', ['a', 'r', throwing(1)]), step(20, 'open', ['b', 'r', cb(2)]), step(61000, 'closeSync', [9])] });
add({ name: 'a no-op lchmod callback that throws', missing: ['lchmod'], script: { open: [ok(3)] },
  steps: [step(0, 'lchmod', ['p', 420, throwing(1)]), step(0, 'open', ['a', 'r', cb(2)])] });
add({ name: 'a read callback that throws after EAGAIN retries', script: { read: [err('EAGAIN', { times: 3 }), ok(1)] },
  steps: [step(0, 'read', [3, hex('00'), 0, 1, null, throwing(1)])] });
add({ name: 'a rename callback that throws after a retry', env: { platform: 'win32', uid: null },
  script: { rename: [err('EBUSY'), done()], stat: [err('ENOENT')] }, steps: [step(0, 'rename', ['a', 'b', throwing(1)])] });

// ---------------------------------------------------------------- the shape of the loaded module
add({ name: 'a module with only close and closeSync',
  missing: ['open', 'openSync', 'readFile', 'writeFile', 'appendFile', 'copyFile', 'readdir', 'read', 'readSync', 'rename', 'chown', 'fchown', 'lchown',
    'chownSync', 'fchownSync', 'lchownSync', 'chmod', 'fchmod', 'lchmod', 'chmodSync', 'fchmodSync', 'lchmodSync', 'stat', 'fstat', 'lstat',
    'statSync', 'fstatSync', 'lstatSync', 'futimes', 'futimesSync', 'lutimes', 'lutimesSync', 'ReadStream', 'WriteStream'],
  steps: [step(0, 'readFile', ['f', cb(1)]), step(0, 'open', ['a', 'r', cb(2)]), step(0, 'readdir', ['d', cb(3)]), step(0, 'writeFile', ['f', 'x', cb(4)]),
    step(1, 'close', [3, cb(5)])] });
add({ name: 'a module without close', missing: ['close', 'closeSync'], steps: [step(0, 'close', [3, cb(1)]), step(0, 'closeSync', [3])] });
add({ name: 'the full module on win32 with O_SYMLINK', env: { platform: 'win32', uid: null, osymlink: 2097152 }, steps: [] });
add({ name: 'the full module on darwin, root', env: { platform: 'darwin', uid: 0, osymlink: 2097152 }, missing: ['lutimes', 'lutimesSync', 'lchmod'], steps: [] });

const scheduler = [
  { name: 'ticks first, then tasks by due time and scheduling order', ops: [
    { timeout: 0, id: 't0' },
    { after: 0, id: 'io0', then: [{ tick: 'tick-in-io0' }, { timeout: 1, id: 't-from-io0' }, { after: 0, id: 'io0b' }] },
    { tick: 'tick', then: [{ tick: 'tick2', then: [{ tick: 'tick3' }] }] },
    { after: 1, id: 'io1' },
    { timeout: 1, id: 't1' },
    { after: 2, id: 'io2', then: [{ timeout: 0, id: 't-from-io2' }, { tick: 'tick-in-io2' }, { after: 0, id: 'io2b' }] },
  ] },
  { name: 'setTimeout clamps delays outside 1..2^31-1 to 1 ms', ops: [
    { timeout: -5, id: 'neg' }, { timeout: 0, id: 'zero' }, { timeout: 1, id: 'one' }, { timeout: 2147483648, id: 'over' },
    { timeout: 2147483647, id: 'max' }, { after: 0, id: 'io' }, { after: -3, id: 'io-neg' },
  ] },
  { name: 'clearTimeout', ops: [
    { timeout: 5, id: 'a' }, { after: 1, id: 'clear-a', then: [{ clear: 'a' }] }, { timeout: 3, id: 'self', then: [{ clear: 'self' }] },
    { timeout: 2, id: 'b' }, { timeout: 4, id: 'clear-b-late', then: [{ clear: 'b' }] }, { clear: 'missing' },
    { timeout: 6, id: 'c', then: [{ timeout: 1, id: 'd' }] }, { after: 6, id: 'clear-d-same-ms', then: [{ clear: 'd' }] },
  ] },
  { name: 'equal due times from different delays run in scheduling order', ops: [
    { timeout: 10, id: 'a' }, { after: 5, id: 'b', then: [{ timeout: 5, id: 'c' }, { after: 5, id: 'd' }] }, { after: 10, id: 'e' },
  ] },
  { name: 'uncaught exceptions', ops: [
    { after: 0, id: 'a', panic: true, then: [{ tick: 'not-reached' }] }, { tick: 't', panic: true }, { tick: 'u' }, { after: 1, id: 'b' },
    { timeout: 1, id: 'c', panic: true },
  ] },
  { name: 'a clock that starts at a real time', start: 1700000000000, ops: [{ timeout: 0, id: 'a' }, { after: 7, id: 'b' }] },
];

process.stdout.write(JSON.stringify({ scheduler, cases }, null, 1) + '\n');
