# Bugfix: JMS Type fields lose typed text to the type scan

Date: 2026-10-10

Found while live-verifying bugfix-purge-completeness (PR #40); agreed
there to fix separately. Three defects around the JMS Type autocomplete
shared by the purge/move-all prompt (`JMSTypePrompt`, spec/09) and the
message filter (`MessageFilter`, spec/08) — two lose what the user typed,
one is a data race in their tests.

## Problems

### 1. The prompt's automatic scan wipes text typed while it runs

`JMSTypePrompt.Show` starts an automatic scan of up to 500 messages
(spec/09). When it finishes, `handleScanResult` clears the field
unconditionally — its comment assumes the field is untouched during the
automatic scan, which isn't true if the user starts typing straight
away. Reproduced live: press `p`, type a type immediately, and the text
disappears about a second later when the scan completes.

Only the prompt has an automatic scan; the message filter's scan only
starts on request (below), so this doesn't affect it.

### 2. Enter or Tab start a scan instead of accepting the text

Both fields always append the scan entry ("↻ Scan up to 5000 messages
for JMS types") as the last suggestion. While the drop-down is open,
`tview.InputField` treats **both Enter and Tab** as "accept the
highlighted entry" (tview v0.42 `inputfield.go`, "Intentional
selection"), before the field's own Enter/Tab handling runs. And when it
rebuilds the drop-down it keeps whichever entry was highlighted before —
the scan entry is the only entry on the very first build, so it stays
highlighted from then on unless the user moves the highlight with the
arrow keys. Confirmed with a throwaway unit-level probe (`TypeA` among
the suggestions):

| Field | Typed | Key | Result |
|---|---|---|---|
| Prompt | (nothing) | Tab | scan starts, field holds the scan entry |
| Prompt | `TypeB` | Tab | scan starts |
| Prompt | `Type` (matches `TypeA`) | Enter | scan starts — not `TypeA` |
| Filter | (nothing) | Tab or Enter | scan starts |
| Filter | `TypeB` | Tab or Enter | scan starts |

When the scan finishes the field is cleared, so the typed text is lost
and, in the prompt, the purge/move-all doesn't continue. In the message
filter this also means Tab can't move past the JMS Type field without
starting a scan. Which entry is highlighted depends on history, so the
outcome varies — a live attempt with `TypeA` + Enter once worked.

The prompt already intercepts Enter on an *empty* field (spec/09:
"blank + Enter proceeds with no filter"); nothing protects Tab, or
Enter on a non-empty field, in either field.

### 3. `go test -race ./internal/dialog/` fails

`TestDialogsFullyRecolorOnLiveThemeSwitch/JMSTypePrompt` opens the
prompt twice; each `Show` starts the automatic scan. The test host's
fake `ScanJMSTypes` returns at once, and its `QueueUpdateDraw` runs the
result handler inline on the scan's goroutine — concurrently with the
test drawing and re-theming the prompt. 10 data races, all in that one
subtest; every other TUI package passes under `-race`. It's test-only:
in the app, `QueueUpdateDraw` runs the handler on the UI goroutine. CI
runs plain `go test ./...` (no `-race`), so it went unnoticed.

## Fix (expected behavior)

- **Text typed during the prompt's automatic scan stays.** When the
  scan finishes, its types become suggestions for whatever is typed;
  the field's text is left alone. Only the scan entry's own text, left
  in the field when the user chose "↻ Scan up to …", is cleared when
  that scan finishes (unchanged).
- **Enter and Tab never start a scan on their own.** The scan only
  runs when the user deliberately chooses the scan entry: moves the
  highlight onto it with the arrow keys and presses Enter (or Tab), or
  clicks it.
- **Enter/Tab accept a suggestion only when the user chose it**: moved
  the highlight onto it with the arrow keys since last typing (or
  clicked it). Because tview's default highlight depends on history, a
  suggestion that merely happens to be highlighted is never accepted —
  so a partly typed `Type` no longer sometimes completes to `TypeA`;
  arrow down to pick it. (Plan-stage refinement, 2026-10-10.)
- **Enter/Tab otherwise keep the typed text**, in both fields:
  - Prompt: Enter continues the purge/move-all with the typed text
    (blank = no filter, as today); Tab keeps the text in the field.
  - Message filter: Enter and Tab keep the typed text and move on as
    the form normally does, so Apply filters by it.
- Unchanged: choosing the scan entry runs the wider scan and clears its
  own text when done; Esc closes the drop-down or the dialog as today.
- **`go test -race ./internal/dialog/` passes.** Mostly a test fix (the
  automatic scan must not complete concurrently with it). *Amended
  2026-10-10 (task 1):* with that fixed, a second race showed up that is
  real: `startScan`'s goroutine read `jp.queueName` lazily, while
  reopening the prompt (`Show`) writes it. The scan now copies the queue
  name on the UI goroutine before starting.
- **The message filter always opens with focus on the JMS Type field.**
  *Added 2026-10-10 (found in task 4):* it reopened with focus wherever
  it was left — e.g. on Apply after applying — so typing straight away
  went nowhere. `tview.Form` remembers its focused item across `Show`
  calls (tui/CLAUDE.md), and `MessageFilter.Show` didn't reset it.
- **CI runs the TUI tests with `-race` on the Ubuntu job**, so a data
  race fails the build. One platform is enough to catch races; macOS
  and Windows keep running plain `go test`. (Decided 2026-10-10.)

## Out of scope

- `-race` on the macOS/Windows CI jobs (needs a C toolchain there,
  unchecked; one platform suffices).
- Any change to what the scans find or how many messages they scan
  (spec/08, spec/09).
- The send-message overlay's JMS Type field — a plain text field with no
  autocomplete (spec/09).
