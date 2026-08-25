# Tasks

1. [x] Fix `onGlobalKey`'s `focusExemptInputs` check
   (`tui/internal/app/app.go`) to use `HasFocus()` instead of identity
   comparison; correct the two existing Datadog dropdown regression
   tests (`tui/internal/app/app_test.go`) to open the popup before
   asserting, confirming each fails without the fix and passes with it.
2. [x] Manual verification: build and run the TUI, open Datadog Logs'
   Service and Env filter dropdowns, confirm a global-hotkey letter
   (`q`, `h`, `s`) typed while the popup is open no longer quits/
   navigates away (record what was checked here).

   Checked by driving the built binary in tmux, against real Datadog
   data (both dropdowns populated with real service/env values):
   opened the Service filter's popup (`S` then Enter) and typed `q`
   — the app did **not** quit; `q` was typed into the dropdown's own
   prefix-jump field ("Service: qany)"). Typed `h` next in the same
   still-open popup — again went into the field, no navigation to
   Home. `Esc` closed the popup cleanly with no side effect. Repeated
   for the Env filter's popup (`E` then Enter): typed `s` — went into
   the field ("Env: sany)"), did not navigate to Settings. Finally,
   confirmed the legitimate hotkey path is unaffected: with no dropdown
   open, `q` correctly quit the app (tmux session ended). All behavior
   matches the fix's intent exactly.
3. [ ] Merge-back: update `spec/18-datadog-logs/spec.md`'s "Notable
   gotchas worth preserving" section with this second instance of the
   global-hotkey-leak class of bug; delete
   `spec-wip/bugfix-dropdown-hotkey-leak/`.
