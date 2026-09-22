# Bugfix: live theme switch leaves widgets in the old theme's colors

_Date: 2026-09-22_

## What

Switching the theme in Settings recolors the app immediately, but only
partly. Until the app is restarted, these keep the **previous** theme's
colors:

- **Lists:** every row except the selected one keeps its old text and
  background colors. This affects the confirmation dialog, the
  Move-to-Queue picker, the theme picker, the connection manager, the
  time-range presets, the Settings list, and the snippet picker.
- **Forms:** field labels, input fields and buttons keep their old
  colors. Only the border and background change. This affects the
  send-message, message-filter, Datadog, connection-editor and
  time-range dialogs, and the save-as-snippet dialog.
- **Stand-alone input fields and dropdowns** (e.g. the `/` filter fields
  and the Datadog facet dropdowns) are probably affected the same way.
  The plan will confirm which ones are.

After a restart everything renders correctly, because widgets are then
built after the new theme is in place.

Found during the live check of `fe-message-snippets` (PR #29):

- After switching `dark` → `cyberpunk`, the snippet picker's unselected
  rows kept `dark`'s text/background colors.
- The Move-to-Queue picker behaved the same way.
- The save dialog's `Name` label and input field kept `dark`'s colors.

## Why

**The cause** is in how tview builds widgets:

- `tview.NewList` and `tview.NewForm` copy their colors from the global
  `tview.Styles` **when the widget is created**, and store them inside
  the widget:
  - a list's colors for unselected rows
  - a form's label color, field style and button styles
- A live theme switch updates `tview.Styles` and then calls each widget's
  `ApplyPalette`. But:
  - `ui.StyleList` only resets the *selected*-row colors
  - most form dialogs' `ApplyPalette` only reset the border and
    background
- Nothing resets the colors stored at creation time.

**Why it matters:** a mixed palette can be hard to read. For example, a
light theme's text color on a dark theme's background leaves unselected
rows or input fields barely readable until a restart. That defeats the
point of switching live.

## Scope

- **Expected behavior:** after a live theme switch, every list, form,
  input field and dropdown that is built once and reused renders fully
  in the new theme, exactly as it would after a restart. This applies to
  overlays and views alike.
- **Lists:** the shared list styling (`ui.StyleList`) also sets the
  unselected-row colors from the palette. That fixes every list that
  uses it in one place.
- **Forms:** every form dialog's `ApplyPalette` also sets its label,
  field and button colors from the palette.
- **Input fields and dropdowns:** stand-alone ones get the same
  treatment wherever the plan finds them affected.
- **Snippet dialogs:** the fix covers `SnippetPicker` and
  `SnippetSaveDialog` (merged in PR #29).
- **Tests:** each fixed widget gets a test that builds it under one
  palette, applies another, and checks that the rendered colors match
  the new palette. Where a render check isn't practical, the test checks
  the widget's stored style.

## Out of scope

- Any change to the theme files themselves or to which palette colors
  exist.
- Changing *which* palette color a widget uses (e.g. deciding labels
  should be `Accent` rather than `Label`). The fix makes a live switch
  match what a restart already shows, nothing more.
- Tables and text views whose content is rebuilt on every repaint,
  unless the plan finds them affected too.
- Anything drawn only once at startup that a theme switch rebuilds
  anyway.
