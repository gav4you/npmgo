package chalk

// source/util.js.

import "strings"

// stringReplaceAll is util.js's stringReplaceAll: every occurrence of
// substring followed by replacer (the substring itself is kept). substring
// is a non-empty close code.
func stringReplaceAll(s, substring, replacer string) string {
	index := strings.Index(s, substring)
	if index == -1 {
		return s
	}
	substringLength := len(substring)
	endIndex := 0
	var b strings.Builder
	for {
		b.WriteString(s[endIndex:index])
		b.WriteString(substring)
		b.WriteString(replacer)
		endIndex = index + substringLength
		next := strings.Index(s[endIndex:], substring)
		if next == -1 {
			break
		}
		index = endIndex + next
	}
	b.WriteString(s[endIndex:])
	return b.String()
}

// stringEncaseCRLFWithFirstIndex is util.js's
// stringEncaseCRLFWithFirstIndex: every line break, starting with the one at
// index, gets prefix before it and postfix after it; a CR before the LF
// stays with it ("\r\n").
func stringEncaseCRLFWithFirstIndex(s, prefix, postfix string, index int) string {
	endIndex := 0
	var b strings.Builder
	for {
		gotCR := index > 0 && s[index-1] == '\r'
		end := index
		if gotCR {
			end--
		}
		b.WriteString(s[endIndex:end])
		b.WriteString(prefix)
		if gotCR {
			b.WriteString("\r\n")
		} else {
			b.WriteString("\n")
		}
		b.WriteString(postfix)
		endIndex = index + 1
		next := strings.IndexByte(s[endIndex:], '\n')
		if next == -1 {
			break
		}
		index = endIndex + next
	}
	b.WriteString(s[endIndex:])
	return b.String()
}
