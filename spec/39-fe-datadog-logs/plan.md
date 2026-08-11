# Plan — FE 39: Datadog Logs search

## Confirmed API shape

`POST https://api.<site>/api/v2/logs/events/search` (the same `<site>`
suffix as the web UI's `app.<site>` domain — e.g. `app.datadoghq.eu` →
API host `api.datadoghq.eu`).

Request:
```json
{
  "filter": { "query": "service:my-service status:error", "from": "2026-08-10T10:00:00Z", "to": "2026-08-10T14:00:00Z" },
  "sort": "-timestamp",
  "page": { "limit": 1000 }
}
```
Headers: `Authorization: Bearer <personal-access-token>`,
`Content-Type: application/json`. (Not `DD-API-KEY`/`DD-APPLICATION-KEY`
— see spec's "Auth model": the user can self-service create a Personal
Access Token but not an API key, and a PAT authenticates alone via the
`Authorization` header, per Datadog's docs.)

Response: `data[]`, each with `id` and `attributes.{timestamp, message,
status, service, host, tags[]}`; `meta.page.after` present ⇒ more
results exist (pagination cursor) — same "surface, don't auto-fetch"
treatment as FE 34's `NextToken`.

## `internal/config`

```go
type DatadogConfig struct {
    Site        string `yaml:"site"`
    AccessToken string `yaml:"accessToken"`
}
```
Added as `Datadog DatadogConfig` on `Config`. `Load()`: inject
`DD_ACCESS_TOKEN` env var only when the config-file field is empty —
same shape as the existing `MQPROXY_CLIENT_PASSWORD` block
(`config.go:322-337`), applied once (not per-connection — Datadog
config isn't per-connection). No default `Site` in `config.Default()`
(stays empty like `ActiveAWSProfile`); the package function defaults it
to `datadoghq.com` if unset, so an empty config still does something
reasonable.

**Status: implemented** (task 1) — `internal/config/config.go`,
`config_test.go`, `config.example.yaml` all updated and tested.

## `internal/datadoglogs` (new package)

`datadoglogs.go`: `LogEvent{Timestamp time.Time, Service, Status, Host,
Message string, Tags []string}`; a small `http.Client`-based helper,
package doc explaining the "hand-rolled, no SDK" choice from spec
decision 3.

`search.go`:
```go
func Search(ctx context.Context, cfg config.DatadogConfig, query string, from, to time.Time) ([]LogEvent, bool, error)
```
Builds the request, POSTs, decodes the response, returns
`buildLogEvents(raw)` — split out the same way `awslogs/filter.go` split
`buildLogEvents`, so the actual mapping logic is unit-testable without a
real HTTP call. Non-2xx response → error including status code and
response body (truncated) for a debuggable message, matching this
project's "errors wrapped with context" convention. `cfg.AccessToken`
empty → early error ("Datadog access token not configured — set it in
Settings or DD_ACCESS_TOKEN"), same shape as `awsssm.newClient`'s "no
profile selected" guard.

## `internal/app`

- `app.go`: new func field `searchDatadogLogs func(ctx, cfg
  config.DatadogConfig, query string, from, to time.Time) ([]LogEvent,
  bool, error)` → `datadoglogs.Search`, same DI pattern as
  `filterLogEvents`; Home "Apps" entry `datadog-logs`.
- `datadoglogs.go` (view): single screen combining the query input +
  time-range preset + results table — structurally `logSearchView`
  minus the log-group-selection step (no `Open(logGroupName)`, it's
  always "search everything" scoped by the query string itself, per
  spec decision 1). Reuses `cycleTimeRange`-equivalent key + the same
  results-table shape (timestamp/service/status/message preview).
- `datadoglogdetail.go`: mirrors `logdetail.go` — full message, all
  attributes rendered as label:value pairs (reusing the
  `accent`-colored label convention already used there), `c` copies the
  message via `App.copyToClipboard`.
- `settings.go` + `app.go`: 4th settings-list item, "Datadog:
  <site-or-'(none)'>", opens a new `datadogEditorForm` overlay —
  `AddInputField("Site", ...)` (plain, not masked — not a secret),
  `AddPasswordField("Access Token", ...)`, Save/Cancel — same shape and
  sizing approach as `connEditorForm` in `app.go:398-423` (just two
  fields instead of seven).

## Testing

- `internal/datadoglogs`: `buildLogEvents` table-driven unit tests (no
  network) — empty tags, missing optional fields, multiple events,
  timestamp parsing. `Search` tested against an `httptest.Server` stub
  (request shape assertions: `Authorization: Bearer <token>` header
  present, body fields correct; response parsing; non-2xx → error;
  empty access token → error without a request being made).
- `internal/config`: `Load()` test cases for `DD_ACCESS_TOKEN` injection
  (env wins only when field empty), alongside the existing
  `MQPROXY_CLIENT_PASSWORD` tests in `config_test.go`. **Done.**
- `internal/app`: view tests mirroring `logsearch_test.go`/
  `logdetail_test.go` conventions (fake `searchDatadogLogs`, no real
  HTTP), plus a settings-editor round-trip test (open → fill → save →
  `cfg.Datadog` reflects it) — `connections_test.go` only tests
  Escape/other-key handling for `connEditorForm`, not a save round-trip,
  so this is a new test shape, not a mirrored one.
- Manual verification (per `tui/CLAUDE.md`): with the user's real
  Personal Access Token and `datadoghq.eu` site entered via Settings,
  run the search `env:testt service:bar-proxy` and confirm the results
  match what their browser shows at the equivalent
  `app.datadoghq.eu/logs?query=...` URL for the same query/time range.
