package db

import (
	"database/sql"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	conn, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return NewStore(conn)
}

func TestUserCreateAndLookup(t *testing.T) {
	s := newTestStore(t)

	u, err := s.CreateUser("a@example.com", "Alice", "hash", true)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	got, err := s.UserByEmail("a@example.com")
	if err != nil {
		t.Fatalf("UserByEmail: %v", err)
	}
	if got.ID != u.ID || got.Name != "Alice" || !got.IsAdmin {
		t.Errorf("got %+v, want matching Alice/admin", got)
	}

	if _, err := s.UserByEmail("missing@example.com"); err != sql.ErrNoRows {
		t.Errorf("expected ErrNoRows for missing user, got %v", err)
	}
}

func TestPolicyStepsRoundTrip(t *testing.T) {
	s := newTestStore(t)
	u, _ := s.CreateUser("a@example.com", "Alice", "hash", false)

	p, err := s.CreatePolicy("Primary", "desc", 1)
	if err != nil {
		t.Fatalf("CreatePolicy: %v", err)
	}

	steps := []EscalationStep{
		{WaitMinutes: 5, Targets: []EscalationStepTarget{{TargetType: "user", TargetID: u.ID, ViaEmail: true, ViaPush: true}}},
		{WaitMinutes: 10, Targets: []EscalationStepTarget{{TargetType: "user", TargetID: u.ID, ViaSlack: true}}},
	}
	if err := s.ReplaceSteps(p.ID, steps); err != nil {
		t.Fatalf("ReplaceSteps: %v", err)
	}

	got, err := s.StepsForPolicy(p.ID)
	if err != nil {
		t.Fatalf("StepsForPolicy: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(got))
	}
	if got[0].StepOrder != 0 || got[0].WaitMinutes != 5 {
		t.Errorf("step 0 = %+v", got[0])
	}
	if got[1].StepOrder != 1 || got[1].WaitMinutes != 10 {
		t.Errorf("step 1 = %+v", got[1])
	}
	if len(got[0].Targets) != 1 || !got[0].Targets[0].ViaEmail || !got[0].Targets[0].ViaPush {
		t.Errorf("step 0 targets = %+v", got[0].Targets)
	}
	if len(got[1].Targets) != 1 || !got[1].Targets[0].ViaSlack {
		t.Errorf("step 1 targets = %+v", got[1].Targets)
	}

	// Replacing again must fully overwrite, not append.
	if err := s.ReplaceSteps(p.ID, steps[:1]); err != nil {
		t.Fatalf("ReplaceSteps (again): %v", err)
	}
	got, err = s.StepsForPolicy(p.ID)
	if err != nil {
		t.Fatalf("StepsForPolicy (again): %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 step after replace, got %d", len(got))
	}
}

func TestIncidentFingerprintDedup(t *testing.T) {
	s := newTestStore(t)
	u, _ := s.CreateUser("a@example.com", "Alice", "hash", false)
	p, _ := s.CreatePolicy("Primary", "", 0)
	s.ReplaceSteps(p.ID, []EscalationStep{{WaitMinutes: 5, Targets: []EscalationStepTarget{{TargetType: "user", TargetID: u.ID}}}})
	svc, _ := s.CreateService("API", p.ID)

	inc1, err := s.CreateIncident(CreateIncidentParams{ServiceID: svc.ID, Title: "high cpu", Source: "alertmanager", Fingerprint: "fp-1"})
	if err != nil {
		t.Fatalf("CreateIncident: %v", err)
	}

	// Same fingerprint, still open: should be found via OpenIncidentByFingerprint
	// rather than creating a second incident, so callers can dedupe re-fired alerts.
	found, err := s.OpenIncidentByFingerprint(svc.ID, "fp-1")
	if err != nil {
		t.Fatalf("OpenIncidentByFingerprint: %v", err)
	}
	if found.ID != inc1.ID {
		t.Errorf("expected to find existing incident %s, got %s", inc1.ID, found.ID)
	}

	if err := s.ResolveIncident(inc1.ID, u.ID); err != nil {
		t.Fatalf("ResolveIncident: %v", err)
	}

	// Once resolved, the same fingerprint should no longer match (a new alert should open a new incident).
	if _, err := s.OpenIncidentByFingerprint(svc.ID, "fp-1"); err != ErrNotFound {
		t.Errorf("expected ErrNotFound after resolve, got %v", err)
	}
}

func TestAcknowledgeIsOneWay(t *testing.T) {
	s := newTestStore(t)
	u, _ := s.CreateUser("a@example.com", "Alice", "hash", false)
	p, _ := s.CreatePolicy("Primary", "", 0)
	svc, _ := s.CreateService("API", p.ID)
	inc, _ := s.CreateIncident(CreateIncidentParams{ServiceID: svc.ID, Title: "t"})

	if err := s.AcknowledgeIncident(inc.ID, u.ID); err != nil {
		t.Fatalf("AcknowledgeIncident: %v", err)
	}
	got, _ := s.IncidentByID(inc.ID)
	if got.Status != "acknowledged" || !got.AcknowledgedBy.Valid || got.AcknowledgedBy.String != u.ID {
		t.Errorf("got %+v", got)
	}

	// A second ack attempt (already-acknowledged incident) must be a no-op, not
	// silently reset acknowledged_by to a different user.
	other, _ := s.CreateUser("b@example.com", "Bob", "hash", false)
	if err := s.AcknowledgeIncident(inc.ID, other.ID); err != nil {
		t.Fatalf("second AcknowledgeIncident: %v", err)
	}
	got2, _ := s.IncidentByID(inc.ID)
	if got2.AcknowledgedBy.String != u.ID {
		t.Errorf("expected acknowledged_by to remain %s, got %s", u.ID, got2.AcknowledgedBy.String)
	}
}
