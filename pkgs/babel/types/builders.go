// Copyright (c) 2014-present Sebastian McKenzie and other contributors. MIT License (see LICENSE).
// Translated to Go by the npmgo contributors from @babel/types 7.29.0 (lib/builders/productions.js,
// lib/builders/flow/*.js, lib/builders/typescript/createTSUnionType.js, lib/builders/react/buildChildren.js,
// lib/utils/react/cleanJSXElementLiteralChild.js).

package types

import (
	"regexp"
	"strings"

	"github.com/gav4you/npmgo/internal/jsvalue"
)

// BuildUndefinedNode is buildUndefinedNode(): the expression void 0.
func BuildUndefinedNode() *jsvalue.Object {
	return UnaryExpression("void", NumericLiteral(0.0), true)
}

// CreateTypeAnnotationBasedOnTypeof is createTypeAnnotationBasedOnTypeof(type): the flow annotation of a typeof
// result; anything else panics with JavaScript's Error.
func CreateTypeAnnotationBasedOnTypeof(typ any) *jsvalue.Object {
	if s, ok := typ.(string); ok {
		switch s {
		case "string":
			return StringTypeAnnotation()
		case "number":
			return NumberTypeAnnotation()
		case "undefined":
			return VoidTypeAnnotation()
		case "boolean":
			return BooleanTypeAnnotation()
		case "function":
			return GenericTypeAnnotation(Identifier("Function"))
		case "object":
			return GenericTypeAnnotation(Identifier("Object"))
		case "symbol":
			return GenericTypeAnnotation(Identifier("Symbol"))
		case "bigint":
			return AnyTypeAnnotation()
		}
	}
	panic(&jsvalue.Error{Message: "Invalid typeof value: " + toStr(typ)})
}

// CreateFlowUnionType is createFlowUnionType(types): the one type left by removeTypeDuplicates, or their union.
func CreateFlowUnionType(types any) any {
	flattened := RemoveTypeDuplicates(types)
	if flattened.Len() == 1 {
		return flattened.At(0)
	}
	return UnionTypeAnnotation(flattened)
}

// CreateUnionTypeAnnotation is createUnionTypeAnnotation, another export of createFlowUnionType.
func CreateUnionTypeAnnotation(types any) any { return CreateFlowUnionType(types) }

// CreateTSUnionType is createTSUnionType(typeAnnotations): the TypeScript union of the types (a TSTypeAnnotation
// stands for its type), or the one type left without duplicates.
func CreateTSUnionType(typeAnnotations any) any {
	methodTarget(typeAnnotations, "map", "typeAnnotations")
	types := &jsvalue.Array{}
	for _, typ := range items(typeAnnotations) {
		if IsTSTypeAnnotation(typ) {
			types.Push(getProp(typ, "typeAnnotation"))
		} else {
			types.Push(typ)
		}
	}
	flattened := removeTypeDuplicates(types, typeDuplicatesTS, 0)
	if flattened.Len() == 1 {
		return flattened.At(0)
	}
	return TSUnionType(flattened)
}

// buildChildren is react.buildChildren(node): a JSX element's children as expressions (text cleaned up as JSX does,
// expression containers unwrapped, empty expressions dropped).
func buildChildren(node any) *jsvalue.Array {
	elements := &jsvalue.Array{}
	children := getProp(node, "children")
	for i := 0; float64(i) < jsvalue.ToNumber(getProp(children, "length")); i++ {
		child := getIndex(children, i)
		if IsJSXText(child) {
			cleanJSXElementLiteralChild(child, elements)
			continue
		}
		if IsJSXExpressionContainer(child) {
			child = getProp(child, "expression")
		}
		if IsJSXEmptyExpression(child) {
			continue
		}
		elements.Push(child)
	}
	return elements
}

var (
	jsxLineBreak  = regexp.MustCompile(`\r\n|\n|\r`)
	nonBlank      = regexp.MustCompile(`[^ \t]`)
	leadingSpaces = regexp.MustCompile(`^ +`)
	trailingSpace = regexp.MustCompile(` +$`)
)

// cleanJSXElementLiteralChild pushes the text of a JSXText child as JSX reads it (lines trimmed, joined by spaces)
// onto args, as a string literal that inherits from the child; nothing for blank text.
func cleanJSXElementLiteralChild(child any, args *jsvalue.Array) {
	value := getProp(child, "value")
	s, ok := value.(string)
	if !ok {
		if nullish(value) {
			panic(readError(value, "split"))
		}
		panic(typeError("child.value.split is not a function"))
	}
	lines := jsxLineBreak.Split(s, -1)
	lastNonEmptyLine := 0
	for i, line := range lines {
		if nonBlank.MatchString(line) {
			lastNonEmptyLine = i
		}
	}
	str := ""
	for i, line := range lines {
		isFirstLine := i == 0
		isLastLine := i == len(lines)-1
		isLastNonEmptyLine := i == lastNonEmptyLine
		trimmedLine := strings.ReplaceAll(line, "\t", " ")
		if !isFirstLine {
			trimmedLine = leadingSpaces.ReplaceAllString(trimmedLine, "")
		}
		if !isLastLine {
			trimmedLine = trailingSpace.ReplaceAllString(trimmedLine, "")
		}
		if trimmedLine != "" {
			if !isLastNonEmptyLine {
				trimmedLine += " "
			}
			str += trimmedLine
		}
	}
	if str != "" {
		args.Push(Inherits(StringLiteral(str), child))
	}
}
