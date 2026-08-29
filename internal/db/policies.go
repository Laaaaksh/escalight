package db

import (
	"fmt"

	"github.com/google/uuid"
)

func (s *Store) CreatePolicy(name, description string, repeat int) (*EscalationPolicy, error) {
	p := &EscalationPolicy{ID: uuid.NewString(), Name: name, Description: description, Repeat: repeat, CreatedAt: now()}
	_, err := s.DB.Exec(
		`INSERT INTO escalation_policies (id, name, description, repeat, created_at) VALUES (?, ?, ?, ?, ?)`,
		p.ID, p.Name, p.Description, p.Repeat, p.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (s *Store) ListPolicies() ([]*EscalationPolicy, error) {
	rows, err := s.DB.Query(`SELECT id, name, description, repeat, created_at FROM escalation_policies ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*EscalationPolicy
	for rows.Next() {
		p := &EscalationPolicy{}
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.Repeat, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) PolicyByID(id string) (*EscalationPolicy, error) {
	p := &EscalationPolicy{}
	err := s.DB.QueryRow(`SELECT id, name, description, repeat, created_at FROM escalation_policies WHERE id = ?`, id).
		Scan(&p.ID, &p.Name, &p.Description, &p.Repeat, &p.CreatedAt)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (s *Store) DeletePolicy(id string) error {
	_, err := s.DB.Exec(`DELETE FROM escalation_policies WHERE id = ?`, id)
	return err
}

// ReplaceSteps atomically replaces every step (and target) of a policy. Steps
// are supplied in the order they should fire.
func (s *Store) ReplaceSteps(policyID string, steps []EscalationStep) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM escalation_steps WHERE policy_id = ?`, policyID); err != nil {
		return err
	}

	for i, step := range steps {
		stepID := uuid.NewString()
		if _, err := tx.Exec(
			`INSERT INTO escalation_steps (id, policy_id, step_order, wait_minutes) VALUES (?, ?, ?, ?)`,
			stepID, policyID, i, step.WaitMinutes,
		); err != nil {
			return fmt.Errorf("insert step %d: %w", i, err)
		}
		for _, target := range step.Targets {
			if _, err := tx.Exec(
				`INSERT INTO escalation_step_targets (id, step_id, target_type, target_id, via_email, via_push, via_slack, via_discord)
				 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
				uuid.NewString(), stepID, target.TargetType, target.TargetID,
				boolToInt(target.ViaEmail), boolToInt(target.ViaPush), boolToInt(target.ViaSlack), boolToInt(target.ViaDiscord),
			); err != nil {
				return fmt.Errorf("insert target: %w", err)
			}
		}
	}
	return tx.Commit()
}

// StepsForPolicy returns a policy's steps in fire order, each with its targets loaded.
func (s *Store) StepsForPolicy(policyID string) ([]EscalationStep, error) {
	rows, err := s.DB.Query(`SELECT id, policy_id, step_order, wait_minutes FROM escalation_steps WHERE policy_id = ? ORDER BY step_order`, policyID)
	if err != nil {
		return nil, err
	}
	var steps []EscalationStep
	for rows.Next() {
		var st EscalationStep
		if err := rows.Scan(&st.ID, &st.PolicyID, &st.StepOrder, &st.WaitMinutes); err != nil {
			rows.Close()
			return nil, err
		}
		steps = append(steps, st)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i := range steps {
		targets, err := s.targetsForStep(steps[i].ID)
		if err != nil {
			return nil, err
		}
		steps[i].Targets = targets
	}
	return steps, nil
}

func (s *Store) targetsForStep(stepID string) ([]EscalationStepTarget, error) {
	rows, err := s.DB.Query(
		`SELECT id, step_id, target_type, target_id, via_email, via_push, via_slack, via_discord FROM escalation_step_targets WHERE step_id = ?`,
		stepID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []EscalationStepTarget
	for rows.Next() {
		var t EscalationStepTarget
		var email, push, slack, discord int
		if err := rows.Scan(&t.ID, &t.StepID, &t.TargetType, &t.TargetID, &email, &push, &slack, &discord); err != nil {
			return nil, err
		}
		t.ViaEmail, t.ViaPush, t.ViaSlack, t.ViaDiscord = email != 0, push != 0, slack != 0, discord != 0
		out = append(out, t)
	}
	return out, rows.Err()
}
