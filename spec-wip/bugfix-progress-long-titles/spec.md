# Bugfix: progress modal cuts off long queue names

Date: 2026-10-17

## What's broken

The TUI's progress modal (spec/24) is a fixed 60×11 box. Its title
names the queues the operation works on, e.g. `Moving all OrderCreated
messages from "<source>" to "<target>"` or `Deleting message from
"<queue>"`. The title gets one line of 56 characters and is simply cut
off at the edge, without any "…", so with long (often dotted) queue
names the user can't see which queue is being changed, or the target
of a move is missing entirely. Reported by the user.

The same happens in the spinner look (`⠹ <title>…  3 of 10`, one line),
and in the two detail rows below the bar: they wrap at spaces, but a
word longer than a row (a queue name, or the URL in the AWS SSO
message, spec/14) is cut with "…". The SSO message with its code and
URL can also need more than two rows, so the URL to copy can be lost.

## Desired behavior

- **The title wraps** over as many lines as it needs, up to 4, centered,
  instead of being cut. It breaks at spaces; a word longer than a line
  (a queue name) is broken inside the word rather than cut. The modal
  keeps its 60-column width (the rainbow bar and cat need it) and grows
  taller by the extra lines; the bar, count, detail and footer move
  down with it.
- **Spinner look:** the spinner, title, "…" and count wrap the same
  way; the spinner stays at the start of the first line.
- **Detail rows** break overlong words the same way, and get up to 3
  rows instead of 2, so the SSO message keeps its code and full URL.
- Only text that still doesn't fit (more than 4 title lines, more than
  3 detail rows) ends in "…".
- On a terminal too short for the grown modal, it's clipped to the
  screen as today.
- Unchanged: the titles themselves, the 60-column width, the look of
  the bar, cat, spinner and count, the timing, blocking, and keys
  (spec/24); the web console's progress overlay.

## Out of scope

- Making the modal wider (on a wide terminal).
- Shortening queue names.
- Wrapping elsewhere (the status line, table cells, confirm dialogs).
