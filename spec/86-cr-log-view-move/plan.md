# Plan — CR 86: move `log.go` into `internal/view`

## Approach

Same staged strategy CR 84/85 used: do everything except the physical
move while the file still lives in `internal/app`, verified
incrementally; the move itself is the last, purely mechanical step.
Simpler than CR 85 throughout — no dialogs, no `ui.ViewHost` adoption
needed at all, since the host reference is being dropped, not kept.

### 1. `LogView`'s new shape — drop the unused host reference

```go
// LogView is the Log screen: a scrollable read-only tview.TextView that
// displays the contents of ~/.cloudtui/cloudtui.log. It reloads on Activate
// (navigation) and on the 'r' shortcut.
type LogView struct {
	textView *tview.TextView
	path     string
}

var _ ui.View = (*LogView)(nil)
var _ ui.Shortcuttable = (*LogView)(nil)
var _ ui.Themeable = (*LogView)(nil)

func (lv *LogView) Name() string               { return "log" }
func (lv *LogView) Title() string              { return "Log" }
func (lv *LogView) Primitive() tview.Primitive { return lv.textView }

func (lv *LogView) Shortcuts() []ui.Shortcut {
	return []ui.Shortcut{
		{Key: "r", Description: "refresh"},
	}
}

// NewLogView constructs the log view, defaulting to ~/.cloudtui/cloudtui.log.
func NewLogView() *LogView {
	home, _ := os.UserHomeDir()
	return NewLogViewWithPath(filepath.Join(home, ".cloudtui", "cloudtui.log"))
}

// NewLogViewWithPath constructs the log view reading from path. Used by tests.
func NewLogViewWithPath(path string) *LogView {
	tv := tview.NewTextView()
	tv.SetBorder(true).SetTitle(" Log ")
	tv.SetScrollable(true).SetDynamicColors(true).SetWrap(true).SetWordWrap(true)

	lv := &LogView{textView: tv, path: path}

	tv.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Rune() == 'r' {
			lv.load()
			return nil
		}
		return event
	})

	return lv
}
```

`Activate`, `load`, `colorizeLog`, `logLevelColor`, `ApplyPalette` move
unchanged (only their receiver's type name changes, `logView` →
`LogView` — no body edits; none of them ever referenced `lv.app`).

### 2. `app.go` changes

**Construction call** — no reordering needed (unlike CR 85: nothing is
being added as a dependency, one is being removed). Same line,
simplified:

```go
a.logV = view.NewLogView()
```

**Struct field**: `logV *logView` → `logV *view.LogView` (app.go:60).

**`a.views` literal** (app.go:284): `a.logV` already refers to the
field, not a local var — only its declared type changes, the literal
itself (`[]ui.View{homeView, a.settingsV, a.logV, ...}`) is untouched.

**`a.themables` literal** (app.go:386): same — `a.logV` entry
untouched, only the field's declared type changes.

### 3. Tests — split by what each actually exercises

`log_test.go` has 6 tests today. 3 construct a real `a := New(config.Default())`
and read through `a.logV` — these test `app.go`'s own wiring (does
`New()` register the log view under name `"log"`, does it implement
`ui.Shortcuttable`, does its shortcut list include `"r"`), not
anything `LogView`'s own logic does differently. The other 3 construct
`LogView` directly via `newLogViewWithPath` and never touch `*App` —
pure `LogView` behavior.

**Move to `internal/view/log_test.go`, adapted for the dropped
parameter**:

```go
func TestLogViewActivateWithMissingFile(t *testing.T) {
	lv := NewLogViewWithPath(filepath.Join(t.TempDir(), "nonexistent.log"))
	lv.Activate()
	if got := lv.textView.GetText(true); !strings.Contains(got, "No log file") {
		t.Errorf("Activate() with missing file: text = %q, want 'No log file' message", got)
	}
}
```

(`TestLogViewActivateWithMissingFile`, `TestLogViewActivateLoadsFile`
— both just drop the `a` argument to `NewLogViewWithPath`, no other
change.) `TestLogLevelColor` and `TestColorizeLogEscapesLiteralBrackets`
port unchanged (already package-level function tests, no `*App`
involved). `TestColorizeLogWrapsRecognizedLevels` same.

**Stay in `internal/app`, relocated to `viewwiring_test.go`** (matches
CR 84's precedent — "does app.go wire this view correctly" tests live
there, not beside the view's own logic):
`TestLogViewName` (renamed `TestLogViewIsWiredAsLogPage` for clarity,
same body), `TestLogViewImplementsShortcuttable`,
`TestLogViewShortcutsIncludeR` — all 3 unchanged internally, since
`a.logV` still satisfies the same interfaces through the same field
access; only their file location changes.

No new tests needed — this CR is a pure relocation, and both halves of
the existing 6 tests already cover exactly the same things they did
before the split.

### 4. The physical move

Once (1)–(3) leave `log.go` self-contained (already true today except
for the dead `app` field, which step 1 removes), `git mv
internal/app/log.go internal/view/log.go` + `git mv
internal/app/log_test.go internal/view/log_test.go` (containing only
the 3 pure-`LogView` tests after step 3's split — the other 3 move to
`internal/app/viewwiring_test.go` directly, not through a temporary
file), `package app` → `package view`, `app.go`'s field type and
construction call gain the `view.` qualifier.

### 5. Verification order

Step 1 (the view's own new shape, still `package app`) → step 2
(`app.go` updated to match) → step 3 (tests split, still `package
app`) → step 4 (the move). `gofmt -l`/`go build ./...`/`go vet
./...`/`go test ./...` after each step. Final repo-wide pass, then
live verification.

## Files touched

- `internal/app/log.go` → `internal/view/log.go` (moved, `app` field
  dropped per step 1).
- `internal/app/log_test.go` → `internal/view/log_test.go` (3 of 6
  tests, adapted for the dropped constructor parameter).
- `internal/app/app.go` (field type, construction call — both
  one-line changes; `a.views`/`a.themables` literals untouched).
- `internal/app/viewwiring_test.go` (gains the other 3 tests,
  relocated unchanged).

## Key decisions

- **Drop the `app`/host field instead of swapping it for `ui.ViewHost`**
  — per spec.md's Problem section: it's provably dead code today (zero
  reads), and CR 82's `ui.ViewHost` adoption exists to let a view reach
  its host, which `LogView` never needs to do. Adding an unused `host
  ui.ViewHost` field would just be dead code with extra ceremony.
- **No `onBack`-style callback, no dialog fields** — `LogView` has
  neither sibling-view navigation (no detail view of its own) nor
  dialog coupling, so none of CR 82's other adoption machinery applies
  here at all.
- **Test split by construction shape, not a blanket move** — mirrors
  CR 84/85's existing precedent (`viewwiring_test.go` for "does app.go
  wire this correctly", the view's own test file for "does the view's
  own logic work") rather than moving all 6 tests together and leaving
  3 of them awkwardly reconstructing a full `*App` from inside
  `internal/view`, which isn't possible without an import cycle.
- **No new coverage** — this CR changes no behavior and removes no
  tested path (the `app` field was never asserted on by any test
  either), so the existing 6 tests, split but otherwise unchanged, are
  sufficient.

## Definition of done

Unchanged from spec.md — `internal/view/log.go` holds `LogView`,
`NewLogView`, `NewLogViewWithPath` with no App/host parameter
anywhere; `internal/app` has no `logView`/`newLogView`/
`newLogViewWithPath` symbols; `go build`/`go test`/`go vet` clean,
`gofmt -l` clean, zero import cycle; all 6 existing tests pass,
split per step 3; live verification confirms no behavior change.
