// Package notify sends incident notifications over email, Slack, Discord,
// and web push, and logs the outcome of each attempt.
package notify

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/Laaaaksh/escalight/internal/db"
)

type Dispatcher struct {
	Store   *db.Store
	BaseURL string

	Email   *EmailSender
	Slack   *SlackSender
	Discord *DiscordSender
	Push    *WebPushSender
	Logger  *slog.Logger
}

type pushPayload struct {
	Title      string `json:"title"`
	Body       string `json:"body"`
	IncidentID string `json:"incidentId"`
	URL        string `json:"url"`
	AckURL     string `json:"ackUrl"`
}

// NotifyUser sends an incident notification to a user over every channel the
// step target enabled, and records an incident_events row per attempt.
func (d *Dispatcher) NotifyUser(inc *db.Incident, user *db.User, target db.EscalationStepTarget) {
	incidentURL := fmt.Sprintf("%s/incidents/%s", d.BaseURL, inc.ID)
	subject := fmt.Sprintf("[Escalight] %s", inc.Title)
	body := fmt.Sprintf("%s\n\n%s\n\nAcknowledge: %s", inc.Title, inc.Description, incidentURL)

	if target.ViaEmail {
		err := d.Email.Send(user.Email, subject, body)
		d.logAttempt(inc.ID, user, "email", err)
	}

	if target.ViaSlack {
		mention := user.SlackUserID
		shortID := inc.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		text := fmt.Sprintf("*%s* — %s\n%s\nAck from here: `/escalight ack %s`", inc.Title, inc.Description, incidentURL, shortID)
		err := d.Slack.Post(text, mention)
		d.logAttempt(inc.ID, user, "slack", err)
	}

	if target.ViaDiscord {
		err := d.Discord.Post(fmt.Sprintf("**%s** — %s\n%s", inc.Title, inc.Description, incidentURL))
		d.logAttempt(inc.ID, user, "discord", err)
	}

	if target.ViaPush {
		d.sendPush(inc, user, incidentURL)
	}
}

func (d *Dispatcher) sendPush(inc *db.Incident, user *db.User, incidentURL string) {
	subs, err := d.Store.PushSubscriptionsForUser(user.ID)
	if err != nil {
		d.logAttempt(inc.ID, user, "push", err)
		return
	}
	if len(subs) == 0 {
		return // user hasn't enabled push in their browser; not an error worth logging per-attempt
	}

	payload, err := json.Marshal(pushPayload{
		Title:      inc.Title,
		Body:       inc.Description,
		IncidentID: inc.ID,
		URL:        incidentURL,
		AckURL:     fmt.Sprintf("%s/api/incidents/%s/acknowledge", d.BaseURL, inc.ID),
	})
	if err != nil {
		d.logAttempt(inc.ID, user, "push", err)
		return
	}

	var lastErr error
	delivered := 0
	for _, sub := range subs {
		gone, err := d.Push.Send(sub, payload)
		if gone {
			if err := d.Store.DeletePushSubscription(user.ID, sub.Endpoint); err != nil && d.Logger != nil {
				d.Logger.Warn("delete stale push subscription failed", "user", user.Email, "error", err)
			}
			continue
		}
		if err != nil {
			lastErr = err
			continue
		}
		delivered++
	}
	if delivered == 0 && lastErr != nil {
		d.logAttempt(inc.ID, user, "push", lastErr)
	} else {
		d.logAttempt(inc.ID, user, "push", nil)
	}
}

func (d *Dispatcher) logAttempt(incidentID string, user *db.User, channel string, err error) {
	var eventErr error
	if err != nil {
		if d.Logger != nil {
			d.Logger.Warn("notification failed", "channel", channel, "user", user.Email, "incident", incidentID, "error", err)
		}
		eventErr = d.Store.AddEvent(incidentID, "notify_failed", "system", fmt.Sprintf("%s to %s: %v", channel, user.Email, err))
	} else {
		eventErr = d.Store.AddEvent(incidentID, "notified", "system", fmt.Sprintf("%s to %s", channel, user.Email))
	}
	if eventErr != nil && d.Logger != nil {
		d.Logger.Warn("log notification event failed", "incident", incidentID, "error", eventErr)
	}
}
