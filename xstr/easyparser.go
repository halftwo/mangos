package xstr

import (
	"io"
	"strconv"
	"strings"
)

type EasyParser struct {
	Str string
	Err error
}

func NewEasyParser(s string) *EasyParser {
	return &EasyParser{Str:s}
}

// value,
// sep=","
func (ep *EasyParser) Next(sep string) (value string) {
	if ep.Err != nil {
		return
	}

	if ep.Str == "" {
		ep.Err = io.EOF
		return
	}

	k := -1
	if sep != "" {
		k = strings.Index(ep.Str, sep)
	}

	if k < 0 {
		value = ep.Str
		ep.Str = ""
	} else {
		value = ep.Str[:k]
		ep.Str = ep.Str[k+len(sep):]
	}
	return
}

// equals ep.TrimSapce(ep.Next(sep))
func (ep *EasyParser) NextTrim(sep string) (value string) {
	value = ep.Next(sep)
	return strings.TrimSpace(value)
}

// Base 10 integer
func (ep *EasyParser) NextInt(sep string) (value int) {
	value, ep.Err = strconv.Atoi(ep.NextTrim(sep))
	return
}

// If the base argument is 0, the true base is implied by the string's prefix
// following the sign (if present): base 2 for "0b", 8 for "0" or "0o", 
// 16 for "0x", and 10 for otherwise.
func (ep *EasyParser) NextInt64(sep string, base int) (value int64) {
	value, ep.Err = strconv.ParseInt(ep.NextTrim(sep), base, 64)
	return
}

// If the base argument is 0, the true base is implied by the string's prefix
// following the sign (if present): base 2 for "0b", 8 for "0" or "0o", 
// 16 for "0x", and 10 for otherwise.
func (ep *EasyParser) NextUint64(sep string, base int) (value uint64) {
	value, ep.Err = strconv.ParseUint(ep.NextTrim(sep), base, 64)
	return
}

func (ep *EasyParser) NextFloat(sep string) (value float64) {
	value, ep.Err = strconv.ParseFloat(ep.NextTrim(sep), 64)
	return
}

// value:extra,
// sep1=":" sep0=","
func (ep *EasyParser) NextExtra(sep1, sep0 string) (value, extra string) {
	if ep.Err != nil {
		return
	}

	if ep.Str == "" {
		ep.Err = io.EOF
		return
	}

	s := ep.Next(sep0)
	k := strings.Index(s, sep1)
	if k < 0 {
		value = s
	} else {
		value = s[:k]
		extra = s[k+len(sep1):]
	}
	return
}

func (ep *EasyParser) NextExtraTrim(sep1, sep0 string) (value, extra string) {
	value, extra = ep.NextExtraTrim(sep1, sep0)
	value = strings.TrimSpace(value)
	extra = strings.TrimSpace(extra)
	return
}

// value(quote),
// left="("  right="),"  sep=","
func (ep *EasyParser) NextQuote(left, right, sep string) (value, quote string) {
	if ep.Err != nil {
		return
	}

	if ep.Str == "" {
		ep.Err = io.EOF
		return
	}

	k := -1
	if sep != "" {
		k = strings.Index(ep.Str, sep)
	}

	i := strings.Index(ep.Str, left)
	if i >= 0 && (k < 0 || i < k) {
		temp := ep.Str[i+len(left):]
		j := strings.Index(temp, right)
		if j >= 0 {
			value = ep.Str[:i]
			quote = temp[:j]
			ep.Str = temp[j+len(right):]
			return
		}
	}

	if k < 0 {
		value = ep.Str
		ep.Str = ""
	} else {
		value = ep.Str[:k]
		ep.Str = ep.Str[k+len(sep):]
	}
	return
}

func (ep *EasyParser) NextQuoteTrim(left, right, sep string) (value, quote string) {
	value, quote = ep.NextQuoteTrim(left, right, sep)
	value = strings.TrimSpace(value)
	quote = strings.TrimSpace(quote)
	return
}

