// Records String.prototype.toUpperCase and toLowerCase of every code point, as Node's ICU computes them: the code
// points whose mapping is not the identity, each with the code points of its upper and lower forms (null where the
// form is the code point itself). internal/jsvalue's TestCaseMappingParity checks ToUpperCase and ToLowerCase of
// every code point against the recording, and tools/gen_casemap.go derives from it the mappings golang.org/x/text
// lacks (internal/jsvalue/casemap_supplement.go).
//
// Run: node tools/gen_casemap.cjs > parity/jsvalue/casemap.json
'use strict';
const cps = s => Array.from(s, c => c.codePointAt(0));
const entries = [];
for (let cp = 0; cp <= 0x10ffff; cp++) {
  if (cp >= 0xd800 && cp <= 0xdfff) continue;
  const ch = String.fromCodePoint(cp);
  const upper = ch.toUpperCase(), lower = ch.toLowerCase();
  if (upper === ch && lower === ch) continue;
  entries.push(`[${cp},${upper === ch ? 'null' : JSON.stringify(cps(upper))},${lower === ch ? 'null' : JSON.stringify(cps(lower))}]`);
}
process.stdout.write(`{"node":${JSON.stringify(process.version)},"icu":${JSON.stringify(process.versions.icu)},` +
  `"unicode":${JSON.stringify(process.versions.unicode)},\n"mappings":[\n${entries.join(',\n')}\n]}\n`);
