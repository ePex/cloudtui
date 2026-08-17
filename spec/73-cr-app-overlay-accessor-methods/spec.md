# Spec — CR 73: add `Primitive()`/`Visible()` accessor methods to the 10 overlays

Date: 2026-08-17

## Background

Phase 3's recorded roadmap (`spec/70`) still has two big pieces left:
exporting all 10 overlay types/constructors/`Show` methods, and fixing
`app.go`'s direct `.flex`/`.form`/`.visible` field access (found by the
audit in `spec/70`'s spec.md — `New()` reads `.flex`/`.form` directly
for `ui.Centered(...)` sizing, and takes the address of the raw
`.visible` field directly for `overlayVisible []*bool`). Doing both at
once — renaming everything to exported AND redesigning how `app.go`
reaches into the moved types — makes one large, harder-to-verify
change. Splitting them: this CR adds two small accessor methods to
each of the 10 overlays and redesigns `app.go` to call them instead of
reaching into the raw fields, entirely within package `app`, nothing
exported yet. The actual export/rename pass becomes CR 74 — purely
mechanical at that point, since access already goes through methods,
not raw fields.

Re-confirmed the exact current shape before designing the fix:

```go
// app.go, New() — 10 lines, one per overlay
confirmOverlay := ui.Centered(a.confirm.flex, 52, 8)
// ... connEditor/messageFilter/datadogEditor use .form instead of .flex

// app.go, New() — overlayVisible field is []*bool
a.overlayVisible = []*bool{
	&a.confirm.visible,
	// ... 9 more
}

// host.go — anyOverlayVisible dereferences each pointer
func (a *App) anyOverlayVisible() bool {
	for _, v := range a.overlayVisible {
		if *v {
			return true
		}
	}
	return false
}
```

## Problem

Both patterns reach past each overlay's own encapsulation boundary
(a raw field read, and the address of a raw field) — fine today since
everything's one package, but exactly the kind of access that breaks
across the `app`/`dialog` boundary the move introduces. Neither can be
fixed by exporting the field alone without also weakening
encapsulation further than necessary (exporting `.visible`/`.flex`/
`.form` directly would let anything reach in and mutate them, not just
read).

## Solution

Two new methods per overlay, mirroring an existing codebase convention
(`ui.View`'s `Primitive()`) rather than inventing a new one:

- `Primitive() tview.Primitive` — returns `.flex` (7 overlays:
  `confirmDialog`, `movePicker`, `sendMessageOverlay`, `connManager`,
  `timeRangeModal`, `themePicker`, `awsProfilesPicker`) or `.form` (3:
  `connEditor`, `messageFilter`, `datadogEditor`) — whichever each
  already uses for `ui.Centered(...)` sizing today.
- `Visible() bool` — returns `.visible`, for all 10.

`app.go`'s `New()` calls `a.confirm.Primitive()` etc. instead of
`a.confirm.flex`/`.form`. `App.overlayVisible`'s type changes from
`[]*bool` to `[]visibler`, a new unexported local interface
(`interface{ Visible() bool }`) — all 10 overlays already satisfy it
once the method exists, so the slice literal changes from
`&a.confirm.visible` to plain `a.confirm` (the overlay itself, not a
field of it). `anyOverlayVisible()` changes from `if *v` to
`if v.Visible()`.

## Scope

### In scope

- All 10 overlay files: one `Primitive()` + one `Visible()` method
  each (still unexported types/methods otherwise — only these two are
  new).
- `app.go`: the 10 `ui.Centered(...)` sizing lines updated to call
  `.Primitive()`; `overlayVisible` field retyped to `[]visibler`; its
  slice literal updated to hold the 10 overlays directly, not
  addresses of their fields; the `visibler` interface declared
  (in `app.go`, next to the field, since it's a small App-internal
  concern).
- `host.go`: `anyOverlayVisible()` updated to call `.Visible()`.

### Out of scope

- Exporting any of the 10 types, constructors, or `show`/`close`
  methods — CR 74.
- The actual move to `internal/dialog` — CR 75 (or later, depending on
  how CR 74 goes).
- Fixing the test files that reach into `.visible`/other unexported
  fields directly (`app_test.go`, `messages_test.go`,
  `datadoglogs_test.go`, `logsearch_test.go`) — they still compile
  fine after this CR (the field itself isn't removed, just no longer
  the only way to read it), so fixing them can wait for whichever CR
  actually needs them not to reach in directly (the export pass or the
  move).
- Any behavior change. Pure additive methods + one internal
  representation change (`[]*bool` → `[]visibler`) with identical
  runtime behavior.

## Definition of done

1. `go build ./...` and `go test ./...` pass in `tui/`.
2. All 10 overlays have `Primitive()` and `Visible()` methods;
   `app.go` calls them instead of reaching into `.flex`/`.form`/
   `.visible` directly for these two purposes.
3. No behavior change — pure refactor of internal access patterns;
   `go test ./...` passing is sufficient, no live verification needed
   (nothing here changes what's drawn or when).
