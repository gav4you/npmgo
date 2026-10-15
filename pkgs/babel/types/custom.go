// Copyright (c) 2014-present Sebastian McKenzie and other contributors. MIT License (see LICENSE).
// Translated to Go by the npmgo contributors from @babel/types 7.29.0 (lib/definitions/core.js, experimental.js,
// typescript.js: the validators written inline in the definitions).

package types

// The validators of the definitions that no factory of utils.js makes. tools/gen_babel_types.cjs lists each with
// the exact source text it translates and calls the method below where the definitions use it; the properties set
// here are the ones the JavaScript function carries (Object.assign, or an assignment), and Name its name.

import (
	"regexp"

	"github.com/gav4you/npmgo/internal/jsvalue"
	hsp "github.com/gav4you/npmgo/pkgs/babel/helper-string-parser"
	hvi "github.com/gav4you/npmgo/pkgs/babel/helper-validator-identifier"
)

// computedOr returns the validator `node.computed ? computed : normal`.
func computedOr(computed, normal *Validator) func(node, key, val any) {
	return func(node, key, val any) {
		v := normal
		if truthy(getProp(node, "computed")) {
			v = computed
		}
		v.fn(node, key, val)
	}
}

// BinaryExpression.left: an Expression, or a PrivateName when the operator is "in".
func (d *definer) customBinaryExpressionLeft() *Validator {
	expression := d.assertNodeType("Expression")
	inOp := d.assertNodeType("Expression", "PrivateName")
	return newValidator("", func(node, key, val any) {
		v := expression
		if strictEquals(getProp(node, "operator"), "in") {
			v = inOp
		}
		v.fn(node, key, val)
	}, "oneOfNodeTypes", jsvalue.NewArray("Expression", "PrivateName"))
}

// NumericLiteral.value's second validator builds an Error for a negative or non-finite value and never throws it.
func (d *definer) customNumericLiteralValue() *Validator {
	return newValidator("", func(node, key, val any) {}, "type", "number")
}

// MemberExpression.property.
func (d *definer) customMemberExpressionProperty() *Validator {
	normal := d.assertNodeType("Identifier", "PrivateName")
	computed := d.assertNodeType("Expression")
	return newValidator("validator", computedOr(computed, normal),
		"oneOfNodeTypes", jsvalue.NewArray("Expression", "Identifier", "PrivateName"))
}

// ObjectMethod.key.
func (d *definer) customObjectMethodKey() *Validator {
	normal := d.assertNodeType("Identifier", "StringLiteral", "NumericLiteral", "BigIntLiteral")
	computed := d.assertNodeType("Expression")
	return newValidator("validator", computedOr(computed, normal),
		"oneOfNodeTypes", jsvalue.NewArray("Expression", "Identifier", "StringLiteral", "NumericLiteral", "BigIntLiteral"))
}

// ObjectProperty.key.
func (d *definer) customObjectPropertyKey() *Validator {
	normal := d.assertNodeType("Identifier", "StringLiteral", "NumericLiteral", "BigIntLiteral", "DecimalLiteral", "PrivateName")
	computed := d.assertNodeType("Expression")
	return newValidator("", computedOr(computed, normal),
		"oneOfNodeTypes", jsvalue.NewArray("Expression", "Identifier", "StringLiteral", "NumericLiteral", "BigIntLiteral", "DecimalLiteral", "PrivateName"))
}

// The first validator of classMethodOrPropertyCommon()'s key (ClassMethod, ClassProperty, TSDeclareMethod).
func (d *definer) customClassMethodOrPropertyKey() *Validator {
	normal := d.assertNodeType("Identifier", "StringLiteral", "NumericLiteral", "BigIntLiteral")
	computed := d.assertNodeType("Expression")
	return newValidator("", computedOr(computed, normal))
}

// The first validator of ClassAccessorProperty.key.
func (d *definer) customClassAccessorPropertyKey() *Validator {
	normal := d.assertNodeType("Identifier", "StringLiteral", "NumericLiteral", "BigIntLiteral", "PrivateName")
	computed := d.assertNodeType("Expression")
	return newValidator("", computedOr(computed, normal))
}

// TemplateElement.value's templateElementCookedValidator: the raw text must be the contents of a template up to its
// end, and value.cooked is set from it (null where an escape is invalid).
func (d *definer) customTemplateElementCooked() *Validator {
	return newValidator("templateElementCookedValidator", func(node, key, val any) {
		raw := getProp(getProp(node, "value"), "raw")
		unterminatedCalled := false
		internal := func(int, int, int) { panic(&jsvalue.Error{Message: "Internal @babel/types error."}) }
		rawText, ok := raw.(string)
		if !ok {
			// node.value is not val when validate(node, "value", val) is called with another object: readStringContents
			// then reads a value that is not a string.
			readNonString(raw)
			unterminatedCalled = true // an empty array: the loop ends at once
		}
		res := hsp.ReadStringContents("template", rawText, 0, 0, 0, hsp.ErrorHandlers{
			Unterminated:                     func(int, int, int) { unterminatedCalled = true },
			StrictNumericEscape:              internal,
			InvalidEscapeSequence:            internal,
			NumericSeparatorInEscapeSequence: internal,
			UnexpectedNumericSeparator:       internal,
			InvalidDigit:                     func(a, b, c, _ int) bool { internal(a, b, c); return false },
			InvalidCodePoint:                 internal,
		})
		if !unterminatedCalled {
			panic(&jsvalue.Error{Message: "Invalid raw"})
		}
		var cooked any = res.Str
		if res.FirstInvalidLoc != nil {
			cooked = nil
		}
		setProp(getProp(node, "value"), "cooked", cooked)
	})
}

// readNonString is what readStringContents("template", input, 0, ...) does with an input that is not a string:
// `const {length} = input` throws for null and undefined, the loop calls input.charCodeAt (or, for a length of 0 or
// less, input.slice), which only an empty array has. It returns for an empty array, whose contents read as "".
func readNonString(input any) {
	if nullish(input) {
		what := "null"
		if jsvalue.IsUndefined(input) {
			what = "undefined"
		}
		panic(typeError("Cannot destructure property 'length' of 'input' as it is " + what + "."))
	}
	length := getProp(input, "length")
	if jsvalue.IsNumber(length) && jsvalue.Float(length) <= 0 {
		if isArray(input) {
			return
		}
		panic(typeError("input.slice is not a function"))
	}
	panic(typeError("input.charCodeAt is not a function"))
}

// quasisCount is the third validator of TemplateLiteral.expressions and TSTemplateLiteralType.types: there must be
// one more quasi than expressions (types).
func quasisCount(what string) func(node, key, val any) {
	return func(node, key, val any) {
		quasis := getProp(getProp(node, "quasis"), "length")
		expected := jsvalue.ToNumber(getProp(val, "length")) + 1
		if !strictEquals(quasis, expected) {
			panic(typeError("Number of " + toStr(getProp(node, "type")) + " quasis should be exactly one more than the number of " + what +
				".\nExpected " + toStr(jsvalue.ToNumber(getProp(val, "length"))+1) + " quasis but got " + toStr(getProp(getProp(node, "quasis"), "length"))))
		}
	}
}

// TemplateLiteral.expressions' third validator.
func (d *definer) customTemplateLiteralExpressions() *Validator {
	return newValidator("", quasisCount("expressions"))
}

// TSTemplateLiteralType.types' third validator.
func (d *definer) customTSTemplateLiteralTypeTypes() *Validator {
	return newValidator("", quasisCount("types"))
}

// OptionalMemberExpression.property.
func (d *definer) customOptionalMemberExpressionProperty() *Validator {
	normal := d.assertNodeType("Identifier")
	computed := d.assertNodeType("Expression")
	return newValidator("", computedOr(computed, normal), "oneOfNodeTypes", jsvalue.NewArray("Expression", "Identifier"))
}

// TSLiteralType.literal: a literal, or a UnaryExpression "-" of a number or bigint.
func (d *definer) customTSLiteralTypeLiteral() *Validator {
	unaryExpression := d.assertNodeType("NumericLiteral", "BigIntLiteral")
	unaryOperator := d.assertOneOf("-")
	literal := d.assertNodeType("NumericLiteral", "StringLiteral", "BooleanLiteral", "BigIntLiteral", "TemplateLiteral")
	return newValidator("validator", func(parent, key, node any) {
		if Is("UnaryExpression", node) {
			unaryOperator.fn(node, "operator", getProp(node, "operator"))
			unaryExpression.fn(node, "argument", getProp(node, "argument"))
		} else {
			literal.fn(parent, key, node)
		}
	}, "oneOfNodeTypes", jsvalue.NewArray("NumericLiteral", "StringLiteral", "BooleanLiteral", "BigIntLiteral", "TemplateLiteral", "UnaryExpression"))
}

// The element validator of File.tokens: anything.
func (d *definer) customFileTokensEach() *Validator {
	return newValidator("", func(node, key, val any) {}, "type", "any")
}

// File.comments: no check (its each property describes the comments, as a plain object).
func (d *definer) customFileComments() *Validator {
	return newValidator("", func(node, key, val any) {},
		"each", jsvalue.NewObject("oneOfNodeTypes", jsvalue.NewArray("CommentBlock", "CommentLine")))
}

// BindExpression.object and .callee: no check.
func (d *definer) customBindExpressionNoop() *Validator {
	return newValidator("", func(node, key, val any) {}, "oneOfNodeTypes", jsvalue.NewArray("Expression"))
}

// --- BABEL_TYPES_8_BREAKING

// AssignmentExpression.operator: "=" where the left side is a Pattern, any assignment operator otherwise.
func (d *definer) customAssignmentExpressionOperator() *Validator {
	identifier := d.assertOneOfArray(jsvalue.NewArray(items(ASSIGNMENT_OPERATORS)...))
	pattern := d.assertOneOf("=")
	return newValidator("", func(node, key, val any) {
		v := identifier
		if Is("Pattern", getProp(node, "left")) {
			v = pattern
		}
		v.fn(node, key, val)
	}, "oneOf", ASSIGNMENT_OPERATORS)
}

// Identifier.name's second validator: a valid identifier name (reserved words allowed).
func (d *definer) customIdentifierName() *Validator {
	return newValidator("", func(node, key, val any) {
		if !IsValidIdentifier(val, false) {
			panic(typeError(`"` + toStr(val) + `" is not a valid identifier name`))
		}
	}, "type", "string")
}

// RegExpLiteral.flags' second validator: only the flags d, g, i, m, s, u, v and y.
func (d *definer) customRegExpLiteralFlags() *Validator {
	return newValidator("", func(node, key, val any) {
		// /[^dgimsuvy]/.exec(val): the first code unit that is not a flag.
		for _, u := range utf16Units(toStr(val)) {
			switch u {
			case 'd', 'g', 'i', 'm', 's', 'u', 'v', 'y':
				continue
			}
			panic(typeError(`"` + unitString(u) + `" is not a valid RegExp flag`))
		}
	}, "type", "string")
}

// ObjectProperty.shorthand's second validator.
func (d *definer) customObjectPropertyShorthand() *Validator {
	return newValidator("", func(node, key, shorthand any) {
		if !truthy(shorthand) {
			return
		}
		if truthy(getProp(node, "computed")) {
			panic(typeError("Property shorthand of ObjectProperty cannot be true if computed is true"))
		}
		if !Is("Identifier", getProp(node, "key")) {
			panic(typeError("Property shorthand of ObjectProperty cannot be true if key is not an Identifier"))
		}
	}, "type", "boolean")
}

// TryStatement.block's second validator.
func (d *definer) customTryStatementBlock() *Validator {
	return newValidator("", func(node, key, val any) {
		if !truthy(getProp(node, "handler")) && !truthy(getProp(node, "finalizer")) {
			panic(typeError("TryStatement expects either a handler or finalizer, or both"))
		}
	}, "oneOfNodeTypes", jsvalue.NewArray("BlockStatement"))
}

// ExportNamedDeclaration.declaration's second validator.
func (d *definer) customExportNamedDeclarationDeclaration() *Validator {
	return newValidator("", func(node, key, val any) {
		if truthy(val) && truthy(getProp(getProp(node, "specifiers"), "length")) {
			panic(typeError("Only declaration or specifiers is allowed on ExportNamedDeclaration"))
		}
		if truthy(val) && truthy(getProp(node, "source")) {
			panic(typeError("Cannot export a declaration from a source"))
		}
	}, "oneOfNodeTypes", jsvalue.NewArray("Declaration"))
}

// The element validator of ExportNamedDeclaration.specifiers: only ExportSpecifiers without a source.
func (d *definer) customExportNamedDeclarationSpecifier() *Validator {
	sourced := d.assertNodeType("ExportSpecifier", "ExportDefaultSpecifier", "ExportNamespaceSpecifier")
	sourceless := d.assertNodeType("ExportSpecifier")
	return newValidator("", func(node, key, val any) {
		v := sourceless
		if truthy(getProp(node, "source")) {
			v = sourced
		}
		v.fn(node, key, val)
	}, "oneOfNodeTypes", jsvalue.NewArray("ExportSpecifier", "ExportDefaultSpecifier", "ExportNamespaceSpecifier"))
}

// ForOfStatement.left.
func (d *definer) customForOfStatementLeft() *Validator {
	declaration := d.assertNodeType("VariableDeclaration")
	lval := d.assertNodeType("Identifier", "MemberExpression", "ArrayPattern", "ObjectPattern", "TSAsExpression", "TSSatisfiesExpression", "TSTypeAssertion", "TSNonNullExpression")
	return newValidator("", func(node, key, val any) {
		if Is("VariableDeclaration", val) {
			declaration.fn(node, key, val)
		} else {
			lval.fn(node, key, val)
		}
	}, "oneOfNodeTypes", jsvalue.NewArray("VariableDeclaration", "Identifier", "MemberExpression", "ArrayPattern", "ObjectPattern", "TSAsExpression", "TSSatisfiesExpression", "TSTypeAssertion", "TSNonNullExpression"))
}

// MetaProperty.meta's second validator: function.sent, new.target, import.meta.
func (d *definer) customMetaPropertyMeta() *Validator {
	return newValidator("", func(node, key, val any) {
		var property any = jsvalue.Undefined{}
		switch n := getProp(val, "name"); {
		case strictEquals(n, "function"):
			property = "sent"
		case strictEquals(n, "new"):
			property = "target"
		case strictEquals(n, "import"):
			property = "meta"
		}
		if !Is("Identifier", getProp(node, "property"), jsvalue.NewObject("name", property)) {
			panic(typeError("Unrecognised MetaProperty"))
		}
	}, "oneOfNodeTypes", jsvalue.NewArray("Identifier"))
}

// YieldExpression.delegate's second validator.
func (d *definer) customYieldExpressionDelegate() *Validator {
	return newValidator("", func(node, key, val any) {
		if truthy(val) && !truthy(getProp(node, "argument")) {
			panic(typeError("Property delegate of YieldExpression cannot be true if there is no argument"))
		}
	}, "type", "boolean")
}

// idUnlessExportDefault is the parent validation of FunctionDeclaration and ClassDeclaration: an id is required
// outside export default.
func (d *definer) idUnlessExportDefault() *Validator {
	identifier := d.assertNodeType("Identifier")
	return newValidator("", func(parent, key, node any) {
		if !Is("ExportDefaultDeclaration", parent) {
			identifier.fn(node, "id", getProp(node, "id"))
		}
	})
}

// FunctionDeclaration's parent validation.
func (d *definer) customFunctionDeclarationParent() *Validator { return d.idUnlessExportDefault() }

// ClassDeclaration's parent validation.
func (d *definer) customClassDeclarationParent() *Validator { return d.idUnlessExportDefault() }

var lastKeyPart = regexp.MustCompile(`\.(\w+)$`)

// Identifier's parent validation: a keyword or reserved word is not an identifier, except where a name is allowed.
func (d *definer) customIdentifierParent() *Validator {
	return newValidator("", func(parent, key, node any) {
		match := lastKeyPart.FindStringSubmatch(keyToString(key))
		if match == nil {
			return
		}
		parentKey := match[1]
		nonComp := jsvalue.NewObject("computed", false)
		switch parentKey {
		case "property":
			if Is("MemberExpression", parent, nonComp) {
				return
			}
			if Is("OptionalMemberExpression", parent, nonComp) {
				return
			}
		case "key":
			if Is("Property", parent, nonComp) {
				return
			}
			if Is("Method", parent, nonComp) {
				return
			}
		case "exported":
			if Is("ExportSpecifier", parent) {
				return
			}
		case "imported":
			if Is("ImportSpecifier", parent, jsvalue.NewObject("imported", node)) {
				return
			}
		case "meta":
			if Is("MetaProperty", parent, jsvalue.NewObject("meta", node)) {
				return
			}
		}
		name := getProp(node, "name")
		s, _ := name.(string)
		_, isString := name.(string)
		if isString && (hvi.IsKeyword(s) || hvi.IsReservedWord(s, false)) && !strictEquals(name, "this") {
			panic(typeError(`"` + toStr(name) + `" is not a valid identifier`))
		}
	})
}

// keyToString is key.toString() for a validation key: a string, or the "key[i]" object of assertEach.
func keyToString(key any) string {
	if s, ok := key.(string); ok {
		return s
	}
	return toStr(key)
}

// ObjectProperty's parent validation: its value is a pattern inside an ObjectPattern, an expression elsewhere.
func (d *definer) customObjectPropertyParent() *Validator {
	pattern := d.assertNodeType("Identifier", "Pattern", "TSAsExpression", "TSSatisfiesExpression", "TSNonNullExpression", "TSTypeAssertion")
	expression := d.assertNodeType("Expression")
	return newValidator("", func(parent, key, node any) {
		v := expression
		if Is("ObjectPattern", parent) {
			v = pattern
		}
		v.fn(node, "value", getProp(node, "value"))
	})
}

var listKeyIndex = regexp.MustCompile(`(\w+)\[(\d+)\]`)

// RestElement's parent validation: it must be the last element of its list.
func (d *definer) customRestElementParent() *Validator {
	return newValidator("", func(parent, key, node any) {
		match := listKeyIndex.FindStringSubmatch(keyToString(key))
		if match == nil {
			panic(&jsvalue.Error{Message: "Internal Babel error: malformed key."})
		}
		listKey, index := match[1], match[2]
		length := jsvalue.ToNumber(getProp(getProp(parent, listKey), "length"))
		if length > jsvalue.StringToNumber(index)+1 {
			panic(typeError("RestElement must be last element of " + listKey))
		}
	})
}

// VariableDeclaration's parent validation: one declarator in a for-in / for-of head, and the patterns each kind of
// declaration allows.
func (d *definer) customVariableDeclarationParent() *Validator {
	withoutInit := d.assertNodeType("Identifier", "Placeholder")
	constOrLetOrVar := d.assertNodeType("Identifier", "ArrayPattern", "ObjectPattern", "Placeholder")
	usingOrAwaitUsing := d.assertNodeType("Identifier", "VoidPattern", "Placeholder")
	return newValidator("", func(parent, key, node any) {
		kind := getProp(node, "kind")
		declarations := getProp(node, "declarations")
		parentIsForX := Is("ForXStatement", parent, jsvalue.NewObject("left", node))
		if parentIsForX {
			if !strictEquals(getProp(declarations, "length"), 1.0) {
				panic(typeError("Exactly one VariableDeclarator is required in the VariableDeclaration of a " + toStr(getProp(parent, "type"))))
			}
		}
		for _, decl := range iterate(declarations, "declarations") {
			if strictEquals(kind, "const") || strictEquals(kind, "let") || strictEquals(kind, "var") {
				if !parentIsForX && !truthy(getProp(decl, "init")) {
					withoutInit.fn(decl, "id", getProp(decl, "id"))
				} else {
					constOrLetOrVar.fn(decl, "id", getProp(decl, "id"))
				}
			} else {
				usingOrAwaitUsing.fn(decl, "id", getProp(decl, "id"))
			}
		}
	})
}
