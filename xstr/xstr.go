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

func IndexNotByte(s []byte, c byte) int {
	for i, b := range s {
		if b != c {
			return i
		}
	}
	return -1
}

func IndexNotInBytes(s []byte, set []byte) int {
	if len(set) == 1 {
		return IndexNotByte(s, set[0])
	}

again:
	for i, b := range s {
		for _, c := range set {
			if b == c {
				continue again
			}
		}
		return i
	}
	return -1
}

func LastIndexNotByte(s []byte, c byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] != c {
			return i
		}
	}
	return -1
}

func LastIndexNotInBytes(s []byte, set []byte) int {
	if len(set) == 1 {
		return LastIndexNotByte(s, set[0])
	}

again:
	for i := len(s) - 1; i >= 0; i-- {
		b := s[i]
		for _, c := range set {
			if b == c {
				continue again
			}
		}
		return i
	}
	return -1
}

