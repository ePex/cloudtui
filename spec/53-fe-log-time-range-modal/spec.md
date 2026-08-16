# Spec — FE 53: Time range modal (relative + absolute) for CloudWatch/Datadog Logs

Date: 2026-08-16

## What

Replace the current `t`-key relative-preset cycling (`15m → 1h → 3h → 24h → 15m …`,
shared by `logSearchView` (CloudWatch Logs) and `datadogLogsView` (Datadog
Logs) via the package-level `timeRangePresets`/`presetIdx`) with a modal
overlay that has two tabs:

- **Relative**: a list of fixed windows — ~~15 minutes, 30 minutes, 1 hour,
  3 hours, 24 hours~~ **revised post-implementation, before commit**: 15
  minutes, 1 hour, 4 hours, 1 day, 2 days, 3 days, 7 days, 15 days, 1 month
  (`1mo` approximated as 30 days — `time.Duration` has no calendar-aware
  month). Selecting one applies immediately and closes the modal, same as
  today's cycling.
- **Absolute**: `From`/`Until` fields for an explicit range, applied via
  an Apply action (free text needs validation before it's usable, unlike
  a list selection). **Revised post-implementation**: the fields accept a
  time of day, not just a date — `YYYY-MM-DD HH:MM` (minute precision, UTC)
  in addition to the existing RFC3339/bare-date formats (decision 4).

`t` opens the modal directly (no more cycling) in both views — **confirmed
live**: cycling goes away entirely, not kept alongside the modal.

## Why

Discussed live: the current presets are fixed and there's no way to look
at an arbitrary historical window (e.g. "what happened between 14:00 and
14:15 yesterday") without a real absolute-range picker.

**Confirmed live**: applies to both CloudWatch Logs and Datadog Logs —
they already share the exact same `timeRangePreset`/`presetIdx` mechanism
today, so one shared component avoids building the tab/date-parsing logic
twice.

## Decisions (proposed — confirm before plan)

1. **One shared, `App`-owned modal**, reused by both views — same
   established pattern as `showMovePicker(sourceQueue, onSelect,
   onClose)`/`showSendMessage`: a single overlay built once in `New()`,
   opened via something like `a.showTimeRangeModal(current timeRange,
   onApply func(timeRange))`, gated by a `timeRangeModalVisible bool`
   field folded into the existing overlay-blocking checks in `app.go`
   (`onGlobalKey`'s `if a.confirmVisible || ... `-style conditions).
2. **New `timeRange` type replaces bare `presetIdx int`** in both
   `logSearchView` and `datadogLogsView`:
   ```go
   type timeRangeMode int
   const (
       timeRangeRelative timeRangeMode = iota
       timeRangeAbsolute
   )
   type timeRange struct {
       mode      timeRangeMode
       presetIdx int       // meaningful when mode == timeRangeRelative
       from, to  time.Time // meaningful when mode == timeRangeAbsolute
   }
   ```
   with `bounds(now time.Time) (start, end time.Time)` and `label()
   string` methods — `search()` in both views calls `tr.bounds(time.Now())`
   instead of today's `time.Now().Add(-timeRangePresets[presetIdx].duration)`,
   and title-building calls `tr.label()` instead of
   `timeRangePresets[presetIdx].label`.
3. **Opening the modal prefills it from the calling view's current
   `timeRange`** (which tab, which preset/dates) — same precedent as
   `showMessageFilter` prefilling from `mv.filter`. Not a global "last
   used" state; each view keeps its own.
4. **Absolute dates reuse the existing parse convention, extended for
   time-of-day** — `parseFilterDate` (née `parseMessageFilterForm`'s
   inline closure) gained a middle layout, `"2006-01-02 15:04"` (tried
   between RFC3339 and the bare `"2006-01-02"` date), **revised
   post-implementation**: the original date-only convention couldn't set
   a time, which the user explicitly asked for before commit; extending
   the shared parser (rather than forking a time-range-only one) avoids
   duplicating the RFC3339/error-message handling. Display uses a *new*
   `formatTimeRangeDateTime` (`"2006-01-02 15:04"`), not the message
   filter's date-only `formatFilterDate` — reusing the latter would
   silently drop the time of day every time the modal reopens (caught
   live via `verify-live` during the original implementation's own manual
   verification pass, before this revision even added the requirement —
   `formatFilterDate` was always date-only, so this was latent from the
   start).
4b. **The new middle layout is interpreted as *local* time, not UTC —
   unlike the unchanged bare-date layout.** Found live, after decision 4
   above was already implemented and re-verified: entering an absolute
   range like `2026-08-11 15:00` to `15:30` (intending local time — the
   only thing a user would naturally type) returned messages timestamped
   `17:29` (2 hours off, matching a UTC+2 local zone). Root cause: the
   results table displays timestamps via `.Local()` (existing, correct,
   predates this feature), but `parseFilterDate`'s bare layouts default to
   UTC when parsed with plain `time.Parse` — so a typed local-looking time
   was silently read as UTC, producing a search window offset by the
   local UTC offset from what was intended. Fix: the new
   `"2006-01-02 15:04"` layout is parsed via
   `time.ParseInLocation(layout, s, time.Local)`, then converted to UTC
   for storage (matching the rest of the app's UTC-internal convention);
   `formatTimeRangeDateTime` and `timeRange.label()` both render via
   `.Local()` so prefill/round-trip and the view's title bar show back
   what the user typed, not the UTC-shifted equivalent.
   **Deliberately scoped to only the new layout** — the bare *date-only*
   `"2006-01-02"` layout keeps its original UTC-midnight interpretation
   unchanged. That's the message filter's (spec 46, already-shipped)
   convention; changing it wasn't asked for, and at day granularity a
   local/UTC mismatch is far less visible than it is at minute
   granularity. This intentionally makes `parseFilterDate`'s two bare
   layouts use different implicit zones — documented here specifically so
   that isn't mistaken for an oversight later.
5. **Relative tab presets**: ~~`15m, 30m, 1h, 3h, 24h`~~ **revised
   post-implementation**: `15m, 1h, 4h, 1d, 2d, 3d, 7d, 15d, 1mo` — wider
   spread requested by the user before commit, trading the original's
   sub-day granularity (30m) for multi-day reach (up to a month). Default
   preset stays "1h" (`defaultPresetIdx` moved from 2 to 1 to track it).
   Replaces cycling as the *only* way to reach these values (the
   underlying `timeRangePreset` slice/type stays, just no longer walked
   via a raw index increment outside the modal).
6. **Tab switching**: ~~Left/Right arrow keys~~ **uppercase `R`/`A`**
   switch between Relative and Absolute — revised at plan stage: Left/Right
   don't work because `tview.InputField` already consumes those for
   in-field cursor movement while a date field has focus, and stealing
   them at a higher level would break editing on the Absolute tab.
   Uppercase (not lowercase `r`/`a`) avoids colliding with typed text in
   the date fields, same reasoning as `'S'`/`'E'` (Service/Env focus
   shortcuts) in spec 42. Up/Down stay reserved for list/field navigation
   *within* a tab, Tab stays reserved for `tview.Form`'s existing
   field-to-field navigation on the Absolute tab. Esc cancels/closes
   without applying, from either tab — same as every other overlay in
   this app.
   **Exact visual tab-indicator styling is a plan-stage detail** (can't
   use literal `[Relative]`/`[Absolute]` — square brackets are swallowed
   as tview color/region tags, same gotcha noted elsewhere in this repo).

## Scope

- `internal/app/logsearch.go`: `timeRangePreset`/`timeRangePresets`
  replaced with the 9-entry list (decision 5); new `timeRange` type +
  `bounds`/`label` methods (shared file, since this is where the preset
  list already lives).
- `internal/app/timerangemodal.go` (new): the shared modal — construction
  (`tview.Pages` with a "relative" `tview.List` page and an "absolute"
  `tview.Form` page), `App.showTimeRangeModal`/`closeTimeRangeModal`,
  tab-switching input capture.
- `internal/app/app.go`: new fields (modal widgets, `onApply` callback,
  `timeRangeModalVisible`), wiring into `New()` and the existing
  overlay-blocking checks.
- `internal/app/logsearch.go` / `internal/app/datadoglogs.go`: replace
  `presetIdx int` with `tr timeRange`; `cycleTimeRange` removed, `'t'`
  now calls `a.showTimeRangeModal(sv.tr, func(tr timeRange) { sv.tr = tr;
  sv.search() })` (analogous for `datadogLogsView`); `search()`/title
  building updated to use `tr.bounds`/`tr.label`.
- Existing tests referencing `presetIdx`/`cycleTimeRange`/
  `timeRangePresets[...].label` updated for the new type.

## Out of scope

- Any other view gaining a time-range filter (this is CloudWatch/Datadog
  Logs only, matching where the shared mechanism already lives).
- Relative-time expressions beyond the fixed preset list (e.g. free-form
  "now-2d" input) — the Absolute tab's explicit From/Until fields cover
  the "arbitrary window" need instead.
- Persisting the last-used range across app restarts (matches spec 42's
  precedent of not persisting filter state either).
- Timezone handling beyond what `parseMessageFilterForm`/`formatFilterDate`
  already do (UTC normalization) — no new timezone picker.
