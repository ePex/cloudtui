# Debug logging

_Condensed from spec/03-fe-logging — see that folder for the incremental history._

## Purpose

A persistent, inspectable log so startup problems are diagnosable and the app's runtime behavior is observable without attaching a debugger.

## Behavior

### Log file

On startup, the app initializes a `log/slog` text-format logger writing to `~/.cloudtui/cloudtui.log`. The directory and file are created if absent; the file is **truncated on each startup** so it never grows unboundedly (no rotation beyond that). `log/slog` is Go standard library (Go 1.21+) — no external dependency.

What is logged at startup: the config file path that was loaded, and the active theme. (Runtime logging beyond startup is added incrementally per-feature as needed, not blanket-covered by this spec.)

### Queue backend requests

Every HTTP request the queue backends make — Jolokia (spec/07) and mq-proxy (spec/11), for every operation, including mq-proxy's warm-up retries — gets one line once the response has been read (or the request failed), with the message `queue backend request` and:

- `backend` (`jolokia` or `proxy`) and `connection` (the connection's name),
- `method` and `url` (any user name/password in the URL removed),
- `op`, the operation:
  - Jolokia: the request type and target, the target being the queue named by the MBean's `destinationName` (else the whole MBean), with an operation's signature dropped — `exec removeMessage on orders`, `read QueueSize on orders`; the queue search is `search`; a bulk request is summarized as `bulk: 672 × read QueueSize/ConsumerCount/EnqueueCount/DequeueCount`. Operation arguments (message IDs, bodies, headers) are never decoded.
  - mq-proxy: the URL path (e.g. `/api/management/command/list-messages`); the query string (queue name, filter) is in `url`.
- `status` (HTTP), and for Jolokia `jolokiaStatus`, the status inside its JSON reply (Jolokia answers HTTP 200 even when the operation failed): `200`, or per status for a bulk reply (`200×46, 404×2`),
- `duration` (including reading the body) and `bytes` (response size).

Levels: `INFO` normally; `WARN` (with `error`) when there's no response, the HTTP status isn't 2xx, or a Jolokia status isn't 200; a request cancelled by the user (Esc, spec/24) is `INFO` with `result=cancelled`. Two expected broker fallbacks therefore leave a `WARN` line followed by the fallback's `INFO` line, deliberately kept so the log shows exactly what the broker answered: `browseMessages` → `browse()` (spec/07) and `purgeQueue` → `removeMatchingMessages` (spec/09).

**Never logged:** request or response bodies, headers (including `Authorization` with the broker password) and passwords in any form, so the log stays safe to attach to a ticket. Datadog and AWS SDK requests aren't logged this way.

### `:log` view

A read-only, scrollable text view that reads and displays `~/.cloudtui/cloudtui.log`. It is a standard registered view: reachable from Home's "System" section and via `:log`, with `l` as its global hotkey. `Activate()` reloads the file every time the view becomes active; `r` manually refreshes. If the log file is absent, a short fallback message is shown instead of an error.

Lines are colorized by slog level: `level=ERROR` red, `level=WARN` yellow, `level=INFO` aqua/cyan (tcell has no separate "cyan" tag name; aqua is the same `0x00FFFF` value). Unrecognized lines (e.g. `DEBUG`, or a wrapped continuation line) keep the default text color. Each line is passed through `tview.Escape` before rendering, so a literal `[` in logged content (e.g. a Go slice's `%v` formatting) is never misread as a `tview` color tag.

## Data & config

- Log file path: `~/.cloudtui/cloudtui.log` (directory `~/.cloudtui/` created if missing).
- `.gitignore` excludes the log file/directory — it must never be committed.

## Implementation notes

- Logger initialization lives in `cmd/cloudtui/main.go`, before `app.New()` is called (so applying the theme and building the shell are themselves covered by the startup log entries).
- `tui/internal/view/log.go` — `LogView`, implementing `ui.View`, `ui.Shortcuttable`, and an `activatable` interface (`Activate()` reload-on-entry). Originally lived at `internal/app/log.go`; moved into `internal/view` as part of the later package split (see spec/03-architecture-and-package-layout).
- Global hotkey `l` and the `l: Log` status-bar/help-modal entries are wired in the shell composition root (`internal/app/app.go`) and the help modal.
- Queue backend requests: `tui/internal/queue/httplog` — `Transport`, an `http.RoundTripper` wrapping the client's transport, so every call site is covered without touching it and each retry is its own line. It names the operation through the backend's `Describe` func before sending (reading a copy of the request body via `httplog.RequestBody`/`req.GetBody`, never `req.Body`), reads the whole response body once (size, duration, and the bytes for the backend's `AppStatus`), and hands the client an identical body (a read error is passed on after the bytes read). Logs through `slog.Default()` at request time. The backend parts are `jolokia/httplog.go` (`describeRequest`, `appStatus`, decoding only `type`/`mbean`/`operation`/`attribute` and `status`) and `proxy/httplog.go` (the path); each client's `WithRequestLog(connection)` installs the transport, and `secretbackend.buildBackend` calls it with `conn.Name`. `cmd/seedqueue` doesn't, so the dev tool's output is unchanged.

## Notable gotchas worth preserving

- Always run log lines through `tview.Escape` before display — unescaped `[...]`-shaped content in a logged value (a Go slice, a JSON array literal, etc.) will otherwise be interpreted as a tview color/region tag and corrupt the rendered line.
