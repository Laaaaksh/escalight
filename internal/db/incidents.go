package db

import (
	"database/sql"

	"github.com/google/uuid"
)

type CreateIncidentParams struct {
	ServiceID   string
	Title       string
	Description string
	Source      string
	Fingerprint string
}

func (s *Store) CreateIncident(p CreateIncidentParams) (*Incident, error) {
	inc := &Incident{
		ID:          uuid.NewString(),
		ServiceID:   p.ServiceID,
		Title:       p.Title,
		Description: p.Description,
		Source:      p.Source,
		Fingerprint: p.Fingerprint,
		Status:      "triggered",
		CreatedAt:   now(),
	}
	_, err := s.DB.Exec(
		`INSERT INTO incidents (id, service_id, title, description, source, fingerprint, status, current_step, repeat_count, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, 'triggered', 0, 0, ?)`,
		inc.ID, inc.ServiceID, inc.Title, inc.Description, inc.Source, inc.Fingerprint, inc.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return inc, nil
}

// OpenIncidentByFingerprint finds a non-resolved incident for the same
// service+fingerprint, so a re-firing alert (e.g. Alertmanager re-sending the
// same active alert) updates the existing incident instead of paging again.
func (s *Store) OpenIncidentByFingerprint(serviceID, fingerprint string) (*Incident, error) {
	if fingerprint == "" {
		return nil, ErrNotFound
	}
	row := s.DB.QueryRow(
		`SELECT `+incidentCols+` FROM incidents WHERE service_id = ? AND fingerprint = ? AND status != 'resolved' ORDER BY created_at DESC LIMIT 1`,
		serviceID, fingerprint,
	)
	return scanIncident(row)
}

func (s *Store) IncidentByID(id string) (*Incident, error) {
	row := s.DB.QueryRow(`SELECT `+incidentCols+` FROM incidents WHERE id = ?`, id)
	return scanIncident(row)
}

// IncidentByIDPrefix finds a non-resolved incident whose ID starts with
// prefix, for Slack slash commands where typing a full UUID isn't practical.
// Returns ErrNotFound if none match, or if the prefix is ambiguous.
func (s *Store) IncidentByIDPrefix(prefix string) (*Incident, error) {
	rows, err := s.DB.Query(`SELECT `+incidentCols+` FROM incidents WHERE id LIKE ? AND status != 'resolved' ORDER BY created_at DESC LIMIT 2`, prefix+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var matches []*Incident
	for rows.Next() {
		inc, err := scanIncident(rows)
		if err != nil {
			return nil, err
		}
		matches = append(matches, inc)
	}
	if len(matches) != 1 {
		return nil, ErrNotFound
	}
	return matches[0], nil
}

func (s *Store) ListIncidents(status string, limit int) ([]*Incident, error) {
	var rows *sql.Rows
	var err error
	if status == "" || status == "all" {
		rows, err = s.DB.Query(`SELECT `+incidentCols+` FROM incidents ORDER BY created_at DESC LIMIT ?`, limit)
	} else {
		rows, err = s.DB.Query(`SELECT `+incidentCols+` FROM incidents WHERE status = ? ORDER BY created_at DESC LIMIT ?`, status, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*Incident
	for rows.Next() {
		inc, err := scanIncident(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, inc)
	}
	return out, rows.Err()
}

// DueForEscalation returns triggered incidents whose next escalation time has passed.
func (s *Store) DueForEscalation(asOf string) ([]*Incident, error) {
	rows, err := s.DB.Query(
		`SELECT `+incidentCols+` FROM incidents WHERE status = 'triggered' AND next_escalation_at IS NOT NULL AND next_escalation_at <= ?`,
		asOf,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*Incident
	for rows.Next() {
		inc, err := scanIncident(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, inc)
	}
	return out, rows.Err()
}

func (s *Store) SetNextEscalation(incidentID string, step, repeatCount int, nextAt sql.NullString) error {
	_, err := s.DB.Exec(
		`UPDATE incidents SET current_step = ?, repeat_count = ?, next_escalation_at = ? WHERE id = ?`,
		step, repeatCount, nextAt, incidentID,
	)
	return err
}

// AcknowledgeIncident marks the incident acknowledged. userID may be empty to
// record a system actor (e.g. an automated ack) as NULL rather than
// violating the acknowledged_by foreign key with an empty string.
func (s *Store) AcknowledgeIncident(incidentID, userID string) error {
	_, err := s.DB.Exec(
		`UPDATE incidents SET status = 'acknowledged', acknowledged_by = ?, acknowledged_at = ?, next_escalation_at = NULL WHERE id = ? AND status = 'triggered'`,
		nullIfEmpty(userID), now(), incidentID,
	)
	return err
}

// ResolveIncident marks the incident resolved. userID may be empty for a
// system-initiated resolve (e.g. the alert source itself reports resolved).
func (s *Store) ResolveIncident(incidentID, userID string) error {
	_, err := s.DB.Exec(
		`UPDATE incidents SET status = 'resolved', resolved_by = ?, resolved_at = ?, next_escalation_at = NULL WHERE id = ? AND status != 'resolved'`,
		nullIfEmpty(userID), now(), incidentID,
	)
	return err
}

func nullIfEmpty(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

const incidentCols = `id, service_id, title, description, source, fingerprint, status, current_step, repeat_count, next_escalation_at, acknowledged_by, acknowledged_at, resolved_by, resolved_at, created_at`

func scanIncident(row scanner) (*Incident, error) {
	inc := &Incident{}
	err := row.Scan(
		&inc.ID, &inc.ServiceID, &inc.Title, &inc.Description, &inc.Source, &inc.Fingerprint, &inc.Status,
		&inc.CurrentStep, &inc.RepeatCount, &inc.NextEscalationAt, &inc.AcknowledgedBy, &inc.AcknowledgedAt,
		&inc.ResolvedBy, &inc.ResolvedAt, &inc.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return inc, nil
}
