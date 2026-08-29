package webhooks

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Laaaaksh/escalight/internal/db"
)

// emailInPayload accepts either lowercase generic field names or Postmark's
// inbound-webhook field names (From/Subject/TextBody), since Postmark's
// payload can be posted here with no transformation. Other providers (Mailgun,
// SendGrid Inbound Parse) can be pointed at this endpoint with a small
// payload template in their dashboard - see docs/EMAIL_INTEGRATION.md.
type emailInPayload struct {
	From      string `json:"from"`
	Subject   string `json:"subject"`
	Text      string `json:"text"`
	FromPM    string `json:"From"`
	SubjectPM string `json:"Subject"`
	TextPM    string `json:"TextBody"`
}

func (p emailInPayload) from() string {
	if p.From != "" {
		return p.From
	}
	return p.FromPM
}

func (p emailInPayload) subject() string {
	if p.Subject != "" {
		return p.Subject
	}
	return p.SubjectPM
}

func (p emailInPayload) text() string {
	if p.Text != "" {
		return p.Text
	}
	return p.TextPM
}

// EmailIn handles POST /webhooks/email-in/{key}: turns a forwarded email
// (relayed by the operator's own inbound-email provider) into an incident.
// There is no bundled SMTP server - see docs/EMAIL_INTEGRATION.md.
func (ig *Ingestor) EmailIn(w http.ResponseWriter, r *http.Request) {
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

	var p emailInPayload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	subject := p.subject()
	if subject == "" {
		subject = "(no subject)"
	}
	title := subject
	description := p.text()
	if from := p.from(); from != "" {
		description = "From: " + from + "\n\n" + description
	}

	// Emails have no natural fingerprint; dedupe on subject so a mail thread
	// (same subject, multiple replies) doesn't open a new incident per message.
	fingerprint := fingerprintOf(subject)

	inc, created, err := ig.Ingest(svc, title, description, "email", fingerprint)
	if err != nil {
		ig.log("email-in ingest", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if created {
		w.WriteHeader(http.StatusCreated)
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"incident_id": inc.ID})
}

func fingerprintOf(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:8])
}
