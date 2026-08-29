// Package webhooks turns inbound alerts (a generic JSON webhook, Prometheus
// Alertmanager, or an inbound-email provider) into Escalight incidents.
package webhooks

import (
	"fmt"
	"log/slog"

	"github.com/Laaaaksh/escalight/internal/db"
	"github.com/Laaaaksh/escalight/internal/engine"
)

type Ingestor struct {
	Store  *db.Store
	Engine *engine.Engine
	Logger *slog.Logger
}

// Ingest creates a new incident for svc, or - if fingerprint matches an
// already-open incident - logs the re-fire against the existing one instead
// of paging a second time for the same underlying alert.
func (ig *Ingestor) Ingest(svc *db.Service, title, description, source, fingerprint string) (*db.Incident, bool, error) {
	if fingerprint != "" {
		existing, err := ig.Store.OpenIncidentByFingerprint(svc.ID, fingerprint)
		if err == nil {
			ig.log("log duplicate-alert event", ig.Store.AddEvent(existing.ID, "created", source, "duplicate alert re-fired; not re-triggering"))
			return existing, false, nil
		}
		if err != db.ErrNotFound {
			return nil, false, fmt.Errorf("check existing incident: %w", err)
		}
	}

	inc, err := ig.Store.CreateIncident(db.CreateIncidentParams{
		ServiceID:   svc.ID,
		Title:       title,
		Description: description,
		Source:      source,
		Fingerprint: fingerprint,
	})
	if err != nil {
		return nil, false, fmt.Errorf("create incident: %w", err)
	}
	if err := ig.Store.AddEvent(inc.ID, "created", source, title); err != nil {
		ig.log("log created event", err)
	}

	if err := ig.Engine.TriggerIncident(inc); err != nil {
		ig.log("trigger incident "+inc.ID, err)
	}
	return inc, true, nil
}

// ResolveByFingerprint resolves the open incident matching svc+fingerprint,
// if any (used when a source like Alertmanager reports an alert as resolved).
func (ig *Ingestor) ResolveByFingerprint(svc *db.Service, fingerprint, source string) error {
	if fingerprint == "" {
		return nil
	}
	existing, err := ig.Store.OpenIncidentByFingerprint(svc.ID, fingerprint)
	if err == db.ErrNotFound {
		return nil
	}
	if err != nil {
		return err
	}
	if err := ig.Store.ResolveIncident(existing.ID, ""); err != nil {
		return err
	}
	return ig.Store.AddEvent(existing.ID, "resolved", source, "source reported alert resolved")
}

func (ig *Ingestor) log(msg string, err error) {
	if err != nil && ig.Logger != nil {
		ig.Logger.Error(msg, "error", err)
	}
}
