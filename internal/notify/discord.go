package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type DiscordSender struct {
	WebhookURL string
}

func (d *DiscordSender) Configured() bool {
	return d.WebhookURL != ""
}

func (d *DiscordSender) Post(content string) error {
	if !d.Configured() {
		return fmt.Errorf("discord not configured (set ESCALIGHT_DISCORD_WEBHOOK_URL)")
	}
	body, _ := json.Marshal(map[string]string{"content": content})
	resp, err := http.Post(d.WebhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("discord webhook returned %d", resp.StatusCode)
	}
	return nil
}
