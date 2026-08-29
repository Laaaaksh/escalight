package db

import "github.com/google/uuid"

func (s *Store) SavePushSubscription(userID, endpoint, p256dh, auth string) error {
	_, err := s.DB.Exec(
		`INSERT INTO push_subscriptions (id, user_id, endpoint, p256dh, auth, created_at) VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(user_id, endpoint) DO UPDATE SET p256dh = excluded.p256dh, auth = excluded.auth`,
		uuid.NewString(), userID, endpoint, p256dh, auth, now(),
	)
	return err
}

func (s *Store) DeletePushSubscription(userID, endpoint string) error {
	_, err := s.DB.Exec(`DELETE FROM push_subscriptions WHERE user_id = ? AND endpoint = ?`, userID, endpoint)
	return err
}

func (s *Store) PushSubscriptionsForUser(userID string) ([]*PushSubscription, error) {
	rows, err := s.DB.Query(`SELECT id, user_id, endpoint, p256dh, auth, created_at FROM push_subscriptions WHERE user_id = ?`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*PushSubscription
	for rows.Next() {
		p := &PushSubscription{}
		if err := rows.Scan(&p.ID, &p.UserID, &p.Endpoint, &p.P256dh, &p.Auth, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
