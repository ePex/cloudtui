# Tasks — FE 32: AWS Systems Manager Parameter Store integration

Plan: [plan.md](plan.md)

Each task needs separate approval before it's implemented — see
`CLAUDE.md`.

1. [x] Add `github.com/aws/aws-sdk-go-v2/service/ssm` to `tui/go.mod`
   (`go get` + `go mod tidy`); confirm `go build ./...` still passes with
   no code using it yet.
2. [x] `tui/internal/awsssm/awsssm.go`: `ParameterType`/`Parameter` types,
   `List` (paginated `GetParametersByPath`, `WithDecryption: false`,
   populating `Value` for `String`/`StringList` and leaving it empty for
   `SecureString`), `Reveal` (single `GetParameter` with
   `WithDecryption: true`).
3. [x] `awsssm` tests: the pagination-loop and type-split logic factored
   into pure functions that take an already-fetched page and build
   `[]Parameter` (per plan.md's testing note — the AWS-calling functions
   themselves aren't unit tested, same precedent as `devtool`'s
   `StartProxy`/`StopProxy`).
4. [x] `App.listParameters` field (defaulting to `awsssm.List`) — same
   dependency-injection shape as `listAWSProfiles`.
5. [x] New view `tui/internal/app/ssmparams.go`: table + filter input
   (mirrors `queuesView`), registered as `ui.View` + `ui.Shortcuttable`,
   added to `a.views` and Home's "Apps" section. Errors clearly if
   `cfg.ActiveAWSProfile` is empty rather than calling `awsssm` with one.
6. [x] Detail view for a selected parameter, reusing
   `messageDetailView`'s label/value rendering shape: immediate value for
   `String`/`StringList`; "encrypted — press `r` to reveal" +
   async-`Reveal`-on-`r` for `SecureString`.
7. [x] Tests for the view layer: construction, header, filter, the
   no-active-profile error path, and the detail view's two display modes
   — all via the injected `listParameters`/a `revealParameter` field, no
   real AWS calls in tests.
8. [x] `go build ./...`, `go vet ./...`, `go test ./...`.
9. [x] Manual verification per `verify-live`, against this machine's real
   active AWS profile: list loads, a `String`/`StringList` value shows
   immediately, a `SecureString` stays masked until revealed. No
   parameter values — especially decrypted ones — get pasted into any
   commit message or shown beyond confirming the mechanism works.
10. [x] `c` shortcut in the detail view to copy the shown value to the
    system clipboard via `tcell.Screen.SetClipboard` (gated on a value
    actually being present, so a no-op/hidden for an unrevealed
    `SecureString`); `App` captures its `tcell.Screen` once via
    `SetAfterDrawFunc`. Unit-tested with `tcell.NewSimulationScreen`
    (its `SetClipboard`/`GetClipboardData` are real in-memory, no
    terminal needed). Live-verified via tmux: the `<c> copy value`
    shortcut appears in the context panel exactly when a value is on
    screen, pressing it updates the status bar to name the parameter
    (never the value), and disappears/no-ops for an unrevealed
    `SecureString`. The actual OS-level clipboard write (OSC 52) could
    not be confirmed end-to-end this way — a detached tmux session has
    no attached terminal to interpret the escape sequence into a real
    paste-buffer write — so that link is covered by the
    `SimulationScreen`-based unit tests instead, which assert the exact
    bytes reach `Screen.SetClipboard`.

## Bugs found during live verification (task 9)

Both were pre-existing defects surfaced by driving the real binary, not
regressions introduced by this feature — see commit history for the
fixes.

- **Global-hotkey/filter-input focus collision.** Typing into the new
  SSM Parameters filter input triggered global single-key shortcuts
  (e.g. navigating to the Log view) mid-keystroke, because
  `onGlobalKey` only exempted overlay-tracked filter inputs (via their
  `*Visible` bool) and had no exemption for a regular top-level view's
  filter input. Fixed with an explicit `a.ssmParamsV.filterInput` check
  in `onGlobalKey`, matching the existing `a.queuesV.filterInput`
  exemption; covered by
  `TestOnGlobalKeyPassesThroughWhenSSMParamsFilterFocused`.
- **`tview.Box` titles swallow `[...]` as color tags.** The filtered
  title (`" SSM Parameters [text] "`) never visibly rendered the
  bracketed filter text, because `tview.Box.Draw()` runs the title
  through the same tag-parsing `Print()` that `Table` cells use (the
  same defect class as the `messages.go` checkbox issue from spec 24).
  `GetTitle()` still returns the literal stored string, so no existing
  test caught it. This was pre-existing in `queuesView` and
  `awsProfiles` too, not something new — reproduced there first to
  confirm. Fixed by switching the filtered-title format from
  `[%s]` to `(%s)` in `queues.go`, `awsprofiles.go`, and `ssmparams.go`.
  Added `renderedScreenText` (draws to a real `tcell.SimulationScreen`
  and reads back `GetContents()`, since `GetTitle()` alone can't catch
  this class of bug) plus render-based regression tests in
  `queues_test.go`, `ssmparams_test.go`, and `awsprofiles_test.go`.

Also verified live: no `SecureString` parameters exist under `/` in the
profile used for testing, so the reveal flow's UI mechanism (masked
display, `r` to reveal, async fetch + redraw) is verified by
`paramdetail_test.go`'s unit tests only, not by an end-to-end reveal
against a real encrypted value. This is consistent with the plan's
testing note that AWS-calling functions themselves aren't exercised by
live traffic in tests.
