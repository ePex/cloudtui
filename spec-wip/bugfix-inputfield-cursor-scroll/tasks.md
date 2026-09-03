# Tasks

1. [x] Add `ui.SetInputFieldText(field *tview.InputField, text string)`
   to new file `tui/internal/ui/inputfield.go` (doc comment explains
   the `tview` v0.42.0 bug and why a synthetic `KeyEnd` keypress fixes
   it — see `plan.md`). Add `tui/internal/ui/inputfield_test.go`: a
   narrow field + long value, drawn onto a `tcell.SimulationScreen`,
   asserting `screen.GetCursor()` reports the cursor visible; a control
   case with a short value.

   One design detail not in `plan.md`: `TextArea.lastWidth` (used by
   `findCursor`'s clamp math) is only ever set inside `Draw()` — a
   field populated *before* its first `Draw()` never hits this bug at
   all, since `Draw()`'s own first-time handling resolves and clamps
   the cursor itself in that case. The test draws the field once
   before populating it, matching the real scenario (an already-drawn
   field gets repopulated, e.g. reopening the connection editor). A
   third test (`TestInputFieldSetTextHidesCursorForOverflowingValue`)
   pins the upstream bug itself as a regression signal for the
   workaround's continued necessity. Full suite (`go build ./... && go
   vet ./... && go test ./...`) passes.

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
