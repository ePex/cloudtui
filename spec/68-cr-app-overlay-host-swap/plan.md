# Plan — CR 68: swap 8 overlays to `host ui.Host`

## Approach

One task per overlay (8 total), each independently buildable — each
overlay's `app`/`host` field is private to that type, so swapping one
doesn't affect the others, and `New()` needs **no changes at all**:
Go interfaces satisfy structurally, so `newConfirmDialog(a)` (passing
`*App`) keeps compiling unchanged even after `newConfirmDialog`'s
parameter type changes from `*App` to `ui.Host` — confirmed by
checking every one of the 8 `New()` call sites (`app.go:270-309`), all
plain `newXxx(a)`, no cast needed either way.

### Substitution table (applies across all 8 files)

| Old (`app *App` accessed as `X.app.xxx` / local `a := X.app`) | New (`host ui.Host` accessed as `X.host.Xxx()`) |
|---|---|
| `.rootPages.ShowPage(name)` | `.ShowPage(name)` |
| `.rootPages.HidePage(name)` | `.HidePage(name)` |
| `.tv.SetFocus(p)` (any `p` other than `.pages`) | `.SetFocus(p)` |
| `.tv.SetFocus(.pages)` (the "return to main" fallback) | `.FocusMain()` |
| `.tv.QueueUpdateDraw(f)` | `.QueueUpdateDraw(f)` |
| `.statusBar.SetText(s)` | `.SetStatus(s)` |
| `.contextPanel.SetText(s)` | `.SetContextHint(s)` |
| `.cfg` (any field read: `.cfg.Colors.X`, `.cfg.Theme`, `.cfg.Datadog.X`, `.cfg.ActiveAWSProfile`) | `.Config().` (same field path) |
| `.backend` | `.Backend()` |
| `.switchTheme(name)` | `.SwitchTheme(name)` |
| `.SaveDatadogConfig(cfg)` | unchanged method name, just via `.host.` |
| `.SetActiveAWSProfile(name)` | unchanged method name, just via `.host.` |
| `.listAWSProfiles(ctx)` | `.ListAWSProfiles(ctx)` |
| `.ReloadAfterSend(q)` | unchanged method name, just via `.host.` |
| `.ApplyMessagesFilter(f)` | unchanged method name, just via `.host.` |

Plus two call sites that currently reach past `*App` into `messagesV`
directly (not covered by the table above, since they predate CR 67's
`MessagesFilter()`/`FocusMessages()` additions — CR 67 added the
methods but only rewired `apply`/`clear`, not `show`/`close`):

- `messagefilter.go`, `show()`: `mv := mf.app.messagesV; ...mv.filter.X...`
  → `f := mf.host.MessagesFilter(); ...f.X...`
- `messagefilter.go`, `close()`: `mf.app.tv.SetFocus(mf.app.messagesV.table)`
  → `mf.host.FocusMessages()`

### Per-file notes (struct field rename `app *App` → `host ui.Host`,
constructor param `a *App` → `host ui.Host`, struct literal `app: a` →
`host: host`, applied throughout; only file-specific notes below)

- **`confirm.go`**: every touch is chrome (`ShowPage`/`HidePage`/
  `SetFocus`/`FocusMain`) — no `Config()`/other Host methods needed.
- **`movepicker.go`**: constructor closures (`SetDoneFunc`/
  `SetInputCapture` registered in `newMovePicker`) capture `a`; these
  become closures capturing `host` instead — same shape, just renamed.
  `show()`'s goroutine also uses `host.Backend()` and
  `host.QueueUpdateDraw(...)`.
- **`sendmessage.go`**: same constructor-closure-capture note as
  `movepicker.go`. `doSend`'s goroutine uses `host.Backend()` and
  `host.QueueUpdateDraw(...)`.
- **`messagefilter.go`**: the two extra call sites noted above
  (`show`/`close`), on top of the table's substitutions in `apply`/
  `clear` (already partially on `Host` methods since CR 67 — just the
  receiver changes from `a`/`mf.app` to `host`/`mf.host`).
- **`timerangemodal.go`**: `renderTabs()` uses `host.Config().Colors.Accent`/
  `.Text`; `switchTab()` and `close()` use `host.SetFocus(...)`/
  `host.FocusMain()`; `applyAbsolute()` uses `host.SetStatus(...)`
  (twice, one per parse-error branch).
- **`datadogsettings.go`**: `show()` reads
  `host.Config().Datadog.Site`/`.AccessToken`; `save()`'s
  `a.SaveDatadogConfig(...)` becomes `de.host.SaveDatadogConfig(...)`
  (the `SaveDatadogConfig` method itself, defined lower in the same
  file as `func (a *App) SaveDatadogConfig(...)`, is untouched — it's
  the `Host`-implementing side, not the overlay).
- **`settings.go`** (`themePicker` only): `show()` reads
  `host.Config().Theme` (twice) and calls `host.SwitchTheme(n)`
  inside the per-item closure.
- **`awsprofiles.go`** (`awsProfilesPicker` only): constructor reads
  `host.Config().Colors.X` (was `a.cfg.Colors.X`, 4 places, all at
  construction time); `setHeader()`/`repaint()` read
  `host.Config().Colors`; `repaint()` also reads
  `host.Config().ActiveAWSProfile`; `populate()` calls
  `host.ListAWSProfiles(ctx)`; `activate()`'s
  `a.SetActiveAWSProfile(name)` becomes `ap.host.SetActiveAWSProfile(name)`,
  its trailing `a.statusBar.SetText(...)` becomes `ap.host.SetStatus(...)`.

## Files touched

`confirm.go`, `movepicker.go`, `sendmessage.go`, `messagefilter.go`,
`timerangemodal.go`, `datadogsettings.go`, `settings.go`, `awsprofiles.go`
— each gets its `Themeable`/`View`/`Shortcuttable` `var _` assertions
left untouched (those are unrelated to this swap) and no new assertion
added for `Host` (overlays don't implement `Host` — `*App` does; the
overlay just *depends on* something satisfying it).

## Key decisions

- **`Config()` returns by value** (established in CR 67) — every
  `.Config().X` read is a snapshot at call time, matching the exact
  semantics `.cfg.X` had (both were reading through `*App`'s live
  field; `Config()` just adds one indirection, no staleness risk since
  Go evaluates the call fresh each time it's written).
- **Constructor-time `Config()` reads stay one-shot, not cached** —
  e.g. `awsprofiles.go`'s constructor reads `host.Config().Colors.X`
  once, same as it read `a.cfg.Colors.X` once before. No behavior
  change; palette updates after construction go through `ApplyPalette`
  either way, already unaffected by this CR.
- **No new tests, no new dependencies.**

## Testing

Per spec.md: live-verify (`verify-live` skill) a representative sample
given the volume — `confirmDialog` (a delete confirmation),
`movePicker` (moving a message), `themePicker` (switching theme via
the picker specifically, not `:theme`), `timeRangeModal` (open + apply
a relative preset). `sendMessageOverlay`/`messageFilter`/
`datadogEditor`/`awsProfilesPicker` already got live coverage in CR 66/
67 for their behavior-adjacent methods — re-run a quick smoke check on
each (not the full flow again) to confirm the retype didn't silently
break something CR 66/67's narrower check missed.

## Definition of done

Unchanged from spec.md — all 8 overlays hold `host ui.Host`, `go build`/
`go test` pass, no leftover `.app.` access in these 8 files, live
verification per the sample above shows no behavior change.
