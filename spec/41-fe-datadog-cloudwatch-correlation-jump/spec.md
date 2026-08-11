# Spec — FE 41: Jump from a Datadog log's CorrelationID to CloudWatch Logs

Date: 2026-08-10

## Background

FE 39 (Datadog Logs) and FE 34 (CloudWatch Logs) are both search-and-
detail views, built independently, with no cross-references between
them. In practice, a single request/operation often produces log lines
in both systems tagged with the same CorrelationID, and the natural
next step after finding one in Datadog is "show me the matching line in
CloudWatch."

## Decisions (confirmed)

1. **CorrelationID is embedded in the Datadog log message text**, not a
   tag or structured attribute — format confirmed from a real example:
   `CorrelationID: 1745d042-94e8-49f0-b223-8900ed9e951e` (label, colon,
   standard 36-char UUID). Extraction is a regex against
   `datadogLogDetailView.event.Message`, case-insensitive on the label,
   strict on the UUID shape (`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
   so it doesn't over-match trailing punctuation/words. **Found live:**
   the queued pattern must be double-quoted (`"<uuid>"`) before being
   handed to CloudWatch — its filter-pattern syntax otherwise tokenizes
   an unquoted term on the UUID's internal hyphens, so it never matches
   as the literal phrase it is. Scoped to this one programmatically-
   injected value; a user's own typed search pattern in the CloudWatch
   Logs search view is still passed straight through unmodified (FE 34
   decision 3 — no client-side reinterpretation of what the user types).
2. **No cross-log-group search.** CloudWatch Logs Insights (which could
   search all log groups at once) is explicitly out of scope per FE 34
   decision 3 — not revisited here. Instead, jumping lands on the
   CloudWatch Logs *group list* (already-existing `cloudwatch-logs`
   view) with the CorrelationID queued; picking a group (the existing
   Enter-on-a-row flow) opens that group's search **pre-filled** with
   the CorrelationID as the filter pattern, rather than the normal
   empty-pattern default.
3. **Trigger: `g` on the Datadog log detail view** (`datadogLogDetailView`),
   joining the existing `c`/`Esc` shortcuts. If the current event's
   message has no CorrelationID, `g` shows a status-bar message
   ("No CorrelationID found in this log message") and does nothing else
   — same "no-op with feedback" shape as other guard clauses in this
   app (e.g. "no AWS profile selected").
4. **The queued CorrelationID is one-shot and self-clearing** if
   abandoned: if the user jumps to the CloudWatch Logs group list via
   `g` but then navigates elsewhere (Home, Settings, another top-level
   view) without picking a group, the queued value is dropped rather
   than silently pre-filling an unrelated, later, manually-opened log
   group's search.

## Scope

- `internal/app/datadoglogdetail.go`: `extractCorrelationID(message
  string) (string, bool)` (pure regex helper, unit-testable); `g` key
  handler; `Shortcuts()` gains `{Key: "g", Description: "go to
  CloudWatch"}`.
- `internal/app/app.go`: new `App.pendingCloudWatchPattern string`
  field; `openLogSearch` consumes-and-clears it into the search view's
  initial pattern; `switchTo` clears it when navigating to any view
  other than `cloudwatch-logs` (the abandonment case from decision 4).
- `internal/app/logsearch.go`: `logSearchView.open(logGroupName,
  initialPattern string)` — signature change (single existing call
  site, in `openLogSearch`) to accept a non-empty starting pattern
  instead of always resetting to `""`.

## Out of scope

- Any change to Datadog's or CloudWatch's search APIs themselves.
- Auto-selecting/remembering "the" log group for a service — the user
  picks a group each time, per the confirmed decision.
- Matching on anything other than the exact `CorrelationID: <uuid>`
  text shape — no fuzzy matching, no support for differently-labeled
  correlation fields.
- The reverse direction (CloudWatch → Datadog) — not requested.
