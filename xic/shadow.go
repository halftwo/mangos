package xic

import (
	"time"
	"fmt"
	"bytes"
	"strings"
	"strconv"
	"io"
	"io/ioutil"
	"os"
	"encoding/base64"
	"encoding/hex"
	"unicode"
	"unicode/utf8"

	"halftwo/mangos/xstr"
	"halftwo/mangos/xerr"
)


type _Srp6a struct {
	Bits uint
	Gen uint64
	N []byte
}

type _Verifier struct {
	Method string
	ParamId string
	HashId string
	Salt []byte
	Verifier []byte
}

type ShadowBox struct {
	srp6aMap map[string]*_Srp6a
	verifierMap map[string]*_Verifier
	filename string
	mtime time.Time
}

func newShadowBox() *ShadowBox {
	sb := &ShadowBox{}
	sb.srp6aMap = make(map[string]*_Srp6a)
	sb.verifierMap = make(map[string]*_Verifier)
	return sb
}

func NewShadowBox(content string) (*ShadowBox, error) {
	sb := newShadowBox()
	err := sb.initialize([]byte(content))
	if err != nil {
		return nil, err
	}
	return sb, nil
}

func NewShadowBoxFromFile(filename string) (*ShadowBox, error) {
	fi, err := os.Stat(filename)
	if err != nil {
		return nil, xerr.Tracef(err, "os.Stat() failed on file \"%s\"", filename)
	}

	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, xerr.Tracef(err, "ioutil.ReadFile() failed on file \"%s\"", filename)
	}

	sb := newShadowBox()
	sb.filename = filename
	sb.mtime = fi.ModTime()

	err = sb.initialize(content)
	if err != nil {
		return nil, xerr.Tracef(err, "initialize() failed on file \"%s\"", filename)
	}
	return sb, nil
}

func (sb *ShadowBox) Reload() (*ShadowBox, error) {
	if sb.filename != "" {
		fi, err := os.Stat(sb.filename)
		if err == nil && fi.ModTime() != sb.mtime {
			newsb, err := NewShadowBoxFromFile(sb.filename)
			return newsb, err
		}
	}
	return nil, nil
}

type _Section int

const (
	SCT_UNKNOWN _Section = iota
	SCT_SRP6A
	SCT_VERIFIER
)

func removeSpace(buf []byte) []byte {
	buf = bytes.TrimSpace(buf)
	bc := xstr.NewBytesCutter(buf)
	part := bc.NextPartFunc(unicode.IsSpace)
	if !bc.HasMore() {
		return part
	}

	bf := &bytes.Buffer{}
	bf.Write(part)
	for bc.HasMore() {
		bf.Write(bc.NextPartFunc(unicode.IsSpace))
	}
	return bf.Bytes()
}

func (sb *ShadowBox) _addItem(lineno int, section _Section, buf []byte) error {
	i := bytes.IndexByte(buf, '=')
	key := bytes.TrimSpace(buf[:i])
	value := bytes.TrimSpace(buf[i+1:])

	 /* internal parameters are ignored */
	if key[0] == '@' {
		return nil
	}

	if key[0] == '!' {
		 // temporarily change the section to SCT_VERIFIER for this item
                section = SCT_VERIFIER
		key = key[1:]
                if (len(key) == 0) {
			return xerr.Tracef(nil, "Invalid syntax on line %d", lineno)
		}
	}

	var n uint64
	var err error
	if section == SCT_SRP6A {
		// paramId = bits:g:N
		bc := xstr.NewBytesCutter(value)
		bits_ := bytes.TrimSpace(bc.NextPartByte(':'))
		gen_ := bytes.TrimSpace(bc.NextPartByte(':'))

		s := &_Srp6a{}
		n, err = strconv.ParseUint(string(bits_), 10, 16)
		if err != nil || n < 512 || n > 1024*32 {
                        return xerr.Tracef(n, "Invalid bits on line %d", lineno)
		}
		s.Bits = uint(n)

		s.Gen, err = strconv.ParseUint(string(gen_), 10, 64)
		if err != nil {
                        return xerr.Tracef(nil, "Invalid gen on line %d", lineno)
		}

		N_ := removeSpace(bc.Remain())
		s.N, err = hex.DecodeString(string(N_))
		if err != nil {
                        return xerr.Tracef(err, "Invalid N on line %d", lineno)
		}

		if int(s.Bits) != len(s.N) * 8 {
                        return xerr.Tracef(nil, "N too small or too large on line %d", lineno)
		}

		sb.srp6aMap[string(key)] = s

	} else if section == SCT_VERIFIER {
		// id = method:paramId:hashId:salt:verifier
		bc := xstr.NewBytesCutter(value)
		v := &_Verifier{}
		v.Method = string(bytes.TrimSpace(bc.NextPartByte(':')))
		v.ParamId = string(bytes.TrimSpace(bc.NextPartByte(':')))
		v.HashId = string(bytes.TrimSpace(bc.NextPartByte(':')))

		salt_ := bytes.TrimSpace(bc.NextPartByte(':'))
		v.Salt, err = base64.RawURLEncoding.DecodeString(string(salt_))
		if err != nil {
                        return xerr.Tracef(err, "Invalid salt on line %d", lineno)
		}

		verifier_ := removeSpace(bc.Remain())
		v.Verifier, err = base64.RawURLEncoding.DecodeString(string(verifier_))
		if err != nil {
                        return xerr.Tracef(err, "Invalid verifier on line %d", lineno)
		}

		sb.verifierMap[string(key)] = v

	} else {
		return xerr.Tracef(nil, "Section not specified until line %d", lineno)
	}

	return nil
}

func (sb *ShadowBox) _addInternalParameters() {
	N512_str :=
	"d4c7f8a2b32c11b8fba9581ec4ba4f1b04215642ef7355e37c0fc0443ef756ea"+
	"2c6b8eeb755a1c723027663caa265ef785b8ff6a9b35227a52d86633dbdfca43"

	N1024_str :=
	"eeaf0ab9adb38dd69c33f80afa8fc5e86072618775ff3c0b9ea2314c9c256576"+
	"d674df7496ea81d3383b4813d692c6e0e0d5d8e250b98be48e495c1d6089dad1"+
	"5dc7d7b46154d6b6ce8ef4ad69b15d4982559b297bcf1885c529f566660e57ec"+
	"68edbc3c05726cc02fd4cbf4976eaa9afd5138fe8376435b9fc61d2fc0eb06e3"

	N2048_str :=
	"ac6bdb41324a9a9bf166de5e1389582faf72b6651987ee07fc3192943db56050"+
	"a37329cbb4a099ed8193e0757767a13dd52312ab4b03310dcd7f48a9da04fd50"+
	"e8083969edb767b0cf6095179a163ab3661a05fbd5faaae82918a9962f0b93b8"+
	"55f97993ec975eeaa80d740adbf4ff747359d041d5c33ea71d281e446b14773b"+
	"ca97b43a23fb801676bd207a436c6481f1d2b9078717461a5b9d32e688f87748"+
	"544523b524b0d57d5ea77a2775d2ecfa032cfbdbf52fb3786160279004e57ae6"+
	"af874e7303ce53299ccc041c7bc308d82a5698f3a8d0c38271ae35f8e9dbfbb6"+
	"94b5c803d89f7ae435de236d525f54759b65e372fcd68ef20fa7111f9e4aff73"

	N4096_str :=
	"ffffffffffffffffc90fdaa22168c234c4c6628b80dc1cd129024e088a67cc74"+
	"020bbea63b139b22514a08798e3404ddef9519b3cd3a431b302b0a6df25f1437"+
	"4fe1356d6d51c245e485b576625e7ec6f44c42e9a637ed6b0bff5cb6f406b7ed"+
	"ee386bfb5a899fa5ae9f24117c4b1fe649286651ece45b3dc2007cb8a163bf05"+
	"98da48361c55d39a69163fa8fd24cf5f83655d23dca3ad961c62f356208552bb"+
	"9ed529077096966d670c354e4abc9804f1746c08ca18217c32905e462e36ce3b"+
	"e39e772c180e86039b2783a2ec07a28fb5c55df06f4c52c9de2bcbf695581718"+
	"3995497cea956ae515d2261898fa051015728e5a8aaac42dad33170d04507a33"+
	"a85521abdf1cba64ecfb850458dbef0a8aea71575d060c7db3970f85a6e1e4c7"+
	"abf5ae8cdb0933d71e8c94e04a25619dcee3d2261ad2ee6bf12ffa06d98a0864"+
	"d87602733ec86a64521f2b18177b200cbbe117577a615d6c770988c0bad946e2"+
	"08e24fa074e5ab3143db5bfce0fd108e4b82d120a92108011a723c12a787e6d7"+
	"88719a10bdba5b2699c327186af4e23c1a946834b6150bda2583e9ca2ad44ce8"+
	"dbbbc2db04de8ef92e8efc141fbecaa6287c59474e6bc05d99b2964fa090c3a2"+
	"233ba186515be7ed1f612970cee2d7afb81bdd762170481cd0069127d5b05aa9"+
	"93b4ea988d8fddc186ffb7dc90a6c08f4df435c934063199ffffffffffffffff"

	sb._addInternal("@512", 512, 2, N512_str)
	sb._addInternal("@1024", 1024, 2, N1024_str)
	sb._addInternal("@2048", 2048, 2, N2048_str)
	sb._addInternal("@4096", 4096, 5, N4096_str)
}

func (sb *ShadowBox) _addInternal(paramId string, bits uint, g uint64, Nstr string) {
	if paramId[0] != '@' {
		panic("Invalid internal paramId")
	}

	N, err := hex.DecodeString(Nstr)
	if err != nil {
		panic(err)
	}

	s := &_Srp6a{bits, g, N}
	sb.srp6aMap[paramId] = s
}

func (sb *ShadowBox) initialize(content []byte) error {
	bc := xstr.NewBytesCutter(content)
	section := SCT_UNKNOWN
	item_start := -1
	item_lineno := -1
	for lineno := 1; bc.HasMore(); lineno++ {
		lstart, lend := bc.NextIndexByte('\n')
		line := content[lstart:lend]
		line = bytes.TrimRight(line, " \n\r\t\v\f")
		if len(line) == 0 || line[0] == '#' {
			if item_start >= 0 {
				err := sb._addItem(item_lineno, section, content[item_start:lstart])
				if err != nil {
					return err
				}
			}
			item_start = -1
		} else if line[0] == '[' {
			if bytes.Equal(line, []byte("[SRP6a]")) {
				section = SCT_SRP6A
			} else if bytes.Equal(line, []byte("[verifier]")) {
				section = SCT_VERIFIER
			} else {
				section = SCT_UNKNOWN
				return xerr.Tracef(line, "Unknown section `%s` on line %d", line, lineno)
			}
			if item_start >= 0 {
				err := sb._addItem(item_lineno, section, content[item_start:lstart])
				if err != nil {
					return err
				}
			}
			item_start = -1
		} else {
			r, _ := utf8.DecodeRune(line)
			if unicode.IsSpace(r) {
				/* Do nothing.
				 * lines start with space are continued from previous line
				 */
			} else {
				if bytes.IndexByte(line, '=') <= 0 {
					return xerr.Tracef(line, "Invalid syntax on line %d", lineno)
				}
				item_start = lstart
				item_lineno = lineno
			}
		}
	}
	sb._addInternalParameters()
	return nil
}

func (sb *ShadowBox) Dump(w io.Writer) {
	io.WriteString(w, "[SRP6a]\n\n")
	for paramId, s := range sb.srp6aMap {
		if strings.HasPrefix(paramId, "@") {
			continue
		}

		fmt.Fprintf(w, "%s = %d:%d:\n", paramId, s.Bits, s.Gen)
		nstr := hex.EncodeToString(s.N)
		length := len(nstr)
		for i := 0; i < length; i += 64 {
			n := length - i
			if n > 64 {
				n = 64
			}
                        io.WriteString(w, "        ")
			io.WriteString(w, nstr[i:i+n])
			io.WriteString(w, "\n")
		}
		io.WriteString(w, "\n")
	}

	io.WriteString(w, "[verifier]\n\n")
	for id, v := range sb.verifierMap {
		salt := base64.RawURLEncoding.EncodeToString(v.Salt)
		fmt.Fprintf(w, "!%s = %s:%s:%s:%s:\n", id, v.Method, v.ParamId, v.HashId, salt)
		vstr := base64.RawURLEncoding.EncodeToString(v.Verifier)
		length := len(vstr)
		for i := 0; i < length; i += 64 {
			n := length - i
			if n > 64 {
				n = 64
			}
                        io.WriteString(w, "        ")
			io.WriteString(w, vstr[i:i+n])
			io.WriteString(w, "\n")
		}
		io.WriteString(w, "\n")
	}
}

func (sb *ShadowBox) GetContent() string {
	b := &bytes.Buffer{}
	sb.Dump(b)
	return string(b.Bytes())
}

func (sb *ShadowBox) GetSrp6a(paramId string) *_Srp6a {
	return sb.srp6aMap[paramId]
}

func (sb *ShadowBox) GetVerifier(identity string) *_Verifier {
	return sb.verifierMap[identity]
}


