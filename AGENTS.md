# Project agent memory

This file is the project's committed home for project-intrinsic agent knowledge: build, test, release, architecture, and sharp-edge notes that should travel with the code.

## What this is

Escalight is a self-hosted PagerDuty/Opsgenie replacement: escalation policies, on-call
schedules, alert ingestion (generic webhook, Alertmanager, email-in), and paging over
email/Slack/Discord/web push. Single Go binary, single SQLite file, no other services. See
README.md for the product pitch and CONTRIBUTING.md for the build/test/release workflow —
this file only holds things not already obvious from reading those or the code.

## Architecture

- `internal/db` — schema (`migrations/*.sql`, additive-only, numbered) + hand-written SQL
  queries via `database/sql`, no ORM. One file per entity.
- `internal/engine` — pure business logic, no DB/HTTP imports beyond `internal/db` types:
  schedule rotation math (`schedule.go`) and the escalation ticker (`escalation.go`, polls
  every 15s for incidents whose `next_escalation_at` has passed).
- `internal/notify` — one sender per channel (email/Slack/Discord/webpush) plus a
  `Dispatcher` that logs every attempt as an `incident_events` row, success or failure.
- `internal/webhooks` — turns an inbound alert into an incident via `Ingestor.Ingest`,
  shared by the generic/Alertmanager/email-in HTTP handlers.
- `internal/httpserver` — chi router, `html/template` pages (each page file
  `{{define "content"}}`s into `layout.html`; register new pages in `render.go`'s
  `pageNames`), plus the JSON endpoints for push and Slack.

## Sharp edges

- **SQLite only, deliberately** — no Postgres support in v1. `modernc.org/sqlite` is
  pure-Go (no cgo), which is why cross-compiling in `.goreleaser.yml` needs no C toolchain
  and the Docker image can be `distroless/static`. Don't reach for cgo-sqlite or add a second
  DB backend without discussing the tradeoff — one well-tested backend beat two thin ones for
  v1.
- **All schedule/override/rotation times are UTC**, not per-schedule timezone. The `Timezone`
  column on `schedules` is stored but not yet used for conversion — this is a known, documented
  v1 limitation (see README Configuration section), not a bug.
- **`internal/httpserver/static/htmx.min.js` is vendored, not fetched from a CDN** — the
  dashboard needs to work with zero external network dependency at request time. If you bump
  htmx, re-download it and update `docs/THIRD_PARTY_NOTICES.md`'s version note.
- **Session cookie auth only** (no OAuth) — this was a deliberate v1 cut to protect time spent
  on the escalation engine and notification channels, not an oversight.
- Demo screenshots/GIF in `docs/assets/` were captured from the real running app via headless
  Chrome (chromedp) driving actual HTTP requests and clicks — see the PR that introduced them
  for the capture script if you need to regenerate them after a UI change.
- **Default branch is `master`**, not `main` — `.github/workflows/*` target `master`; if you
  add a workflow or see one referencing `main`, that's a bug, not a style choice.
- As of PR #7, the GitHub account hosting this repo has a billing/payment block that fails
  every Actions job before a runner is even assigned (0 billed minutes, check-run annotation
  cites "recent account payments have failed or your spending limit needs to be increased").
  This blocks CI and the tag-triggered release workflow account-wide, independent of any code
  change here — verify with `gh api repos/<owner>/<repo>/actions/runs/<id>/jobs` before
  assuming a red check is a real test failure. Also, the repo is currently **private**, so the
  Docker/release-binary README install paths intentionally point at "from source" until a
  release actually ships — don't restore them without confirming a release exists.

## Maintaining this file

Keep this file for knowledge useful to almost every future agent session in this project.
Do not repeat what the codebase already shows; point to the authoritative file or command instead.
Prefer rewriting or pruning existing entries over appending new ones.
When updating this file, preserve this bar for all agents and keep entries concise.
