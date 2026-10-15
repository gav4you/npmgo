// Generates the regular parts of pkgs/babel/types from the ORIGINAL @babel/types 7.29.0:
//   definitions_gen.go  every defineType call of lib/definitions/*.js, with its options as they were passed (before
//                       defineType mutated them, object identity kept), in load order, for the default environment and
//                       for BABEL_TYPES_8_BREAKING (each recorded in a child process with that environment);
//   builders_gen.go     lib/builders/generated/lowercase.js and uppercase.js;
//   validators_gen.go   lib/validators/generated/index.js;
//   asserts_gen.go      lib/asserts/generated/index.js;
//   constants_gen.go    lib/constants/generated/index.js;
//   exports_gen.go      the names of the main entry point's exports, in order, and their Go values.
//
// Run from the repository root: node tools/gen_babel_types.cjs <root>
// <root> holds a node_modules layout of the original tarballs (see parity/oracle_babel_types.cjs). Treat them as
// untrusted data; this script loads the module (as the oracle does) and reads the generated files as text. It fails
// on anything it does not recognise: a validator closure that is neither made by one of the factories of
// definitions/utils.js (recognised by the source text of a fresh instance of each factory) nor listed in CUSTOM below
// with its exact source text, a builder, validator or assert body of an unknown shape, or a change to the files the
// Go port translates by hand (HAND_TRANSLATED_SHA256).
'use strict';
const fs = require('fs');
const path = require('path');
const Module = require('module');
const crypto = require('crypto');
const { spawnSync } = require('child_process');

const argv = process.argv.slice(2);
const childMode = argv[0] === '--definitions';
const root = path.resolve(childMode ? argv[1] : argv[0]);
const typesDir = path.join(root, 'node_modules', '@babel', 'types');
const lib = path.join(typesDir, 'lib');
const VERSION = '7.29.0';
const OUT = path.join('pkgs', 'babel', 'types');

const fail = msg => { throw new Error(msg); };

// ---------------------------------------------------------------------------------------------- Go literals
function goString(s) {
  let out = '"';
  for (const ch of s) {
    const c = ch.codePointAt(0);
    if (ch === '"' || ch === '\\') out += '\\' + ch;
    else if (c >= 0x20 && c < 0x7f) out += ch;
    else if (c < 0x10000) out += '\\u' + c.toString(16).padStart(4, '0');
    else out += '\\U' + c.toString(16).padStart(8, '0');
  }
  return out + '"';
}
const capitalize = s => s[0].toUpperCase() + s.slice(1);

// ---------------------------------------------------------------------------------------------- definitions
// CUSTOM lists the validator closures that no factory makes: where each is first met (its path through the options
// passed to defineType), its exact source text, and the method of the Go definer that makes its translation.
const CUSTOM = [
  // Both environments.
  ['BinaryExpression.fields.left.validate', 'customBinaryExpressionLeft', `function (node, key, val) {
          const validator = node.operator === "in" ? inOp : expression;
          validator(node, key, val);
        }`],
  ['NumericLiteral.fields.value.validate.chainOf[1]', 'customNumericLiteralValue', `function (node, key, val) {
        if (1 / val < 0 || !Number.isFinite(val)) {
          const error = new Error("NumericLiterals must be non-negative finite numbers. " + \`You can use t.valueToNode(\${val}) instead.\`);
        }
      }`],
  ['MemberExpression.fields.property.validate', 'customMemberExpressionProperty', `function (node, key, val) {
          const validator = node.computed ? computed : normal;
          validator(node, key, val);
        }`],
  ['ObjectMethod.fields.key.validate', 'customObjectMethodKey', `function (node, key, val) {
          const validator = node.computed ? computed : normal;
          validator(node, key, val);
        }`],
  ['ObjectProperty.fields.key.validate', 'customObjectPropertyKey', `function (node, key, val) {
          const validator = node.computed ? computed : normal;
          validator(node, key, val);
        }`],
  ['ClassMethod.fields.key.validate.chainOf[0]', 'customClassMethodOrPropertyKey', `function (node, key, val) {
        const validator = node.computed ? computed : normal;
        validator(node, key, val);
      }`],
  ['ClassProperty.fields.key.validate.chainOf[0]', 'customClassMethodOrPropertyKey', `function (node, key, val) {
        const validator = node.computed ? computed : normal;
        validator(node, key, val);
      }`],
  ['TSDeclareMethod.fields.key.validate.chainOf[0]', 'customClassMethodOrPropertyKey', `function (node, key, val) {
        const validator = node.computed ? computed : normal;
        validator(node, key, val);
      }`],
  ['ClassAccessorProperty.fields.key.validate.chainOf[0]', 'customClassAccessorPropertyKey', `function (node, key, val) {
          const validator = node.computed ? computed : normal;
          validator(node, key, val);
        }`],
  ['TemplateElement.fields.value.validate.chainOf[1]', 'customTemplateElementCooked', `function templateElementCookedValidator(node) {
        const raw = node.value.raw;
        let unterminatedCalled = false;
        const error = () => {
          throw new Error("Internal @babel/types error.");
        };
        const {
          str,
          firstInvalidLoc
        } = (0, _helperStringParser.readStringContents)("template", raw, 0, 0, 0, {
          unterminated() {
            unterminatedCalled = true;
          },
          strictNumericEscape: error,
          invalidEscapeSequence: error,
          numericSeparatorInEscapeSequence: error,
          unexpectedNumericSeparator: error,
          invalidDigit: error,
          invalidCodePoint: error
        });
        if (!unterminatedCalled) throw new Error("Invalid raw");
        node.value.cooked = firstInvalidLoc ? null : str;
      }`],
  ['TemplateLiteral.fields.expressions.validate.chainOf[2]', 'customTemplateLiteralExpressions', `function (node, key, val) {
        if (node.quasis.length !== val.length + 1) {
          throw new TypeError(\`Number of \${node.type} quasis should be exactly one more than the number of expressions.\\nExpected \${val.length + 1} quasis but got \${node.quasis.length}\`);
        }
      }`],
  ['OptionalMemberExpression.fields.property.validate', 'customOptionalMemberExpressionProperty', `function (node, key, val) {
          const validator = node.computed ? computed : normal;
          validator(node, key, val);
        }`],
  ['TSTemplateLiteralType.fields.types.validate.chainOf[2]', 'customTSTemplateLiteralTypeTypes', `function (node, key, val) {
        if (node.quasis.length !== val.length + 1) {
          throw new TypeError(\`Number of \${node.type} quasis should be exactly one more than the number of types.\\nExpected \${val.length + 1} quasis but got \${node.quasis.length}\`);
        }
      }`],
  ['TSLiteralType.fields.literal.validate', 'customTSLiteralTypeLiteral', `function validator(parent, key, node) {
          if ((0, _is.default)("UnaryExpression", node)) {
            unaryOperator(node, "operator", node.operator);
            unaryExpression(node, "argument", node.argument);
          } else {
            literal(parent, key, node);
          }
        }`],
  ['File.fields.tokens.validate.each', 'customFileTokensEach', '() => {}'],
  // The default environment only.
  ['File.fields.comments.validate', 'customFileComments', '() => {}'],
  ['BindExpression.fields.object.validate', 'customBindExpressionNoop', '() => {}'],
  ['BindExpression.fields.callee.validate', 'customBindExpressionNoop', '() => {}'],
  // BABEL_TYPES_8_BREAKING only.
  ['AssignmentExpression.fields.operator.validate', 'customAssignmentExpressionOperator', `function (node, key, val) {
          const validator = (0, _is.default)("Pattern", node.left) ? pattern : identifier;
          validator(node, key, val);
        }`],
  ['Identifier.fields.name.validate.chainOf[1]', 'customIdentifierName', `function (node, key, val) {
        if (!(0, _isValidIdentifier.default)(val, false)) {
          throw new TypeError(\`"\${val}" is not a valid identifier name\`);
        }
      }`],
  ['RegExpLiteral.fields.flags.validate.chainOf[1]', 'customRegExpLiteralFlags', `function (node, key, val) {
        const invalid = /[^dgimsuvy]/.exec(val);
        if (invalid) {
          throw new TypeError(\`"\${invalid[0]}" is not a valid RegExp flag\`);
        }
      }`],
  ['ObjectProperty.fields.shorthand.validate.chainOf[1]', 'customObjectPropertyShorthand', `function (node, key, shorthand) {
        if (!shorthand) return;
        if (node.computed) {
          throw new TypeError("Property shorthand of ObjectProperty cannot be true if computed is true");
        }
        if (!(0, _is.default)("Identifier", node.key)) {
          throw new TypeError("Property shorthand of ObjectProperty cannot be true if key is not an Identifier");
        }
      }`],
  ['TryStatement.fields.block.validate.chainOf[1]', 'customTryStatementBlock', `function (node) {
        if (!node.handler && !node.finalizer) {
          throw new TypeError("TryStatement expects either a handler or finalizer, or both");
        }
      }`],
  ['ExportNamedDeclaration.fields.declaration.validate.chainOf[1]', 'customExportNamedDeclarationDeclaration', `function (node, key, val) {
        if (val && node.specifiers.length) {
          throw new TypeError("Only declaration or specifiers is allowed on ExportNamedDeclaration");
        }
        if (val && node.source) {
          throw new TypeError("Cannot export a declaration from a source");
        }
      }`],
  ['ExportNamedDeclaration.fields.specifiers.validate.chainOf[1].each', 'customExportNamedDeclarationSpecifier', `function (node, key, val) {
          const validator = node.source ? sourced : sourceless;
          validator(node, key, val);
        }`],
  ['ForOfStatement.fields.left.validate', 'customForOfStatementLeft', `function (node, key, val) {
          if ((0, _is.default)("VariableDeclaration", val)) {
            declaration(node, key, val);
          } else {
            lval(node, key, val);
          }
        }`],
  ['MetaProperty.fields.meta.validate.chainOf[1]', 'customMetaPropertyMeta', `function (node, key, val) {
        let property;
        switch (val.name) {
          case "function":
            property = "sent";
            break;
          case "new":
            property = "target";
            break;
          case "import":
            property = "meta";
            break;
        }
        if (!(0, _is.default)("Identifier", node.property, {
          name: property
        })) {
          throw new TypeError("Unrecognised MetaProperty");
        }
      }`],
  ['YieldExpression.fields.delegate.validate.chainOf[1]', 'customYieldExpressionDelegate', `function (node, key, val) {
        if (val && !node.argument) {
          throw new TypeError("Property delegate of YieldExpression cannot be true if there is no argument");
        }
      }`],
  ['FunctionDeclaration.validate', 'customFunctionDeclarationParent', `function (parent, key, node) {
      if (!(0, _is.default)("ExportDefaultDeclaration", parent)) {
        identifier(node, "id", node.id);
      }
    }`],
  ['ClassDeclaration.validate', 'customClassDeclarationParent', `function (parent, key, node) {
      if (!(0, _is.default)("ExportDefaultDeclaration", parent)) {
        identifier(node, "id", node.id);
      }
    }`],
  ['Identifier.validate', 'customIdentifierParent', `function (parent, key, node) {
    const match = /\\.(\\w+)$/.exec(key.toString());
    if (!match) return;
    const [, parentKey] = match;
    const nonComp = {
      computed: false
    };
    if (parentKey === "property") {
      if ((0, _is.default)("MemberExpression", parent, nonComp)) return;
      if ((0, _is.default)("OptionalMemberExpression", parent, nonComp)) return;
    } else if (parentKey === "key") {
      if ((0, _is.default)("Property", parent, nonComp)) return;
      if ((0, _is.default)("Method", parent, nonComp)) return;
    } else if (parentKey === "exported") {
      if ((0, _is.default)("ExportSpecifier", parent)) return;
    } else if (parentKey === "imported") {
      if ((0, _is.default)("ImportSpecifier", parent, {
        imported: node
      })) return;
    } else if (parentKey === "meta") {
      if ((0, _is.default)("MetaProperty", parent, {
        meta: node
      })) return;
    }
    if (((0, _helperValidatorIdentifier.isKeyword)(node.name) || (0, _helperValidatorIdentifier.isReservedWord)(node.name, false)) && node.name !== "this") {
      throw new TypeError(\`"\${node.name}" is not a valid identifier\`);
    }
  }`],
  ['ObjectProperty.validate', 'customObjectPropertyParent', `function (parent, key, node) {
      const validator = (0, _is.default)("ObjectPattern", parent) ? pattern : expression;
      validator(node, "value", node.value);
    }`],
  ['RestElement.validate', 'customRestElementParent', `function (parent, key) {
    const match = /(\\w+)\\[(\\d+)\\]/.exec(key.toString());
    if (!match) throw new Error("Internal Babel error: malformed key.");
    const [, listKey, index] = match;
    if (parent[listKey].length > +index + 1) {
      throw new TypeError(\`RestElement must be last element of \${listKey}\`);
    }
  }`],
  ['VariableDeclaration.validate', 'customVariableDeclarationParent', `function (parent, key, node) {
      const {
        kind,
        declarations
      } = node;
      const parentIsForX = (0, _is.default)("ForXStatement", parent, {
        left: node
      });
      if (parentIsForX) {
        if (declarations.length !== 1) {
          throw new TypeError(\`Exactly one VariableDeclarator is required in the VariableDeclaration of a \${parent.type}\`);
        }
      }
      for (const decl of declarations) {
        if (kind === "const" || kind === "let" || kind === "var") {
          if (!parentIsForX && !decl.init) {
            withoutInit(decl, "id", decl.id);
          } else {
            constOrLetOrVar(decl, "id", decl.id);
          }
        } else {
          usingOrAwaitUsing(decl, "id", decl.id);
        }
      }
    }`],
];

// snapshotDefinitions loads @babel/types with defineType intercepted and returns the Go statements that replay its
// definitions.
function snapshotDefinitions() {
  const events = [];
  const nodes = []; // snapshot nodes, by id
  const idOf = new Map();
  let utils = null;
  let sig = null;
  const kindOf = fn => {
    const s = fn.toString();
    for (const [k, v] of Object.entries(sig)) if (s === v) return k;
    return 'custom';
  };
  const usedCustom = new Set();

  function snapFn(fn, site) {
    const keys = Object.keys(fn);
    const expectKeys = want => {
      if (JSON.stringify(keys) !== JSON.stringify(want)) fail(`${site}: validator properties ${JSON.stringify(keys)}, expected ${JSON.stringify(want)}`);
    };
    const strings = (a, what) => {
      if (!Array.isArray(a) || a.some(x => typeof x !== 'string')) fail(`${site}: ${what} is not an array of strings`);
      return a.map(x => ({ prim: x }));
    };
    const kind = kindOf(fn);
    switch (kind) {
      case 'valueType':
        expectKeys(['type']);
        return { kind: 'factory', factory: 'assertValueType', args: [{ prim: fn.type }] };
      case 'nodeType':
        expectKeys(['oneOfNodeTypes']);
        return { kind: 'factory', factory: 'assertNodeType', args: strings(fn.oneOfNodeTypes, 'oneOfNodeTypes') };
      case 'nodeOrValueType':
        expectKeys(['oneOfNodeOrValueTypes']);
        return { kind: 'factory', factory: 'assertNodeOrValueType', args: strings(fn.oneOfNodeOrValueTypes, 'oneOfNodeOrValueTypes') };
      case 'oneOf':
        expectKeys(['oneOf']);
        for (const v of fn.oneOf) if (typeof v !== 'string' && typeof v !== 'boolean') fail(`${site}: oneOf value ${v}`);
        return { kind: 'factory', factory: 'assertOneOf', args: fn.oneOf.map(x => ({ prim: x })) };
      case 'each':
        expectKeys(['each']);
        return { kind: 'factory', factory: 'assertEach', args: [snap(fn.each, site + '.each')] };
      case 'shape':
        expectKeys(['shapeOf']);
        return { kind: 'factory', factory: 'assertShape', args: [snap(fn.shapeOf, site + '.shapeOf')] };
      case 'chain':
        expectKeys(['chainOf']);
        return { kind: 'factory', factory: 'chain', args: fn.chainOf.map((f, i) => snap(f, site + '.chainOf[' + i + ']')) };
      case 'optionalChainStart':
        expectKeys([]);
        return { kind: 'factory', factory: 'assertOptionalChainStart', args: [] };
    }
    const entry = CUSTOM.find(([s, , src]) => s === site && src === fn.toString());
    if (!entry) fail(`${site}: unknown validator closure:\n${fn.toString()}`);
    usedCustom.add(entry[0]);
    return { kind: 'custom', method: entry[1] };
  }

  function snap(v, site) {
    if (v === null || v === undefined || typeof v === 'string' || typeof v === 'boolean') return { prim: v };
    if (typeof v === 'number') fail(`${site}: a number in the definitions`);
    if (idOf.has(v)) return { ref: idOf.get(v) };
    const id = nodes.length;
    idOf.set(v, id);
    nodes.push(null);
    if (typeof v === 'function') {
      nodes[id] = snapFn(v, site);
    } else if (Array.isArray(v)) {
      if (Object.getOwnPropertyNames(v).length !== v.length + 1) fail(`${site}: an array with holes or properties`);
      nodes[id] = { kind: 'array', items: v.map((x, i) => snap(x, site + '[' + i + ']')) };
    } else if (typeof v === 'object') {
      if (Object.getPrototypeOf(v) !== Object.prototype) fail(`${site}: not a plain object`);
      if (Object.getOwnPropertySymbols(v).length) fail(`${site}: symbol keys`);
      const names = Object.getOwnPropertyNames(v);
      for (const k of names) {
        const d = Object.getOwnPropertyDescriptor(v, k);
        if (!d.enumerable || d.get || d.set) fail(`${site}.${k}: not an enumerable data property`);
      }
      nodes[id] = { kind: 'object', entries: names.map(k => [k, snap(v[k], site + '.' + k)]) };
    } else fail(`${site}: unexpected ${typeof v}`);
    return { ref: id };
  }

  const definitionsFile = () => {
    const stack = new Error().stack.split('\n');
    for (const line of stack) {
      const m = /[\\/]lib[\\/]definitions[\\/](\w+)\.js:/.exec(line);
      if (m && m[1] !== 'utils') return 'definitions/' + m[1] + '.js';
    }
    fail('no definitions file on the stack');
  };
  const record = (via, aliases, type, opts) => {
    events.push({ kind: 'define', via, aliases, type, opts: snap(opts, type), file: definitionsFile() });
  };

  let placeholdersSeen = false;
  const origLoad = Module._load;
  Module._load = function (request, parent, isMain) {
    const filename = Module._resolveFilename(request, parent, isMain);
    const exp = origLoad.apply(this, arguments);
    const rel = path.relative(lib, filename).split(path.sep).join('/');
    if (rel === 'definitions/utils.js' && !utils) {
      utils = exp;
      sig = {
        valueType: utils.assertValueType('x').toString(),
        nodeType: utils.assertNodeType('x').toString(),
        nodeOrValueType: utils.assertNodeOrValueType('x').toString(),
        oneOf: utils.assertOneOf('x').toString(),
        each: utils.assertEach(() => {}).toString(),
        shape: utils.assertShape({}).toString(),
        chain: utils.chain().toString(),
        optionalChainStart: utils.assertOptionalChainStart().toString(),
      };
      // The probe validators above went into allExpandedTypes; that list is internal and they are never used.
      const origAliased = utils.defineAliasedType;
      utils.defineAliasedType = function (...aliases) {
        const inner = origAliased.apply(this, aliases);
        return (type, opts) => {
          record('aliased', aliases, type, opts);
          return inner(type, opts);
        };
      };
      const origDefine = utils.default;
      utils.default = function (type, opts) {
        record('direct', null, type, opts);
        return origDefine(type, opts);
      };
    }
    if (rel === 'definitions/placeholders.js' && !placeholdersSeen) {
      placeholdersSeen = true;
      events.push({ kind: 'placeholders' });
    }
    return exp;
  };
  const warn = console.warn;
  const warnings = [];
  console.warn = (...a) => warnings.push(a.join(' '));
  try {
    require(typesDir);
  } finally {
    console.warn = warn;
    Module._load = origLoad;
  }
  if (!utils || !placeholdersSeen) fail('the definitions were not intercepted');
  const breaking = !!process.env.BABEL_TYPES_8_BREAKING;

  // Count references, to give shared values a variable.
  const refs = new Map();
  const count = s => {
    if (s.ref === undefined) return;
    refs.set(s.ref, (refs.get(s.ref) || 0) + 1);
    if (refs.get(s.ref) > 1) return;
    const n = nodes[s.ref];
    if (n.kind === 'array') n.items.forEach(count);
    else if (n.kind === 'object') n.entries.forEach(([, v]) => count(v));
    else if (n.kind === 'factory') n.args.forEach(count);
  };
  for (const e of events) if (e.kind === 'define') count(e.opts);

  const lines = [];
  const declared = new Map();
  let nextVar = 0;
  // expr returns the Go expression of a snapshot value, first declaring the shared values it reaches.
  const expr = s => {
    if (s.ref === undefined) {
      const v = s.prim;
      if (v === null) return 'nil';
      if (v === undefined) return 'jsvalue.Undefined{}';
      if (typeof v === 'boolean') return String(v);
      return goString(v);
    }
    if (declared.has(s.ref)) return declared.get(s.ref);
    const n = nodes[s.ref];
    let text;
    switch (n.kind) {
      case 'array':
        text = 'jsvalue.NewArray(' + n.items.map(expr).join(', ') + ')';
        break;
      case 'object':
        text = 'jsvalue.NewObject(' + n.entries.map(([k, v]) => goString(k) + ', ' + expr(v)).join(', ') + ')';
        break;
      case 'factory':
        text = 'd.' + n.factory + '(' + n.args.map(expr).join(', ') + ')';
        break;
      case 'custom':
        text = 'd.' + n.method + '()';
        break;
    }
    if (refs.get(s.ref) > 1) {
      const name = 'v' + nextVar++;
      lines.push(`\t${name} := ${text}`);
      declared.set(s.ref, name);
      return name;
    }
    return text;
  };
  let file = null;
  for (const e of events) {
    if (e.kind === 'placeholders') {
      lines.push('\td.placeholders() // definitions/placeholders.js is loaded here, by definitions/misc.js');
      continue;
    }
    if (e.file !== file) {
      file = e.file;
      lines.push(`\t// ${file}`);
    }
    const opts = expr(e.opts);
    if (e.via === 'aliased') lines.push(`\td.aliased([]string{${e.aliases.map(goString).join(', ')}}, ${goString(e.type)}, ${opts})`);
    else lines.push(`\td.defineType(${goString(e.type)}, ${opts})`);
  }
  return { breaking, lines, usedCustom: [...usedCustom], warnings, defines: events.filter(e => e.kind === 'define').length };
}

if (childMode) {
  const r = snapshotDefinitions();
  process.stdout.write(JSON.stringify(r));
  process.exit(0);
}

// ---------------------------------------------------------------------------------------------- parent
delete process.env.BABEL_TYPES_8_BREAKING; // this process loads the default environment
const pkgVersion = require(path.join(typesDir, 'package.json')).version;
if (pkgVersion !== VERSION) fail(`expected @babel/types ${VERSION}, found ${pkgVersion}`);

// The files the Go port translates by hand. A change means the translation must be reviewed.
const HAND_TRANSLATED = [
  'asserts/assertNode.js', 'builders/flow/createFlowUnionType.js', 'builders/flow/createTypeAnnotationBasedOnTypeof.js',
  'builders/productions.js', 'builders/react/buildChildren.js', 'builders/typescript/createTSUnionType.js',
  'builders/validateNode.js', 'clone/clone.js', 'clone/cloneDeep.js', 'clone/cloneDeepWithoutLoc.js', 'clone/cloneNode.js',
  'clone/cloneWithoutLoc.js', 'comments/addComment.js', 'comments/addComments.js', 'comments/inheritInnerComments.js',
  'comments/inheritLeadingComments.js', 'comments/inheritTrailingComments.js', 'comments/inheritsComments.js',
  'comments/removeComments.js', 'constants/index.js', 'converters/ensureBlock.js', 'converters/gatherSequenceExpressions.js',
  'converters/toBindingIdentifierName.js', 'converters/toBlock.js', 'converters/toComputedKey.js', 'converters/toExpression.js',
  'converters/toIdentifier.js', 'converters/toKeyAlias.js', 'converters/toSequenceExpression.js', 'converters/toStatement.js',
  'converters/valueToNode.js', 'definitions/deprecated-aliases.js', 'definitions/index.js', 'definitions/placeholders.js',
  'definitions/utils.js', 'index.js', 'modifications/appendToMemberExpression.js', 'modifications/flow/removeTypeDuplicates.js',
  'modifications/inherits.js', 'modifications/prependToMemberExpression.js', 'modifications/removeProperties.js',
  'modifications/removePropertiesDeep.js', 'modifications/typescript/removeTypeDuplicates.js',
  'retrievers/getAssignmentIdentifiers.js', 'retrievers/getBindingIdentifiers.js', 'retrievers/getFunctionName.js',
  'retrievers/getOuterBindingIdentifiers.js', 'traverse/traverse.js', 'traverse/traverseFast.js',
  'utils/deprecationWarning.js', 'utils/inherit.js', 'utils/react/cleanJSXElementLiteralChild.js', 'utils/shallowEqual.js',
  'validators/buildMatchMemberExpression.js', 'validators/is.js', 'validators/isBinding.js', 'validators/isBlockScoped.js',
  'validators/isImmutable.js', 'validators/isLet.js', 'validators/isNode.js', 'validators/isNodesEquivalent.js',
  'validators/isPlaceholderType.js', 'validators/isReferenced.js', 'validators/isScope.js', 'validators/isSpecifierDefault.js',
  'validators/isType.js', 'validators/isValidES3Identifier.js', 'validators/isValidIdentifier.js', 'validators/isVar.js',
  'validators/matchesPattern.js', 'validators/react/isCompatTag.js', 'validators/react/isReactComponent.js',
  'validators/validate.js',
];
const HAND_TRANSLATED_SHA256 = '17447b653c968c126e1cab80d7200e52261791e1c73869858a89bb3dcfcdb387';
{
  const h = crypto.createHash('sha256');
  for (const f of HAND_TRANSLATED) h.update(f + '\0' + fs.readFileSync(path.join(lib, f), 'utf8') + '\0');
  const listed = new Set(HAND_TRANSLATED);
  const generated = new Set(['builders/generated/index.js', 'builders/generated/lowercase.js', 'builders/generated/uppercase.js',
    'validators/generated/index.js', 'asserts/generated/index.js', 'constants/generated/index.js', 'ast-types/generated/index.js',
    'definitions/core.js', 'definitions/flow.js', 'definitions/jsx.js', 'definitions/misc.js', 'definitions/experimental.js',
    'definitions/typescript.js']);
  const walk = dir => fs.readdirSync(dir, { withFileTypes: true }).flatMap(d => d.isDirectory() ? walk(path.join(dir, d.name)) : [path.join(dir, d.name)]);
  for (const f of walk(lib)) {
    const rel = path.relative(lib, f).split(path.sep).join('/');
    if (!rel.endsWith('.js')) continue;
    if (!listed.has(rel) && !generated.has(rel)) fail('a module neither translated nor generated: ' + rel);
  }
  const digest = h.digest('hex');
  if (digest !== HAND_TRANSLATED_SHA256 && !process.env.BABEL_TYPES_PRINT_HASH) {
    fail(`the hand-translated modules changed (sha256 ${digest}): review the Go port, then update HAND_TRANSLATED_SHA256`);
  }
  if (process.env.BABEL_TYPES_PRINT_HASH) console.error('hand-translated sha256', digest);
}

// Definitions, each environment in its own process.
const runChild = breaking => {
  const env = { ...process.env };
  delete env.BABEL_TYPES_8_BREAKING;
  if (breaking) env.BABEL_TYPES_8_BREAKING = '1';
  const r = spawnSync(process.execPath, [__filename, '--definitions', root], { env, encoding: 'utf8', maxBuffer: 1 << 28 });
  if (r.status !== 0) fail('definitions child failed:\n' + r.stderr);
  return JSON.parse(r.stdout);
};
const defs = runChild(false);
const defs8 = runChild(true);
if (defs.breaking || !defs8.breaking) fail('environment not applied');
if (defs.warnings.length) fail('unexpected warnings: ' + defs.warnings.join('\n'));
const used = new Set([...defs.usedCustom, ...defs8.usedCustom]);
for (const [site] of CUSTOM) if (!used.has(site)) fail('CUSTOM entry never met: ' + site);

const header = `// Code generated by tools/gen_babel_types.cjs from @babel/types ${VERSION}; DO NOT EDIT.

// Copyright (c) 2014-present Sebastian McKenzie and other contributors. MIT License (see LICENSE).
`;
const written = [];
const write = (name, body) => {
  fs.writeFileSync(path.join(OUT, name), header + '\n' + body);
  written.push(path.join(OUT, name));
};

write('definitions_gen.go', `package types

import "github.com/gav4you/npmgo/internal/jsvalue"

// defineDefault replays the ${defs.defines} defineType calls that loading lib/definitions makes, in order, with the
// options they are passed (before defineType mutates them; a value passed more than once is one Go value).
func defineDefault(d *definer) {
${defs.lines.join('\n')}
}

// defineBreaking8 is defineDefault with BABEL_TYPES_8_BREAKING set when @babel/types is loaded (${defs8.defines} calls).
func defineBreaking8(d *definer) {
${defs8.lines.join('\n')}
}
`);

// ---------------------------------------------------------------------------------------------- generated JS modules
const t = require(typesDir);
const readLib = f => fs.readFileSync(path.join(lib, f), 'utf8');

// parseFunctions returns the top-level functions of a module: name, parameters and body lines.
function parseFunctions(src, file) {
  const out = [];
  const re = /^function (\w+)\(([^)]*)\) \{\n([\s\S]*?)\n\}$/gm;
  let m;
  while ((m = re.exec(src))) out.push({ name: m[1], params: m[2] ? m[2].split(', ') : [], body: m[3].split('\n') });
  const declared = (src.match(/^function /gm) || []).length;
  if (declared !== out.length) fail(`${file}: parsed ${out.length} of ${declared} functions`);
  return out;
}
// parseExports returns export name -> local function name for "exports.a = exports.b = f;" lines.
function parseExports(src) {
  const map = new Map();
  for (const line of src.split('\n')) {
    const m = /^exports\.(\w+)((?: = exports\.\w+)*) = (\w+);$/.exec(line);
    if (!m || m[3] === 'void') continue;
    const names = [m[1], ...m[2].split(' = exports.').filter(Boolean)];
    for (const n of names) map.set(n, m[3]);
  }
  return map;
}

// --- builders
const lowerSrc = readLib('builders/generated/lowercase.js');
const upperSrc = readLib('builders/generated/uppercase.js');
const lowerFns = parseFunctions(lowerSrc, 'lowercase.js');
const lowerExports = parseExports(lowerSrc);
const upperExports = new Map();
{
  const m = /^const ArrayExpression = exports\.ArrayExpression = alias\("arrayExpression"\),\n([\s\S]*?);\nconst NumberLiteral/m.exec(upperSrc);
  if (!m) fail('uppercase.js: alias block not found');
  const all = ('ArrayExpression = exports.ArrayExpression = alias("arrayExpression"),\n' + m[1]).split(',\n');
  for (const line of all) {
    const a = /^\s*(\w+) = exports\.(\w+) = alias\("(\w+)"\)$/.exec(line);
    if (!a || a[1] !== a[2]) fail('uppercase.js: ' + line);
    upperExports.set(a[1], lowerExports.get(a[3]) || fail('uppercase.js: no lowercase ' + a[3]));
  }
  const d = /^const NumberLiteral = exports\.NumberLiteral = b\.numberLiteral,\n  RegexLiteral = exports\.RegexLiteral = b\.regexLiteral,\n  RestProperty = exports\.RestProperty = b\.restProperty,\n  SpreadProperty = exports\.SpreadProperty = b\.spreadProperty;$/m.exec(upperSrc);
  if (!d) fail('uppercase.js: deprecated block');
  for (const n of ['NumberLiteral', 'RegexLiteral', 'RestProperty', 'SpreadProperty']) upperExports.set(n, lowerExports.get(n[0].toLowerCase() + n.slice(1)));
}
const builderGoNames = new Map(); // local function -> {primary, aliases, exportNames}
for (const fn of lowerFns) builderGoNames.set(fn.name, { exportNames: [] });
for (const [e, f] of lowerExports) builderGoNames.get(f).exportNames.push(e);
for (const [e, f] of upperExports) builderGoNames.get(f).exportNames.push(e);
for (const [f, info] of builderGoNames) {
  const upper = info.exportNames.find(e => upperExports.has(e));
  const goNames = [...new Set(info.exportNames.map(capitalize))];
  info.primary = upper || capitalize(info.exportNames[0]);
  info.aliases = goNames.filter(n => n !== info.primary);
  if (!info.exportNames.length) fail('builder not exported: ' + f);
}
const goParam = p => (p.startsWith('_') ? p.slice(1) : p) + '_';
function parseDefault(text) {
  if (text === '[]') return 'jsvalue.NewArray()';
  if (text === 'null') return 'nil';
  if (text === 'true' || text === 'false') return text;
  if (/^"[^"\\]*"$/.test(text)) return text;
  fail('builder default ' + text);
}
const builderOut = [];
const nodeFieldsOf = new Map(); // builder -> type (for the cases)
for (const fn of lowerFns) {
  const info = builderGoNames.get(fn.name);
  const params = fn.params.map(p => {
    const m = /^(\w+)(?: = (.*))?$/.exec(p);
    if (!m) fail('param ' + p);
    return { js: m[1], go: goParam(m[1]), def: m[2] };
  });
  const sig = fn.params.join(', ');
  const doc = `// ${info.primary} is the builder ${fn.name}(${sig})${info.exportNames.length > 1 ? ', exported as ' + info.exportNames.join(', ') : ''}.`;
  const lines = [doc, `func ${info.primary}(args ...any) *jsvalue.Object {`];
  params.forEach((p, i) => {
    lines.push(`\t${p.go} := arg(args, ${i})`);
    if (p.def !== undefined) lines.push(`\tif jsvalue.IsUndefined(${p.go}) {\n\t\t${p.go} = ${parseDefault(p.def)}\n\t}`);
  });
  const body = fn.body;
  let i = 0;
  const dep = /^  \(0, _deprecationWarning\.default\)\("(\w+)", "(\w+)", "The node type "\);$/.exec(body[0]);
  if (dep) {
    const call = /^  return (\w+)\(([\w, ]*)\);$/.exec(body[1]);
    if (!call || body.length !== 2) fail('deprecated builder ' + fn.name);
    const target = builderGoNames.get(call[1]).primary;
    lines.push(`\tdeprecationWarning(${goString(dep[1])}, ${goString(dep[2])}, "The node type ")`);
    const callArgs = call[2].split(', ').map(a => params.find(p => p.js === a).go);
    lines.push(`\treturn ${target}(${callArgs.join(', ')})`);
  } else if (body[0] === '  return {') {
    const m = /^    type: "(\w+)"$/.exec(body[1]);
    if (!m || body[2] !== '  };' || body.length !== 3 || params.length) fail('empty builder ' + fn.name);
    lines.push(`\treturn jsvalue.NewObject("type", ${goString(m[1])})`);
    nodeFieldsOf.set(fn.name, m[1]);
  } else {
    if (body[0] === '  if (typeof value === "bigint") {') {
      if (body[1] !== '    value = value.toString();' || body[2] !== '  }') fail('bigint builder');
      lines.push('\tif b, ok := value_.(*big.Int); ok && b != nil {\n\t\tvalue_ = b.String()\n\t}');
      i = 3;
    }
    if (body[i++] !== '  const node = {') fail('builder ' + fn.name + ': ' + body[i - 1]);
    const tm = /^    type: "(\w+)",$/.exec(body[i++]);
    if (!tm) fail('builder type ' + fn.name);
    const type = tm[1];
    nodeFieldsOf.set(fn.name, type);
    const kv = [`"type", ${goString(type)}`];
    while (body[i] !== '  };') {
      const pm = /^    (\w+)(?:: (\w+|null|true|false))?,?$/.exec(body[i++]);
      if (!pm) fail('builder property ' + body[i - 1]);
      if (pm[2] === 'null' || pm[2] === 'true' || pm[2] === 'false') {
        kv.push(`${goString(pm[1])}, ${pm[2] === 'null' ? 'nil' : pm[2]}`);
        continue;
      }
      const p = params.find(q => q.js === (pm[2] || pm[1]));
      if (!p) fail('builder property param ' + body[i - 1]);
      kv.push(`${goString(pm[1])}, ${p.go}`);
    }
    i++;
    lines.push(`\tnode := jsvalue.NewObject(${kv.join(', ')})`);
    if (body[i++] !== `  const defs = NODE_FIELDS.${type};`) fail('builder defs ' + fn.name);
    lines.push(`\tdefs := getProp(NODE_FIELDS, ${goString(type)})`);
    while (body[i] !== '  return node;') {
      const vm = /^  validate\(defs\.(\w+), node, "(\w+)", (\w+)(, 1)?\);$/.exec(body[i++]);
      if (!vm || vm[1] !== vm[2]) fail('builder validate ' + body[i - 1]);
      const p = params.find(q => q.js === vm[3]);
      lines.push(`\tvalidateInternal(getProp(defs, ${goString(vm[1])}), node, ${goString(vm[2])}, ${p.go}, ${vm[4] ? 'true' : 'false'})`);
    }
    if (i !== body.length - 1) fail('builder tail ' + fn.name);
    lines.push('\treturn node');
  }
  lines.push('}');
  for (const a of info.aliases) {
    lines.push('', `// ${a} is ${info.primary}.`, `func ${a}(args ...any) *jsvalue.Object { return ${info.primary}(args...) }`);
  }
  builderOut.push(lines.join('\n'));
}
write('builders_gen.go', `package types

import (
	"math/big"

	"github.com/gav4you/npmgo/internal/jsvalue"
)

${builderOut.join('\n\n')}
`);

// --- validators
const validatorsSrc = readLib('validators/generated/index.js');
const validatorFns = parseFunctions(validatorsSrc, 'validators/generated/index.js');
const exportNames = new Set(Object.keys(t));
const validatorOut = [];
const goValidatorName = new Map();
for (const fn of validatorFns) {
  if (fn.params.join(',') !== 'node,opts') fail('validator params ' + fn.name);
  const exported = exportNames.has(fn.name) && t[fn.name] === require(path.join(lib, 'validators/generated/index.js'))[fn.name];
  const go = exported ? capitalize(fn.name) : fn.name + 'Generated';
  goValidatorName.set(fn.name, go);
  const body = fn.body;
  const lines = [`// ${go} is ${fn.name}(node, opts)${exported ? '' : ' of lib/validators/generated (the main entry point exports another ' + fn.name + ')'}.`, `func ${go}(node any, opts ...any) bool {`];
  let i = 0;
  const dep = /^  \(0, _deprecationWarning\.default\)\("(\w+)", "(\w+)"\);$/.exec(body[0]);
  if (dep) {
    lines.push(`\tdeprecationWarning(${goString(dep[1])}, ${goString(dep[2])})`);
    i = 1;
    const del = /^  return (\w+)\(node, opts\);$/.exec(body[1]);
    if (del) {
      if (body.length !== 2) fail('delegating validator');
      lines.push(`\treturn ${capitalize(del[1])}(node, opts...)`, '}');
      validatorOut.push(lines.join('\n'));
      continue;
    }
  }
  if (body[i++] !== '  if (!node) return false;') fail('validator ' + fn.name + ' head');
  const simple = /^  if \(node\.type !== "(\w+)"\) return false;$/.exec(body[i]);
  if (simple) {
    i++;
    lines.push('\tif !jsvalue.Truthy(node) {\n\t\treturn false\n\t}');
    lines.push(`\tif typ, ok := stringProp(node, "type"); !ok || typ != ${goString(simple[1])} {\n\t\treturn false\n\t}`);
  } else {
    if (body[i++] !== '  switch (node.type) {') fail('validator ' + fn.name + ' switch');
    const cases = [];
    let placeholder = null;
    while (body[i] !== '      break;') {
      const c = /^    case "(\w+)":$/.exec(body[i++]);
      if (!c) fail('validator case ' + body[i - 1]);
      cases.push(c[1]);
    }
    i++;
    if (body[i] === '    case "Placeholder":') {
      i++;
      const single = /^      if \(node\.expectedNode === "(\w+)"\) break;$/.exec(body[i]);
      if (single) {
        // case "Placeholder": if (node.expectedNode === "X") break; (falling through to default)
        placeholder = [single[1]];
        i++;
      } else {
        if (body[i++] !== '      switch (node.expectedNode) {') fail('placeholder switch ' + fn.name);
        placeholder = [];
        while (body[i] !== '          break;') {
          const c = /^        case "(\w+)":$/.exec(body[i++]);
          if (!c) fail('placeholder case ' + body[i - 1]);
          placeholder.push(c[1]);
        }
        i++;
        const tail = ['        default:', '          return false;', '      }', '      break;'];
        for (const l of tail) if (body[i++] !== l) fail('placeholder tail ' + fn.name);
      }
    }
    for (const l of ['    default:', '      return false;', '  }']) if (body[i++] !== l) fail('switch tail ' + fn.name);
    lines.push('\tif !jsvalue.Truthy(node) {\n\t\treturn false\n\t}');
    lines.push('\ttyp, ok := stringProp(node, "type")\n\tif !ok {\n\t\treturn false\n\t}');
    lines.push('\tswitch typ {');
    if (cases.length) lines.push(`\tcase ${cases.map(goString).join(', ')}:`);
    if (placeholder) {
      lines.push('\tcase "Placeholder":');
      lines.push(`\t\tif expected, ok := stringProp(node, "expectedNode"); !ok || !(${placeholder.map(p => 'expected == ' + goString(p)).join(' || ')}) {\n\t\t\treturn false\n\t\t}`);
    }
    lines.push('\tdefault:\n\t\treturn false\n\t}');
  }
  if (body[i++] !== '  return opts == null || (0, _shallowEqual.default)(node, opts);' || i !== body.length) fail('validator tail ' + fn.name);
  lines.push('\treturn optsMatch(node, opts)', '}');
  validatorOut.push(lines.join('\n'));
}
write('validators_gen.go', `package types

import "github.com/gav4you/npmgo/internal/jsvalue"

${validatorOut.join('\n\n')}
`);

// --- asserts
const assertsSrc = readLib('asserts/generated/index.js');
const assertFns = parseFunctions(assertsSrc, 'asserts/generated/index.js').filter(f => f.name !== 'assert');
const assertOut = [];
for (const fn of assertFns) {
  if (fn.params.join(',') !== 'node,opts') fail('assert params ' + fn.name);
  const go = capitalize(fn.name);
  const lines = [`// ${go} is ${fn.name}(node, opts): it panics with the Error JavaScript throws when the node does not match.`, `func ${go}(node any, opts ...any) {`];
  let i = 0;
  const dep = /^  \(0, _deprecationWarning\.default\)\("(\w+)", "(\w+)"\);$/.exec(fn.body[0]);
  if (dep) {
    lines.push(`\tdeprecationWarning(${goString(dep[1])}, ${goString(dep[2])})`);
    i = 1;
  }
  const a = /^  assert\("(\w+)", node, opts\);$/.exec(fn.body[i++]);
  if (!a || i !== fn.body.length) fail('assert body ' + fn.name);
  lines.push(`\tassertType(${goString(a[1])}, node, optArg(opts))`, '}');
  assertOut.push(lines.join('\n'));
}
write('asserts_gen.go', `package types

${assertOut.join('\n\n')}
`);

// --- constants
const constantsSrc = readLib('constants/generated/index.js');
const constLines = [];
for (const line of constantsSrc.split('\n')) {
  let m = /^const (\w+) = exports\.(\w+) = _index\.FLIPPED_ALIAS_KEYS\["(\w+)"\];$/.exec(line);
  if (m) {
    if (m[1] !== m[2]) fail('constant ' + line);
    constLines.push({ name: m[1], alias: m[3] });
    continue;
  }
  m = /^const (\w+) = exports\.(\w+) = (\w+);$/.exec(line);
  if (m) {
    if (m[1] !== m[2]) fail('constant ' + line);
    constLines.push({ name: m[1], same: m[3] });
  }
}
if (constLines.length !== 51) fail('constants: ' + constLines.length);
write('constants_gen.go', `package types

import "github.com/gav4you/npmgo/internal/jsvalue"

// The lists of node types of each alias, from lib/constants/generated: each is FLIPPED_ALIAS_KEYS's own array.
var (
${constLines.map(c => `\t${c.name} *jsvalue.Array`).join('\n')}
)

func setAliasConstants() {
${constLines.map(c => c.alias ? `\t${c.name} = flippedAliasArray(${goString(c.alias)})` : `\t${c.name} = ${c.same}`).join('\n')}
}
`);

// --- the export table
const builderByExport = new Map();
for (const [f, info] of builderGoNames) for (const e of info.exportNames) builderByExport.set(e, capitalize(e));
const specialGo = {
  react: 'React', __internal__deprecationWarning: 'InternalDeprecationWarning',
};
const exportLines = [];
for (const name of Object.keys(t)) {
  let go;
  if (specialGo[name]) go = specialGo[name];
  else if (builderByExport.has(name)) go = builderByExport.get(name);
  else go = /^[A-Z_0-9]+$/.test(name) ? name : capitalize(name);
  const v = t[name];
  const kind = typeof v === 'function' ? 'function' : Array.isArray(v) ? 'array' : typeof v;
  // Variables are read when the table is used, after the package has been initialised.
  const value = kind === 'function' ? go : `func() any { return ${go} }`;
  exportLines.push(`\t{${goString(name)}, ${goString(kind)}, ${value}},`);
}
write('exports_gen.go', `package types

// exportTable lists the main entry point's exports in the order Object.keys(require("@babel/types")) gives them, with
// the Go value of each: functions as themselves, other values through a function that reads the package variable.
var exportTable = []exportEntry{
${exportLines.join('\n')}
}
`);

// gofmt aligns what the templates above leave unaligned (the Go toolchain of go.mod makes it reproducible).
{
  const r = spawnSync('gofmt', ['-w', ...written], { encoding: 'utf8' });
  if (r.status !== 0) fail('gofmt failed:\n' + r.stderr);
}
console.error(`wrote ${OUT}: ${defs.defines} + ${defs8.defines} definitions, ${lowerFns.length} builders, ${validatorFns.length} validators, ${assertFns.length} asserts, ${constLines.length} constants, ${Object.keys(t).length} exports`);
