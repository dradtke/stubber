// This file was generated by stubber; DO NOT EDIT

// +build !nostubs

package bank

import (
	"io"
)

// StubbedAccount is a stubbed implementation of Account.
type StubbedAccount struct {
	// BalanceStub defines the implementation for Balance.
	BalanceStub  func() int
	balanceCalls []struct{}
	// SummarizeStub defines the implementation for Summarize.
	SummarizeStub  func(w io.Writer)
	summarizeCalls []struct{ W io.Writer }
}

// Balance delegates its behavior to the field BalanceStub.
func (s *StubbedAccount) Balance() int {
	if s.BalanceStub == nil {
		panic("StubbedAccount.Balance: nil method stub")
	}
	s.balanceCalls = append(s.balanceCalls, struct{}{})
	return (s.BalanceStub)()
}

// BalanceCalls returns a slice of calls made to Balance. Each element
// of the slice represents the parameters that were provided.
func (s *StubbedAccount) BalanceCalls() []struct{} {
	return s.balanceCalls
}

// Summarize delegates its behavior to the field SummarizeStub.
func (s *StubbedAccount) Summarize(w io.Writer) {
	if s.SummarizeStub == nil {
		panic("StubbedAccount.Summarize: nil method stub")
	}
	s.summarizeCalls = append(s.summarizeCalls, struct{ W io.Writer }{W: w})
	(s.SummarizeStub)(w)
}

// SummarizeCalls returns a slice of calls made to Summarize. Each element
// of the slice represents the parameters that were provided.
func (s *StubbedAccount) SummarizeCalls() []struct{ W io.Writer } {
	return s.summarizeCalls
}

// Compile-time check that the implementation matches the interface.
var _ Account = (*StubbedAccount)(nil)

// StubbedWithdrawableAccount is a stubbed implementation of WithdrawableAccount.
type StubbedWithdrawableAccount struct {
	// BalanceStub defines the implementation for Balance.
	BalanceStub  func() int
	balanceCalls []struct{}
	// SummarizeStub defines the implementation for Summarize.
	SummarizeStub  func(w io.Writer)
	summarizeCalls []struct{ W io.Writer }
	// WithdrawStub defines the implementation for Withdraw.
	WithdrawStub  func(amount int) (int, error)
	withdrawCalls []struct{ Amount int }
}

// Balance delegates its behavior to the field BalanceStub.
func (s *StubbedWithdrawableAccount) Balance() int {
	if s.BalanceStub == nil {
		panic("StubbedWithdrawableAccount.Balance: nil method stub")
	}
	s.balanceCalls = append(s.balanceCalls, struct{}{})
	return (s.BalanceStub)()
}

// BalanceCalls returns a slice of calls made to Balance. Each element
// of the slice represents the parameters that were provided.
func (s *StubbedWithdrawableAccount) BalanceCalls() []struct{} {
	return s.balanceCalls
}

// Summarize delegates its behavior to the field SummarizeStub.
func (s *StubbedWithdrawableAccount) Summarize(w io.Writer) {
	if s.SummarizeStub == nil {
		panic("StubbedWithdrawableAccount.Summarize: nil method stub")
	}
	s.summarizeCalls = append(s.summarizeCalls, struct{ W io.Writer }{W: w})
	(s.SummarizeStub)(w)
}

// SummarizeCalls returns a slice of calls made to Summarize. Each element
// of the slice represents the parameters that were provided.
func (s *StubbedWithdrawableAccount) SummarizeCalls() []struct{ W io.Writer } {
	return s.summarizeCalls
}

// Withdraw delegates its behavior to the field WithdrawStub.
func (s *StubbedWithdrawableAccount) Withdraw(amount int) (int, error) {
	if s.WithdrawStub == nil {
		panic("StubbedWithdrawableAccount.Withdraw: nil method stub")
	}
	s.withdrawCalls = append(s.withdrawCalls, struct{ Amount int }{Amount: amount})
	return (s.WithdrawStub)(amount)
}

// WithdrawCalls returns a slice of calls made to Withdraw. Each element
// of the slice represents the parameters that were provided.
func (s *StubbedWithdrawableAccount) WithdrawCalls() []struct{ Amount int } {
	return s.withdrawCalls
}

// Compile-time check that the implementation matches the interface.
var _ WithdrawableAccount = (*StubbedWithdrawableAccount)(nil)
