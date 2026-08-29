package db

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/google/uuid"
)

func newWebhookKey() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (s *Store) CreateService(name, policyID string) (*Service, error) {
	key, err := newWebhookKey()
	if err != nil {
		return nil, err
	}
	svc := &Service{ID: uuid.NewString(), Name: name, EscalationPolicyID: policyID, WebhookKey: key, CreatedAt: now()}
	_, err = s.DB.Exec(
		`INSERT INTO services (id, name, escalation_policy_id, webhook_key, created_at) VALUES (?, ?, ?, ?, ?)`,
		svc.ID, svc.Name, svc.EscalationPolicyID, svc.WebhookKey, svc.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return svc, nil
}

func (s *Store) DeleteService(id string) error {
	_, err := s.DB.Exec(`DELETE FROM services WHERE id = ?`, id)
	return err
}

func (s *Store) ListServices() ([]*Service, error) {
	rows, err := s.DB.Query(`SELECT id, name, escalation_policy_id, webhook_key, created_at FROM services ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*Service
	for rows.Next() {
		svc := &Service{}
		if err := rows.Scan(&svc.ID, &svc.Name, &svc.EscalationPolicyID, &svc.WebhookKey, &svc.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, svc)
	}
	return out, rows.Err()
}

func (s *Store) ServiceByID(id string) (*Service, error) {
	svc := &Service{}
	err := s.DB.QueryRow(`SELECT id, name, escalation_policy_id, webhook_key, created_at FROM services WHERE id = ?`, id).
		Scan(&svc.ID, &svc.Name, &svc.EscalationPolicyID, &svc.WebhookKey, &svc.CreatedAt)
	if err != nil {
		return nil, err
	}
	return svc, nil
}

func (s *Store) ServiceByWebhookKey(key string) (*Service, error) {
	svc := &Service{}
	err := s.DB.QueryRow(`SELECT id, name, escalation_policy_id, webhook_key, created_at FROM services WHERE webhook_key = ?`, key).
		Scan(&svc.ID, &svc.Name, &svc.EscalationPolicyID, &svc.WebhookKey, &svc.CreatedAt)
	if err != nil {
		return nil, err
	}
	return svc, nil
}
