package notify

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type SlackSender struct {
	WebhookURL string
}

func (s *SlackSender) Configured() bool {
	return s.WebhookURL != ""
}

// Post sends a plain-text message to the configured Slack incoming webhook.
// mention, if non-empty, is a Slack user ID (e.g. "U012ABC3DE") rendered as
// an @-mention so the on-call user's phone actually buzzes.
func (s *SlackSender) Post(text, mention string) error {
	if !s.Configured() {
		return fmt.Errorf("slack not configured (set ESCALIGHT_SLACK_WEBHOOK_URL)")
	}
	if mention != "" {
		text = fmt.Sprintf("<@%s> %s", mention, text)
	}
	body, _ := json.Marshal(map[string]string{"text": text})
	resp, err := http.Post(s.WebhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("slack webhook returned %d", resp.StatusCode)
	}
	return nil
}

// VerifySlackSignature checks a Slack slash-command/interactivity request
// against its signing secret, per Slack's documented v0 signature scheme:
// https://api.slack.com/authentication/verifying-requests-from-slack
//
// timestamp and body are the raw X-Slack-Request-Timestamp header and raw
// request body; sig is the raw X-Slack-Signature header.
func VerifySlackSignature(signingSecret, timestamp, body, sig string) bool {
	if signingSecret == "" || timestamp == "" || sig == "" {
		return false
	}

	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return false
	}
	// Reject requests older than 5 minutes to block replay of a captured request.
	if age := time.Now().Unix() - ts; age > 300 || age < -300 {
		return false
	}

	base := "v0:" + timestamp + ":" + body
	mac := hmac.New(sha256.New, []byte(signingSecret))
	mac.Write([]byte(base))
	expected := "v0=" + hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expected), []byte(strings.TrimSpace(sig)))
}
