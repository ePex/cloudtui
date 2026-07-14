# Shell color palette evolution

Date: 2026-07-10 (revision 1), 2026-07-14 (revision 2) — condensed
2026-07-14 from two originally separate feature entries. First tracked as
a "change request": both revisions modify shell chrome that
`02-fe-tui-shell-and-starting-features` already shipped, rather than
adding new capability.

## Change request

The shell's color palette (`internal/config.Palette`), first shipped
with `02-fe-tui-shell-and-starting-features`'s built-in defaults, has
been revised twice so far:

**Revision 1 (2026-07-10).** Added per-view border/title colors (each of
`home`/`secrets`/`params`/`queues`/`settings` got its own accent, k9s-
style, falling back to a shared `Border` color if a view wasn't listed)
and schema-only `Success`/`Warning`/`Error` fields for a later feature to
use. This also finally wired up `Border` itself, which
`02-fe-tui-shell-and-starting-features` had defined but nothing read.

**Revision 2 (2026-07-14).** Replaced revision 1's per-view rainbow
scheme entirely with a single new default palette matching a reference
TUI Philipp wanted to match: dark navy background, orange labels, cyan
values, pink/magenta key-binding accents, teal list selection, and an
orange status bar. The per-view color map from revision 1 collapsed to
one shared default (the map mechanism itself stayed, just re-defaulted).
The palette now applies globally via `tview.Styles` at startup instead of
per-widget; new `Background`/`Text`/`SelectionBg`/`SelectionText`/
`StatusBarBg`/`StatusBarText` fields were added to express it. The top
bar gained a divider column and a `Navigation:` heading with bracketed
key tokens; the status bar changed from a static "cloudtui ready" string
to a persistent hotkey legend.

## Why

Revision 1 aimed for k9s-style visual distinctiveness between views.
Revision 2 was a deliberate re-theme toward a specific reference look
Philipp provided screenshots of — superseding revision 1's per-view
scheme rather than building on it, since the reference used one neutral
border color throughout instead of a different color per view.

## Scope

- `internal/config.Palette`: all fields listed above, across both
  revisions.
- `internal/app`: per-view border/title coloring (revision 1);
  `theme.go`'s `applyTheme`/`styleList`, top bar divider/heading/key
  format, status bar hotkey legend (revision 2).
- `config.example.yaml`: kept in sync with the schema at each revision.
- Unit tests for both revisions' config defaults/merge behavior and the
  rendering they drive.

## Out of scope

- Any specific view's actual content (Queues table, Settings list,
  forms) — both revisions are shell chrome only.
- Distinct styling for the status bar's idle vs. transient
  (loading/error) state — both revisions use the same bar colors for
  either.
- Any new hotkeys.
