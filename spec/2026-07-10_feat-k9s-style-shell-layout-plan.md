# Plan — k9s-style shell layout

Spec: [2026-07-10_feat-k9s-style-shell-layout.md](2026-07-10_feat-k9s-style-shell-layout.md)

## Approach

### Layout (`internal/app`)

Root `tview.Flex` (rows), replacing the current header/prompt/pages stack:

```
topBar      (fixed height, computed — see below)
pages       (flexible, unchanged — existing Pages area)
statusBar   (fixed, 1 line)
```

`topBar` is a `Flex` (columns):

- **Left — `topLeft`, a `tview.Pages` with two pages:**
  - `"info"` — a `TextView` with two placeholder lines (`Profile:` /
    `Queue Broker:`), visible by default.
  - `"prompt"` — the existing `*tview.InputField`, added as a page item
    instead of a standalone Flex row.
- **Right — shortcuts + logo**, a `Flex` (columns) of two `TextView`s:
  a fixed key-binding list (`:` command, `q`/`quit`, `esc` cancel) and the
  configured ASCII logo, right-aligned.

`topBar`'s height is computed at construction time as
`max(2 /* info lines */, 3 /* shortcut lines */, len(cfg.Logo))` — so a
longer custom logo isn't clipped, and a short one doesn't leave the row
oversized.

`statusBar` is a single unbordered `TextView` rendering a fixed placeholder
string (no dynamic content yet — see spec's Out of scope).

### Prompt overlay behavior

- `onGlobalKey`: unchanged trigger (`:` when the prompt doesn't have
  focus), but now also calls `topLeft.SwitchToPage("prompt")` before
  focusing the prompt.
- `onPromptDone`: the existing `defer` (clear prompt text, refocus
  `pages`) gains `topLeft.SwitchToPage("info")`. The Enter-key routing
  logic (`q`/`quit` stops the app, known name switches view, unknown name
  is a no-op) is unchanged.

None of the top/bottom bar panels take focus — only `pages` and, while
active, `prompt` do, same as today.

### Config (`internal/config`, new package)

```go
type Config struct {
    Logo   []string
    Colors Palette
}

type Palette struct {
    Border string // main-view border color (existing placeholder borders — unchanged this pass, see decisions)
    Label  string // field labels, e.g. "Profile:"
    Value  string // field values / default text
    Accent string // key-binding tokens in the shortcuts panel
}

func Default() Config
func Load(path string) (Config, error)      // parses YAML at path; Default() if path doesn't exist
func LoadDefault() (Config, error)          // resolves the user config path and calls Load
```

- `Load` starts from `Default()` and unmarshals the YAML on top of it, so
  a config file that only overrides part of `Colors` still gets defaults
  for the rest (both `yaml.v3` and the caller's zero-alloc struct reuse
  make this free — no manual field-by-field merging).
- `LoadDefault` resolves the path as `config.yaml` in the current working
  directory (Task's `build:tui`/`run:tui`/`test:tui` targets all set
  `dir: tui`, so this is `tui/config.yaml` under normal dev usage); if
  the file is absent, falls back to `Default()`. This is a repo-local
  file, gitignored via a new `tui/config.yaml` entry in the root
  `.gitignore` (the existing `*.local.yaml` rule doesn't match this
  filename, and renaming to fit it was rejected in favor of the plainer
  `config.yaml`).
- Colors are plain strings (hex or W3C name), fed directly into tview's
  `[color]` dynamic-color tags — no `tcell.Color` conversion needed for
  the (unbordered) top/bottom bar text.
- Default palette: `Border: "green"`, `Label: "yellow"`, `Value: "white"`,
  `Accent: "aqua"`. Default logo: a small bordered `CLOUDTUI` wordmark
  (3 lines), swappable via config.
- `App.New()` calls `config.LoadDefault()`; on error, falls back to
  `Default()` and prints a one-line warning to stderr (config loading
  must never block startup).

### New files

- `tui/internal/config/config.go`, `config_test.go`
- `tui/internal/app/topbar.go` (`topLeft`/info panel + shortcuts/logo
  panel construction), `topbar_test.go`
- `tui/internal/app/statusbar.go` (bottom bar construction),
  `statusbar_test.go`
- `tui/config.example.yaml` — checked-in schema example/template
  (documentation only, not auto-loaded; a user copies it to
  `config.yaml` to customize)

### Modified files

- `tui/internal/app/app.go` — `App` struct gains `topLeft *tview.Pages`
  and `cfg config.Config`; `New()` builds the new layout;
  `onGlobalKey`/`onPromptDone` gain the overlay switch.
- `tui/internal/app/app_test.go` — existing assertions (routing, quit,
  unknown command, focus-to-pages) stay the same; add assertions that
  `topLeft`'s front page is `"prompt"` while active and reverts to
  `"info"` afterward.
- `tui/go.mod`/`go.sum` — add `gopkg.in/yaml.v3` (no YAML support in the
  standard library; it's the de facto standard for Go, single
  well-maintained dependency).
- `.gitignore` (root) — add `tui/config.yaml` under the existing "Local
  configuration & secrets" section.

## Key decisions / trade-offs

- **Prompt as a `Pages` page, not a swapped Flex item.** `tview.Flex`
  doesn't support cleanly swapping a child at a fixed slot; a small
  `Pages` with two named pages (`"info"`/`"prompt"`) does this natively
  via `SwitchToPage` and is the same pattern already used for the main
  view area.
- **Palette applies to new shell chrome only, not existing view borders.**
  The approved spec scoped this feature to the top/bottom bars and
  prompt overlay; the existing placeholder views' border color is
  unchanged in this pass. `Palette.Border` is defined now (for
  forward compatibility if/when the views get recolored) but nothing
  reads it yet — flagging this explicitly since an unread field could
  otherwise look like dead code; it will be consumed at whatever point
  the resource views themselves are worked on.
- **Config lives in the repo working tree, as a gitignored local file
  named plainly `config.yaml`.** `tui/config.yaml` is the actual config a
  developer edits, ignored via a new (path-specific, not a blanket
  `*.yaml`) `.gitignore` entry; `config.example.yaml` is committed as the
  schema template. Simpler than a user-config-dir lookup and keeps the
  filename unsurprising, at the cost of one new `.gitignore` line instead
  of reusing the existing `*.local.yaml` pattern.
- **No color-name validation beyond what tview/tcell already do.** An
  invalid color string in a user's config degrades to tview's own
  fallback (it silently ignores unrecognized tags) rather than the app
  crashing — acceptable since this is cosmetic, not functional.
- **`gopkg.in/yaml.v3` is the one new dependency.** Justified per
  CLAUDE.md ("dependencies are deliberate... prefer stdlib where
  reasonable") since there's no stdlib YAML support and this is the
  standard choice in the Go ecosystem.

## Testing

- `internal/config`: `Default()` returns the documented default
  logo/palette; `Load()` on a missing path returns `Default()`; `Load()`
  on a temp file with full overrides returns exactly those values;
  `Load()` on a temp file with a partial `colors:` block merges with
  defaults for the untouched fields; `LoadDefault()` resolves to
  `config.yaml` in the working directory.
- `internal/app/topbar_test.go`: the info panel's rendered text contains
  the placeholder profile/broker labels; the shortcuts panel's text
  contains the three documented bindings; the logo panel's text matches
  `cfg.Logo` joined with newlines.
- `internal/app/statusbar_test.go`: the bar renders as a single-line,
  unbordered `TextView` with the placeholder text.
- `internal/app/app_test.go`: extend the existing `onGlobalKey`/
  `onPromptDone` tests to also assert `topLeft`'s front page switches to
  `"prompt"` and back to `"info"`.
