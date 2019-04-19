package main_test

import (
	"bytes"
	"io/ioutil"
	"testing"

	main "github.com/dradtke/stubber"
)

func TestStubber(t *testing.T) {
	expected, err := ioutil.ReadFile("./testdata/bank/bank_stubs.go")
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	main.Main(nil, []string{"./testdata/bank"}, "", &buf)
	t.Log(buf.String())

	if !bytes.Equal(buf.Bytes(), expected) {
		t.Error("unexpected output (run with -v to see it)")
	}
}
