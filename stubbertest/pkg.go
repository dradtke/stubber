package stubbertest

import "database/sql"

type SessionManager interface {
	GetUserID(db *sql.DB, username string) (int64, error)
	Deactivate(db *sql.DB, userIds ...int64)
}

//go:generate stubber
