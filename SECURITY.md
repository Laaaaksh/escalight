# Security Policy

## Supported versions

Escalight is a young project. Security fixes are made against the **latest release** and
`main` only — please confirm you can reproduce the issue on the newest release before
reporting.

| Version        | Supported |
| -------------- | --------- |
| latest release | yes       |
| older releases | no        |

## Reporting a vulnerability

Please do **not** open a public GitHub issue for anything you believe is a security problem.

Use GitHub's private vulnerability reporting instead:

> https://github.com/Laaaaksh/escalight/security/advisories/new

That link reaches the maintainer privately — the report, follow-up discussion, and any fix
coordination stay confidential until a patched release ships.

When reporting, please include:

- your Escalight version (`escalight version`)
- how you deployed it (Docker, systemd, `go run`, etc.)
- clear steps to reproduce, including any relevant configuration (environment variables)

## What belongs in a report

Escalight pages real people and holds account credentials, webhook keys, and Slack/Discord
tokens for a whole team. Things worth reporting:

- **Auth bypass**: any path to a session, an admin action, or another user's incidents
  without a valid session cookie.
- **Webhook key exposure or forgery**: a way to guess, enumerate, or bypass a service's
  webhook key (the secret in `/webhooks/generic/<key>`, `/webhooks/alertmanager/<key>`, and
  `/webhooks/email-in/<key>`) to inject a fake incident or read another service's alerts.
- **Slack request forgery**: a way to bypass the `X-Slack-Signature` verification on
  `/integrations/slack/commands` and trigger an acknowledge/resolve as another user.
- **Stored XSS**: alert titles/descriptions or incident timeline text rendering as
  unescaped HTML anywhere in the dashboard (Escalight renders all of this through Go's
  `html/template`, which auto-escapes by default — a bypass of that is the interesting bug).
- **SQL injection**: any query path that doesn't go through a parameterized statement.
- **Web push subscription theft**: a way to read or register push subscriptions for a user
  other than the authenticated session's own.

Out of scope:

- Missing rate limiting on the login form (track this as a regular issue, not a security
  report, until there's a concrete abuse case).
- Denial of service via a self-hoster's own misconfiguration (e.g. exposing the generic
  webhook endpoint to the public internet with a weak/reused key is a deployment choice, not
  a vulnerability in Escalight itself — though we're glad to hear if there's a way to make
  that safer by default).

## Credits

Reporters who wish to be credited in a fix's release notes may say so in the private report;
otherwise reports are handled without attribution.
