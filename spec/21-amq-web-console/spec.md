# AMQ web console

_Condensed from spec-wip/fe-amq-web-console — see that PR for the incremental history and rationale._

## Purpose

A standalone, static browser page for non-technical users (support/ops
staff without dev tooling or the TUI installed) to browse ActiveMQ
queues/messages and perform purge/move/send/delete, using the same
`mq-proxy` REST API (spec/10, spec/11) the TUI's proxy backend talks to.
A companion surface to the TUI, not a replacement — same operational
tasks as spec/09, through a click-first UI instead of a keyboard-driven
one.

## Behavior / user flow

- Source is `mq-proxy-web/src/*.js` — real ES modules, one per concern
  (connect screen, queue list, message list/detail, move picker,
  dialogs, the `mq-proxy` API client, shared DOM helpers, and
  `main.js` wiring it together) — built by `esbuild`
  (`task build:mq-proxy-web`) into a single, self-contained
  `dist/index.html`: the actual distributable, not committed to the
  repo. **Using** it needs neither install nor a build step (that's
  what `dist/index.html` is for); only *building* it does. Works both
  opened directly via `file://` (double-click) and served over
  `http(s)://` from any static file server.
- **Visual design**: a card-based layout (soft shadows, rounded
  corners) with a small topbar brand mark, and `.btn-primary`/
  `.btn-danger` styling distinguishing the page's main create/submit
  actions (Connect, Send, the JMS Type prompt's Continue) from
  destructive ones (Delete, Purge, the confirm dialog's Yes) —
  zebra-striped tables throughout. Follows the OS/browser's
  `prefers-color-scheme` automatically for both light and dark — no
  manual toggle, no settings/theming system. `color-scheme: light dark`
  on `:root` so native form controls (inputs/textareas) adapt too, and
  so the browser doesn't force-invert the page on its own
  (worse-looking than either declared palette — confirmed live).
  System fonts only, no network font fetch — the page has to keep
  working fully offline over `file://`.
- **Connect**: a form for the `mq-proxy` base URL plus its HTTP Basic
  username/password (the single `proxy.auth.username`/`password` pair
  configured on that `mq-proxy` instance — spec/10; not per-broker JMS
  credentials). Submitting validates via a `list-queues` call before
  switching views. Remembered in the browser's `localStorage` across
  visits ("Disconnect" clears it) — a deliberate convenience-over-security
  trade-off: credentials sit in the browser and travel as HTTP Basic
  (base64, not encrypted) on every request, so serving `mq-proxy` over
  HTTPS is effectively mandatory for any real deployment.
- **Queue list**: name, pending/consumer/enqueue/dequeue/producer counts
  (`list-queues`, spec/11). A live substring filter narrows by queue
  name, and clicking a column header sorts by it (ascending, click again
  for descending) — both client-side over the already-fetched list, and
  they compose. Each row: click the name to browse its messages, or use
  its Purge/Move all…/Send… actions. Every mutating action anywhere in
  the app that can change a queue's counts — Purge, Move all…, and the
  message-list/detail actions below (single or bulk delete/move) —
  refreshes the queue list in the background (`loadQueues()`) on
  success, so its counts are already current by the time the user
  navigates back to it; no manual "Refresh" click needed after any of
  the app's own actions (it still exists for external changes the app
  can't know about).
- **Message list**: `list-messages` with `returnBody=true` and a
  client-side default `filter.maxCount` of 500 (editable, shown in the
  view's title) — mirrors the TUI client's own default, satisfying
  `mq-proxy`'s hard requirement that `maxCount` be a positive integer on
  this endpoint (spec/11). An optional JMS Type filter narrows further
  (placeholder `*`, a "matches everything" convention — leaving it blank
  already meant "no filter" and still does; `*` is never actually sent).
  A **"Load more"** button below the table, shown only when the last
  response's `hasMore` (spec/11) was true, fetches the next page
  (`filter.afterMessageId` = the last rendered row's message ID) and
  appends to the currently-rendered list rather than replacing it —
  already-checked selections (below) survive the append since they're
  keyed by message ID, not row index. Applying the filter, opening a
  different queue, or a delete/move action completing all reset back to
  a single first page (all of which already trigger a full reload).
  Every row has a checkbox; a header checkbox
  (checked/indeterminate/unchecked synced to selection vs. total) plus
  "Select all"/"Select none" buttons operate on the currently-loaded list
  (client-side — selection isn't a server concept, and resets on every
  reload). One or more selected messages enables "Delete selected"/"Move
  selected…", each sending one `delete-messages`/`move-messages` array
  element per selected message (`filter.messageId` + `maxCount: 1` each)
  — the bulk-array shape spec/11 documents as existing "for
  bulk-operation parity" but, until this page, never actually used by
  either client. Row click opens message detail.
- **Message detail**: four fields (Queue, Message ID, JMS Type,
  Timestamp — `MessageSummary.sourceQueue`, spec/11, falls back to the
  currently-open queue if ever unset), then a **Headers** section listing
  every entry from `mq-proxy`'s `headers` map (sorted `Key: value`,
  mirroring the TUI's own Headers section, spec/08 — `mq-proxy` always
  returns this map, independent of `returnBody`; the section is omitted
  entirely when the map is empty), then the body. Delete (single
  `delete-messages` call) and Move… (opens the target-queue picker).
- **Purge / Move all (drain)**: an optional JMS Type prompt — blank means
  "match everything" (`filter.maxCount` stays unset, matching the TUI's
  `PurgeQueue`/`MoveAllMessages` vs. `DeleteMessages`/`MoveMessages`
  distinction, spec/09) — followed, for purge, by a confirm dialog
  (`Purge "<queue>"? All messages will be deleted.`, or `All <type>
  messages...` when a type was entered). The prompt's field has
  **autocomplete**: the moment it opens, it scans the queue in the
  background (`list-messages`, `returnBody=false`, capped at 500 —
  mirrors the TUI's own automatic-scan cap, spec/08) and populates a
  native `<datalist>` with the distinct JMS Types found; the scan never
  blocks or gates Continue/Cancel, and a failed scan just leaves the
  suggestion list empty rather than surfacing an error. Unlike the TUI,
  there's no second opt-in "scan more" tier — a single fixed-cap
  automatic scan was judged sufficient for this simpler surface.
- **Move target picker**: same four-tier DLQ-priority ordering as the
  TUI's move picker (spec/09) — a queue matching the source's stripped
  `dlq.`/`imq.` prefix first, then regular queues, then
  `dlq.`/`imq.`-prefixed queues, then `activemq.`/`statistics.`-prefixed
  ones, alphabetical within each tier, source queue excluded. A live
  substring filter narrows the list.
- **Send**: a modal with a body field and an optional JMS Type field —
  unlike spec/09's TUI, which has no JMS Type field and hardcodes
  `"text"`, this page exposes one since `mq-proxy`'s `send-message` DTO
  requires the field and it's a normal thing to want to set; a blank
  entry still defaults to `"text"`. No headers/properties/templates.
- Errors (auth failure, network failure, `mq-proxy` validation errors —
  the `{ data, errors }`/`{ data, error }` envelope, spec/11) are shown
  inline in the page, not just logged to the console.

## `mq-proxy` CORS support

`mq-proxy` (spec/10) has a `CorsConfigurationSource` bean
(`SecurityConfig.corsConfigurationSource`) so a page hosted on a
different origin — like this one — can call its API:

- **Allowed origins**: a configurable list (`proxy.cors.allowed-origins`
  in `application.yml`, env override `CORS_ALLOWED_ORIGINS`,
  comma-separated) for served `http(s)://` deployments, **plus** the
  literal `null` origin always allowed unconditionally — what a browser
  sends for a `file://`-opened page. Defaults to allowing only `null`
  (no configured origins) when unset.
- **Allowed methods**: `GET`, `POST`, `OPTIONS`. **Allowed headers**:
  `Authorization`, `Content-Type`.
- **No `allowCredentials`**: this API is called with an explicit
  `Authorization` header the client sets itself, not cookies/credentialed
  fetch mode, so it isn't a "credentialed" CORS request in the spec
  sense — the origin allow-list is a deliberate access control, not
  something CORS forces.
- A CORS preflight (`OPTIONS`) is `permitAll`'d in
  `authorizeHttpRequests`, ahead of the `/api/**` `authenticated()` rule
  — it never carries the `Authorization` header, so without this every
  cross-origin call would fail its preflight with 401 before the real
  request is ever sent.

## Data & config

```
mq-proxy-web/
  src/
    dom.js          # $, escapeHtml, truncate, showError/clearError/showView, ...
    api.js          # mq-proxy REST client + request/response shaping
    state.js        # the page's single shared mutable state object
    connection.js   # connect screen, localStorage
    queues.js       # queue list: filter/sort/render, purge, move all
    messages.js     # message list/detail: pagination, bulk actions
    movepicker.js   # move-target picker, DLQ-tier ordering
    dialogs.js      # JMS Type prompt, confirm dialog, send modal
    main.js         # entry point — wires every view's static controls
    *.test.js       # co-located tests, import directly from the module next to them
  index.html   # dev template: markup + inline <style>, <script type="module" src="./src/main.js">
  package.json # esbuild devDependency
  build.mjs    # bundles + inlines src/main.js into index.html -> dist/index.html
  dist/        # build output (gitignored) — dist/index.html is the distributable
  README.md
```

- `mq-proxy/src/main/kotlin/.../config/CorsProperties.kt` —
  `proxy.cors.allowed-origins`, defaults to an empty list.
- `mq-proxy/src/main/kotlin/.../config/SecurityConfig.kt` — CORS wiring
  (see above).
- `Taskfile.yml`: `test:mq-proxy-web` (`node --test`, run from
  `mq-proxy-web/`, no `npm install` needed — tests only import local
  files), included in `task test`; `build:mq-proxy-web` (`npm ci && npm
  run build`) producing `dist/index.html`.
- `.github/workflows/ci.yml`: a `mq-proxy-web` job, matrixed over the
  same three OSes as `tui`/`mq-proxy` — no OS-specific branching needed,
  since all three GitHub-hosted runners ship Node by default and `node
  --test`/`npm` behave identically on each. Runs `test:mq-proxy-web`
  then `build:mq-proxy-web`, so a broken build fails CI the same way a
  broken test does.

## Implementation notes

- `src/*.js` are real ES modules (native `import`/`export`), bundled by
  `esbuild`'s JS API (`build.mjs`) into a single **IIFE** — not
  `type="module"` output — and inlined into `index.html`'s script tag as
  `dist/index.html`. The IIFE format is what keeps the *built* file a
  classic script, safe to open via `file://` (Chrome blocks ES module
  loading over `file://` as a cross-origin fetch, which would break the
  double-click-to-open case) — the same constraint an earlier version's
  single `app.js` + UMD-lite wrapper worked around without a build step
  at all. The checked-in `index.html` (the dev template) uses
  `<script type="module" src="./src/main.js">` instead, so source can be
  iterated on directly by serving the folder — no build needed to see a
  change, only to produce the real distributable.
- Circular imports exist between `queues.js`, `messages.js`, and
  `dialogs.js` (e.g. a queue row's click opens `messages.js`, while a
  message action calls back into `queues.js`'s `loadQueues` to refresh
  counts) — safe here since every cross-reference is a hoisted
  `export function` invoked only at runtime, never at module-evaluation
  time; confirmed via `esbuild` producing zero bundling
  warnings/errors. Left as direct imports rather than threading
  callbacks through every call site to avoid the cycle.
- DOM wiring lives in each module's own `initXxx()` function (e.g.
  `initQueues`, `initMessages`), called once from `main.js` — keeps
  `main.js` a thin entry point rather than a dumping ground for logic
  that belongs with its own concern. Only the pure logic (auth header,
  envelope parsing, query building, purge/move-all filter branching, DLQ
  move-target ordering, bulk request bodies, header sorting) is unit
  tested; DOM rendering/click-handling is not.
- CORS preflight behavior is tested with a full `@SpringBootTest`
  (`webEnvironment = RANDOM_PORT`) hitting a real embedded server, not a
  `@WebMvcTest` slice — a slice test gave false 401s for this specific
  filter-chain interaction (Spring Security's `CorsFilter` short-circuits
  the chain before a mock-dispatcher-based test reliably exercises it);
  confirmed correct against the real running app via `curl` first, then
  matched with `SecurityConfigTest`.

## Notable gotchas worth preserving

- **Every `.modal` and the topbar set their own `display: flex`, which —
  being author-origin CSS — beats the browser's default `[hidden] {
  display: none }` UA rule regardless of selector specificity.** Found
  live: the send-message modal covered the entire page on first load,
  before any button had ever been clicked. Fixed with a single `[hidden]
  { display: none !important; }` rule in `index.html`. The general
  lesson: any element whose own stylesheet sets `display` needs an
  explicit `[hidden]` override if it also relies on the `hidden`
  attribute for visibility — the attribute alone isn't enough once an
  author rule sets `display`.
- **Table cells and `<dl>` values need `overflow-wrap: anywhere`.** A
  value with no natural break point — a UUID, an
  `ALL_CAPS_WITH_UNDERSCORES` JMS type, a long URL, raw JSON — has
  nowhere to wrap by default and overflows its column/grid track instead,
  blowing out the whole layout rather than staying contained. Found live
  twice, independently: once on the messages table (`th, td`), once on
  the message detail view's `<dl>` (`dt, dd`) once the Headers section
  existed — each element type needed its own explicit rule; `overflow-wrap`
  doesn't inherit across element boundaries in a way that would have
  covered both from one declaration.
- **A stale "Load more" cursor can surface a visible duplicate/stale
  row, by design.** `filter.afterMessageId`'s server-side fallback
  (spec/11) silently restarts from the beginning of the queue when the
  cursor message is gone (e.g. deleted by a concurrent action between
  page fetches) — but the client still *appends* that fallback page
  rather than replacing what's already rendered, so the already-deleted
  cursor message and a duplicate of the first message can both remain
  visible until the next full reload. Confirmed live, driven through the
  real UI (not just `curl`): no error surfaces, matching the design —
  simplicity over reconciliation was the deliberate trade-off, not a bug
  to fix.

## Out of scope (deliberate)

- No Jolokia backend support — this page only ever talks to `mq-proxy`.
- No multiple saved/named connections (spec/12 is TUI-only) — one active
  `mq-proxy` connection at a time per browser.
- No login system of its own beyond `mq-proxy`'s single shared Basic-auth
  pair, no per-user roles or audit trail.
- No page-number-based navigation (jump to page 5) for messages —
  "Load more" only, matching the cursor's forward-only nature (spec/11).
- No keyboard-driven power-user UX, no theme picker/settings.
- No offline/PWA support.
