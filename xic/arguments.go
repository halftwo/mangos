package xic

import (
	"fmt"
	"reflect"
	"math"

	"halftwo/mangos/vbs"
)

type Arguments map[string]interface{}

func NewArguments() Arguments {
	return make(map[string]interface{})
}

func (args Arguments) CopyFrom(rhs interface{}) error {
	return assignMap(reflect.ValueOf(args), reflect.ValueOf(rhs))
}

func (args Arguments) CopyTo(x interface{}) error {
	v := reflect.ValueOf(x)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return fmt.Errorf("The parameter shoud be pointer to struct or map[string]interface{}")
	}

	var err error
	switch v.Elem().Kind() {
	case reflect.Map:
		err = assignMap(v.Elem(), reflect.ValueOf(args))
	case reflect.Struct:
		err = args.ArgStruct(v.Interface())
	default:
		err = fmt.Errorf("The parameter shoud be pointer to struct or map[string]interface{}")
	}
	return err
}

func (args Arguments) Has(name string) bool {
	_, ok := args[name]
	return ok
}

func (args Arguments) Get(name string) interface{} {
	return args[name]
}

func (args Arguments) GetString(name string) string {
	return args.GetStringDefault(name, "")
}

func (args Arguments) GetStringDefault(name string, dft string) string {
	v, ok := args[name]
	if ok {
		s, ok := v.(string)
		if ok && len(s) > 0 {
			return s
		}
	}
	return dft
}

func (args Arguments) GetInt(name string) int64 {
	return args.GetIntDefault(name, 0)
}

func (args Arguments) GetIntDefault(name string, dft int64) int64 {
	v, ok := args[name]
	if ok {
		i, ok := v.(int64)
		if ok {
			return i
		}
	}
	return dft
}

func (args Arguments) GetBool(name string) bool {
	return args.GetBoolDefault(name, false)
}

func (args Arguments) GetBoolDefault(name string, dft bool) bool {
	v, ok := args[name]
	if ok {
		t, ok := v.(bool)
		if ok {
			return t
		}
	}
	return dft
}

func (args Arguments) GetFloat(name string) float64 {
	return args.GetFloatDefault(name, 0.0)
}

func (args Arguments) GetFloatDefault(name string, dft float64) float64 {
	v, ok := args[name]
	if ok {
		f, ok := v.(float64)
		if ok {
			return f
		}
	}
	return dft
}

func (args Arguments) GetBlob(name string) []byte {
	return args.GetBlobDefault(name, []byte{})
}

func (args Arguments) GetBlobDefault(name string, dft []byte) []byte {
	v, ok := args[name]
	if ok {
		b, ok := v.([]byte)
		if ok {
			return b
		}
	}
	return dft
}

func (args Arguments) checkInterface(name string, kind reflect.Kind, p interface{}) (interface{}, reflect.Value, error) {
	x := reflect.ValueOf(p)
	if x.Kind() != reflect.Ptr || x.IsNil() || x.Elem().Kind() != kind {
		return nil, reflect.Value{}, fmt.Errorf("The second parameter should be pointer to %v", kind)
	}

	v, ok := args[name]
	if !ok {
		return nil, reflect.Value{}, fmt.Errorf("No such name \"%s\" found in the arguments", name)
	}

	k := reflect.TypeOf(v).Kind()
	if k != kind {
		if !(kind == reflect.Struct && k == reflect.Map) {
			return nil, reflect.Value{}, fmt.Errorf("The value of \"%s\" in the arguments has a incompatible kind (%s) other than %v", name, k, kind)
		}
	}

	return v, x.Elem(), nil
}

func (args Arguments) GetSlice(name string, p interface{}) error {
	v, x, err := args.checkInterface(name, reflect.Slice, p)
	if err != nil {
		return err
	}
	w := reflect.ValueOf(v)
	err = assignSlice(x, w)
	return err
}

func (args Arguments) GetMap(name string, p interface{}) error {
	v, x, err := args.checkInterface(name, reflect.Map, p)
	if err != nil {
		return err
	}
	w := reflect.ValueOf(v)
	err = assignMap(x, w)
	return err
}

func (args Arguments) GetStruct(name string, p interface{}) error {
	v, x, err := args.checkInterface(name, reflect.Struct, p)
	if err != nil {
		return err
	}
	w := reflect.ValueOf(v)

	fields := vbs.CachedStructFields(x.Type())
	for _, f := range fields {
		value := w.MapIndex(reflect.ValueOf(f.Name))
		if !value.IsValid() {
			if f.OmitEmpty {
				continue
			} else {
				return fmt.Errorf("No value for struct field \"%s\"", f.Name)
			}
		}

		err := assignValue(x.Field(f.Index), value)
		if err != nil {
			return err
		}
	}

	return nil
}

func (args Arguments) SetArgStruct(p interface{}) error {
	x := reflect.ValueOf(p)
	if x.Kind() == reflect.Ptr {
		if !x.IsNil() {
			x = x.Elem()
		}
	}

	if x.Kind() != reflect.Struct {
		return fmt.Errorf("The parameter should be struct or pointer to struct")
	}

	fields := vbs.CachedStructFields(x.Type())
	for _, f := range fields {
		value := x.Field(f.Index)
		if f.OmitEmpty && vbs.IsEmptyValue(value) {
			continue
		}

		err := args.Set(f.Name, value.Interface())
		if err != nil {
			return err
		}
	}

	return nil
}

func (args Arguments) ArgStruct(p interface{}) error {
	x := reflect.ValueOf(p)
	if x.Kind() != reflect.Ptr || x.IsNil() || x.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("The parameter should be pointer to struct")
	}
	x = x.Elem()

	w := reflect.ValueOf(args)
	fields := vbs.CachedStructFields(x.Type())
	for _, f := range fields {
		value := w.MapIndex(reflect.ValueOf(f.Name))
		if !value.IsValid() {
			if f.OmitEmpty {
				continue
			} else {
				return fmt.Errorf("Parameter \"%s\" missing", f.Name)
			}
		}

		err := assignValue(x.Field(f.Index), value)
		if err != nil {
			return err
		}
	}

	return nil
}

func assignInt(lhs, rhs reflect.Value) error {
	switch k := rhs.Kind(); k {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i := rhs.Int()
		lhs.SetInt(i)
		if lhs.Int() != i {
			return fmt.Errorf("Can't assign %v(%d) to %v", k, i, lhs.Kind())
		}

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		u := rhs.Uint()
		i := int64(u)
		lhs.SetInt(i)
		if i < 0 || lhs.Int() != i {
			return fmt.Errorf("Can't assign %v(%d) to %v", k, u, lhs.Kind())
		}
	default:
		return fmt.Errorf("Can't assign %v to %v", k, lhs.Kind())
	}
	return nil
}

func assignUint(lhs, rhs reflect.Value) error {
	switch k := rhs.Kind(); k {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i := rhs.Int()
		u := uint64(i)
		lhs.SetUint(u)
		if i < 0 || lhs.Uint() != u {
			return fmt.Errorf("Can't assign %v(%d) to %v", k, i, lhs.Kind())
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		u := rhs.Uint()
		lhs.SetUint(u)
		if lhs.Uint() != u {
			return fmt.Errorf("Can't assign %v(%d) to %v", k, u, lhs.Kind())
		}
	default:
		return fmt.Errorf("Can't assign %v to %v", k, lhs.Kind())
	}
	return nil
}

func assignString(lhs, rhs reflect.Value) error {
	if k := rhs.Kind(); k == reflect.String {
		lhs.SetString(rhs.String())
	} else {
		return fmt.Errorf("Can't assign %v to %v", k, lhs.Kind())
	}
	return nil
}

func assignBool(lhs, rhs reflect.Value) error {
	if k := rhs.Kind(); k == reflect.Bool {
		lhs.SetBool(rhs.Bool())
	} else {
		return fmt.Errorf("Can't assign %v to %v", k, lhs.Kind())
	}
	return nil
}

func assignFloat(lhs, rhs reflect.Value) error {
	switch k := rhs.Kind(); k {
	case reflect.Float32:
		lhs.SetFloat(rhs.Float())
	case reflect.Float64:
		f := rhs.Float()
		if lhs.Kind() != reflect.Float64 {
			if math.Abs(f) > math.MaxFloat32 {
				return fmt.Errorf("Can't assign %v(%v) to %v", k, f, lhs.Kind())
			}
		}
		lhs.SetFloat(f)
	default:
		return fmt.Errorf("Can't assign %v to %v", k, lhs.Kind())
	}
	return nil
}

func assignComplex(lhs, rhs reflect.Value) error {
	switch k := rhs.Kind(); k {
	case reflect.Complex64:
		lhs.SetComplex(rhs.Complex())
	case reflect.Complex128:
		c := rhs.Complex()
		if lhs.Kind() != reflect.Complex128 {
			if math.Abs(real(c)) > math.MaxFloat32 || math.Abs(imag(c)) > math.MaxFloat32 {
				return fmt.Errorf("Can't assign %v(%v) to %v", k, c, lhs.Kind())
			}
		}
		lhs.SetComplex(c)
	default:
		return fmt.Errorf("Can't assign %v to %v", k, lhs.Kind())
	}
	return nil
}

func assignArray(lhs, rhs reflect.Value) error {
	lek := lhs.Type().Elem().Kind()
	rek := rhs.Type().Elem().Kind()
	switch k := rhs.Kind(); k {
	case reflect.Array:
		break
	case reflect.Slice:
		if lhs.Len() != rhs.Len() {
			return fmt.Errorf("Can't assign [%d]%v to [%d]%v", rhs.Len(), rek, lhs.Len(), lek)
		}
		if lek == reflect.Uint8 && rek == reflect.Uint8 {
			lhs.Slice(0, lhs.Len()).SetBytes(rhs.Bytes())
		}
		return nil
	default:
		return fmt.Errorf("Can't assign %v to %v", k, lhs.Kind())
	}

	for i := 0; i < lhs.Len(); i++ {
		assignValue(lhs.Index(i), rhs.Index(i))
	}
	return nil
}

func assignSlice(lhs, rhs reflect.Value) error {
	switch k := rhs.Kind(); k {
	case reflect.Array:
		break
	case reflect.Slice:
		if rhs.Type().Elem().Kind() == reflect.Uint8 && lhs.Type().Elem().Kind() == reflect.Uint8 {
			lhs.SetBytes(rhs.Bytes())
			return nil
		}
	default:
		return fmt.Errorf("Can't assign %v to %v", k, lhs.Kind())
	}

	for i := 0; i < rhs.Len(); i++ {
		if i >= lhs.Cap() {
			newcap := lhs.Cap() + lhs.Cap()/2
			if newcap < 4 {
				newcap = 4
			}
			newx := reflect.MakeSlice(lhs.Type(), lhs.Len(), newcap)
			reflect.Copy(newx, lhs)
			lhs.Set(newx)
		}
		if i >= lhs.Len() {
			lhs.SetLen(i + 1)
		}

		err := assignValue(lhs.Index(i), rhs.Index(i))
		if err != nil {
			return err
		}
	}
	return nil
}

func assignMap(lhs, rhs reflect.Value) error {
	if k := rhs.Kind(); k != reflect.Map {
		if k == reflect.Struct {
			return assignMapFromStruct(lhs, rhs)
		}
		return fmt.Errorf("Can't assign %v to %v", k, lhs.Kind())
	}

	lkeyType := lhs.Type().Key()
	lelemType := lhs.Type().Elem()

	keys := rhs.MapKeys()
	for _, key := range keys {
		lkey := reflect.New(lkeyType).Elem()
		if err := assignValue(lkey, key); err != nil {
			return err
		}

		elem := rhs.MapIndex(key)
		lelem := reflect.New(lelemType).Elem()
		if err := assignValue(lelem, elem); err != nil {
			return err
		}

		lhs.SetMapIndex(lkey, lelem)
	}
	return nil
}

func assignMapFromStruct(lhs, rhs reflect.Value) error {
	if k := rhs.Kind(); k != reflect.Struct {
		return fmt.Errorf("Can't assign %v to %v", k, lhs.Kind())
	}

	lkeyType := lhs.Type().Key()
	if lkeyType.Kind() != reflect.String {
		return fmt.Errorf("To assign map from struct, the kind of map key must be string")
	}
	lelemType := lhs.Type().Elem()

	fields := vbs.CachedStructFields(rhs.Type())
	for _, f := range fields {
		value := rhs.Field(f.Index)
		if f.OmitEmpty && vbs.IsEmptyValue(value) {
			continue
		}

		lkey := reflect.New(lkeyType).Elem()
		if err := assignString(lkey, reflect.ValueOf(f.Name)); err != nil {
			return err
		}

		lelem := reflect.New(lelemType).Elem()
		if err := assignValue(lelem, value); err != nil {
			return err
		}

		lhs.SetMapIndex(lkey, lelem)
	}
	return nil
}

func assignStructFromMap(lhs, rhs reflect.Value) error {
	if k := rhs.Kind(); k != reflect.Map {
		return fmt.Errorf("Can't assign %v to %v", k, lhs.Kind())
	}

	rkeyType := rhs.Type().Key()
	if rkeyType.Kind() != reflect.String {
		return fmt.Errorf("To assign struct from map, the kind of map key must be string")
	}

	fields := vbs.CachedStructFields(lhs.Type())
	for _, f := range fields {
		value := lhs.Field(f.Index)
		elem := lhs.MapIndex(reflect.ValueOf(f.Name))
		if !elem.IsValid() {
			continue
		}
		if err := assignValue(value, elem); err != nil {
			return err
		}
	}
	return nil
}

func dereference(v reflect.Value) (reflect.Value, error) {
	if v.Kind() != reflect.Ptr && v.Kind() != reflect.Interface {
		return v, nil
	}

	for v.IsValid() {
		elem := v.Elem()
		k := elem.Kind()
		if k != reflect.Ptr && k != reflect.Interface {
			break
		}
		v = elem
	}

	if !v.IsValid() {
		return reflect.Value{}, fmt.Errorf("Value can't be nil pointer or interface")
	}

	return v.Elem(), nil
}

func assignValue(lhs, rhs reflect.Value) error {
	if !rhs.IsValid() {
		//  TODO
		return nil
	}

	if lhs.Type() == rhs.Type() {
		lhs.Set(rhs)
	}

	var err error
	k := rhs.Kind()
	if k == reflect.Ptr || k == reflect.Interface {
		rhs, err = dereference(rhs)
		if err != nil {
			return err
		}
	}

	switch lk := lhs.Kind(); lk {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		err = assignInt(lhs, rhs)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		err = assignUint(lhs, rhs)
	case reflect.String:
		err = assignString(lhs, rhs)
	case reflect.Bool:
		err = assignBool(lhs, rhs)
	case reflect.Float32:
		err = assignFloat(lhs, rhs)
	case reflect.Complex64:
		err = assignComplex(lhs, rhs)
	case reflect.Array:
		err = assignArray(lhs, rhs)
	case reflect.Slice:
		err = assignSlice(lhs, rhs)
	case reflect.Map:
		err = assignMap(lhs, rhs)
	case reflect.Struct:
		err = assignStructFromMap(lhs, rhs)
	case reflect.Interface:
		if lhs.NumMethod() == 0 {
			lhs.Set(rhs)
		} else {
			err = fmt.Errorf("lhs of assignment can't be non-empty interface")
		}
	case reflect.Ptr:
		err = fmt.Errorf("lhs of assignment can't be pointer")
	default:
		err = fmt.Errorf("lhs of assignment can't be %v", lk)
	}

	return err
}

func (args Arguments) Set(name string, x interface{}) error {
	v := reflect.ValueOf(x)
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
		x = v.Int()

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		x = int64(v.Uint())

	case reflect.Float32:
		x = v.Float()

	case reflect.Array:
		if v.Elem().Kind() == reflect.Uint8 {
			buf := make([]byte, v.Len())
			reflect.Copy(reflect.ValueOf(buf), v)
			x = buf
		}

	case reflect.Complex64:
		x = v.Complex()

	case reflect.Map:
		if err := checkMap(v); err != nil {
			return err
		}

	case reflect.Struct:
		x = structToMap(v)

	case reflect.Ptr, reflect.Interface:
		v, err := dereference(v)
		if err != nil {
			return err
		}
		return args.Set(name, v.Interface())

	default:
		// reflect.String, reflect.Slice, reflect.Bool,
		// reflect.Int64, reflect.Float64, reflect.Complex128,
	}

	args[name] = x
	return nil
}

func checkMap(v reflect.Value) error {
	kk := v.Type().Key().Kind()
	switch kk {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
	case reflect.String:
	default:
		return fmt.Errorf("Only integer or string can be the key of a map")
	}
	return nil
}

func structToMap(v reflect.Value) interface{} {
	m := map[string]interface{}{}
	fields := vbs.CachedStructFields(v.Type())
	for _, f := range fields {
		value := v.Field(f.Index)
		if f.OmitEmpty && vbs.IsEmptyValue(value) {
			continue
		}

		m[f.Name] = value.Interface()
	}
	return m
}

