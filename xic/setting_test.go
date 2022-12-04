package xic

import (
	"testing"
	"fmt"
)

func TestSettingPathname(t *testing.T) {
	st := &_Setting{}
	st.filename = "hello/world"
	st.Set("A", "")
	a := st.Pathname("A")
	if a != "" {
		fmt.Printf("expect %#v got %#v", "", a)
		t.Fatal("setting.Pathname() failed")
	}
}

