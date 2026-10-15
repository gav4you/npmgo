// Mutation check of the @babel/types parity harness: applies each deliberate change below to the Go ports (generated
// and hand-written code, the two helpers, and the runtime helpers they run on), runs the parity tests of the package
// it targets (for pkgs/babel/types both TestParity and TestParityBreaking8, which re-runs the test binary with
// BABEL_TYPES_8_BREAKING set), and restores the file with `git checkout -- <file>`. Every mutation should make the
// tests fail ("caught").
//
// Run from the repository root, on a clean tree (it refuses otherwise, since it restores files from git):
//   node tools/mutate_babel_types.cjs
// It exits with status 1 when a mutation survives, does not apply or does not build.
'use strict';
const fs = require('fs');
const path = require('path');
const { spawnSync } = require('child_process');

const root = process.cwd();
const status = spawnSync('git', ['status', '--porcelain'], { cwd: root, encoding: 'utf8' });
if (status.status !== 0) throw new Error('git status failed: ' + status.stderr);
if (status.stdout.trim() !== '') throw new Error('the working tree is not clean: commit or set aside your changes first');

const T = 'pkgs/babel/types', S = 'pkgs/babel/helper-string-parser', I = 'pkgs/babel/helper-validator-identifier';
const types = ['./' + T + '/', 'TestParity'];
const strp = ['./' + S + '/', 'TestParity'];
const ident = ['./' + I + '/', 'TestParity'];
const jsv = ['./internal/jsvalue/', 'TestJSONCircular'];
// [description, file, text to replace (its first occurrence), replacement, [package, test pattern]]. An argument, if
// given, runs only the mutations whose description contains it.
const only = process.argv[2];
const M = [
  // Generated code.
  ['builders: arrayExpression\'s default [] is null', T + '/builders_gen.go', 'elements_ = jsvalue.NewArray()', 'elements_ = nil', types],
  ['builders: bindExpression does not check object\'s parent validation', T + '/builders_gen.go',
    'defs := getProp(NODE_FIELDS, "BindExpression")\n\tvalidateInternal(getProp(defs, "object"), node, "object", object_, true)',
    'defs := getProp(NODE_FIELDS, "BindExpression")\n\tvalidateInternal(getProp(defs, "object"), node, "object", object_, false)', types],
  ['validators: isExpression misses ArrayExpression', T + '/validators_gen.go',
    'func IsExpression(node any, opts ...any) bool {\n\tif !jsvalue.Truthy(node) {\n\t\treturn false\n\t}\n\ttyp, ok := stringProp(node, "type")\n\tif !ok {\n\t\treturn false\n\t}\n\tswitch typ {\n\tcase "ArrayExpression", ',
    'func IsExpression(node any, opts ...any) bool {\n\tif !jsvalue.Truthy(node) {\n\t\treturn false\n\t}\n\ttyp, ok := stringProp(node, "type")\n\tif !ok {\n\t\treturn false\n\t}\n\tswitch typ {\n\tcase ', types],
  ['validators: isStatement takes no Declaration placeholder', T + '/validators_gen.go', 'expected == "Statement" || expected == "Declaration" || expected == "BlockStatement"', 'expected == "Statement" || expected == "BlockStatement"', types],
  ['validators: isIdentifier checks another type', T + '/validators_gen.go', 'typ != "Identifier" {', 'typ != "Identifiers" {', types],
  ['asserts: assertIdentifier passes null options', T + '/asserts_gen.go', 'assertType("Identifier", node, optArg(opts))', 'assertType("Identifier", node, nil)', types],
  ['definitions: an alias of BinaryExpression misspelt', T + '/definitions_gen.go', '"aliases", jsvalue.NewArray("Binary", "Expression")', '"aliases", jsvalue.NewArray("Binar", "Expression")', types],
  ['constants: EXPRESSION_TYPES is a copy', T + '/constants_gen.go', 'EXPRESSION_TYPES = flippedAliasArray("Expression")', 'EXPRESSION_TYPES = jsvalue.NewArray(flippedAliasArray("Expression").Items()...)', types],
  // Definitions.
  ['definer: aliases added at the end', T + '/definer.go', 'arr.Unshift(additional...)', 'arr.Push(additional...)', types],
  ['definer: inherited fields without their deprecated key', T + '/definer.go', '\t\t\t\t\t"deprecated", getProp(field, "deprecated"),\n', '', types],
  ['definer: builder keys copy the visitor keys', T + '/definer.go', 'builder = getProp(opts, "visitor")', 'builder = jsvalue.NewArray(items(getProp(opts, "visitor"))...)', types],
  ['definer: Pattern placeholders alias LVal first', T + '/definer.go', '"Pattern", jsvalue.NewArray("PatternLike", "LVal"),', '"Pattern", jsvalue.NewArray("LVal", "PatternLike"),', types],
  ['definer: a deprecated alias gets a copy of the list', T + '/definer.go',
    'd.FLIPPED_ALIAS_KEYS.Set(deprecatedAlias, tableGet(d.FLIPPED_ALIAS_KEYS, d.DEPRECATED_ALIASES.Get(deprecatedAlias)))',
    'd.FLIPPED_ALIAS_KEYS.Set(deprecatedAlias, jsvalue.NewArray(items(tableGet(d.FLIPPED_ALIAS_KEYS, d.DEPRECATED_ALIASES.Get(deprecatedAlias)))...))', types],
  ['definer: assertNodeType types are not expanded', T + '/definer.go', '\t\t\tif truthy(aliases) {\n\t\t\t\tfor _, a := range items(aliases) {', '\t\t\tif false {\n\t\t\t\tfor _, a := range items(aliases) {', types],
  // Validators.
  ['validator: assertNodeType ignores placeholders', T + '/validator.go', 'if strictEquals(valType, "Placeholder") {', 'if false {', types],
  ['validator: assertShape reports its first error only', T + '/validator.go', '\t\t\tif failed {\n\t\t\t\terrs = append(errs, msg)', '\t\t\tif failed && len(errs) == 0 {\n\t\t\t\terrs = append(errs, msg)', types],
  ['validator: assertEach without parent validations (BABEL_TYPES_8_BREAKING)', T + '/validator.go', '\tif d.breaking {\n\t\tchildValidator = validateChild', '\tif false {\n\t\tchildValidator = validateChild', types],
  ['validator: the optional chain walks objects of calls (BABEL_TYPES_8_BREAKING)', T + '/validator.go', 'current = getProp(current, "callee")', 'current = getProp(current, "object")', types],
  ['customs: RestElement may be one before last (BABEL_TYPES_8_BREAKING)', T + '/custom.go', 'if length > jsvalue.StringToNumber(index)+1 {', 'if length > jsvalue.StringToNumber(index)+2 {', types],
  ['customs: Identifier "this" is no exception (BABEL_TYPES_8_BREAKING)', T + '/custom.go', ' && !strictEquals(name, "this")', '', types],
  ['customs: TemplateElement cooked never null', T + '/custom.go', '\t\tif res.FirstInvalidLoc != nil {\n\t\t\tcooked = nil\n\t\t}', '', types],
  ['customs: BinaryExpression.left never takes a PrivateName', T + '/custom.go', 'if strictEquals(getProp(node, "operator"), "in") {', 'if false {', types],
  // The JavaScript object model.
  ['jsobject: tables inherit nothing from Object.prototype', T + '/jsobject.go', '\tif v, ok := table.Lookup(k); ok {\n\t\treturn v\n\t}\n\tif v, ok := inheritedProp(k); ok {\n\t\treturn v\n\t}', '\tif v, ok := table.Lookup(k); ok {\n\t\treturn v\n\t}', types],
  ['jsmap: BigInts by identity, not value', T + '/jsmap.go', 'return "bigint:" + x.String()', 'return x', types],
  ['jsmap: NaN is not NaN', T + '/jsmap.go', 'return nanKey{}', 'return f', types],
  // Utilities.
  ['is: a placeholder matches even with options', T + '/is.go', 'if !truthy(optArg(opts)) && strictEquals(getProp(node, "type"), "Placeholder")', 'if strictEquals(getProp(node, "type"), "Placeholder")', types],
  ['isType: no ALIAS_KEYS check', T + '/is.go', 'if truthy(tableGet(ALIAS_KEYS, targetType)) {', 'if false {', types],
  ['isReferenced: an ObjectProperty value in a pattern is referenced', T + '/is.go', 'return !strictEquals(gpType, "ObjectPattern")', 'return !strictEquals(gpType, "ObjectPattern") || true', types],
  ['matchesPattern: allowPartial ignored', T + '/is.go', 'if !allowPartial && len(nodes) > len(parts) {', 'if len(nodes) > len(parts) {', types],
  ['cloneNode: optional copied whatever its type', T + '/clone.go', 'if b, ok := getProp(node, "optional").(bool); ok {\n\t\t\t\tnewNode.Set("optional", b)', 'if b := getProp(node, "optional"); true {\n\t\t\t\tnewNode.Set("optional", b)', types],
  ['cloneNode: comments are not cached', T + '/clone.go', 'if cached, _ := commentsCache.get(comment); truthy(cached) {', 'if cached, _ := commentsCache.get(comment); false && truthy(cached) {', types],
  ['addComments: leading comments after the old ones', T + '/comments.go', 'setProp(node, key, concat(comments, "comments", existing))', 'setProp(node, key, concat(existing, "comments", comments))', types],
  ['toIdentifier: leading digits kept', T + '/converters.go', "name[i] == '-' || name[i] >= '0' && name[i] <= '9'", "name[i] == '-'", types],
  ['toKeyAlias: the counter resets one later', T + '/converters.go', 'if toKeyAliasState.uid >= maxSafeInteger {', 'if toKeyAliasState.uid > maxSafeInteger {', types],
  ['valueToNode: an own __proto__ key is not computed', T + '/converters.go', '\t\t\t\t\tcomputed = true\n', '', types],
  ['getBindingIdentifiers: last in, first out', T + '/retrievers.go', 'id := search.Shift()', 'id := search.Pop()', types],
  ['removeTypeDuplicates: no base annotations', T + '/modifications.go', 'for _, baseType := range bases.vals {', 'for _, baseType := range generics.vals {', types],
  ['traverseFast: skip ignored', T + '/traverse.go', '\t\t\tcase TraverseFastSkip:\n\t\t\t\treturn false', '\t\t\tcase TraverseFastSkip:', types],
  ['deprecation: nothing is internal', T + '/deprecation.go', 'return strings.HasPrefix(short[1].Function, babelPackages), strings.Join(lines, "\\n")', 'return false, strings.Join(lines, "\\n")', types],
  ['deprecation: an internal caller is not remembered', T + '/deprecation.go', '\tif internal {\n\t\treturn\n\t}', '\tif internal {\n\t\tdelete(warnings.seen, cacheKey)\n\t\treturn\n\t}', types],
  ['deprecation: the stack one frame off', T + '/deprecation.go', 'const skip = 3', 'const skip = 2', types],
  // jsvalue.
  ['jsvalue: unshift appends', 'internal/jsvalue/array.go', 'a.items = append(items, a.items...)', 'a.items = append(a.items, items...)', types],
  ['jsvalue: Object.assign copies no symbols', 'internal/jsvalue/object.go', '\t\t\tfor _, e := range src.syms {\n\t\t\t\to.SetSymbol(e.key, e.val)\n\t\t\t}\n', '', types],
  ['jsvalue: circular message shows three prefix lines', 'internal/jsvalue/jsvalue.go', 'const prefixCount, postfixCount = 2, 1', 'const prefixCount, postfixCount = 3, 1', jsv],
  // helper-string-parser.
  ['string parser: \\400 not trimmed to \\40', S + '/stringparser.go', 'if octal > 255 {', 'if octal > 256 {', strp],
  ['string parser: CR LF not folded in templates', S + '/stringparser.go', '\t\t\t\tif ch == 13 && in.at(pos) == 10 {\n\t\t\t\t\tpos++\n\t\t\t\t}\n\t\t\t\tcurLine++', '\t\t\t\tcurLine++', strp],
  ['string parser: a second invalid escape adds nothing', S + '/stringparser.go', 'out = append(out, nullText...) // out += null', '_ = nullText', strp],
  ['string parser: a separator after another is allowed', S + '/stringparser.go', ' || forbiddenSiblings[prev] || ', ' || forbiddenSiblings[prev] && false || ', strp],
  // helper-validator-identifier.
  ['identifier: one range of the start class shorter', I + '/data.go', '\\u00c0-\\u00d6', '\\u00c0-\\u00d5', ident],
  ['identifier: astral runs end one early', I + '/identifier.go', 'if pos > code {', 'if pos >= code {', ident],
  ['identifier: surrogate pairs lose their low half', I + '/identifier.go', 'cp = 0x10000 + ((cp & 0x3ff) << 10) + (trail & 0x3ff)', 'cp = 0x10000 + ((cp & 0x3ff) << 10)', ident],
  ['identifier: enum is not reserved', I + '/keyword.go', 'return inModule && word == "await" || word == "enum"', 'return inModule && word == "await"', ident],
];

let failures = 0;
let caught = 0;
for (const [name, file, from, to, [pkg, pattern]] of M.filter(m => !only || m[0].includes(only))) {
  const full = path.join(root, file);
  const src = fs.readFileSync(full, 'utf8').replace(/\r\n/g, '\n');
  const at = src.indexOf(from);
  let verdict;
  if (at < 0) {
    verdict = 'NOT APPLIED (text not found)';
  } else {
    fs.writeFileSync(full, src.slice(0, at) + to + src.slice(at + from.length));
    try {
      const r = spawnSync('go', ['test', pkg, '-run', pattern, '-count=1'], { cwd: root, encoding: 'utf8' });
      const out = r.stdout + r.stderr;
      if (/\[build failed\]|\[setup failed\]|cannot use|undefined:|declared and not used/.test(out)) verdict = 'BUILD ERROR';
      else if (r.status === 0) verdict = 'SURVIVED';
      else verdict = `caught (${(out.match(/^\s+\S+_test\.go:\d+: /gm) || []).length} failure reports)`;
    } finally {
      spawnSync('git', ['checkout', '--', file], { cwd: root });
    }
  }
  if (verdict.startsWith('caught')) caught++;
  else failures++;
  console.log(`${verdict.padEnd(32)} ${name}`);
}
const after = spawnSync('git', ['status', '--porcelain'], { cwd: root, encoding: 'utf8' });
console.log('working tree after: ' + (after.stdout.trim() || 'clean'));
console.log(`${caught} of ${caught + failures} mutations caught`);
process.exitCode = failures ? 1 : 0;
