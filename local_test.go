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

func Test_miniConvert(t *testing.T) {
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

var tstMiniEncodeData = []struct {
	n       int
	grave   bool
	in, out string
}{
	{
		n:     0,
		grave: true,
		in:    "Cat",
		out:   "0V%T",
	},
	{
		n:     0,
		grave: true,
		in:    "http://www.wikipedia.org\r\n",
		out:   ":'1T",
	},
	{
		n:     0,
		grave: true,
		in:    "p://www.wikipedia.org\r\n",
		out:   "<#HO",
	},
	{
		n:     0,
		grave: true,
		in:    "/www.wikipedia.org\r\n",
		out:   "+W=W",
	},
	{
		n:     0,
		grave: true,
		in:    "w.wikipedia.org\r\n",
		out:   "=RYW",
	},
	{
		n:     0,
		grave: true,
		in:    "ikipedia.org\r\n",
		out:   ":6MI",
	},
	{
		n:     0,
		grave: true,
		in:    "pedia.org\r\n",
		out:   "<&5D",
	},
	{
		n:     0,
		grave: true,
		in:    "ia.org\r\n",
		out:   ":6$N",
	},
	{
		n:     0,
		grave: true,
		in:    "org\r\n",
		out:   ";W)G",
	},
	{
		n:     2,
		grave: true,
		in:    "o",
		out:   ";P``",
	},
	{
		n:     1,
		grave: true,
		in:    "\r\n",
		out:   "#0H`",
	},
	{
		n:     1,
		grave: false,
		in:    "\r\n",
		out:   "#0H ",
	},
}

func Test_miniEncode(t *testing.T) {
	for _, d := range tstMiniEncodeData {
		var out [4]byte
		miniEncode(out[:], []byte(d.in), d.n, d.grave)
		if string(out[:]) != d.out {
			t.Errorf("Want: %s\n Got: %s", d.out, string(out[:]))
		}
	}
}

var tstLineEncodeData = []struct {
	grave   bool
	in, out string
}{
	{
		grave: true,
		in:    "Cat",
		out:   "0V%T",
	},
	{
		grave: true,
		in:    "http://www.wikipedia.org\r\n",
		out:   ":'1T<#HO+W=W=RYW:6MI<&5D:6$N;W)G#0H`",
	},
}

func TestLineEncode(t *testing.T) {
	for _, d := range tstLineEncodeData {
		out := make([]byte, len(d.out))
		lineEncode(out, []byte(d.in), len(d.in), d.grave)
		if string(out) != d.out {
			t.Errorf("Want: %s\n Got: %s", d.out, string(out))
		}
	}
}
