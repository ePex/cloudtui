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

**`internal/ui/style.go`**

- **`StyleList(l, p)`**: in addition to the selected colors it sets
  today, also set the main, secondary and shortcut text styles from the
  table above. This fixes every list, because they all go through it
  already: confirm, move picker, theme picker, connection manager,
  time-range presets, the Settings list, and the snippet picker.
- **New `StyleForm(f, p)`**: sets the form's label color, field style,
  and button, activated-button and disabled-button styles from the table
  above.
- **New `StyleFilterInput(i, p)`** sets the colors the stand-alone
  inputs use today, plus the label background:
  - label style: `Label` on `Background`
  - field: `SelectionText` on `SelectionBg`
  - placeholder: `Value` on `SelectionBg`

  It replaces the identical three-line blocks, both at construction and
  in `ApplyPalette`, in:
  - `queues.go`, `messages.go`, `ssmparams.go`, `secrets.go`, `logs.go`,
    `logsearch.go`, `codepipelinelist.go`, `datadoglogs.go` (query input)
  - `movepicker.go`, `awsprofiles.go`, `jmstypeprompt.go`

  Replacing those blocks is part of the fix, not a drive-by: each one
  would otherwise need the same extra line added.
- **`StyleDropDown(dd, p)`**: also set the label style (`Label` on
  `Background`) and field colors (`SelectionText` on `SelectionBg`), on
  top of the list styles it sets today. The Datadog view's `ApplyPalette`
  already calls it, so that's fixed with no change there, and the
  now-duplicated construction-time lines in `datadoglogs.go` go.
  - **One exception:** in the connection editor, the Backend dropdown
    sits inside a form, which sets its label and field colors every
    draw. `StyleDropDown`'s new label and field lines would be
    overridden there, which is harmless. To keep that obvious, I'll
    split it into `StyleDropDownList` (the existing list styles, for
    dropdowns inside a form) and `StyleDropDown` (list, label and field,
    for stand-alone dropdowns).

**Forms: `ApplyPalette` calls `ui.StyleForm`**

- `sendmessage.go`: also drop its now-redundant explicit `TextArea`
  styling, since `Form.Draw` overrides it anyway.
- `messagefilter.go`
- `datadogsettings.go`
- `connections.go` (`ConnEditor`)
- `timerangemodal.go` (the absolute-range form)
- `snippetsave.go`

**Not changed**

- Tables (queue, message and resource lists, `AWSProfilesPicker`'s
  table): their cells are rebuilt with explicit palette colors on every
  repaint or load.
- Text views, and the command prompt: `reapplyTheme` already handles
  the prompt via `SetFormAttributes`.

I'll re-check all of these in the live check (task list) rather than
assuming.

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
