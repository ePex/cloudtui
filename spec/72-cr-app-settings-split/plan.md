# Plan — CR 72: split `settings.go`

## Approach

Cut everything from `// themePicker is the theme-picker overlay...`
through the end of the file (the `themePicker` type, `newThemePicker`,
`show`, `close`, `ApplyPalette`, `var _ ui.Themeable = (*themePicker)(nil)`)
verbatim into a new `internal/app/themepicker.go`, with its own
`package app` header and only the imports it actually uses
(`github.com/gdamore/tcell/v2`, `github.com/rivo/tview`,
`github.com/ePex/cloudtui/tui/internal/config`,
`github.com/ePex/cloudtui/tui/internal/ui` — all four, confirmed by
what `themePicker`'s code references: `tcell.GetColor`/`tcell.EventKey`,
`tview.NewList`/`tview.NewFlex`, `config.AvailableThemes`/
`config.Palette`, `ui.Host`/`ui.StyleList`/`ui.Themeable`).

`settings.go` keeps its existing `package app` header and trims its
import list to only what `settingsView`/`refreshSettingsList`/
`datadogSettingsLabel` still use after the cut (`fmt`,
`github.com/gdamore/tcell/v2`, `github.com/rivo/tview`,
`github.com/ePex/cloudtui/tui/internal/config`,
`github.com/ePex/cloudtui/tui/internal/ui`) — likely unchanged, since
both halves use largely the same import set, but confirmed by letting
the compiler flag anything actually unused rather than guessing.

## Files touched

- New: `internal/app/themepicker.go`
- Modified: `internal/app/settings.go` (shrinks)

## Key decisions

- **No behavior change, no test moves** — confirmed in spec.md's
  background that `settings_test.go` doesn't reference `themePicker`.
- **No new dependencies.**

## Definition of done

Unchanged from spec.md — `go build`/`go test` pass, `themePicker` and
its methods live in `themepicker.go`, `settings.go` only defines
`settingsView`-related code.
