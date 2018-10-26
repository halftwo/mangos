package xic

import (
	"testing"
	"os"
	"io/ioutil"
	"time"
)

var shadow = `
#
# paramId = bits:g:N
#
[SRP6a]

#
# Following group parameters are taken from
#         http://srp.stanford.edu/demo/demo.html
#

P512 = 512:2:
        D4C7F8A2B32C11B8FBA9581EC4BA4F1B04215642EF7355E37C0FC0443EF756EA
        2C6B8EEB755A1C723027663CAA265EF785B8FF6A9B35227A52D86633DBDFCA43

#
# identity = method:paramId:hash:salt-base64:verifier-base64
# 
# If the paramId begins with @, it is an internal parameter.
#
[verifier]

!hello = SRP6a:@512:SHA1:rqX72x3PSi08Eu0BSMomQg:
        yC-CKkhIbyFT7BZhdm5WvQPWMm1qFNRlZGHnwljri3Ms0DqS42hJRzUw4T1DSonC
        1chc_HOI8Son-TbsPoDNYQ

`

func TestShadowBox(t *testing.T) {
	sb, err := NewShadowBox(shadow)
	if err != nil {
		t.Fatal(err)
	}

	v := sb.GetVerifier("hello")
	if v.Method != "SRP6a" || v.ParamId != "@512" || v.HashId != "SHA1" {
		t.Log("method", v.Method, "paramId", v.ParamId, "hashId", v.HashId)
		t.Errorf("Bug in (*Shadow).GetVerifier()")
	}

	s := sb.GetSrp6a(v.ParamId)
	if s.Bits != 512 || s.Gen != 2 {
		t.Log("bits", s.Bits, "g", s.Gen)
		t.Errorf("Bug in (*Shadow).GetSrp6a()")
	}

	fp, err := ioutil.TempFile("", "shadow.*")
	fp.WriteString(shadow)
	fp.Close()
	filename := fp.Name()
	defer os.Remove(filename)

	sb, err = NewShadowBoxFromFile(filename)
	if err != nil {
		t.Fatal(err)
	}

	nsb, _ := sb.Reload()
	if nsb != nil {
		t.Errorf("Bug in (*Shadow).Reload()")
	}

	os.Chtimes(filename, time.Now(), time.Now())
	nsb, _ = sb.Reload()
	if nsb == nil {
		t.Errorf("Bug in (*Shadow).Reload()")
	}

	if testing.Verbose() {
		sb.Dump(os.Stderr)
	}
}


