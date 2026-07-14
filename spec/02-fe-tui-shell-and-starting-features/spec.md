# TUI shell: starting behavior, layout, and features

Date: 2026-07-07 to 2026-07-10 (condensed 2026-07-14 from four originally
separate entries: the app scaffold, the k9s-style shell layout, global
hotkeys, and AWS profile selection).

## Feature

Stand up the tui's initial shell and its first real, interactive
capability: the tview application skeleton, a k9s-style three-row layout
driven by a user-configurable YAML file (logo + color palette), global
single-key hotkeys alongside the `:` command prompt, and the first
feature backed by something outside the process itself — local AWS
profile discovery and selection, persisted across restarts.

## What

- **App skeleton.** Go module `github.com/ePex/cloudtui/tui`;
  `cmd/tui/main.go` entrypoint; a `ui.View` interface (`Name`, `Title`,
  `Primitive`) so the shell can register/switch views without knowing
  their concrete type; a shared `placeholder` view type backing
  `secrets`/`params`/`queues` until each gets a real implementation.
- **Three-row k9s-style layout.** Top bar (a connection-info panel that
  swaps out for the `:` command prompt or the `/` filter input while
  either is active, plus a shortcuts+logo panel), the existing `Pages`
  area in the middle, a bottom status bar reserved for transient
  loading/error text. A new `internal/config` package loads the ASCII
  logo and color palette from a gitignored `config.yaml`, falling back
  to built-in defaults with zero config present; `config.example.yaml`
  documents the schema.
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
matches how the AWS CLI already presents the choice.

## Scope

- `internal/ui/view.go`, `internal/ui/views/placeholder.go`, and the
  `secrets`/`params`/`queues` placeholder constructors.
- `internal/config`: `Config`/`Palette`/`AWSConfig`, `Default`/`Load`/
  `LoadDefault`/`Save`/`SaveDefault`.
- `internal/app`: the three-row layout (`topbar.go`, `statusbar.go`),
  global hotkey routing, the help modal, the `Filterable` contract, and
  `home`/`settings` views (`settings` living in `internal/app`, not
  `internal/ui/views`, since it needs live config read/write).
- `internal/awsprofile`: profile discovery, stdlib only.
- Unit tests throughout: config load/save/defaults, hotkey routing
  (including the prompt/filter-focused no-op cases), help modal
  show/hide, profile discovery (both file formats, merge/de-dup, missing
  files, env overrides), settings list + picker behavior.

## Out of scope

- Real secrets/parameters behavior — those views stay placeholders.
- Queues/mq-proxy — see `03-fe-mq-proxy`.
- Validating the selected AWS profile against AWS in any way (no STS
  call, no credential resolution) — this is name selection only.
- The specific color palette values shipped this pass — later revisions
  (including a full re-theme) are tracked separately, see
  `04-cr-shell-color-palette-evolution`.
