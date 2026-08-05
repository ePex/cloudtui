# Spec — Bugfix 12: Esc in message detail view does not return to message list

Date: 2026-08-06

## Problem

Pressing Esc in the message detail view does not visually return to the message
list. A second Esc is required, which then navigates all the way to the queue
list (skipping the message list).

## Root cause

The Esc handler called `pages.ShowPage("messages")`, which makes the messages
page visible but does not hide the "message-detail" page. Because "message-detail"
was added to the pages stack after "messages", it sits on top in z-order and
remains visible. Only focus was moved to the messages table, so the next Esc
fired on that table and invoked `switchTo("queues")`.

## Fix

Use `pages.SwitchToPage("messages")` instead of `pages.ShowPage("messages")`.
`SwitchToPage` hides all other pages before showing the target, matching the
behaviour used everywhere else in the app.

## Out of scope

- Changes to any other view or navigation flow.
