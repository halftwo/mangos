package xic

import (
	"time"
	"fmt"
	"bytes"
	"strings"
	"strconv"
	"net"
	"io"
	"io/ioutil"
	"os"

	"halftwo/mangos/xstr"
	"halftwo/mangos/xerr"
	"halftwo/mangos/bitmap"
)

type _Secret struct {
	service string
	proto string
	host string
	ipv6 [16]byte
	netPrefix uint8
	port uint16
	identity string
	password string
}

func (s *_Secret) matchHost(host string) bool {
	if s.host == host {
		return true
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}

	ip = ip.To16()
        return bitmap.EqualPrefix(s.ipv6[:], ip, uint(s.netPrefix))
}


type SecretBox struct {
	secrets []_Secret
	filename string
	mtime time.Time
}

func NewSecretBox(content string) (*SecretBox, error) {
	sb := &SecretBox{}
	err := sb.initialize([]byte(content))
	if err != nil {
		return nil, err
	}
	return sb, nil
}

func NewSecretBoxFromFile(filename string) (*SecretBox, error) {
	fi, err := os.Stat(filename)
	if err != nil {
		return nil, xerr.Tracef(err, "os.Stat() failed on file \"%s\"", filename)
	}

	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, xerr.Tracef(err, "ioutil.ReadFile() failed on file \"%s\"", filename)
	}

	sb := &SecretBox{filename:filename, mtime:fi.ModTime()}
	err = sb.initialize(content)
	if err != nil {
		return nil, xerr.Tracef(err, "initialize() failed on file \"%s\"", filename)
	}
	return sb, nil
}

func (sb *SecretBox) Reload() (*SecretBox, error) {
	if sb.filename != "" {
		fi, err := os.Stat(sb.filename)
		if err == nil && fi.ModTime() != sb.mtime {
			newsb, err := NewSecretBoxFromFile(sb.filename)
			return newsb, err
		}
	}
	return nil, nil
}

func (sb *SecretBox) initialize(content []byte) error {
	b := bytes.NewBuffer(content)
	lineno := 0
	for {
		line, _ := b.ReadString('\n')
		if len(line) == 0 {
			break
		}
		lineno++

		line = strings.TrimSpace(line)
		if len(line) == 0 || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, err := xstr.SplitKeyValue(line, "=")
		if len(key) == 0 || len(value) == 0 {
			return xerr.Errorf("Invalid syntax on line %d", lineno)
		}

		var n uint64
		var tmp string
		var s _Secret
		s.service, tmp, err = xstr.SplitKeyValue(key, "@")
		if err != nil {
			return xerr.Tracef(err, "Invalid port on line %d", lineno)
		}
		splitter := xstr.NewSplitter(tmp, "+")
		s.proto = splitter.Next()
		s.host = splitter.Next()
		tmp = splitter.Remain()
		if tmp == "" {
			s.port = 0
		} else {
			n, err = strconv.ParseUint(tmp, 10, 16)
			if err != nil {
				return xerr.Tracef(err, "Invalid port on line %d", lineno)
			}
			s.port = uint16(n)
		}

		s.host, tmp = xstr.Split2(s.host, "/")
		if tmp == "" {
			s.netPrefix = 128
		} else {
			n, err = strconv.ParseUint(tmp, 10, 8)
			if err != nil {
				return xerr.Tracef(err, "Invalid net prefix on line %d", lineno)
			} else if n == 0 || n > 128 {
				return xerr.Errorf("Invalid net prefix on line %d: %s", lineno, tmp)
			}
			s.netPrefix = uint8(n)
		}

		ip := net.ParseIP(s.host)
		if ip != nil {
			if ip.To4() != nil {
				if s.netPrefix < 32 {
					s.netPrefix += 96
				}
			}
			copy(s.ipv6[:], ip)
		} else if s.netPrefix != 128 {
			return xerr.Errorf("Invalid net prefix on line %d: %d", lineno, s.netPrefix)
		}

		s.identity, s.password, err = xstr.SplitKeyValue(value, ":")
		if err != nil {
			return xerr.Tracef(err, "Invalid identity or password on line %d", lineno)
		} else if len(s.password) == 0 {
			return xerr.Errorf("Empty password on line %d", lineno)
		}

		sb.secrets = append(sb.secrets, s)
	}
	return nil
}

func (sb *SecretBox) Dump(w io.Writer) {
	for _, s := range sb.secrets {
		fmt.Fprintf(w, "%s@%s+%s", s.service, s.proto, s.host)
		if s.netPrefix != 128 {
			prefix := s.netPrefix
			if !strings.ContainsRune(s.host, ':') {
				prefix -= 96
			}
			fmt.Fprintf(w, "/%d", prefix)
		}

		if s.port > 0 {
			fmt.Fprintf(w, "+%d", s.port)
		} else {
			io.WriteString(w, "+")
		}

		fmt.Fprintf(w, " = %s:%s\n", s.identity, s.password)
	}
}

func (sb *SecretBox) GetContent() string {
	b := strings.Builder{}
	sb.Dump(&b)
	return b.String()
}

func (sb *SecretBox) Find(service, endpoint string) (identity, password string) {
	ei, err := parseEndpoint(endpoint)
	if err != nil {
		return "", ""
	}
	return sb.FindEndpoint(service, ei)
}

func (sb *SecretBox) FindEndpoint(service string, ei *EndpointInfo) (identity, password string) {
	return sb.find(service, ei.proto, ei.host, ei.port)
}

func (sb *SecretBox) find(service, proto, host string, port uint16) (identity, password string) {
	if proto == "" {
		proto = "tcp"
	}

	for _, s := range sb.secrets {
		if len(s.service) > 0 && s.service != service {
			continue
		}
		if len(s.proto) > 0 && s.proto != proto {
			continue
		}
		if len(s.host) > 0 && !s.matchHost(host) {
			continue
		}
		if s.port >0 && s.port != port {
			continue
		}

		identity = s.identity
		password = s.password
		break
	}
	return
}


