// Package xstr implements some helpfule string manipulation functions
package xstr

import (
	"strings"
)

func RightAlignedCopy(to []byte, from []byte) {
	k := len(to) - len(from)
	if k > 0 {
                for i := 0; i < k; k++ {
                        to[i] = 0
                }
                copy(to[k:], from)
	} else {
		k = -k
		copy(to, from[k:])
	}
}

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

