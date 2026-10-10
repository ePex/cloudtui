# Tasks: highlight only the JMS Type suggestion that Enter will take

Each task needs its own approval before it starts, and is pushed once
done. Live checks run with an isolated HOME and a separate tmux server
(`TMUX_TMPDIR` set to a short path), never against the real
`~/.cloudtui`.

1. [ ] **`ui.AutocompleteStyles`** (plan "Theme switches").
   `internal/ui/style.go` + `style_test.go`: return the background, row
   and selection styles for a palette; `StyleInputFieldAutocomplete`
   uses it. Test: the returned styles match the palette's colors, and
   `StyleInputFieldAutocomplete` still renders the same as before. No
   behavior change.

2. [ ] **Two looks in `jmsTypeAutocomplete`** (plan "Approach",
   "Theme switches").
   `internal/dialog/jmstypeautocomplete.go`: store the palette in
   `applyPalette(p)` and apply the plain or chosen styles for the
   current state (blurring an unfocused field to drop the cached list);
   the changed func and `reset()` switch to the plain look.
   `JMSTypePrompt` and `MessageFilter` style the field through
   `applyPalette` in their constructors (before `SetAutocompleteFunc`)
   and in `ApplyPalette`. No new key handling yet, so ↑/↓ still moves
   an invisible current item.
   `jmstypeautocomplete_test.go`, on a `SimulationScreen`: opened with
   several suggestions → no row in the selection background; a theme
   switch keeps none; `reset()` → none. Mutation-check: with the
   selected style left at the selection colors, these fail.

3. [ ] **Switch looks on keys** (plan "Approach", transitions).
   `capture`: first ↑/↓ while not chosen (drop-down open) switches to
   the chosen look by rebuilding the list, marks `chosen`, records the
   current entry (first entry starting with the typed text, else the
   first) and swallows the key; any other key while chosen, except
   Enter/Tab, switches back to plain before it's handled. Drop the
   one-entry special case.
   Tests (rendering): ↓ → exactly one highlighted row, the expected
   entry; another ↓ → the next row; typing a character → none; ←/→ and
   Backspace → none; one-entry list (scan entry) → none, then ↓ → that
   row; theme switch while chosen → highlight in the new theme's
   selection color. Behavior: Enter after the first ↓ accepts the
   highlighted entry; existing Enter/Tab/click/scan tests still pass.

4. [ ] **Live check and regression run.** `task test`,
   `task test:tui:race`, smoke test (isolated).
   *Live (tmux, isolated HOME, local broker):* a scratch queue with
   messages of two JMS types. Purge prompt: no highlighted row on
   open; ↓ highlights one; typing removes the highlight; ↓ again, Enter
   puts it in the field; Enter again purges only that type (the other
   type's messages remain). The message filter's JMS Type field: same
   highlight behavior. Repeat the highlight checks in the dark theme
   and one other theme, including a live theme switch with the
   drop-down open. Remove the scratch queue.
