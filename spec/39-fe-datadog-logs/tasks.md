# Tasks — FE 39: Datadog Logs search

1. [x] `internal/config`: add `DatadogConfig{Site, AccessToken}` +
   `Datadog DatadogConfig` field on `Config`; `Load()` injects
   `DD_ACCESS_TOKEN` only when `Datadog.AccessToken` is empty (mirroring
   the existing `MQPROXY_CLIENT_PASSWORD` block). Updated
   `config.example.yaml` to document the new section (commented out,
   like the existing `activeAWSProfile` example). Unit tests in
   `config_test.go` alongside the existing env-injection tests.
2. [x] New package `internal/datadoglogs`: `LogEvent` type,
   `Search(ctx, cfg, query string, from, to time.Time) ([]LogEvent, bool, error)`
   posting to `https://api.<site>/api/v2/logs/events/search` with
   `Authorization: Bearer <token>`; `buildLogEvents` split out for
   network-free unit testing. Unit tests: `buildLogEvents` table-driven
   cases, `Search` against an `httptest.Server` (request shape, response
   parsing, non-2xx error, empty-token early error).
3. [x] `internal/app`: `App.searchDatadogLogs` func field wired to
   `datadoglogs.Search` (same DI pattern as `filterLogEvents`); register
   `datadog-logs` as a `ui.View` in Home's "Apps" section.
4. [x] Search view (`internal/app/datadoglogs.go`): query input +
   time-range preset (reusing the CloudWatch Logs search view's
   preset-cycling shape) + results table
   (timestamp/service/status/message preview); `Enter` opens the detail
   view. Unit tests mirroring `logsearch_test.go`'s conventions (fake
   `searchDatadogLogs`, no real HTTP).
5. [x] Detail view (`internal/app/datadoglogdetail.go`): full message +
   attributes (service/host/status/tags/timestamp) as label:value pairs;
   `c` copies the message via `App.copyToClipboard`; `Esc` returns to
   search. Unit tests mirroring `logdetail_test.go`.
6. [x] Settings: 4th `settingsList` item "Datadog: <site-or-'(none)'>"
   opening a new `datadogEditorForm` overlay (`Site` plain input,
   `Access Token` `AddPasswordField`, Save/Cancel) — same shape as
   `connEditorForm`. Unit tests: open/prefill, save round-trip
   (`cfg.Datadog` reflects form values), Escape closes without saving.
7. [ ] Manual verification (per `tui/CLAUDE.md`): create a Personal
   Access Token in Datadog (Personal Settings → Access Tokens, scope
   `logs_read_data`), enter it + site `datadoghq.eu` via Settings, run
   the search `env:testt service:fibuproxy`, confirm results match the
   Datadog web UI for the same query/time range. Record what was
   checked here once done.
