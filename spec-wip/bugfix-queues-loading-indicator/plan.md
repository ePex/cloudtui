# Implementation plan

## Approach

`tui/internal/view/queues.go`:

- **Closer precedent found while planning, refining the spec's "status
  bar" framing**: `CodePipelineDetailView.showStatus(msg string)`
  (`tui/internal/view/codepipelinedetail.go`) already solves exactly
  this — an in-progress, non-error message shown as a single table row,
  same shape as its `showError` but accent-colored so it doesn't read as
  a failure (used today while an AWS SSO re-auth runs). That's a closer
  match than the bottom status bar (`host.SetStatus`), which is used
  elsewhere for dialog/modal-driven async actions, not table views.
  `QueuesView` gets its own `showStatus(msg string)` mirroring that
  shape — table-only, no bottom-status-bar message, matching the
  existing table-view convention rather than the dialog convention.
- New field: `loadSeq int` on `QueuesView` — incremented at the start of
  every `Load()` call; the goroutine's completion callback captures its
  own `seq` value at spawn time and compares against `qv.loadSeq` when
  it runs. A mismatch means a newer `Load()` has since started, so the
  stale result is silently discarded (return before touching
  `qv.table`/`qv.host` at all). No mutex needed: `loadSeq` is only ever
  read/written from within `QueueUpdateDraw` callbacks or `Load()`
  itself, both of which run on tview's single UI goroutine — same
  concurrency model the rest of the codebase already relies on (e.g.
  `jmstypeprompt.go`'s `scanning` bool).
- `Load()` becomes:

  ```go
  func (qv *QueuesView) Load() {
      qv.loadSeq++
      seq := qv.loadSeq
      qv.showStatus("Loading queues…")
      go func() {
          summaries, err := qv.backend.List(context.Background())
          qv.host.QueueUpdateDraw(func() {
              if seq != qv.loadSeq {
                  return // superseded by a newer Load()
              }
              if err != nil {
                  slog.Error("queues: failed to list queues", "error", err)
                  qv.showError(err)
                  return
              }
              qv.repaint(summaries)
          })
      }()
  }
  ```

- `showStatus` mirrors `showError`'s existing shape exactly (clear rows
  below the header, set a single accent-colored row at (1,0)):

  ```go
  func (qv *QueuesView) showStatus(msg string) {
      for qv.table.GetRowCount() > 1 {
          qv.table.RemoveRow(qv.table.GetRowCount() - 1)
      }
      qv.table.SetCell(1, 0,
          tview.NewTableCell(msg).
              SetTextColor(tcell.GetColor(qv.host.Config().Colors.Accent)).
              SetExpansion(5),
      )
  }
  ```

  (`SetExpansion(5)` matches `showError`'s existing value for this
  table, not `codepipelinedetail.go`'s `3` — each table's columns
  already use different expansion weights.)

## Testing

- `TestQueuesViewLoadShowsLoadingStatusImmediately` — call `Load()`
  against a `fakeBackend`/similar whose `List` blocks on a channel
  (mirroring `blockForever` from `jmstypeprompt_test.go`), assert the
  table shows the loading row *before* unblocking the channel.
- `TestQueuesViewLoadStaleResponseDiscarded` — the key regression test:
  two overlapping `Load()` calls where the first's backend call is
  gated on a channel; unblock the *second* call's response first,
  assert it renders; then unblock the first (stale) response, assert
  the table is unchanged (still showing the second's data, not
  clobbered by the first's).
- `TestQueuesViewLoadReplacesLoadingRowOnSuccess` /
  `...OnError` — existing coverage for `repaint`/`showError` already
  exists; add the "loading row was showing first" setup to confirm the
  transition, not just the end state.
- Run with `-race` — this is exactly the class of bug (`internal/app`'s
  `TestSetActiveAWSProfileRebuildsSecretBackedBackend`,
  `jmstypeprompt_test.go`'s `blockForever` pattern) that's bitten this
  codebase before when a test drives real async production wiring
  without gating it.

## Manual verification

Per `CLAUDE.md`'s testing section and the `verify-live` skill: unit
tests cover the logic, but the actual visual transition (loading row →
real data, and the stale-discard *not* causing a visible flicker back to
old data) needs eyes on a real terminal. Steps for `tasks.md`: drive the
real TUI in tmux against two connections (a fast local Jolokia one and
either a slow/unreachable proxy one, or an artificially delayed one),
switch between them, and confirm the loading row appears immediately
and the final state always matches the *last* connection selected, even
when switching rapidly back and forth.
