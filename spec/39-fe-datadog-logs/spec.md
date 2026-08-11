# Spec — FE 39: Datadog Logs search

Date: 2026-08-10

## Background

FE 34 added CloudWatch Logs search for the same "investigate without
leaving the app" reason FE 32/33 added Parameter Store/Secrets Manager
browsing. Many teams also (or instead) ship logs to Datadog. This adds
an equivalent search view for Datadog Logs.

## Auth model (confirmed, differs from AWS)

Datadog has no SSO-token-exchange equivalent to AWS SSO — the browser
login (e.g. via Google) that reaches the Datadog web console is
unrelated to API access. The traditional method is a static **API Key +
Application Key** pair (Organization Settings → API Keys / Application
Keys) — but creating an *API key* specifically requires an org
permission the user doesn't have. What they *can* self-service create is
a **Personal Access Token** (Personal Settings → Access Tokens, scope
**`logs_read_data`** — the specific permission
`POST /api/v2/logs/events/search` checks), which Datadog's own docs
describe as the modern replacement for the key pair: a PAT authenticates
**alone**, via `Authorization: Bearer <token>` — no API key or
Application key involved at all. So: **cloudtui uses a Personal Access
Token, not the API/App key pair.** Simpler too (one credential instead
of two) and matches what the user can actually create without asking an
admin for anything.

A **site** (regional API host — `datadoghq.com`, `datadoghq.eu`,
`us3.datadoghq.com`, `us5.datadoghq.com`, `ap1.datadoghq.com`,
`ddog-gov.com`, ...) is still needed to know which host to call — the
same suffix as the web UI's `app.<site>` domain (e.g. the user's own
`app.datadoghq.eu` → API host `api.datadoghq.eu`).

There's nothing for cloudtui to "authenticate" interactively the way
FE 36 does for AWS SSO — the token is just a credential the user pastes
in once.

## Decisions (confirmed)

1. **New top-level app**, `datadog-logs`, alongside `cloudwatch-logs` in
   Home's "Apps" section. Single search view (query input + relative
   time-range preset, same 15m/1h/3h/24h cycle as FE 34 decision 4) →
   results table → detail view. No log-group-list step first — Datadog
   Logs is one flat, taggable/queryable stream, not organized into named
   groups the way CloudWatch is, so this is a shape *simpler* than FE 34
   (skips straight to the search screen).
2. **`POST /api/v2/logs/events/search`** (Datadog's Logs Search API) —
   `filter.query` (Datadog's log query syntax, passed straight through,
   no client-side reinterpretation — same principle as FE 34 decision 3
   for CloudWatch filter patterns), `filter.from`/`filter.to` (ISO-8601,
   the resolved time range), `sort: -timestamp` (newest first, matching
   FE 34 decision 6), `page.limit` (single page, capped — same
   "investigate not export" reasoning as FE 34 decision 5;
   more-results-available is surfaced in the title, not
   auto-paginated).
3. **No SDK dependency** — a small hand-rolled `net/http` client (like
   `internal/queue/jolokia`'s `Client`, not the AWS SDK pattern), since
   this is one JSON POST endpoint with one static bearer-token
   credential. The official `datadog-api-client-go` is a large, fully
   code-genned client covering Datadog's entire API surface — not
   justified for one endpoint, and against this project's "prefer the
   standard library where reasonable" default.
4. **Credentials stored the same way connection passwords already are**:
   new `DatadogConfig{Site, AccessToken}` on `config.Config` — a single
   token field, not a key pair — editable via a form (mirroring the
   existing connection editor's `AddPasswordField` for `Password`),
   saved to `config.yaml` (gitignored — this project's established
   convention already puts `Connection.Queue.Password`/`Proxy.Password`
   there, see `connEditorForm` in `internal/app/app.go`).
   `DD_ACCESS_TOKEN` env var is injected only when the config-file field
   is empty, mirroring `MQPROXY_CLIENT_PASSWORD`'s exact behavior in
   `config.Load`.
5. **`Site` is a plain text field, defaulting to `datadoghq.com`** (the
   most common US1 site) — rather than hardcode a picker list that will
   inevitably miss one, the user types theirs. The API host is
   `api.<site>`.
6. **A result opens a detail view** showing the full log message plus
   its attributes (service, host, status, tags, timestamp) — same
   detail-view precedent as FE 34's log event detail (`c` copies the
   message, no reveal-gating needed, nothing is masked).
7. **Read-only**: no log deletion/archival, no saved views/monitors, no
   writing.

## Proposed scope for this slice

- `tui/internal/datadoglogs`: `Search(ctx, cfg config.DatadogConfig,
  query string, from, to time.Time) ([]LogEvent, hasMore bool, error)`
  — thin wrapper posting to `https://api.<site>/api/v2/logs/events/search`
  with an `Authorization: Bearer <token>` header.
- `config.DatadogConfig{Site, AccessToken}` field on `Config`, `Load`
  env-var injection (`DD_ACCESS_TOKEN`), settings-screen editor form
  (new overlay, same shape as the connection editor).
- New search view: query input (server round-trip on Enter), time-range
  preset cycled by a key, results table (timestamp/service/status/message
  preview). `Enter` opens the detail view for a result.
- Registered as a real `ui.View`/`ui.Shortcuttable`, listed under Home's
  "Apps" section.

## Out of scope (this slice)

- Datadog APM traces, metrics, or dashboards — logs only.
- Saved views, log-based monitors/alerts.
- Live tailing (this is historical search, matching FE 34's same
  decision for CloudWatch).
- Multiple Datadog orgs/sites configured simultaneously — one
  `DatadogConfig` for the whole app, like `ActiveAWSProfile`.
- Free-form/absolute time ranges — only the relative presets, matching
  FE 34.
- OS keychain / any credential storage mechanism beyond what
  `Connection.Queue.Password` already establishes.
- Supporting the classic API Key + Application Key pair as an
  alternative to a Personal Access Token — PAT supersedes it for this
  use case (see "Auth model" above); add pair support later only if a
  concrete need for it shows up.
