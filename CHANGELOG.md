# Changelog

All notable changes to Escalight are documented in this file. Format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added
- Escalation policies: ordered, multi-step, with per-step notification channels and an
  optional repeat.
- On-call schedules: daily/weekly rotations, manual overrides, a two-week calendar view.
- Alert ingestion: a generic JSON webhook, a Prometheus Alertmanager adapter, and an
  inbound-email adapter.
- Notification channels: web push (VAPID, installable PWA), Slack (incoming webhook +
  `/escalight ack|resolve` slash command), Discord, and email.
- Acknowledge/resolve from the web UI, from a push notification's own action button, and
  from Slack.
- Incident timeline audit trail.
- Single SQLite-backed binary; email+password auth with a first-run setup flow.

No release has been tagged yet — see [CONTRIBUTING.md](CONTRIBUTING.md#releases) for how one
gets cut.
