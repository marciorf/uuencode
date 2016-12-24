package uuutil_test

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/kylelemons/godebug/pretty"
	"github.com/sanylcs/uuencode/uuutil"
	"golang.org/x/net/context"
)

const (
	tstFolder = "test-data"
	tConvert  = "testConvert"
)

// readTestFile is a test utility function to io.Reader of test file. All test
// files must in the format of name.in for input and name.out for output
// compare.
func readTestFile(path, filename string) io.ReadCloser {
	raw, err := os.Open(filepath.Join(tstFolder, path, filename))
	if err != nil {
		panic(fmt.Sprintf("Failed to open test file: %v", err))
	}
	return raw
}

func readInputFile(p, f string) io.ReadCloser {
	return readTestFile(p, f+".in")
}

func readOutputFile(p, f string) io.ReadCloser {
	return readTestFile(p, f+".out")
}

var testConvertFiles = []string{
	"test1",
}

func TestConvert(t *testing.T) {
	for _, f := range testConvertFiles {
		p := filepath.Join(tstFolder, tConvert, f)
		files := make([]string, 0, 8)
		p = fmt.Sprint(p, "_%d.in")
		for i := 1; ; i++ {
			name := fmt.Sprintf(p, i)
			_, err := os.Stat(name)
			if err != nil {
				break
			}
			err = os.Chmod(name, 0666)
			if err != nil {
				t.Fatal("Chmod must success for cross platform test", err)
			}
			files = append(files, name)
		}
		b := new(bytes.Buffer)
		err := uuutil.Convert(b, true, "\r\n", files...)
		if err != nil {
			t.Fatalf("Err=%v", err)
		}
		rgex := regexp.MustCompile("^begin [0-9]+ (.*)\r\n")
		newb := rgex.ReplaceAll(b.Bytes(), []byte("begin 666 $1\r\n"))
		rc := readOutputFile(tConvert, f)
		defer rc.Close()
		expect, err := ioutil.ReadAll(rc)
		if err != nil {
			t.Fatalf("Test TestConvert out file=%s failed with err=%v",
				f, err)
		}
		if diff := pretty.Compare(string(newb), string(expect)); diff != "" {
			t.Errorf("Diff: %s", diff)
		}
	}
}

func TestConvertFail1(t *testing.T) {
	err := uuutil.Convert(nil, true, "\n", []string{"unknown file"}...)
	if err == nil {
		t.Error("Expected error but return nil")
	}
}

func TestConvertFail2(t *testing.T) {
	b := new(bytes.Buffer)
	err := uuutil.Convert(b, true, "\n")
	if err == nil {
		t.Fatal("expected error but return nil")
	}
}

func TestConvertFail3(t *testing.T) {
	readOnlyFile := filepath.Join(tstFolder, tConvert,
		fmt.Sprint(testConvertFiles[0], "_1.in"))
	w, err := os.Open(readOnlyFile)
	err = uuutil.Convert(w, true, "\n",
		[]string{readOnlyFile}...)
	if err == nil {
		t.Error("Expected error but return nil")
	}
}

const tstParse = "testParse"

var (
	dirTemp        = filepath.Join(tstFolder, tstParse, "temp")
	testParseFiles = []string{
		"test1",
	}
)

func TestParse(t *testing.T) {
	defer os.RemoveAll(dirTemp)
	for _, f := range testParseFiles {
		rc := readInputFile(tstParse, f)
		err := uuutil.Parse(context.TODO(), nil, dirTemp, rc)
		if err != nil {
			t.Error("Expected nil-error but got:", err)
		}
		rc.Close()
	}
}

var (
	unknownDir     = filepath.Join(tstFolder, tstParse, "unknown")
	testParseNoDir = []string{
		"test1", "nofilename1",
	}
)

func TestParseUnknownDir(t *testing.T) {
	for i, f := range testParseNoDir {
		rc := readInputFile(tstParse, f)
		defer rc.Close()
		dir := fmt.Sprint(unknownDir, i)
		func() {
			defer os.RemoveAll(dir)
			err := uuutil.Parse(context.TODO(), nil, dir, rc)
			if err != nil {
				t.Error("Expected nil-error but got:", err)
			}
		}()
	}
}

func TestParseCancel(t *testing.T) {
	defer os.RemoveAll(dirTemp)
	rc := readInputFile(tstParse, testParseFiles[0])
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := uuutil.Parse(ctx, nil, dirTemp, rc)
	if err == nil {
		t.Error("Expected error but no error")
	}
}
