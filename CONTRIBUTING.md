# Contributing to Escalight

Thank you for your interest in contributing. Escalight is a self-hosted on-call escalation
and paging tool, open source under the MIT license.

## Getting started

```bash
git clone https://github.com/<your-username>/escalight.git   # your fork, see below
cd escalight
go mod download
make build
make test
```

## Requirements

- Go 1.26+ (see `go.mod`)
- No database server, no Node.js, no other services required — Escalight uses an embedded
  pure-Go SQLite driver and a hand-written CSS file, so `go build` is the entire build.

## Contribution workflow

The `main` branch is protected: every change lands through a pull request, required status
checks must pass, and protection is enforced for everyone — including the maintainer. There
are no direct pushes to `main`.

1. Fork the repo on GitHub, then clone your fork (command above).
2. Create a descriptively named feature branch from `main`.
3. Make your changes as small, focused commits, each leaving the tree buildable.
4. Run `make lint` and `make test` — both must pass.
5. If your change is user-facing (a feature, fix, or behavior change), add one bullet under
   the `Unreleased` heading in [CHANGELOG.md](CHANGELOG.md).
6. Push the branch to your fork.
7. Open a pull request against `main` here.

A PR can merge only when every required check passes (`Test`, `Lint`, `Build`) and all conversation
threads are resolved.

### Manual testing

`make test` runs the full suite against an in-memory SQLite database, so it needs no external
services. To exercise the running app end to end:

```bash
go run . serve
```

Then visit `http://localhost:8080` — the first run redirects you to a setup page to create an
admin account. To fire a synthetic alert once you've created a service:

```bash
curl -X POST http://localhost:8080/webhooks/generic/<service-webhook-key> \
  -H 'Content-Type: application/json' \
  -d '{"title":"test alert"}'
```

## Releases

Releases are cut by pushing a tag; GitHub Actions does the rest (`.github/workflows/release.yml`):

1. Make sure every user-facing change since the last release has a bullet under `Unreleased`
   in [CHANGELOG.md](CHANGELOG.md) (step 5 of the workflow above).
2. Give the release its own changelog section: insert `## [x.y.z] - YYYY-MM-DD` above the
   (now empty) `## [Unreleased]` heading, following the format of the existing sections, and
   update the compare links at the bottom of the file — add
   `[x.y.z]: https://github.com/Laaaaksh/escalight/compare/v<prev>...vx.y.z` and repoint
   `[Unreleased]` at `compare/vx.y.z...HEAD`.
3. Land those changelog edits on `main` through a pull request (see the contribution workflow
   above), then tag and push:

   ```bash
   git tag vx.y.z && git push origin vx.y.z
   ```

The release workflow builds and publishes binaries for Linux/macOS/Windows and a multi-arch
Docker image, using the tagged version's CHANGELOG section as the release notes — a tag with
no changelog entry fails the release rather than publishing empty notes.

## Code style

- Standard `gofmt` formatting (enforced by CI).
- Package layout: `internal/db` (schema + queries, one file per entity), `internal/engine`
  (escalation + schedule-rotation logic — no HTTP or DB-schema concerns), `internal/notify`
  (email/Slack/Discord/web-push senders), `internal/webhooks` (alert ingestion adapters),
  `internal/httpserver` (routes, handlers, templates). Keep business logic (what should
  happen) out of `internal/httpserver` handlers — they should read as: parse the request,
  call one or two `db`/`engine`/`notify` calls, render or redirect.
- No ORM: `internal/db` uses hand-written SQL via `database/sql`. Keep queries parameterized
  (`?` placeholders) — never string-format a value into SQL.
- Templates live in `internal/httpserver/templates/`; each page file `{{define "content"}}`s
  a block that `layout.html` wraps. See `render.go` for how the set is assembled — a new page
  template needs adding to `pageNames` in `render.go`.
- New database migrations are additive SQL files in `internal/db/migrations/`, numbered
  `NNNN_description.sql`, applied in order once and never edited after landing on `main`.
- Comments explain *why*, not *what* — see the top-level CLAUDE.md-style guidance in this
  repo's code for the house style if you're unsure.

## Reporting issues

Please open a GitHub issue before starting large changes or proposing new features, so scope
and approach can be settled before code is written. Bug reports should include:
- Escalight version (`escalight version`)
- How you deployed it (Docker, systemd, `go run`, etc.)
- Steps to reproduce
- What you expected vs what happened
