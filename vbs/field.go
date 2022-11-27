package vbs

import (
	"strings"
	"unicode"
	"reflect"
	"sort"
	"sync"
	"strconv"
	"math"
	"bytes"
)

// _TagOptions is the string following a comma in a struct field's "json"
// tag, or the empty string. It does not include the leading comma.
type _TagOptions string

// parseTag splits a struct field's json tag into its name and
// comma-separated options.
func parseTag(tag string) (string, _TagOptions) {
	if idx := strings.Index(tag, ","); idx != -1 {
		return tag[:idx], _TagOptions(tag[idx+1:])
	}
	return tag, _TagOptions("")
}

// Contains reports whether a comma-separated list of options
// contains a particular substr flag. substr must be surrounded by a
// string boundary or commas.   
func (o _TagOptions) Contains(optionName string) bool {
	if len(o) == 0 {
		return false
	}
	s := string(o)
	for s != "" {
		var next string
		i := strings.Index(s, ",")
		if i >= 0 {
			s, next = s[:i], s[i+1:]
		}
		if s == optionName {
			return true
		}
		s = next
	}
	return false
}

func isValidName(s string) bool {
	for _, c := range s {
		switch {
		case strings.ContainsRune("!#$%&()*+-./:<=>?@[]^_{|}~ ", c):
			// Backslash and quote chars are reserved, but
			// otherwise any punctuation chars are allowed
			// in a tag name.
		default:
			if !unicode.IsLetter(c) && !unicode.IsDigit(c) {
				return false
			}
		}
	}
	return true
}

type _FieldInfo struct {
	Name      string
	NameBlob  []byte
	NameInt   uint32
	Idx       int
	OmitEmpty bool
}

type FieldInfos []_FieldInfo

func (p FieldInfos) Len() int           { return len(p) }
func (p FieldInfos) Less(i, j int) bool { return p[i].Name < p[j].Name }
func (p FieldInfos) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func (p FieldInfos) FindName(name string) *_FieldInfo {
	i, j := 0, len(p)
	for i < j {
		m := int(uint(i+j) >> 1) // avoid overflow
		if p[m].Name >= name {
			j = m
		} else {
			i = m + 1
		}
	}

	if i >= len(p) || p[i].Name != name {
		return nil
	}
	return &p[i]
}

func (p FieldInfos) FindNameBlob(name []byte) *_FieldInfo {
	i, j := 0, len(p)
	for i < j {
		m := int(uint(i+j) >> 1) // avoid overflow
		if bytes.Compare(p[m].NameBlob, name) >= 0 {
			j = m
		} else {
			i = m + 1
		}
	}

	if i >= len(p) || !bytes.Equal(p[i].NameBlob, name) {
		return nil
	}
	return &p[i]
}

func (p FieldInfos) FindInt(n int64) *_FieldInfo {
	if n > 0 && n <= math.MaxUint32 {
		for i, _ := range p {
			f := &p[i]
			if f.NameInt == uint32(n) {
				return f
			}
		}
	}
	return nil
}

func getFieldInfos(t reflect.Type) FieldInfos {
	fields := []_FieldInfo{}
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		if !sf.IsExported() || sf.Anonymous {
			continue
		}

		tag := sf.Tag.Get("vbs")
		if tag == "" {
			tag = sf.Tag.Get("json")
		}

		if tag == "-" {
			continue
		}

		name, opts := parseTag(tag)

		/* NB: do we need to check the name?
		if !isValidName(name) {
			continue
		}
		*/

		nameInt := uint32(0)
		if name == "" {
			name = sf.Name
		} else if n, err := strconv.ParseUint(name, 10, 32); err == nil && n > 0 {
			nameInt = uint32(n)
		}

		vf := _FieldInfo{Name:name, NameBlob:[]byte(name), NameInt:nameInt, Idx:i, OmitEmpty:opts.Contains("omitempty"),}
		fields = append(fields, vf)
	}
	sort.Sort(FieldInfos(fields))
	return fields
}

var fieldMap sync.Map

// GetStructFieldInfos is like getFieldInfos but uses a cache to avoid repeated work.
func GetStructFieldInfos(t reflect.Type) FieldInfos {
	if f, ok := fieldMap.Load(t); ok {
		return f.(FieldInfos)
	}

	f, _ := fieldMap.LoadOrStore(t, getFieldInfos(t))
	return f.(FieldInfos)
}

func IsEmptyValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Pointer:
		return v.IsNil()
	}
	return false
}

