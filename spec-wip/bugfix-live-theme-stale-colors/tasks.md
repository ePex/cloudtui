# Tasks: live theme switch leaves widgets in the old theme's colors

Each task is implemented and approved on its own, then pushed.

1. [x] **Test scaffolding and `StyleList`.**
   - Move the palette → `tview.Styles` mapping into
     `ui.ApplyTviewStyles` (called by `app.applyTheme`).
   - Add a `tview.Styles` save/restore helper and a render-colors helper
     to `internal/ui` tests.
   - Extend `StyleList` to set the main, secondary and shortcut styles.
   - Test: a list built under `dark` and styled with `cyberpunk` renders
     unselected rows in `cyberpunk`'s `Text` on `Background`.
2. [x] **`StyleForm`, used by every form dialog.**
   - Add `ui.StyleForm` with a render test covering the label, input
     field and a button.
   - Call it from the `ApplyPalette` of:
     - `SendMessageOverlay` (and drop its redundant `TextArea` styling)
     - `MessageFilter`, `DatadogEditor`, `ConnEditor`
     - `TimeRangeModal`'s absolute form
     - `SnippetSaveDialog`
3. [x] **`StyleFilterInput`, used by every stand-alone input.**
   - Add `ui.StyleFilterInput` with a render test covering the label
     background and the field colors.
   - Replace the three-line blocks, at construction and in
     `ApplyPalette`, in the 8 views and 3 dialogs listed in `plan.md`.
4. [x] **`StyleDropDown` / `StyleFormDropDown`.**
   - `StyleDropDown` also sets its box background, label and field, on
     top of `StyleFormDropDown`.
   - New `StyleFormDropDown` (pop-up list plus focused, prefix and
     disabled styles) for the connection editor's in-form dropdowns.
     `ConnEditor.ApplyPalette` now restyles both of them.
   - Drop the duplicated construction-time lines in `datadoglogs.go`.
   - Restart-comparison tests for both helpers, each with the dropdown
     unfocused, focused and open.
5. [x] **Apply every palette at startup too.**
   - `App.New` calls `ApplyPalette(cfg.Colors)` on every themable once,
     after all widgets are built. Startup then shows the same colors as a
     live switch and a restart.
   - Test in `internal/app`: after `New`, a widget whose colors come only
     from `ApplyPalette` (e.g. the Move-to-Queue picker's selected row)
     already draws in the palette's selection colors.
6. [x] **Per-package regression tests.**
   - In `internal/dialog` and `internal/view`, a table-driven test builds
     every affected widget under `dark`, calls `ApplyPalette(cyberpunk)`,
     renders it, and fails if any cell still uses a `dark`-only color.
   - If this surfaces a widget the plan missed, fix it here and note it
     in `plan.md`.
7. [ ] **Live verification.**
   - Use the `verify-live` skill with a temporary `HOME`, run from the
     scratch folder (not `tui/`).
   - Switch `dark` → `cyberpunk` in Settings, then check with per-cell
     color capture that no `dark`-only colors remain in:
     1. the Settings list
     2. the confirmation dialog (e.g. `d` on a message)
     3. the Move-to-Queue picker, including its ` / filter: ` label
     4. the send-message dialog (labels, fields, buttons) and its
        snippet picker
     5. the save-as-snippet dialog (`S`)
     6. the message-filter dialog (`f`)
     7. a view's `/` filter input (queues)
     8. the time-range modal (both tabs) and the Datadog view's
        dropdowns, if reachable without Datadog credentials; otherwise
        note that they're covered by unit tests only
     9. the AWS profiles picker's table and the connection manager and
        editor
   - Restart and confirm the same screens look identical, including the
     Move-to-Queue and snippet pickers' selected row and the
     Move-to-Queue search box right after startup (task 5).
   - Record the results here.
8. [ ] **Merge-back** (needs your explicit go-ahead before it's
   committed).
   - Fold the fix into `spec/04-theming/spec.md`: live switching now
     fully recolors lists, forms, inputs and dropdowns via the shared
     `ui` style helpers.
   - Update `spec/03`'s `style.go` line if the helper list changed.
   - Delete `spec-wip/bugfix-live-theme-stale-colors/` and mark the PR
     ready for review.
