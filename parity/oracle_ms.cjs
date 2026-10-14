// Runs the ORIGINAL npm ms against parity cases; prints expected outputs as JSON.
// Run: node parity/oracle_ms.cjs <ms package dir> <absolute path to parity/ms/cases.json> > parity/ms/expected.json
const ms = require(process.argv[2]);
const c = require(process.argv[3]);
const long = 'x'.repeat(101) + 'ms';
const parse = [...c.parse, long].map(v => { const r = ms(v); return r === undefined ? null : r; });
const short = c.num.map(n => ms(n)), lng = c.num.map(n => ms(n, { long: true }));
let err = []; for (const v of [NaN, Infinity]) { try { ms(v) } catch (e) { err.push(e.message) } }
console.log(JSON.stringify({ parseIn: [...c.parse, long], parse, short, long: lng, err }));
