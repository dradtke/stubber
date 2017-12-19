package app

// This is the simplest invocation of stubber. By default, it will scan the
// package in the current directory for all interface definitions and write
// them out to <file>_stubs.go. You can use the -types argument to specify a
// comma-separated list of interface types to limit what gets generated, and a
// single positional argument is accepted, which specifies the directory to
// scan. This means that "stubber -types=SessionManager ." is equivalent to the
// version below.

//go:generate stubber

type SessionManager interface {
	CurrentUser() (int64, error)
	UserName(id int64) (string, error)
}
