package utils

import "strings"

// FirstUpper transfer first character to upper case
func FirstUpper(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
