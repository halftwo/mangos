package xstr

import (
	"testing"
)

func TestEasyParser1(t *testing.T) {
	str := "size=   93952kB time=01:06:00.25 bitrate= 194.3kbits/s speed= 792x    "
	ep := NewEasyParser(str)
	ep.Next("size=")
	size := ep.NextInt("kB time=")
	ep.Next(" bitrate=")
	bs := ep.NextFloat("kbits/s")
	if size != 93952 || bs != 194.3 {
		t.Fatal("Parse error", ep.Err)
	}
}

func TestEasyParser2(t *testing.T) {
	str := "  Stream #0:0(eng): Video: hevc (Main), yuv420p(tv, progressive), 1280x720 [SAR 1:1 DAR 16:9], 23.98 fps, 23.98 tbr, 1k tbn, 23.98 tbc (default)"
	ep := NewEasyParser(str)
	ep.Next(":")					// Stream #0
        num, lang := ep.NextQuote("(", "): ", ": ")	// 0 eng
	media := ep.Next(": ")				// Video
	codec, _ := ep.NextExtra(" (", ", ")		// hevc
	mod, _ := ep.NextQuote("(", "), ", ", ")	// yuv420p(tv, progressive),
        resolution, _ := ep.NextQuote(" [", "], ", ", ")      // 1280x720 [SAR 1:1 DAR 16:9],
	fps := ep.NextFloat("fps, ")			// 23.98
	if num != "0" || lang != "eng" || media != "Video" || codec != "hevc" ||
			mod != "yuv420p" || resolution != "1280x720" || fps != 23.98 {
		t.Fatal("Parse error", ep.Err)
	}
}
