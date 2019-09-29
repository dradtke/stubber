package main_test

import (
	"bytes"
	"flag"
	"io/ioutil"
	"os"
	"os/exec"
	"testing"

	"github.com/google/go-cmp/cmp"

	main "github.com/dradtke/stubber"
)

var update bool

func init() {
	flag.BoolVar(&update, "update", false, "update the golden file")
	testing.Init()
	flag.Parse()
}

func TestStubber(t *testing.T) {
	if update {
		main.Main(nil, []string{"./testdata/bank"}, "./testdata/stubs", nil)
		if v, err := exec.Command("go", "build", "-o", os.DevNull, "./testdata/stubs").CombinedOutput(); err != nil {
			t.Errorf("new golden file failed to build:\n%s", string(v))
		}
		return
	}

	var buf bytes.Buffer
	main.Main(nil, []string{"./testdata/bank"}, "", &buf)

	expected, err := ioutil.ReadFile("./testdata/stubs/bank_stubs.go")
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(string(expected), buf.String()); diff != "" {
		t.Errorf("output mismatch (-want +got):\n%s", diff)
	}
}
