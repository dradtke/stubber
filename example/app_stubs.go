// This file was generated by stubber; DO NOT EDIT

package app

type StubbedSessionManager struct {
	CurrentUserStub func() (int64, error)
	UserNameStub    func(id int64) (string, error)
}

func (s *StubbedSessionManager) CurrentUser() (int64, error) {
	if s.CurrentUserStub == nil {
		panic("StubbedSessionManager.CurrentUser: nil method stub")
	}
	return (s.CurrentUserStub)()
}

func (s *StubbedSessionManager) UserName(id int64) (string, error) {
	if s.UserNameStub == nil {
		panic("StubbedSessionManager.UserName: nil method stub")
	}
	return (s.UserNameStub)(id)
}

// Compile-time check that the implementation matches the interface.
var _ SessionManager = (*StubbedSessionManager)(nil)