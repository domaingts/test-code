package permission

import (
	"regexp"
	"strings"
)

// MatchWildcard matches a pattern (containing * wildcards) against an input string.
//
// Rules:
//   - \* is a literal asterisk (escape)
//   - \\ is a literal backslash (escape)
//   - * matches any sequence of characters (including empty)
//   - If the pattern ends with " *" and has exactly one wildcard,
//     the trailing space+wildcard is made optional (so "git *" matches "git" too)
func MatchWildcard(pattern, input string) bool {
	re := wildcardToRegex(pattern)
	return re.MatchString(input)
}

// wildcardToRegex converts a wildcard pattern to a compiled regex.
func wildcardToRegex(pattern string) *regexp.Regexp {
	var buf strings.Builder
	buf.WriteString("^")

	// Count wildcards for the trailing-optional case
	wildcardCount := 0
	for _, ch := range pattern {
		if ch == '*' {
			wildcardCount++
		}
	}

	// Check for trailing " *" pattern with single wildcard
	trailingOptional := false
	if wildcardCount == 1 && len(pattern) >= 2 && pattern[len(pattern)-1] == '*' && pattern[len(pattern)-2] == ' ' {
		trailingOptional = true
		// Remove the trailing " *" — we'll handle it specially
		pattern = pattern[:len(pattern)-2]
	}

	i := 0
	for i < len(pattern) {
		ch := pattern[i]
		switch ch {
		case '\\':
			// Escape sequence
			if i+1 < len(pattern) {
				next := pattern[i+1]
				switch next {
				case '*':
					buf.WriteString(regexp.QuoteMeta("*"))
					i++
				case '\\':
					buf.WriteString(regexp.QuoteMeta("\\"))
					i++
				default:
					buf.WriteString(regexp.QuoteMeta(string(ch)))
				}
			} else {
				buf.WriteString(regexp.QuoteMeta("\\"))
			}
		case '*':
			buf.WriteString(".*")
		default:
			buf.WriteString(regexp.QuoteMeta(string(ch)))
		}
		i++
	}

	if trailingOptional {
		buf.WriteString("( .*)?")
	}

	buf.WriteString("$")
	return regexp.MustCompile("(?s)" + buf.String())
}
