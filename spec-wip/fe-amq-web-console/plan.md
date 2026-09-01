# Implementation plan

## Approach

Two independent pieces, one PR:

1. **`mq-proxy-web/`** — a new top-level module (sibling to `tui/` and
   `mq-proxy/`), the static browser page itself.
2. **`mq-proxy/`** — a small, additive CORS change so the page (served
   from a different origin, or opened via `file://`) can call it.

No shared build between any of these three modules, consistent with how
`tui/` and `mq-proxy/` already coexist.

## `mq-proxy-web/` — the static page

### File layout

```
mq-proxy-web/
  index.html      # markup + inline <style>, loads app.js as a classic <script src>
  app.js          # all logic (connection, API calls, rendering, actions)
  app.test.js     # unit tests for app.js's pure functions (Node's built-in test runner)
  README.md       # how to open it, how to run the tests
```

**Why not one single file, and why not ES modules.** A genuinely
self-contained single `index.html` was the first instinct (simplest to
hand someone), but it's incompatible with unit-testing: there'd be no way
for a test to import logic out of an inline `<script>` block without
duplicating it. Splitting into `index.html` + `app.js` keeps things
dependency-free while making `app.js` importable by tests. ES modules
(`<script type="module">`) were considered instead of a classic script,
since they'd let `app.js` use real `export`/`import` — but Chrome (and,
inconsistently, other browsers) blocks ES module loading over `file://`
as a cross-origin fetch, which breaks the double-click-to-open requirement
from the spec. A **classic** `<script src="app.js">` has no such
restriction. To still make `app.js` reachable from Node tests without a
bundler, it uses a small UMD-lite pattern:

```js
(function (exports) {
  // ...all logic...
  exports.buildAuthHeader = buildAuthHeader;
  exports.parseEnvelope = parseEnvelope;
  // ...
})(typeof module !== 'undefined' ? module.exports : (window.CloudtuiMQ = {}));
```

This is a well-worn zero-dependency pattern (no bundler, no `package.json`,
no `node_modules`) — the same file loads as a global in the browser and as
a CommonJS module (`require('./app.js')`) in Node.

### Behavior, mapped to the mq-proxy wire contract (spec/11)

- **Connect form**: base URL, username, password. On submit, stores them
  in `localStorage` (key e.g. `cloudtui-mq-proxy-connection`) and does a
  cheap `list-queues` call to validate before switching to the main view;
  on 401/network failure, shows the error inline and stays on the form.
  A "Forget connection" action clears `localStorage` and returns here.
- **Auth**: every request manually sets
  `Authorization: Basic <base64(username:password)>` — never relies on
  the browser's own HTTP-auth prompt/cache, so the app controls when
  credentials are sent and can present its own connect form and its own
  error message on 401, matching the rest of this page's UI instead of a
  native browser dialog.
- **Queue list**: `GET /api/management/command/list-queues`, table with
  name/pending/consumer/enqueue/dequeue/producer columns (spec/11's
  `list-queues` fields). Row click opens that queue's messages.
- **Message list**: `GET /api/management/command/list-messages` with
  `sourceQueue`, `returnBody=true`, and `filter.maxCount` (default 500,
  shown in the view's title, editable) — nested `filter.*` query params
  only, per spec/11 (the flat-param shape is superseded, never used
  here). Row click opens message detail.
- **Message detail**: full headers + body, a **Delete** button
  (`POST delete-messages`, one-element array, `filter.messageId` set) and
  a **Move** button opening the target-queue picker (`POST
  move-messages`, one-element array).
- **Purge** (queue list row action): optional JMS Type prompt → confirm
  dialog → `POST delete-messages` with `filter.jmsType` set and
  `filter.maxCount` **omitted** (matches spec/11's "unset means match
  everything" behavior — the same distinction the TUI's `PurgeQueue` vs.
  `DeleteMessages` makes, spec/09).
- **Move all / drain** (queue list row action): optional JMS Type prompt
  → target-queue picker → `POST move-messages`, filter as above.
- **Send**: modal with a plain-text body field → `POST send-message` with
  `targetQueue` + `body` (no headers/JMS type/templates, matching spec/09's
  TUI scope).
- **Response envelope**: every response unwrapped as `{ data, errors }`
  (or `{ data, error }`) per spec/11 — a small `parseEnvelope()` helper
  used by every call site, surfacing `errors`/`error` as the inline error
  banner text.
- **No client-side retry logic** — spec/11 documents the TUI's Go client
  retrying GETs once on transport failure; this page does not replicate
  that (out of scope, not a safety requirement — a manual "reload" click
  covers it, and browser `fetch` failures are already visibly reported).

### Testing

- `app.test.js` (Node's built-in `node:test` + `node:assert`, zero
  npm dependencies) covers the pure, DOM-free logic: `buildAuthHeader`,
  `parseEnvelope`, the `list-messages` query-string builder (nested
  `filter.*` shape), the purge/move-all "blank filter → omit maxCount"
  branching, and the JMS-Type/DLQ-prefix move-target ordering (mirroring
  the TUI's four-tier logic from spec/09, reused here for the same
  reason: DLQ requeue is the dominant workflow).
- DOM rendering and click-handling itself is **not** unit tested (no DOM
  test runner is introduced) — covered instead by an explicit manual
  test pass in `tasks.md` against a live `mq-proxy` (using the existing
  dev tooling, spec/13), for both a served-origin load and a `file://`
  double-click load.
- `Taskfile.yml` gets a new task (e.g. `test:mq-proxy-web`) running
  `node --test mq-proxy-web/`. Whether to also add this to the CI
  workflow (spec/02) is worth a quick confirmation — GitHub's
  `ubuntu-latest` runner ships Node by default, so it should be a
  low-cost addition, but it's still a change to an already-shipped CI
  spec, called out explicitly rather than assumed.

## `mq-proxy/` — CORS change

- New Spring `CorsConfigurationSource` bean in the existing
  `config/SecurityConfig` (spec/10's package layout), wired into the
  security filter chain via `http.cors(...)`.
- **Allowed origins**: a configurable list from `application.yml`
  (new key, e.g. `proxy.cors.allowed-origins`), overridable via env var
  (`CORS_ALLOWED_ORIGINS`, comma-separated) — matching the existing
  `BROKER_URL`-style override convention (spec/10) — **plus** the literal
  origin `null` always included, unconditionally (not configurable),
  since that's what a `file://`-opened page sends and there's no
  meaningful narrower value to configure for it.
- **Allowed methods**: `GET`, `POST`, `OPTIONS` (matches spec/11's
  actual verbs; `OPTIONS` for CORS preflight itself).
- **Allowed headers**: `Authorization`, `Content-Type` — `Authorization`
  is what makes `list-messages`/etc. non-"simple" requests, which is why
  a CORS preflight is triggered at all and needs to be explicitly
  permitted through Spring Security (not just the CORS filter).
- **`allowCredentials`**: **not** enabled — this app sends `Authorization`
  as an explicit header it sets itself, not via `fetch(..., {credentials:
  "include"})`/cookies, so it isn't a "credentialed" CORS request in the
  spec sense, and an explicit origin allow-list is a deliberate access
  control rather than a technical requirement of using Basic auth.
- Swagger UI / `/v3/api-docs*` (already `permitAll`, spec/10) are
  unaffected — CORS only matters for cross-origin browser calls, and this
  change is additive.
- Test: extend the existing Spring test slice (`QueueControllerTest` or a
  new small test) with a `MockMvc` preflight (`OPTIONS`) check — one
  configured origin is allowed, an arbitrary unlisted origin is rejected,
  and `null` is always allowed.

## Decisions (previously open)

1. **CI wiring**: `test:mq-proxy-web` is added to `.github/workflows/ci.yml`
   (spec/02 gets a small update for this) — it's fast, dependency-free,
   needs no live broker, unlike spec/13's manual tooling.
2. **CORS config docs**: the new `proxy.cors.allowed-origins` /
   `CORS_ALLOWED_ORIGINS` keys are documented in the existing
   `mq-proxy/README.md` (alongside `BROKER_URL` etc.), not in
   `mq-proxy-web/README.md` — that one stays scoped to "how to open the
   page / how to run its tests".
