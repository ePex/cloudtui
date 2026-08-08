# Tasks — Bugfix 30: global hotkeys disappear from the home screen

Plan: [plan.md](plan.md)

1. [x] `HomeView.Shortcuts()` + `var _ ui.Shortcuttable` assertion.
2. [x] Update `ui.Shortcuttable`'s doc comment.
3. [x] Fix `TestSwitchToClearsContextPanelForNonShortcuttableView` (used
   Home as its example; switched to Settings) and add
   `TestSwitchToHomeShowsGlobalHotkeysInContextPanel`.
4. [x] `views_test.go`: `TestHomeViewImplementsShortcuttable`,
   `TestHomeViewShortcutsIncludeAllGlobalHotkeys`.
5. [x] `go build ./...`, `go vet ./...`, `go test ./...` — all pass.
6. [x] Manual verification: confirmed Home's context panel shows the
   legend on launch, and — the actual bug scenario, not just the happy
   path — still shows it correctly after a transient status-bar message
   elsewhere left the bottom bar stuck on stale text.
