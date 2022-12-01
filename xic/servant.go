package xic

import (
	"fmt"
	"reflect"
	"strings"
	"errors"
)

type DefaultServant struct {
}

func (s DefaultServant) Xic(cur Current, in Arguments, out Arguments) error {
	return NewExf(MethodNotFoundException, "Method %#v not found in service %#v", cur.Method(), cur.Service())
}

func getServantInfo(name string, servant Servant) (*ServantInfo, error) {
	svc := &ServantInfo{Service: name, Servant: servant}
	mt, err := getMethodTable(servant)
	if err != nil {
		return nil, err
	}
	svc.Methods = mt
	return svc, nil
}

var typeOfError = reflect.TypeOf((*error)(nil)).Elem()

func IsValidInType(t reflect.Type) bool {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Struct:
		return true
	case reflect.Map:
		if t.Key().Kind() == reflect.String {
			return true
		}
	}
	return false
}

func IsValidOutType(t reflect.Type) bool {
	is_ptr := false
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
		is_ptr = true
	}

	switch t.Kind() {
	case reflect.Struct:
		return is_ptr
	case reflect.Map:
		if t.Key().Kind() == reflect.String {
			return true
		}
	}
	return false
}

func getMethodTable(servant Servant) (map[string]*MethodInfo, error) {
	v := reflect.TypeOf(servant)
	mt := make(map[string]*MethodInfo, v.NumMethod())
	for i := 0; i < v.NumMethod(); i++ {
		m := v.Method(i)
		if !strings.HasPrefix(m.Name, "Xic_") {
			continue
		}

		/* receiver, Current, in, out */
		if m.Type.NumIn() < 3 || m.Type.NumIn() > 4 || m.Type.NumOut() != 1 {
			return nil, fmt.Errorf("The number of input arguments (%d) or output arguments (%d) is not correct", m.Type.NumIn(), m.Type.NumOut())
		}

		if errType := m.Type.Out(0); errType != typeOfError {
			return nil, fmt.Errorf("The output argument must be an error interface")
		}

		cur := m.Type.In(1)
		if cur.Name() != "Current" && cur.PkgPath() != "halftwo/mangos/xic" {	// TODO
			return nil, fmt.Errorf("The first argument must be of type xic.Current instead of %s", cur.Name())
		}

		mi := &MethodInfo{
			Name: m.Name[4:],
			Method: m,
			InType: m.Type.In(2),
		}
		if !IsValidInType(mi.InType) {
			return nil, errors.New("Argument in of xic method must be a (pointer to) map[string]any or a (pointer to) struct")
		}

		if m.Type.NumIn() == 3 {
			mi.Oneway = true
		} else {
			mi.OutType = m.Type.In(3)
			if !IsValidOutType(mi.OutType) {
				return nil, errors.New("Argument out of xic method must be a (pointer to) map[string]any or a pointer to struct")
			}
		}

		mt[mi.Name] = mi
	}
	return mt, nil
}
