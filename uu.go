/*
Implementation of Uuencoding https://en.wikipedia.org/wiki/Uuencoding
https://godoc.org/golang.org/x/text/transform#Transformer
https://godoc.org/golang.org/x/text/encoding#Encoding
*/
package uuencode

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"strings"

	"golang.org/x/text/transform"
)

// type simpleEncoding struct {
// 	Decoder transform.Transformer
// 	Encoder transform.Transformer
// }
//
// func (e *simpleEncoding) NewDecoder() *encoding.Decoder {
// 	return &encoding.Decoder{Transformer: e.Decoder}
// }
//
// func (e *simpleEncoding) NewEncoder() *encoding.Encoder {
// 	return &encoding.Encoder{Transformer: e.Encoder}
// }
//
// var (
// 	Uuencode encoding.Encoding = &simpleEncoding{
// 		uuBodyDec{},
// 		uuBodyEnc{},
// 	}
// )

// ErrBadUU is returned to indicate error during decoding
var (
	ErrBadUUDec = errors.New("uuencode: bad uuencode format (decoding)")
	errFoundEOF = errors.New("uuencode: found EOF marker")
)

const (
	// uuBase64 codec use to encode/decode uuencode. Note that this string start
	// with bit 1 as bit 0 can either be ' ' or the grave char '`'
	uuBase64      = `!"#$%&'()*+,-./0123456789:;<=>?@ABCDEFGHIJKLMNOPQRSTUVWXYZ[\]^_`
	uuOffset      = ' ' // space is the first ASCII char uuencode start
	uuPadding     = '`' // grave is used as first uuencode char (0 char) or padding
	uuBeginMarker = "begin"
	uuEndMarker   = "end"
	whiteSpace    = "\t\r\n"
	number        = "0123456789"
	eol           = "\n"
)

const (
	uuStart int = iota
	uuBody
	uuEnd
)

type UUDecFirstOne struct {
	uuBodyDec
	state      int
	Filename   string
	Permission int
}

// NewDecFirstOne return UUDecFirstOne which can find first encounter uuencode
// valid header and decode the body. The filename and file permission can be
// obtained from UUDecFirstOne after decoding finish. LIMITATION: Can not
// decode multiple uuencoded body. Only one begin..end pair will be converted.
func NewDecFirstOne() *UUDecFirstOne {
	return &UUDecFirstOne{
		state: uuStart,
	}
}

func (d *UUDecFirstOne) Transform(dst, src []byte, atEOF bool) (int, int, error) {
	var nDst, nSrc int
	maxLen := len(src)
	if maxLen == 0 {
		if d.state == uuEnd {
			return 0, 0, nil // good ending
		}
		return 0, 0, ErrBadUUDec
	}
	switch d.state {
	case uuStart:
		for n := 0; n < maxLen; n++ {
			// find EOL
			if src[n] != '\n' {
				continue
			}
			// found EOL
			if !bytes.HasPrefix(src[nSrc:n], []byte(uuBeginMarker)) {
				if len(dst[nDst:]) < len(src[nSrc:n+1]) {
					return nDst, nSrc, transform.ErrShortDst
				}
				m := copy(dst[nDst:], src[nSrc:n+1])
				nDst += m
				nSrc += m
				continue
			}
			nSrc = n + 1
			d.state = uuBody
			// get the file permission and filename here
			break
		}
		if d.state != uuBody {
			return nDst, nSrc, transform.ErrShortSrc
		}
		fallthrough
	case uuBody:
		mDst, mSrc, err := d.uuBodyDec.Transform(dst[nDst:], src[nSrc:], atEOF)
		nDst += mDst
		nSrc += mSrc
		if err != errFoundEOF {
			return nDst, nSrc, err
		}
		d.state = uuEnd
		fallthrough
	default:
		n := copy(dst[nDst:], src[nSrc:])
		if len(src[nSrc:]) > len(dst[nDst:]) {
			return nDst + n, nSrc + n, transform.ErrShortDst
		}
		return nDst + n, nSrc + n, nil
	}
}

func (d *UUDecFirstOne) Reset() {
	d.state = uuStart
	d.Permission = 0
	d.Filename = ""
}

type uuBodyDec struct {
	transform.NopResetter
}

// Transform implement transform.Transform and it output errFoundEOF when
// discover uuencode end marker. It do not maintenance any state. So, any call
// after errFoundEOF will continue deocoding and most likely output error if the
// next line is not a valid uuencode formatted line.
func (uuBodyDec) Transform(dst, src []byte, atEOF bool) (int, int, error) {
	var nDst, nSrc, linelen int
	srclen := len(src)
	for nSrc < srclen {
		m := strings.Index(string(src[nSrc:]), "\n")
		if m < 0 {
			return nDst, nSrc, transform.ErrShortSrc
		}
		b := src[nSrc : nSrc+m]
		if b[0] == uuPadding {
			// uuPadding grave mean 0 total bytes, checking ending procedure
			endlen := nSrc + m + 1
			if endlen > len(src) {
				return nDst, nSrc, transform.ErrShortSrc
			}
			m = strings.Index(string(src[endlen:]), "\n")
			if m < 0 {
				if atEOF && string(src[endlen:]) == uuEndMarker {
					// take care of uuencode that end without LF
					return nDst, endlen + len(src[endlen:]), errFoundEOF
				}
				return nDst, nSrc, transform.ErrShortSrc
			}
			b = src[endlen : endlen+m]
			linelen = len(b)
			if b[linelen-1] == '\r' {
				b = b[:linelen-1]
			}
			nSrc = endlen + m + 1
			if string(b) == uuEndMarker {
				return nDst, nSrc, errFoundEOF
			}
			// can not has grave (end) marker but without the "end\n" word
			return nDst, nSrc, ErrBadUUDec
		}
		linelen = len(b)
		if b[linelen-1] == '\r' {
			b = b[:linelen-1]
			linelen--
		}
		linelen-- // first byte is total bytes count which should be removed
		if linelen%4 != 0 {
			return nDst, nSrc, ErrBadUUDec
		}
		tmp := linelen / 4 * 3 // total expected decoded chars (include padding)
		if tmp > len(dst) {
			return nDst, nSrc, transform.ErrShortDst
		} else if realTotal := int(b[0] - uuOffset); tmp < realTotal {
			// not enough uuencoded characters to generate origin characters
			return nDst, nSrc, ErrBadUUDec
		} else {
			tmp -= realTotal // get the total zero bit bytes (padding bytes)
			if tmp > 2 {
				// padding can only either 0, 1 or 2
				return nDst, nSrc, ErrBadUUDec
			}
		}
		nSrc += m + 1 // total bytes read, +1 to include the 0x0a char
		b = b[1:]     // remove the first byte from data bytes
		nDst += miniConvert(dst[nDst:], b)
		nDst -= tmp // tmp hold the total padding bytes
	}
	if nSrc != srclen {
		return nDst, srclen, ErrBadUUDec
	}
	return nDst, nSrc, nil
}

func miniConvert(out []byte, in []byte) int {
	var totalConvert int
	for i := 0; i < len(in); i += 4 {
		tmp1 := getOffset(in[i+1])
		out[totalConvert] = (getOffset(in[i+0]) << 2) | ((0x30 & tmp1) >> 4)
		tmp2 := getOffset(in[i+2])
		out[totalConvert+1] = (tmp1 << 4) | ((0x3c & tmp2) >> 2)
		tmp1 = getOffset(in[i+3])
		out[totalConvert+2] = (tmp2 << 6) | (0x3f & tmp1)
		totalConvert += 3
	}
	return totalConvert
}

func getOffset(c byte) byte {
	if c != uuPadding {
		return c - uuOffset
	}
	return 0
}

// HasUuencode quick inefficient hack to check if r contains uuencode contents.
// It go through the whole transformation, so might as well do the transform.
func HasUuencode(r io.Reader) bool {
	r = transform.NewReader(r, NewDecFirstOne())
	_, err := ioutil.ReadAll(r)
	if err == nil {
		return true
	}
	return false
}
