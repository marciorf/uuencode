package uuutil

import (
	"errors"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	uu "github.com/sanylcs/uuencode"
	"golang.org/x/net/context"
	"golang.org/x/text/transform"
)

// Convert convert files into uuencoded bytes and write into w. useGrave true
// mean grave character is used for zero bit. eol is end of line characters.
func Convert(w io.Writer, useGrave bool, eol string, files ...string) error {
	if len(files) <= 0 {
		return errors.New("nothing to convert")
	}
	e := uu.NewEncode(useGrave, eol)
	// loop through all the input files
	for _, f := range files {
		rc, err := os.Open(f)
		if err != nil {
			return err
		}
		fi, err := rc.Stat()
		if err != nil {
			return err
		}
		// format int to string file permission should be in base-8.
		permit := strconv.FormatUint(uint64(fi.Mode().Perm()), 8)
		e.ResetAll(permit, fi.Name())
		// write the converted result into w which is provided by caller.
		_, err = io.Copy(w, transform.NewReader(rc, e))
		if err != nil {
			return err
		}
		// close and release the file contents.
		if err = rc.Close(); err != nil {
			return err
		}
	}
	return nil
}

// getDir only make the directory once by using sync.Once.
func getDir(once *sync.Once, dir string) (string, error) {
	var err error
	once.Do(func() {
		// every parsing process only write to one directory and should only
		// need to run directory creation once.
		_, err = os.Stat(dir)
		if err != nil && os.IsNotExist(err) {
			err = os.MkdirAll(dir, 0644)
		}
	})
	return dir, err
}

// Parse decode uuencoded data from r into directory path dir and write any non
// uuencode bytes into w. Parse block decoding finish or error.
func Parse(ctx context.Context, w io.Writer, dir string, r io.Reader) error {
	var wait sync.WaitGroup
	if w == nil {
		w = ioutil.Discard
	}
	wait.Add(2)
	d, cancel, ch := uu.NewMultiDecode()
	// run reading of decoded result in goroutine
	go func() {
		var (
			once sync.Once
			err  error
		)
		defer wait.Done()
		// get the io.Reader from chan
		for r := range ch {
			dir, err = getDir(&once, dir)
			if err != nil {
				r.Close()
				continue
			}
			// create the filenames either base on the input file's begin header
			// or create random file is filename can not be found on the begin
			// header.
			var f *os.File
			if d.Filename != "" {
				name := filepath.Join(dir, d.Filename)
				// create or overwrite the content of existing file.
				f, err = os.OpenFile(name,
					os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
			} else {
				// create a random file inside the provided directory.
				f, err = ioutil.TempFile(dir, "uu_")
			}
			if err != nil {
				r.Close()
				continue
			}
			// copy out the content of decoded contents into file.
			_, err = io.Copy(f, r)
			if err != nil {
				r.Close()
				f.Close()
				continue
			}
			f.Close()
		}
	}()
	// decoding process run in goroutine as to allow cancelable action on
	// transform method.
	var err1 error
	go func() {
		_, err1 = io.Copy(w, transform.NewReader(r, d))
		d.Close()
		wait.Done()
	}()
	done := make(chan int)
	// wait both reading goroutine and processing goroutine to end here in
	// another goroutine.
	go func() {
		wait.Wait()
		close(done)
	}()
	// check either the process end sanely or it was ended by context.
	var err2 error
	select {
	case <-ctx.Done():
		cancel()
		err2 = ctx.Err()
	case <-done:
	}
	// done signaling here both reading goroutine and process goroutine ended.
	<-done
	if err1 == nil {
		err1 = err2
	}
	return err1
}
