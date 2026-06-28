package engine

import "strings"

// splitArgs splits a command-line argument string into fields, honoring single
// and double quotes so values containing spaces are preserved. It is a small,
// dependency-free shell-word splitter and does not perform variable, glob, or
// backslash-escape expansion.
func splitArgs(s string) []string {
	var args []string
	var cur strings.Builder
	inSingle := false
	inDouble := false
	hasToken := false

	flush := func() {
		if hasToken {
			args = append(args, cur.String())
			cur.Reset()
			hasToken = false
		}
	}

	for _, c := range s {
		switch {
		case inSingle:
			if c == '\'' {
				inSingle = false
			} else {
				cur.WriteRune(c)
			}
		case inDouble:
			if c == '"' {
				inDouble = false
			} else {
				cur.WriteRune(c)
			}
		case c == '\'':
			inSingle = true
			hasToken = true
		case c == '"':
			inDouble = true
			hasToken = true
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			flush()
		default:
			cur.WriteRune(c)
			hasToken = true
		}
	}
	flush()

	return args
}
