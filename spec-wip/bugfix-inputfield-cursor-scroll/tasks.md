# Tasks

1. [x] Add `ui.SetInputFieldText(field *tview.InputField, text string)`
   to new file `tui/internal/ui/inputfield.go` (doc comment explains
   the `tview` v0.42.0 bug and why a synthetic `KeyEnd` keypress fixes
   it — see `plan.md`). Add `tui/internal/ui/inputfield_test.go`: a
   narrow field + long value, drawn onto a `tcell.SimulationScreen`,
   asserting `screen.GetCursor()` reports the cursor visible; a control
   case with a short value.

   **Correction, found via the user's own live testing after this task
   first shipped**: the initial implementation only fired the
   synthetic `KeyEnd` keypress, no throwaway draw. That's a no-op
   against a field that's never been drawn for real yet
   (`TextArea.lastWidth` stays `0`, and `moveCursor()` bails out
   completely in that case) — which is the *common* real-world case
   (editing any connection for the first time in a session), not the
   edge case. Worse, it left the cursor "resolved" in a way that
   permanently blocked `tview`'s own first-draw handling (which only
   ever resolves *position*, never scroll offset — it doesn't actually
   self-heal the way the first version of this doc comment claimed).
   Fixed by having `SetInputFieldText` establish `lastWidth` itself via
   a throwaway off-screen `Draw()` first — see the corrected `spec.md`
   and `inputfield.go`'s doc comment. Added
   `TestSetInputFieldTextShowsCursorForNeverDrawnField` (plus a
   dynamic-width variant) as the primary regression test — it fails
   against the old implementation, passes against the fix. Full suite
   (`go build ./... && go vet ./... && go test ./...`) passes. Also
   re-verified live via `tmux`/`cursor_flag` in a single fresh session
   (no priming), matching the user's exact repro: edit an existing
   connection, tab straight to a long field.

2. [x] Switch the 15 `dialog`-package call sites to
   `ui.SetInputFieldText`: `connections.go` (Name, Broker Name, URL,
   Username, AWS Profile, Secret Name, Password — 7),
   `datadogsettings.go` (Site, Access Token — 2), `messagefilter.go`
   (JMS Type, From Date, To Date, Max Count — 4), `timerangemodal.go`
   (From, To — 2). Leave the field-clearing (`SetText("")`) and
   `*tview.TextView` call sites untouched (see `plan.md`).

   Verified the 4 excluded raw `SetText` calls (`cm.hints`, `tm.tabs` —
   both `*tview.TextView` — and the two `SetText("")` clears in
   `messagefilter.go`) are the only ones remaining. Full `dialog`
   package test suite passes, no regressions.

3. [x] Switch the 9 `view`-package filter/search-input call sites to
   `ui.SetInputFieldText`: `logsearch.go` (2), `datadoglogs.go` (1),
   `messages.go` (1), `logs.go`, `queues.go`, `ssmparams.go`,
   `secrets.go`, `codepipelinelist.go` (1 each). Full test suite
   (`go build ./... && go vet ./... && go test ./...`) passes.

   **Manual verification**, driven live via `tmux` (`.claude/skills/
   verify-live/`): built the binary, drove the real TUI. tmux exposes
   the *actual* terminal's cursor state (`#{cursor_flag}`,
   `#{cursor_x}`/`#{cursor_y}`) — not just visible text — so this
   checks the exact thing `tview`'s `Draw()` decides, not an
   approximation.
   - Created a test connection, edited its Name to a value well over
     the field's 30-column width, saved, then **reopened the editor**
     (the actual repro: repopulating an already-drawn field
     programmatically) — the field showed the end of the value and
     `cursor_flag` reported visible, at the same `cursor_x` a real
     keystroke would land on. Before the fix this showed the start of
     the value with the cursor hidden. Deleted the test connection
     afterward.
   - Same check against the Queues view's filter (no fixed
     `SetFieldWidth` there — used a long-enough string to overflow the
     pane's width): applied a long filter, reopened it via `/` a
     second time (the same "restore last-applied filter text" path,
     `queues.go`) — cursor visible at the end, matching live typing.
     Cleared the filter afterward.
   - Switched the active connection to the local `default` (Jolokia)
     entry for this check rather than the real AWS-backed one already
     active, to avoid triggering AWS SSO; restored the original
     connection before quitting. `config.yaml` verified unchanged.
