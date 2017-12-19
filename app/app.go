package app

import "net/http"

type Frobnicator interface {
	Frobnicate(r *http.Request) ([]byte, error)
}
