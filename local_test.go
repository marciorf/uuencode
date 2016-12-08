package uuencode

import "testing"

var tstMiniConvertData = []struct {
	in, out string
}{
	{
		in:  "0V%T",
		out: "Cat",
	},
	{
		in:  ":'1T<#HO+W=W=RYW:6MI<&5D:6$N;W)G#0H`",
		out: "http://www.wikipedia.org\r\n",
	},
}

func TestMiniConvert(t *testing.T) {
	for _, d := range tstMiniConvertData {
		outlen := len(d.out)
		out := make([]byte, outlen+2)
		miniConvert(out, []byte(d.in))
		out = out[:outlen]
		if string(out) != d.out {
			t.Errorf("Want: %s\n Got: %s", d.out, string(out))
		}
	}
}
