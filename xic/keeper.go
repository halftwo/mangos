package xic

import (
	"fmt"
	"reflect"
	"io"
	"sort"
	"bytes"

	"halftwo/mangos/vbs"
)

type _KeeperServant struct {
	DefaultServant
	engine *_Engine
}

type _Out_adapters struct {
	Adapters map[string]string	`vbs:"adapters"`
}

func (kp *_KeeperServant) Xic_adapters(cur Current, in struct{}, out *_Out_adapters) error {
	out.Adapters = map[string]string{}
	kp.engine.getAllAdapters(out.Adapters)
	return nil
}

type _In_services struct {
	Adapter string			`vbs:"adapter"`
}

type _Out_services struct {
	Ok bool				`vbs:"ok"`
	Services map[string][]string	`vbs:"services"`
}

func (kp *_KeeperServant) Xic_services(cur Current, in _In_services, out *_Out_services) error {
	ap := kp.engine.findAdapter(in.Adapter)
	if ap != nil {
		var sis []*ServantInfo
		ap.srvMap.Range(func (key, value any) bool {
			sis = append(sis, value.(*ServantInfo))
			return true
		})
		out.Ok = true
		out.Services = map[string][]string{}
		for _, si := range sis {
			out.Services[si.Service] = getServantMethods(si)
		}
	}
	return nil
}

func type2rune(t reflect.Type) rune {
	if t == vbs.ReflectTypeOfDecimal64 {
		return 'd'
	}
	switch t.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return 'i'
	case reflect.String:
		return 's'
	case reflect.Bool:
		return 't'
	case reflect.Float32, reflect.Float64:
		return 'f'
	case reflect.Array, reflect.Slice:
		if t.Elem().Kind() == reflect.Uint8 {
			return 'b'
		}
		return 'l'
	case reflect.Map, reflect.Struct:
		return 'm'
	}
	return 'x'
}

/**
  (name1/type1,name2/type2,?name3/type3,...,nameN/typeN) if struct
  () if empty struct
  (...) if map
**/
func printMethodArg(w io.Writer, t reflect.Type) {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Map:
		fmt.Fprint(w, "(...)")
	case reflect.Struct:
		fmt.Fprint(w, "(")
		fields := vbs.GetStructFieldInfos(t)
		for i, f := range fields {
			if i > 0 {
				fmt.Fprint(w, ",")
			}
			if f.OmitEmpty {
				fmt.Fprint(w, "?")
			}
			fmt.Fprintf(w, "%s/%c", f.Name, type2rune(t.Field(f.Idx).Type))
		}
		fmt.Fprint(w, ")")
	}
}

func printMethodInfo(w io.Writer, mi *MethodInfo) {
	fmt.Fprintf(w, "%s", mi.Name)
	printMethodArg(w, mi.InType)
	if !mi.Oneway {
		printMethodArg(w, mi.OutType)
	}
}

func getServantMethods(si *ServantInfo) (methods []string) {
	names := make([]string, 0, len(si.Methods))
	for name := range si.Methods {
		names = append(names, name)
	}
	sort.Strings(names)
	b := &bytes.Buffer{}
	for _, name := range names {
		b.Reset()
		printMethodInfo(b, si.Methods[name])
		methods = append(methods, string(b.Bytes()))
	}
	return
}

