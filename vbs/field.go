package vbs

import (
	"strings"
	"unicode"
	"reflect"
	"sort"
	"sync"
	"sync/atomic"
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
	name      string
	index     int
	tagged    bool
	omitEmpty bool
}

type fieldSlice []vbsField

func (p fieldSlice) Len() int           { return len(p) }
func (p fieldSlice) Less(i, j int) bool { return p[i].name < p[j].name }
func (p fieldSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func typeFields(t reflect.Type) []vbsField {
	var fields []vbsField

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

		tagged := name != ""
		if name == "" {
			name = sf.Name
		}

		vf := vbsField{name:name, index:i, tagged:tagged, omitEmpty:opts.Contains("omitempty"),}
		fields = append(fields, vf)
	}
	sort.Sort(fieldSlice(fields))
	return fields
}

var fieldCache struct {
	value atomic.Value // map[reflect.Type][]vbsField
	mutex sync.Mutex   // used only by writers
}

// cachedTypeFields is like typeFields but uses a cache to avoid repeated work.
func cachedTypeFields(t reflect.Type) []vbsField {
	m, _ := fieldCache.value.Load().(map[reflect.Type][]vbsField)
	f := m[t]
	if f != nil {
		return f
	}

	// Compute fields without lock.
	// Might duplicate effort but won't hold other computations back.
	f = typeFields(t)
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


