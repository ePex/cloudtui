# Plan — CR 73: `Primitive()`/`Visible()` accessors

## Approach

### 1. Two methods per overlay (10 files)

Same template each time, receiver and root-widget field varying:

```go
// Primitive returns <Type>'s root widget, for sizing/embedding.
func (x *<type>) Primitive() tview.Primitive { return x.<field> }

// Visible reports whether <type> is currently shown.
func (x *<type>) Visible() bool { return x.visible }
```

| Type | File | Root-widget field |
|---|---|---|
| `confirmDialog` | `confirm.go` | `.flex` |
| `movePicker` | `movepicker.go` | `.flex` |
| `sendMessageOverlay` | `sendmessage.go` | `.flex` |
| `connManager` | `connections.go` | `.flex` |
| `connEditor` | `connections.go` | `.form` |
| `messageFilter` | `messagefilter.go` | `.form` |
| `timeRangeModal` | `timerangemodal.go` | `.flex` |
| `datadogEditor` | `datadogsettings.go` | `.form` |
| `themePicker` | `themepicker.go` | `.flex` |
| `awsProfilesPicker` | `awsprofiles.go` | `.flex` |

Placed right after each type's `ApplyPalette`/`var _ ui.Themeable`
block (same "grouped with the type's other small interface-satisfying
methods" convention already used for `ApplyPalette`).

### 2. `app.go`

```go
// before (New(), 10 lines total, one per overlay)
a.confirm = newConfirmDialog(a)
confirmOverlay := ui.Centered(a.confirm.flex, 52, 8)
// ...

// after
a.confirm = newConfirmDialog(a)
confirmOverlay := ui.Centered(a.confirm.Primitive(), 52, 8)
// ...
```

All 10 sizing lines change the same way — `.flex`/`.form` →
`.Primitive()`, no other change (the width/height literals and their
explanatory comments are untouched).

```go
// before
overlayVisible []*bool
// ...
a.overlayVisible = []*bool{
	&a.confirm.visible,
	&a.movePicker.visible,
	&a.sendMessage.visible,
	&a.connManager.visible,
	&a.connEditor.visible,
	&a.messageFilter.visible,
	&a.timeRangeModal.visible,
	&a.datadogEditor.visible,
	&a.themePicker.visible,
	&a.awsProfiles.visible,
}

// after
overlayVisible []visibler
// ...
a.overlayVisible = []visibler{
	a.confirm,
	a.movePicker,
	a.sendMessage,
	a.connManager,
	a.connEditor,
	a.messageFilter,
	a.timeRangeModal,
	a.datadogEditor,
	a.themePicker,
	a.awsProfiles,
}
```

`visibler` declared next to the `overlayVisible` field's doc comment:

```go
// visibler is satisfied by every modal overlay via its Visible()
// accessor — overlayVisible holds these instead of raw *bool pointers
// so App doesn't need to reach into an overlay's unexported field
// (blocking once the overlays move to a different package).
type visibler interface{ Visible() bool }
```

### 3. `host.go`

```go
// before
func (a *App) anyOverlayVisible() bool {
	for _, v := range a.overlayVisible {
		if *v {
			return true
		}
	}
	return false
}

// after
func (a *App) anyOverlayVisible() bool {
	for _, v := range a.overlayVisible {
		if v.Visible() {
			return true
		}
	}
	return false
}
```

## Files touched

- `confirm.go`, `movepicker.go`, `sendmessage.go`, `connections.go`
  (both types), `messagefilter.go`, `timerangemodal.go`,
  `datadogsettings.go`, `themepicker.go`, `awsprofiles.go` (+1 method
  pair each, 11 pairs total across 9 files since `connections.go`
  holds 2 types)
- `app.go` (`overlayVisible` field type, its slice literal, the 10
  sizing lines, the new `visibler` interface)
- `host.go` (`anyOverlayVisible`)

## Key decisions

- **`visibler` stays unexported, declared in `app.go`** — it's an
  `App`-internal implementation detail (how `onGlobalKey`/
  `onPromptDone` check overlay visibility), not part of any contract
  the overlays or `ui` package need to know about. Doesn't belong in
  `ui.Host` (overlays don't need to ask `Host` "is some other overlay
  visible") or on the overlays themselves as an exported type.
- **`Primitive()`/`Visible()` are real exported-shaped names even
  though nothing outside package `app` calls them yet** — matches
  spec.md's reasoning (mirrors `ui.View.Primitive()`) and avoids a
  second rename in CR 74 for these two specific methods (they're
  already correctly named, just need their *type* exported then).
- **No new tests** — pure additive methods + one internal
  representation swap with identical behavior.
- **No new dependencies.**

## Definition of done

Unchanged from spec.md — `go build`/`go test` pass, all 10 overlays
have both methods, `app.go`/`host.go` use them instead of raw
field access for sizing and visibility checks.
