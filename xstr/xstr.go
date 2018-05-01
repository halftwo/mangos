// Package xstr implements some helpfule string manipulation functions
package xstr

import (
	"strings"
)


func SplitKeyValue(s string, sep string) (key, value string) {
	ss := strings.SplitN(s, sep, 2)
	if len(ss) == 1 {
		return ss[0], ""
	}
	return ss[0], ss[1]
}

