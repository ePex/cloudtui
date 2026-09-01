# AMQ web console: static browser UI for mq-proxy

Date: 2026-09-01

## Purpose

Give non-technical users (support/ops staff without dev tooling or the TUI
installed) a way to browse and operate on ActiveMQ queues from an ordinary
web browser, using the same `mq-proxy` REST API (spec/10, spec/11) that the
TUI's proxy backend already talks to. This is a companion surface to the
TUI, not a replacement — it targets the same operational tasks (spec/09)
through a click-first UI instead of a keyboard-driven one.

## Scope

- A new, standalone static web bundle (plain HTML/CSS/JS, no build-time
  framework decided yet — that's a `plan.md` question) deployed
  independently of `mq-proxy` itself (e.g. served from S3/nginx/wherever),
  not bundled into the `mq-proxy` JAR. It has no server-side component of
  its own — every operation is a direct browser call to a running
  `mq-proxy` instance's REST API.
- **Dark mode**: follows the OS/browser's `prefers-color-scheme`
  automatically — no manual toggle, no settings/theming system. CSS
  custom properties swap to a dark palette under a media query;
  `color-scheme: light dark` on `:root` so native form controls
  (inputs/textareas) adapt too, and so the browser doesn't force-invert
  the page on its own (which otherwise renders worse than either palette
  on its own — confirmed live, some browsers do this to any page that
  only declares light support).
- **Connecting**: on load, a form asks for the `mq-proxy` base URL plus its
  HTTP Basic username/password (the single `proxy.auth.username`/
  `proxy.auth.password` pair configured on that `mq-proxy` instance —
  spec/10; this is not per-broker JMS credentials, there's only one to
  enter). Remembered in browser `localStorage` across visits, so the user
  doesn't re-enter it every time — a deliberate convenience-over-security
  trade-off (see "Security considerations" below).
- **Queue list**: name, pending/consumer/enqueue/dequeue/producer counts
  (the fields `list-queues` already returns — spec/11). A live
  substring filter narrows by queue name (client-side — `list-queues`
  has no server-side filter param), and clicking a column header sorts
  by that column (ascending, click again for descending), all
  client-side over the already-fetched list.
- **Message browsing**: list + detail view for a queue's messages
  (conceptually mirrors spec/08), calling `list-messages` with
  `returnBody=true`. Since `mq-proxy` requires a positive `filter.maxCount`
  on every `list-messages` call, the page applies a sane client-side
  default (matching the TUI's 500) when the user hasn't set one, and shows
  the effective cap in use — same reasoning as spec/11's client already
  applies.
- **Actions — full parity with spec/09**:
  - Purge a queue (confirm dialog, optional JMS Type filter, with
    autocomplete — see below).
  - Delete a single message.
  - Move a single message (target-queue picker).
  - Move all messages / drain a queue (optional JMS Type filter, same
    autocomplete).
  - Send a new message: body plus an optional JMS Type field (unlike
    spec/09's TUI, which has no JMS Type field and hardcodes `"text"` —
    this page exposes one since `mq-proxy`'s `send-message` DTO requires
    it, and it's a normal, common thing to want to set; a blank entry
    still defaults to `"text"`). No headers/properties/templates.
- **JMS Type autocomplete** (purge and move-all, which share one prompt
  component): the moment the prompt opens, it scans the queue in the
  background (`list-messages`, `returnBody=false`, capped at 500 —
  mirroring the TUI's own automatic-scan cap, spec/08) and populates a
  native `<datalist>` with the distinct JMS Types found. The scan never
  blocks or gates Continue/Cancel — a blank field still proceeds
  immediately regardless of whether the scan has finished, and a failed
  scan just leaves the suggestion list empty rather than surfacing an
  error (a nice-to-have, not a required step). Unlike the TUI, there's
  no second opt-in "scan more" tier — a single fixed-cap automatic scan
  was judged sufficient for this simpler surface.
- Errors (auth failure, network failure, mq-proxy validation errors) are
  surfaced inline in the UI, not just logged to the browser console —
  the audience is explicitly non-technical.

## Out of scope (deliberate, for this first iteration)

- No Jolokia backend support — this page only ever talks to `mq-proxy`,
  never directly to a broker's Jolokia endpoint.
- No multiple saved/named connections or switching between them (spec/12
  is a TUI-only concept for now) — one active `mq-proxy` connection at a
  time per browser.
- No login system of its own, no per-user roles or audit trail beyond
  whatever `mq-proxy`'s single shared Basic-auth pair provides.
- No pagination/"load more" for messages — mirrors `mq-proxy`'s own
  stated out-of-scope (spec/11).
- No keyboard-driven power-user UX, no theme picker/settings — click-first,
  matching the target audience; not trying to be a lightweight TUI clone.
  (Light/dark *does* follow the OS preference automatically — see the
  connect/queues/messages bullets above — that's not a theming system,
  just standard `prefers-color-scheme` support.)
- No offline/PWA support.

## `mq-proxy` CORS change (part of this same PR)

`mq-proxy` (spec/10) currently has no CORS configuration — Spring Security
is set up for Basic auth + disabled CSRF only. The web console must work
both when opened directly via `file://` (double-click, no server at all)
and when served from a normal `http(s)://` origin — so `mq-proxy` needs a
`CorsConfigurationSource` bean that:

- Allows the `null` origin (what browsers send for `file://` pages) —
  this is what makes the file:// case work at all.
- Allows a configurable list of `http(s)://` origins for served
  deployments, rather than a hardcoded value — likely a new
  `application.yml` key (env-overridable, matching the existing
  `BROKER_URL`-style pattern) rather than wide-open `*`, since `*` is
  incompatible with allowing credentialed requests in most browsers and
  this API requires an `Authorization` header on every call.
- Allows the `Authorization` header and the HTTP methods this API
  actually uses (GET/POST) — `Authorization` is a non-simple header, so
  these requests trigger a CORS preflight (`OPTIONS`), which also needs
  to be permitted through Spring Security's chain.

This is a change to an already-shipped service, folded into this same
`spec-wip` folder rather than split into its own `cr-mq-proxy-cors` — one
PR spans both the `mq-proxy/` CORS config and the new static web bundle.

## Security considerations (for the record, not a decision point)

HTTP Basic credentials held in `localStorage` and resent on every request
means they sit in the browser (readable by any script that can run on that
origin, e.g. via a future XSS bug in this same page) and travel as
base64 — not encrypted — on the wire. This makes serving `mq-proxy` over
HTTPS effectively mandatory for any real deployment of this page, though
enforcing that is outside what the static page itself can control. Worth
a callout in the page's own UI (e.g. a warning if the entered URL is
`http://`) — a `plan.md`-level detail, noted here since it's a direct
consequence of the credentials decision above.

## Data & config

No changes to `tui/` or its config. New top-level location TBD in
`plan.md` (something like `mq-proxy-web/` at the repo root, sibling to
`tui/` and `mq-proxy/`).
