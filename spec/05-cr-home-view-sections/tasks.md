# Tasks — CR 05: Home view navigatable sections

Plan: [plan.md](plan.md)

1. [x] **`SectionInfo` type + `NewHome` signature** — replace `[]ViewInfo`
   parameter with `[]SectionInfo` in `NewHome` and `RepaintHomeTable`; add
   `onSelect func(name string)` parameter to `NewHome`; update `HomeView`
   struct to store sections and callback.

2. [x] **Section header rendering** — in `RepaintHomeTable`, render each
   section as a styled non-selectable header row (`─── <Title> ─────`) in
   the `border` palette color (separate from label color) followed by
   selectable entry rows; dashes fill both columns so the line spans the
   full table width; enable `SetSelectable(true, false)` on the table.

3. [x] **Navigation and Enter wiring** — wire j/k via `SetInputCapture` to
   forward synthetic ↑/↓ events; wire `SetSelectedFunc` to call
   `onSelect(name)` with the view name for the selected row.

4. [x] **App wiring** — update `app.go`: define `[]SectionInfo` with Apps
   (Queues) and System (Settings, Log); remove "home" from the list; pass
   `a.switchTo` as the `onSelect` callback; update `homeViewInfos` field type.

5. [x] **`reapplyTheme` update** — update the `RepaintHomeTable` call in
   `theme.go` to pass `[]SectionInfo` instead of `[]ViewInfo`.

6. [x] **Tests** — update `home_test.go` for new API (section headers
   non-selectable, entry rows selectable, callback fires with correct name);
   update `app_test.go` for changed `homeViewInfos` type.

7. [ ] **Manual verification** — run `task run:tui`; confirm home shows Apps
   and System sections with styled headers; arrow keys and j/k move cursor
   skipping headers; Enter switches to the selected view; theme switch
   repaints header colors correctly.
