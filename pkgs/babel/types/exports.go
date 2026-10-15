// Copyright (c) 2014-present Sebastian McKenzie and other contributors. MIT License (see LICENSE).
// Written for the Go translation of @babel/types 7.29.0 by the npmgo contributors.

package types

// exportEntry is an export of the main entry point: its JavaScript name, the kind of its value ("function",
// "object", "array" or "symbol") and its Go value (a function itself; for other kinds a function that reads the
// package variable).
type exportEntry struct {
	name  string
	kind  string
	value any
}

// ExportNames returns the names of the exports of require("@babel/types"), in the order Object.keys lists them.
func ExportNames() []string {
	names := make([]string, len(exportTable))
	for i, e := range exportTable {
		names[i] = e.name
	}
	return names
}

// Export is t[name] for an export of @babel/types: its Go value (a function, or the current value of a variable),
// for code that looks exports up by name, as JavaScript code does with t["is" + type].
func Export(name string) (any, bool) {
	for _, e := range exportTable {
		if e.name == name {
			if e.kind == "function" {
				return e.value, true
			}
			return e.value.(func() any)(), true
		}
	}
	return nil, false
}
