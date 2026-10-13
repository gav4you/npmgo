// Records Node's real (ICU) getStringWidth for the width parity cases.
// Run: node --expose-internals parity/oracle_width.cjs parity/width/cases.json
'use strict';
const { getStringWidth, isZeroWidthCodePoint } = require('internal/util/inspect');
const c = require(require('path').resolve(process.argv[2]));
const strs = c.strings.map(s => s.split('{ESC}').join('\u001b'));
console.log(JSON.stringify({
  width: strs.map(s => getStringWidth(s)),
  widthKeepControl: strs.map(s => getStringWidth(s, false)),
  zeroWidth: c.codepoints.map(cp => isZeroWidthCodePoint(cp)),
}));
