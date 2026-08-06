# Spec — CR 15: Move-picker DLQ priority and search filter

Date: 2026-08-06

## What and why

The move-picker (FE 14) lists target queues in alphabetical order. Two
usability gaps exist in DLQ workflows:

1. **DLQ priority ordering** — when moving a message out of a DLQ
   (`dlq.*` queue), the most likely target is the corresponding
   non-DLQ queue (e.g. `dlq.foo.bar` → `foo.bar`). That queue should
   appear at the top of the list prefixed with `⭐` to distinguish it
   visually; all other queues follow alphabetically.

2. **Search filter** — with many queues the user must scroll to find a
   target. Pressing `/` in the picker should open an inline filter input;
   typing narrows the list to queues whose names contain the typed
   substring (case-insensitive). Pressing `Esc` clears the filter and
   returns focus to the list; pressing `Enter` selects the first visible
   item.

## Scope

**In scope:**
- Four-tier queue ordering (each tier sorted alphabetically within itself):
  1. `⭐` **Preferred** — when source is a `dlq.*` queue, the corresponding
     non-DLQ queue (strip prefix) is pinned first with a star emoji.
  2. **Regular** — all other queues.
  3. `➖` **DLQ** — queues starting with `dlq.`, de-prioritized.
  4. `❓` **System** — queues starting with `activemq.` or `statistics.*`,
     de-prioritized to the bottom.
- Search filter: `/` activates a `tview.InputField` shown at the bottom of
  the picker; list items are filtered live as the user types; `Esc` cancels
  (clears filter, refocuses list); `Enter` refocuses the list.
- Unit tests for the tier sorting logic.
- Unit tests for the search filter behavior.

**Out of scope:**
- DLQ detection based on broker metadata (name prefix heuristic only).
- Other queue naming conventions beyond `dlq.` prefix.
- Fuzzy / ranked search — simple substring match is sufficient.
