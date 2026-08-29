package engine

import (
	"database/sql"
	"testing"
	"time"

	"github.com/Laaaaksh/escalight/internal/db"
	"github.com/Laaaaksh/escalight/internal/notify"
)

// sqlNullNow returns a valid NullString for "now + offset", used to force an
// incident into (or out of) the due-for-escalation window deterministically.
func sqlNullNow(offset time.Duration) sql.NullString {
	return sql.NullString{String: time.Now().UTC().Add(offset).Format(time.RFC3339), Valid: true}
}

func newTestEngine(t *testing.T) (*Engine, *db.Store) {
	t.Helper()
	conn, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	store := db.NewStore(conn)

	dispatcher := &notify.Dispatcher{
		Store:   store,
		BaseURL: "http://localhost:8080",
		Email:   &notify.EmailSender{},
		Slack:   &notify.SlackSender{},
		Discord: &notify.DiscordSender{},
		Push:    &notify.WebPushSender{},
	}
	e := New(store, dispatcher, nil)
	return e, store
}

// noChannels marks a target with every channel disabled, so NotifyUser is a
// safe no-op in tests that only care about step-advancement timing, not
// delivery (delivery is covered by the notify package's own tests).
func noChannels(targetType, targetID string) db.EscalationStepTarget {
	return db.EscalationStepTarget{TargetType: targetType, TargetID: targetID}
}

func TestTriggerIncident_FiresFirstStepAndSchedulesNext(t *testing.T) {
	e, store := newTestEngine(t)
	u, _ := store.CreateUser("a@example.com", "Alice", "hash", false)
	p, _ := store.CreatePolicy("Primary", "", 0)
	store.ReplaceSteps(p.ID, []db.EscalationStep{
		{WaitMinutes: 5, Targets: []db.EscalationStepTarget{{TargetType: "user", TargetID: u.ID, ViaEmail: true}}},
		{WaitMinutes: 10, Targets: []db.EscalationStepTarget{noChannels("user", u.ID)}},
	})
	svc, _ := store.CreateService("API", p.ID)
	inc, _ := store.CreateIncident(db.CreateIncidentParams{ServiceID: svc.ID, Title: "db down"})

	if err := e.TriggerIncident(inc); err != nil {
		t.Fatalf("TriggerIncident: %v", err)
	}

	got, err := store.IncidentByID(inc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.CurrentStep != 0 {
		t.Errorf("expected current_step 0, got %d", got.CurrentStep)
	}
	if !got.NextEscalationAt.Valid {
		t.Fatal("expected next_escalation_at to be set")
	}
	nextAt, err := time.Parse(time.RFC3339, got.NextEscalationAt.String)
	if err != nil {
		t.Fatal(err)
	}
	wantMin := time.Now().UTC().Add(4*time.Minute + 55*time.Second)
	if nextAt.Before(wantMin) {
		t.Errorf("next escalation scheduled too soon: %s (now=%s)", nextAt, time.Now().UTC())
	}

	events, err := store.EventsForIncident(inc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) == 0 {
		t.Fatal("expected at least one event logged for step 0 (email attempted, though unconfigured in the test dispatcher)")
	}
}

func TestTick_AdvancesDueIncidentToNextStep(t *testing.T) {
	e, store := newTestEngine(t)
	u, _ := store.CreateUser("a@example.com", "Alice", "hash", false)
	p, _ := store.CreatePolicy("Primary", "", 0)
	store.ReplaceSteps(p.ID, []db.EscalationStep{
		{WaitMinutes: 5, Targets: []db.EscalationStepTarget{noChannels("user", u.ID)}},
		{WaitMinutes: 10, Targets: []db.EscalationStepTarget{noChannels("user", u.ID)}},
	})
	svc, _ := store.CreateService("API", p.ID)
	inc, _ := store.CreateIncident(db.CreateIncidentParams{ServiceID: svc.ID, Title: "db down"})
	e.TriggerIncident(inc)

	// Force the incident to already be due, as if 5 minutes had passed.
	if err := store.SetNextEscalation(inc.ID, 0, 0, sqlNullNow(-time.Minute)); err != nil {
		t.Fatal(err)
	}

	e.Tick()

	got, err := store.IncidentByID(inc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.CurrentStep != 1 {
		t.Errorf("expected current_step 1 after tick, got %d", got.CurrentStep)
	}
	if got.Status != "triggered" {
		t.Errorf("expected incident to remain triggered, got %s", got.Status)
	}
}

func TestTick_StopsAtEndOfPolicyWithNoRepeat(t *testing.T) {
	e, store := newTestEngine(t)
	u, _ := store.CreateUser("a@example.com", "Alice", "hash", false)
	p, _ := store.CreatePolicy("Primary", "", 0) // repeat = 0
	store.ReplaceSteps(p.ID, []db.EscalationStep{
		{WaitMinutes: 5, Targets: []db.EscalationStepTarget{noChannels("user", u.ID)}},
	})
	svc, _ := store.CreateService("API", p.ID)
	inc, _ := store.CreateIncident(db.CreateIncidentParams{ServiceID: svc.ID, Title: "db down"})
	e.TriggerIncident(inc)
	store.SetNextEscalation(inc.ID, 0, 0, sqlNullNow(-time.Minute))

	e.Tick() // advances past the only step; policy has no repeat, so escalation stops

	got, err := store.IncidentByID(inc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.NextEscalationAt.Valid {
		t.Errorf("expected no further escalation scheduled, got %s", got.NextEscalationAt.String)
	}
	if got.Status != "triggered" {
		t.Errorf("incident should remain triggered (not auto-resolved) after policy exhausted, got %s", got.Status)
	}

	// A subsequent tick must not panic or re-fire now that next_escalation_at is NULL.
	e.Tick()
}

func TestTick_RepeatsPolicyWhenConfigured(t *testing.T) {
	e, store := newTestEngine(t)
	u, _ := store.CreateUser("a@example.com", "Alice", "hash", false)
	p, _ := store.CreatePolicy("Primary", "", 1) // repeat once
	store.ReplaceSteps(p.ID, []db.EscalationStep{
		{WaitMinutes: 5, Targets: []db.EscalationStepTarget{noChannels("user", u.ID)}},
	})
	svc, _ := store.CreateService("API", p.ID)
	inc, _ := store.CreateIncident(db.CreateIncidentParams{ServiceID: svc.ID, Title: "db down"})
	e.TriggerIncident(inc)
	store.SetNextEscalation(inc.ID, 0, 0, sqlNullNow(-time.Minute))

	e.Tick() // exhausts the single step, repeat_count 0 < repeat 1: restarts at step 0

	got, err := store.IncidentByID(inc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.CurrentStep != 0 {
		t.Errorf("expected restart at step 0, got %d", got.CurrentStep)
	}
	if got.RepeatCount != 1 {
		t.Errorf("expected repeat_count 1, got %d", got.RepeatCount)
	}
	if !got.NextEscalationAt.Valid {
		t.Error("expected next escalation to be scheduled again after repeat")
	}
}

func TestTick_DoesNotTouchAcknowledgedIncident(t *testing.T) {
	e, store := newTestEngine(t)
	u, _ := store.CreateUser("a@example.com", "Alice", "hash", false)
	p, _ := store.CreatePolicy("Primary", "", 0)
	store.ReplaceSteps(p.ID, []db.EscalationStep{
		{WaitMinutes: 5, Targets: []db.EscalationStepTarget{noChannels("user", u.ID)}},
		{WaitMinutes: 10, Targets: []db.EscalationStepTarget{noChannels("user", u.ID)}},
	})
	svc, _ := store.CreateService("API", p.ID)
	inc, _ := store.CreateIncident(db.CreateIncidentParams{ServiceID: svc.ID, Title: "db down"})
	e.TriggerIncident(inc)

	if err := store.AcknowledgeIncident(inc.ID, u.ID); err != nil {
		t.Fatal(err)
	}

	// DueForEscalation only returns status='triggered' rows, so an acked
	// incident (even with a stale next_escalation_at) must never be picked up.
	e.Tick()

	got, err := store.IncidentByID(inc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.CurrentStep != 0 {
		t.Errorf("acknowledged incident's step must not advance, got %d", got.CurrentStep)
	}
}

func TestScheduleTarget_ResolvesOnCallUser(t *testing.T) {
	e, store := newTestEngine(t)
	alice, _ := store.CreateUser("alice@example.com", "Alice", "hash", false)
	bob, _ := store.CreateUser("bob@example.com", "Bob", "hash", false)

	sch, _ := store.CreateSchedule("Primary on-call", "UTC")
	// Rotation started well in the past so "now" is deterministically bob's shift (odd day offset).
	start := time.Now().UTC().Add(-25 * time.Hour).Format(time.RFC3339)
	if err := store.SetRotation(sch.ID, "daily", "09:00", start, []string{alice.ID, bob.ID}); err != nil {
		t.Fatal(err)
	}

	p, _ := store.CreatePolicy("Primary", "", 0)
	store.ReplaceSteps(p.ID, []db.EscalationStep{
		{WaitMinutes: 5, Targets: []db.EscalationStepTarget{noChannels("schedule", sch.ID)}},
	})
	svc, _ := store.CreateService("API", p.ID)
	inc, _ := store.CreateIncident(db.CreateIncidentParams{ServiceID: svc.ID, Title: "db down"})

	if err := e.TriggerIncident(inc); err != nil {
		t.Fatal(err)
	}

	events, err := store.EventsForIncident(inc.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, ev := range events {
		if ev.EventType == "notify_failed" {
			t.Fatalf("unexpected notify_failed event (schedule should have resolved a user): %s", ev.Detail)
		}
	}
	// The escalation should have advanced (next_escalation_at set), proving the
	// schedule target was resolved to a real on-call user rather than erroring out.
	got, _ := store.IncidentByID(inc.ID)
	if !got.NextEscalationAt.Valid {
		t.Error("expected next escalation to be scheduled, indicating the schedule target resolved successfully")
	}
}
