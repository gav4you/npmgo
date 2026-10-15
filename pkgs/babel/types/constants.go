// Copyright (c) 2014-present Sebastian McKenzie and other contributors. MIT License (see LICENSE).
// Translated to Go by the npmgo contributors from @babel/types 7.29.0 (lib/constants/index.js).

package types

import "github.com/gav4you/npmgo/internal/jsvalue"

// spread assembles an array literal with spread elements: an array part contributes its elements, anything else
// itself.
func spread(parts ...any) *jsvalue.Array {
	out := &jsvalue.Array{}
	for _, p := range parts {
		if a, ok := p.(*jsvalue.Array); ok {
			out.Push(a.Items()...)
		} else {
			out.Push(p)
		}
	}
	return out
}

// suffixed is arr.map(op => op + suffix).
func suffixed(arr *jsvalue.Array, suffix string) *jsvalue.Array {
	out := &jsvalue.Array{}
	for _, op := range arr.Items() {
		out.Push(toStr(op) + suffix)
	}
	return out
}

// The constants of lib/constants/index.js.
var (
	STATEMENT_OR_BLOCK_KEYS         = jsvalue.NewArray("consequent", "body", "alternate")
	FLATTENABLE_KEYS                = jsvalue.NewArray("body", "expressions")
	FOR_INIT_KEYS                   = jsvalue.NewArray("left", "init")
	COMMENT_KEYS                    = jsvalue.NewArray("leadingComments", "trailingComments", "innerComments")
	LOGICAL_OPERATORS               = jsvalue.NewArray("||", "&&", "??")
	UPDATE_OPERATORS                = jsvalue.NewArray("++", "--")
	BOOLEAN_NUMBER_BINARY_OPERATORS = jsvalue.NewArray(">", "<", ">=", "<=")
	EQUALITY_BINARY_OPERATORS       = jsvalue.NewArray("==", "===", "!=", "!==")
	COMPARISON_BINARY_OPERATORS     = spread(EQUALITY_BINARY_OPERATORS, "in", "instanceof")
	BOOLEAN_BINARY_OPERATORS        = spread(COMPARISON_BINARY_OPERATORS, BOOLEAN_NUMBER_BINARY_OPERATORS)
	NUMBER_BINARY_OPERATORS         = jsvalue.NewArray("-", "/", "%", "*", "**", "&", "|", ">>", ">>>", "<<", "^")
	BINARY_OPERATORS                = spread("+", NUMBER_BINARY_OPERATORS, BOOLEAN_BINARY_OPERATORS, "|>")
	ASSIGNMENT_OPERATORS            = spread("=", "+=", suffixed(NUMBER_BINARY_OPERATORS, "="), suffixed(LOGICAL_OPERATORS, "="))
	BOOLEAN_UNARY_OPERATORS         = jsvalue.NewArray("delete", "!")
	NUMBER_UNARY_OPERATORS          = jsvalue.NewArray("+", "-", "~")
	STRING_UNARY_OPERATORS          = jsvalue.NewArray("typeof")
	UNARY_OPERATORS                 = spread("void", "throw", BOOLEAN_UNARY_OPERATORS, NUMBER_UNARY_OPERATORS, STRING_UNARY_OPERATORS)
	INHERIT_KEYS                    = jsvalue.NewObject(
		"optional", jsvalue.NewArray("typeAnnotation", "typeParameters", "returnType"),
		"force", jsvalue.NewArray("start", "loc", "end"),
	)
	BLOCK_SCOPED_SYMBOL = jsvalue.SymbolFor("var used to be block scoped")
	NOT_LOCAL_BINDING   = jsvalue.SymbolFor("should not be considered a local binding")
)
