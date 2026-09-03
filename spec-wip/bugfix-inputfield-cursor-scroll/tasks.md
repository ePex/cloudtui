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

2. [ ] Switch the 15 `dialog`-package call sites to
   `ui.SetInputFieldText`: `connections.go` (Name, Broker Name, URL,
   Username, AWS Profile, Secret Name, Password — 7),
   `datadogsettings.go` (Site, Access Token — 2), `messagefilter.go`
   (JMS Type, From Date, To Date, Max Count — 4), `timerangemodal.go`
   (From, To — 2). Leave the field-clearing (`SetText("")`) and
   `*tview.TextView` call sites untouched (see `plan.md`).

3. [ ] Switch the 9 `view`-package filter/search-input call sites to
   `ui.SetInputFieldText`: `logsearch.go` (2), `datadoglogs.go` (1),
   `messages.go` (1), `logs.go`, `queues.go`, `ssmparams.go`,
   `secrets.go`, `codepipelinelist.go` (1 each).

   **Manual verification** (`task run:tui`): open the connection
   editor on an existing connection whose Name/URL/Secret Name is
   longer than the field's width — confirm the cursor is now visible
   at the end of the value instead of hidden. Spot-check at least one
   filter input (e.g. Queues view's filter) with a long restored
   filter string.
