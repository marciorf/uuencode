package uuencode_test

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/kylelemons/godebug/pretty"
	"github.com/sanylcs/uuencode"
	"golang.org/x/text/transform"
)

const tstFolder = "test-data"

// readTestFile is a test utility function to io.Reader of test file. All test
// files must in the format of name.in for input and name.out for output compare.
func readTestFile(path, filename string) io.ReadCloser {
	raw, err := os.Open(filepath.Join(tstFolder, path, filename))
	if err != nil {
		panic(fmt.Sprintf("Failed to open test file: %v", err))
	}
	return raw
}

// readInputFile read file and expecting the file has .in extension.
func readInputFile(p, f string) io.ReadCloser {
	return readTestFile(p, f+".in")
}

// readOutputFile read file and expecting the file has .out extension.
func readOutputFile(p, f string) io.ReadCloser {
	return readTestFile(p, f+".out")
}

// remove any privTestx as it only work for developer PC.
var testfiles1 = []string{
	"test1", "test2",
	//"privTest1",
}

const tDecFirstOne = "testDecFirstOne"

// TestDecFirstOne test the uuencoding decoding transforming.
func TestDecFirstOne(t *testing.T) {
	for _, f := range testfiles1 {
		rc := readInputFile(tDecFirstOne, f)
		defer rc.Close()
		u := uuencode.Uue.NewDecoder()
		r := transform.NewReader(rc, u)
		got, err := ioutil.ReadAll(r)
		if err != nil {
			t.Fatalf("Test TestDecFirstOne in file=%s failed with err=%v",
				f, err)
		}
		rc = readOutputFile(tDecFirstOne, f)
		expect, err := ioutil.ReadAll(rc)
		if err != nil {
			t.Fatalf("Test TestDecFirstOne out file=%s failed with err=%v",
				f, err)
		}
		if diff := pretty.Compare(string(got), string(expect)); diff != "" {
			t.Errorf("Diff: %s", diff)
			// t.Errorf("Want: %s\nGot: %s\nDiff: %s",
			// 	string(expect), string(got), diff)
		}
	}
}

func TestDecFirstOne2(t *testing.T) {
	// Test dst limit
	tf := uuencode.Uue.NewDecoder()
	src := []byte("some bytes without prefix begin\n")
	dst := [4]byte{}
	_, _, err := tf.Transform(dst[:], src, true)
	if err == nil {
		t.Error("Test TestDecFirstOne2 expected error return no error")
	} else if err != transform.ErrShortDst {
		t.Error("Test TestDecFirstOne2 expect return transform.ErrShortDst")
	}
}

func TestDecFirstOne3(t *testing.T) {
	// Test dst limit
	tf := uuencode.Uue.NewDecoder()
	src := []byte("begin 644 file.txt\n#0V%T\n`\nend\n")
	dst := [2]byte{}
	_, _, err := tf.Transform(dst[:], src, true)
	if err == nil {
		t.Error("expected error return no error")
	} else if err != transform.ErrShortDst {
		t.Error("expect return transform.ErrShortDst but", err)
	}
}

func TestDecFirstOne4(t *testing.T) {
	// Test dst limit
	tf := uuencode.Uue.NewDecoder()
	src := []byte("begin 644 file.txt\n#0V%T\n`\nend\nadditional bytes\n")
	dst := [4]byte{}
	_, _, err := tf.Transform(dst[:], src, true)
	if err == nil {
		t.Error("expected error return no error")
	} else if err != transform.ErrShortDst {
		t.Error("expect return transform.ErrShortDst but", err)
	}
}

const tErrDFO = "testErrDFO"

var testfiles2 = []string{
	"test1", "test2", "test3", "test4", "test5", "test6", "test7", "test8",
	"test9", "test10",
}

func TestErrDFO(t *testing.T) {
	for _, f := range testfiles2 {
		rc := readTestFile(tErrDFO, f+".err")
		defer rc.Close()
		u := uuencode.Uue.NewDecoder()
		r := transform.NewReader(rc, u)
		_, err := ioutil.ReadAll(r)
		if err == nil {
			t.Errorf("Test TestErrDFO file=%s failed as expected err return no error",
				f)
		}
	}
}

var testData1 = []struct {
	path, file string
	has        bool
}{
	{path: tDecFirstOne, file: "test1.in", has: true},
	{path: tDecFirstOne, file: "test2.in", has: true},
	{path: tErrDFO, file: "test1.err", has: false},
	{path: tErrDFO, file: "test2.err", has: false},
	{path: tErrDFO, file: "test3.err", has: false},
	{path: tErrDFO, file: "test4.err", has: false},
	//{path: tDecFirstOne, file: "privTest1.in", has: true},
}

func TestHasUuencode(t *testing.T) {
	for _, d := range testData1 {
		rc := readTestFile(d.path, d.file)
		defer rc.Close()
		got := uuencode.HasUuencode(rc)
		if got != d.has {
			t.Errorf("Test TestHasUuencode Got=%v Wanted=%v", got, d.has)
		}
	}
}

// remove any privTestx as it only work for developer PC.
var testfiles3 = []struct {
	grave     bool
	eol, file string
}{
	{grave: true, eol: "\r\n",
		file: "test1"},
	// {grave: true, eol: "\r\n",
	// 	file: "privTest1"},
}

const tEncode = "testEncode"

func TestEncode1(t *testing.T) {
	for _, f := range testfiles3 {
		rc := readInputFile(tEncode, f.file)
		defer rc.Close()
		r := transform.NewReader(rc, uuencode.NewEncode(f.grave, f.eol))
		got, err := ioutil.ReadAll(r)
		if err != nil {
			t.Fatalf("Test TestEncode in file=%s failed with err=%v",
				f.file, err)
		}
		rc = readOutputFile(tEncode, f.file)
		defer rc.Close()
		expect, err := ioutil.ReadAll(rc)
		if err != nil {
			t.Fatalf("Test TestEncode out file=%s failed with err=%v",
				f.file, err)
		}
		if diff := pretty.Compare(string(got), string(expect)); diff != "" {
			t.Errorf("Diff: %s", diff)
			// t.Errorf("Want: %s\nGot: %s\nDiff: %s",
			// 	string(expect), string(got), diff)
		}
	}
}

func TestEncode2(t *testing.T) {
	// Test dst limit
	tf := uuencode.NewEncode(true, "\n")
	src := []byte("some stupid bytes")
	dst := [4]byte{}
	_, _, err := tf.Transform(dst[:], src, true)
	if err == nil {
		t.Error("expected error return no error")
	} else if err != transform.ErrShortDst {
		t.Error("expect return transform.ErrShortDst but", err)
	}
}

func TestEncode2_1(t *testing.T) {
	// Test dst limit
	tf := uuencode.NewEncode(true, "\n")
	src := []byte("some stupid bytes must more than 45 characters length")
	dst := [20]byte{}
	_, _, err := tf.Transform(dst[:], src, true)
	if err == nil {
		t.Error("expected error return no error")
	} else if err != transform.ErrShortDst {
		t.Error("expect return transform.ErrShortDst but", err)
	}
}

func TestEncode2_2(t *testing.T) {
	// Test dst limit
	tf := uuencode.NewEncode(true, "\n")
	src := []byte("some stupid bytes")
	dst := [20]byte{}
	_, _, err := tf.Transform(dst[:], src, true)
	if err == nil {
		t.Error("expected error return no error")
	} else if err != transform.ErrShortDst {
		t.Error("expect return transform.ErrShortDst but", err)
	}
}

func TestEncode3(t *testing.T) {
	// Test custom Encode file name
	br := bytes.NewBufferString("I love you forever.")
	want := []byte("begin 644 pp.txt\n322!L;W9E('EO=2!F;W)E=F5R+@``\n`\nend\n")
	r := transform.NewReader(br, uuencode.NewEncode(true, "\n", "pp.txt"))
	got, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatal("err:", err)
	}
	if diff := pretty.Compare(string(got), string(want)); diff != "" {
		t.Errorf("Diff: %s", diff)
	}
}

func TestEncode4(t *testing.T) {
	// Test custom Encode file name and permission
	br := bytes.NewBufferString("I love you forever.")
	want := []byte("begin 777 pp.txt\n322!L;W9E('EO=2!F;W)E=F5R+@``\n`\nend\n")
	r := transform.NewReader(br,
		uuencode.NewEncode(true, "\n", "pp.txt", "777"))
	got, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatal("err:", err)
	}
	if diff := pretty.Compare(string(got), string(want)); diff != "" {
		t.Errorf("Diff: %s", diff)
	}
}

func TestEncode5(t *testing.T) {
	// Test Encode ResetAll
	const tstStr = "I love you forever."
	br := bytes.NewBufferString(tstStr)
	w1 := []byte("begin 777 pp.688\n322!L;W9E('EO=2!F;W)E=F5R+@``\n`\nend\n")
	e := uuencode.NewEncode(true, "\n", "pp.txt")
	r := transform.NewReader(br, e)
	got, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatal("err at first encode:", err)
	}
	e.ResetAll("777", "pp.688")
	br = bytes.NewBufferString(tstStr)
	r = transform.NewReader(br, e)
	got, err = ioutil.ReadAll(r)
	if err != nil {
		t.Fatal("err at 2nd encode:", err)
	}
	if diff := pretty.Compare(string(got), string(w1)); diff != "" {
		t.Errorf("Diff: %s", diff)
	}
}

const tEncDecSize = 5000

func TestEncodeDecode(t *testing.T) {
	src := make([]byte, tEncDecSize)
	for i, _ := range src {
		src[i] = byte(i + 1)
	}
	br := bytes.NewBuffer(src)
	r := transform.NewReader(br, uuencode.Uue.NewEncoder())
	got, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatal("err at encoding read all:", err)
	}
	br = bytes.NewBuffer(got)
	r = transform.NewReader(br, uuencode.Uue.NewDecoder())
	got, err = ioutil.ReadAll(r)
	if err != nil {
		t.Fatal("err at decoding read all:", err)
	}
	if diff := pretty.Compare(string(got), string(src)); diff != "" {
		t.Errorf("Diff: %s", diff)
	}
}

const tMultiEncDecSize1 = 1000
const tMultiEncDecSize2 = 3000

func TestMultiEncodeMultiDecode(t *testing.T) {
	src1 := make([]byte, tMultiEncDecSize1)
	for i, _ := range src1 {
		src1[i] = byte(i + 1)
	}
	br := bytes.NewBuffer(src1)
	tf := uuencode.Uue.NewEncoder()
	r := transform.NewReader(br, tf)
	bs := new(bytes.Buffer)
	_, err := io.Copy(bs, r)
	if err != nil {
		t.Fatal("error at copy data from reader")
	}
	w := transform.NewWriter(bs, tf)
	src2 := make([]byte, tMultiEncDecSize2)
	for i, _ := range src2 {
		src2[i] = byte(i * 9)
	}
	_, err = w.Write(src2)
	if err != nil {
		t.Fatal("error at writing pre-gen data to writer")
	}
	w.Close()
	d, _, ch := uuencode.NewMultiDecode()
	var wait sync.WaitGroup
	wait.Add(1)
	go func() {
		var (
			err  error
			gotx [2][]byte
			i    int
		)
		for r := range ch {
			gotx[i], err = ioutil.ReadAll(r)
			if err != nil {
				t.Fatal("error at first getting first uuencoded contents")
			}
			i++
		}
		if diff := pretty.Compare(string(gotx[0]), string(src1)); diff != "" {
			t.Errorf("Diff first source: %s", diff)
		}
		if diff := pretty.Compare(string(gotx[1]), string(src2)); diff != "" {
			t.Errorf("Diff second source: %s", diff)
		}
		wait.Done()
	}()
	got3, err := ioutil.ReadAll(transform.NewReader(bs, d))
	if err != nil {
		t.Fatal("err at encoding read all:", err)
	}
	if len(got3) != 0 {
		t.Error("Expecting empty byte from non-uuencoded bytes")
	}
	d.Close()
	wait.Wait()
}

const tDecBigLen = 5000

func TestMultiDecodeCancel(t *testing.T) {
	src := make([]byte, tDecBigLen)
	for i, _ := range src {
		src[i] = byte(i * 7)
	}
	r := transform.NewReader(bytes.NewBuffer(src), uuencode.Uue.NewEncoder())
	uucontent, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatal("err at encoding read all:", err)
	}
	b := bytes.NewReader(uucontent)
	d, cancel, ch := uuencode.NewMultiDecode()
	go func() {
		for r := range ch {
			var p []byte
			p = make([]byte, 4)
			r.Read(p)
			cancel()
		}
	}()
	_, err = ioutil.ReadAll(transform.NewReader(b, d))
	if err == nil {
		t.Error("Expecting error but got nil err")
	}
}

func TestMultiDecodeCancelEarly(t *testing.T) {
	src := make([]byte, tDecBigLen)
	for i, _ := range src {
		src[i] = byte(i * 7)
	}
	r := transform.NewReader(bytes.NewBuffer(src), uuencode.Uue.NewEncoder())
	uucontent, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatal("err at encoding read all:", err)
	}
	b := bytes.NewReader(uucontent)
	d, cancel, ch := uuencode.NewMultiDecode()
	go func() {
		cancel()
		for _ = range ch {
		}
	}()
	_, err = ioutil.ReadAll(transform.NewReader(b, d))
	if err == nil {
		t.Error("Expecting error but got nil err")
	}
}

const (
	tDecSmallLen = 100
	tCancelTry   = 50
)

func tstMultiDecodeCancelTry(t *testing.T) {
	src := make([]byte, tDecSmallLen)
	for i, _ := range src {
		src[i] = byte(i * 7)
	}
	r := transform.NewReader(bytes.NewBuffer(src), uuencode.Uue.NewEncoder())
	uucontent, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatal("err at encoding read all:", err)
	}
	b := bytes.NewReader(uucontent)
	d, cancel, ch := uuencode.NewMultiDecode()
	go func() {
		for _ = range ch {
		}
	}()
	go cancel()
	_, err = ioutil.ReadAll(transform.NewReader(b, d))
	if err == nil {
		t.Error("Expecting error but got nil err")
	}
}

func TestMultiDecodeCancelTry(t *testing.T) {
	for i := 0; i < tCancelTry; i++ {
		t.Run(fmt.Sprint(i), tstMultiDecodeCancelTry)
	}
}

func TestMultiDecodeReadClose(t *testing.T) {
	src := make([]byte, tDecBigLen)
	for i, _ := range src {
		src[i] = byte(i * 7)
	}
	r := transform.NewReader(bytes.NewBuffer(src), uuencode.Uue.NewEncoder())
	uucontent, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatal("err at encoding read all:", err)
	}
	b := bytes.NewReader(uucontent)
	d, _, ch := uuencode.NewMultiDecode()
	go func() {
		for r := range ch {
			var p []byte
			p = make([]byte, 4)
			r.Read(p)
			r.Close()
		}
	}()
	_, err = ioutil.ReadAll(transform.NewReader(b, d))
	if err != nil {
		t.Error("Expecting non-error but got err:", err)
	}
}

const (
	dummyBeginLine = "begin 666 filename.txt\n"
	dummyEndLine   = "\n`\nend\n"
)

func TestDecodeLongLine(t *testing.T) {
	src := make([]byte, 0, tDecBigLen)
	src = append(src, []byte(dummyBeginLine)...)
	for i := 0; i < tDecBigLen-len(dummyEndLine); i++ {
		src = append(src, 'a')
	}
	src = append(src, []byte(dummyEndLine)...)
	b := bytes.NewReader(src)
	d, _, ch := uuencode.NewMultiDecode()
	go func() {
		for r := range ch {
			var p []byte
			p = make([]byte, 4)
			r.Read(p)
			r.Close()
		}
	}()
	_, err := ioutil.ReadAll(transform.NewReader(b, d))
	if err == nil {
		t.Error("Expecting error but nil error")
	} else if err != uuencode.ErrBadLen {
		t.Error("Got: ", err, " Expecting: ", uuencode.ErrBadLen)
	}
}

const validUuencoded = "22!L;W9E('EO=2!F;W)E=F5R+@``"

var tDecodeWrong1stCharLen = len(dummyBeginLine) + len(dummyEndLine) + 50

func TestDecodeWrongUpperLimnitChar(t *testing.T) {
	src := make([]byte, 0, tDecodeWrong1stCharLen)
	src = append(src, []byte(dummyBeginLine)...)
	src = append(src, 'a')
	src = append(src, []byte(validUuencoded)...)
	src = append(src, '\n')
	src = append(src, []byte(dummyEndLine)...)
	b := bytes.NewReader(src)
	d, _, ch := uuencode.NewMultiDecode()
	go func() {
		for r := range ch {
			var p []byte
			p = make([]byte, tDecodeWrong1stCharLen)
			r.Read(p)
			r.Close()
		}
	}()
	_, err := ioutil.ReadAll(transform.NewReader(b, d))
	if err == nil {
		t.Error("Expecting error but nil error")
	} else if err != uuencode.ErrBadUUDec {
		t.Error("Got: ", err, " Expecting: ", uuencode.ErrBadUUDec)
	}
}

func TestDecodeWrongLowerLimitChar(t *testing.T) {
	src := make([]byte, 0, tDecodeWrong1stCharLen)
	src = append(src, []byte(dummyBeginLine)...)
	src = append(src, 0x1f)
	src = append(src, []byte(validUuencoded)...)
	src = append(src, '\n')
	src = append(src, []byte(dummyEndLine)...)
	b := bytes.NewReader(src)
	d, _, ch := uuencode.NewMultiDecode()
	go func() {
		for r := range ch {
			var p []byte
			p = make([]byte, tDecodeWrong1stCharLen)
			r.Read(p)
			r.Close()
		}
	}()
	_, err := ioutil.ReadAll(transform.NewReader(b, d))
	if err == nil {
		t.Error("Expecting error but nil error")
	} else if err != uuencode.ErrBadUUDec {
		t.Error("Got: ", err, " Expecting: ", uuencode.ErrBadUUDec)
	}
}

const testBeginText = "begin 123 file.log"

func TestDecodeVeryLongBegin(t *testing.T) {
	src := make([]byte, 0, tDecBigLen)
	src = append(src, []byte(testBeginText)...)
	tstlen := tDecBigLen - len(testBeginText) - len(dummyEndLine) -
		len(validUuencoded) + 1
	for i := 0; i < tstlen; i++ {
		src = append(src, 'a')
	}
	src = append(src, []byte(validUuencoded)...)
	src = append(src, '\n')
	src = append(src, []byte(dummyEndLine)...)
	b := bytes.NewReader(src)
	d, _, ch := uuencode.NewMultiDecode()
	go func() {
		for r := range ch {
			var p []byte
			p = make([]byte, tDecodeWrong1stCharLen)
			r.Read(p)
			r.Close()
		}
	}()
	_, err := ioutil.ReadAll(transform.NewReader(b, d))
	if err == nil {
		t.Error("Expecting error but nil error")
	} else if err != uuencode.ErrBadLen {
		t.Error("Got: ", err, " Expecting: ", uuencode.ErrBadLen)
	}
}

const testNoBeginTxt = "NO begin string"

func TestDecodeVeryLongWithoutBegin(t *testing.T) {
	src := make([]byte, 0, tDecBigLen)
	src = append(src, []byte(testNoBeginTxt)...)
	tstlen := tDecBigLen - len(testNoBeginTxt) - len(dummyEndLine) -
		len(validUuencoded) + 1
	for i := 0; i < tstlen; i++ {
		src = append(src, 'a')
	}
	src = append(src, []byte(validUuencoded)...)
	src = append(src, '\n')
	src = append(src, []byte(dummyEndLine)...)
	b := bytes.NewReader(src)
	d, _, ch := uuencode.NewMultiDecode()
	go func() {
		for r := range ch {
			var p []byte
			p = make([]byte, tDecodeWrong1stCharLen)
			r.Read(p)
			r.Close()
		}
	}()
	_, err := ioutil.ReadAll(transform.NewReader(b, d))
	if err == nil {
		t.Error("Expecting error but nil error")
	} else if err != uuencode.ErrBadUUDec {
		t.Error("Got: ", err, " Expecting: ", uuencode.ErrBadUUDec)
	}
}
