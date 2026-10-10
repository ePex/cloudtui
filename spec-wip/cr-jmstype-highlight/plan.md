# Plan: highlight only the JMS Type suggestion that Enter will take

Implements `spec.md` in this folder.

## How tview draws the highlight (v0.42)

- `InputField.Autocomplete()` builds the drop-down's internal `List`
  **once**, copying `autocompleteStyles` (background, main, selected)
  into it, then keeps reusing that list (only its entries are
  refilled) until it's dropped: Esc, an accepted entry, or a call that
  yields no entries. Later `SetAutocompleteStyles` calls don't reach a
  list that already exists.
- The list always has a current item (a new list starts on the first
  entry, or the first one starting with the typed text), drawn in the
  `selected` style. There's no public way to say "no current item".
- Rebuilding keeps the previous current item when it's still listed.
  That's the "sticky" highlight.
- The field's input handler holds the list's mutex while it handles a
  key and while it calls the changed func, so `Autocomplete()` can't be
  called from the changed func (deadlock). Our input capture runs
  before that, without the lock.

## Approach: two style states, switched by rebuilding the list

`jmsTypeAutocomplete` (`internal/dialog/jmstypeautocomplete.go`)
already tracks `chosen`. It gets a second job: the drop-down's look
follows it.

- **Not chosen:** the field's autocomplete `selected` style is set to
  the same style as unselected rows, so tview's current item is
  invisible.
- **Chosen:** `selected` is the theme's selection style, as today.
- **Switching** = set the styles, then drop and rebuild the list in the
  input capture (the existing "suppress" trick: `Autocomplete()` with no
  entries drops it, a second call builds a fresh one with the new
  styles).

Transitions, all in `capture` unless noted:

- **First ↑/↓ while not chosen** (with a drop-down open): switch to the
  chosen look, mark `chosen`, record which entry is current (the
  rebuilt list's: the first entry starting with the typed text, else the
  first entry, the same rule tview uses), and swallow the key. So the
  first arrow press shows the highlight on that entry instead of
  silently moving an invisible one. Further ↑/↓ go to tview as today,
  and its navigation callback updates the recorded entry. The one-entry
  case is no longer special: the first arrow chooses the only entry.
- **Any other key while chosen**, except Enter and Tab: switch back to
  the plain look before the key is handled, so typing, Backspace or ←/→
  never leave a highlight on a list Enter would ignore. If the key
  changes the text, tview's own refill then works on the plain list.
- **Enter/Tab:** unchanged (accept when chosen, otherwise close and let
  the key through).
- **Changed func** (any text change, including `SetText` after
  accepting): `chosen = false` as today, and set the plain styles, but no
  rebuild there (lock). The next list is built plain.
- **`reset()`** (dialog reopens): plain styles, and drop any cached
  list.

## Theme switches

Both fields are restyled today with `ui.StyleInputFieldAutocomplete`,
which sets `selected` to the selection colors. They must keep the state's
look instead:

- `ui.AutocompleteStyles(p)` (new) returns the background, row and
  selection styles that `StyleInputFieldAutocomplete` applies, which
  then uses it. One source for the colors.
- `jmsTypeAutocomplete.applyPalette(p)` stores the palette and applies
  the styles for the current state (plain or chosen), with the same
  blur-to-drop-the-cache step `StyleInputFieldAutocomplete` does for an
  unfocused field. `JMSTypePrompt` and `MessageFilter` call it from their
  construction and `ApplyPalette` instead of styling the field directly.
  The "style first, then wire" rule (tui/CLAUDE.md) still holds: the
  constructor applies the plain styles before `SetAutocompleteFunc`.

## Files

- `tui/internal/ui/style.go` (+ test): `AutocompleteStyles`.
- `tui/internal/dialog/jmstypeautocomplete.go` (+ test): the two states,
  the transitions, `applyPalette`.
- `tui/internal/dialog/jmstypeprompt.go`, `messagefilter.go`: style
  through `applyPalette`.
- spec/08, spec/09 (merge-back): the highlight rule.

## Testing

- **Rendering, on a `SimulationScreen`** (the field focused and drawn
  with its drop-down open), reading each drop-down row's background:
  - opened with several suggestions → no row in the selection
    background;
  - after ↓ → exactly one row, the first entry (or the first one
    starting with the typed text);
  - another ↓ → the highlight moved to the next row;
  - typing a character → no highlighted row again;
  - the one-entry list (scan entry only) → none, then ↓ → that row;
  - a live theme switch while chosen keeps the highlight in the new
    theme's selection color; while not chosen, keeps none;
  - `reset()` after a choice → none.
- **Behavior unchanged:** the existing tests for Enter/Tab/click/scan
  keep passing; plus Enter after the first ↓ accepts the highlighted
  entry.
- Mutation-check: with the selected style left at the selection colors,
  the "no highlighted row" tests fail.
- **Live (tmux, isolated HOME, local broker):** a scratch queue with two
  JMS types. Purge prompt: no highlighted row on open; ↓ highlights
  one; Enter puts it in the field; Enter again purges only that type
  (the other type's messages remain). Typing after ↓ removes the
  highlight. The message filter's JMS Type field the same way. Check in
  the dark and one other theme.
