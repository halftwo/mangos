package xic

import (
	"sync"
	"path/filepath"
	"os"
	"bufio"
	"strings"
	"strconv"

	"halftwo/mangos/xerr"
)

type _Setting struct {
	filename string
	m sync.Map
}

func NewSetting() Setting {
	return &_Setting{}
}

func NewSettingFile(filename string) (Setting, error) {
	st := NewSetting()
	err := st.LoadFile(filepath.FromSlash(filename))
	if err != nil {
		return nil, err
	}
	return st, nil
}

func (st *_Setting) LoadFile(filename string) error {
	fp, err := os.Open(filename)
	if err != nil {
		return xerr.Trace(err)
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
			return xerr.Errorf("setting: invalid key=value pairs in %s:%d", filename, lineno)
		}

		key, value := ss[0], ss[1]
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if len(key) == 0 || len(value) == 0 {
			return xerr.Errorf("setting: invalid key=value pairs in %s:%d", filename, lineno)
		}

		st.m.Store(key, value)
	}
	st.filename = filename
	return nil
}

func (st *_Setting) Set(name string, value string) {
	st.m.Store(name, value)
}

func (st *_Setting) Remove(name string) {
	st.m.Delete(name)
}

func (st *_Setting) Insert(name string, value string) bool {
	_, loaded := st.m.LoadOrStore(name, value)
	return !loaded
}


func (st *_Setting) Has(name string) bool {
	_, ok := st.m.Load(name)
	return ok
}

func (st *_Setting) Get(name string) string {
	return st.GetDefault(name, "")
}

func (st *_Setting) GetDefault(name string, dft string) string {
	v, ok := st.m.Load(name)
	if ok {
		s := v.(string)
		if len(s) > 0 {
			return s
		}
	}
	return dft
}

func (st *_Setting) Int(name string) int64 {
	return st.IntDefault(name, 0)
}

func (st *_Setting) IntDefault(name string, dft int64) int64 {
	s := st.Get(name)
	v, err := strconv.ParseInt(s, 0, 64)
	if err != nil {
		return dft
	}
	return v
}

func (st *_Setting) Bool(name string) bool {
	return st.BoolDefault(name, false)
}

func (st *_Setting) BoolDefault(name string, dft bool) bool {
	s := st.Get(name)
	v, err := strconv.ParseBool(s)
	if err != nil {
		return dft
	}
	return v
}

func (st *_Setting) Float(name string) float64 {
	return st.FloatDefault(name, 0)
}

func (st *_Setting) FloatDefault(name string, dft float64) float64 {
	s := st.Get(name)
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return dft
	}
	return v
}

func (st *_Setting) Pathname(name string) string {
	return st.PathnameDefault(name, "")
}

func (st *_Setting) PathnameDefault(name string, dft string) string {
	s := filepath.FromSlash(st.GetDefault(name, dft))
	if filepath.IsAbs(s) {
		return s
	}
	return filepath.Join(filepath.Dir(st.filename), s)
}

func (st *_Setting) StringSlice(name string) []string {
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

