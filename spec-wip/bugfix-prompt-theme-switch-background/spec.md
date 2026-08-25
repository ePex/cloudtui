# Bugfix: `:` prompt keeps the old theme's background after a live theme switch

Date: 2026-08-25

## Problem

Switching the active theme at runtime (Settings → Theme, or `:theme
<name>`) recolors the app shell via `reapplyTheme`
(`tui/internal/app/theme.go`) — but the `:` command prompt's own visible
background is left on whatever theme was active when the app started.
Everything else around it (status bar, info/divider/context/logo panels,
Home table, every view/dialog's own list or table, and — separately — the
prompt's autocomplete drop-down items, which are rebuilt fresh from
current styles whenever the prompt regains focus) recolors correctly, so
the prompt reads as a visibly stale tile sitting in an otherwise
consistently-themed shell.

## Root cause

**Correction (found during implementation):** the analysis below this
note is what `plan.md` originally shipped with, and it was wrong in a way
that mattered — the proposed fix (`a.prompt.SetBackgroundColor(bg)`)
compiles, and a naive unit test asserting `a.prompt.GetBackgroundColor()`
passes, but it has **no visible effect at all**. The real mechanism and
fix are below the original text.

<details><summary>Original (incomplete) analysis</summary>

`a.prompt` (`tui/internal/app/app.go:167-169`) is constructed once, at
startup, with:

```go
a.prompt = tview.NewInputField().
    SetLabel(" :").
    SetFieldBackgroundColor(tcell.ColorDefault)
```

`SetFieldBackgroundColor(tcell.ColorDefault)` makes the field's own
editable area transparent, so what's actually visible is the surrounding
`tview.Box`'s background — a color tview captures once, at construction,
from whatever `tview.Styles.PrimitiveBackgroundColor` was at that moment,
and never re-reads afterward. `reapplyTheme` has no `SetBackgroundColor`
call for `a.prompt`, unlike every other shell primitive.

</details>

**What's actually true:** `tview.InputField` wraps a *private* `*TextArea`
(the `textArea` field, unexported — inaccessible outside the `tview`
package) which owns its **own, separate** embedded `*Box`, with its own
`backgroundColor`, baked in at `TextArea` construction from whatever
`tview.Styles.PrimitiveBackgroundColor` was at that moment. Confirmed by
tracing `InputField.Draw()`: it calls `i.Box.DrawForSubclass(screen, i)`
first (this is what `a.prompt.SetBackgroundColor(bg)` actually updates —
the *outer* Box), then calls `i.textArea.Draw(screen)`, whose *first line*
is `t.Box.DrawForSubclass(screen, t)` — repainting the exact same screen
cells from the **inner**, never-updated Box's background, overwriting
whatever the outer Box just painted. `InputField.SetBackgroundColor`
therefore has no visible effect once the field has been drawn even once.

Verified with a throwaway test rendering `a.prompt` to a
`tcell.SimulationScreen` and reading `GetContents()` back (mirroring
`TestPromptAutocompleteFirstOpenIsReadable`'s technique): after
`reapplyTheme` to `cyberpunk`, `a.prompt.SetBackgroundColor` alone left
the label's rendered cell at `dark`'s frozen background/foreground; a
second inner-`Box`-reaching call fixed it (see "Fix" below).

The only exported `InputField` method that reaches the private
`TextArea`'s actual background is `SetFormAttributes(labelWidth,
labelColor, bgColor, fieldTextColor, fieldBgColor)` — normally intended
for `tview.Form` usage, but it forwards straight to
`TextArea.SetFormAttributes`, which is the method that sets the
background `TextArea.Draw()` actually paints from. It also happens to fix
two *other* latent instances of the exact same bug that weren't part of
the original report: the label's own text color and the typed-command
text color are **also** frozen at startup's theme forever (same "baked in
at private-TextArea construction, never refreshed" root cause) — visible
once you know to look, but not something a user would necessarily
describe separately from "the prompt didn't change color."

This remains a narrow, `a.prompt`-specific gap, not a systemic one: every
other `tview.InputField` in the app is a plain field with no label (each
view's filter input, dialog form fields), so this specific private-Box
staleness never shows up for them the way it does for a labeled field.

## Fix

Replace the (ineffective) `a.prompt.SetBackgroundColor(bg)` with:

```go
a.prompt.SetFormAttributes(0, tcell.GetColor(p.Value), bg, tcell.GetColor(p.Text), tcell.ColorDefault)
```

- `labelWidth = 0` — preserves the existing auto-width layout (unset,
  same as before).
- `labelColor = p.Value` — matches what the label's foreground already
  defaulted to at construction (`tview.Styles.SecondaryTextColor` ==
  `p.Value`), now kept in sync on every switch instead of frozen once.
- `bgColor = bg` — the actual fix: this is the field the private
  `TextArea`'s `Box.Draw` paints from.
- `fieldTextColor = p.Text` — matches the construction-time default
  (`Styles.PrimaryTextColor` == `p.Text`), now kept in sync.
- `fieldBgColor = tcell.ColorDefault` — unchanged from `New()`'s existing,
  deliberate choice (transparent typed-text background, no colored block
  around the cursor).

## Scope

- In scope: `a.prompt`'s own background, label color, and typed-text
  color on a live theme switch (all three share the same root cause and
  the same fix call).
- Out of scope: any other widget's theming (systematically checked, none
  found broken); the autocomplete drop-down's styling (already correct,
  per the bugfix-autocomplete-suggestions PR); a general "reload
  everything" mechanism — a single corrected call in `reapplyTheme`,
  consistent with how every other primitive there is already handled.

## Manual verification

`a.prompt.GetBackgroundColor()` (the outer, exported `*Box`) is **not**
sufficient to verify this — it's the field the bug proved doesn't
matter. The private `TextArea`'s actual background isn't reachable from
outside `tview` at all, so this can only be verified by rendering
(`tcell.SimulationScreen` + `Draw` + `GetContents`, same technique as
`TestPromptAutocompleteFirstOpenIsReadable`) and reading the label
cell's style back. Manual verification (build + tmux) additionally
confirms the visible color across at least two themes and across more
than one switch in the same running process (the bug was intermittent
in exactly that way during diagnosis).
