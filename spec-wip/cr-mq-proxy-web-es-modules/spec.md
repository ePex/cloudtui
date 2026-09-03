# CR: `mq-proxy-web` as ES modules, built to a single-file HTML artifact

Date: 2026-09-03

## Purpose

`mq-proxy-web/app.js` (spec/21) is a single ~900-line file — every
concern (API calls, connect/localStorage, queue list, message list,
message detail, purge/move-all, move picker, send modal) in one flat
UMD-lite-wrapped script. This CR splits it into real ES modules by
concern for maintainability, while keeping the thing a non-technical
user actually receives a **single, self-contained `index.html`** —
double-click and go, same as today.

`mq-proxy-web/README.md`'s existing "Why two files, not one" section
already explains the tension this resolves: a single inline
`<script>` can't be unit tested (nothing to `import` out of it), which
is why `app.js` exists as a separate classic script with a UMD-lite
wrapper in the first place. Real ES modules as the **source** (tests
import them directly — Node's test runner supports ES modules
natively, no UMD wrapper needed) plus a **build step** that bundles
and inlines them into one HTML file resolves this the standard way:
source stays multi-file and testable, only the generated artifact is
single-file.

## Scope

- **Source restructure**: `app.js` split into ES modules under
  `mq-proxy-web/src/`, grouped by the sections that already exist as
  comments in today's file — roughly `api.js` (auth header, envelope
  parsing, query building, `apiCall`), `connection.js` (connect,
  localStorage), `queues.js` (filter/sort/render), `messages.js`
  (list/pagination/bulk actions/detail), `movepicker.js` (DLQ-tier
  ordering), `dialogs.js` (JMS Type prompt, confirm, send modal), a
  small `dom.js` for shared helpers (`$`, `escapeHtml`, `truncate`,
  ...), and `main.js` as the entry point wiring DOM event listeners to
  the rest. Exact boundaries finalized in `plan.md`.
- **Build tooling**: `esbuild` (new, first npm dependency for this
  module) bundles `src/main.js` and inlines the result into
  `index.html`'s `<script>`, producing a single self-contained
  generated file. `package.json` (new) pins an exact `esbuild`
  version; `node_modules/` and the build output are gitignored — the
  generated HTML is a build artifact, not something committed (same
  treatment as `tui`'s binary or `mq-proxy`'s jar).
- **Tests**: `*.test.js` files import directly from `src/*.js` (native
  ES module `import`, no UMD wrapper, no bundler needed to run them —
  `node --test` keeps working exactly as it does today, no `npm
  install` required just to test).
- **Taskfile/CI**: new `build:mq-proxy-web` task; CI gains a build
  step (Node + `npm ci` to fetch `esbuild`) verifying the artifact
  still builds, alongside the existing test step.
- **`index.html`**: markup + `<style>` unchanged in substance; becomes
  a template with a placeholder the build step fills in (or a `<script
  type="module" src="./src/main.js">` for local dev against a static
  server, with the build step producing the inlined, classic-script,
  `file://`-safe distributable separately — exact approach decided in
  `plan.md`).

## Visual refresh

Bundled into this same CR at the user's request ("while you're on it
beautify the ui"), scoped as a **fuller visual refresh**: more
noticeable than a light polish pass, but still CSS/markup-only — no
new behavior, no new views, no new interaction patterns.

- **Palette**: a more distinctive accent/color system than today's
  single flat `--accent` blue — still driven entirely by
  `prefers-color-scheme` (no manual toggle, no theming system, per the
  existing scope), still system fonts only (no Google Fonts/CDN — the
  page has to keep working fully offline over `file://`, which a
  network font fetch would break).
- **Layout**: move from today's flat bordered `<section>` blocks
  toward a card-based layout with real elevation (subtle shadows, not
  just 1px borders) and clearer visual hierarchy between the topbar,
  primary content, and the queue/message tables.
- **Topbar/branding**: a clearer visual identity for the page itself
  (it's currently just a title + connection status, no distinct
  branding treatment).
- **Buttons**: distinguish primary actions (Connect, Save, Send) from
  secondary/destructive ones (Cancel, Delete, Purge) — today every
  button uses the same flat style regardless of what it does.
- **Tables**: zebra-striping, refined header styling, better
  hover/selected-row treatment for the queue and message lists.
- Dark mode stays automatic (`prefers-color-scheme`, `color-scheme:
  light dark`) — both palettes get the refresh, not just one.

## Out of scope

- No *functional* behavior change — this is a structural/tooling
  change plus a visual refresh. Any difference in what the page does
  (API calls, data shown, click behavior) is a bug, not a feature —
  only how it looks and how the source is organized change.
- No TypeScript, no framework (React/Vue/etc.) — plain ES modules,
  matching the page's existing "no framework" character.
- No change to `mq-proxy`'s CORS/auth story, no change to
  `mq-proxy-web/README.md`'s "Why two files, not one" *intent* (a
  single distributable, no server-side requirement) — just how that's
  achieved.
- Not distributing the bundled/inlined file via git — it's a build
  output, produced on demand (`task build:mq-proxy-web`), same as
  `tui`'s binary isn't committed either.

## Data & config

```
mq-proxy-web/
  src/               # ES modules (api.js, connection.js, queues.js, ...)
  src/*.test.js       # co-located tests, import from src/*.js directly
  index.html          # template (dev-mode module script + build placeholder)
  package.json         # esbuild devDependency, "type": "module"
  build.mjs (or similar) # esbuild invocation + HTML inlining
  README.md
```

New `.gitignore` entries: `mq-proxy-web/node_modules/`,
`mq-proxy-web/dist/` (or wherever the build step writes the generated
`index.html`).
