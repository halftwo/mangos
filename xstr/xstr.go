// Package xstr implements some helpfule string manipulation functions
package xstr

import (
	"strings"
)


func SplitKeyValue(s string, sep string) (key, value string) {
	i := strings.Index(s, sep)
	if i < 0 {
		key = s
	} else {
		key = s[:i]
		value = s[i+1:]
	}
	return
}

