package shlex

import (
	"regexp"
	"strings"
)

func Join(splitCommand []string) string {
	// Return a shell-escaped string from splitCommand.
	return strings.Join(QuoteArg(splitCommand), " ")
}

var unsafeChar = regexp.MustCompile(`[^\w@%+=:,./-]`)

func Quote(s string) string {
	// Return a shell-escaped version of the string s.
	if len(s) == 0 {
		return "''"
	}
	if !unsafeChar.MatchString(s) {
		return s
	}

	// Use single quotes, and put single quotes into double quotes.
	// The string $'b is then quoted as '$'"'"'b'.
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

func QuoteArg(args []string) []string {
	// Quote each argument and return the result.
	var result []string
	for _, arg := range args {
		result = append(result, Quote(arg))
	}
	return result
}
