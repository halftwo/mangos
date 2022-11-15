package xic

import (
	"testing"
	"os"
	"io/ioutil"
	"time"
)

var secret = `
#
# Example of secret file.
#
# service @ type+host+port = identity : password
# service @ type+ip+port = identity : password
# service @ type+ip/prefix+port = identity : password
#
# Empty field matches any value in that field.
#
#

Service @ tcp+192.168.1.1/24+1234 = random : acjd8avh6917ym7x3px4bdbenzndrab4mdyat55

@ tcp+::1+3030 = complex : complicated

@++ = hello:world

`

func TestSecretBox(t *testing.T) {
	sb, err := NewSecretBox(secret)
	if err != nil {
		t.Fatal(err)
	}

	id, pass := sb.Find("XXX", "@+::1+3030")
	if id != "complex" || pass != "complicated" {
		t.Log("id", id, "pass", pass)
		t.Errorf("Bug in (*Secret).Find()")
	}

	fp, err := ioutil.TempFile("", "secret.*")
	fp.WriteString(secret)
	fp.Close()
	filename := fp.Name()
	defer os.Remove(filename)

	sb, err = NewSecretBoxFromFile(filename)
	if err != nil {
		t.Fatal(err)
	}

	nsb, _ := sb.Reload()
	if nsb != nil {
		t.Errorf("Bug in (*Secret).Reload()")
	}

	os.Chtimes(filename, time.Now(), time.Now())
	nsb, _ = sb.Reload()
	if nsb == nil {
		t.Errorf("Bug in (*Secret).Reload()")
	}

	if testing.Verbose() {
		sb.Dump(os.Stderr)
	}
}

