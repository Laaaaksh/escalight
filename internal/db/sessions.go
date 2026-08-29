package db

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

const sessionTTL = 30 * 24 * time.Hour

func NewSessionToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (s *Store) CreateSession(userID string) (token string, expiresAt time.Time, err error) {
	token, err = NewSessionToken()
	if err != nil {
		return "", time.Time{}, err
	}
	expiresAt = time.Now().UTC().Add(sessionTTL)
	_, err = s.DB.Exec(
		`INSERT INTO sessions (token, user_id, created_at, expires_at) VALUES (?, ?, ?, ?)`,
		token, userID, now(), expiresAt.Format(time.RFC3339),
	)
	return token, expiresAt, err
}

// SessionUser resolves a session token to its user, returning ErrNotFound if
// the token is missing, unknown, or expired.
func (s *Store) SessionUser(token string) (*User, error) {
	var userID, expiresAt string
	err := s.DB.QueryRow(`SELECT user_id, expires_at FROM sessions WHERE token = ?`, token).Scan(&userID, &expiresAt)
	if err != nil {
		return nil, err
	}
	exp, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil || time.Now().UTC().After(exp) {
		_ = s.DeleteSession(token) // best-effort cleanup; the caller only needs ErrNotFound either way
		return nil, ErrNotFound
	}
	return s.UserByID(userID)
}

func (s *Store) DeleteSession(token string) error {
	_, err := s.DB.Exec(`DELETE FROM sessions WHERE token = ?`, token)
	return err
}
