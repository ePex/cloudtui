# Tasks

1. [ ] Fix `onGlobalKey`'s `focusExemptInputs` check
   (`tui/internal/app/app.go`) to use `HasFocus()` instead of identity
   comparison; correct the two existing Datadog dropdown regression
   tests (`tui/internal/app/app_test.go`) to open the popup before
   asserting, confirming each fails without the fix and passes with it.
2. [ ] Manual verification: build and run the TUI, open Datadog Logs'
   Service and Env filter dropdowns, confirm a global-hotkey letter
   (`q`, `h`, `s`) typed while the popup is open no longer quits/
   navigates away (record what was checked here).
3. [ ] Merge-back: update `spec/18-datadog-logs/spec.md`'s "Notable
   gotchas worth preserving" section with this second instance of the
   global-hotkey-leak class of bug; delete
   `spec-wip/bugfix-dropdown-hotkey-leak/`.
