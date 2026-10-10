# Plan: JMS Type fields lose typed text to the type scan

See `spec.md`. All code changes are in `tui/internal/dialog/`, plus CI.

## 1. One shared autocomplete wiring for both JMS Type fields

New file `tui/internal/dialog/jmstypeautocomplete.go` with a small
`jmsTypeAutocomplete` type that both `JMSTypePrompt` and `MessageFilter`
use for their JMS Type field, so the two can't diverge again. It owns:

- **`chosen bool`** — the user moved the drop-down's highlight with the
  arrow keys since they last typed. Set from tview's autocompleted
  callback with source `AutocompletedNavigate`; reset by the field's
  changed func (any text change) and when the dialog opens.
- **`SetAutocompleteFunc`** — the dialog's existing suggestion function
  (types, then the scan entry last), unchanged, except it returns
  nothing while `suppress` is set (below).
- **`SetAutocompletedFunc`** — tview's callback for a selection from the
  drop-down:
  - `AutocompletedNavigate` (arrow keys): set `chosen`, return `false`.
    The field text is **not** changed, so tview doesn't rebuild the
    drop-down — its rebuild only runs when the text changed during the
    key (`inputfield.go`'s deferred check). That's the risk the current
    code's comment cites for avoiding this API ("the suggestion list
    collapsing after the first arrow press"); it only applies when the
    callback changes the text.
  - `AutocompletedEnter`/`AutocompletedTab` (only reached when `chosen`,
    see the input capture) and `AutocompletedClick`: the scan entry →
    call the dialog's `onScan` (starts the wider scan) without touching
    the text; a real type → `SetText(type)`. Return `true` (closes the
    drop-down). This is tview's documented use of the callback (the
    caller sets the text).
- **`SetInputCapture`** — runs before tview's own key handling. On Enter
  or Tab when **not** `chosen`: close the drop-down, then let the key
  through, so it does what it does with no drop-down open — the
  prompt's done func (Enter continues, Tab does nothing) or the form's
  normal "next field". tview has no public "close drop-down", but
  `Autocomplete()` with no entries sets it to nil; so the capture sets
  `suppress`, calls `Autocomplete()`, clears `suppress`. When no
  drop-down is open this is a no-op.

**Consequence:** the scan entry's text never goes into the field any
more. That removes the reason for several workarounds, which go too:

- `onJMSTypeChanged` in both dialogs (sentinel detection via the changed
  func) — the scan now starts from the callback.
- `handleScanResult`'s `SetText("")` in both — nothing to clear. **This
  fixes defect 1**: text typed during the prompt's automatic scan stays.
  `handleScanResult` still calls `Autocomplete()` so the new types show
  for whatever is typed.
- `JMSTypePrompt`'s Enter-on-empty input capture — covered by the
  general rule (no drop-down choice → Enter continues).
- `continueNow`'s "still scanning" refusal (it checks for the scan
  entry's text in the field, which can't happen now) and
  `MessageFilter.apply`'s refusal while scanning (same reason: the field
  holds the user's text, so applying mid-scan is safe).

The styling order stays: `ui.StyleInputFieldAutocomplete` runs before
the helper calls `SetAutocompleteFunc` (tui/CLAUDE.md).

**Tests** (`jmstypeautocomplete_test.go`, plus the two dialogs' test
files), driving the real field through `InputHandler()` key events the
way the investigation's probe did:

- Both dialogs: untouched field + Tab/Enter, typed `TypeB` + Tab/Enter,
  typed prefix `Type` + Enter → no scan, text kept. Prompt Enter
  continues with the typed text (blank → no filter). Filter Tab moves
  form focus to the next item.
- Down onto a real type + Enter → that type is in the field; Down onto
  the scan entry + Enter → scan started, text unchanged; two Downs in a
  row still move within the drop-down (no collapse).
- Typing after navigating resets `chosen`.
- Defect 1: prompt `Show`, type while the automatic scan is blocked,
  complete it with `handleScanResult` → text still there, suggestions
  refreshed.
- Remove/replace existing tests that assert the old behavior (the
  sentinel-in-field, "refuse while scanning", Enter-on-empty capture).
- Mutation-check each new test against the old code path.

## 2. The racing test

`TestDialogsFullyRecolorOnLiveThemeSwitch`'s `JMSTypePrompt` case sets
`host.scanJMSTypesFn = blockForever` (already used for this in
`jmstypeprompt_test.go`) before opening the prompt, so the automatic
scan never completes concurrently with the test. No production change.
Verified by `go test -race ./internal/dialog/` passing.

## 3. CI runs the TUI tests with `-race` on Ubuntu

- `Taskfile.yml`: new `test:tui:race` (`go test -race ./...` in `tui/`),
  next to `test:tui` — spec/02 requires CI to use Taskfile commands, not
  ad hoc `go test`.
- `.github/workflows/ci.yml`, `tui` job: on Linux run `task
  test:tui:race` instead of `task test:tui`; macOS/Windows keep `task
  test:tui` (`if: runner.os == 'Linux'` / `!= 'Linux'`). `-race` needs
  cgo; `setup-go` on `ubuntu-latest` has gcc, so no extra setup.
- Verified by the PR's own CI run.

## 4. Specs (merge-back)

- `spec/09`: the prompt's key behavior (Enter/Tab rule, the scan entry
  only on deliberate choice); drop the implementation notes about the
  Enter-on-empty capture, the sentinel left in the field and the
  "still scanning" refusal; add the tview facts (Tab = Enter in an open
  drop-down; sticky highlight) and why `SetAutocompletedFunc` is safe
  here.
- `spec/08`: same for the message filter's JMS Type field; `apply()` no
  longer refuses during a scan.
- `spec/02`: CI runs `task test:tui:race` on Linux.
- `tui/CLAUDE.md` "tview gotchas": add Tab = Enter and the sticky
  highlight in autocomplete drop-downs.

## Trade-offs

- **Predictable over clever:** a suggestion is accepted only after
  arrow navigation. A partly typed `Type` no longer completes to
  `TypeA` on Enter — but that only ever worked by accident (sticky
  highlight), so nothing reliable is lost.
- **The scan entry stays in the list** rather than moving to a hotkey:
  it's discoverable where it is, and with this rule it can no longer
  fire by accident.
- **Closing the drop-down via `Autocomplete()` with no entries** relies
  on documented behavior ("no entries, no list"), not tview internals.

## Manual verification

Driving the real TUI (verify-live, local broker, isolated `HOME` and a
separate tmux server per the memory notes): in the purge prompt, type
immediately after `p` and confirm the text survives the scan; type an
unlisted type and press Enter → confirm dialog for that type; in the
message filter, type an unlisted type, Tab to Apply, Apply → title shows
the filter; arrow to the scan entry + Enter → scan runs.

No new dependencies.
