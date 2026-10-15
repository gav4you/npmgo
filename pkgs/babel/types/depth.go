// Copyright (c) 2014-present Sebastian McKenzie and other contributors. MIT License (see LICENSE).
// Written for the Go translation of @babel/types 7.29.0 by the npmgo contributors.

package types

import "github.com/gav4you/npmgo/internal/jsvalue"

// maxCallDepth bounds the recursion of the functions that walk a tree (cloneNode, traverse, traverseFast,
// isNodesEquivalent, gatherSequenceExpressions, valueToNode, removeTypeDuplicates and its qualified names). A
// cyclic AST (a node that contains itself) recurses without end: V8 throws a RangeError where its stack runs out,
// which for these functions is at 2,000 to 4,000 levels in Node v25 with its default stack, while a Go stack would
// grow to its 1 GB limit and end the program. The RangeError is raised at this depth instead, as a recoverable panic;
// it is above V8's limit, so every tree the original handles is within it.
const maxCallDepth = 10000

// deeper is the depth of a nested call, or V8's RangeError at the limit.
func deeper(depth int) int {
	if depth >= maxCallDepth {
		panic(&jsvalue.RangeError{Message: "Maximum call stack size exceeded"})
	}
	return depth + 1
}
