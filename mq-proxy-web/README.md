# mq-proxy-web

A standalone, static AMQ web console: browse ActiveMQ queues and messages,
and perform purge/move/send/delete, from an ordinary web browser — no
install, no build step, no TUI. It talks directly to a running
[`mq-proxy`](../mq-proxy) instance's REST API (see that module's README
for the wire contract).

## Running it

Just two files, `index.html` and `app.js` (co-located — both need to be
present together).

- **Double-click `index.html`** to open it directly via `file://`. No
  server needed.
- Or serve the folder over `http(s)://` with any static file server, e.g.:
  ```sh
  npx serve mq-proxy-web
  # or
  python3 -m http.server --directory mq-proxy-web
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

## Tests

Pure logic (auth header, response-envelope parsing, query building,
purge/move-all filter branching, DLQ move-target ordering) is covered by
`app.test.js`, using Node's built-in test runner — no npm dependencies,
no `package.json`, no build step:

```sh
node --test
# or, from the repo root:
task test:mq-proxy-web
```

DOM rendering and click-handling are **not** unit tested; see this
feature's `spec-wip` tasks (or `spec/21-amq-web-console` once merged) for
the manual end-to-end test pass this needs instead.

## Why two files, not one

A single self-contained `index.html` (markup + `<style>` + `<script>`
inline) was the first instinct, but it can't be unit tested — there'd be
no way to import logic out of an inline `<script>` block. `app.js` is
loaded as a **classic** `<script src="app.js">`, not an ES module — Chrome
blocks ES module loading over `file://` as a cross-origin fetch, which
would break the double-click-to-open case above. Instead, `app.js` uses a
small UMD-lite wrapper so the same file works as a browser global *and* as
a Node `require()`-able CommonJS module for `app.test.js`, without a
bundler.
