# Tasks

1. [x] Build tooling scaffolding: `mq-proxy-web/package.json` +
   `package-lock.json` (esbuild devDependency, pin latest stable —
   check `npm view esbuild version`), `build.mjs` (esbuild JS API,
   IIFE bundle inlined into `index.html` → `dist/index.html`),
   `.gitignore` entries (`node_modules/`, `dist/` — `dist/` was
   already globally ignored, only `node_modules/` was new), new
   `build:mq-proxy-web` Taskfile task, new CI step
   (`.github/workflows/ci.yml`) running it after `test:mq-proxy-web`
   on all 3 OSes. Verified with a trivial placeholder `src/main.js`
   (sets a visible marker in `document.title`) — `task
   build:mq-proxy-web` produces a working `dist/index.html`; the
   Chrome extension can't reach `file://` URLs directly, so verified
   the bundle actually executes by serving `dist/` over a local static
   server instead (the classic, non-`type="module"` inline `<script>`
   the build produces is what makes it `file://`-safe in the first
   place — the same format `app.js`'s current UMD wrapper already
   relies on — so serving it over http is sufficient to confirm the
   bundle logic itself runs correctly).

   **Gotcha found**: `package.json`'s `"type": "module"` applies to
   every `.js` file in the tree, not just new ones — it broke the
   still-CommonJS `app.js`/`app.test.js` (not yet migrated; that's
   task 2). Left `"type": "module"` out of `package.json` for now;
   `build.mjs` needs no such field (`.mjs` always forces ES module
   treatment regardless), and task 2 adds it back once `app.js`/
   `app.test.js` are deleted and nothing conflicts with it anymore.
   `task test:mq-proxy-web` still passes all 43 existing cases
   unaffected.

2. [x] Module split: `app.js`/`app.test.js` → `src/{dom,api,state,
   connection,movepicker,queues,messages,dialogs,main}.js` +
   co-located `src/*.test.js` (see `plan.md` for the exact function-
   to-module mapping), native ES module imports throughout, no UMD
   wrapper. Delete `app.js`/`app.test.js`. `index.html`'s script tag
   becomes `<script type="module" src="./src/main.js"></script>`.
   Purely structural — same functions, same logic, no behavior change.

   Static wiring (`addEventListener` calls that don't depend on a
   specific row/item) moved into an `initXxx()` function exported by
   each concern module (`initConnection`, `initQueues`, `initMessages`,
   `initMovePicker`, `initDialogs`), each called once from `main.js` —
   keeps `main.js` a thin entry point rather than a dumping ground for
   logic that belongs with its own concern (a deliberate refinement of
   `plan.md`'s "main.js wires everything" wording, not a behavior
   change). `queues.js`/`messages.js`/`dialogs.js` end up mutually
   importing each other (e.g. a queue row's click opens `messages.js`,
   while a message action calls back into `queues.js`'s `loadQueues`
   to refresh counts) — safe for `export function` declarations in ES
   modules (hoisted, live bindings; confirmed via `esbuild`'s own
   bundling producing zero warnings/errors), so left as direct imports
   rather than threading callbacks through every call site to avoid
   the cycle.

   Verification: `task test:mq-proxy-web` passes all 43 cases
   (relocated, not reduced); `task build:mq-proxy-web` succeeds; live
   end-to-end walk (via `claude-in-chrome`, serving the built
   `dist/index.html` over a local static server against a live
   `mq-proxy`, `CORS_ALLOWED_ORIGINS` set for that origin since the
   Chrome extension can't reach `file://` directly — same constraint
   as task 1) of connect, queue list filter + column sort, open a
   queue, send a message, message detail, single delete, bulk
   select/delete/move (move-picker's DLQ-tier ordering confirmed
   correct), purge with a JMS-Type-scoped confirm, and queue-count
   auto-refresh after every mutating action (the earlier
   bugfix-stale-queue-counts behavior, still correct through the
   split). Test data sent/moved/purged during verification was
   scoped to distinctive JMS Types/bodies and cleaned up afterward,
   leaving a pre-existing unrelated message on the broker untouched.

3. [x] Visual refresh (CSS/markup only, plus one small non-behavioral
   JS touch — see below): fuller palette (`--panel-alt`, `--accent`/
   `--accent-hover`/`--accent-text`, `--danger`/`--danger-hover`/
   `--danger-text`, `--shadow`/`--shadow-lg`), card/shadow treatment
   for `section`/`.modal-box`, a topbar brand mark (`.brand`, a small
   accent-colored square + wordmark), `.btn-primary`/`.btn-danger`
   applied to the real Connect/Send/Continue vs. Delete/Purge/confirm-
   Yes buttons, table zebra-striping (`tbody tr:nth-child(even)`) and
   refined `<thead>` styling, a focus ring on inputs, and an
   alert-style treatment for `p.error`. Both light and dark
   (`prefers-color-scheme`) palettes updated together.

   `dom.js`'s `makeButton` gained an optional third `className`
   parameter (was: label, onClick) so the dynamically-created queue-row
   Purge/Send buttons (`queues.js`) could get `btn-danger`/`btn-primary`
   too — the one non-CSS/markup change, purely visual, no behavior
   change.

   Manual verification: `task test:mq-proxy-web` still passes all 43
   cases (the `makeButton` signature change isn't covered by existing
   tests — DOM rendering/click-handling was never unit tested in this
   file, matching spec/21's established scope). Live-checked the built
   `dist/index.html` in dark mode via `claude-in-chrome` (connect
   screen, topbar with brand, queue list zebra-striping and danger/
   primary buttons, send modal, message detail, confirm dialog) — all
   render as intended. Light mode's underlying CSS was verified
   correct via computed-style inspection (`getComputedStyle`) rather
   than a screenshot: this sandboxed Chrome session's own screenshot
   pipeline appears to force a hue-shifted dark-ish repaint of *all*
   colors regardless of actual page/OS preference (confirmed by
   toggling real OS light/dark producing no screenshot difference,
   while non-color computed properties like `border-radius: 12px` and
   the `--bg`/`--panel`/`--accent` custom property values themselves
   all resolved exactly as authored) — an environment artifact of this
   session, not a bug in the page.

4. [ ] `README.md` update: document the `src/` module structure, the
   build step (`task build:mq-proxy-web`, needs Node + npm), and that
   `dist/index.html` — not the repo-root `index.html` template — is
   now the actual distributable end users should be pointed at.
   Rewrite the "Why two files, not one" section to explain the new
   split: source is multi-file and testable, the *build output* is
   what's single-file.

5. [ ] Merge-back: update `spec/21-amq-web-console` to document the
   `src/` ES-module structure, the build step and `dist/index.html`
   as the distributable, and the visual refresh — then delete
   `spec-wip/cr-mq-proxy-web-es-modules/`. Mark the PR ready for
   review.
