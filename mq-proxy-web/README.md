# mq-proxy-web

A standalone, static AMQ web console: browse ActiveMQ queues and messages,
and perform purge/move/send/delete, from an ordinary web browser — no
install, no TUI. It talks directly to a running
[`mq-proxy`](../mq-proxy) instance's REST API (see that module's README
for the wire contract).

## Using it

The distributable is a **single, self-contained file**: `dist/index.html`
(built — see "Building it" below, not committed to the repo).

- **Double-click `dist/index.html`** to open it directly via `file://`.
  No server needed.
- Or serve `dist/` over `http(s)://` with any static file server, e.g.:
  ```sh
  npx serve mq-proxy-web/dist
  # or
  python3 -m http.server --directory mq-proxy-web/dist
  ```

Either way, enter the `mq-proxy` base URL and its HTTP Basic credentials
(`proxy.auth.username`/`password` — see `mq-proxy/README.md`) on the
connect screen. **`mq-proxy` needs to allow this page's origin via CORS**
— see `mq-proxy/README.md`'s `CORS_ALLOWED_ORIGINS` / a locally-run
instance's default already allows `file://` pages out of the box.

Connection details are remembered in the browser's `localStorage` across
visits ("Disconnect" clears them). This is a convenience/security
trade-off worth knowing: credentials sit in the browser and are sent as
HTTP Basic (base64, not encrypted) on every request — serve `mq-proxy`
over HTTPS for any real deployment.

## Building it

```sh
task build:mq-proxy-web
# or, from mq-proxy-web/:
npm ci && npm run build
```

Needs Node + npm (only for building — using the already-built
`dist/index.html` needs neither). `esbuild` bundles `src/main.js` and
inlines the result into `index.html`'s `<script>` tag, producing
`dist/index.html`.

### Developing

`src/main.js` is loaded as a real ES module
(`<script type="module" src="./src/main.js">` in the checked-in
`index.html`), so you can iterate directly against the source — no
build step needed to see a change, just serve the folder:

```sh
npx serve mq-proxy-web
# or
python3 -m http.server --directory mq-proxy-web
```

(`file://` won't work for this — Chrome blocks ES module loading as a
cross-origin fetch over `file://`. That's exactly the problem the build
step exists to solve for the shipped artifact; it doesn't matter for
local development served over `http://`.) Run `task build:mq-proxy-web`
once you're done to produce the real distributable.

## Source layout

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
    *.test.js       # co-located tests, import directly from the module
                     # next to them
  index.html        # dev template: markup + <style>, <script type="module">
  package.json      # esbuild devDependency
  build.mjs         # bundles + inlines src/main.js into index.html
  dist/             # build output (gitignored) — dist/index.html is
                     # the actual distributable
```

## Tests

Pure logic (auth header, response-envelope parsing, query building,
purge/move-all filter branching, DLQ move-target ordering) is covered by
the `src/*.test.js` files, using Node's built-in test runner and native
ES module `import` — no npm install needed just to run them:

```sh
node --test
# or, from the repo root:
task test:mq-proxy-web
```

DOM rendering and click-handling are **not** unit tested; see
`spec/21-amq-web-console` for the manual end-to-end test pass changes to
this page need instead.

## Why the source is many files but the distributable is one

A single self-contained `index.html` (markup + `<style>` + `<script>`
inline) was the original shape of this page, and it's still what end
users get — but authoring the *logic* as one inline `<script>` block
couldn't be unit tested (nothing to `import` out of it), which is why an
early version used a single `app.js` with a small UMD-lite wrapper
instead, working around the lack of a build step.

The current shape resolves that the more standard way: `src/*.js` are
real ES modules, imported directly by their co-located tests — no
wrapper needed. A build step (`esbuild`, see "Building it" above)
bundles them into a single classic `<script>` (not `type="module"` —
that's what keeps the *built* file safe to open via `file://`, same
constraint the old UMD wrapper was working around) and inlines it into
`dist/index.html`. Source stays multi-file and testable; only the
generated artifact is single-file.
