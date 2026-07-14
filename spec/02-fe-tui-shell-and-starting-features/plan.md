# Plan — TUI shell: starting behavior, layout, and features

Spec: [spec.md](spec.md)

## Approach

### App skeleton

`tview`/`tcell` as the only initial dependencies. `ui.View` is a small
three-method interface so `internal/app` can register/switch views
without a type dependency on `internal/ui/views`. One `placeholder`
struct backs all three initial resource views (a bordered `TextView`
with title/description/"not yet implemented") rather than three
near-duplicate types.

### Layout (`internal/app`)

Root `tview.Flex` (rows): `topBar` (height computed as `max(2, 3,
len(cfg.Logo))`), the existing `Pages` area (unchanged, just demoted to
the middle row), `statusBar` (single unbordered line). `topBar` is a
`Flex` of columns: `topLeft` (a `tview.Pages` with `"info"`/`"prompt"`/
later `"filter"` pages — swapping the whole panel rather than trying to
swap a fixed `Flex` child, which `tview.Flex` doesn't support cleanly),
a divider column, and a shortcuts+logo panel.

### Config (`internal/config`, new package)

```go
type Config struct {
    Logo   []string
    Colors Palette
    AWS    AWSConfig
}

type Palette struct {
    Background, Border, Label, Text, Value, Accent string
    Success, Warning, Error                         string // schema-only, unused
    SelectionBg, SelectionText                       string
    StatusBarBg, StatusBarText                       string
    Views map[string]string // view name -> color, falls back to Border
}

func (p Palette) ViewColor(name string) string {
    if c, ok := p.Views[name]; ok && c != "" { return c }
    return p.Border
}
```

`Load` starts from `Default()` and unmarshals YAML on top, so a config
file overriding only part of `Colors` still gets defaults for the rest.
`LoadDefault` resolves `config.yaml` in the working directory (Task's
targets all run with `dir: tui`); a missing file isn't an error.
`gopkg.in/yaml.v3` is the one new dependency — no stdlib YAML support,
and it's the de facto standard for Go. `Default()` ships the palette
described in `spec.md`: navy background, orange labels, cyan values,
pink/magenta accents, teal selection, orange status bar.

An earlier iteration of `Default()` gave each of `home`/`secrets`/
`params`/`queues`/`settings` its own `Views` entry for a k9s-style
per-view border color, wired via a `bordered` structural interface
(`SetBorderColor`/`SetTitleColor`, satisfied by anything embedding
`*tview.Box`) applied in `App.New()`'s view-registration loop — kept
`internal/ui/views` free of any `internal/config` dependency, since
`internal/app` (which already depends on both) did the wiring. That
per-view rainbow scheme was superseded once Philipp supplied reference
screenshots calling for one neutral border color throughout instead;
`Views` and `ViewColor` stayed as the override mechanism (and its
tests), just re-defaulted to a single shared color.

### Global theming (`internal/app/theme.go`)

Confirmed against the `rivo/tview` v0.42.0 source that primitives
capture `tview.Styles` once, at construction time (`NewBox`'s
`backgroundColor: Styles.PrimitiveBackgroundColor`, `NewList`'s
`selectedStyle: Foreground(Styles.PrimitiveBackgroundColor)
.Background(Styles.PrimaryTextColor)`, etc.) — so a single mutation of
`tview.Styles`, done before any primitive is built, themes the whole
shell:

```go
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

Called first thing in `App.New()`, before `proxy.New(...)` or the
`a.views` slice literal — this superseded the per-view `bordered`-
interface wiring above as the shell's primary theming mechanism, since
global `tview.Styles` mutation covers every widget without per-widget
calls. Selection color is a deliberate exception: tview's own computed
default selection style just inverts body text, which wouldn't produce
a distinct teal highlight, so `styleList` wires `SelectionBg`/
`SelectionText` explicitly onto every `tview.List` the shell constructs
(queues, messages, settings, AWS-profile picker).

### Top bar and status bar

Top bar: a one-column `│` divider (colored with `Border`) between the
info panel and the nav panel; the nav panel gets a `Navigation:` heading
and switches key tokens from bare (`q`) to bracketed (`<q>`) form.
`infoPanelText` shows three lines ("Active connection"/"User"/"AWS
Profile") reusing existing config fields. Status bar: `readyStatusText
(cfg)` renders the idle-state hotkey legend on the `StatusBarBg`
background; transient messages (`setStatus(...)`) still temporarily
override it exactly as before.

### Global hotkeys + help modal

`onGlobalKey` checks focus first (prompt/filter-focused → pass through
unmodified), then whether the help modal is open (swallow everything
except `?`/Escape), then dispatches on rune: `:` (focus prompt via
`topLeft`), `h`/`s` (`switchTo`), `q` (stop), `?` (toggle a `rootPages`
overlay — `ShowPage`/`HidePage`, not `SwitchToPage`, since help must
draw on top of the main layout, not replace it), `/` (`beginFilter`,
a no-op unless the active view implements `Filterable`).

```go
type Filterable interface {
    Filter(query string)
}
```

The filter input mirrors the command prompt exactly: a third `topLeft`
page, its own `InputField`, a `SetDoneFunc` that calls `Filter` on the
active view (found via `activeView()`, matching `pages.GetFrontPage()`
against the registered views) if it implements the interface.

### AWS profile selection

`internal/awsprofile` (stdlib only — no `aws-sdk-go-v2` yet, since
nothing here makes a real AWS call):

```go
func List() ([]string, error)
func ListFrom(configPath, credentialsPath string) ([]string, error)
```

`[default]`/`[profile <name>]` sections in `~/.aws/config` map to
profile names; every section in `~/.aws/credentials` is a profile name
directly. `settings` moves into `internal/app` (the one exception to
"views are stateless placeholders" so far) since it needs live config
read/write and to trigger the picker overlay — the same reasoning that
will later move `queues` there too. The picker is a `tview.List` shown
via a `rootPages` overlay (same `ShowPage`/`HidePage` pattern as help),
pre-selecting `AWS_PROFILE`/`AWS_DEFAULT_PROFILE`/`default` if found
among the discovered names. Selecting persists via `config.Save`
(logged, not fatal, on failure) and calls `refreshInfoPanel` to update
the top bar immediately.

## Files touched

- `go.mod`/`go.sum`, `cmd/tui/main.go`, `internal/ui/view.go`,
  `internal/ui/views/{placeholder,secrets,params,queues,home}.go`
- `internal/config/config.go` (+ `config_test.go`)
- `internal/app/{app,topbar,statusbar,help}.go` (+ tests)
- `internal/app/theme.go` (+ `theme_test.go`) — `applyTheme`/`styleList`
- `internal/ui/filterable.go`
- `internal/awsprofile/awsprofile.go` (+ `awsprofile_test.go`)
- `internal/app/{settings,queues}.go` (+ tests) — moved out of
  `internal/ui/views`; `styleList` wiring on every list they construct
- `config.example.yaml`, root `.gitignore` (`tui/config.yaml`)

## Key decisions / trade-offs

- Prompt/filter overlay as `Pages` pages, not swapped `Flex` children —
  native `SwitchToPage` support versus `tview.Flex` having no clean
  child-swap primitive.
- `settings` (and later `queues`) live in `internal/app`, not
  `internal/ui/views` — they need things (config, overlay control, live
  backends) the views package intentionally doesn't depend on.
- No `aws-sdk-go-v2` dependency yet — plain section-name parsing is
  simple enough without it; the SDK arrives once a feature actually
  calls AWS.
- `Filterable` scaffolded with zero implementers — a contract for future
  list/table views rather than filtering bolted on ad hoc later.
- Persistence failures (`config.Save`) are logged, not fatal — a
  read-only `config.yaml` shouldn't crash the picker.
- Global theming via one `tview.Styles` mutation, not per-widget calls —
  the intended tview extension point, at the cost of being
  order-dependent (must run before any primitive exists). Tried a
  per-view `bordered`-interface approach first; abandoned once the
  reference look called for one neutral color throughout rather than
  per-view distinctiveness.
- Selection colors are wired explicitly per list rather than riding on
  `applyTheme`, since tview's own default would just invert body text.
- `Views` keeps its 5-entry map even after collapsing to one shared
  color — preserves the override mechanism (and its tests) for anyone
  who wants per-view colors back via `config.yaml`.

## Testing

- `internal/config`: defaults, load/save round-trip, partial-override
  merge behavior, `ViewColor` fallback.
- `internal/app`: hotkey routing (including prompt/filter-focused
  no-ops), help modal open/close, prompt and filter overlay show/hide,
  settings list + picker (select/cancel) against fixture `~/.aws` files
  via `t.Setenv`, top bar reflecting the selected profile; `applyTheme`
  followed by a fresh `tview.NewBox()` has matching
  `GetBorderColor()`/`GetBackgroundColor()`; info panel/shortcuts
  panel/status bar content and colors; divider column exists.
- `internal/awsprofile`: both file formats, merge/de-dup, missing files,
  env var overrides, excluded non-profile sections (`sso-session`).
- `internal/ui/views`: each constructor's `Name()`/`Title()`.
- `styleList`'s effect isn't independently unit-testable — `tview.List`
  exposes setters but no getters for the resulting selection style;
  verified by manual/visual check instead, noted explicitly rather than
  skipped silently.
