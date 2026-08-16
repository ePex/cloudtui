# Plan — FE 53

## Approach

1. **`internal/app/logsearch.go`** (where the shared preset machinery
   already lives):
   - `timeRangePresets` is `15m, 1h, 4h, 1d, 2d, 3d, 7d, 15d, 1mo`
     (`1mo` = `30 * 24 * time.Hour`, an approximation — no calendar-aware
     duration type). **Revised post-implementation, before commit**: the
     original plan only added `30m` to the prior 5-entry list; the user
     asked for this wider spread instead. `defaultPresetIdx` moved from 2
     to 1 to keep tracking "1h".
   - New `timeRangeMode` (`timeRangeRelative`/`timeRangeAbsolute`) and
     `timeRange` struct (`mode timeRangeMode`, `presetIdx int`, `from,
     to time.Time`), with:
     ```go
     func (tr timeRange) bounds(now time.Time) (start, end time.Time) {
         if tr.mode == timeRangeAbsolute {
             return tr.from, tr.to
         }
         return now.Add(-timeRangePresets[tr.presetIdx].duration), now
     }
     func (tr timeRange) label() string {
         if tr.mode == timeRangeAbsolute {
             return tr.from.Format("2006-01-02 15:04") + " → " + tr.to.Format("2006-01-02 15:04")
         }
         return timeRangePresets[tr.presetIdx].label
     }
     ```
   - `logSearchView.presetIdx int` → `tr timeRange`; `cycleTimeRange()`
     removed; `open()` sets `sv.tr = timeRange{mode: timeRangeRelative,
     presetIdx: defaultPresetIdx}`; `search()`'s `start := ...` line
     becomes `start, end := sv.tr.bounds(time.Now())`; `updateTitle()`'s
     `preset := timeRangePresets[sv.presetIdx].label` becomes `label :=
     sv.tr.label()`; `'t'` case in `table.SetInputCapture` becomes:
     ```go
     case 't':
         a.showTimeRangeModal(sv.tr, func(tr timeRange) {
             sv.tr = tr
             sv.search()
         })
         return nil
     ```
2. **`internal/app/datadoglogs.go`**: identical treatment —
   `presetIdx int` → `tr timeRange` field, `cycleTimeRange()` removed,
   construction sets `tr: timeRange{mode: timeRangeRelative, presetIdx:
   defaultPresetIdx}`, `search()`/`updateTitle()` updated the same way,
   `'t'` case opens the modal with a callback that sets `dv.tr = tr;
   dv.search()`.
3. **`internal/app/messages.go`**: extract `parseMessageFilterForm`'s
   inner `parseDate` closure into a package-level `parseFilterDate(label,
   s string) (time.Time, error)` (identical body, just no longer a
   closure) so the new modal's Absolute tab can reuse the exact same
   "RFC3339 or bare YYYY-MM-DD" parsing without duplicating it.
   `parseMessageFilterForm` calls the extracted function instead of
   defining it inline; behavior unchanged.
4. **`internal/app/timerangemodal.go`** (new file) — the shared modal,
   built once in `App.New()` (construction lives in `app.go` alongside
   every other overlay per existing convention; the *methods* live here):
   - Widgets: `timeRangeTabs *tview.TextView` (`SetDynamicColors(true)`,
     renders the two tab labels, active one in the theme's accent color
     via a real `[color]...[-]` tag — not literal brackets, see spec
     decision 6), `timeRangeRelativeList *tview.List` (9 items, one per
     `timeRangePresets` entry — built via a loop, not hardcoded, so the
     revised preset list needed no construction-code change),
     `timeRangeAbsoluteForm *tview.Form` (From/Until `AddInputField`s +
     Apply/Cancel buttons, labels "From/Until (YYYY-MM-DD HH:MM or
     RFC3339)" — **revised post-implementation**: originally matched
     `messageFilterForm`'s date-only "From (RFC3339 or YYYY-MM-DD)"
     wording, changed when the user asked for time-of-day support before
     commit), `timeRangePages *tview.Pages` (two pages, "relative" /
     "absolute"), `timeRangeFlex *tview.Flex` (tabs row + pages), wrapped
     via the existing `centered(timeRangeFlex, 72, 14)` helper (width 72 /
     height 14, not the originally-planned 50x12 — width caught live via
     `verify-live` even before the time-of-day revision (the *date-only*
     label already overflowed 50), then widened again for the longer
     time-aware label; height bumped from 12 to 14 to fit the relative
     list's 9 items, up from 5) and added to `a.rootPages` as
     `"time-range"` (hidden initially),
     following the exact `messageFilterOverlay`/`movePickerOverlay`
     precedent.
   - `App` fields: `timeRangeFlex`, `timeRangeTabs`, `timeRangePages`,
     `timeRangeRelativeList`, `timeRangeAbsoluteForm`, `timeRangeVisible
     bool`, `timeRangeActiveTab timeRangeMode`, `timeRangeOnApply
     func(timeRange)`.
   - `showTimeRangeModal(current timeRange, onApply func(timeRange))`:
     stores `onApply`; pre-selects the Relative list item or pre-fills
     the Absolute fields from `current` (mirrors `showMessageFilter`
     prefilling from `mv.filter`); sets `timeRangeActiveTab =
     current.mode`, calls `switchTimeRangeTab` to sync the Pages/focus/
     tab-label rendering; `ShowPage("time-range")`; sets
     `timeRangeVisible = true`.
   - `closeTimeRangeModal()`: `HidePage("time-range")`,
     `timeRangeVisible = false`, focus back to whichever table was
     active before (handled naturally since the caller's own `onApply`/
     the Esc-cancel path don't touch table focus — the overlay was
     covering it, not replacing it).
   - `switchTimeRangeTab(mode timeRangeMode)`: sets
     `timeRangeActiveTab`, re-renders `timeRangeTabs`'s text, calls
     `timeRangePages.SwitchToPage(...)`, focuses the relevant widget
     (`timeRangeRelativeList` or the Absolute form's first field).
   - `applyTimeRangeRelative(presetIdx int)`: builds a
     `timeRange{mode: timeRangeRelative, presetIdx: presetIdx}`, closes
     the modal, calls `timeRangeOnApply` — wired as each
     `timeRangeRelativeList` item's selected-func.
   - `applyTimeRangeAbsolute()`: reads both input fields, parses via the
     extracted `parseFilterDate`; on error, writes to the status bar and
     leaves the modal open (same pattern as `applyMessageFilter`'s
     parse-error handling); on success, builds `timeRange{mode:
     timeRangeAbsolute, from, to}`, closes, calls `timeRangeOnApply` —
     wired as the Absolute form's "Apply" button. **Revised
     post-implementation**: `parseFilterDate` itself gained a middle
     layout, `"2006-01-02 15:04"` (between RFC3339 and the bare date), so
     this tab can accept a time of day — see spec decision 4. Prefill
     (`showTimeRangeModal`) uses a new `formatTimeRangeDateTime` (same
     layout) instead of `formatFilterDate`, which is date-only and would
     silently drop the time on every reopen. **Revised again**: the new
     middle layout parses as local time (`time.ParseInLocation(...,
     time.Local)`), not UTC like the unchanged bare-date layout —
     `formatTimeRangeDateTime` and `timeRange.label()` both render via
     `.Local()` to match — see spec decision 4b (found live: a UTC-parsed
     input silently disagreed with the results table's `.Local()`
     display, offset by the local UTC offset).
   - Input capture on `timeRangeFlex` (checked before descending to
     children, so it fires regardless of which child currently has
     focus): `Esc` → `closeTimeRangeModal()` (no apply — same as every
     other overlay's cancel path); `'R'`/`'A'` (uppercase) →
     `switchTimeRangeTab(timeRangeRelative)`/`switchTimeRangeTab(timeRangeAbsolute)`.
     **Uppercase specifically to avoid colliding with typed text in the
     Absolute tab's date fields** — same reasoning spec 42 already used
     for `'S'`/`'E'` (Service/Env focus shortcuts) over lowercase; ruled
     out Left/Right arrows for tab-switching because `tview.InputField`
     already consumes those for in-field cursor movement while a date
     field has focus, so stealing them at a higher level would break
     editing.
5. **`app.go`**: add the new fields to the `App` struct; construct the
   modal's widgets in `New()` right after `messageFilterForm`'s block
   (same file region, same style as the existing overlay constructions);
   add `"time-range"` to the `rootPages` `AddPage` chain; fold
   `timeRangeVisible` into the existing overlay-blocking boolean checks
   in `onGlobalKey` (the `if a.confirmVisible || a.movePickerVisible ||
   ...` conditions) so global hotkeys don't fire while the modal is open.

## Files touched

- `tui/internal/app/logsearch.go`
- `tui/internal/app/logsearch_test.go`
- `tui/internal/app/datadoglogs.go`
- `tui/internal/app/datadoglogs_test.go`
- `tui/internal/app/messages.go` (extract `parseFilterDate`; later gains
  the local-time `filterDateTimeLayout` "YYYY-MM-DD HH:MM" layout,
  distinct from the unchanged UTC `filterDateLayout` "YYYY-MM-DD" one)
- `tui/internal/app/messages_test.go` (case for the new layout)
- `tui/internal/app/timerangemodal.go` (new)
- `tui/internal/app/timerangemodal_test.go` (new)
- `tui/internal/app/app.go`
- `spec/53-fe-log-time-range-modal/tasks.md` (next gate)

## Key decisions

- **`timeRange` as a value type with `bounds`/`label` methods**, not an
  interface — matches this codebase's preference for small concrete
  types over polymorphism when there are only ever two variants (same
  spirit as `queue.MessageFilter` being a flat struct, not a filter
  interface hierarchy).
- **One shared modal, not two.** `logSearchView` and `datadogLogsView`
  already share `timeRangePreset`/`timeRangePresets`; duplicating the
  tab/date-parsing UI per view would be exactly the kind of "reuse over
  duplicate" violation `CLAUDE.md` calls out.
- **`'R'`/`'A'` over Left/Right for tab switching** — the concrete
  reason (InputField needs Left/Right for cursor movement) is a real
  constraint discovered while designing this, not a style preference;
  documented in the modal's own code comment, not just here.
- **`parseFilterDate` extraction is intentionally minimal** — pure
  mechanical extraction of an existing closure into a package-level
  function, no behavior change, done only because the new modal needs
  the exact same parsing and duplicating it would drift over time.
  (The parsing itself later gained a middle layout for time-of-day
  support — see spec decision 4 — but that's a behavior change layered
  on top of this extraction, not part of it.)
- **Relative presets and Absolute time-of-day support were both revised
  after the original implementation was already live-verified and
  working**, at the user's explicit request, before commit — not a
  correction of a bug, a deliberate widening of scope. Both required
  re-running `verify-live` (new preset count needed a taller box; the new
  label wording needed a wider one — same failure mode as the original
  width bug, just triggered again by new content).
- **The local-vs-UTC bug (spec decision 4b) was a genuine bug, unlike the
  two revisions above** — the user hit it by actually using the feature
  ("I filtered 15:00-15:30 and got a message timestamped 17:29"), not by
  asking for different behavior. It was latent from the moment the
  Absolute tab could accept a time of day, undetected by both unit tests
  (which don't exercise the machine's real `time.Local`) and the earlier
  `verify-live` pass (which typed values and immediately re-read them
  back through the same UTC-consistent code path, so a self-consistent-
  but-wrong round trip looked correct).
- **Only the new minute-precision layout was changed to local time — the
  existing bare-date layout stays UTC.** Both bugs share one root cause
  (parse-as-UTC vs. display-as-local) and one function
  (`parseFilterDate`), but the date-only layout backs the already-shipped
  message filter (spec 46); changing its interpretation wasn't requested
  and wasn't this fix's job. `filterDateLayout`/`filterDateTimeLayout` are
  now two separate named constants (not one shared slice) specifically so
  this split is visible in the code, not just documented here.
- **No new "time-range" package** — this stays in `internal/app` like
  every other view-local concern in this codebase (`internal/queue`,
  `internal/datadoglogs`, etc. are for actual backend/API clients, not
  UI state).
- **`switchTimeRangeTab` resets `timeRangeAbsoluteForm`'s internal focus
  to item 0 before focusing it** — `tview.Form` remembers its last-focused
  item across `Application.SetFocus` calls (the same gotcha
  `verify-live`'s doc already calls out for the connection editor).
  Without `a.timeRangeAbsoluteForm.SetFocus(0)`, reopening the modal after
  a previous Absolute-tab session that ended on Apply/Cancel would leave
  keystrokes going to the button, not the From field. Caught live, not by
  unit tests (the tests call `showTimeRangeModal`/`switchTimeRangeTab`
  directly and assert on state, not on where a `tview.Form` actually
  routes a keystroke).

## Manual verification

Per `tui/CLAUDE.md`, UI behavior like this needs driving the real binary
(`verify-live` skill's tmux-driving approach applies even though this
isn't queue/broker-related — same "a screen doesn't behave like the code
reads" risk `tview.Modal`/`DropDown` gotchas already taught this repo):

- Open CloudWatch Logs and Datadog Logs, press `t` in each — modal opens
  with the Relative tab active, current selection highlighted.
- `R`/`A` switch tabs from anywhere in the modal, including while a date
  field has focus (confirm typing in the field still works and doesn't
  accidentally trigger a tab switch on lowercase `r`/`a` if any date
  format ever needs them — it doesn't today, but worth eyeballing).
- Select a relative preset — applies immediately, modal closes, table
  refetches, title shows the new label.
- Switch to Absolute, enter valid From/Until values, Apply — same result
  with the absolute range in the title.
- Enter an invalid date, Apply — status bar shows the parse error, modal
  stays open (matches `applyMessageFilter`'s existing error behavior).
- Esc from either tab — modal closes, no change to the active search.
- Confirm the tab-indicator coloring is actually legible against both a
  light and dark theme (per this repo's `SetListStyles`/`styleDropDown`
  history of these being easy to get wrong).
