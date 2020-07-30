package bank

import (
	"errors"
	"io"
)

//go:generate stubber

var (
	ErrBalanceExceeded = errors.New("balance exceeded")
)

type Account interface {
	Summarize(w io.Writer)
	Balance() int
}

type WithdrawableAccount interface {
	Account
	Withdraw(amount int) (int, error)
}
