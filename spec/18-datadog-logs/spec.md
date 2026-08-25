# Datadog Logs search

_Condensed from spec/39, spec/40, spec/42, spec/52 — see those folders for the incremental history. Time-range UI superseded by spec/53 — see spec/19-log-investigation-crosslinks for the current shared time-range modal._

## Purpose

Search Datadog Logs from inside cloudtui, mirroring CloudWatch Logs
(spec/17) for teams that ship logs to Datadog instead of (or as
well as) AWS.

## Auth model (differs from AWS)

Datadog has no SSO-token-exchange equivalent. cloudtui authenticates with
a **Personal Access Token** (Datadog: Personal Settings → Access Tokens,
scope `logs_read_data`) sent as `Authorization: Bearer <token>` — **not**
the classic API-Key + Application-Key pair (creating an API key requires
an org permission most users don't have; a PAT is Datadog's own documented
modern replacement and is self-serviceable). A **site** (regional API
host, e.g. `datadoghq.com`, `datadoghq.eu`, `us3.datadoghq.com`) selects
which host to call — same suffix as the web UI's `app.<site>` domain. The
token is a static credential the user pastes in once; there's no
interactive re-auth flow the way AWS SSO has (spec/14).

## Behavior / user flow

- A top-level app (`datadog-logs`), its own `ui.View`, listed in Home's
  own "Datadog" section — not grouped with `cloudwatch-logs`, which is
  under "AWS" (see spec/05 for the current section grouping). **Single
  search view** — no log-group-list step first,
  since Datadog Logs is one flat, taggable/queryable stream, not
  organized into named groups: query input → results table → detail view.
- Query is Datadog's own query syntax (`filter.query`), passed straight
  through with no client-side reinterpretation, combined server-side with
  the Service/Env filters (see below) as `service:"<val>" env:"<val>"
  <free-text query>`. Runs against `POST /api/v2/logs/events/search`,
  `sort: -timestamp` (newest first), a single capped page (`page.limit`,
  same "investigate not export" reasoning as CloudWatch) — more-results
  available is surfaced in the title, not auto-paginated.
- **Service / Env filter dropdowns** sit in a row between the results
  table and the query input. `S` focuses the Service dropdown, `E`
  focuses the Env dropdown (uppercase to avoid colliding with lowercase
  global hotkeys). Selecting a value re-searches immediately (no confirm
  step, matching the theme picker's immediate-apply convention). Both
  dropdowns' options are the union of:
  - **Accumulated values** (`knownServices`/`knownEnvs`) — every distinct
    Service/Env value seen across searches *this session*, merged into a
    running set (not replaced per-search — replacing would shrink the
    list to just the current filter's matches once a facet is active).
  - **A facet-listing baseline** — on every view `Activate()`, a
    background call to Datadog's Aggregate API (`POST
    /api/v2/logs/analytics/aggregate`, `group_by` on the `service`/`env`
    facet, `limit: 1000`, sorted by count desc) over a fixed, independent
    **30-day discovery window** (not the user's selected search time
    range), `filter.query: "*"`. This surfaces values that exist in
    Datadog but haven't appeared in anything searched yet this session
    (e.g. an infrequently-logging service outside the widest search
    preset). Fails soft: an error here is logged and ignored, never shown
    as a banner or blocking the view — it's a convenience/completeness
    enhancement, not core search.
  - Both lists are prefixed with an `(any)` option that clears that facet.
  - After every search, if the currently-selected Service/Env value is no
    longer present among the new result set's distinct values, the
    filter resets to `(any)` rather than silently continuing to claim a
    filter that isn't actually being applied to what's displayed.
  - Filter values are double-quoted in the constructed query (same
    reason as the CorrelationID quoting in spec/19 — unquoted
    punctuation would otherwise be tokenized by Datadog's query parser).
- Detail view shows the full log message plus attributes (Service,
  Status, Host, Env, Tags, timestamp). `c` copies the message to the
  clipboard (nothing masked, no reveal-gating).
- **Datadog settings editor** (`datadogEditor` overlay, Settings screen):
  form for `Site` (plain text, default `datadoghq.com`) and `Access
  Token`, mirroring the connection editor's password-field pattern. Fully
  exempted from the app's global-hotkey handling while open (see gotcha
  below) — every keystroke, including letters that would otherwise be
  global hotkeys, must reach the form fields.
- The results table supports `w` (spec-wip/92): a per-session (not
  persisted) word-wrap toggle on the Message column, off by default —
  same shape as CloudWatch Logs search's toggle (spec/17), sharing the
  same underlying helpers (`tui/internal/view/wraptext.go`). Off, the
  Message column shows `logEventPreview`'s compact single-line summary
  (first line only, capped at 200 chars). On, wrapping operates on the
  raw, un-truncated event message — a multi-line event wraps
  line-by-line into non-selectable continuation rows, each of its own
  lines independently word-wrapped, up to `maxWrapLines` (50) before a
  `"… N more line(s)"` indicator takes over. The wrap width itself is
  computed from the table's actual current rendered width, not a fixed
  number — see `dynamicWrapWidth`'s doc comment / spec/08's
  Implementation notes for why a fixed width didn't hold up live. The
  detail view remains the only place with no line cap at all. Only the
  results table — the Service/Env filter dropdowns keep their own
  navigation, unaffected.
- Column widths aren't equal: **Service** is capped at 20 characters
  (`…` if longer) and gets no extra width on a wider terminal;
  **Message** gets by far the largest share of any extra space — found
  live (same issue as spec/08's message browser and spec/17's search
  screen): with every column claiming equal weight, Message was
  consistently the most cramped, especially with 3 other columns
  (Timestamp, Service, Status) competing for the same space.
- Read-only: no log deletion/archival, no saved views/monitors, no
  writing.

## Data & config

- `config.DatadogConfig{Site, AccessToken}` on `Config` — one token field,
  not a key pair; one config for the whole app (no multi-org/site
  support). `DD_ACCESS_TOKEN` env var is injected only when the
  config-file field is empty, mirroring `MQPROXY_CLIENT_PASSWORD`'s
  behavior in `config.Load`. Stored in `config.yaml` (gitignored), same
  convention as connection passwords.
- `tui/internal/datadoglogs/`:
  - `Search(ctx, cfg config.DatadogConfig, query string, from, to
    time.Time) ([]LogEvent, hasMore bool, error)` — posts to
    `https://api.<site>/api/v2/logs/events/search`.
  - `ListFacetValues(ctx, cfg config.DatadogConfig, facet string, from,
    to time.Time) ([]string, error)` — one generic function, called once
    per facet (`"service"`, `"env"`) rather than separate
    `ListServices`/`ListEnvs` functions. Posts to
    `https://api.<site>/api/v2/logs/analytics/aggregate`. Response shape
    is `buckets[].by.<facet>`.
  - `LogEvent.Env` field populated by `extractEnv(tags []string) string`
    — Datadog has no top-level `env` attribute; it's conventionally an
    `env:<value>` tag, so `Env` is extracted from the tags array (`Host`
    stays a real top-level field, shown in the detail view, but is **not**
    a filter facet — an early design pivoted the second filter from Host
    to Env as more useful in practice).
- No AWS-SDK-style dependency: a small hand-rolled `net/http` client (like
  `internal/queue/jolokia`), not the large code-genned
  `datadog-api-client-go` — not justified for two JSON POST endpoints with
  one static bearer token.

## Implementation notes

- `tui/internal/view/datadoglogs.go` (`DatadogLogsView`), `datadoglogdetail.go`
  (`DatadogLogDetailView`). Originally lived under `internal/app`; moved
  as part of the package split (spec/03).
- `tview.DropDown`s (`serviceFilterDD`, `envFilterDD`) require
  `styleDropDown` for their unselected popup-list items to be readable
  against the theme — the same gotcha hit for the theme/connection-editor
  dropdowns.

## Notable gotchas worth preserving

- **Global-hotkey leak**: any new full-page overlay/editor must be added
  to the app's overlay-visibility exemption set (`overlayVisible` — a
  slice of `*bool` built once in `New()`, looped over by a single
  `anyOverlayVisible()` helper) or every keystroke typed into its form
  falls through to the global hotkey switch — `q` quits the whole app
  mid-edit, other letters navigate away. This bit the Datadog settings
  editor specifically (its `datadogEditorVisible` flag was omitted when
  first added) and is a recurring class of bug worth checking for any new
  overlay.
- Facet-listing's response shape (`buckets[].by.<facet>`) was confirmed
  against a real API call, not fully derivable from Datadog's public
  docs alone — worth re-verifying live if this integration is rebuilt.
