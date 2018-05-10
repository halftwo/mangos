package xic

import (
	"sync"
	"path"
	"fmt"
	"os"
	"bufio"
	"strings"
	"strconv"
)

type stdSetting struct {
	filename string
	m sync.Map
}

func NewSetting() Setting {
	return &stdSetting{}
}

func NewSettingFile(filename string) (Setting, error) {
	st := NewSetting()
	err := st.LoadFile(filename)
	if err != nil {
		return nil, err
	}
	return st, nil
}

func (st *stdSetting) LoadFile(filename string) error {
	fp, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer fp.Close()

	scanner := bufio.NewScanner(fp)
	lineno := 0
	for scanner.Scan() {
		lineno++
		line := scanner.Text()
		line = strings.TrimSpace(line)
		if line == "" || line[0] == '#' {
			continue
		}

		ss := strings.SplitN(line, "=", 2)
		if len(ss) != 2 {
			return fmt.Errorf("setting: invalid key=value pairs in %s:%d", filename, lineno)
		}

		key, value := ss[0], ss[1]
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if len(key) == 0 || len(value) == 0 {
			return fmt.Errorf("setting: invalid key=value pairs in %s:%d", filename, lineno)
		}

		st.m.Store(key, value)
	}
	st.filename = filename
	return nil
}

func (st *stdSetting) Set(name string, value string) {
	st.m.Store(name, value)
}

func (st *stdSetting) Remove(name string) {
	st.m.Delete(name)
}

func (st *stdSetting) Insert(name string, value string) bool {
	_, loaded := st.m.LoadOrStore(name, value)
	return !loaded
}


func (st *stdSetting) Has(name string) bool {
	_, ok := st.m.Load(name)
	return ok
}

func (st *stdSetting) Get(name string) string {
	return st.GetDefault(name, "")
}

func (st *stdSetting) GetDefault(name string, dft string) string {
	v, ok := st.m.Load(name)
	if ok {
		s := v.(string)
		if len(s) > 0 {
			return s
		}
	}
	return dft
}

func (st *stdSetting) Int(name string) int64 {
	return st.IntDefault(name, 0)
}

func (st *stdSetting) IntDefault(name string, dft int64) int64 {
	s := st.Get(name)
	v, err := strconv.ParseInt(s, 0, 64)
	if err != nil {
		return dft
	}
	return v
}

func (st *stdSetting) Bool(name string) bool {
	return st.BoolDefault(name, false)
}

func (st *stdSetting) BoolDefault(name string, dft bool) bool {
	s := st.Get(name)
	v, err := strconv.ParseBool(s)
	if err != nil {
		return dft
	}
	return v
}

func (st *stdSetting) Float(name string) float64 {
	return st.FloatDefault(name, 0)
}

func (st *stdSetting) FloatDefault(name string, dft float64) float64 {
	s := st.Get(name)
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return dft
	}
	return v
}

func (st *stdSetting) Pathname(name string) string {
	s := st.Get(name)
	if len(s) == 0 || s[0] == '/' {
		return s
	}

	return path.Join(path.Dir(st.filename), s)
}

func (st *stdSetting) PathnameDefault(name string, dft string) string {
	s := st.GetDefault(name, dft)
	if len(s) == 0 || s[0] == '/' {
		return s
	}

	return path.Join(path.Dir(st.filename), s)
}

func (st *stdSetting) StringSlice(name string) []string {
	s := st.Get(name)
	var array []string
	for {
		i := strings.IndexAny(s, ",; \t\r\n")
		if i >= 0 {
			v := s[0:i]
			if len(v) > 0 {
				array = append(array, s[0:i])
			}

			s = s[i+1:]
			if len(s) == 0 {
				break
			}
		} else {
			array = append(array, s[0:i])
			break
		}
	}
	return array
}


