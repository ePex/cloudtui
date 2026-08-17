# Tasks — CR 72: split `settings.go`

1. [x] Create `internal/app/themepicker.go` with `themePicker`,
   `newThemePicker`, `show`, `close`, `ApplyPalette`, and
   `var _ ui.Themeable = (*themePicker)(nil)`, moved verbatim from
   `settings.go`, with its own import block. Remove the same from
   `settings.go`, trimming its import list to whatever the compiler
   flags as unused. `gofmt -l`, `go vet ./...`, `go build ./...`,
   `go test ./...` all clean.

2. [x] Final verification pass: confirm `settings.go` defines no
   `themePicker`-related symbol (`grep -n 'themePicker' tui/internal/app/settings.go`
   returns nothing); `gofmt -l tui/` and `go vet ./...` clean
   repo-wide; `go build ./...` and `go test ./...` pass repo-wide. No
   commit needed unless this surfaces something to fix.

   Note: `settingsView`'s own code still legitimately calls
   `a.themePicker.show()` (×2, opening the picker) — those are field/
   method *references*, not the type/constructor/method *definitions*,
   which are the actual thing checked. Confirmed via `grep -n 'type
   themePicker\|func newThemePicker\|func (tp \*themePicker)'`
   returning nothing in `settings.go`. All checks clean.
