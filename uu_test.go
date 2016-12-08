package uuencode_test

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/cention-sany/uuencode"
	"github.com/kylelemons/godebug/pretty"
	"golang.org/x/text/transform"
)

// readTestFile is a test utility function to io.Reader of test file. All test
// files must in the format of name.in for input and name.out for output compare.
func readTestFile(path, filename string) io.ReadCloser {
	raw, err := os.Open(filepath.Join("test-data", path, filename))
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

// remove any privTestx as it only work for developer PC.
var testfiles1 = []string{
	"test1", "test2",
	//"privTest1",
}

const tDecFirstOne = "testDecFirstOne"

func TestDecFirstOne(t *testing.T) {
	for _, f := range testfiles1 {
		rc := readInputFile(tDecFirstOne, f)
		defer rc.Close()
		u := uuencode.NewDecFirstOne()
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

const tErrDFO = "testErrDFO"

var testfiles2 = []string{
	"test1", "test2", "test3", "test4",
}

func TestErrDFO(t *testing.T) {
	for _, f := range testfiles2 {
		rc := readTestFile(tErrDFO, f+".err")
		defer rc.Close()
		u := uuencode.NewDecFirstOne()
		r := transform.NewReader(rc, u)
		_, err := ioutil.ReadAll(r)
		if err == nil {
			t.Errorf("Test TestErrDFO file=%s failed as expected err return no error", f)
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
