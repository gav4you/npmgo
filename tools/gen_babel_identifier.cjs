// Generates pkgs/babel/helper-validator-identifier/data.go from the ORIGINAL @babel/helper-validator-identifier
// 7.28.5: the two character-class strings and the two astral code tables of lib/identifier.js, copied as data.
//
// Run from the repository root: node tools/gen_babel_identifier.cjs <root>
// <root> holds a node_modules layout of the original tarball (see parity/oracle_babel_types.cjs). The script reads
// lib/identifier.js as text (it runs nothing from it but the module itself, to check the data it extracted), checks
// that the code around the data is the code the Go port translates (a SHA-256 of identifier.js and keyword.js with
// the data cut out), and checks the extracted classes against the loaded module for every BMP code unit.
'use strict';
const fs = require('fs');
const path = require('path');
const crypto = require('crypto');

const root = path.resolve(process.argv[2]);
const pkg = path.join(root, 'node_modules', '@babel', 'helper-validator-identifier');
const version = require(path.join(pkg, 'package.json')).version;
if (version !== '7.28.5') throw new Error('expected @babel/helper-validator-identifier 7.28.5, found ' + version);
const src = fs.readFileSync(path.join(pkg, 'lib', 'identifier.js'), 'utf8');
const keywordSrc = fs.readFileSync(path.join(pkg, 'lib', 'keyword.js'), 'utf8');

// decodeString decodes a double-quoted JavaScript string literal that uses only \xHH and \uHHHH escapes and
// printable ASCII, into its UTF-16 code units.
function decodeString(lit) {
  if (lit[0] !== '"' || lit[lit.length - 1] !== '"') throw new Error('not a string literal');
  const units = [];
  for (let i = 1; i < lit.length - 1; i++) {
    const c = lit[i];
    if (c === '\\') {
      const kind = lit[i + 1];
      const len = kind === 'x' ? 2 : kind === 'u' ? 4 : 0;
      if (!len) throw new Error('unexpected escape \\' + kind);
      const hex = lit.slice(i + 2, i + 2 + len);
      if (!/^[0-9a-fA-F]+$/.test(hex) || hex.length !== len) throw new Error('bad escape ' + hex);
      units.push(parseInt(hex, 16));
      i += 1 + len;
    } else {
      const code = c.charCodeAt(0);
      if (code < 0x20 || code > 0x7e || c === '"') throw new Error('unexpected character ' + code);
      units.push(code);
    }
  }
  return units;
}

const data = {};
let skeleton = src;
const take = (name, re, decode) => {
  const m = re.exec(src);
  if (!m) throw new Error(name + ' not found');
  data[name] = decode(m[1]);
  skeleton = skeleton.replace(m[1], '<' + name + '>');
};
take('nonASCIIidentifierStartChars', /^let nonASCIIidentifierStartChars = ("[^"\n]*");$/m, decodeString);
take('nonASCIIidentifierChars', /^let nonASCIIidentifierChars = ("[^"\n]*");$/m, decodeString);
const numbers = s => s.split(', ').map(x => {
  if (!/^\d+$/.test(x)) throw new Error('bad number ' + x);
  return Number(x);
});
take('astralIdentifierStartCodes', /^const astralIdentifierStartCodes = \[([\d, ]*)\];$/m, numbers);
take('astralIdentifierCodes', /^const astralIdentifierCodes = \[([\d, ]*)\];$/m, numbers);

// The code the Go port translates: identifier.js without its data, and keyword.js.
const hash = crypto.createHash('sha256').update(skeleton).update('\0').update(keywordSrc).digest('hex');
const expectedHash = 'c5574d32e3acf8c7e68b32a3368cf3ac00d03d17a08e8d9c7716c208abb4c578';
if (hash !== expectedHash && !process.env.BABEL_IDENTIFIER_PRINT_HASH) {
  throw new Error('identifier.js / keyword.js changed (hash ' + hash + '): review the Go port before regenerating');
}
if (process.env.BABEL_IDENTIFIER_PRINT_HASH) console.error('hash', hash);

// Check the classes the way RegExp reads them against the module, for every BMP code unit from 0xaa.
function classRanges(units) {
  const ranges = [];
  for (let i = 0; i < units.length; i++) {
    if (i + 2 < units.length && units[i + 1] === 0x2d) {
      if (units[i] > units[i + 2]) throw new Error('reversed range');
      ranges.push([units[i], units[i + 2]]);
      i += 2;
    } else ranges.push([units[i], units[i]]);
  }
  return ranges;
}
const startRanges = classRanges(data.nonASCIIidentifierStartChars);
const charRanges = classRanges(data.nonASCIIidentifierStartChars.concat(data.nonASCIIidentifierChars));
const inRanges = (rs, c) => rs.some(([lo, hi]) => lo <= c && c <= hi);
const mod = require(pkg);
for (let c = 0xaa; c <= 0xffff; c++) {
  if (inRanges(startRanges, c) !== mod.isIdentifierStart(c)) throw new Error('start class mismatch at ' + c);
  if (inRanges(charRanges, c) !== mod.isIdentifierChar(c)) throw new Error('char class mismatch at ' + c);
}
for (const units of [data.nonASCIIidentifierStartChars, data.nonASCIIidentifierChars]) {
  if (units.some(u => u >= 0xd800 && u <= 0xdfff)) throw new Error('a class holds a surrogate code unit');
}

const goString = units => {
  let s = '"';
  for (const u of units) {
    if (u >= 0x20 && u <= 0x7e && u !== 0x22 && u !== 0x5c) s += String.fromCharCode(u);
    else s += '\\u' + u.toString(16).padStart(4, '0');
  }
  return s + '"';
};
const goInts = (nums, perLine) => {
  const lines = [];
  for (let i = 0; i < nums.length; i += perLine) lines.push('\t' + nums.slice(i, i + perLine).join(', ') + ',');
  return lines.join('\n');
};

const out = `// Code generated by tools/gen_babel_identifier.cjs from @babel/helper-validator-identifier ${version}; DO NOT EDIT.

// Copyright (c) 2014-present Sebastian McKenzie and other contributors. MIT License (see LICENSE).

package helpervalidatoridentifier

// nonASCIIidentifierStartChars and nonASCIIidentifierChars are identifier.js's strings of the same names, written as
// the characters of a RegExp character class ("a-b" is a range).
const (
	nonASCIIidentifierStartChars = ${goString(data.nonASCIIidentifierStartChars)}
	nonASCIIidentifierChars      = ${goString(data.nonASCIIidentifierChars)}
)

// astralIdentifierStartCodes and astralIdentifierCodes are identifier.js's tables of the same names: alternating
// skips and lengths of runs of code points from 0x10000.
var astralIdentifierStartCodes = []float64{
${goInts(data.astralIdentifierStartCodes, 20)}
}

var astralIdentifierCodes = []float64{
${goInts(data.astralIdentifierCodes, 20)}
}
`;
fs.writeFileSync(path.join('pkgs', 'babel', 'helper-validator-identifier', 'data.go'), out);
console.error('wrote data.go:', startRanges.length, 'start ranges,', charRanges.length, 'char ranges,',
  data.astralIdentifierStartCodes.length, '+', data.astralIdentifierCodes.length, 'astral codes');
