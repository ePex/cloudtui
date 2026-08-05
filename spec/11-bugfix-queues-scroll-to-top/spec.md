# Spec — Bugfix 11: Queues list does not scroll to top on navigation

Date: 2026-08-06

## Problem

When navigating to the Queues page (e.g. switching from another view), the
table selection cursor remains at whatever row it was previously on, rather
than resetting to the first data row.

## Expected behavior

Every time the queues list is repainted (on load or filter change), the
table selection resets to the first data row (row 1).

## Out of scope

- Persisting the scroll position across navigations (deliberate UX choice
  to always start at top).
- Changes to any other view.
