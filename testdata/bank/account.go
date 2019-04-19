package bank

import (
	"io"
)

type Account interface {
	Summarize(w io.Writer)
	Balance() int
}

type WithdrawableAccount interface {
	Account
	Withdraw(amount int) (int, error)
}

//go:generate stubber
