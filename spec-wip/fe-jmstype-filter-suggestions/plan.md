# Plan

## Files touched

- `tui/internal/view/messages.go` — new `AllMessages() []queue.Message`
  getter (read-only, mirrors the existing `Filter()`/`QueueName()`
  getters), so `internal/app` can read the loaded set without
  `internal/dialog` needing to import `internal/view` directly (it
  can't — `internal/view` already imports `internal/dialog` for overlay
  types like `*dialog.MessageFilter`, so the reverse import would cycle).
- `tui/internal/ui/host.go` — two new `Host` interface methods:
  `LoadedJMSTypes() []string` (synchronous, tier 1) and
  `ScanJMSTypes(ctx context.Context, maxCount int) ([]string, error)`
  (tier 2, the opt-in scan).
- `tui/internal/app/host.go` — implementations of both, plus an
  unexported `distinctJMSTypes(msgs []queue.Message) []string` helper
  shared between them.
- `tui/internal/app/host_test.go`, `tui/internal/dialog/hosttest_test.go`,
  `tui/internal/view/testfake_test.go` — stub/fake implementations of the
  two new `Host` methods (same three fakes touched for
  `ToggleFavorite` earlier in this session).
- `tui/internal/dialog/messagefilter.go` — the actual feature: wire
  `SetAutocompleteFunc`/`ui.StyleInputFieldAutocomplete` on the "JMS
  Type" field, the sentinel-entry scan trigger via `SetChangedFunc`, and
  dialog-local state for the scanned set.
- `tui/internal/dialog/messagefilter_test.go` — new file; this dialog has
  no existing tests.
- Merge-back: `spec/08-message-browser-and-detail/spec.md` (the current
  home for the message filter overlay's documentation — FE 46's own
  `spec/46-*` folder was since condensed into it).

No new dependencies.

## Host additions

```go
// ui.Host
LoadedJMSTypes() []string
ScanJMSTypes(ctx context.Context, maxCount int) ([]string, error)
```

```go
// internal/app/host.go
func (a *App) LoadedJMSTypes() []string {
	return distinctJMSTypes(a.messagesV.AllMessages())
}

func (a *App) ScanJMSTypes(ctx context.Context, maxCount int) ([]string, error) {
	msgs, err := a.backend.BrowseMessages(ctx, a.messagesV.QueueName(), queue.MessageFilter{MaxCount: maxCount})
	if err != nil {
		return nil, err
	}
	return distinctJMSTypes(msgs), nil
}

// distinctJMSTypes returns the non-empty, deduplicated JMSType values in
// msgs, sorted for stable, predictable suggestion ordering.
func distinctJMSTypes(msgs []queue.Message) []string { ... }
```

### Key decisions

- **Two `Host` methods, not one parameterized by "tier".** Unlike
  `ToggleFavorite`'s three near-identical kinds (genuinely the same
  shape, differing only in which map), these two differ in a way that
  matters to the caller: one is synchronous/free, the other is
  async/network-costly and takes a context + maxCount. Collapsing them
  behind one signature would need a bool/enum parameter changing the
  method's actual behavior (sync vs. async), which is a worse fit than
  the `ToggleFavorite` case.
- **`ScanJMSTypes` returns only the scan's own distinct types, not merged
  with tier 1.** The dialog merges tier 1 + scanned-so-far itself (see
  below) — keeps `Host`'s contract simple ("here's what this specific
  operation found") and the merge/cache lifetime a dialog concern, not an
  `App`-level one.
- **`distinctJMSTypes` lives in `internal/app`, unexported.** Both call
  sites are in this file; no other package needs it. If that changes
  later, promoting it to `internal/queue` (where `Message` itself is
  defined) is the natural next step — not needed yet.

## Dialog wiring (`internal/dialog/messagefilter.go`)

New state on `MessageFilter`:

```go
type MessageFilter struct {
	host      ui.Host
	form      *tview.Form
	visible   bool
	scanned   []string // extra JMS types found by the last scan this dialog session; nil until a scan completes
	scanning  bool     // true while a scan is in flight, to ignore a duplicate trigger
}
```

- `Show()` additionally resets `scanned = nil` and `scanning = false`
  (each time the dialog opens fresh, same as the existing form-field
  reset already there), wires the JMS Type field's autocomplete
  (`ui.StyleInputFieldAutocomplete(field, p)` then
  `field.SetAutocompleteFunc(mf.jmsTypeSuggestions)` — style-before-func
  ordering matters, see this session's earlier
  `TestPromptAutocompleteFirstOpenIsReadable` gotcha, same rule applies
  here), and **also calls `field.Autocomplete()` explicitly after
  prefilling the field's text** — found necessary during manual
  verification (see below): `SetText` doesn't itself refresh an active
  drop-down, so without this, opening the dialog fresh showed only the
  scan sentinel (the suggestion list cached from `SetAutocompleteFunc`'s
  eager call at construction time, before any real messages were loaded),
  never the real loaded types, until a keystroke forced a refresh — same
  gotcha as the `:` prompt (spec/01) and its identical fix.
- `jmsTypeSuggestions(currentText string) []string`: prefix-filters
  `mf.host.LoadedJMSTypes()` deduped with `mf.scanned`, then always
  appends the sentinel string (`jmsTypeScanSentinel`) regardless of
  `currentText` — it's an action, not a data suggestion, so it shouldn't
  disappear just because the user has typed something.
- `onJMSTypeChanged` (the field's `SetChangedFunc`) checks
  `if text == jmsTypeScanSentinel`: if so, calls `mf.startScan()` — and
  **does not** clear the field's text itself (see "Key decisions" below
  for why that turned out to be unsafe); otherwise no-ops.
- `startScan()`: no-ops if `mf.scanning` already true (avoid piling up
  duplicate requests if the user selects the sentinel twice quickly).
  Sets `mf.scanning = true`, `mf.host.SetStatus("Scanning up to 5,000
  messages for JMS types...")`, and runs `mf.host.ScanJMSTypes(ctx,
  5000)` in a goroutine; on completion (via `QueueUpdateDraw`),
  `handleScanResult` sets `mf.scanning = false`, **clears the field's
  text back to empty (this is where that actually happens — see below)**,
  and on success sets `mf.scanned` and calls `field.Autocomplete()` to
  refresh the open drop-down immediately, or on error shows it via
  `mf.host.SetStatus` the same way `apply()`'s parse errors already do.
  `apply()` additionally refuses to submit while `mf.scanning` is true,
  since the field visibly holds the sentinel text for that entire window
  now (see below).

### Key decisions

- **Selecting the sentinel via `SetChangedFunc` detection, not
  `SetAutocompletedFunc`.** Traced through `tview.InputField`'s internals
  earlier in this session (for the `:` prompt's rejected padding idea):
  `SetAutocompletedFunc` loses tview's built-in dodge around
  re-triggering `Autocomplete()` on every arrow-key navigation (a private
  variable this package can't reach), risking the suggestion list
  collapsing to a near-empty set after the first arrow press. Detecting
  the sentinel via the field's existing `SetChangedFunc` (already the
  mechanism other filter inputs in this codebase use for live behavior)
  sidesteps that risk entirely.
- **The field's text is cleared in `handleScanResult`, not in
  `onJMSTypeChanged` — found the hard way.** The original plan (as
  written above until manual verification caught this) called
  `mf.jmsTypeItem.SetText("")` directly inside `onJMSTypeChanged`. That
  detection runs *from inside* `tview.InputField`'s own
  `SetText`-triggered change notification (accepting the sentinel via
  Enter calls `SetText(sentinel)`, which invokes `SetChangedFunc`
  synchronously, from which the code then called `SetText` again on the
  *same* field) — a reentrant call into the field's own text buffer while
  its own change callback is still running. Observed live as visibly
  garbled, duplicated text
  (`"...for JMS typesfor JMS typesfor JMS types..."`) — tview's
  underlying buffer isn't safe to mutate reentrantly this way. Moving the
  clear into `handleScanResult` (which runs from a *completed* goroutine's
  `QueueUpdateDraw` — genuinely outside the original input handler's call
  stack, a fresh top-level call) fixes it. The trade-off this creates: the
  field visibly holds the sentinel text for the *entire scan duration*
  now, not just an instant — `apply()`'s new in-flight guard exists
  specifically because of this wider window, not the original "brief
  flash."
- **`scanned` is dialog-local and reset on every `Show()`, not
  persisted or cached across dialog opens.** A stale scan result from
  five minutes ago (possibly a different queue, since the dialog is
  reused across queues) would be actively misleading; re-scanning is a
  deliberate user action each time, not something to "remember."
- **No debounce/cancellation of an in-flight scan if the dialog is
  closed mid-scan.** The goroutine's `QueueUpdateDraw` callback runs
  regardless; if the dialog is no longer visible when it completes, its
  status/suggestion-refresh calls simply have no visible effect (`mf.host.SetStatus`
  and `field.Autocomplete()` on a hidden form are harmless no-ops
  visually, matching how other async loads in this codebase — e.g. SSM
  Parameters' `load()` — already don't guard against the view having
  navigated away mid-fetch).
- **`apply()` refuses to submit while `mf.scanning` is true**, with a
  status message rather than silently doing nothing — added once the
  clear-on-completion design (above) meant the field genuinely holds the
  sentinel text for the whole scan window, not just an instant; without
  this, pressing Apply mid-scan would filter by that literal sentinel
  string as if it were a real JMS type.

## Testing

- `internal/app`: unit tests for `distinctJMSTypes` (dedup, sort, empty
  strings excluded), `LoadedJMSTypes` (reads through
  `messagesV.AllMessages()`, plus a dedicated nil-`messagesV` regression
  test — see "Bugs found" below), and `ScanJMSTypes` (calls
  `Backend().BrowseMessages` with the given queue/maxCount and no
  `JMSType`, via a small local fake `queue.Backend`, since
  `messagesV.Open()`'s async `Load()`+`QueueUpdateDraw` path can't safely
  run against a `*tview.Application` with no running event loop — no
  other test in this package exercises it either).
- `internal/dialog/messagefilter_test.go` (new): `jmsTypeSuggestions`
  returns tier-1 types (plus any scanned) prefix-filtered with the
  sentinel always present; `onJMSTypeChanged`/`startScan` cover the
  synchronous trigger and duplicate-scan guard directly, without ever
  running the real goroutine (same reasoning as `internal/view`'s
  `load()` tests); `handleScanResult` is called directly to cover the
  completion path (success, error, and that it clears the field);
  `TestShowRefreshesStaleAutocomplete` renders to a `tcell.SimulationScreen`
  to catch the stale-cache bug (a plain call-and-check test wouldn't —
  see "Bugs found" below); `TestApplyRefusesWhileScanning` covers the new
  guard.
- Manual verification via the `verify-live` skill against a real broker
  (both backends): confirms the two-tier suggestion behavior end-to-end,
  as described in `spec.md`'s "Manual verification" — this is what
  actually caught both bugs in "Bugs found" below; neither was visible
  from the unit tests alone until written afterward specifically to
  pin them.

## Bugs found during implementation (see tasks.md for the full account)

1. `LoadedJMSTypes` initially panicked on a nil `a.messagesV` —
   `NewMessageFilter` is built before `a.messagesV` in `App.New()`, and
   `SetAutocompleteFunc` eagerly calls `Autocomplete()` once immediately.
   Fixed with a nil guard (same pattern `ReloadAfterSend` already uses in
   the same file).
2. Stale autocomplete cache on first open, and reentrant `SetText`
   corruption when clearing the sentinel — both described in detail
   under "Key decisions" above.

## Trade-offs / risks accepted

- The field visibly holds the sentinel text for the whole scan duration
  (not just a brief flash, as originally planned — see "Key decisions"),
  guarded by `apply()`'s refusal to submit mid-scan.
- 5,000 is a fixed constant, not user-configurable — consistent with
  `defaultBrowseMaxCount` (500) already being a fixed constant, not a
  setting.
