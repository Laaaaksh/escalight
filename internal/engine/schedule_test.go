package engine

import (
	"testing"
	"time"

	"github.com/Laaaaksh/escalight/internal/db"
)

func mustParse(t *testing.T, s string) time.Time {
	t.Helper()
	tm, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return tm
}

func TestOnCallUserID_DailyRotation(t *testing.T) {
	sch := &db.Schedule{
		Rotation: &db.ScheduleRotation{
			RotationType: "daily",
			StartAt:      "2026-08-01T09:00:00Z",
			UserOrder:    []string{"alice", "bob", "carol"},
		},
	}

	cases := []struct {
		at   string
		want string
	}{
		{"2026-08-01T09:00:00Z", "alice"},
		{"2026-08-01T20:00:00Z", "alice"}, // still day 0
		{"2026-08-02T09:00:00Z", "bob"},   // exactly one period later
		{"2026-08-03T09:00:01Z", "carol"},
		{"2026-08-04T09:00:00Z", "alice"}, // wraps back to the start of the list
		{"2026-07-31T00:00:00Z", "alice"}, // before rotation start: clamp to first
	}
	for _, c := range cases {
		got, err := OnCallUserID(sch, nil, mustParse(t, c.at))
		if err != nil {
			t.Fatalf("OnCallUserID(%s): %v", c.at, err)
		}
		if got != c.want {
			t.Errorf("OnCallUserID(%s) = %q, want %q", c.at, got, c.want)
		}
	}
}

func TestOnCallUserID_WeeklyRotation(t *testing.T) {
	sch := &db.Schedule{
		Rotation: &db.ScheduleRotation{
			RotationType: "weekly",
			StartAt:      "2026-08-03T09:00:00Z", // a Monday
			UserOrder:    []string{"alice", "bob"},
		},
	}

	got, err := OnCallUserID(sch, nil, mustParse(t, "2026-08-09T08:00:00Z")) // still within week 0
	if err != nil {
		t.Fatal(err)
	}
	if got != "alice" {
		t.Errorf("expected alice still on call, got %s", got)
	}

	got, err = OnCallUserID(sch, nil, mustParse(t, "2026-08-10T10:00:00Z")) // one week later
	if err != nil {
		t.Fatal(err)
	}
	if got != "bob" {
		t.Errorf("expected bob on call after handoff, got %s", got)
	}
}

func TestOnCallUserID_OverrideWins(t *testing.T) {
	sch := &db.Schedule{
		Rotation: &db.ScheduleRotation{
			RotationType: "daily",
			StartAt:      "2026-08-01T09:00:00Z",
			UserOrder:    []string{"alice", "bob"},
		},
	}
	overrides := []*db.ScheduleOverride{
		{UserID: "dana", StartAt: "2026-08-01T12:00:00Z", EndAt: "2026-08-01T18:00:00Z"},
	}

	// Inside the override window: dana, even though alice is the rotation's on-call.
	got, err := OnCallUserID(sch, overrides, mustParse(t, "2026-08-01T15:00:00Z"))
	if err != nil {
		t.Fatal(err)
	}
	if got != "dana" {
		t.Errorf("expected override user dana, got %s", got)
	}

	// Outside the override window: back to the rotation.
	got, err = OnCallUserID(sch, overrides, mustParse(t, "2026-08-01T19:00:00Z"))
	if err != nil {
		t.Fatal(err)
	}
	if got != "alice" {
		t.Errorf("expected rotation user alice after override ends, got %s", got)
	}
}

func TestOnCallUserID_NoRotation(t *testing.T) {
	sch := &db.Schedule{}
	got, err := OnCallUserID(sch, nil, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Errorf("expected empty user for schedule with no rotation, got %q", got)
	}
}

func TestShiftAt(t *testing.T) {
	sch := &db.Schedule{
		Rotation: &db.ScheduleRotation{
			RotationType: "daily",
			StartAt:      "2026-08-01T09:00:00Z",
			UserOrder:    []string{"alice", "bob"},
		},
	}

	user, start, end, err := ShiftAt(sch, mustParse(t, "2026-08-02T15:00:00Z"))
	if err != nil {
		t.Fatal(err)
	}
	if user != "bob" {
		t.Errorf("expected bob, got %s", user)
	}
	wantStart := mustParse(t, "2026-08-02T09:00:00Z")
	wantEnd := mustParse(t, "2026-08-03T09:00:00Z")
	if !start.Equal(wantStart) || !end.Equal(wantEnd) {
		t.Errorf("shift = [%s, %s), want [%s, %s)", start, end, wantStart, wantEnd)
	}
}
