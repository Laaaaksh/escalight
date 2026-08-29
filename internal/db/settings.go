package db

func (s *Store) GetSetting(key string) (string, bool, error) {
	var v string
	err := s.DB.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&v)
	if err == ErrNotFound {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return v, true, nil
}

func (s *Store) SetSetting(key, value string) error {
	_, err := s.DB.Exec(
		`INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, value,
	)
	return err
}
