// Copyright (c) 2014-present Sebastian McKenzie and other contributors. MIT License (see LICENSE).
// Translated to Go by the npmgo contributors from @babel/types 7.29.0 (lib/definitions/utils.js: defineType and
// defineAliasedType; lib/definitions/placeholders.js; lib/definitions/deprecated-aliases.js; lib/definitions/index.js;
// lib/index.js's load-time warning).

package types

import (
	"os"

	"github.com/gav4you/npmgo/internal/jsvalue"
)

// The definition tables, built when the package is initialised, as lib/definitions builds them when it is loaded.
// They are JavaScript's plain objects and arrays (*jsvalue.Object, *jsvalue.Array), shared the way JavaScript shares
// them (BUILDER_KEYS[t] is VISITOR_KEYS[t] where a type defines no builder, a type that inherits another shares its
// arrays, PLACEHOLDERS_ALIAS.Identifier is ALIAS_KEYS.Identifier, ...). Like JavaScript's, they can be changed; the
// package reads them when it uses them. Changing them while other goroutines use the package is a data race.
var (
	VISITOR_KEYS               *jsvalue.Object // node type → the keys of its child nodes, in visiting order
	ALIAS_KEYS                 *jsvalue.Object // node type → its aliases
	FLIPPED_ALIAS_KEYS         *jsvalue.Object // alias → its node types
	NODE_FIELDS                *jsvalue.Object // node type → field → {validate, optional, default, deprecated}
	BUILDER_KEYS               *jsvalue.Object // node type → the parameters of its builder
	DEPRECATED_KEYS            *jsvalue.Object // deprecated node type → its replacement
	NODE_PARENT_VALIDATIONS    *jsvalue.Object // node type → the validation it applies to its parent
	NODE_UNION_SHAPES__PRIVATE *jsvalue.Object // node type → the shapes of a union (internal to Babel)
	PLACEHOLDERS               *jsvalue.Array  // the node types a Placeholder can stand for
	PLACEHOLDERS_ALIAS         *jsvalue.Object // placeholder type → the aliases it also matches
	PLACEHOLDERS_FLIPPED_ALIAS *jsvalue.Object // alias → the placeholder types that match it
	DEPRECATED_ALIASES         *jsvalue.Object // deprecated alias → its replacement
	TYPES                      *jsvalue.Array  // every node type, alias and deprecated type name
)

// breaking8 records whether BABEL_TYPES_8_BREAKING was set when the package was initialised.
var breaking8 bool

func init() {
	breaking8 = os.Getenv("BABEL_TYPES_8_BREAKING") != ""
	d := loadDefinitions(breaking8)
	VISITOR_KEYS, ALIAS_KEYS, FLIPPED_ALIAS_KEYS = d.VISITOR_KEYS, d.ALIAS_KEYS, d.FLIPPED_ALIAS_KEYS
	NODE_FIELDS, BUILDER_KEYS, DEPRECATED_KEYS = d.NODE_FIELDS, d.BUILDER_KEYS, d.DEPRECATED_KEYS
	NODE_PARENT_VALIDATIONS, NODE_UNION_SHAPES__PRIVATE = d.NODE_PARENT_VALIDATIONS, d.NODE_UNION_SHAPES__PRIVATE
	PLACEHOLDERS, PLACEHOLDERS_ALIAS, PLACEHOLDERS_FLIPPED_ALIAS = d.PLACEHOLDERS, d.PLACEHOLDERS_ALIAS, d.PLACEHOLDERS_FLIPPED_ALIAS
	DEPRECATED_ALIASES, TYPES = d.DEPRECATED_ALIASES, d.TYPES
	setAliasConstants()
	if breaking8 {
		ConsoleWarn("BABEL_TYPES_8_BREAKING is not supported anymore. Use the latest Babel 8.0.0 pre-release instead!")
	}
}

// loadDefinitions runs lib/definitions: every definition, in load order, then index.js's last steps.
func loadDefinitions(breaking bool) *definer {
	d := newDefiner(breaking)
	if breaking {
		defineBreaking8(d)
	} else {
		defineDefault(d)
	}
	d.finish()
	return d
}

// definer holds what lib/definitions/utils.js keeps in its module scope while the definitions load.
type definer struct {
	breaking bool // process.env.BABEL_TYPES_8_BREAKING

	VISITOR_KEYS, ALIAS_KEYS, FLIPPED_ALIAS_KEYS, NODE_FIELDS, BUILDER_KEYS, DEPRECATED_KEYS *jsvalue.Object
	NODE_PARENT_VALIDATIONS, NODE_UNION_SHAPES__PRIVATE                                      *jsvalue.Object
	PLACEHOLDERS                                                                             *jsvalue.Array
	PLACEHOLDERS_ALIAS, PLACEHOLDERS_FLIPPED_ALIAS, DEPRECATED_ALIASES                       *jsvalue.Object
	TYPES                                                                                    *jsvalue.Array

	store            *jsvalue.Object // type → the options it was defined with
	allExpandedTypes []*expandedTypes
}

func newDefiner(breaking bool) *definer {
	return &definer{
		breaking:                   breaking,
		VISITOR_KEYS:               jsvalue.NewObject(),
		ALIAS_KEYS:                 jsvalue.NewObject(),
		FLIPPED_ALIAS_KEYS:         jsvalue.NewObject(),
		NODE_FIELDS:                jsvalue.NewObject(),
		BUILDER_KEYS:               jsvalue.NewObject(),
		DEPRECATED_KEYS:            jsvalue.NewObject(),
		NODE_PARENT_VALIDATIONS:    jsvalue.NewObject(),
		NODE_UNION_SHAPES__PRIVATE: jsvalue.NewObject(),
		store:                      jsvalue.NewObject(),
	}
}

var validTypeOpts = map[string]bool{
	"aliases": true, "builder": true, "deprecatedAlias": true, "fields": true, "inherits": true, "visitor": true,
	"validate": true, "unionShape": true,
}

var validFieldKeys = map[string]bool{"default": true, "optional": true, "deprecated": true, "validate": true}

// aliased is defineAliasedType(...aliases)(type, opts): the aliases are put in front of the type's own (or inherited)
// aliases, where missing, and the type is defined.
func (d *definer) aliased(aliases []string, typ string, opts *jsvalue.Object) {
	defined := getProp(opts, "aliases")
	if !truthy(defined) {
		if inh := getProp(opts, "inherits"); truthy(inh) {
			defined = jsvalue.Undefined{}
			parentAliases := getProp(tableGet(d.store, inh), "aliases")
			if !nullish(parentAliases) {
				defined = jsvalue.NewArray(items(parentAliases)...) // .slice()
			}
		}
		if nullish(defined) {
			defined = jsvalue.NewArray()
		}
		setProp(opts, "aliases", defined)
	}
	var additional []any
	for _, a := range aliases {
		if !includes(defined, a, "defined") {
			additional = append(additional, a)
		}
	}
	arr, ok := defined.(*jsvalue.Array)
	if !ok {
		panic(typeError("defined.unshift is not a function"))
	}
	arr.Unshift(additional...)
	d.defineType(typ, opts)
}

// defineType is utils.js's defineType(type, opts).
func (d *definer) defineType(typ string, opts *jsvalue.Object) {
	var inherits any = jsvalue.NewObject()
	if inh := getProp(opts, "inherits"); truthy(inh) {
		if stored := tableGet(d.store, inh); truthy(stored) {
			inherits = stored
		}
	}
	fields := getProp(opts, "fields")
	if !truthy(fields) {
		newFields := jsvalue.NewObject()
		if inheritedFields := getProp(inherits, "fields"); truthy(inheritedFields) {
			for _, key := range objectKeys(inheritedFields) {
				field := getProp(inheritedFields, key)
				def := getProp(field, "default")
				if isArray(def) && len(items(def)) > 0 || !isArray(def) && truthy(def) && jsvalue.TypeOf(def) == "object" {
					panic(&jsvalue.Error{Message: "field defaults can only be primitives or empty arrays currently"})
				}
				newDef := def
				if isArray(def) {
					newDef = jsvalue.NewArray()
				}
				newFields.Set(key, jsvalue.NewObject(
					"default", newDef,
					"optional", getProp(field, "optional"),
					"deprecated", getProp(field, "deprecated"),
					"validate", getProp(field, "validate"),
				))
			}
		}
		fields = newFields
	}
	visitor := getProp(opts, "visitor")
	if !truthy(visitor) {
		visitor = getProp(inherits, "visitor")
	}
	if !truthy(visitor) {
		visitor = jsvalue.NewArray()
	}
	aliases := getProp(opts, "aliases")
	if !truthy(aliases) {
		aliases = getProp(inherits, "aliases")
	}
	if !truthy(aliases) {
		aliases = jsvalue.NewArray()
	}
	builder := getProp(opts, "builder")
	if !truthy(builder) {
		builder = getProp(inherits, "builder")
	}
	if !truthy(builder) {
		builder = getProp(opts, "visitor")
	}
	if !truthy(builder) {
		builder = jsvalue.NewArray()
	}
	for _, k := range objectKeys(opts) {
		if !validTypeOpts[k] {
			panic(&jsvalue.Error{Message: `Unknown type option "` + k + `" on ` + typ})
		}
	}
	if dep := getProp(opts, "deprecatedAlias"); truthy(dep) {
		if k, ok := propertyKey(dep); ok {
			d.DEPRECATED_KEYS.Set(k, typ)
		}
	}
	for _, key := range append(append([]any{}, items(visitor)...), items(builder)...) {
		k := toStr(key)
		if !truthy(getProp(fields, k)) {
			setProp(fields, k, jsvalue.NewObject())
		}
	}
	for _, key := range objectKeys(fields) {
		field := getProp(fields, key)
		if !jsvalue.IsUndefined(getProp(field, "default")) && !includes(builder, key, "builder") {
			setProp(field, "optional", true)
		}
		if def := getProp(field, "default"); jsvalue.IsUndefined(def) {
			setProp(field, "default", nil)
		} else if !truthy(getProp(field, "validate")) && !nullish(def) {
			setProp(field, "validate", d.assertValueType(getType(def)))
		}
		for _, k := range objectKeys(field) {
			if !validFieldKeys[k] {
				panic(&jsvalue.Error{Message: `Unknown field key "` + k + `" on ` + typ + "." + key})
			}
		}
	}
	setProp(opts, "visitor", visitor)
	d.VISITOR_KEYS.Set(typ, visitor)
	setProp(opts, "builder", builder)
	d.BUILDER_KEYS.Set(typ, builder)
	setProp(opts, "fields", fields)
	d.NODE_FIELDS.Set(typ, fields)
	setProp(opts, "aliases", aliases)
	d.ALIAS_KEYS.Set(typ, aliases)
	for _, alias := range items(aliases) {
		a := toStr(alias)
		if !truthy(tableGet(d.FLIPPED_ALIAS_KEYS, a)) {
			d.FLIPPED_ALIAS_KEYS.Set(a, jsvalue.NewArray())
		}
		list, ok := d.FLIPPED_ALIAS_KEYS.Get(a).(*jsvalue.Array)
		if !ok {
			panic(typeError("FLIPPED_ALIAS_KEYS[alias].push is not a function"))
		}
		list.Push(typ)
	}
	if v := getProp(opts, "validate"); truthy(v) {
		d.NODE_PARENT_VALIDATIONS.Set(typ, v)
	}
	if v := getProp(opts, "unionShape"); truthy(v) {
		d.NODE_UNION_SHAPES__PRIVATE.Set(typ, v)
	}
	d.store.Set(typ, opts)
}

// placeholders is lib/definitions/placeholders.js, run when definitions/misc.js first loads it.
func (d *definer) placeholders() {
	d.PLACEHOLDERS = jsvalue.NewArray("Identifier", "StringLiteral", "Expression", "Statement", "Declaration", "BlockStatement", "ClassBody", "Pattern")
	d.PLACEHOLDERS_ALIAS = jsvalue.NewObject(
		"Declaration", jsvalue.NewArray("Statement"),
		"Pattern", jsvalue.NewArray("PatternLike", "LVal"),
	)
	for _, typ := range items(d.PLACEHOLDERS) {
		alias := tableGet(d.ALIAS_KEYS, typ)
		if !nullish(alias) && truthy(getProp(alias, "length")) {
			d.PLACEHOLDERS_ALIAS.Set(toStr(typ), alias)
		}
	}
	d.PLACEHOLDERS_FLIPPED_ALIAS = jsvalue.NewObject()
	for _, typ := range d.PLACEHOLDERS_ALIAS.Keys() {
		for _, alias := range items(d.PLACEHOLDERS_ALIAS.Get(typ)) {
			a := toStr(alias)
			if !d.PLACEHOLDERS_FLIPPED_ALIAS.Has(a) {
				d.PLACEHOLDERS_FLIPPED_ALIAS.Set(a, jsvalue.NewArray())
			}
			d.PLACEHOLDERS_FLIPPED_ALIAS.Get(a).(*jsvalue.Array).Push(typ)
		}
	}
}

// finish is the end of lib/definitions/index.js: the deprecated aliases share their replacement's list, every
// assertNodeType validator's types are expanded to node types, and TYPES is assembled.
func (d *definer) finish() {
	d.DEPRECATED_ALIASES = jsvalue.NewObject("ModuleDeclaration", "ImportOrExportDeclaration")
	for _, deprecatedAlias := range d.DEPRECATED_ALIASES.Keys() {
		d.FLIPPED_ALIAS_KEYS.Set(deprecatedAlias, tableGet(d.FLIPPED_ALIAS_KEYS, d.DEPRECATED_ALIASES.Get(deprecatedAlias)))
	}
	for _, e := range d.allExpandedTypes {
		for _, typ := range items(e.types) {
			aliases := tableGet(d.FLIPPED_ALIAS_KEYS, typ)
			if truthy(aliases) {
				for _, a := range items(aliases) {
					if s, ok := a.(string); ok {
						e.set[s] = true
					}
				}
			} else if s, ok := typ.(string); ok {
				e.set[s] = true
			}
		}
	}
	types := &jsvalue.Array{}
	for _, table := range []*jsvalue.Object{d.VISITOR_KEYS, d.FLIPPED_ALIAS_KEYS, d.DEPRECATED_KEYS} {
		for _, k := range table.Keys() {
			types.Push(k)
		}
	}
	d.TYPES = types
}

// flippedAliasArray is FLIPPED_ALIAS_KEYS[alias], for the alias constants.
func flippedAliasArray(alias string) *jsvalue.Array {
	a, _ := FLIPPED_ALIAS_KEYS.Get(alias).(*jsvalue.Array)
	return a
}
