package webhooks

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Laaaaksh/escalight/internal/db"
)

// alertmanagerPayload matches Prometheus Alertmanager's webhook_config
// payload: https://prometheus.io/docs/alerting/latest/configuration/#webhook_config
type alertmanagerPayload struct {
	Status string              `json:"status"`
	Alerts []alertmanagerAlert `json:"alerts"`
}

type alertmanagerAlert struct {
	Status       string            `json:"status"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	StartsAt     string            `json:"startsAt"`
	GeneratorURL string            `json:"generatorURL"`
	Fingerprint  string            `json:"fingerprint"`
}

// Alertmanager handles POST /webhooks/alertmanager/{key}. Each alert in the
// group is ingested independently, keyed on its own fingerprint, since a
// single Alertmanager group can bundle unrelated alerts together.
func (ig *Ingestor) Alertmanager(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	svc, err := ig.Store.ServiceByWebhookKey(key)
	if err == db.ErrNotFound {
		http.Error(w, "unknown webhook key", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	var p alertmanagerPayload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	results := make([]map[string]string, 0, len(p.Alerts))
	for _, alert := range p.Alerts {
		status := alert.Status
		if status == "" {
			status = p.Status
		}

		if status == "resolved" {
			if err := ig.ResolveByFingerprint(svc, alert.Fingerprint, "alertmanager"); err != nil {
				ig.log("alertmanager resolve", err)
			}
			continue
		}

		title := alert.Labels["alertname"]
		if title == "" {
			title = "Alertmanager alert"
		}
		description := alert.Annotations["description"]
		if description == "" {
			description = alert.Annotations["summary"]
		}
		if alert.GeneratorURL != "" {
			description = fmt.Sprintf("%s\n\nSource: %s", description, alert.GeneratorURL)
		}

		inc, _, err := ig.Ingest(svc, title, description, "alertmanager", alert.Fingerprint)
		if err != nil {
			ig.log("alertmanager ingest", err)
			continue
		}
		results = append(results, map[string]string{"incident_id": inc.ID})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"processed": results})
}
