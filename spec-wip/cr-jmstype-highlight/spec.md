# Change request: highlight only the JMS Type suggestion that Enter will take

Date: 2026-10-10

## What and why

The JMS Type field's suggestion drop-down (the purge/move-all prompt,
spec/09, and the message filter's JMS Type field, spec/08) always shows
one entry in the selection colors, even when the user never picked it.
tview keeps the last highlight when it rebuilds the list, so this is
often the "↻ Scan up to 5,000 messages for JMS types" entry, or a type
that only matched an earlier prefix.

But Enter and Tab only take a suggestion the user moved to with ↑/↓
since they last typed (spec/09, "JMS Type autocomplete"). Otherwise the
typed text counts, and the drop-down just closes. So the screen shows
one thing as selected while Enter does another. Reported for the purge
prompt: it isn't clear what is selected and what isn't, or whether a
purge will be filtered.

## Desired behavior

- **Highlight means "Enter takes this".** A suggestion shows in the
  selection colors only once the user has moved to it with ↑/↓ since
  their last keystroke in the field (the existing "chosen" rule).
- **No choice, no highlight.** While nothing is chosen, every entry
  looks the same (the drop-down's normal row colors), so it's clear the
  typed text is what counts. This includes the first open, right after
  typing, and after a rebuild.
- **Typing drops the choice and its highlight**, as it already drops
  the choice itself.
- **A one-entry list** (often just the scan entry) gets highlighted by
  ↑/↓ too, since that already counts as choosing it.
- Unchanged: what Enter, Tab, Esc and a mouse click do; the suggestion
  contents; the scans; the colors themselves (still the theme's
  selection colors, spec/04).

Applies to both fields, since they share the same autocomplete.

## Out of scope

- A hint line saying what Enter will do (e.g. "Enter: purge all
  messages"). It could follow if the highlight alone isn't enough.
- The web console's JMS Type field (a native browser `<datalist>`, whose
  highlighting is the browser's own).
- Other autocompletes (the `:` prompt's).
