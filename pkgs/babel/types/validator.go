// Copyright (c) 2014-present Sebastian McKenzie and other contributors. MIT License (see LICENSE).
// Translated to Go by the npmgo contributors from @babel/types 7.29.0 (lib/definitions/utils.js: the validator
// factories; lib/validators/validate.js).

package types

import (
	"github.com/gav4you/npmgo/internal/jsvalue"
)

// Validator is the validate function of a field definition (NODE_FIELDS[type][field].validate) or a parent
// validation (NODE_PARENT_VALIDATIONS[type]): JavaScript's (node, key, val) => void, which throws when val does not
// fit, together with the properties @babel/types puts on it to describe what it accepts (oneOfNodeTypes, type,
// chainOf, each, oneOf, shapeOf, oneOfNodeOrValueTypes). Its typeof is "function".
type Validator struct {
	name   string
	length int // the JavaScript function's length: how many parameters it declares
	fn     func(node, key, val any)
	props  *jsvalue.Object
}

func newValidator(name string, length int, fn func(node, key, val any), props ...any) *Validator {
	return &Validator{name: name, length: length, fn: fn, props: jsvalue.NewObject(props...)}
}

// JSTypeOf reports "function".
func (v *Validator) JSTypeOf() string { return "function" }

// Name is the JavaScript function's name ("validate", "validator", or "" for an anonymous function).
func (v *Validator) Name() string { return v.name }

// Length is the JavaScript function's length: the number of parameters it declares.
func (v *Validator) Length() int { return v.length }

// Props returns the function's own enumerable properties (the object itself, as JavaScript code can change them).
func (v *Validator) Props() *jsvalue.Object { return v.props }

// Call calls the validator: validate(node, key, val). It panics with the error JavaScript throws.
func (v *Validator) Call(node, key, val any) { v.fn(node, key, val) }

// getType is utils.js's getType: "array", "null" or typeof.
func getType(val any) string {
	if isArray(val) {
		return "array"
	}
	if jsvalue.IsNull(val) {
		return "null"
	}
	return jsvalue.TypeOf(val)
}

// expandedTypes is an entry of utils.js's allExpandedTypes: the types an assertNodeType validator was made with, and
// the Set its node types are expanded into once all definitions are loaded.
type expandedTypes struct {
	types *jsvalue.Array
	set   map[string]bool
}

// assertValueType is utils.js's assertValueType(type).
func (d *definer) assertValueType(typ string) *Validator {
	return newValidator("validate", 3, func(node, key, val any) {
		if getType(val) == typ {
			return
		}
		panic(typeError("Property " + toStr(key) + " expected type of " + typ + " but got " + getType(val)))
	}, "type", typ)
}

// assertNodeType is utils.js's assertNodeType(...types).
func (d *definer) assertNodeType(types ...string) *Validator {
	typesArr := strArray(types)
	expanded := map[string]bool{}
	d.allExpandedTypes = append(d.allExpandedTypes, &expandedTypes{typesArr, expanded})
	return newValidator("validate", 3, func(node, key, val any) {
		var valType any = jsvalue.Undefined{}
		if !nullish(val) {
			valType = getProp(val, "type")
		}
		if !nullish(valType) {
			if s, ok := valType.(string); ok && expanded[s] {
				validateChild(node, key, val)
				return
			}
			if strictEquals(valType, "Placeholder") {
				for _, typ := range items(typesArr) {
					if Is(typ, val) {
						validateChild(node, key, val)
						return
					}
				}
			}
		}
		panic(typeError("Property " + toStr(key) + " of " + toStr(getProp(node, "type")) + " expected node to be of a type " +
			jsonText(typesArr) + " but instead got " + jsonText(valType)))
	}, "oneOfNodeTypes", typesArr)
}

// assertNodeOrValueType is utils.js's assertNodeOrValueType(...types).
func (d *definer) assertNodeOrValueType(types ...string) *Validator {
	typesArr := strArray(types)
	return newValidator("validate", 3, func(node, key, val any) {
		primitiveType := getType(val)
		for _, typ := range items(typesArr) {
			if strictEquals(primitiveType, typ) || Is(typ, val) {
				validateChild(node, key, val)
				return
			}
		}
		var valType any = jsvalue.Undefined{}
		if !nullish(val) {
			valType = getProp(val, "type")
		}
		panic(typeError("Property " + toStr(key) + " of " + toStr(getProp(node, "type")) + " expected node to be of a type " +
			jsonText(typesArr) + " but instead got " + jsonText(valType)))
	}, "oneOfNodeOrValueTypes", typesArr)
}

// assertOneOf is utils.js's assertOneOf(...values).
func (d *definer) assertOneOf(values ...any) *Validator {
	valuesArr := jsvalue.NewArray(values...)
	return d.assertOneOfArray(valuesArr)
}

func (d *definer) assertOneOfArray(valuesArr *jsvalue.Array) *Validator {
	return newValidator("validate", 3, func(node, key, val any) {
		if !valuesArr.Includes(val) {
			panic(typeError("Property " + toStr(key) + " expected value to be one of " + jsonText(valuesArr) + " but got " + jsonText(val)))
		}
	}, "oneOf", valuesArr)
}

// assertEach is utils.js's assertEach(callback): the callback validates each element of an array, under the key
// "key[i]"; with BABEL_TYPES_8_BREAKING the element's parent validation runs too.
func (d *definer) assertEach(callback *Validator) *Validator {
	childValidator := func(node, key, val any) {}
	if d.breaking {
		childValidator = validateChild
	}
	return newValidator("validator", 3, func(node, key, val any) {
		if !isArray(val) {
			return
		}
		i := 0
		subKey := jsvalue.NewObject("toString", func() string { return toStr(key) + "[" + itoa(i) + "]" })
		for ; i < len(items(val)); i++ {
			v := items(val)[i]
			callback.fn(node, subKey, v)
			childValidator(node, subKey, v)
		}
	}, "each", callback)
}

// assertShape is utils.js's assertShape(shape): each property of the shape is a field definition of val's property;
// the TypeErrors they raise are collected into one.
func (d *definer) assertShape(shape *jsvalue.Object) *Validator {
	keys := shape.Keys()
	return newValidator("validate", 3, func(node, key, val any) {
		var errs []string
		for _, property := range keys {
			msg, failed := catchTypeError(func() {
				validateField(node, property, getProp(val, property), getProp(shape, property))
			})
			if failed {
				errs = append(errs, msg)
			}
		}
		if len(errs) > 0 {
			text := ""
			for i, e := range errs {
				if i > 0 {
					text += "\n"
				}
				text += e
			}
			panic(typeError("Property " + toStr(key) + " of " + toStr(getProp(node, "type")) + " expected to have the following:\n" + text))
		}
	}, "shapeOf", shape)
}

// catchTypeError runs f; a TypeError it throws is caught and its message returned, anything else propagates.
func catchTypeError(f func()) (msg string, failed bool) {
	defer func() {
		if r := recover(); r != nil {
			if te, ok := r.(*jsvalue.TypeError); ok {
				msg, failed = te.Message, true
				return
			}
			panic(r)
		}
	}()
	f()
	return "", false
}

// assertOptionalChainStart is utils.js's assertOptionalChainStart().
func (d *definer) assertOptionalChainStart() *Validator {
	return newValidator("validate", 1, func(node, key, val any) {
		current := node
		for truthy(node) {
			if nullish(current) {
				what := "null"
				if jsvalue.IsUndefined(current) {
					what = "undefined"
				}
				panic(typeError("Cannot destructure property 'type' of 'current' as it is " + what + "."))
			}
			typ := getProp(current, "type")
			if strictEquals(typ, "OptionalCallExpression") {
				if truthy(getProp(current, "optional")) {
					return
				}
				current = getProp(current, "callee")
				continue
			}
			if strictEquals(typ, "OptionalMemberExpression") {
				if truthy(getProp(current, "optional")) {
					return
				}
				current = getProp(current, "object")
				continue
			}
			break
		}
		var found any = jsvalue.Undefined{}
		if !nullish(current) {
			found = getProp(current, "type")
		}
		panic(typeError("Non-optional " + toStr(getProp(node, "type")) +
			" must chain from an optional OptionalMemberExpression or OptionalCallExpression. Found chain from " + toStr(found)))
	})
}

// chain is utils.js's chain(...fns): the validators run in order.
func (d *definer) chain(fns ...*Validator) *Validator {
	list := &jsvalue.Array{}
	for _, f := range fns {
		list.Push(f)
	}
	v := newValidator("validate", 0, func(node, key, val any) {
		for _, f := range items(list) {
			callValue(f, "fn", node, key, val)
		}
	}, "chainOf", list)
	if len(fns) >= 2 {
		first := fns[0].props
		if first.Has("type") && strictEquals(first.Get("type"), "array") && !fns[1].props.Has("each") {
			panic(&jsvalue.Error{Message: `An assertValueType("array") validator can only be followed by an assertEach(...) validator.`})
		}
	}
	return v
}

// --- lib/validators/validate.js

// Validate is validate(node, key, val): it checks val against the definition of the field key of node's type, and
// val's parent validation, panicking with the error JavaScript throws.
func Validate(node, key, val any) {
	if !truthy(node) {
		return
	}
	fields := tableGet(NODE_FIELDS, getProp(node, "type"))
	if !truthy(fields) {
		return
	}
	var field any = jsvalue.Undefined{}
	if o, ok := fields.(*jsvalue.Object); ok {
		if k, ok := propertyKey(key); ok {
			field = getProp(o, k)
		}
	}
	// A type name that is one of Object.prototype's (fields is then a function, or Object.prototype) finds a field
	// without a validate property, as in JavaScript.
	validateField(node, key, val, field)
	validateChild(node, key, val)
}

// fieldValidate reads field.validate for validateInternal and validateField: ok false where the field is null or
// undefined or its validate is falsy.
func fieldValidate(field any) (any, bool) {
	if nullish(field) {
		return nil, false
	}
	v := getProp(field, "validate")
	return v, truthy(v)
}

// validateInternal is validate.js's validateInternal(field, node, key, val, maybeNode), which the builders call.
func validateInternal(field any, node *jsvalue.Object, key string, val any, maybeNode bool) {
	validate, ok := fieldValidate(field)
	if !ok {
		return
	}
	if truthy(getProp(field, "optional")) && nullish(val) {
		return
	}
	callValue(validate, "field.validate", node, key, val)
	if maybeNode {
		typ := getProp(val, "type")
		if nullish(typ) {
			return
		}
		callParentValidation(tableGet(NODE_PARENT_VALIDATIONS, typ), "_NODE_PARENT_VALIDATI", node, key, val)
	}
}

// validateField is validate.js's validateField(node, key, val, field).
func validateField(node, key, val, field any) {
	validate, ok := fieldValidate(field)
	if !ok {
		return
	}
	if truthy(getProp(field, "optional")) && nullish(val) {
		return
	}
	callValue(validate, "field.validate", node, key, val)
}

// validateChild is validate.js's validateChild(node, key, val): the parent validation of val's type.
func validateChild(node, key, val any) {
	var typ any = jsvalue.Undefined{}
	if !nullish(val) {
		typ = getProp(val, "type")
	}
	if nullish(typ) {
		return
	}
	callParentValidation(tableGet(NODE_PARENT_VALIDATIONS, typ), "_NODE_PARENT_VALIDATI2", node, key, val)
}

// callParentValidation is `NODE_PARENT_VALIDATIONS[type]?.call(NODE_PARENT_VALIDATIONS, node, key, val)`, tmp being
// the name of the temporary the compiled code reads it into (V8 names it in its TypeError). A type name that is one of
// Object.prototype's calls that built-in with NODE_PARENT_VALIDATIONS as this.
func callParentValidation(entry any, tmp string, node, key, val any) {
	switch e := entry.(type) {
	case jsvalue.Undefined, nil:
		return
	case *Validator:
		e.fn(node, key, val)
		return
	case builtin:
		switch e.name {
		case "__defineGetter__", "__defineSetter__":
			// Object.prototype.__defineGetter__(node, key): key is never callable.
			if jsvalue.TypeOf(key) != "function" {
				panic(typeError("Object.prototype." + e.name + ": Expecting function"))
			}
		}
		return // the other built-ins return without throwing
	case *objectPrototypeValue:
		panic(typeError(tmp + ".call is not a function"))
	}
	callValue(entry, tmp+".call", node, key, val)
}
