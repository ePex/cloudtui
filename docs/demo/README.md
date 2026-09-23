# Demo recordings

The GIFs in the root README are recorded here, against an **example
instance** that holds only made-up data: a throwaway ActiveMQ broker in a
container, generic queues and messages (`orders.created`,
`dlq.orders.created`, …), the repo's example snippets, and a separate
`HOME` with a single connection called `example`. Nothing from your own
broker, config or AWS/Datadog accounts is involved.

## Regenerating them

Needs podman or docker, Go, python3, and
[VHS](https://github.com/charmbracelet/vhs) **0.11.0**. VHS 0.12.0 exits
without writing the GIF
([vhs#787](https://github.com/charmbracelet/vhs/issues/787)), and
Homebrew ships 0.12.0, so install it with Go:
`go install github.com/charmbracelet/vhs@v0.11.0` (it also needs `ttyd`
and `ffmpeg`, e.g. `brew install ttyd ffmpeg`).

```sh
docs/demo/setup.sh          # build the example instance (/tmp/cloudtui-demo)
for t in queues send-snippet snippets dlq-requeue themes; do vhs docs/demo/$t.tape; done
docs/demo/setup.sh --down   # remove the broker container and /tmp/cloudtui-demo
```

- Each `<name>.tape` starts the app with the demo `HOME` (hidden), plays
  one scenario as key presses, and writes `docs/demo/<name>.gif`. Adjust
  a tape and re-run it. The scenarios change the broker's data as they
  go (sending, importing, requeuing), so a fresh `setup.sh` followed by
  all of them, in the order above, gives the intended result.
- VHS drives a headless browser, so it doesn't run in sandboxed shells.
- The demo `HOME` and files live under `/tmp/cloudtui-demo` on purpose:
  the app shows full paths in some places (e.g. after an import), and a
  path under your own home folder would put your username in the GIF.
- Before committing new GIFs, check them for anything that isn't example
  data. For instance, the theme scenario types `:theme <name>` while hidden
  rather than using the theme picker, so the full list of bundled themes
  isn't shown.
