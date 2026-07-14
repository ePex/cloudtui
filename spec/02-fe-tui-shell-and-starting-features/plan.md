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
the middle row), `statusBar` (single unbordered line, structural
placeholder until a later feature gives it real content). `topBar` is a
`Flex` of columns: `topLeft` (a `tview.Pages` with `"info"`/`"prompt"`/
later `"filter"` pages — swapping the whole panel rather than trying to
swap a fixed `Flex` child, which `tview.Flex` doesn't support cleanly)
and a shortcuts+logo panel.

### Config (`internal/config`, new package)

```go
type Config struct {
    Logo   []string
    Colors Palette
    AWS    AWSConfig
}
```

`Load` starts from `Default()` and unmarshals YAML on top, so a config
file overriding only part of `Colors` still gets defaults for the rest.
`LoadDefault` resolves `config.yaml` in the working directory (Task's
targets all run with `dir: tui`); a missing file isn't an error.
`gopkg.in/yaml.v3` is the one new dependency — no stdlib YAML support,
and it's the de facto standard for Go.

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
- `internal/ui/filterable.go`
- `internal/awsprofile/awsprofile.go` (+ `awsprofile_test.go`)
- `internal/app/settings.go` (+ `settings_test.go`) — moved out of
  `internal/ui/views`
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

## Testing

- `internal/config`: defaults, load/save round-trip, partial-override
  merge behavior.
- `internal/app`: hotkey routing (including prompt/filter-focused
  no-ops), help modal open/close, prompt and filter overlay show/hide,
  settings list + picker (select/cancel) against fixture `~/.aws` files
  via `t.Setenv`, top bar reflecting the selected profile.
- `internal/awsprofile`: both file formats, merge/de-dup, missing files,
  env var overrides, excluded non-profile sections (`sso-session`).
- `internal/ui/views`: each constructor's `Name()`/`Title()`.
