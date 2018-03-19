package pkg

import "database/sql"

type SessionManager interface {
	GetUserID(db *sql.DB, username string) (int64, error)
}
