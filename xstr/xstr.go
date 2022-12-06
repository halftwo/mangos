// Package xstr implements some helpfule string manipulation functions
package xstr

import (
	"fmt"
	"strings"
)

func BytesRightCopy(to []byte, from []byte) {
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

func SplitKeyValue(s string, sep string) (key, value string, err error) {
	i := strings.Index(s, sep)
	if i < 0 {
		err = fmt.Errorf("No seperator \"%s\" found in the string", sep)
	} else {
		key = strings.TrimSpace(s[:i])
		value = strings.TrimSpace(s[i+len(sep):])
	}
	return
}

func Split2(s string, sep string) (left, right string) {
	i := strings.Index(s, sep)
	if i < 0 {
		left = s
	} else {
		left = s[:i]
		right = s[i+len(sep):]
	}
	return
}

func SuffixIndexByte(str string, c byte) string {
	k := strings.LastIndexByte(str, c)
	if k < 0 {
		return str
	}
	return str[k+1:]
}

func SuffixIndexByteN(str string, c byte, n int) string {
	if n <= 0 {
		return ""
	}

	var k int
	s := str
	for i := 0; i < n; i++ {
		k = strings.LastIndexByte(s, c)
		if k < 0 {
			return str
		}
		s = s[:k]
	}
	return str[k+1:]
}

func PrefixIndexByte(str string, c byte) string {
	k := strings.IndexByte(str, c)
	if k < 0 {
		return str
	}
	return str[:k-1]
}

func PrefixIndexByteN(str string, c byte, n int) string {
	if n <= 0 {
		return ""
	}

	m := 0
	s := str
	for i := 0; i < n; i++ {
		k := strings.IndexByte(s, c)
		if k < 0 {
			return str
		}
		m += k + 1
		s = str[m:]
	}
	return str[:m-1]
}

