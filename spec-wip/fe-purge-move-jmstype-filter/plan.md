# Plan

## Files touched

- `tui/internal/ui/host.go` — generalize `ScanJMSTypes` to take an
  explicit `queueName` (see "Key decisions"); add `MessagesQueueName()
  string`.
- `tui/internal/app/host.go` — update `ScanJMSTypes`'s implementation
  (drop its implicit `a.messagesV.QueueName()` lookup, take the caller's
  `queueName` instead); add `MessagesQueueName()` (`return
  a.messagesV.QueueName()`).
- `tui/internal/dialog/messagefilter.go` — update `startScan()`'s call
  site: `mf.host.ScanJMSTypes(ctx, mf.host.MessagesQueueName(),
  jmsTypeScanCount)`.
- `tui/internal/dialog/messagefilter_test.go`,
  `tui/internal/app/host_test.go`, `tui/internal/dialog/hosttest_test.go`,
  `tui/internal/view/testfake_test.go` — update for the new
  `ScanJMSTypes` signature and the new `MessagesQueueName` stub.
- `tui/internal/dialog/jmstypeprompt.go` (new) — `JMSTypePrompt`, the
  shared JMS Type filter prompt.
- `tui/internal/dialog/jmstypeprompt_test.go` (new).
- `tui/internal/app/app.go` — construct `a.jmsTypePrompt =
  dialog.NewJMSTypePrompt(a)` alongside the other early-constructed
  dialogs (`a.confirm`, `a.movePicker`, ...), pass it into
  `NewQueuesView`, register its page.
- `tui/internal/view/queues.go` — new `jmsTypePrompt
  *dialog.JMSTypePrompt` constructor parameter; rewrite the `p`/`M`
  handlers to show it first.
- `tui/internal/view/queues_test.go` — update constructor calls; new
  tests for the filtered/unfiltered routing.
- Merge-back: `spec/09-queue-message-actions/spec.md` (primary
  documentation); a short cross-reference note added to spec/08 where it
  already mentions `Host.ScanJMSTypes`, since that method's signature is
  changing.

No new dependencies.

## `Host.ScanJMSTypes` generalization

```go
// Before (from the previous PR, MessageFilter-only):
ScanJMSTypes(ctx context.Context, maxCount int) ([]string, error)

// After:
ScanJMSTypes(ctx context.Context, queueName string, maxCount int) ([]string, error)
MessagesQueueName() string
```

`App.ScanJMSTypes`'s body barely changes — it already just called
`a.backend.BrowseMessages(ctx, a.messagesV.QueueName(), ...)`; the
queue name source moves from an implicit internal lookup to an explicit
parameter. `MessageFilter.startScan()` is updated to fetch its own
(unchanged) implicit queue via the new `MessagesQueueName()` and pass it
through explicitly.

### Key decisions

- **Generalize the existing method rather than add a second,
  similarly-named one.** The alternative — leaving `ScanJMSTypes(ctx,
  maxCount)` as-is and adding `ScanJMSTypesForQueue(ctx, queueName,
  maxCount)` for the new caller — avoids touching the
  just-shipped/tested `MessageFilter` code path at all, but leaves two
  near-identical methods on `Host` differing only in an implicit vs.
  explicit queue name, which reads as an inconsistency to a future
  reader rather than a deliberate design. `ScanJMSTypes` is brand new
  (this session, previous PR) with exactly one caller today, so the
  "churn" from generalizing it is small and contained — worth doing now
  rather than carrying the asymmetry forward.
- **`MessagesQueueName()` instead of having `MessageFilter` reach for the
  queue name some other way.** Mirrors the existing
  `MessagesFilter()`/`ApplyMessagesFilter()`/`FocusMessages()` cluster of
  `Host` methods already scoped to "the Messages view," rather than
  inventing a different access pattern for this one new need.

## `JMSTypePrompt` (`internal/dialog/jmstypeprompt.go`)

```go
type JMSTypePrompt struct {
	host      ui.Host
	flex      *tview.Flex
	field     *tview.InputField
	visible   bool
	queueName string
	scanned   []string
	scanning  bool
	onContinue func(jmsType string)
	onClose    func()
}

// Show opens the prompt for queueName. onContinue is called (close()
// already run) with the entered JMS Type — empty string means "no
// filter, proceed as before". onClose is called on every dismissal
// (Enter or Esc), before onContinue if applicable — the caller uses it
// to restore focus/context, same contract as MovePicker.Show.
func (jp *JMSTypePrompt) Show(queueName string, onContinue func(jmsType string), onClose func())
```

Internals mirror `MessageFilter`'s JMS Type field almost exactly (same
`ui.StyleInputFieldAutocomplete` styling, same sentinel/`SetChangedFunc`/
`handleScanResult` shape to avoid the reentrant-`SetText` bug this
session already found and fixed there) — the only real differences:

- No tier 1: `jmsTypeSuggestions` only ever returns `[sentinel]` until a
  scan completes, then `scanned` prefix-filtered plus the sentinel — no
  `Host.LoadedJMSTypes()` call at all, since there's nothing loaded to
  ask about.
- A single field, not a 4-field `tview.Form` — `Enter` on the field
  itself continues (no separate "Continue" button needed for one field);
  `Esc` cancels.
- `queueName` is a `Show()` parameter (the prompt is reused across
  whichever queue `p`/`M` was pressed on), not implicit — so
  `Host.ScanJMSTypes(ctx, jp.queueName, jmsTypeScanCount)` uses it
  directly, no `MessagesQueueName()` involved here (that getter is
  `MessageFilter`-specific, per its own doc comment).

### Key decisions

- **A new, separate dialog type, not a generalized/parameterized
  `MessageFilter`.** `MessageFilter` is tightly coupled to
  `MessagesView`/`ApplyMessagesFilter` (4 fields, Apply/Clear/Cancel,
  `ui.ParseMessageFilterForm`) — bending it to also serve a single-field,
  queue-scoped, continue-only prompt would either bloat it with
  conditional fields or fork its behavior internally. A small, separate
  type sharing only the *technique* (styling, sentinel, scan handling)
  is more in keeping with this codebase's existing one-purpose-per-dialog
  pattern (`ConfirmDialog`, `MovePicker`, `SendMessageOverlay` are all
  similarly narrow).
- **Shared by both purge and move-all, not two separate prompts.** Same
  field, same behavior, same styling — the only difference between the
  two call sites is what happens in `onContinue`.

## Queues view routing (`internal/view/queues.go`)

The `p` and `M` handlers each gain a step: instead of going straight to
`qv.confirm.Show(...)` / `qv.movePicker.Show(...)`, they first call
`qv.jmsTypePrompt.Show(action, name, onContinue, qv.restoreShortcuts)`.
**Revised during implementation** from the originally-planned single
inline closure: `onContinue` calls a small named method
(`confirmPurge`/`pickMoveAllTarget`), which in turn calls a pure
backend-dispatch method (`doPurge`/`doMoveAll`) — extracted specifically
so the routing decision (`PurgeQueue` vs. `DeleteMessages`,
`MoveAllMessages` vs. `MoveMessages`) is unit-testable on its own,
without needing to drive the confirm dialog / move-picker's async
selection flow (which this codebase has no existing precedent for unit
testing at all — `ConfirmDialog`/`SendMessageOverlay` have no dedicated
test files either; that layer is `verify-live`-tested instead, per
`tui/CLAUDE.md`).

```go
func (qv *QueuesView) confirmPurge(name, jmsType string) {
	question := fmt.Sprintf("Purge %q? All messages will be deleted.", name)
	if jmsType != "" {
		question = fmt.Sprintf("Purge %q? All %s messages will be deleted.", name, jmsType)
	}
	qv.confirm.Show(question, func() {
		go func() {
			err := qv.doPurge(context.Background(), name, jmsType)
			qv.host.QueueUpdateDraw(func() { /* unchanged error/reload handling */ })
		}()
	})
}

func (qv *QueuesView) doPurge(ctx context.Context, name, jmsType string) error {
	if jmsType == "" {
		return qv.backend.PurgeQueue(ctx, name)
	}
	_, err := qv.backend.DeleteMessages(ctx, name, queue.MessageFilter{JMSType: jmsType})
	return err
}
```

`pickMoveAllTarget`/`doMoveAll` follow the identical shape for move-all
(`movePicker.Show` unchanged; the branch is in `doMoveAll`). Both
`confirmPurge`/`pickMoveAllTarget` are passed `qv.restoreShortcuts` as
the *inner* dialog's `onClose` (`confirm.Show`'s onConfirm path doesn't
need one — it doesn't take an onClose parameter at all; `movePicker.Show`
already did before this change), same as before.
`qv.restoreShortcuts` (a small extracted method — see below) is what
`JMSTypePrompt.Show`'s own `onClose` uses too.

### Key decisions

- **Branch inside a small named dispatch method (`doPurge`/`doMoveAll`),
  not inline in the `onContinue` closure as originally planned.** The
  two backend calls per action differ only in that one line, so a single
  `if jmsType == ""` branch was always going to read more clearly than
  two near-duplicate closures — but pulling that branch out to its own
  named method (rather than leaving it inline inside `onContinue`) is
  what actually makes it unit-testable in isolation. This is the one
  place this plan's original design changed during implementation.
- **`restoreShortcuts` extracted as its own `QueuesView` method.** It
  already existed as an inline closure duplicated between `'M'`'s
  move-picker `onClose` and `'c'`'s send-message `onClose` before this
  change; adding two more call sites (`'p'`'s confirm/prompt, `'M'`'s
  prompt) made the duplication clearly worth naming once rather than
  copy-pasting a third and fourth time.

## Testing

- `internal/app`: update existing `ScanJMSTypes` tests for the new
  `queueName` parameter; new test for `MessagesQueueName`.
- `internal/dialog/jmstypeprompt_test.go` (new): mirrors
  `messagefilter_test.go`'s structure closely (suggestions include only
  the sentinel until a scan completes; sentinel triggers a scan; a
  second trigger while scanning no-ops; `handleScanResult` clears the
  field and merges scanned types; `Show` resets `scanned`/`queueName`
  per-open) — reusing the same technique (`SimulationScreen` regression
  test for the eager-autocomplete-cache gotcha, `handleScanResult`
  tested directly rather than through the real goroutine) proven in the
  Messages view's filter dialog.
- `internal/view/queues_test.go`: `doPurge`/`doMoveAll` called directly
  with an empty `jmsType` assert `PurgeQueue`/`MoveAllMessages` are
  called and `DeleteMessages`/`MoveMessages` are not (and vice versa for
  a non-empty `jmsType`, asserting the filter's `JMSType` field and that
  the *other* pair of methods stays uncalled) — via `fakeQueueBackend`,
  extended with injectable per-method functions (mirroring how
  `fakeViewHost`/`testHost` already inject functions per-test for
  data-fetcher methods). Two further tests confirm pressing `p`/`M`
  actually shows `jmsTypePrompt` before either `confirm`/`movePicker`
  becomes visible, closing the gap between "the dispatch logic is
  correct" and "it's actually reachable from the keybinding" without
  needing to drive the prompt's own field interaction (already covered
  by `jmstypeprompt_test.go`).
- Manual verification via the `verify-live` skill against a real broker
  (both backends): as described in `spec.md`'s "Manual verification" —
  unfiltered purge/move-all behave exactly as before; filtered
  purge/move-all only affect matching messages.

## Trade-offs / risks accepted

- Everything already listed in `spec.md`'s "Considered, not
  implemented" and "Out of scope" sections.
- `ScanJMSTypes`'s signature change is a small, contained breaking
  change to a brand-new (this-session) method with one existing caller,
  not a public/stable API — acceptable churn, not a compatibility
  concern.
