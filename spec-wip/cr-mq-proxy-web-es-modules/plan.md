# Plan

## Module boundaries

Migrating `app.js`'s existing comment-delimited sections into files
under `mq-proxy-web/src/`, mostly 1:1:

- **`dom.js`** — `$`, `escapeHtml`, `truncate`, `sortedHeaderEntries`,
  `showError`, `clearError`, `showView`, `makeButton`.
- **`api.js`** — `base64Encode`, `buildAuthHeader`, `parseEnvelope`,
  `errorMessageFrom`, `buildQueryString`, `apiCall`,
  `buildListMessagesParams`, `buildJmsTypeScanParams`,
  `buildBulkFilter`, `resolveJmsType`, `buildSingleMessageFilter`,
  `buildBulkDeleteBody`, `buildBulkMoveBody`, `appendMessages`,
  `extractDistinctJmsTypes`, `DEFAULT_MAX_COUNT`.
- **`state.js`** — the shared mutable `state` object (`conn`,
  `currentQueue`, `currentMessage`, `queues`, `queueSort`, `messages`,
  `messagesHasMore`, `selectedMessageIds`), exported for the other
  modules to import and mutate directly (same shape as today's
  closure-scoped `state`, just given a module boundary).
- **`connection.js`** — `loadStoredConnection`, `storeConnection`,
  `forgetConnection`, `tryConnect`.
- **`movepicker.js`** — `tierForQueue`, `sortMoveTargets`,
  `openMovePicker`, `renderMovePickerList`.
- **`queues.js`** — `filterQueues`, `sortQueues`, `loadQueues`,
  `applyQueueView`, `updateSortIndicators`, `renderQueues`,
  `purgeQueue`, `moveAllMessages` (queue-list-level actions, which use
  `movepicker.js` and the shared JMS Type prompt from `dialogs.js`).
- **`messages.js`** — `openMessages`, `loadMessages`,
  `loadMoreMessages`, `updateLoadMoreButton`, `renderMessages`,
  `updateBulkActionsUI`, `selectAllMessages`, `selectNoneMessages`,
  `openMessageDetail`.
- **`dialogs.js`** — `jmsTypePrompt`, `confirmDialog`,
  `openSendModal`.
- **`main.js`** — entry point: every `addEventListener`/DOM-wiring
  call currently scattered through `app.js`, plus startup
  (`loadStoredConnection` call, initial view). Imports from all of the
  above.

No behavior change in this migration — same functions, same logic,
just given module boundaries and explicit `import`/`export` instead of
one shared closure.

## Build tooling

- **`package.json`** (new): `"type": "module"`, `esbuild` as the sole
  `devDependency` (pin whatever's the latest stable release at
  implementation time — check `npm view esbuild version`), `scripts.build`
  → `node build.mjs`, `scripts.test` → `node --test` (mirrors
  `task test:mq-proxy-web`, useful for local iteration without `task`).
  `package-lock.json` committed (it's a pinned-dependency manifest,
  not a build output — same category as `go.sum`).
- **`build.mjs`** (new): uses `esbuild`'s JS API —
  `esbuild.build({ entryPoints: ['src/main.js'], bundle: true, write:
  false, format: 'iife' })` (IIFE, not ESM output — this is what lets
  the *built* file stay a classic `<script>`, avoiding the
  `file://`-blocks-ES-modules problem the current UMD-lite wrapper
  works around today). Reads `index.html`, replaces the `<script
  type="module" src="./src/main.js"></script>` tag with `<script>
  ${bundledCode} </script>`, writes the result to `dist/index.html`.
- **`index.html`** (checked-in template): keeps its markup/`<style>`,
  but its script tag becomes `<script type="module"
  src="./src/main.js"></script>` — this means it can be iterated on
  directly via a static server (`npx serve`, `python3 -m http.server`)
  during development, no build step needed to just look at changes.
  Only `file://` use and final distribution need the built
  `dist/index.html`.
- **`.gitignore`**: add `mq-proxy-web/node_modules/`,
  `mq-proxy-web/dist/`.
- **`Taskfile.yml`**: new `build:mq-proxy-web` task (`dir:
  mq-proxy-web`, `cmds: [npm ci, npm run build]`).
- **`.github/workflows/ci.yml`**: `mq-proxy-web` job gains a
  `task build:mq-proxy-web` step after the existing `task
  test:mq-proxy-web` step, on all 3 OSes (Node's already available by
  default on GitHub-hosted runners, per spec/21).

## Tests

`app.test.js`'s 43 existing cases move into co-located `src/*.test.js`
files matching the module split above (e.g. `src/api.test.js`
importing `{ buildAuthHeader, parseEnvelope, ... } from
'./api.js'`) — native ES module `import`, no UMD wrapper. `node --test`
auto-discovers `*.test.js` recursively, so `task test:mq-proxy-web`
(`node --test`, run from `mq-proxy-web/`) keeps working unchanged and
still needs no `npm install` (tests only import local files, nothing
from `node_modules`).

## Visual refresh

CSS/markup-only, no logic changes, done as its own task after the
module split lands (keeps that diff purely structural and this one
purely cosmetic):

- New CSS custom properties for a fuller palette (beyond today's
  single `--accent`): distinguish primary/secondary/destructive
  intent via color, not just via which button happens to say "Delete".
- Card-based `section`/`.modal-box` treatment: soft shadow +
  slightly larger border-radius instead of today's flat 1px border,
  in both light and dark palettes.
- Topbar: a small visual identity treatment (e.g. an accent-colored
  wordmark/badge) instead of plain text next to the connection status.
- Buttons: a `.btn-primary`/`.btn-danger` class distinction layered
  onto the existing button styling, applied to the actual
  Connect/Save/Send and Delete/Purge buttons in `index.html`.
- Tables: zebra-striped rows, refined `<th>` styling (background tint
  instead of just a bottom border), clearer hover/selected treatment.
- Still driven only by `prefers-color-scheme` — no manual toggle
  added, no theming system, matching spec/21's existing scope. Still
  system fonts only (no network font fetch — the built artifact has to
  keep working fully offline over `file://`).

## Task breakdown approach

1. Build tooling scaffolding (`package.json`, `build.mjs`,
   `.gitignore`, `Taskfile.yml`, CI step) — verified against a trivial
   placeholder `src/main.js` before any real logic moves, so the
   toolchain itself is proven working in isolation.
2. Module split: `app.js`/`app.test.js` → `src/*.js`/`src/*.test.js`,
   `index.html`'s script tag updated, old files deleted. Purely
   structural — verified via unchanged test count/behavior and a
   manual smoke test of the built `dist/index.html` against a live
   `mq-proxy`.
3. Visual refresh (CSS/markup only).
4. `README.md` update: new `src/` structure, the build step, and that
   `dist/index.html` (built) — not the repo-root `index.html` template
   — is the actual distributable now.
