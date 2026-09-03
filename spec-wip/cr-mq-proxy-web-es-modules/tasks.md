# Tasks

1. [ ] Build tooling scaffolding: `mq-proxy-web/package.json` +
   `package-lock.json` (esbuild devDependency, pin latest stable —
   check `npm view esbuild version`), `build.mjs` (esbuild JS API,
   IIFE bundle inlined into `index.html` → `dist/index.html`),
   `.gitignore` entries (`node_modules/`, `dist/`), new
   `build:mq-proxy-web` Taskfile task, new CI step
   (`.github/workflows/ci.yml`) running it after `test:mq-proxy-web`
   on all 3 OSes. Verified with a trivial placeholder `src/main.js`
   (e.g. sets some visible DOM text) — `task build:mq-proxy-web`
   produces a working `dist/index.html`, opened directly via `file://`
   to confirm the IIFE bundle actually runs (not just that the build
   command exits 0).

2. [ ] Module split: `app.js`/`app.test.js` → `src/{dom,api,state,
   connection,movepicker,queues,messages,dialogs,main}.js` +
   co-located `src/*.test.js` (see `plan.md` for the exact function-
   to-module mapping), native ES module imports throughout, no UMD
   wrapper. Delete `app.js`/`app.test.js`. `index.html`'s script tag
   becomes `<script type="module" src="./src/main.js"></script>`.
   Purely structural — same functions, same logic, no behavior change.

   Verification: `task test:mq-proxy-web` passes with the same 43
   cases (relocated, not reduced); `task build:mq-proxy-web` succeeds;
   manually open the built `dist/index.html` via `file://` against a
   live `mq-proxy` and walk the golden path (connect, browse queues,
   browse/filter/paginate messages, select/delete/move messages, purge,
   move all, send, message detail) to confirm nothing regressed in the
   move from one file to many.

3. [ ] Visual refresh (CSS/markup only, no logic changes): fuller
   palette, card/shadow treatment for `section`/`.modal-box`, topbar
   visual identity, `.btn-primary`/`.btn-danger` distinction applied to
   the real Connect/Save/Send vs. Delete/Purge buttons, table
   zebra-striping and refined header styling. Both light and dark
   (`prefers-color-scheme`) palettes updated together.

   Manual verification: open the built `dist/index.html`, check both
   an OS light-mode and dark-mode session (toggle the OS setting, or
   use browser devtools' rendering emulation), confirm every existing
   view/dialog still functions identically and looks like a
   deliberate, cohesive refresh rather than a broken one-off page.

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
