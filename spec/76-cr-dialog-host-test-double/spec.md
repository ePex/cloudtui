# Spec — CR 76: `ui.Host` test double, proven on `connEditor`/`datadogEditor`/`timeRangeModal`

Date: 2026-08-17

## Background

The next step toward closing phase 3 of `spec/64` is the physical
move of the 10 overlay types into `internal/dialog`. Before scoping
that move, audited how the overlays' existing tests would fare once
the type they test lives in a different package from `internal/app`.

Every test for these overlays today builds a full `*App` via
`New(config.Default())`, then reaches directly into the overlay's
unexported fields (`.visible`, `.list`, `.table`, `.form`, `.flex`,
`.absoluteForm`, `.activeTab`) and unexported methods (`.close()`,
`.save()`, `.activate()`, `.applyRelative()`, ...) — all of which
compile today only because the test file and the overlay are in the
same package. Read all four test files with dedicated overlay
coverage in full (`connections_test.go`, `datadogsettings_test.go`,
`timerangemodal_test.go`, `awsprofiles_test.go`) to see how each test
actually uses `*App`. They split into three shapes:

1. **Pure overlay-logic tests** — the large majority. They only
   exercise the overlay itself (open/close, key handling, prefill from
   `Show()`'s arguments, internal state transitions like tab
   switching). `*App` is present only to obtain a `ui.Host` and to
   satisfy sibling-reference constructor params (`connEditor` needs a
   `*connManager`) — nothing about `*App` itself is under test.
2. **True `App`/`Host` integration tests** — a handful. E.g.
   `TestSaveDatadogEditorRoundTrip` verifies a save actually persists
   to `config.yaml` on disk (that's `App`'s real `SaveDatadogConfig`
   implementation, not `DatadogEditor` — see `internal/app/host.go`),
   and `TestActivateAWSProfilePersistsAndUpdatesUI` verifies
   activating a profile also updates `App`'s info panel and settings
   list, which only `App` wires together.
3. **One misplaced test** — `TestDatadogSettingsLabel` tests
   `settings.go`'s `datadogSettingsLabel` function, not `DatadogEditor`
   at all; it's in `datadogsettings_test.go` only because the two
   files were adjacent.

Shape 1 breaks once the type moves — unexported cross-package access
stops compiling. Shapes 2 and 3 are unaffected by the move itself (they
don't reach into an overlay's internals in a way a package boundary
would block, or they don't test the overlay at all) — nothing to do
for them in this CR.

## Problem

`internal/dialog` can't import `internal/app` (the cycle CR 75 already
established), so shape-1 tests can no longer get their `ui.Host` from
a real `*App`. They need a lightweight, hand-built `ui.Host`
implementation instead — but no such test double exists yet anywhere
in the codebase, and `ui.Host` has 20 methods spanning chrome, status,
config, connections, Datadog, AWS profiles, and message filtering.
Designing it well (once) now, and proving it against real tests before
the physical move, de-risks that move — the alternative is discovering
design gaps in the test double *during* a much larger CR that's also
moving files and rewriting imports.

## Solution

Build a `testHost` type (unexported, test-only, in a new
`internal/app/hosttest_test.go`) implementing all 20 `ui.Host` methods:

- **Void/side-effecting methods** (`ShowPage`, `HidePage`, `SetFocus`,
  `FocusMain`, `SetStatus`, `SetContextHint`, `SwitchTheme`,
  `SwitchConnection`, `SaveConnection`, `DeleteConnection`,
  `SaveDatadogConfig`, `SetActiveAWSProfile`, `ReloadAfterSend`,
  `ApplyMessagesFilter`) — each records its call (arguments, or a
  simple counter) on the `testHost` struct, so a test can assert
  "`Show()`'s Esc handler called `close()`, which called
  `host.FocusMain()`" the same way it previously asserted against real
  `App` state.
- **Data methods** (`Config`, `Backend`, `ListAWSProfiles`,
  `MessagesFilter`) — return canned/injectable data: `Config()` reads
  a plain `config.Config` field the test sets directly before
  constructing the overlay (mirrors today's `a.cfg.Datadog = ...`
  pattern); `Backend()` reuses the existing `fakeQueueBackend`
  (`queues_test.go`, same package — no new mock needed);
  `ListAWSProfiles` reads an injectable func field (mirrors today's
  `a.listAWSProfiles = func(...) {...}` pattern, just moved from an
  `App` field to a `testHost` field).
- **`QueueUpdateDraw(f func())`** — calls `f()` synchronously. None of
  the tests converted in this CR exercise a goroutine+`QueueUpdateDraw`
  path (`movePicker.Show`/`sendMessageOverlay.doSend` are the only two
  call sites using that pattern, and neither has any dedicated test
  today — confirmed by grep, zero hits) — a synchronous stub is
  correct for everything actually exercised here, and it's what those
  two files would need too if they ever gain tests. Not addressed in
  this CR (no existing tests need it).

Proved by converting the smaller, structurally-varied files first
(sibling-reference construction, config-driven prefill, and the
biggest single behavioral surface):

- `connections_test.go` (`ConnEditor`, 2 tests) — proves constructing
  the overlay directly (`NewConnEditor(host, NewConnManager(host,
  NewConfirmDialog(host)))`) instead of through `App.New()`, and that
  the sibling-reference pattern (`ConnEditor.manager`) works with
  hand-built collaborators, not just `App`-constructed ones.
- `datadogsettings_test.go` (`DatadogEditor`, 5 tests) — proves
  `Config()`-driven prefill (`TestDatadogEditorPrefillsFromConfig`)
  and separates the one true integration test
  (`TestSaveDatadogEditorRoundTrip`, left as-is — still compiles,
  still valid, not part of this CR's scope) from the misplaced one
  (`TestDatadogSettingsLabel`, relocated to a new `settings_test.go`).
- `timerangemodal_test.go` (`TimeRangeModal`, 13 tests) — the largest
  single behavioral surface (tab switching, relative/absolute apply,
  parse-error status reporting), proving the pattern scales to a type
  with real internal state transitions, not just open/close.

`awsprofiles_test.go` (14 tests, the largest file) is deliberately
**not** touched in this CR — see Scope below.

## Scope

### In scope

- New `internal/app/hosttest_test.go`: `testHost` type, all 20
  `ui.Host` methods, `var _ ui.Host = (*testHost)(nil)`.
- `connections_test.go`: both tests converted to construct
  `ConnEditor`/`ConnManager`/`ConfirmDialog` directly via `testHost`.
- `datadogsettings_test.go`: the 3 pure-overlay tests
  (`TestDatadogEditorEscapeCloses`, `TestDatadogEditorOtherKeysPassThrough`,
  `TestDatadogEditorPrefillsFromConfig`) converted; `TestDatadogSettingsLabel`
  relocated to a new `settings_test.go` (unchanged otherwise);
  `TestSaveDatadogEditorRoundTrip` left as-is.
- `timerangemodal_test.go`: all 13 tests converted, including
  replacing the one `a.statusBar.GetText(true)` check
  (`TestApplyTimeRangeAbsoluteInvalidDate`) with an assertion against
  `testHost`'s recorded `SetStatus` argument.
- `gofmt`/`go vet`/`go build`/`go test` clean across `tui/`.

### Out of scope

- `awsprofiles_test.go` — next CR, applying this CR's now-proven
  pattern to the largest remaining file (14 tests, ~12 convertible).
- `confirm_test.go`, `movepicker_test.go`, `sendmessage_test.go`,
  `messagefilter_test.go`, `themepicker_test.go` — don't exist; these
  5 overlays have no dedicated unit tests today (confirmed by file
  listing). Adding coverage for them is separate scope, not part of
  this test-infrastructure migration.
- The 3 true integration tests identified above
  (`TestSaveDatadogEditorRoundTrip`, and — once audited in the
  `awsprofiles` CR — `TestActivateAWSProfilePersistsAndUpdatesUI`) —
  they stay in `internal/app`, unmodified, both now and after the
  physical move (they test `App`/`Host` behavior, not the overlay).
- Fixing any other test file's raw `.visible`/`.list`/etc. access
  (`app_test.go`, `messages_test.go`, `logsearch_test.go`,
  `datadoglogs_test.go`) — those test `App`-level routing/integration
  (`:aq` opens the connection manager, `moveMarked()` opens the move
  picker) and are unaffected by this CR; they'll need their `.visible`
  reads changed to `.Visible()` when the physical move actually
  happens (already-exported, from CR 73/74), not before.
- The physical move itself, and `hosttest_test.go`'s eventual
  relocation to `internal/dialog` — later CRs.
- Any behavior change to production code. This CR only touches test
  files plus adds one new test-only file.

## Definition of done

1. `go build ./...` and `go test ./...` pass in `tui/`.
2. `testHost` implements all 20 `ui.Host` methods; `connections_test.go`
   and `timerangemodal_test.go` construct their overlay directly via
   `testHost` with zero remaining `New(config.Default())` calls;
   `datadogsettings_test.go` does the same for its 3 pure-overlay
   tests, with `TestDatadogSettingsLabel` relocated and
   `TestSaveDatadogEditorRoundTrip` left untouched.
3. `gofmt -l` reports nothing; `go vet ./...` clean.
4. No production-code behavior change — test infrastructure only.
