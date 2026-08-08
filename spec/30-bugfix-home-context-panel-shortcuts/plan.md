# Plan — Bugfix 30: global hotkeys disappear from the home screen

## `home.go`

Add `Shortcuts() []ui.Shortcut` to `HomeView`, returning the same six
entries `readyStatusText` (in `internal/app/statusbar.go`) renders for the
bottom bar. These live in different packages and render through different
paths (plain data here, a pre-colored string there), so they're two
independent lists rather than a shared abstraction — noted in a comment on
`Shortcuts()` as a "keep both in sync if a global hotkey changes" flag
rather than engineering a cross-package dependency for six static entries.

`var _ ui.Shortcuttable = (*HomeView)(nil)` alongside the existing
`var _ ui.View = (*HomeView)(nil)`.

No changes to `app.go`'s `updateContextPanel` — it already renders any
`Shortcuttable` view's shortcuts generically; Home simply starts
qualifying.

## `shortcuttable.go`

Update the doc comment: it asserted the status bar always carries the
legend, which this bugfix is evidence against.

## Testing

`views_test.go`: `HomeView` implements `Shortcuttable`; `Shortcuts()`
returns exactly the six global hotkeys with correct descriptions.

`app_test.go`: the existing "non-Shortcuttable view leaves the context
panel blank" test used Home as its example — switched to Settings (still
accurate) and added a new test asserting Home's panel now contains all six
hotkeys after `switchTo("home")`.
