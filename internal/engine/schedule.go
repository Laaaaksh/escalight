// Package engine implements Escalight's two pieces of scheduling logic:
// resolving who is on call for a schedule at a given instant, and advancing
// incidents through their escalation policy over time.
package engine

import (
	"fmt"
	"time"

	"github.com/Laaaaksh/escalight/internal/db"
)

// OnCallUserID returns the user ID on call for sch at instant at, or "" if
// the schedule has no rotation configured yet or an empty user_order.
// A manual override active at `at` always wins over the computed rotation.
func OnCallUserID(sch *db.Schedule, overrides []*db.ScheduleOverride, at time.Time) (string, error) {
	for _, o := range overrides {
		start, err := time.Parse(time.RFC3339, o.StartAt)
		if err != nil {
			return "", fmt.Errorf("parse override start: %w", err)
		}
		end, err := time.Parse(time.RFC3339, o.EndAt)
		if err != nil {
			return "", fmt.Errorf("parse override end: %w", err)
		}
		if !at.Before(start) && at.Before(end) {
			return o.UserID, nil
		}
	}

	if sch.Rotation == nil || len(sch.Rotation.UserOrder) == 0 {
		return "", nil
	}
	rot := sch.Rotation

	start, err := time.Parse(time.RFC3339, rot.StartAt)
	if err != nil {
		return "", fmt.Errorf("parse rotation start_at: %w", err)
	}

	var period time.Duration
	switch rot.RotationType {
	case "daily":
		period = 24 * time.Hour
	case "weekly":
		period = 7 * 24 * time.Hour
	default:
		return "", fmt.Errorf("unknown rotation type %q", rot.RotationType)
	}

	elapsed := at.Sub(start)
	if elapsed < 0 {
		// Rotation hasn't started yet: the first person on the list is on call
		// from the moment the schedule is created, so there's no gap.
		return rot.UserOrder[0], nil
	}

	idx := int(elapsed/period) % len(rot.UserOrder)
	return rot.UserOrder[idx], nil
}

// ShiftAt returns the [start, end) instants of the on-call shift that
// contains `at`, along with the on-call user for that shift. Used to render
// the schedule calendar view.
func ShiftAt(sch *db.Schedule, at time.Time) (userID string, start, end time.Time, err error) {
	if sch.Rotation == nil || len(sch.Rotation.UserOrder) == 0 {
		return "", time.Time{}, time.Time{}, nil
	}
	rot := sch.Rotation

	rotStart, err := time.Parse(time.RFC3339, rot.StartAt)
	if err != nil {
		return "", time.Time{}, time.Time{}, fmt.Errorf("parse rotation start_at: %w", err)
	}

	var period time.Duration
	switch rot.RotationType {
	case "daily":
		period = 24 * time.Hour
	case "weekly":
		period = 7 * 24 * time.Hour
	default:
		return "", time.Time{}, time.Time{}, fmt.Errorf("unknown rotation type %q", rot.RotationType)
	}

	elapsed := at.Sub(rotStart)
	shiftIdx := int64(0)
	if elapsed >= 0 {
		shiftIdx = int64(elapsed / period)
	} else {
		shiftIdx = -1 // clamp: before rotation start, treat as the first shift starting at rotStart
	}
	if shiftIdx < 0 {
		shiftIdx = 0
	}

	start = rotStart.Add(time.Duration(shiftIdx) * period)
	end = start.Add(period)
	userIdx := int(shiftIdx) % len(rot.UserOrder)
	return rot.UserOrder[userIdx], start, end, nil
}
