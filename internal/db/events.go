package db

import "github.com/google/uuid"

func (s *Store) AddEvent(incidentID, eventType, actor, detail string) error {
	_, err := s.DB.Exec(
		`INSERT INTO incident_events (id, incident_id, event_type, actor, detail, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		uuid.NewString(), incidentID, eventType, actor, detail, now(),
	)
	return err
}

func (s *Store) EventsForIncident(incidentID string) ([]*IncidentEvent, error) {
	rows, err := s.DB.Query(
		`SELECT id, incident_id, event_type, actor, detail, created_at FROM incident_events WHERE incident_id = ? ORDER BY created_at`,
		incidentID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*IncidentEvent
	for rows.Next() {
		e := &IncidentEvent{}
		if err := rows.Scan(&e.ID, &e.IncidentID, &e.EventType, &e.Actor, &e.Detail, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
