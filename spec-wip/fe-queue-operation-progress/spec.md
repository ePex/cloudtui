# Queue operation progress overlay

Date: 2026-10-10

## Purpose

Queue loads and bulk message operations can take noticeable time, especially through the proxy connection. During these requests, the TUI should show that work is underway instead of appearing unresponsive.

## Behavior

- Show a blocking progress overlay while loading queues and while bulk delete, move, move-all, or purge requests are running.
- When an operation exposes a meaningful completed/total count, show a Nyan Cat-style progress bar with that progress. When it does not, show an animated Nyan Cat-style indeterminate indicator; use a spinner if a suitable animation cannot be rendered in the terminal.
- Label the overlay with the operation and relevant queue where available.
- Remove the overlay when the request finishes, then preserve the existing success, partial-failure, error, and refreshed-view behavior.
- Prevent additional input from starting conflicting queue operations while the overlay is active.
- Provide a broker-free `:progress-demo` command that animates a simulated queue operation so the visual can be previewed safely.

## Scope

This covers queue list loading, bulk message deletion and movement, move-all, and queue purge. Single-message actions and unrelated AWS resource loading are out of scope unless they share the same operation plumbing and can be included without changing their behavior.
