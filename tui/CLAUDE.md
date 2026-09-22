# CLAUDE.md — tui module

Go-specific conventions for `tui/`. Repo-wide rules (workflow gating,
`spec/` conventions, cross-platform constraints) live in the root
`CLAUDE.md` and apply here too; this file only adds what's specific to
this module.

## Style and formatting

- `gofmt`/`goimports` formatting is mandatory; run before every commit.
- Errors are wrapped with context: `fmt.Errorf("...: %w", err)`, never
  discarded or logged-and-swallowed.
- Idiomatic Go naming (MixedCaps, no underscores); avoid package-name
  stutter.
- No package-level mutable state.

## Package layout

- `cmd/cloudtui/` — entrypoint only (`main.go`); no logic beyond wiring.
- `cmd/seedqueue/` — entrypoint only; dev tool that sends sample JSON
  messages to a queue (`task seed:queue -- <queue> <count>`).
- `cmd/devtool/` — entrypoint only; dev tool for live-testing setup: create/
  remove disposable queues via JMX, start/stop a local mq-proxy instance
  (`task test:queue:add`/`remove`, `task dev:proxy:start`/`stop`).
- `internal/app/` — the application shell: layout, global hotkeys, view routing.
- `internal/dialog/` — modal overlay types (confirm, connection
  manager/editor, message filter, time range, send message, snippet
  browser/picker/save/editor, text prompt, Datadog/AMQ Manager settings,
  theme/AWS profile pickers) implementing internal/ui's Host contract.
- `internal/ui/` — the `View`/`Host`/`ViewHost` interfaces shared across
  resource views.
- `internal/ui/views/` — the home dashboard's table rendering (`home.go`);
  not to be confused with `internal/view/`.
- `internal/view/` — the resource views themselves (queues, messages,
  the snippet library, SSM parameters, Secrets Manager, CloudWatch/Datadog
  logs, CodePipeline, Settings, Log, and each one's detail view), each
  depending on `ui.ViewHost` rather than `internal/app`'s concrete `*App`.
- `internal/queue/` — `Backend` interface and `Summary` type for queue data sources.
- `internal/queue/jolokia/` — Jolokia HTTP client implementing `queue.Backend`.
- `internal/queue/secretbackend/` — `queue.Backend` decorator that
  resolves a connection's password from AWS Secrets Manager on first
  use, caching and transparently re-resolving on a stale/rotated
  secret (see `spec/56-fe-amq-connection-aws-secret-password`).
- `internal/seed/` — sample JSON message generation, used by `cmd/seedqueue`.
- `internal/snippet/` — the message snippet library: the file format
  (optional `jmsType` front matter plus the body, which is JSON/XML
  formatted on save), the `~/.cloudtui/snippets/` folder store, and
  reading files to import (see `spec/22-message-snippets`). No UI
  dependency.
- `internal/devtool/` — JMX queue admin and mq-proxy process management,
  used by `cmd/devtool`.
- `internal/awsprofile/` — read-only discovery of AWS CLI profiles from
  `~/.aws/config`/`~/.aws/credentials` (via `aws-sdk-go-v2/config`), shown
  in Settings → AWS Profiles. No credential resolution, no AWS API calls.

## Testing

- Standard library `testing` only — no assertion library. Table-driven
  tests where a function has multiple cases; `t.Helper()` on test
  helpers; `t.TempDir()`/`t.Setenv()` for filesystem/env-dependent tests.
- One `_test.go` file per source file, same package (no separate `_test`
  package), colocated in the same directory.
- If something is genuinely untestable, say so explicitly in the test file
  or the relevant spec's `plan.md`, and verify manually instead.
- For changes touching queue/message/connection behavior, "verify manually"
  means driving the real TUI against a real broker, not just reading the
  code — use the `verify-live` skill (`.claude/skills/verify-live/`), which
  also covers `task seed:queue`, `task test:queue:add`/`remove`, and
  `task dev:proxy:start`/`stop`. Record what you checked in the feature's
  `tasks.md`.
- `task smoke:test` (`tui/scripts/smoke-test.sh`) automates the golden path
  (list/browse/mark/delete/move messages, switch to the proxy backend) as a
  quick regression check — run it after changes likely to affect that path,
  but it doesn't replace verifying a *new* feature's specific behavior by
  hand (see the `verify-live` skill for why).

## tview gotchas (v0.42)

Traps hit in this codebase that aren't obvious from tview's API. Read
this before adding or restyling a widget.

- **`tview.Modal` focuses its *last* button.** Enter then triggers the
  last button (usually Cancel), not the confirm action. Confirmation
  dialogs use a `tview.List` instead, so the cursor starts on item 0
  (see `dialog.ConfirmDialog`).
- **`NewTableCell(...).SetTextColor(c)` stores the color in the cell's
  `Style`**, not the legacy `Color` field. Read it back with
  `fg, _, _ := cell.Style.Decompose()`.
- **`[...]` in table cells and list items is parsed as a color tag**,
  with no per-cell opt-out. Pass untrusted or user text through
  `tview.Escape`, and use glyphs (`✓`, `⭐`) rather than `[x]` for
  markers.
- **Autocomplete: style first, then wire.** Call
  `ui.StyleInputFieldAutocomplete` *before*
  `InputField.SetAutocompleteFunc`. `SetAutocompleteFunc` builds the
  drop-down's internal list immediately, with whatever styles are set at
  that moment. tview keeps that list until the field loses focus or has
  no suggestions, so styles set later don't reach it. That's why
  `StyleInputFieldAutocomplete` blurs an unfocused field after
  restyling.
- **Widgets copy `tview.Styles` at construction.** Setting
  `tview.Styles` later (a live theme switch) doesn't reach widgets that
  already exist. What keeps its construction-time colors unless
  re-applied:
  - `List`: unselected rows
  - `Form`: label color, field style, button styles
  - `DropDown`: focused, prefix and disabled styles
  - `TextView`: base text color, used by untagged characters
  - table cells: colored when set

  Every `ApplyPalette` re-applies them through the `ui.Style*` helpers
  (`StyleList`, `StyleForm`, `StyleFilterInput`,
  `StyleDropDown`/`StyleFormDropDown`, `StyleInputFieldAutocomplete`).
  It also redraws table headers and resets borders colors. See
  `spec/04-theming`.
- **An `InputField` has two backgrounds.** It wraps a private `TextArea`
  whose own box background is what the label is drawn on. Only
  `SetFormAttributes` reaches it; `SetLabelColor`,
  `SetFieldBackgroundColor` and `SetBackgroundColor` don't. The outer
  box background paints wherever the field doesn't reach (beside it,
  below it), so reset both.
- **`tview.Form` moves focus after an item's done func.** Its own
  "finished" handler runs right after `SetDoneFunc`'s callback and
  focuses the next element through the application's `setFocus`. That
  steals focus from any overlay the callback just opened, e.g. a
  confirmation. Handle Enter in the item's input capture and swallow it
  instead (see `dialog.SnippetSaveDialog`).
- **`tview.Form` remembers its focused item across `Show` calls.** Reset
  it with `form.SetFocus(0)` when reopening a form.
- **Testing rendering:** draw the primitive on a
  `tcell.NewSimulationScreen` and read `GetContents()` for runes and
  styles; `internal/ui/style_test.go` has helpers. For a theme fix,
  compare a live switch against a restart cell by cell. Always
  mutation-check a new test: remove the fix and confirm the test fails.
  Several first attempts here passed while testing nothing (a build
  error counted as "no failures", test data that never hit the code
  path).

## Dependencies

- Currently: `tview`/`tcell` (UI), `aws-sdk-go-v2/config` (AWS shared
  config/credentials file parsing — justification in
  `spec/28-fe-aws-profile-discovery/plan.md`).
- Justify any new dependency in the relevant spec's `plan.md` before
  adding it to `go.mod`.
