package httpserver

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/Laaaaksh/escalight/internal/db"
	"github.com/Laaaaksh/escalight/internal/notify"
)

// slackCommand handles Slack's slash command request for /escalight, e.g.
// "/escalight ack a1b2c3d4" or "/escalight resolve a1b2c3d4". Configure this
// URL as the command's Request URL in the Slack app; the signing secret is
// verified against the raw request body before anything else runs.
func (s *Server) slackCommand(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if !notify.VerifySlackSignature(s.Config.SlackSigningSecret, r.Header.Get("X-Slack-Request-Timestamp"), string(body), r.Header.Get("X-Slack-Signature")) {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	values, err := url.ParseQuery(string(body))
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	slackUserID := values.Get("user_id")
	text := strings.Fields(strings.TrimSpace(values.Get("text")))

	respond := func(text string) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"response_type": "ephemeral", "text": text})
	}

	if len(text) != 2 || (text[0] != "ack" && text[0] != "resolve") {
		respond("Usage: `/escalight ack <incident-id>` or `/escalight resolve <incident-id>` — the short incident ID is in every page notification.")
		return
	}
	action, idPrefix := text[0], text[1]

	user, err := s.Store.UserBySlackID(slackUserID)
	if err != nil {
		respond("Your Slack account isn't linked to an Escalight user yet. In Escalight, go to Settings and paste your Slack user ID (Slack profile → “Copy member ID”).")
		return
	}

	inc, err := s.Store.IncidentByIDPrefix(idPrefix)
	if err == db.ErrNotFound {
		respond("No open incident matches `" + idPrefix + "`. It may already be resolved, or the ID is ambiguous.")
		return
	}
	if err != nil {
		respond("Something went wrong looking that up.")
		return
	}

	switch action {
	case "ack":
		if err := s.Store.AcknowledgeIncident(inc.ID, user.ID); err != nil {
			respond("Failed to acknowledge.")
			return
		}
		s.logErr(s.Store.AddEvent(inc.ID, "acknowledged", user.Name+" (via Slack)", ""), "log slack acknowledge event")
		respond("Acknowledged *" + inc.Title + "*.")
	case "resolve":
		if err := s.Store.ResolveIncident(inc.ID, user.ID); err != nil {
			respond("Failed to resolve.")
			return
		}
		s.logErr(s.Store.AddEvent(inc.ID, "resolved", user.Name+" (via Slack)", ""), "log slack resolve event")
		respond("Resolved *" + inc.Title + "*.")
	}
}
