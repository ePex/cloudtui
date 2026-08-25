# Bugfix: `:` prompt autocomplete is hard to read and suggests redundant aliases

Date: 2026-08-25

This bugfix covers two problems in the `:` command prompt's autocomplete,
reported together since both are about the same suggestion drop-down.

## Problem 1: the drop-down is hard to read

The `:` command prompt's autocomplete drop-down (`tui/internal/app/app.go`
wiring `ui.StyleInputFieldAutocomplete`, see `spec/01-repo-and-tui-shell`)
renders its entries with the theme's normal `Text`-on-`Background` colors —
the same colors used everywhere else on screen. Because the popup's
background is identical to the surrounding screen background, and tview
draws it with no border or other separator, the drop-down has no visible
edge: it reads as loose lines of text floating over whatever else is on
screen (in practice, it can visually overlap the top-right shortcuts panel)
rather than as a distinct suggestion box.

## Root cause (why a real border isn't the fix)

`tview.InputField` (pinned at v0.42.0) owns the autocomplete popup
internally as a private `*tview.List` with no exported accessor. Its
`Draw()` method computes the popup's rect to exactly fit the entries
(`lwidth` = widest entry, `lheight` = entry count) with no padding
reserved for a border, and recomputes that rect on every draw. Calling
`SetBorder(true)` on that list isn't possible through the public API
(there's no accessor), and even if it were, the existing rect math leaves
no room for one — content would be clipped rather than framed. Getting a
real drawn border would require vendoring/forking tview to patch that
geometry, which is a separate, larger decision (new dependency-management
burden) than a color fix. This was raised to the user, who chose the
lighter-touch fix below over forking tview.

### Fix

Give the drop-down's unselected rows (and its own background) a solid
color that's visibly distinct from the plain theme background, so the
popup reads as a colored panel even without a drawn border — instead of
reusing `Palette.Background` outright, blend it toward `Palette.Accent`
by a small, fixed amount. Selected-row colors (`SelectionBg`/
`SelectionText`) are unchanged.

This is implemented as a small color-blend helper in
`tui/internal/ui/style.go`, used only by `StyleInputFieldAutocomplete`.

## Problem 2: suggestions duplicate global hotkeys that need no prompt at all

`promptCommandTable()` (`tui/internal/app/promptcommands.go`) lists, for
several commands, both a single-letter alias and its full name as
interchangeable typed forms — e.g. `{"q", "quit"}`, `{"h", "home"}`,
`{"s", "settings"}`, `{"l", "log"}`. `promptSuggestions` currently offers
*every* name in the table as an autocomplete entry, so typing `:q`, `:h`,
`:s`, or `:l` suggests itself right back.

But `q`/`h`/`s`/`l` are also global single-key hotkeys, wired
independently in `onGlobalKey` (`tui/internal/app/app.go:456-483`) —
pressing `q`/`h`/`s`/`l` directly (no `:` prompt at all) already does the
same thing. Suggesting them inside the prompt adds no value and just
clutters the list; a user who wants the short form doesn't need the
prompt to tell them about it, and a user who opened the prompt is more
likely looking for the discoverable, self-describing form.

`aq`/`ap` (for `connections`/`awsprofiles`) are a different case: they're
two characters, so they can never be a global single-rune hotkey — the
prompt is the *only* way to reach them short of their full name, so they
keep showing.

### Fix

In `promptSuggestions`, when a `promptCommand`'s alias is also one of the
global hotkey runes handled by `onGlobalKey` (`h`, `s`, `l`, `q`), skip
adding that alias to the suggestion list — but leave it in `pc.names` so
typing it in full and pressing Enter still executes (`onPromptDone` is
untouched). The full names (`quit`, `home`, `settings`, `log`) keep
suggesting, as do `aq`/`ap`.

This was double-checked against a specific worry: does keeping `q`/`h`/
`s`/`l` in `pc.names` risk the prompt firing early while typing a longer
command that starts with one of those letters (e.g. typing `settings`
triggering the `s` action partway through)? It doesn't, for two
independent reasons already in place: (1) `a.prompt` is listed in
`a.focusExemptInputs`, so `onGlobalKey`'s `h`/`s`/`l`/`q` switch cases
never fire at all while the prompt has focus — keystrokes go to the input
field, not the hotkey handler; and (2) `onPromptDone` only runs on Enter
and matches the *entire* typed string exactly (`cmd == n`), so `"s"` typed
en route to `"settings"` never equals `"settings"` until it's fully typed.
Confirmed with the user: aliases stay executable when typed in full; only
the suggestion list changes.

Because `promptcommands.go` and `onGlobalKey`'s switch are two separate
pieces of code, the exclusion list is called out with a comment
cross-referencing `onGlobalKey`'s line range, so a future change to the
global hotkeys doesn't silently leave a stale exclusion behind.

## Scope

- In scope: `:` prompt autocomplete popup background (Problem 1);
  `promptSuggestions`' filtering of single-letter global-hotkey aliases
  (Problem 2).
- Out of scope: `tview.DropDown` popups (Settings' theme/AWS-profile
  pickers, etc. — `StyleDropDown`) — not reported as hard to read, and
  its popup already sits on its own dialog surface rather than floating
  over other views. A real drawn border for either popup type. Any
  change to `tview` version or vendoring. Changing which forms
  `onPromptDone` accepts for execution — `:q`, `:h`, `:s`, `:l` keep
  working, they just stop being suggested. Any change to the global
  hotkeys themselves.

## Manual verification

- Since `tview.List`'s internal styles aren't exposed for assertion,
  Problem 1's visual correctness is checked manually (`task run:tui` or
  the built binary in tmux) against at least two themes with different
  accent colors (e.g. `dark` and `cyberpunk`) to confirm the popup reads
  as a distinct panel in both, and that unselected/selected text stays
  readable.
- Problem 2's suggestion filtering is covered by a unit test on
  `promptSuggestions` (typing `q`/`h`/`s`/`l` suggests nothing for that
  exact single character; typing `qu`/`ho`/`se`/`lo` still suggests
  `quit`/`home`/`settings`/`log`; `a` still suggests `aq` and `ap`), plus
  a manual check that `:q`<Enter> etc. still execute.
