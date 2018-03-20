package main_test

import (
	"bytes"
	"database/sql"
	"io/ioutil"
	"testing"

	main "github.com/dradtke/stubber"
	"github.com/dradtke/stubber/testdata/pkg"
)

func TestStubber(t *testing.T) {
	expected, err := ioutil.ReadFile("./testdata/pkg/pkg_stubs.go")
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	main.Main(nil, "./testdata/pkg", "", &buf)
	t.Log(buf.String())

	if !bytes.Equal(buf.Bytes(), expected) {
		t.Error("unexpected output (run with -v to see it)")
	}
}

func TestPkg(t *testing.T) {
	sm := pkg.StubbedSessionManager{
		GetUserIDStub: func(db *sql.DB, username string) (int64, error) {
			return 13, nil
		},
	}

	userID, err := sm.GetUserID(nil, "dradtke")
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	if userID != 13 {
		t.Errorf("unexpected user id: %d", userID)
	}

	calls := sm.GetUserIDCalls()
	if len(calls) != 1 {
		t.Errorf("unexpected number of calls: %d", len(calls))
	}

	call := calls[0]
	if call.Db != nil || call.Username != "dradtke" {
		t.Errorf("unexpected call params: %v", call)
	}
}
