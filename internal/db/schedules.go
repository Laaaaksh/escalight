package db

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

func (s *Store) CreateSchedule(name, timezone string) (*Schedule, error) {
	sch := &Schedule{ID: uuid.NewString(), Name: name, Timezone: timezone, CreatedAt: now()}
	_, err := s.DB.Exec(`INSERT INTO schedules (id, name, timezone, created_at) VALUES (?, ?, ?, ?)`, sch.ID, sch.Name, sch.Timezone, sch.CreatedAt)
	if err != nil {
		return nil, err
	}
	return sch, nil
}

func (s *Store) DeleteSchedule(id string) error {
	_, err := s.DB.Exec(`DELETE FROM schedules WHERE id = ?`, id)
	return err
}

func (s *Store) ListSchedules() ([]*Schedule, error) {
	rows, err := s.DB.Query(`SELECT id, name, timezone, created_at FROM schedules ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*Schedule
	for rows.Next() {
		sch := &Schedule{}
		if err := rows.Scan(&sch.ID, &sch.Name, &sch.Timezone, &sch.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, sch)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, sch := range out {
		rot, err := s.rotationForSchedule(sch.ID)
		if err != nil {
			return nil, err
		}
		sch.Rotation = rot
	}
	return out, nil
}

func (s *Store) ScheduleByID(id string) (*Schedule, error) {
	sch := &Schedule{}
	err := s.DB.QueryRow(`SELECT id, name, timezone, created_at FROM schedules WHERE id = ?`, id).
		Scan(&sch.ID, &sch.Name, &sch.Timezone, &sch.CreatedAt)
	if err != nil {
		return nil, err
	}
	rot, err := s.rotationForSchedule(id)
	if err != nil {
		return nil, err
	}
	sch.Rotation = rot
	return sch, nil
}

func (s *Store) rotationForSchedule(scheduleID string) (*ScheduleRotation, error) {
	rot := &ScheduleRotation{}
	var userOrderJSON string
	err := s.DB.QueryRow(
		`SELECT id, schedule_id, rotation_type, handoff_time, start_at, user_order FROM schedule_rotations WHERE schedule_id = ?`,
		scheduleID,
	).Scan(&rot.ID, &rot.ScheduleID, &rot.RotationType, &rot.HandoffTime, &rot.StartAt, &userOrderJSON)
	if err == ErrNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(userOrderJSON), &rot.UserOrder); err != nil {
		return nil, fmt.Errorf("decode user_order: %w", err)
	}
	return rot, nil
}

// SetRotation creates or replaces a schedule's single rotation definition.
func (s *Store) SetRotation(scheduleID, rotationType, handoffTime, startAt string, userOrder []string) error {
	orderJSON, err := json.Marshal(userOrder)
	if err != nil {
		return err
	}
	_, err = s.DB.Exec(
		`INSERT INTO schedule_rotations (id, schedule_id, rotation_type, handoff_time, start_at, user_order)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(schedule_id) DO UPDATE SET
		   rotation_type = excluded.rotation_type,
		   handoff_time = excluded.handoff_time,
		   start_at = excluded.start_at,
		   user_order = excluded.user_order`,
		uuid.NewString(), scheduleID, rotationType, handoffTime, startAt, string(orderJSON),
	)
	return err
}

func (s *Store) AddOverride(scheduleID, userID, startAt, endAt string) (*ScheduleOverride, error) {
	o := &ScheduleOverride{ID: uuid.NewString(), ScheduleID: scheduleID, UserID: userID, StartAt: startAt, EndAt: endAt, CreatedAt: now()}
	_, err := s.DB.Exec(
		`INSERT INTO schedule_overrides (id, schedule_id, user_id, start_at, end_at, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		o.ID, o.ScheduleID, o.UserID, o.StartAt, o.EndAt, o.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return o, nil
}

func (s *Store) DeleteOverride(id string) error {
	_, err := s.DB.Exec(`DELETE FROM schedule_overrides WHERE id = ?`, id)
	return err
}

// OverridesInRange returns overrides for a schedule that intersect [from, to), ordered by start.
func (s *Store) OverridesInRange(scheduleID, from, to string) ([]*ScheduleOverride, error) {
	rows, err := s.DB.Query(
		`SELECT id, schedule_id, user_id, start_at, end_at, created_at FROM schedule_overrides
		 WHERE schedule_id = ? AND start_at < ? AND end_at > ? ORDER BY start_at`,
		scheduleID, to, from,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*ScheduleOverride
	for rows.Next() {
		o := &ScheduleOverride{}
		if err := rows.Scan(&o.ID, &o.ScheduleID, &o.UserID, &o.StartAt, &o.EndAt, &o.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}
