package db

import (
	"database/sql"
	"time"
)

// Store wraps the database connection with Escalight's queries.
type Store struct {
	DB *sql.DB
}

func NewStore(conn *sql.DB) *Store {
	return &Store{DB: conn}
}

func now() string {
	return time.Now().UTC().Format(time.RFC3339)
}
