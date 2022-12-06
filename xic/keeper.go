package xic

import (
	"reflect"
	"sort"
	"strings"

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
	var ap *_Adapter
	if in.Adapter == SLACK_ADAPTER_NAME {
		ap = kp.engine.slackAdapter
	} else {
		ap = kp.engine.findAdapter(in.Adapter)
	}
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

func BuildTypeString(b *strings.Builder, t reflect.Type) {
	if t == vbs.ReflectTypeOfDecimal64 {
		b.WriteByte('d')
		return
	}

	switch t.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		b.WriteByte('i')
	case reflect.String:
		b.WriteByte('s')
	case reflect.Bool:
		b.WriteByte('t')
	case reflect.Float32, reflect.Float64:
		b.WriteByte('f')
	case reflect.Array, reflect.Slice:
		if t.Elem().Kind() == reflect.Uint8 {
			b.WriteByte('b')
		} else {
			b.WriteByte('[')
			BuildTypeString(b, t.Elem())
			b.WriteByte(']')
		}
	case reflect.Map:
		b.WriteByte('{')
		BuildTypeString(b, t.Key())
		b.WriteByte(':')
		BuildTypeString(b, t.Elem())
		b.WriteByte('}')
	case reflect.Struct:
		b.WriteByte('(')
		fields := vbs.GetStructFieldInfos(t)
		for i, f := range fields {
			if i > 0 {
				b.WriteByte(',')
			}
			if f.OmitEmpty {
				b.WriteByte('?')
			}
			b.WriteString(f.Name)
			b.WriteByte('/')
			BuildTypeString(b, t.Field(f.Idx).Type)
		}
		b.WriteByte(')')
	default:
		b.WriteByte('x')
	}
}

/**
  (name1/type1,name2/type2,?name3/type3,...,nameN/typeN) if struct
  () if empty struct
  (...) if map
**/
func printMethodArg(b *strings.Builder, t reflect.Type) {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Map:
		b.WriteString("(...)")
	case reflect.Struct:
		BuildTypeString(b, t)
	}
}

func printMethodInfo(w *strings.Builder, mi *MethodInfo) {
	w.WriteString(mi.Name)
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
	b := strings.Builder{}
	for _, name := range names {
		b.Reset()
		printMethodInfo(&b, si.Methods[name])
		methods = append(methods, b.String())
	}
	return
}

