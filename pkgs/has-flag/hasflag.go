// Package hasflag is a Go translation of the npm package has-flag@4.0.0
// (https://github.com/sindresorhus/has-flag): whether a command line holds a
// flag. The original copyright notice and MIT license are preserved in LICENSE.
package hasflag

import (
	"os"
	"slices"
	"strings"
	"unicode/utf16"
)

// HasFlag is hasFlag(flag, argv): whether argv holds the flag before any "--"
// terminator. A flag that starts with "-" is looked for as it is; any other is
// prefixed with "--", or with "-" when it is one UTF-16 code unit long
// (flag.length === 1).
func HasFlag(flag string, argv []string) bool {
	prefix := "--"
	switch {
	case strings.HasPrefix(flag, "-"):
		prefix = ""
	case len(utf16.Encode([]rune(flag))) == 1:
		prefix = "-"
	}
	position := slices.Index(argv, prefix+flag)
	terminatorPosition := slices.Index(argv, "--")
	return position != -1 && (terminatorPosition == -1 || position < terminatorPosition)
}

// Has is hasFlag(flag) with its default argv, process.argv: it looks in
// os.Args. (process.argv starts with the node executable and the script,
// os.Args with the program; only the arguments after them carry flags.)
func Has(flag string) bool { return HasFlag(flag, os.Args) }
