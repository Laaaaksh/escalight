package httpserver

import (
	"encoding/json"
	"net/http"
	"strings"
)

type settingsPageData struct {
	pageData
	VAPIDPublicKey    string
	EmailConfigured   bool
	SlackConfigured   bool
	DiscordConfigured bool
}

func (s *Server) settingsPage(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, "settings.html", settingsPageData{
		pageData:          s.pageData(r, "settings"),
		VAPIDPublicKey:    s.VAPIDPub,
		EmailConfigured:   s.Notify.Email.Configured(),
		SlackConfigured:   s.Notify.Slack.Configured(),
		DiscordConfigured: s.Notify.Discord.Configured(),
	})
}

func (s *Server) settingsUpdateSlack(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	slackID := strings.TrimSpace(r.FormValue("slack_user_id"))
	s.logErr(s.Store.UpdateUserSlackID(contextUser(r).ID, slackID), "update slack user id")
	http.Redirect(w, r, "/settings", http.StatusFound)
}

type pushSubscribeRequest struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

func (s *Server) pushSubscribe(w http.ResponseWriter, r *http.Request) {
	var req pushSubscribeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Endpoint == "" {
		http.Error(w, "invalid subscription", http.StatusBadRequest)
		return
	}
	user := contextUser(r)
	if err := s.Store.SavePushSubscription(user.ID, req.Endpoint, req.Keys.P256dh, req.Keys.Auth); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) pushUnsubscribe(w http.ResponseWriter, r *http.Request) {
	var req pushSubscribeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	user := contextUser(r)
	s.logErr(s.Store.DeletePushSubscription(user.ID, req.Endpoint), "delete push subscription")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) pushTest(w http.ResponseWriter, r *http.Request) {
	user := contextUser(r)
	subs, err := s.Store.PushSubscriptionsForUser(user.ID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if len(subs) == 0 {
		http.Error(w, "no push subscription on file yet - click Enable push first", http.StatusBadRequest)
		return
	}
	payload, _ := json.Marshal(map[string]string{
		"title": "Escalight test notification",
		"body":  "If you can see this, web push is working.",
		"url":   "/settings",
	})
	sent := false
	for _, sub := range subs {
		if gone, err := s.Notify.Push.Send(sub, payload); err == nil && !gone {
			sent = true
		} else if gone {
			s.logErr(s.Store.DeletePushSubscription(user.ID, sub.Endpoint), "delete stale push subscription")
		}
	}
	if !sent {
		http.Error(w, "failed to deliver test push", http.StatusBadGateway)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
