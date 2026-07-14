# TUI shell: starting behavior, layout, and features

Date: 2026-07-07 to 2026-07-14 (condensed 2026-07-14 from six originally
separate entries: the app scaffold, the k9s-style shell layout, global
hotkeys, AWS profile selection, and two color-palette revisions).

## Feature

Stand up the tui's initial shell and its first real, interactive
capability: the tview application skeleton, a k9s-style three-row layout
with the dark, config-driven color palette the shell ships with, global
single-key hotkeys alongside the `:` command prompt, and the first
feature backed by something outside the process itself — local AWS
profile discovery and selection, persisted across restarts.

## What

- **App skeleton.** Go module `github.com/ePex/cloudtui/tui`;
  `cmd/tui/main.go` entrypoint; a `ui.View` interface (`Name`, `Title`,
  `Primitive`) so the shell can register/switch views without knowing
  their concrete type; a shared `placeholder` view type backing
  `secrets`/`params`/`queues` until each gets a real implementation.
- **Three-row k9s-style layout.** Top bar — a connection-info panel
  ("Active connection"/"User"/"AWS Profile", swapped out for the `:`
  command prompt or the `/` filter input while either is active), a
  divider column, a shortcuts/nav panel (`Navigation:` heading,
  bracketed `<key>` tokens), and the ASCII logo. The existing `Pages`
  area sits in the middle. A bottom status bar shows a persistent
  hotkey legend (`?: Help  h: Home  s: Settings  q: Quit  /: Filter
  :: Command`) at idle, temporarily replaced by transient
  loading/error text.
- **Color palette.** A dark, config-driven default matching a reference
  TUI Philipp wanted the shell to look like: navy background, orange
  labels, cyan values, pink/magenta key-binding accents, teal list
  selection, an orange status bar. A new `internal/config` package
  loads the ASCII logo and the full palette (background, border, label,
  text, value, accent, selection, status-bar colors, a per-view
  border-color override map, and schema-only success/warning/error
  fields for a later feature to use) from a gitignored `config.yaml`,
  applying it globally via `tview.Styles` at startup so every widget
  picks it up without per-widget wiring; list selection colors are
  wired explicitly per list, since tview's own default would just
  invert body text. Built-in defaults ship so the app runs with zero
  config present; `config.example.yaml` documents the schema.
- **Global hotkeys.** `h` (home), `s` (settings), `q` (quit, alongside
  the existing `:q`/`:quit`), `?` (a dismissable help modal listing every
  binding), `/` (filter, via a `Filterable` interface no view implements
  yet — a forward-looking contract, not live behavior). New `home`
  (default view) and `settings` placeholder views.
- **AWS profile selection.** A new `internal/awsprofile` package scans
  `~/.aws/config`/`~/.aws/credentials` (respecting the `AWS_CONFIG_FILE`/
  `AWS_SHARED_CREDENTIALS_FILE` overrides), merging and de-duplicating
  profile names; missing files aren't an error. The `settings`
  placeholder becomes a real list (structured to grow more rows later)
  with an "AWS Profile" entry; selecting it opens a modal picker
  (pre-selecting `AWS_PROFILE`/`AWS_DEFAULT_PROFILE`/`default` if
  present) that persists the choice to `config.yaml` and updates the top
  bar's connection-info panel.

## Why

`CLAUDE.md`'s architecture section calls for a k9s-style shell (command
prompt, resource views, non-blocking UI) with a configurable look, plus
three AWS-backed resource domains. This is that shell and its first real
feature: every other AWS-backed capability (secrets, parameters, queues)
needs to know which local profile to use, and reading real profile names
out of the user's own `~/.aws` files (rather than requiring a typed name)
matches how the AWS CLI already presents the choice. The palette
specifically matches reference screenshots Philipp provided of an
existing TUI look he wanted this shell to have from the start; an
intermediate per-view rainbow-color iteration was tried and superseded
along the way (see `plan.md`).

## Scope

- `internal/ui/view.go`, `internal/ui/views/placeholder.go`, and the
  `secrets`/`params`/`queues` placeholder constructors.
- `internal/config`: `Config`/`Palette`/`AWSConfig`, `Default`/`Load`/
  `LoadDefault`/`Save`/`SaveDefault`.
- `internal/app`: the three-row layout (`topbar.go`, `statusbar.go`),
  global theming (`theme.go`: `applyTheme`/`styleList`), global hotkey
  routing, the help modal, the `Filterable` contract, and `home`/
  `settings` views (`settings` living in `internal/app`, not
  `internal/ui/views`, since it needs live config read/write).
- `internal/awsprofile`: profile discovery, stdlib only.
- Unit tests throughout: config load/save/defaults, palette
  application, hotkey routing (including the prompt/filter-focused
  no-op cases), help modal show/hide, profile discovery (both file
  formats, merge/de-dup, missing files, env overrides), settings list +
  picker behavior.

## Out of scope

- Real secrets/parameters behavior — those views stay placeholders.
- Queues/mq-proxy — see `03-fe-mq-proxy`.
- Validating the selected AWS profile against AWS in any way (no STS
  call, no credential resolution) — this is name selection only.
- Any specific view's actual content (Queues table, Settings list,
  forms) beyond the shell chrome described above.
- Distinct styling for the status bar's idle vs. transient
  (loading/error) state — both use the same bar colors.
