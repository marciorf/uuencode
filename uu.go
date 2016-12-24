/*
Package uuencode implements https://en.wikipedia.org/wiki/Uuencoding
https://godoc.org/golang.org/x/text/transform#Transformer
https://godoc.org/golang.org/x/text/encoding#Encoding
*/
package uuencode

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/text/encoding"
	"golang.org/x/text/transform"
)

type uuEncoding struct{}

// Uue implment encoding.Encoding interface.
var Uue = uuEncoding{}

// NewDecoder implments encoding.Decoder. It only decodes first encountered
// uuencode begin header line.
func (uuEncoding) NewDecoder() *encoding.Decoder {
	return &encoding.Decoder{
		Transformer: NewDecode(),
	}
}

// NewEncoder implements encoding.Encoder.
func (uuEncoding) NewEncoder() *encoding.Encoder {
	return &encoding.Encoder{
		Transformer: NewEncode(true, "\n"),
	}
}

var (
	// ErrBadUUDec is returned to indicate error during decoding
	ErrBadUUDec = errors.New("uuencode: bad uuencode format (decoding)")
	// ErrBadLen indictes decoding process fail because of single line too long
	// without end of line (\n \r\n)
	ErrBadLen = errors.New("uuencode: line too long (decoding)")
	// ErrUuCancel indicates there is a cancelation request triggered
	// internnally that stop the transforming process.
	ErrUuCancel = errors.New("uuencode: decoder cancel processing")
	// errFoundEOF is used internnally to indicate end line marker found for one
	// section of uuencoded contents.
	errFoundEOF = errors.New("uuencode: found EOF marker")
)

const (
	uuOffset = ' ' // space is the first ASCII char uuencode start
	// grave is used as first uuencode char (0 char) or padding
	uuPadding     = '`'
	uuBeginMarker = "begin"
	uuEndMarker   = "end"
	maxSingleLine = 45
	maxEncLine    = 61
	// max characters per line is marked as M in uuencoding.
	maxMarker = 'M'
)

const (
	uuStart int = iota
	uuBody
	uuEnd
)

// Decode implements transform.Transformer for single decoding uuencoded
// content. For multiple uuencoded contents, use Get method to get the result.
type Decode struct {
	uuBodyDec
	multi    bool
	multiErr error
	cancel   chan struct{}
	internal []byte
	ch       chan io.ReadCloser
	sync.Mutex
	pipeR      *io.PipeReader
	pipeW      *io.PipeWriter
	warn       int
	state      int
	Filename   string
	Permission string
}

const defaultMaxBuff = 4096

// NewMultiDecode return Decode that decode all uuencode contents. It return
// three args - Decode pointer, cancel function and io.ReadCloser chan. cancel
// function is used to unblock the Transform method. io.ReadCloser contains the
// decoded contents.
func NewMultiDecode() (*Decode, func(), <-chan io.ReadCloser) {
	c := make(chan io.ReadCloser)
	// cancel channel is used to quit the blocking process
	csign := make(chan struct{})
	d := &Decode{
		multi:  true,
		cancel: csign,
		ch:     c,
	}
	return d, func() {
		close(csign)
		d.closePipe()
	}, c
}

// NewDecode return Decode decode first encounter uuencoded content.
func NewDecode() *Decode {
	return &Decode{}
}

// Transform implment golang/x/text/transform.Transformer interface for single
// uuencoded content.
//
// For multiple uuencoded contents, Transform will block. dst will out any
// content that isn't belong to uuencoded body. Refer to Get method for decoded
// uuencoded contents.
func (d *Decode) Transform(dst, src []byte, atEOF bool) (int, int, error) {
	var nDst, nSrc int
	maxLen := len(src)
	if maxLen == 0 {
		if d.state == uuEnd || d.multi && d.state == uuStart {
			return 0, 0, nil // good ending
		}
		return 0, 0, ErrBadUUDec
	}
	for {
		switch d.state {
		case uuStart:
			// search the begin header line
			for n := nSrc; n < maxLen; n++ {
				// find EOL
				if src[n] != '\n' {
					continue
				}
				// found EOL
				begin := src[nSrc:n]
				if !bytes.HasPrefix(begin, []byte(uuBeginMarker)) {
					if len(dst[nDst:]) < len(src[nSrc:n+1]) {
						return nDst, nSrc, transform.ErrShortDst
					}
					m := copy(dst[nDst:], src[nSrc:n+1])
					nDst += m
					nSrc += m
					continue
				}
				lastIndex := len(begin) - 1
				if begin[lastIndex] == '\r' {
					begin = begin[:lastIndex]
				}
				// get the file permission and filename here
				as := strings.Split(string(begin), " ")
				aslen := len(as)
				if aslen > 2 {
					d.Filename = as[2]
				}
				if aslen > 1 {
					if _, err := strconv.Atoi(as[1]); err == nil {
						d.Permission = as[1]
					}
				}
				nSrc = n + 1
				d.state = uuBody
				break
			}
			if d.state != uuBody {
				if nSrc != 0 {
					return nDst, nSrc, transform.ErrShortSrc
				}
				// nSrc not move and n == maxlen == maximun available internal
				// buffer
				if !strings.HasPrefix(string(src[nSrc:]), "begin") {
					return nDst, nSrc, ErrBadUUDec
				}
				return nDst, nSrc, ErrBadLen
			} else if d.multi {
				// if multi decoding uuencoded contents, then create piped files
				// which allow this method to pass the decoded contents to
				// another chan and the process state is controlled through the
				// chan.
				d.multiErr = nil
				r, w := io.Pipe()
				d.Lock()
				d.pipeR = r
				d.pipeW = w
				d.Unlock()
				select {
				case d.ch <- r:
				case <-d.cancel:
					d.closePipe()
					return nDst, nSrc, ErrUuCancel
				}
			}
			fallthrough
		case uuBody:
			// after the begin header line found, here start the real uuencoded
			// decoding process.
			mDst, mSrc, err := d.uuBodyDec.Transform(dst[nDst:], src[nSrc:],
				atEOF)
			nSrc += mSrc
			if d.multi && d.multiErr == nil {
				wdst := dst[nDst:]
				// if err == transform.ErrShortDst && mDst == 0 && mSrc == 0 {
				// 	// ErrShortDst can not be propagate out without adv
				// 	// dst and src length.
				// 	if d.internal == nil {
				// 		d.internal = make([]byte, defaultMaxBuff)
				// 	}
				// 	wdst = d.internal
				// 	mDst, mSrc, err = d.uuBodyDec.Transform(d.internal,
				// 		src[nSrc:], atEOF)
				// 	nSrc += mSrc
				// }
				if mDst > 0 {
					select {
					case <-d.cancel:
						d.closePipe()
						return nDst, nSrc, ErrUuCancel
					default:
						_, werr := d.pipeW.Write(wdst[:mDst])
						if werr != nil {
							if werr == ErrUuCancel {
								return nDst, nSrc, werr
							}
							d.multiErr = werr
						}
					}
				}
			} else {
				nDst += mDst
			}
			if err != errFoundEOF {
				return nDst, nSrc, err
			} else if d.multi {
				d.state = uuStart
				d.pipeW.Close()
				continue
			}
			d.state = uuEnd
			fallthrough
		default:
			// only single uuencoded decode process will fall through here. Any
			// extra bytes after the end line encounter will be outputted
			// plainly without transform.
			n := copy(dst[nDst:], src[nSrc:])
			if len(src[nSrc:]) > len(dst[nDst:]) {
				return nDst + n, nSrc + n, transform.ErrShortDst
			}
			return nDst + n, nSrc + n, nil
		}
	}
}

// closePipe close the piped file that transferring the decoded bytes to another
// goroutine to be expected to be read out. Piped file internally use mutex to
// handle the synchronization, so it is safe to call the provided Close method
// in any goroutine.
func (d *Decode) closePipe() {
	d.Lock()
	if d.pipeW != nil {
		d.pipeW.CloseWithError(ErrUuCancel)
	}
	if d.pipeR != nil {
		d.pipeR.CloseWithError(ErrUuCancel)
	}
	d.Unlock()
}

// Reset implements golang/x/text/transform.Transformer interface. It reset the
// transform internal state. Only useful for single decoding process. For
// multiple uuencoded contents deocding, it does nothing on reseting the reading
// chan of decoded contents.
func (d *Decode) Reset() {
	d.state = uuStart
	d.Permission = ""
	d.Filename = ""
}

// Close closes the returned io.ReadCloser chan from NewMultiDecode.
func (d *Decode) Close() {
	if d.multi {
		close(d.ch)
	}
}

type uuBodyDec struct {
	transform.NopResetter
}

const maxUuDecLine = 64

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
			if len(src[nSrc:]) > maxUuDecLine {
				return nDst, nSrc, ErrBadLen
			}
			return nDst, nSrc, transform.ErrShortSrc
		}
		b := src[nSrc : nSrc+m]
		if b[0] == uuPadding {
			// uuPadding grave mean 0 total bytes, checking ending procedure
			endlen := nSrc + m + 1
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
		} else if b[0] < uuOffset || b[0] > uuPadding {
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
		nSrc += m + 1 // total bytes read, +1 to include the \n char
		b = b[1:]     // remove the first byte from data bytes
		nDst += miniConvert(dst[nDst:], b)
		nDst -= tmp // tmp hold the total padding bytes
	}
	return nDst, nSrc, nil
}

// miniConvert converts each minimum quanta bytes of uuencoded contents into
// actual content. Uuencoding has the same base64 decoded length that is 4 to 3.
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

// getOffset get the number of bytes of the line. This information carries on
// first character of the line.
func getOffset(c byte) byte {
	if c != uuPadding {
		return c - uuOffset
	}
	return 0
}

// HasUuencode quick inefficient hack to check if r contains uuencode contents.
// It go through the whole transformation, so might as well do the transform.
func HasUuencode(r io.Reader) bool {
	r = transform.NewReader(r, Uue.NewDecoder())
	_, err := ioutil.ReadAll(r)
	if err == nil {
		return true
	}
	return false
}

// NewEncode return *Encode that can convert bytes into uuencode format.
// useGrave uses grave as padding and replace all space with grave character.
// eol determine the end of line pattern, eg: \r\n or \n. option provide(s) file
// name (first) or permission (second) to be outputted as begin line.
func NewEncode(useGrave bool, eol string, option ...string) *Encode {
	// if no filename provided in option then default it to `filename`
	name := "filename"
	// if no permission bits supplied then default it to 644
	permit := "644"
	optlen := len(option)
	if optlen > 1 {
		// more than 1 option mean the file name or permission is supplied.
		name = option[0]
		permit = option[1]
	} else if optlen > 0 {
		// first option is the file name
		name = option[0]
	}
	return &Encode{
		uuBodyEnc: uuBodyEnc{
			useGrave: useGrave,
			eol:      eol,
		},
		state:  uuStart,
		permit: permit,
		name:   name,
	}
}

// Encode encodes bytes into uuencode format and implement
// transform.Transformer.
type Encode struct {
	uuBodyEnc
	state        int
	permit, name string
}

// Transform implements transform.Transformer.
func (e *Encode) Transform(dst, src []byte, atEOF bool) (int, int, error) {
	var nDst int
	switch e.state {
	case uuStart:
		// encoding start with creating the begin line of uuencoded which
		// consist of `begin <file permission mode> filename`
		startline := fmt.Sprint(uuBeginMarker, " ", e.permit, " ", e.name,
			e.eol)
		if len(startline) > len(dst) {
			return 0, 0, transform.ErrShortDst
		}
		nDst = copy(dst, []byte(startline))
		e.state = uuBody
		fallthrough
	default:
		// this is the main uuencode encoding process
		m, n, err := e.uuBodyEnc.Transform(dst[nDst:], src, atEOF)
		return nDst + m, n, err
	}
}

// Reset implements transform.Transformer to reset internal state of Encode eg:
// begin marker will be output again for the next transformation start.
func (e *Encode) Reset() {
	e.state = uuStart
}

// ResetAll call Reset and also reset the file name and permission bit at begin
// header marker to the value provided by name and permit respectively.
func (e *Encode) ResetAll(permit, name string) {
	e.name = name
	e.permit = permit
	e.Reset()
}

type uuBodyEnc struct {
	useGrave bool   // indicate using ` as zero bits instead of space
	eol      string // end of line string eg \n or \r\n
	transform.NopResetter
}

// uuBodyEnc implements transform.Transformer converting src to uuencoded bytes
// store inside dst. It outputs uuencoded end marker at the end of transform
// where atEOF is true.
func (u uuBodyEnc) Transform(dst, src []byte, atEOF bool) (int, int, error) {
	var nDst, nSrc int
	srclen := len(src)
	eollen := len(u.eol)
	for nSrc+maxSingleLine <= srclen {
		// check if the dst buffer enough for decoded contents to be stored.
		if len(dst[nDst:]) < maxEncLine+eollen {
			return nDst, nSrc, transform.ErrShortDst
		}
		dst[nDst] = maxMarker
		// encode the content into lines of uuencoded lines.
		lineEncode(dst[nDst+1:], src[nSrc:], maxSingleLine, u.useGrave)
		nSrc += maxSingleLine
		nDst += maxEncLine
		nDst += copy(dst[nDst:], []byte(u.eol))
	}
	if atEOF {
		// create the end line marker that base on uuencode spec.
		endline := fmt.Sprint(u.eol, "`", u.eol, uuEndMarker, u.eol)
		eollen = len(endline)
		srclen = len(src[nSrc:])
		expectedLen := srclen / 3
		if srclen%3 > 0 {
			expectedLen++
		}
		expectedLen = expectedLen*4 + 1
		if len(dst[nDst:]) < expectedLen+eollen {
			return nDst, nSrc, transform.ErrShortDst
		}
		dst[nDst] = byte(srclen) + uuOffset
		lineEncode(dst[nDst+1:], src[nSrc:], srclen, u.useGrave)
		nSrc += srclen
		nDst += expectedLen
		nDst += copy(dst[nDst:], []byte(endline))
	} else {
		return nDst, nSrc, transform.ErrShortSrc
	}
	return nDst, nSrc, nil
}

// lineEncode encode max 45 bytes data into uuconded data.
func lineEncode(dst []byte, src []byte, n int, useGrave bool) {
	r := n % 3
	if r > 0 {
		n -= r
		r = 3 - r
	}
	var i, j int
	for i = 0; i < n; i += 3 {
		// encoding without padding
		miniEncode(dst[j:], src[i:], 0, useGrave)
		j += 4
	}
	if r > 0 {
		// encoding that need padding
		miniEncode(dst[j:], src[i:], r, useGrave)
	}
}

// miniEncode encode 3 bytes into 4 bytes uuencoded data. dst store the result
// of encoded bytes. src is the source of bytes that need to be encoded. n is
// total number of padding.
func miniEncode(dst []byte, src []byte, n int, useGrave bool) {
	dst[0] = src[0] & 0xfc >> 2
	dst[0] += uuOffset
	var secondp1, secondp2, thirdp1, thirdlast byte
	// if n < 2 {
	// 	secondp1 = src[1] & 0xf0 >> 4
	// 	secondp2 = src[1] & 0x0f << 2
	// }
	// if n < 3 {
	// 	thirdp1 = src[2] & 0x03 >> 6
	// 	thirdlast = src[2] & 0x3f
	// }
	if n < 1 {
		thirdp1 = src[2] & 0xc0 >> 6
		thirdlast = src[2] & 0x3f
		secondp1 = src[1] & 0xf0 >> 4
		secondp2 = src[1] & 0x0f << 2
	} else if n < 2 {
		secondp1 = src[1] & 0xf0 >> 4
		secondp2 = src[1] & 0x0f << 2
	}
	dst[1] = src[0]&0x03<<4 | secondp1
	dst[1] += uuOffset
	dst[2] = secondp2 | thirdp1
	dst[2] += uuOffset
	dst[3] = thirdlast
	dst[3] += uuOffset
	if useGrave {
		for i := 0; i < 4; i++ {
			if dst[i] == uuOffset {
				dst[i] = uuPadding
			}
		}
	}
}
