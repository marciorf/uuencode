// Examples for uuencode package.

package uuencode_test

import (
	"bytes"
	"fmt"
	"io/ioutil"

	"github.com/cention-sany/uuencode"
	"golang.org/x/text/transform"
)

func ExampleDecode() {
	r := bytes.NewBufferString("begin 664 uutest1.txt\n($@`0$!&0````\n`\nend\n")
	tf := uuencode.Uue.NewDecoder()
	tr := transform.NewReader(r, tf)
	output, err := ioutil.ReadAll(tr)
	if err != nil {
		// encounter error
		panic(err)
	}
	fmt.Println("Bytes1:", output)
	r = bytes.NewBufferString("begin 664 uutest2.txt\n322!L;W9E('EO=2!F;W)E=F5R+@``\n`\nend\n")
	tf.Reset()
	tr = transform.NewReader(r, tf)
	output, err = ioutil.ReadAll(tr)
	if err != nil {
		// encounter error
		panic(err)
	}
	fmt.Println("Bytes2:", string(output))
	// Output:
	// Bytes1: [18 0 16 16 17 144 0 0]
	// Bytes2: I love you forever.
}
