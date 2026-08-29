<div align="center">

<img src="docs/assets/escalight-banner.svg" alt="Escalight" width="640">

**Escalight** — self-hosted on-call escalation and paging: define who gets notified, wait,
then escalate — over email, Slack, Discord, or a real push notification with an Acknowledge
button on the lock screen. No per-seat fee, no phone bill, one binary.

[![Star this repo](https://img.shields.io/github/stars/Laaaaksh/escalight?style=for-the-badge&logo=github&label=star%20this%20repo&color=yellow)](https://github.com/Laaaaksh/escalight/stargazers)
[![No per-seat pricing](https://img.shields.io/badge/pricing-$0%2Fseat%2C%20forever-00ADD8?style=for-the-badge)](#why-escalight)

[![CI](https://github.com/Laaaaksh/escalight/actions/workflows/ci.yml/badge.svg)](https://github.com/Laaaaksh/escalight/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/Laaaaksh/escalight?color=green&display_name=tag)](https://github.com/Laaaaksh/escalight/releases)
[![License: MIT](https://img.shields.io/badge/license-MIT-purple.svg)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.26+-00ADD8?logo=go&logoColor=white)](go.mod)
[![Docker](https://img.shields.io/badge/docker-ghcr.io-2496ED?logo=docker&logoColor=white)](#install)

**[Install](#install) • [Usage](#usage) • [Configuration](#configuration) • [Changelog](CHANGELOG.md) • [Contributing](CONTRIBUTING.md) • [License](LICENSE)**

**[Code of conduct](CODE_OF_CONDUCT.md) • [Security](SECURITY.md)**

</div>

## What it does

- **Escalation policies**: notify person A, wait N minutes, escalate to person B, repeat or
  stop — with a per-step choice of email, Slack, Discord, and/or push.
- **On-call schedules**: daily or weekly rotations with a two-week calendar view, plus
  one-off manual overrides for "Bob covers Friday instead of Alice."
- **Alert ingestion** from a generic JSON webhook, Prometheus Alertmanager, or an inbound
  email provider (Postmark, Mailgun, SendGrid) — no bespoke integration code needed for the
  two most common setups.
- **Real push notifications**: an installable PWA with web push (VAPID) — a page shows up on
  your phone's lock screen with the alert's actual title and an **Acknowledge** button that
  fires right from the notification, no app open required.
- **Acknowledge/resolve from anywhere**: the web UI, the push notification itself, or a Slack
  `/escalight ack <id>` slash command.
- **An incident timeline**: who was paged, when, and what they did — the minimum audit trail
  you need after a 3am page, not a full postmortem tool.
- **One binary, one SQLite file**: `go build`, run it, done. No Postgres, no Redis, no Node.js
  build step.

<div align="center">
<img src="docs/assets/escalight-demo.gif" alt="Escalight demo: a synthetic alert fires, appears as a triggered incident, then gets acknowledged with the timeline updating live" width="900">
</div>

*The GIF above is a real recording of the running app: a synthetic Prometheus Alertmanager
alert POSTed at the webhook endpoint, appearing as a triggered incident, then acknowledged
with the timeline updating live. It does not show an actual OS push notification banner —
capturing a real lock-screen notification isn't practical to script — but web push is real
and working; see [Configuration](#configuration).*

<div align="center">
<img src="docs/assets/schedule-calendar.png" alt="Escalight schedule page: a daily rotation editor and a two-week calendar view" width="900">
</div>

*The schedule page: a daily/weekly rotation editor above a two-week calendar showing who's
on call each day, plus manual overrides below.*

## Why Escalight

**Opsgenie is being discontinued** by Atlassian, forcing existing customers to migrate right
now. **PagerDuty** charges $21–41/user/month (plus a ~$699/month AIOps add-on), which punishes
exactly the teams growing fast enough to need real on-call discipline. The one credible free
alternative, [GoAlert](https://github.com/target/goalert), is actively maintained but
deliberately bare-bones — no mobile push, a utilitarian UI most non-engineers bounce off. The
other prior free option, [Grafana OnCall](https://github.com/grafana/oncall), was archived in
March 2026. Escalight aims to sit between GoAlert's minimalism and a full observability
platform: paging that works, self-hosted, free.

## Requirements

- Go 1.26+ to build from source (or use the Docker image / a release binary — no Go needed
  then)
- A place to run one small binary or container — no managed database, no external queue

## Install

**From source** (the only path available today — no tagged release has been published yet,
so the Docker image and release binaries below don't exist):

```bash
git clone https://github.com/Laaaaksh/escalight.git
cd escalight
go build -o escalight .
./escalight serve
```

**Docker / release binary (coming soon):** once the first tagged release ships, `docker run
ghcr.io/laaaaksh/escalight:latest` and a binary on the
[Releases](https://github.com/Laaaaksh/escalight/releases) page will also work — see
[CONTRIBUTING.md](CONTRIBUTING.md#releases) for how a release is cut.

## Usage

Visit `http://localhost:8080` — the first run redirects you to a setup page to create an
admin account. Then:

1. **Create an escalation policy** (Policies → New policy): who gets notified, in what order,
   with how long to wait between steps.
2. **Create a service** (Services → New service) and attach the policy — this gives you a
   webhook URL for a generic JSON alert, an Alertmanager-shaped webhook, and an email-in
   webhook.
3. **Fire a test alert:**

   ```bash
   curl -X POST http://localhost:8080/webhooks/generic/<service-webhook-key> \
     -H 'Content-Type: application/json' \
     -d '{"title":"disk full on db-1","description":"/var is at 98%"}'
   ```

4. Watch it show up under Incidents, get escalated per your policy, and acknowledge it from
   the incident page (or from a push notification, once you've enabled push in Settings).

Optionally, add a **schedule** (Schedules → New schedule) with a daily/weekly rotation, then
point an escalation step at "On-call: `<schedule name>`" instead of a specific person.

## Configuration

Escalight is configured entirely through environment variables — there's no config file.

| Variable | Default | Purpose |
| --- | --- | --- |
| `ESCALIGHT_ADDR` | `:8080` | Listen address |
| `ESCALIGHT_BASE_URL` | `http://localhost:8080` | Public URL, used in email/Slack/push links |
| `ESCALIGHT_DB_PATH` | `escalight.db` | SQLite database file path |
| `ESCALIGHT_SMTP_HOST`, `_PORT`, `_USER`, `_PASS`, `_FROM` | unset | Enable email notifications via any SMTP relay |
| `ESCALIGHT_SLACK_WEBHOOK_URL` | unset | Enable Slack notifications (Incoming Webhook URL) |
| `ESCALIGHT_SLACK_SIGNING_SECRET` | unset | Verifies the `/escalight` slash command's requests |
| `ESCALIGHT_DISCORD_WEBHOOK_URL` | unset | Enable Discord notifications |
| `ESCALIGHT_VAPID_SUBJECT` | `mailto:admin@localhost` | Contact URL required by the Web Push protocol |

Web push needs no configuration — VAPID keys are generated automatically on first boot and
persisted in the database. Each user enables it for their own device from **Settings**.

Slack's slash command (`/escalight ack <id>` / `/escalight resolve <id>`) needs a Slack app
with its Request URL set to `<base-url>/integrations/slack/commands` and the signing secret
above configured; the incoming-webhook notifications work with just the webhook URL, no app
needed.

Two things worth knowing about v1: **all schedule and override times are UTC** (no per-user
timezone conversion yet), and **SMS/voice calling isn't supported** — those need a paid
telephony provider (Twilio et al.), which would break the "free to run" premise. Email, Slack,
Discord, and push cover the free channels; a documented bring-your-own-Twilio path is a
natural fast-follow, not a v1 requirement.

See [docs/EMAIL_INTEGRATION.md](docs/EMAIL_INTEGRATION.md) for wiring up email-to-alert with
Postmark, Mailgun, or SendGrid.

## Changelog

See [CHANGELOG.md](CHANGELOG.md).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for the build/test/PR workflow.

## Security

See [SECURITY.md](SECURITY.md) for supported versions and how to report a vulnerability
privately.

## Star this repo

If Escalight is useful to you, [star it](https://github.com/Laaaaksh/escalight/stargazers) —
it helps others evaluating a PagerDuty/Opsgenie replacement find it.

## License

[MIT](LICENSE)
