# Plan — TUI theme refresh

Spec: [2026-07-14_feat-tui-theme-refresh.md](2026-07-14_feat-tui-theme-refresh.md)

## Approach

### `Palette` schema (`internal/config`)

```go
type Palette struct {
    Background string `yaml:"background"`
    Border     string `yaml:"border"`
    Label      string `yaml:"label"`
    Text       string `yaml:"text"`
    Value      string `yaml:"value"`
    Accent     string `yaml:"accent"`

    Success string `yaml:"success"`
    Warning string `yaml:"warning"`
    Error   string `yaml:"error"`

    SelectionBg   string `yaml:"selectionBg"`
    SelectionText string `yaml:"selectionText"`
    StatusBarBg   string `yaml:"statusBarBg"`
    StatusBarText string `yaml:"statusBarText"`

    Views map[string]string `yaml:"views"`
}
```

`Default()`'s values change to the spec'd hex codes; `Views`' five entries
all collapse to the same value as `Border` (the map/`ViewColor` fallback
mechanism is untouched — only what it defaults to).

### Applying the theme globally (`internal/app/theme.go`, new)

tview primitives capture their colors from the package-level `tview.Styles`
var *at construction time* (confirmed against `rivo/tview` v0.42.0's
`box.go`/`list.go`: `NewBox()` sets `backgroundColor: Styles.PrimitiveBackgroundColor`,
`NewList()` sets `selectedStyle: Foreground(Styles.PrimitiveBackgroundColor).Background(Styles.PrimaryTextColor)`,
etc.). So one mutation of `tview.Styles`, done once before any primitive is
built, re-themes the whole shell without touching every constructor:

```go
// applyTheme sets tview's package-level default styles from p. Must run
// before any tview primitive is constructed (see App.New()) — primitives
// read tview.Styles once, at construction time, not on every draw.
func applyTheme(p config.Palette) {
    bg := tcell.GetColor(p.Background)
    tview.Styles.PrimitiveBackgroundColor = bg
    tview.Styles.ContrastBackgroundColor = bg
    tview.Styles.MoreContrastBackgroundColor = bg
    tview.Styles.BorderColor = tcell.GetColor(p.Border)
    tview.Styles.TitleColor = tcell.GetColor(p.Border)
    tview.Styles.GraphicsColor = tcell.GetColor(p.Border)
    tview.Styles.PrimaryTextColor = tcell.GetColor(p.Text)
    tview.Styles.SecondaryTextColor = tcell.GetColor(p.Value)
    tview.Styles.TertiaryTextColor = tcell.GetColor(p.Label)
    tview.Styles.InverseTextColor = tcell.GetColor(p.SelectionText)
    tview.Styles.ContrastSecondaryTextColor = tcell.GetColor(p.Value)
}
```

`App.New()` calls `applyTheme(cfg.Colors)` immediately after `cfg` is
resolved (default fallback included), before `proxy.New(...)` and the
`a.views` slice literal — the two places primitives first get built.

Selection can't ride on the same global mutation: `tview.List`'s computed
default selection style is "invert body text" (`Foreground(Background).Background(Text)`),
which would render navy-on-near-white — not the reference's teal
highlight. So selection is a distinct concept, wired explicitly per list:

```go
// styleList applies p's selection colors to l. Every tview.List the shell
// constructs (queues, messages, settings, AWS-profile picker) gets the
// same selected-row look.
func styleList(l *tview.List, p config.Palette) *tview.List {
    return l.
        SetSelectedBackgroundColor(tcell.GetColor(p.SelectionBg)).
        SetSelectedTextColor(tcell.GetColor(p.SelectionText))
}
```

### Top bar (`internal/app/topbar.go`)

- `newTopBar` gains a one-column divider (`│`, repeated to the bar's
  height, colored with `Border`) between `left` and the shortcuts panel;
  the existing nested "right" Flex (shortcuts + logo) is flattened into
  four direct children of `root`: left, divider, shortcuts, logo.
- `newShortcutsPanel` prepends a `Navigation:` heading (colored with
  `Label`) and switches key tokens from bare (`q`) to bracketed (`<q>`)
  form. The existing `q`/`quit` double-token line collapses to a single
  `<q> quit` line (it rendered as the slightly nonsensical "q/quit quit"
  before — a byproduct of the format change, not a separate content
  decision).
- `infoPanelText` drops "Profile:"/"Queue Broker:" for three lines —
  "Active connection" (`cfg.Queue.ProxyURL`), "User" (`cfg.Queue.Username`),
  "AWS Profile" (`cfg.AWS.Profile`) — reusing existing config fields, no
  schema change.

### Status bar (`internal/app/statusbar.go`)

```go
// readyStatusText renders the idle-state hotkey legend from cfg — only
// hotkeys App.onGlobalKey actually wires up today.
func readyStatusText(cfg config.Config) string {
    key := func(k, desc string) string {
        return fmt.Sprintf("[%s]%s[-]: %s", cfg.Colors.Accent, k, desc)
    }
    return strings.Join([]string{
        key("?", "Help"), key("h", "Home"), key("s", "Settings"),
        key("q", "Quit"), key("/", "Filter"), key(":", "Command"),
    }, "  ")
}

func newStatusBar(cfg config.Config) *tview.TextView {
    tv := tview.NewTextView().
        SetDynamicColors(true).
        SetTextColor(tcell.GetColor(cfg.Colors.StatusBarText)).
        SetText(readyStatusText(cfg))
    tv.SetBackgroundColor(tcell.GetColor(cfg.Colors.StatusBarBg))
    return tv
}
```

`statusReadyText` (the old parameterless const) goes away; its 5 call
sites in `queues.go` (`a.setStatus(statusReadyText)`, used to restore the
idle state after a transient message) switch to a new `App.readyText()`
method (`return readyStatusText(a.cfg)`), since `queues.go`'s call sites
are `App` methods with `a.cfg` in scope.

### Wiring selection colors + the accent-colored hint (`queues.go`, `settings.go`)

The four `tview.NewList()` construction sites (`queuesList`, `messagesList`
in `queues.go`; the settings list and the AWS-profile picker in
`settings.go`) each get wrapped: `styleList(tview.NewList()..., a.cfg.Colors)`.

`queues.go`'s detail-pane hint (`"[green]a[-] send  [green]d[-] purge..."`)
swaps the hardcoded `green` for `a.cfg.Colors.Accent` — a recolor only,
matching the new palette; wording/format is untouched (that's view
content, out of scope per the spec).

### `config.example.yaml`

Document every new/changed field under `colors:`, with the same values as
`Default()`, and a short comment on `views:` explaining it now ships one
shared color (still overridable per-view).

## Files touched

- `tui/internal/config/config.go` (modified) — `Palette` fields;
  `Default()` values.
- `tui/internal/config/config_test.go` (modified) — `TestDefault` and
  `TestLoadFullOverride` updated to the new field set/values (both
  hardcode the full `Palette`); no other test in this file changes shape,
  since the rest build `want` from `Default()` plus one overridden field.
- `tui/config.example.yaml` (modified).
- `tui/internal/app/theme.go` (new) — `applyTheme`, `styleList`.
- `tui/internal/app/theme_test.go` (new) — see Testing.
- `tui/internal/app/app.go` (modified) — call `applyTheme(cfg.Colors)`;
  `newStatusBar(cfg)`; new `readyText()` method.
- `tui/internal/app/topbar.go` (modified) — divider, nav heading/key
  format, relabeled info panel.
- `tui/internal/app/topbar_test.go` (modified) — updated substring
  assertions; `TestInfoPanelTextShowsConfiguredProfile` checks the third
  line (index 2) instead of the first, since "AWS Profile" moves down;
  a new test asserts the divider column exists.
- `tui/internal/app/statusbar.go` (modified) — `readyStatusText`,
  cfg-driven construction, background/text color.
- `tui/internal/app/statusbar_test.go` (modified) — asserts against
  `readyStatusText(config.Default())` and the configured background
  color instead of a bare constant.
- `tui/internal/app/queues.go` (modified) — `styleList` calls, accent-
  colored hint, `a.readyText()` at the 5 `statusReadyText` call sites.
- `tui/internal/app/queues_test.go` (modified) — same 3 references
  updated to `a.readyText()`.
- `tui/internal/app/settings.go` (modified) — `styleList` calls.

No changes to `internal/ui/views`, `view.go`, `filterable.go`, `help.go`
(already fully cfg-driven — it inherits the new palette automatically),
or anything outside `tui/`.

## Key decisions / trade-offs

- **Theming via one `tview.Styles` mutation at app startup**, not
  per-widget `SetBackgroundColor` calls scattered everywhere. This is the
  intended tview extension point (a public package var), but it's a
  global and order-dependent side effect — it must run before the first
  primitive is constructed, which `applyTheme`'s doc comment and its
  single call site in `New()` make explicit.
- **Selection colors are a separate palette concept from
  background/text**, not derived from them, because tview's own computed
  default for list selection is "invert body text," which doesn't produce
  the reference's teal highlight.
- **`readyStatusText`/`newStatusBar` becoming cfg-dependent is this
  change's one real ripple** — every caller of the old bare
  `statusReadyText` constant needs to become a call, contained entirely
  within `internal/app` (5 sites in `queues.go`, plus both test files).
- **`Views` keeps its 5-entry map, just re-defaulted** — collapsing to a
  shared value rather than removing the map preserves the existing
  per-view-override feature and its tests unchanged in shape.
- **`styleList`'s effect isn't independently unit-testable**: `tview.List`
  exposes setters (`SetSelectedBackgroundColor`/`SetSelectedTextColor`)
  but no getters for the resulting style, so there's no public way to
  assert the colors landed short of reflection. Flagging this explicitly
  per `CLAUDE.md`'s testing section rather than skipping it silently —
  covered by manual/visual verification (a screenshot after `task run:tui`)
  instead. `applyTheme`'s effect *is* testable (via `tview.NewBox()`'s
  resulting `GetBorderColor()`/`GetBackgroundColor()`) and gets a test.

## Testing

- `internal/config`: `TestDefault` covers the new fields/values;
  `TestLoadFullOverride` covers a config that overrides every scalar
  field (still expecting the new fields to fall back to `Default()` since
  the YAML in that test doesn't set them — confirms partial-override
  merge behavior extends to the new fields without code changes, per
  `Load`'s existing `cfg := Default(); yaml.Unmarshal(...)` pattern).
- `internal/app/theme_test.go`: `applyTheme(palette)` followed by
  `tview.NewBox()` has `GetBorderColor()`/`GetBackgroundColor()` matching
  the palette.
- `internal/app/topbar_test.go`: info panel contains the three new
  labels and no placeholder once configured (checking the correct line
  index); shortcuts panel contains `Navigation:` and the bracketed key
  tokens; the divider column exists between `left` and the nav panel.
- `internal/app/statusbar_test.go`: idle text matches
  `readyStatusText(cfg)`; background color matches `cfg.Colors.StatusBarBg`.
- `internal/app/queues_test.go`: unchanged behavior, just updated to call
  `a.readyText()` instead of referencing the removed constant.
- Manual verification: `task run:tui` (or the local dev stack), eyeballed
  against the reference screenshots for background/label/value/accent/
  selection/status-bar colors.
