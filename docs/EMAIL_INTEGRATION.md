# Email-to-alert integration

Escalight does not run its own SMTP server — receiving raw email safely (SPF/DKIM
checks, spam filtering, attachment handling) is a job better left to an existing inbound-email
provider. Instead, `/webhooks/email-in/<service-webhook-key>` accepts a small JSON payload and
turns it into an incident; you point your provider's own inbound webhook at that URL.

## Payload shape

```json
{
  "from": "alerts@example.com",
  "subject": "Disk space critical on db-1",
  "text": "/var is at 98% capacity"
}
```

Escalight also accepts Postmark's native inbound-webhook field names (`From`, `Subject`,
`TextBody`) unchanged, so a Postmark inbound stream can point straight at the URL with no
transformation.

## Postmark

1. Create an **Inbound** server stream in Postmark.
2. Set its webhook URL to `https://<your-escalight-host>/webhooks/email-in/<webhook-key>`.
3. Done — Postmark's payload already matches the field names Escalight looks for.

## Mailgun

1. Set up an inbound route in Mailgun for the address you want to page from.
2. Point the route's `forward()` action at
   `https://<your-escalight-host>/webhooks/email-in/<webhook-key>`.
3. Mailgun's default inbound payload uses different field names (`sender`, `subject`,
   `body-plain`) — use Mailgun's "store and notify" template, or add a small transform, to
   post `{"from": ..., "subject": ..., "text": ...}` instead. A follow-up release may add
   native Mailgun field support if this comes up in practice — please open an issue.

## SendGrid Inbound Parse

Same idea: configure Inbound Parse to POST to the URL above, mapping SendGrid's `from`,
`subject`, and `text` fields (SendGrid's own field names already match).

## Deduplication

Incidents from this adapter are deduplicated on the email subject: three emails with the
same subject line update the same open incident's timeline instead of opening three separate
incidents, since a mail thread commonly shares one subject across replies.
