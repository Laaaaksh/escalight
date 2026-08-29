package db

import "github.com/google/uuid"

func (s *Store) CreateUser(email, name, passwordHash string, isAdmin bool) (*User, error) {
	u := &User{
		ID:           uuid.NewString(),
		Email:        email,
		Name:         name,
		PasswordHash: passwordHash,
		IsAdmin:      isAdmin,
		CreatedAt:    now(),
	}
	_, err := s.DB.Exec(
		`INSERT INTO users (id, email, name, password_hash, is_admin, slack_user_id, created_at) VALUES (?, ?, ?, ?, ?, '', ?)`,
		u.ID, u.Email, u.Name, u.PasswordHash, boolToInt(u.IsAdmin), u.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (s *Store) UserByEmail(email string) (*User, error) {
	row := s.DB.QueryRow(`SELECT id, email, name, password_hash, is_admin, slack_user_id, created_at FROM users WHERE email = ?`, email)
	return scanUser(row)
}

func (s *Store) UserByID(id string) (*User, error) {
	row := s.DB.QueryRow(`SELECT id, email, name, password_hash, is_admin, slack_user_id, created_at FROM users WHERE id = ?`, id)
	return scanUser(row)
}

func (s *Store) UsersByIDs(ids []string) (map[string]*User, error) {
	out := map[string]*User{}
	for _, id := range ids {
		if _, ok := out[id]; ok {
			continue
		}
		u, err := s.UserByID(id)
		if err != nil {
			return nil, err
		}
		out[id] = u
	}
	return out, nil
}

func (s *Store) ListUsers() ([]*User, error) {
	rows, err := s.DB.Query(`SELECT id, email, name, password_hash, is_admin, slack_user_id, created_at FROM users ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*User
	for rows.Next() {
		u, err := scanUserRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *Store) CountUsers() (int, error) {
	var n int
	err := s.DB.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

func (s *Store) UpdateUserSlackID(userID, slackUserID string) error {
	_, err := s.DB.Exec(`UPDATE users SET slack_user_id = ? WHERE id = ?`, slackUserID, userID)
	return err
}

func (s *Store) UserBySlackID(slackUserID string) (*User, error) {
	row := s.DB.QueryRow(`SELECT id, email, name, password_hash, is_admin, slack_user_id, created_at FROM users WHERE slack_user_id = ? AND slack_user_id != ''`, slackUserID)
	return scanUser(row)
}

type scanner interface {
	Scan(dest ...any) error
}

func scanUser(row scanner) (*User, error) {
	u := &User{}
	var isAdmin int
	if err := row.Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &isAdmin, &u.SlackUserID, &u.CreatedAt); err != nil {
		return nil, err
	}
	u.IsAdmin = isAdmin != 0
	return u, nil
}

func scanUserRows(rows scanner) (*User, error) {
	return scanUser(rows)
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
