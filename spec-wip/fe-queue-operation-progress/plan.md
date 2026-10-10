# Implementation plan: queue operation progress overlay

## Approach

Add a small reusable progress overlay in `internal/dialog` and expose show/update/hide operations through the UI host contract. Render it as a centered, blocking overlay so keyboard input cannot start conflicting work. Use tview/tcell primitives and existing theme colors; add no dependency. The overlay will show an operation label, an animated Nyan Cat motif, and a determinate bar when completed and total work are known. For a network call with no observable substeps, animate the Nyan Cat motif as an indeterminate indicator (with a spinner fallback if the terminal cannot render the chosen glyphs).

Queue listing and native queue purge are single backend calls with no progress callback, so they use indeterminate animation. Bulk message delete and move already iterate over individual messages; update the overlay after each attempt, counting failures as completed work so the bar reaches completion. Filtered purge and move-all are also opaque backend calls and use indeterminate animation. Keep current status messages, error reporting, and reload behavior after hiding the overlay.

## Files and modules

- `tui/internal/dialog/`: add the reusable operation-progress overlay and focused rendering/state tests.
- `tui/internal/ui/host.go` and `tui/internal/app/host.go`: add and implement the host methods needed to show, update, and close the overlay.
- `tui/internal/app/`: create and register the overlay with the root pages, following the existing overlay wiring and theme application patterns.
- `tui/internal/view/queues.go`, `messages.go`, and `purge.go`: bracket queue loads and the specified bulk actions with overlay lifecycle calls; report per-message completion for bulk delete/move.
- `tui/internal/view/*_test.go`, app tests, and dialog tests: cover progress rendering, lifecycle on success/error, progress updates, and unchanged operation outcomes.
- `spec-wip/fe-queue-operation-progress/tasks.md`: record the approved implementation breakdown in the next gate.

## Technical decisions

- Reuse the existing tview overlay/page stack rather than adding a new rendering dependency or terminal control sequence.
- Keep backend APIs unchanged. Their current opaque calls cannot provide internal progress, while existing per-message loops can report honest completed/total progress.
- Hide the overlay on every completion path before reporting the result or starting a follow-up reload. Preserve queue-load sequence handling so superseded responses cannot close a newer operation's overlay incorrectly; implementation should use an operation token or equivalent ownership check if overlapping loads are possible.
- Keep the progress overlay non-cancelable because the backend calls currently have no cancellation contract. It blocks conflicting UI input while active.
- Apply palette changes to the overlay using the existing dialog theme lifecycle so theme switching remains consistent.

## Verification

Add unit tests for determinate and indeterminate rendering and overlay lifecycle. Run the focused Go package tests and the existing queue/message smoke path. Manually verify queue loading, bulk delete/move, move-all, and purge against a broker, including a proxy connection, confirming the overlay is visible, progress updates for bulk loops, and existing completion/error feedback remains intact.
