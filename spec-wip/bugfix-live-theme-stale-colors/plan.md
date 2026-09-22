# Plan: live theme switch leaves widgets in the old theme's colors

## Approach

Fix it where the colors are applied, not widget by widget. Three shared
helpers in `internal/ui/style.go` set **every** color a widget keeps
from its construction time, using the same palette → tview mapping that
`app.applyTheme` uses at startup. A live switch then produces exactly
what a restart would. Each affected `ApplyPalette` calls the matching
helper.

No new dependencies, and nothing changes in `config` or the theme files.

## What each widget keeps from construction, and what a restart gives it

`applyTheme` maps the palette onto `tview.Styles` as follows:

| `tview.Styles` field | Palette color |
|---|---|
| `PrimitiveBackgroundColor`, `ContrastBackgroundColor` | `Background` |
| `PrimaryTextColor` | `Text` |
| `SecondaryTextColor`, `ContrastSecondaryTextColor` | `Value` |
| `TertiaryTextColor` | `Label` |

`tview` v0.42 copies those values into these widgets when they're
created:

| Widget | Colors kept from construction | What a restart gives it |
|---|---|---|
| `List` | main text style | `Text` on `Background` |
| `List` | secondary text style | `Label` on `Background` |
| `List` | shortcut style | `Value` on `Background` |
| `Form` | label color | `Value` |
| `Form` | field style | `Text` on `Background` |
| `Form` | button style | `Text` on `Background` |
| `Form` | activated button style | `Background` on `Text` |
| `Form` | disabled button style | `Value` on `Background` |
| `InputField` / `DropDown` | label style | background `Background` |

`List` also keeps a selected style, but `ui.StyleList` already resets
that one, and forms already reset their border and background.

`Form.Draw` passes the form's label color, background and field style
down to every item on every draw, via `SetFormAttributes`. So setting
them on the form fixes its input fields, text areas and dropdowns too.
That includes the connection editor's `sectionHeaderItem`, which reads
the label color from `SetFormAttributes`.

The stand-alone filter inputs set their label and field colors
explicitly from the palette (`Label`, `SelectionBg`, `SelectionText`),
both at construction and in `ApplyPalette`. But `SetLabelColor` only
changes the label's *text* color. The label's background is kept from
construction, which is why the live check found the old background
behind ` / filter: `. The Datadog view's two dropdowns have the same
label problem. Worse, their `ApplyPalette` only calls `StyleDropDown`,
so their label and field colors are never reapplied at all.

## Changes

**`internal/ui/theme.go`**

- The palette → `tview.Styles` mapping moves out of `app.applyTheme`
  into an exported `ui.ApplyTviewStyles(p)`; `app.applyTheme` now just
  calls it. This wasn't in the first draft of this plan. Tests in `ui`,
  `dialog` and `view` must all put `tview.Styles` into palette A the way
  startup does, and one shared, exported mapping keeps them (and the
  `Style*` helpers' notion of "what a restart gives") from drifting away
  from the real one.

**`internal/ui/style.go`**

- **`StyleList(l, p)`**: in addition to the selected colors it sets
  today, also set the main, secondary and shortcut text styles from the
  table above. This fixes every list, because they all go through it
  already: confirm, move picker, theme picker, connection manager,
  time-range presets, the Settings list, and the snippet picker.
- **New `StyleForm(f, p)`**: sets the form's label color, field style,
  and button, activated-button and disabled-button styles from the table
  above. It also resets every item's and button's own box background.
  Found by the restart-comparison test: an `InputField` wraps its field
  in a separate inner widget, `SetFormAttributes` reaches only that inner
  one, and the outer box keeps painting its construction-time background
  beside the field. This is the same trap `reapplyTheme` already works
  around for the `:` prompt.
- **New `StyleFilterInput(i, p)`** sets the colors the stand-alone
  inputs use today, plus the label background:
  - label: `Label`, on `Background`
  - field: `SelectionText` on `SelectionBg`
  - placeholder: `Value` on `Background`, which is what a freshly built
    field gets. The first draft said `SelectionBg`; no input uses a
    placeholder today.

  **How it's applied (found by the restart-comparison test):** the label
  is drawn on the background of the input's *inner* text area, not on
  its outer box. `SetLabelColor` and `SetFieldBackgroundColor` can't
  reach that inner background, so the helper uses `SetFormAttributes`,
  the same workaround `reapplyTheme` uses for the `:` prompt. It passes
  label width 0 ("fit the label"), since no filter input sets a label
  width. It also resets the outer box's background.

  It replaces the identical three-line blocks, both at construction and
  in `ApplyPalette`, in:
  - `queues.go`, `messages.go`, `ssmparams.go`, `secrets.go`, `logs.go`,
    `logsearch.go`, `codepipelinelist.go`, `datadoglogs.go` (query input)
  - `movepicker.go`, `awsprofiles.go`, `jmstypeprompt.go`

  Replacing those blocks is part of the fix, not a drive-by: each one
  would otherwise need the same extra line added.

  **Two gaps fixed along the way:** `MessagesView.ApplyPalette` and
  `AWSProfilesPicker.ApplyPalette` never restyled their input
  (` / search:` and ` / filter:`) at all, so after a live switch those
  kept the old theme entirely. Both now call `StyleFilterInput`.
- **Dropdowns: two helpers.** A dropdown keeps more from construction
  than the first draft of this plan listed: besides the label, field and
  pop-up list, also its focused, prefix and disabled styles and its own
  box background. A form passes on none of the last four. So the split
  is by where the dropdown sits:
  - **New `StyleFormDropDown(dd, p)`,** for dropdowns inside a form.
    It sets everything a form *doesn't* pass on:
    - the pop-up list's styles (what `StyleDropDown` did before)
    - focused and prefix: `Background` on `Text`
    - disabled: `Value` on `Background`

    These match a restart. The name is clearer than the first draft's
    `StyleDropDownList`, since it covers more than the list.
  - **`StyleDropDown(dd, p)`,** for stand-alone dropdowns, calls
    `StyleFormDropDown` and also sets its own box background, the label
    (`Label`) and the field (`SelectionText` on `SelectionBg`). It uses
    `SetFormAttributes` with label width 0, like `StyleFilterInput`.
  - The Datadog view's `ApplyPalette` already calls `StyleDropDown`, so
    it's fixed with no change there. The now-duplicated
    construction-time lines in `datadoglogs.go` go.
  - The connection editor's in-form dropdowns switch to
    `StyleFormDropDown`. Its `ApplyPalette` now restyles **both**
    dropdowns, Backend and Authentication Mode; only Backend was
    restyled before.

**Forms: `ApplyPalette` calls `ui.StyleForm`**

- `sendmessage.go`: also drop its now-redundant explicit `TextArea`
  styling, since `Form.Draw` overrides it anyway.
- `messagefilter.go`
- `datadogsettings.go`
- `connections.go` (`ConnEditor`)
- `timerangemodal.go` (the absolute-range form)
- `snippetsave.go`

**Found by the per-package regression tests (task 6)**

The first draft assumed tables and text views needed no change. The
regression tests showed that some parts of them are also colored only
when created:

- **Table headers.** Every table view colors its header row (and the
  cell above the favorites star column) in `setHeader`. `QueuesView`
  redraws it on each repaint and `LogSearchView` on each search; the
  rest only at construction. So after a switch the header kept the old
  theme's `Label` background until a restart. Every table view's
  `ApplyPalette` now calls `setHeader()`:
  - `QueuesView`, `MessagesView`, `SSMParamsView`, `SecretsView`
  - `LogsView`, `LogSearchView`, `DatadogLogsView`,
    `CodePipelineListView`

  `setHeader` only rewrites row 0's cells with fixed per-column widths,
  so calling it again is safe.
- **Table column separators** use the table's borders color, which is
  copied at construction. Each of those views, and the AWS profiles
  picker, now resets it (`SetBordersColor(Border)`). They're blank
  cells, so this was invisible, but it keeps the test strict.
- **`AWSProfilesPicker`:**
  - its header row (`setHeader`) and its key-hint footer (`Accent`
    color tags) were built only at construction
  - both are now rebuilt in `ApplyPalette`; the hints moved into a
    `setHints` method
- **`TimeRangeModal`'s tab row:** `renderTabs` color-tags the two tab
  labels, but the spaces around them use the text view's own base text
  color, copied at construction. `ApplyPalette` now resets it.

**Found during the live check (task 7)**

- **Text views' base text color.** A `TextView` copies its base text
  color at construction. Color-tagged text is rebuilt on a switch or
  reopen, but untagged characters are drawn in that base color: the
  spaces between tags, the whole logo, and log lines without a level.
  Fixed with `SetTextColor(Text)`, which is what a freshly built text
  view gets:
  - in `reapplyTheme`, for the top bar's info, context and logo panels
    (the logo was visibly stale)
  - in the `ApplyPalette` of `MessageDetailView`, `ParamDetailView`,
    `SecretDetailView`, `LogDetailView`, `DatadogLogDetailView` and
    `LogView`
- **`CodePipelineDetailView`** gets the same header and
  column-separator treatment as the other table views.
- **Tests:**
  - `app.TestReapplyThemeRecolorsShellTextPanels` covers the shell
    panels.
  - The view regression test now also covers the detail views and the
    log view. Those are opened again after the switch, as in real use.
    Their text is rebuilt then, but the base color isn't.
  - The first draft of this plan left detail views out, which was wrong.

`ConnManager`'s hints looked stale at first, but they're rebuilt every
time it opens. The dialog regression test reopens overlays after the
switch, as happens in real use, since only the theme picker is open
during a switch.

**Not changed**

- The command prompt: `reapplyTheme` already handles it via
  `SetFormAttributes`.
- **Table data rows:** open question below.

## Testing

- **`internal/ui/style_test.go`:** for each helper, build the widget
  while `tview.Styles` holds palette A, apply palette B through the
  helper, draw it on a simulation screen, and check that:
  - list: an unselected row renders in B's `Text` on B's `Background`
  - form: the label, input field and a button render in B's colors
  - filter input: the label background is B's `Background`
  - dropdown: the label and field render in B's colors

  A helper saves and restores `tview.Styles`, so tests don't leak global
  state.
- **Per dialog/view:** one table-driven test per package (`dialog`,
  `view`) builds each affected widget under palette A, calls its
  `ApplyPalette(B)`, renders it, and checks that no cell still carries
  one of A's colors that B doesn't also use.
  - For widgets that can't easily be rendered in isolation, the test
    checks the stored style instead: e.g. `List.GetItemText` doesn't
    expose styles, but a render of a one-item list does, so render
    wherever possible.
- **Palettes:** A = `dark` and B = `cyberpunk` from the embedded themes
  (`config.PaletteForTheme`). Their `Background`, `Text`, `Label`,
  `Value` and `Selection*` colors all differ, so a stale color is
  detectable.
- **Live check:** switch `dark` → `cyberpunk` in the running TUI and
  check with per-cell color capture (as in the snippets live check):
  - the snippet picker, Move-to-Queue picker and confirmation dialog
  - the send-message, save-as-snippet and message-filter dialogs
  - a view's `/` filter input
  - the Datadog dropdowns
  - the AWS profiles picker's table
  - Settings

  Then restart and confirm it looks identical.

## Startup applies every palette too (added after task 3)

`ApplyPalette` used to run only on a live theme switch, never at startup
(`App.New` builds the widgets but didn't call it). So a few widgets that
get their palette colors *only* from `ApplyPalette` looked different
right after startup than after a live switch:

- **`MovePicker` and `SnippetPicker`:** their lists got `StyleList`, and
  so the palette's `SelectionBg`/`SelectionText` highlight, only on a
  live switch. At startup the selected row used tview's default instead:
  `Background` text on `Text` background, i.e. inverted body colors.
  Confirmed in the #29 live check's color capture after a restart.
- **`MovePicker`'s search box:** got `StyleFilterInput` only on a live
  switch. At startup it had tview's default input colors.

**Decision (user, task 3 review):** fix it in this bugfix.
- `App.New` calls `ApplyPalette(cfg.Colors)` on every entry of
  `a.themables` once, right after that slice is built.
- Startup, live switch and restart then all show the same colors: the
  ones each `ApplyPalette` already defines.
- This does change how those pickers look at startup, which is
  intended.

## Table data rows (decided after task 6)

Table **data rows** are also colored when they're drawn: the queue
names, message types, the favorites star and so on. After a live switch
they keep the old theme's colors until the view next repaints (a
refresh, a filter change, a reload, or the queue list's auto-refresh).

Repainting on a switch would move the cursor, since each view's
`repaint` resets the selection to the first row and scrolls to the top.

**Decision (user, task 6 review):** leave as is. Rows pick up the new
theme on the view's next repaint. Out of scope for this bugfix.
