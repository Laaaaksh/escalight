package webhooks

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Laaaaksh/escalight/internal/db"
)

type genericPayload struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	DedupKey    string `json:"dedup_key"`
}

// Generic handles POST /webhooks/generic/{key}: a minimal JSON contract any
// monitoring tool can hit with a simple HTTP POST.
func (ig *Ingestor) Generic(w http.ResponseWriter, r *http.Request) {
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

	var p genericPayload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if p.Title == "" {
		http.Error(w, `"title" is required`, http.StatusBadRequest)
		return
	}

	inc, created, err := ig.Ingest(svc, p.Title, p.Description, "generic", p.DedupKey)
	if err != nil {
		ig.log("generic ingest", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if created {
		w.WriteHeader(http.StatusCreated)
	} else {
		w.WriteHeader(http.StatusOK)
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"incident_id": inc.ID, "status": inc.Status})
}
