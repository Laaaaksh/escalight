// Package config loads Escalight's server configuration from the environment.
package config

import "os"

type Config struct {
	Addr    string // listen address, e.g. ":8080"
	BaseURL string // public URL Escalight is reachable at; used in email/Slack/push links
	DBPath  string

	SMTPHost string
	SMTPPort string
	SMTPUser string
	SMTPPass string
	SMTPFrom string

	SlackWebhookURL    string
	SlackSigningSecret string

	DiscordWebhookURL string

	VAPIDPublicKey  string
	VAPIDPrivateKey string
	VAPIDSubject    string // mailto: or https: contact, required by the Web Push protocol
}

func Load() Config {
	return Config{
		Addr:    getEnv("ESCALIGHT_ADDR", ":8080"),
		BaseURL: getEnv("ESCALIGHT_BASE_URL", "http://localhost:8080"),
		DBPath:  getEnv("ESCALIGHT_DB_PATH", "escalight.db"),

		SMTPHost: os.Getenv("ESCALIGHT_SMTP_HOST"),
		SMTPPort: getEnv("ESCALIGHT_SMTP_PORT", "587"),
		SMTPUser: os.Getenv("ESCALIGHT_SMTP_USER"),
		SMTPPass: os.Getenv("ESCALIGHT_SMTP_PASS"),
		SMTPFrom: getEnv("ESCALIGHT_SMTP_FROM", "escalight@localhost"),

		SlackWebhookURL:    os.Getenv("ESCALIGHT_SLACK_WEBHOOK_URL"),
		SlackSigningSecret: os.Getenv("ESCALIGHT_SLACK_SIGNING_SECRET"),

		DiscordWebhookURL: os.Getenv("ESCALIGHT_DISCORD_WEBHOOK_URL"),

		// VAPID keys are normally left unset here and auto-generated on first
		// boot (see notify.EnsureVAPIDKeys), persisted in the settings table so
		// they stay stable across restarts. Set these only to pin specific keys.
		VAPIDPublicKey:  os.Getenv("ESCALIGHT_VAPID_PUBLIC_KEY"),
		VAPIDPrivateKey: os.Getenv("ESCALIGHT_VAPID_PRIVATE_KEY"),
		VAPIDSubject:    getEnv("ESCALIGHT_VAPID_SUBJECT", "mailto:admin@localhost"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
