package xic

import (
	"fmt"
	"reflect"
	"strings"
)

type DefaultServant struct {
}

func (s DefaultServant) Xic(cur Current, in Arguments, out *Arguments) error {
	return NewExceptionf(MethodNotFoundException, "Method \"%s\" not found in service \"%s\"", cur.Method(), cur.Service())
}

func getServantInfo(name string, servant Servant) (*ServantInfo, error) {
	svc := &ServantInfo{Service:name, Servant:servant}
	mt, err := getMethodTable(servant)
	if err != nil {
		return nil, err
	}
	svc.methods = mt
	return svc, nil
}

var typeOfError = reflect.TypeOf((*error)(nil)).Elem()

func isValidArgs(x reflect.Type) bool {
	kind := x.Kind()
	if kind == reflect.Struct {
		return true
	}

	if kind != reflect.Map {
		return false
	}

	if x.Key().Kind() != reflect.String {
		return false
	}
	return true
}

func getMethodTable(servant Servant) (map[string]*MethodInfo, error) {
	mt := map[string]*MethodInfo{}
	v := reflect.TypeOf(servant)
	for i := 0; i < v.NumMethod(); i++ {
		m := v.Method(i)
		if !strings.HasPrefix(m.Name, "Xic_") {
			continue
		}

		/* receiver, Current, in, out */
		if m.Type.NumIn() < 3 || m.Type.NumIn() > 4 || m.Type.NumOut() != 1 {
			return nil, fmt.Errorf("The number of input arguments (%d) or output arguments (%s) is not correct", m.Type.NumIn(), m.Type.NumOut())
		}

		if errType := m.Type.Out(0); errType != typeOfError {
			return nil, fmt.Errorf("The output argument must be an error interface")
		}

		cur := m.Type.In(1)
		if cur.Name() != "Current" && cur.PkgPath() != "mangos/xic" {
			return nil, fmt.Errorf("The first argument must be of type xic.Current instead of %s", cur.Name)
		}

		mi := &MethodInfo{}
		mi.method = m
		mi.name = m.Name[4:]

		mi.inType = m.Type.In(2)
		in := mi.inType
		if mi.inType.Kind() == reflect.Ptr {
			in = mi.inType.Elem()
		}

		if !isValidArgs(in) {
			return nil, fmt.Errorf("The 2nd argument has invalid type %v", mi.inType)
		}

		if m.Type.NumIn() == 3 {
			mi.oneway = true
		} else {
			mi.outType = m.Type.In(3)
			if mi.outType.Kind() != reflect.Ptr {
				return nil, fmt.Errorf("The 3rd argument must be a pointer to struct or to map[string]interface{}")
			}

			out := mi.outType.Elem()
			if !isValidArgs(out) {
				return nil, fmt.Errorf("The 3nd argument has invalid type %v", mi.outType)
			}
		}

		mt[mi.name] = mi
	}
	return mt, nil
}

