package engine

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/Laaaaksh/escalight/internal/db"
	"github.com/Laaaaksh/escalight/internal/notify"
)

// Engine advances triggered incidents through their escalation policy: it
// fires the current step's notifications, waits the step's configured
// duration, then either advances to the next step, repeats the policy from
// the top, or stops once the policy is exhausted. It never touches an
// acknowledged or resolved incident.
type Engine struct {
	Store    *db.Store
	Notify   *notify.Dispatcher
	Logger   *slog.Logger
	Interval time.Duration
}

func New(store *db.Store, dispatcher *notify.Dispatcher, logger *slog.Logger) *Engine {
	return &Engine{Store: store, Notify: dispatcher, Logger: logger, Interval: 15 * time.Second}
}

// Run polls for incidents due for escalation until ctx is cancelled.
func (e *Engine) Run(ctx context.Context) {
	ticker := time.NewTicker(e.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.Tick()
		}
	}
}

// Tick processes every incident whose next_escalation_at has passed. Exported
// so callers (and tests) can drive the engine deterministically instead of
// waiting on the ticker.
func (e *Engine) Tick() {
	due, err := e.Store.DueForEscalation(time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		e.log("query due incidents", err)
		return
	}
	for _, inc := range due {
		if err := e.fireStep(inc, inc.CurrentStep+1, inc.RepeatCount); err != nil {
			e.log("advance incident "+inc.ID, err)
		}
	}
}

// TriggerIncident fires step 0 of the incident's service's escalation policy
// immediately. Call this once, right after creating a new incident.
func (e *Engine) TriggerIncident(inc *db.Incident) error {
	return e.fireStep(inc, 0, 0)
}

func (e *Engine) fireStep(inc *db.Incident, stepIdx, repeatCount int) error {
	svc, err := e.Store.ServiceByID(inc.ServiceID)
	if err != nil {
		return fmt.Errorf("load service: %w", err)
	}
	steps, err := e.Store.StepsForPolicy(svc.EscalationPolicyID)
	if err != nil {
		return fmt.Errorf("load steps: %w", err)
	}

	if stepIdx >= len(steps) {
		policy, err := e.Store.PolicyByID(svc.EscalationPolicyID)
		if err != nil {
			return fmt.Errorf("load policy: %w", err)
		}
		if len(steps) > 0 && repeatCount < policy.Repeat {
			return e.fireStep(inc, 0, repeatCount+1)
		}
		e.log("log policy-exhausted event", e.Store.AddEvent(inc.ID, "escalated", "system", "escalation policy exhausted; no further steps"))
		return e.Store.SetNextEscalation(inc.ID, stepIdx, repeatCount, sql.NullString{})
	}

	step := steps[stepIdx]
	notified := 0
	for _, target := range step.Targets {
		userIDs, err := e.resolveTargetUsers(target)
		if err != nil {
			e.log("resolve target", err)
			continue
		}
		for _, uid := range userIDs {
			user, err := e.Store.UserByID(uid)
			if err != nil {
				e.log("load target user "+uid, err)
				continue
			}
			e.Notify.NotifyUser(inc, user, target)
			notified++
		}
	}
	if notified == 0 {
		e.log("log no-resolvable-target event", e.Store.AddEvent(inc.ID, "notify_failed", "system", fmt.Sprintf("step %d has no resolvable on-call user", stepIdx+1)))
	}

	nextAt := time.Now().UTC().Add(time.Duration(step.WaitMinutes) * time.Minute)
	return e.Store.SetNextEscalation(inc.ID, stepIdx, repeatCount, sql.NullString{String: nextAt.Format(time.RFC3339), Valid: true})
}

// resolveTargetUsers turns a step target into concrete user IDs: a "user"
// target is just itself; a "schedule" target resolves to whoever is on call
// right now (manual overrides included).
func (e *Engine) resolveTargetUsers(target db.EscalationStepTarget) ([]string, error) {
	switch target.TargetType {
	case "user":
		return []string{target.TargetID}, nil
	case "schedule":
		sch, err := e.Store.ScheduleByID(target.TargetID)
		if err != nil {
			return nil, fmt.Errorf("load schedule: %w", err)
		}
		now := time.Now().UTC()
		overrides, err := e.Store.OverridesInRange(sch.ID, now.Format(time.RFC3339), now.Add(time.Second).Format(time.RFC3339))
		if err != nil {
			return nil, fmt.Errorf("load overrides: %w", err)
		}
		uid, err := OnCallUserID(sch, overrides, now)
		if err != nil {
			return nil, err
		}
		if uid == "" {
			return nil, nil
		}
		return []string{uid}, nil
	default:
		return nil, fmt.Errorf("unknown target type %q", target.TargetType)
	}
}

func (e *Engine) log(msg string, err error) {
	if err != nil && e.Logger != nil {
		e.Logger.Error(msg, "error", err)
	}
}
