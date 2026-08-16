# Tasks — FE 53

1. [x] `internal/app/messages.go`: extract `parseMessageFilterForm`'s
   inline date-parsing closure into package-level `parseFilterDate`
   (mechanical, no behavior change).
2. [x] `internal/app/logsearch.go`: add `timeRangeMode`/`timeRange`
   (`bounds`/`label`); `timeRangePresets` revised to `15m, 1h, 4h, 1d, 2d,
   3d, 7d, 15d, 1mo` (originally just added `30m` — widened before commit,
   see spec decision 5).
3. [x] `internal/app/timerangemodal.go` (new) + `app.go` wiring: the
   shared Relative/Absolute modal (widgets, `showTimeRangeModal`/
   `closeTimeRangeModal`/`switchTimeRangeTab`/`applyTimeRangeRelative`/
   `applyTimeRangeAbsolute`, `R`/`A` tab switching, `Esc` cancel);
   `timerangemodal_test.go`.
4. [x] `internal/app/logsearch.go` + `logsearch_test.go`: replace
   `presetIdx` with `tr timeRange`, remove `cycleTimeRange`, wire `'t'`
   to `a.showTimeRangeModal`.
5. [x] `internal/app/datadoglogs.go` + `datadoglogs_test.go`: identical
   treatment for the Datadog Logs view.

## Manual verification

Done via `verify-live` (tmux-driving the real binary), launched from an
**isolated empty working directory** (no `config.yaml` in cwd, so
`config.Default()` applies — no active AWS profile, no Datadog token) —
necessary because the first attempt, run from `tui/` with the real local
`config.yaml`, triggered a live AWS SSO browser login on entering
CloudWatch Logs using the real active profile. That session was killed
immediately without letting it proceed. `datadoglogs.Search`'s synchronous
"access token not configured" guard makes the Datadog Logs screen safe to
drive with no token configured; CloudWatch Logs' equivalent guard is at
the log-*group-list* level (`logsView.load`), one screen before
`logSearchView` (the one with the `'t'` key) — since the modal is a
single App-owned instance shared byte-for-byte between both views (same
widgets, same methods), verifying it live through Datadog Logs exercises
the real modal `logSearchView` would show too; `logSearchView`'s own
`'t'`-key wiring is covered separately by
`TestLogSearchViewTKeyOpensTimeRangeModal`.

- [x] Open Datadog Logs, press `t` — modal opens with the Relative tab
      active, current selection ("1h") highlighted. (CloudWatch Logs not
      reachable live without real AWS credentials — see above; its `'t'`
      wiring is unit-tested instead.)
- [x] `R`/`A` switch tabs from anywhere in the modal, including while a
      date field has focus — confirmed, and confirmed lowercase `r`/`a`
      typed into a date field are *not* intercepted (pass through as
      literal text).
- [x] Select a relative preset — applies immediately, modal closes, a new
      search fires.
- [x] Switch to Absolute, enter valid From/Until values, Apply — applies
      and closes the same way.
- [x] Enter an invalid date, Apply — status bar shows
      `invalid from "not-a-date": want RFC3339 or YYYY-MM-DD`, modal
      stays open.
- [x] Esc from either tab — modal closes, no change to the active search.
- [x] Tab-indicator coloring confirmed theme-driven (re-renders per
      `a.cfg.Colors.Accent`/`Text`, not hardcoded) across this repo's two
      shipped themes (`dark`, `cyberpunk`), both legible against their
      backgrounds. **Caveat**: this repo currently ships no light theme
      (`dark`/`cyberpunk` are both dark-background), so "legible in a
      light theme" specifically couldn't be exercised — nothing to fix,
      just a gap in what themes exist to test against.

Two real bugs found live and fixed (neither caught by unit tests):

1. **Absolute tab's field labels overflowed the modal's right border** —
   the box was 50 wide; `messageFilterForm` uses 64 for the identical
   "From/Until (RFC3339 or YYYY-MM-DD)" label wording. Fixed:
   `centered(a.timeRangeFlex, 64, 12)`.
2. **Reopening the modal after a previous Absolute-tab session left
   keystrokes going nowhere** — `tview.Form` remembers its last-focused
   item (Apply/Cancel) across `Application.SetFocus` calls, so typing
   after reopening did nothing until the form's *own* internal focus was
   explicitly reset. Fixed: `switchTimeRangeTab` now calls
   `a.timeRangeAbsoluteForm.SetFocus(0)` before focusing the form.

## Revision: wider presets + Absolute time-of-day (before commit)

Requested by the user after the above was already implemented and
verified: relative presets widened to `15m, 1h, 4h, 1d, 2d, 3d, 7d, 15d,
1mo`, and the Absolute tab's From/Until fields extended to accept a time
of day (`YYYY-MM-DD HH:MM`), not just a date. Implementation:
`parseFilterDate` (`messages.go`) gained the middle `filterDateTimeLayout`
layout; new `formatTimeRangeDateTime` (time-preserving prefill, unlike the
message filter's date-only `formatFilterDate`); relative list construction
was already loop-driven so the new 9-entry preset list needed no
widget-count changes. See spec decisions 4/5 and plan.md's "Key decisions"
for the full rationale.

Re-verified via `verify-live` (same isolated-empty-directory setup as
above, Datadog Logs only):

- [x] All 9 relative presets render inside the resized box (72x14, up
      from 64x12) with no scrolling/clipping needed.
- [x] Absolute tab's new labels ("From/Until (YYYY-MM-DD HH:MM or
      RFC3339)") fit inside the box.
- [x] Typing a value with a time (`2026-08-01 09:30`) into the From field
      works and displays correctly.
- [x] Applying an absolute range with times, then reopening the modal —
      the From/Until fields show the full `YYYY-MM-DD HH:MM` value,
      confirming the time of day round-trips (not just the date).

Unit tests: `TestApplyTimeRangeAbsoluteWithTime` (new),
`TestShowTimeRangeModalAbsolutePrefill` (updated to use non-midnight
times), a `messages_test.go` case for the new `filterDateTimeLayout`
entry, and `TestTimeRangeBounds`/`TestTimeRangeLabel` updated for the new
preset indices.

## Bugfix: Absolute tab parsed local-looking input as UTC (before commit)

Reported by the user immediately after using the time-of-day feature
above: filtering `2026-08-11 15:00` to `15:30` returned a message
displayed as `17:29` (2h off — a UTC+2 local zone), even though the
message's own raw log content showed `15:29:59`. Root cause: the results
table displays timestamps via `.Local()` (existing, correct, predates
this feature), but the newly-added datetime layout parsed as UTC — so a
value the user naturally typed as local time was read as UTC, silently
searching a different (UTC-labeled) window offset by the local UTC
offset from what was intended. Full diagnosis and fix rationale in spec
decision 4b and plan.md's "Key decisions".

Fix: the new layout now parses via
`time.ParseInLocation(filterDateTimeLayout, s, time.Local)` instead of
plain `time.Parse` (which defaults to UTC for a zone-less layout);
`formatTimeRangeDateTime` and `timeRange.label()` both render via
`.Local()` to match. The unchanged bare-date layout (`filterDateLayout`,
backing the already-shipped message filter) deliberately keeps its
original UTC-midnight interpretation — not touched by this fix.

- [x] Re-verified live: entered `2026-08-11 15:00`→`15:30`, applied, no
      error; reopened the modal — From/Until fields showed back exactly
      `15:00`/`15:30` (before the fix, this round-trip would have shown
      the UTC-shifted `17:00`/`17:30`).

Unit tests updated to assert against `time.Local`-built fixtures instead
of hardcoded UTC ones (so they pass regardless of the test machine's
timezone, per `.Equal()` comparing absolute instants, not wall-clock
strings): `TestParseMessageFilterForm`'s date+time case,
`TestShowTimeRangeModalAbsolutePrefill`, `TestApplyTimeRangeAbsoluteWithTime`,
`TestTimeRangeLabel`'s absolute case.
