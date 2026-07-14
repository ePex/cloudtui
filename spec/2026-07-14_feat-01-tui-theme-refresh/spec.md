# 2026-07-14 — TUI theme refresh

## Feature

Replace the shell's current color palette and top/bottom bar layout with a
new default derived from a set of reference screenshots (an existing
"AMQ Manager" TUI Philipp likes the look of). This is a re-theme, not a
re-feature: the goal is that `cloudtui` *ships* with this look out of the
box, while staying just as overridable via `config.yaml` as today.

Reference screenshots (for the record — not committed, described here):
1. Home screen — top info panel + nav panel + logo, bordered menu list below.
2. Settings/connections list — same chrome, plain (unbordered) grouped
   tables below.
3. A connection "edit" form — bordered box, labeled fields, teal buttons.
4. A queues list — bordered box, colored column-header row, colored
   selection.

## Why

Philipp wants `cloudtui`'s default look to match this reference: a dark
navy background, orange labels, cyan values, pink/magenta key-binding
tokens, teal selection highlighting, and an orange bottom status bar
functioning as a persistent hotkey legend — instead of the current
rainbow-bordered, default-terminal-background look. Exact per-view content
(tables, columns, sort indicators, colored severity states) is explicitly
**not** part of this pass — those get designed view-by-view later. This
pass is about the shell's chrome: the persistent frame every view sits in.

## What

### Color palette (`internal/config.Palette`)

Existing fields get new default values; a few fields are added because
nothing in the current palette can express them:

- `background` *(new)* — the app's base background, applied globally via
  `tview.Styles.PrimitiveBackgroundColor` (and the two contrast variants)
  so every primitive is dark-navy by default without per-widget wiring.
  Default: `#1a1b26`.
- `border` — box borders/titles. Default changes from `green` to a muted
  light blue-white (`#c0caf5`), matching the reference's neutral thin
  borders.
- `label` — field labels (`Active connection:`, `User:`, section headers
  like `--- ActiveMQ ---`). Default changes from `yellow` to an amber
  orange (`#e0af68`).
- `text` *(new)* — general/primary text color (list item text, body copy),
  applied globally via `tview.Styles.PrimaryTextColor`. Default: `#c0caf5`
  (near-white, slightly cool).
- `value` — field values in label/value rows (`queue_mgmt`, `mlf-testt`).
  Default changes from `white` to cyan (`#7dcfff`).
- `accent` — key-binding tokens in the shortcuts/nav panel and help modal.
  Default changes from `aqua` to pink/magenta (`#ff79c6`), matching the
  reference's `<Enter>`, `</>`-style tokens.
- `success` / `warning` / `error` — unchanged role (still schema-only, no
  feature renders these yet); retinted to fit the new palette:
  `#9ece6a` / `#e0af68` / `#f7768e`.
- `selectionBg` / `selectionText` *(new)* — selected-row colors, wired to
  every `tview.List` the shell currently constructs (queues, messages,
  settings, AWS-profile picker) via `SetSelectedBackgroundColor` /
  `SetSelectedTextColor`. Default: `#2ac3de` / `#1a1b26` (teal bg, dark
  text) — the reference's selected-row look.
- `statusBarBg` / `statusBarText` *(new)* — the bottom bar's background
  and text color. Default: `#ff9e64` / `#1a1b26` (orange bar, dark text).
- `views` (per-view border/title color) — the map's five default entries
  all collapse to the same value as `border` (instead of today's
  aqua/yellow/teal/fuchsia/gray rainbow), so views share one neutral
  border by default. The map itself, and per-view overriding, stays as a
  feature — a user who wants k9s-style per-view colors can still set them
  in `config.yaml`.

`config.example.yaml` gets updated to document every new/changed field.

### Layout

- **Top bar divider.** A one-column vertical rule (`│`, colored with
  `border`) is added between the connection-info panel and the
  shortcuts/nav panel, matching the reference's three-column top bar
  (info | nav | logo).
- **Nav panel heading + key style.** The shortcuts panel gains a
  `Navigation:` heading line (colored with `label`, since it plays the
  same "section heading" role as `--- ActiveMQ ---`), and its key tokens
  switch from bare characters (`q`) to angle-bracket form (`<q>`), matching
  the reference.
- **Status bar becomes a hotkey legend.** Instead of a plain-text,
  unstyled strip that only ever says "cloudtui ready" or a transient
  status message, the bottom bar renders on the `statusBarBg` background
  and shows the currently-implemented global hotkeys as its idle state:
  `?: Help  h: Home  s: Settings  q: Quit  /: Filter  :: Command`. Transient
  messages (`Loading queues…`, error text) still temporarily replace this
  text via the existing `setStatus`/`statusReadyText` mechanism — only the
  idle text and the bar's background/text color change.
- **Info panel labels.** The connection-info panel's two lines
  (`Profile:`, `Queue Broker:`) are relabeled to three lines matching the
  reference's `Active connection:` / `User:` / `AWS Profile:`, reusing
  existing config data: `Queue.ProxyURL` → "Active connection" (the
  reference shows a friendly connection name, but `ProxyURL` is the field
  that actually exists today), `Queue.Username` → "User", `AWS.Profile` →
  "AWS Profile". No new config fields — just relabeling/reordering what's
  already there.

### Out of scope

- Any specific view's content: the Queues table's columns/sorting, the
  Settings connections list's grouped/unbordered layout, the "Connection
  Details" edit form, colored pending/consumer counts, etc. Those are
  real features to design later, not shell chrome.
- The reference's specific ASCII art logo. `cfg.Logo` stays
  user-customizable as today; only its position (right of the nav panel,
  past the new divider) is unchanged.
- Distinct styling for the status bar's transient (loading/error) state
  vs. its idle hotkey-legend state — both use the same bar colors this
  pass.
- Any new hotkeys (the reference shows `l: Logs` / `L: AWS Login`, which
  don't exist in `cloudtui` yet) — the legend only lists hotkeys that
  already work today.
- Config migration/validation tooling for users with an existing
  `config.yaml` — partial-override merge behavior (already tested)
  continues to apply; existing files simply keep whatever they already
  override and inherit new fields' defaults.

## Scope

- `internal/config`: `Palette` gains `background`, `text`, `selectionBg`,
  `selectionText`, `statusBarBg`, `statusBarText`; `Default()`'s values
  updated as described above (including collapsing `views` to one shared
  color); `config.example.yaml` updated to match.
- `internal/app`: a small theme-application step (new file, e.g.
  `theme.go`) that sets `tview.Styles` from `cfg.Colors` once at startup,
  before any primitive is constructed; the four existing `tview.List`
  construction sites get explicit selection-color calls; `topbar.go`
  gets the divider column, the `Navigation:` heading, `<key>` formatting,
  and the relabeled info panel; `statusbar.go` gets the background color
  and the new idle-text hotkey legend.
- Unit tests: updated `config_test.go` expectations for the new `Default()`
  values and fields; updated `topbar_test.go` assertions for the new info
  panel labels and nav panel heading/key format; updated `statusbar_test.go`
  for the new idle text; a new small test asserting the divider exists and
  a List's selection colors match `cfg.Colors.SelectionBg/Text`.
