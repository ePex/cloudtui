# Demo recordings

The GIFs in the root README are recorded here, against an **example
instance** that holds only made-up data: a throwaway ActiveMQ broker in a
container, generic queues and messages (`orders.created`,
`dlq.orders.created`, …), the repo's example snippets, and a separate
`HOME` with a single connection called `example`. Nothing from your own
broker, config or AWS/Datadog accounts is involved.

## Regenerating them

Needs podman or docker, Go, python3, tmux, and
[agg](https://github.com/asciinema/agg) (`brew install agg`).

```sh
docs/demo/setup.sh          # build the example instance (/tmp/cloudtui-demo)
docs/demo/record.py         # record all GIFs into docs/demo/
docs/demo/setup.sh --down   # remove the broker container and /tmp/cloudtui-demo
```

- `record.py` drives the real TUI in a tmux session, samples the screen
  about ten times a second, and renders the frames with agg. It needs no
  browser or network. `record.py <name>` records a single scenario, but
  the scenarios change the broker's data as they go (sending, requeuing),
  so a fresh `setup.sh` followed by all of them gives the intended result.
- The scenarios are plain lists of key presses in `record.py`. Adjust
  one and re-run.
- The demo `HOME` and files live under `/tmp/cloudtui-demo` on purpose:
  the app shows full paths in some places (e.g. after an import), and a
  path under your own home folder would put your username in the GIF.
- Before committing new GIFs, check them for anything that isn't example
  data. For instance, the theme scenario uses `:theme <name>` rather than
  the theme picker, so the full list of bundled themes isn't shown.
