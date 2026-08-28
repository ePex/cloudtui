---
name: verify-live
description: Live-verify a cloudtui change against a real ActiveMQ broker (Jolokia and/or mq-proxy backends) by driving the actual TUI binary in tmux, instead of trusting unit tests alone for UI/broker-interacting behavior. Use before checking off a tasks.md item for any feature/bugfix/CR that touches queue, message, or connection behavior.
---

# Verify a cloudtui change live

Unit tests catch logic bugs. They do not catch what a real `tview` screen
actually renders, or what a real broker actually does. Several bugs in this
project were only caught by driving the real binary against a real broker
(a `tview.Table` silently swallowing `"[x]"` as a color tag; a queue list
that scrolled to the bottom instead of the top on first load; a confirm
dialog rendering invisibly under another overlay). Use this skill instead of
declaring a queue/message/connection change done from tests alone.

**Already-covered golden path?** `task smoke:test` (`tui/scripts/smoke-test.sh`)
automates exactly this kind of driving for the core path — list queues,
seed/browse/mark/delete/move messages, switch to the proxy backend and
confirm it sees the same broker state — against disposable, uniquely-named
queues it creates and removes itself, with `config.yaml` backed up and
restored around it. Run it as a quick regression sanity check, or read it as
a worked example of the patterns below. It does **not** replace verifying a
*new* feature's specific behavior by hand — a script's assertions only
catch regressions of things its author already knew to check for.

## Before you start: know what's on the broker

The default local broker (`localhost:8161`, Jolokia, `admin`/`admin`) may
have real, non-disposable queues on it (this repo's dev broker has ended up
with business-looking queue names like `foo.*`/`bar.*`/`baz.*` at
various points — don't assume a fresh empty broker). Rules:

- **Never purge, delete-all, or blind-move against a queue you didn't create
  or confirm is test data.** Browse it first if you're not sure.
- Use `orders` as the default scratch queue (it's referenced in
  `mq-proxy/requests.http` as the project's example queue) or create your
  own disposable queue with `task test:queue:add -- <name>` (removed after
  with `task test:queue:remove -- <name>`).
- If a queue already has messages you didn't expect, check whether they
  match this project's own sample-message shape (`{"id":N,"event":"order.*","customer":"..."}`,
  from `task seed:queue`) before treating them as safe to delete — that
  usually means an earlier verification session (yours or a teammate's)
  left them there, not that they're unrelated real data.

## Setup

```bash
cd tui
go build -o /tmp/cloudtui_verify ./cmd/cloudtui   # use your session's scratch dir if you have one, not shared /tmp under concurrent jobs
```

Seed test data (Jolokia only): `task seed:queue -- <queue> <count>` sends
`<count>` sample JSON messages.

Need a queue that doesn't exist yet: `task test:queue:add -- <name>` /
`task test:queue:remove -- <name>` (JMX `addQueue`/`removeQueue` — needed
because ActiveMQ's `sendTextMessage` requires the destination to already
exist, so you can't just seed a brand-new queue into existence).

## Driving the TUI via tmux

```bash
tmux new-session -d -s verify -x 130 -y 40 '/tmp/cloudtui_verify'
tmux send-keys -t verify 'h'          # a key
sleep 0.2                             # give the redraw a beat
tmux capture-pane -t verify -p        # read the screen
```

Cleanup: `tmux send-keys -t verify 'q'; tmux kill-session -t verify`.

### Key reference

| Key | Where | Action |
|---|---|---|
| `h` / `s` / `l` | global | Home / Settings / Log |
| `:` | global | command prompt |
| `?` | global | help |
| `/` | queues, messages, move picker | filter/search |
| `Enter` | queues list | open messages |
| `space` | messages list | mark/unmark cursor row, advances cursor |
| `a` / `n` | messages list | mark all / clear marks |
| `d` / `m` | messages list | delete / move marked (or cursor row if nothing marked) |
| `p` | queues, messages | purge (confirm dialog) |
| `n` / `e` / `d` / `Del`/`x` | connection manager | new / edit / duplicate / delete |
| `Esc` | every overlay | cancel/close |

### Verifying a confirm dialog or picker

After a key that opens `Confirm`/`Move to Queue`/etc., capture the pane and
read the dialog text before pressing anything else — the exact wording is
often the thing you're actually verifying (e.g. singular vs. plural
phrasing). To answer a confirm dialog: `Down` then `Enter` selects "Yes"
(cursor starts on "No").

### Filtering a picker to one item

`tmux send-keys -t verify '/'` then the search text, then `Enter` twice
(once to accept the filtered selection, once more only if a confirm follows).
Pick a filter string that's actually distinctive — `p` will match half the
queue names on a broker full of `foo`/`bar-proxy` queues; test with an
obviously-non-matching string like `zzz` first if you're not sure filtering
is even working, before concluding a feature is broken.

## Known tview gotchas (don't rediscover these)

- **`"[x]"` and other `[...]`-looking cell text gets silently swallowed** —
  `tview.Table` always parses `[...]` in cell text as a color/region tag,
  with no per-cell opt-out (unlike `TextView`, which has
  `SetDynamicColors`). If a marker/checkbox column renders blank, this is
  almost certainly why. Use glyphs with no bracket syntax (`✓`, `⭐`, etc.).
- **A `tview.Table` can silently scroll away from row 0 on first load.**
  `Select(1, 0)` resets the cursor but not the internal `trackEnd`
  auto-scroll flag (meant for tailing logs), which can latch `true` during
  the first empty-table draw and stick through the real data's repaint.
  Fix: `SetOffset(0, 0)` alongside `Select`, or `ScrollToBeginning()`.
- **A confirm dialog can render invisibly underneath another open overlay.**
  `tview.Pages` draws pages in `AddPage` order, later pages on top. Any
  page meant to always be frontmost (like `"confirm"`) needs to be added
  *last*.
- **`tview.Form` remembers focus index across `SetFocus` calls.** Reopening
  a form (e.g. the connection editor) doesn't reset focus to item 0 — it
  can land back on whatever button/field had focus when it last closed. If
  typed text isn't appearing where expected, type one throwaway character
  first and check which field it landed in before typing real data.

## Testing the proxy backend

`mq-proxy` requires **Java 21+** to run (the built jar's bytecode won't
load under Java 17, which is what a plain `java` on `PATH` may resolve to
via tools like sdkman — `task dev:proxy:start` resolves `$JAVA_HOME/bin/java`
explicitly for this reason, but only if `JAVA_HOME` is actually set to a
21+ JDK in your shell).

```bash
export JAVA_HOME=/path/to/a/jdk-21-or-newer   # if not already set correctly
task dev:proxy:start                          # builds + backgrounds it, waits until ready
# ... drive the TUI against it ...
task dev:proxy:stop
```

To point the TUI at it, add a connection via the app itself rather than
hand-editing `config.yaml`: `s` → Down → `Enter` (Connection) → `n` (new) →
fill Name/Alias, switch Backend to `proxy` (Enter, Down, Enter), URL
`http://localhost:8080`, Username `cloudtui`, Password `changeme` (mq-proxy's
defaults) → Tab to Save → `Enter` → select the new connection → `Enter` to
activate. Remember the Form-focus gotcha above while typing.

mq-proxy's `BrowseMessages` always returns a real message ID (unlike
Jolokia's occasional "limited info" fallback), so anything depending on
`Message.ID` being present should be *more* permissive against the proxy
backend, not less.

## Cleanup checklist

- Purge/remove only the queues you created or confirmed are test data.
- If you added a test connection via the connection manager, delete it
  (`d`/`x` in the manager) so `config.yaml` (gitignored, but still your
  real local file) ends up back where you found it.
- `task dev:proxy:stop` if you started mq-proxy.
- `tmux kill-session` for any session you created.
- Remove any scratch Go files/binaries you created for one-off checks.
