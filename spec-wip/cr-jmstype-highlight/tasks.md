# Tasks: highlight only the JMS Type suggestion that Enter will take

Each task needs its own approval before it starts, and is pushed once
done. Live checks run with an isolated HOME and a separate tmux server
(`TMUX_TMPDIR` set to a short path), never against the real
`~/.cloudtui`.

1. [x] **`ui.AutocompleteStyles`** (plan "Theme switches").
   `internal/ui/style.go` + `style_test.go`: return the background, row
   and selection styles for a palette; `StyleInputFieldAutocomplete`
   uses it. Test: the returned styles match the palette's colors, and
   `StyleInputFieldAutocomplete` still renders the same as before. No
   behavior change.
   *Done (2026-10-10):* `AutocompleteStyles(p)` returns
   `(background, main, selected)` in `SetAutocompleteStyles` order.
   `TestAutocompleteStyles` checks the three values against the palette;
   `TestStyleInputFieldAutocompleteDrawsAutocompleteStyles` draws an
   opened drop-down and checks the current entry in the selection
   colors and the other in the row colors. Mutation-checked: with the
   field given the row style as its selected style, the rendering test
   fails (`alpha` drawn on the panel background). `task test:tui`,
   gofmt and vet pass. Merge-back: add `AutocompleteStyles` to
   spec/03's `style.go` helper list.

2. [x] **Two looks in `jmsTypeAutocomplete`** (plan "Approach",
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
   *Done (2026-10-10):* `newJMSTypeAutocomplete` now takes the palette
   and styles the field itself before `SetAutocompleteFunc`, so the
   callers no longer call `ui.StyleInputFieldAutocomplete`; their
   `ApplyPalette` calls `ac.applyPalette`. The changed func became
   `textChanged` (drop the choice, plain styles, no rebuild), and the
   Enter/Tab close moved into `closeDropDown`, which `reset()` reuses.
   Tests: `TestJMSTypeAutocompleteNothingChosenHighlightsNothing`,
   `…ThemeSwitchKeepsNothingHighlighted` (an unfocused field's cached
   drop-down redrawn only in cyberpunk's row background) and
   `…ResetDropsTheHighlight` (a chosen look built by a theme switch,
   then `reset()`). Mutation-checked, each breaking change failing at
   least one test: selected style always the selection colors (all
   three fail); `reset()` without restyling, or without dropping the
   list; `applyPalette` without the blur. The last one first went
   uncaught because the test blurred the field itself; the test now
   checks the new theme's row colors on a never-focused field.
   `task test:tui`, gofmt and vet pass. Merge-back: spec/08 line about
   the filter's drop-down styling (`ui.StyleInputFieldAutocomplete`),
   and tui/CLAUDE.md's "style first, then wire" / sticky-highlight
   gotchas, which should mention `jmsTypeAutocomplete` styling itself.

3. [x] **Switch looks on keys** (plan "Approach", transitions).
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
   *Done (2026-10-10):* `setChosen` switches the look (styles, then
   drop and rebuild the drop-down); `firstMatch` mirrors tview's
   starting-entry rule. Three things beyond the plan, found reading
   tview's key handling:
   - **PgUp/PgDn** move tview's current entry like ↑/↓, so they're
     handled the same way. Otherwise the first PgDn would choose an
     invisible entry.
   - **↑ on a closed drop-down** now opens it with the highlight, like
     ↓ already did (capture can't tell whether a drop-down is open).
   - **Accepting an entry uses up the choice.** The scan entry leaves
     the text alone, so the changed func never reset `chosen`. With the
     new key handling, an Esc right after a scan would have reopened and
     closed a drop-down instead of closing the prompt.
   Tests: `…FirstArrowHighlightsTheCurrentEntry` (↓/↑/PgDn on empty
   text, ↓ on `TypeC` and on an unmatched prefix, then Enter takes that
   entry), `…LaterArrowsMoveTheHighlight`, `…OtherKeysDropTheHighlight`
   (typing, Backspace, ←, →, then Enter keeps the text),
   `…EscAfterAChoiceOnlyClosesTheDropDown`,
   `…ScanEntryUsesUpTheChoice`,
   `…SingleEntryHighlightedOnlyAfterAnArrow` and
   `…ThemeSwitchKeepsTheChoiceHighlighted`. Mutation-checked, each
   failing at least one test: first arrow not swallowed; first arrow
   without the rebuild; other keys keeping the highlight; accepting
   keeping the choice; `firstMatch` ignoring the text; PgUp/PgDn not
   handled. `task test:tui`, `task test:tui:race`, gofmt and vet pass.
   Merge-back: spec/08 (around line 188) and spec/09 (around lines 155
   and 410–432, including the one-entry special case, now gone) for the
   highlight rule and the PgUp/PgDn and ↑ details.

4. [x] **Live check and regression run.** `task test`,
   `task test:tui:race`, smoke test (isolated).
   *Live (tmux, isolated HOME, local broker):* a scratch queue with
   messages of two JMS types. Purge prompt: no highlighted row on
   open; ↓ highlights one; typing removes the highlight; ↓ again, Enter
   puts it in the field; Enter again purges only that type (the other
   type's messages remain). The message filter's JMS Type field: same
   highlight behavior. Repeat the highlight checks in the dark theme
   and one other theme, including a live theme switch with the
   drop-down open. Remove the scratch queue.
   *Done (2026-10-10):* `task test` (Go, mq-proxy, 78/78 web tests),
   `task test:tui:race` and the smoke test pass. The smoke test ran
   with the scratch HOME and its own tmux server, through a `tmux -L`
   wrapper on `PATH`; the scratch HOME was unchanged afterwards. Live:
   a fresh build in its own tmux server (truecolor), scratch HOME, local
   broker, scratch queue `jmshl-scratch` with 3 `OrderCreated` and 2
   `OrderCancelled` messages. Highlighted rows were read from
   `capture-pane -e` by the selection background.
   - **Purge prompt (dark):** no highlighted suggestion on open (only
     the input field itself, which uses the same color); ↓ highlighted
     `OrderCancelled`, ↓ moved it to `OrderCreated`; typing `O` removed
     it. Backspace, ↓ ↓, Enter put `OrderCreated` in the field; Enter
     again showed "All OrderCreated messages will be deleted"; Yes
     purged 3, and the broker still held the 2 `OrderCancelled`
     messages. Reopening after a pick: no highlight. ↑ ↑ highlighted
     the scan entry (the list wraps); Enter scanned, and the refreshed
     drop-down had no highlight.
   - **Message filter (dark):** no highlight on open; ↓ highlighted,
     ↓ moved it, ← removed it; ↓ then Enter put `OrderCancelled` in
     the field.
   - **Cyberpunk** (`:theme cyberpunk`): purge prompt with no highlight
     on open, ↓ highlighted in cyberpunk's selection color, typing
     removed it, no dark selection color anywhere on screen.
   - **Not done live:** a theme switch with the drop-down open. While
     the field is focused every key goes to it, so the switch can only
     happen with the prompt closed; the cached drop-down case is
     covered by `…ThemeSwitchKeepsTheChoiceHighlighted`.
   Scratch queue removed; no mq-proxy left running.
