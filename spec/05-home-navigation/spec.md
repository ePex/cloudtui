# Home screen and global navigation legend

_Condensed from spec/05-cr-home-view-sections, spec/54-cr-home-screen-sections, spec/30-bugfix-home-context-panel-shortcuts, spec/31-bugfix-status-bar-duplicate-legend — see those folders for the incremental history._

## Purpose

Home is a keyboard-navigable launcher, sectioned by backend, rather than a flat non-interactive list. The global hotkey legend is shown reliably regardless of what the status bar happens to be displaying.

## Behavior

### Home screen

- A sectioned, keyboard-navigable list, rendered as a single-column `tview.Table` (one row per section header or entry). Arrow keys (or `j`/`k`) move the cursor between selectable entries; Enter activates the selected view (equivalent to typing its name in the command prompt).
- Section headers are non-selectable rows rendered as a dashed rule that fills the full row width: `─── <Title> ` followed by enough `─` to reach the table's right edge (tview clips the trailing dashes to the exact cell width, so it always spans the full terminal width regardless of size), colored with the palette's border color.
- Each entry is a single row with two columns **on the same line**: the view name, left-aligned and padded to a fixed width, in the palette's label color, followed by its description in the palette's text color — never the name and description stacked on separate lines.
- Selecting a row overrides both columns uniformly with the palette's `selectionBg`/`selectionText`, replacing the row's normal label/text colors — this is required for contrast, since `selectionText` is chosen to be legible against `selectionBg` specifically (e.g. dark text on a bright selection highlight) while the entry's normal per-column colors are not.
- Home does **not** list itself (listing "Home" from within Home is circular).
- Sections are grouped **by backend**, in this order:

  1. **ActiveMQ**: `queues`
  2. **AWS**: `ssm-parameters`, `secrets-manager`, `cloudwatch-logs`, `codepipeline` (in that order)
  3. **Datadog**: `datadog-logs`
  4. **System**: `settings`, `log`

  (An earlier revision used a single flat "Apps" section for all resource views; it was split by backend once AWS alone grew to 4 of 6 entries, since a flat list mixing three unrelated backends stops scaling and doesn't match how a user thinks about the tool — "I want the AWS stuff" vs. "I want the queue browser".)
- Sections are a hardcoded literal in the shell's composition root, not dynamically registered at runtime.

### Global hotkey legend placement

- **Home's own context panel** (top bar, middle section — the same `ui.Shortcuttable` mechanism every view uses for its own shortcuts) shows the five global hotkeys: `? l s q :` (Help, Log, Settings, Quit, Command). `h` (Home) is deliberately **not** included — it would be a no-op reminder to do the thing you're already doing, since it's only ever visible while already on Home.
- **The status bar has no idle default text at all**, on any screen. It is purely a transient-message strip: loading indicators, errors, confirmations (e.g. "Marked N message(s)") appear there and stay until overwritten by the next transient message or a view switch — nothing resets it back to a legend, including a theme switch.
- The full hotkey reference, globally, lives in exactly one place per context: Home's context panel for the global set, `?` (help modal) for the complete list from anywhere, and each other view's own context panel for its view-specific bindings.

(This design is the result of two iterations: giving Home a context-panel legend was the fix for the legend silently vanishing whenever a transient status message overwrote the status bar with nothing to restore it — Home had no fallback of its own. That fix initially left the status bar's own idle-legend default in place too, producing visible duplication on Home specifically; the status bar's idle default was then removed entirely, in favor of the scheme described above.)

## Data & config

`homeSections` is a `[]views.SectionInfo` literal (each with a `Title` and `[]views.ViewInfo{Name, Description}`), constructed once in the shell's composition root and passed to the Home view constructor.

## Implementation notes

- `tui/internal/ui/views/home.go` — `HomeView`: generic sectioned-list rendering (accepts an arbitrary number of named sections; no special-casing of section count or names) and `Shortcuts()` (the five global hotkeys, no `h`). `RepaintHomeTable` builds each row's text as a single cell with color tags (name and description are not separate table columns — the two-column look comes from padding the name to a fixed width within one cell) and sets `SetSelectedStyle` to uniformly recolor whichever row is selected.
- `tui/internal/app/app.go` — `homeSections` literal in `New()` (the only place that encodes which view goes in which section).
- `tui/internal/ui/statusbar.go` — status bar has no idle default text; theme switches only recolor whatever text is currently present, never reset it.
- `tui/internal/ui/shortcuttable.go` — `Shortcuttable` interface, used by both Home and every other view.

## Notable gotchas worth preserving

- Don't reintroduce an idle default string on the status bar — it will silently duplicate whatever a view's own context panel already shows, and go stale the moment a transient message overwrites it with no way back.
- `Shortcuttable`'s original doc comment claimed "the status bar already carries the global hotkey legend, so there is nothing to repeat" — that assumption was wrong (a transient message can permanently evict it for the rest of the session) and the comment has been corrected; don't reintroduce that assumption when adding new views.
