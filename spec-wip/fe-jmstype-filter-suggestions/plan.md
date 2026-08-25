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
  reset already there) and wires the JMS Type field's autocomplete:
  `ui.StyleInputFieldAutocomplete(field, p)` then
  `field.SetAutocompleteFunc(mf.jmsTypeSuggestions)` (style-before-func
  ordering matters — see this session's earlier
  `TestPromptAutocompleteFirstOpenIsReadable` gotcha, same rule applies
  here).
- `jmsTypeSuggestions(currentText string) []string`: prefix-filters
  `mf.host.LoadedJMSTypes()` deduped with `mf.scanned`, then always
  appends the sentinel string (`scanSentinel` constant) regardless of
  `currentText` — it's an action, not a data suggestion, so it shouldn't
  disappear just because the user has typed something.
- The field's `SetChangedFunc` (not currently used on this field) checks
  `if text == scanSentinel`: if so, `SetText("")` (guarded against
  re-entrant firing — see Testing) and calls `mf.startScan()`; otherwise
  no-ops (the field's normal text just changed, nothing special to do —
  suggestions refresh automatically via tview's own
  `Autocomplete()`-on-text-change behavior).
- `startScan()`: no-ops if `mf.scanning` already true (avoid piling up
  duplicate requests if the user selects the sentinel twice quickly).
  Sets `mf.scanning = true`, `mf.host.SetStatus("Scanning up to 5,000
  messages for JMS types...")`, and runs `mf.host.ScanJMSTypes(ctx,
  5000)` in a goroutine; on completion (via `QueueUpdateDraw`), sets
  `mf.scanned` to the result (deduped against what tier 1 already
  offered — cosmetic only, `jmsTypeSuggestions` already dedupes),
  `mf.scanning = false`, clears the status, and calls
  `field.Autocomplete()` to refresh the open drop-down immediately. An
  error is shown via `mf.host.SetStatus` the same way `apply()`'s parse
  errors already are.

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
  sidesteps that risk entirely, at the cost of a brief visible moment
  where the field literally contains the sentinel text before it's
  cleared back — an accepted, minor UX blip in exchange for not risking
  the navigation bug.
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

## Testing

- `internal/app`: unit tests for `distinctJMSTypes` (dedup, sort, empty
  strings excluded), `LoadedJMSTypes` (reads through
  `messagesV.AllMessages()`), and `ScanJMSTypes` (calls
  `Backend().BrowseMessages` with the given queue/maxCount and no
  `JMSType`, via the existing fake backend pattern already used for
  other `Host`-method tests).
- `internal/dialog/messagefilter_test.go` (new): `jmsTypeSuggestions`
  returns tier-1 types prefix-filtered plus the sentinel always present;
  simulating the sentinel's `SetChangedFunc` firing triggers a scan
  (fake `Host.ScanJMSTypes`) and the result is reflected in subsequent
  suggestions; a second sentinel trigger while `scanning` is a no-op
  (fake records call count).
- Manual verification via the `verify-live` skill against a real broker
  (both backends): confirms the two-tier suggestion behavior end-to-end,
  as described in `spec.md`'s "Manual verification".

## Trade-offs / risks accepted

- The brief visible sentinel-text flash when accepting it (see "Key
  decisions" above) is a minor, accepted cosmetic quirk, not fixed, to
  avoid a worse (navigation-breaking) risk.
- 5,000 is a fixed constant, not user-configurable — consistent with
  `defaultBrowseMaxCount` (500) already being a fixed constant, not a
  setting.
