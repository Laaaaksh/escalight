# Third-party notices

Escalight vendors one third-party asset directly into the binary (`internal/httpserver/static/htmx.min.js`)
so the dashboard works offline with no CDN dependency at request time:

- **[htmx](https://htmx.org)** v2.0.4 — Zero-Clause BSD license. Copyright (c) 2020 Big Sky Software.

All other dependencies are pulled via `go.mod` at build time; see that file and `go.sum` for
the full list and their respective licenses.
