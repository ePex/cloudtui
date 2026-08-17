# Tasks — CR 73: `Primitive()`/`Visible()` accessors

1. [x] Add `Primitive()`/`Visible()` to `confirmDialog` (`confirm.go`)
   per plan.md's template. `gofmt -l`, `go build ./...` clean (pure
   addition — nothing calls these yet).

2. [x] Add `Primitive()`/`Visible()` to `movePicker` (`movepicker.go`).
   `gofmt -l`, `go build ./...` clean.

3. [x] Add `Primitive()`/`Visible()` to `sendMessageOverlay`
   (`sendmessage.go`). `gofmt -l`, `go build ./...` clean.

4. [x] Add `Primitive()`/`Visible()` to both `connManager` and
   `connEditor` (`connections.go`). `gofmt -l`, `go build ./...` clean.

5. [x] Add `Primitive()`/`Visible()` to `messageFilter`
   (`messagefilter.go`). `gofmt -l`, `go build ./...` clean.

6. [x] Add `Primitive()`/`Visible()` to `timeRangeModal`
   (`timerangemodal.go`). `gofmt -l`, `go build ./...` clean.

7. [x] Add `Primitive()`/`Visible()` to `datadogEditor`
   (`datadogsettings.go`). `gofmt -l`, `go build ./...` clean.

8. [x] Add `Primitive()`/`Visible()` to `themePicker`
   (`themepicker.go`). `gofmt -l`, `go build ./...` clean.

9. [x] Add `Primitive()`/`Visible()` to `awsProfilesPicker`
   (`awsprofiles.go`). `gofmt -l`, `go build ./...` clean.

10. [x] Update `app.go`: retype `overlayVisible` to `[]visibler`,
    declare the `visibler` interface, update its slice literal to hold
    the 10 overlays directly, and change all 10 `ui.Centered(...)`
    sizing lines to call `.Primitive()`. Update `host.go`'s
    `anyOverlayVisible` to call `.Visible()`. `gofmt -l`,
    `go vet ./...`, `go build ./...`, `go test ./...` all clean — this
    is the task where the old raw-field access is actually removed.

    Note: `anyOverlayVisible` turned out to already live in `app.go`
    itself (next to `onGlobalKey`/`onPromptDone`, its two callers), not
    `host.go` as plan.md assumed — updated in place, same change either
    way. `visibler` declared alongside the existing small local
    interfaces (`bordered`, `activatable`) at the end of `app.go`, per
    plan.md's stated placement rationale.

11. [x] Final verification pass: `grep -n '\.flex,\|\.form,' tui/internal/app/app.go`
    (the old sizing-call pattern) and `grep -n '&a\.' tui/internal/app/app.go`
    (the old `overlayVisible` pattern) both return nothing; `gofmt -l tui/`
    and `go vet ./...` clean repo-wide; `go build ./...` and
    `go test ./...` pass repo-wide. No commit needed unless this
    surfaces something to fix.

    Confirmed: `&a.` pattern fully gone; the two remaining `.flex,`
    hits are `messagesV`/`logSearchV` (unrelated resource-view page
    routing, not overlay sizing) — refined the check to
    `ui.Centered(a\.[a-zA-Z]*\.\(flex\|form\)` to confirm zero overlay
    sizing calls remain unconverted. All checks clean.
