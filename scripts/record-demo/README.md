# Demo recorder

Records a genuine, re-runnable capture of Escalight's escalation loop for
`docs/assets/demo.mp4` / `docs/assets/demo.gif`, used in the README's Demo
section.

It is real end-to-end automation, not a mockup: it builds the actual
`escalight` binary, boots it against a throwaway SQLite file and a minimal
local SMTP relay (`fake-smtp.js`, just enough of the protocol for
`internal/notify`'s real email client to complete a genuine send), then drives
the real web UI with Playwright — creating a user, an on-call schedule with a
rotation, and a two-level escalation policy, firing a real incident at the
service's ingest webhook, and watching it page the primary on-call, escalate
to the secondary responder when unacknowledged, then get acknowledged and
resolved.

This package is dev-only tooling. It is not part of the Escalight build and
has its own `package.json` so `go build` never touches it.

## Usage

From the repo root:

```bash
make demo
```

Or manually:

```bash
cd scripts/record-demo
npm install
npx playwright install chromium   # only needed once
npm run record
```

This builds `escalight`, boots it on `:8080` (the port must be free), records
a video with Playwright, then uses `ffmpeg` (must be on `PATH`) to produce
`docs/assets/demo.mp4` and `docs/assets/demo.gif`. Re-running produces the
same walkthrough against a freshly seeded database each time.

## Why the fake SMTP relay

Escalight's email notifications need an SMTP relay configured
(`ESCALIGHT_SMTP_HOST`) or `internal/notify`'s real client logs a genuine
`notify_failed` event instead of `notified` — accurate, but a worse demo.
`fake-smtp.js` is a real (if minimal) SMTP server on `127.0.0.1:1025` that the
app's real SMTP client talks to over the real network stack, so the recorded
"notified" events reflect an actual successful send, not a fabricated one.

## Troubleshooting

- **Port 8080 already in use**: stop whatever is bound to it, or wait — this
  script does not change the app's default port to work around it.
- **`demo.gif` over 10 MB**: lower `fps` or shorten pauses in `record.js`
  rather than changing the width past 960px.
