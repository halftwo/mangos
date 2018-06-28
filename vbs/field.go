package vbs

import (
	"strings"
	"unicode"
	"reflect"
	"sort"
	"sync"
	"sync/atomic"
	"strconv"
	"math"
	"fmt"
)

// tagOptions is the string following a comma in a struct field's "json"
// tag, or the empty string. It does not include the leading comma.
type tagOptions string

// parseTag splits a struct field's json tag into its name and
// comma-separated options.
func parseTag(tag string) (string, tagOptions) {
	if idx := strings.Index(tag, ","); idx != -1 {
		return tag[:idx], tagOptions(tag[idx+1:])
	}
	return tag, tagOptions("")
}

// Contains reports whether a comma-separated list of options
// contains a particular substr flag. substr must be surrounded by a
// string boundary or commas.   
func (o tagOptions) Contains(optionName string) bool {
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

func isValidTag(s string) bool {
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

type vbsField struct {
	Name      string
	IntName   uint32
	Index     uint32
	OmitEmpty bool
}

type StructFields []vbsField

func (p StructFields) Len() int           { return len(p) }
func (p StructFields) Less(i, j int) bool { return p[i].Name < p[j].Name }
func (p StructFields) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func (p StructFields) Find(name string) *vbsField {
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

func (p StructFields) FindInt(n int64) *vbsField {
	if n > 0 && n <= math.MaxUint32 {
		for i := 0; i < len(p); i++ {
			f := &p[i]
			if f.IntName == uint32(n) {
				return f
			}
		}
	}
	return nil
}

func getStructFields(t reflect.Type) StructFields {
	var fields []vbsField

	if t.NumField() > math.MaxUint32 {
		panic(fmt.Sprintf("vbs: too much fields in struct %v", t))
	}

	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		isUnexported := sf.PkgPath != ""
		if isUnexported || sf.Anonymous {
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
		if !isValidTag(name) {
			continue
		}

		iName := uint32(0)
		if name == "" {
			name = sf.Name
		} else if n, err := strconv.ParseUint(name, 10, 32); err == nil && n > 0 {
			iName = uint32(n)
		}

		vf := vbsField{Name:name, IntName:iName, Index:uint32(i), OmitEmpty:opts.Contains("omitempty"),}
		fields = append(fields, vf)
	}
	sort.Sort(StructFields(fields))
	return fields
}

var fieldCache struct {
	value atomic.Value // map[reflect.Type][]vbsField
	mutex sync.Mutex   // used only by writers
}

// CachedStructFields is like getStructFields but uses a cache to avoid repeated work.
func CachedStructFields(t reflect.Type) StructFields {
	m, _ := fieldCache.value.Load().(map[reflect.Type][]vbsField)
	f := m[t]
	if f != nil {
		return f
	}

	// Compute fields without lock.
	// Might duplicate effort but won't hold other computations back.
	f = getStructFields(t)
	if f == nil {
		f = []vbsField{}
	}

	fieldCache.mutex.Lock()
	m, _ = fieldCache.value.Load().(map[reflect.Type][]vbsField)
	newM := make(map[reflect.Type][]vbsField, len(m)+1)
	for k, v := range m {
		newM[k] = v
	}
	newM[t] = f
	fieldCache.value.Store(newM)
	fieldCache.mutex.Unlock()
	return f
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
	case reflect.Interface, reflect.Ptr:
		return v.IsNil()
	}
	return false
}

