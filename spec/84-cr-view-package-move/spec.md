# Spec — CR 84: move the 14 `ui.ViewHost`-adopted views into `internal/view`

Date: 2026-08-17

## Background

Phase 4's package split reaches its last step. CR 79/80/81 built the
`ui.ViewHost` interface and callback-injection machinery; CR 82/83
converted all 14 resource-view types (`ssmparams.go`, `paramdetail.go`,
`secrets.go`, `secretdetail.go`, `logs.go`, `logdetail.go`,
`datadoglogdetail.go`, `codepipelinelist.go`, `codepipelinedetail.go`,
`queues.go`, `messages.go`, `message_detail.go`, `logsearch.go`,
`datadoglogs.go`) to depend on `ui.ViewHost` and direct `*dialog.X`
params instead of `*App`. `settings.go`, `log.go` (the app's own
internal log tailer — distinct from `logs.go`/`logsearch.go`'s AWS
CloudWatch views), and `codepipelinewatch.go` were explicitly scoped
*out* of that adoption back in CR 79 (see that spec's Solution section)
and stay in `internal/app`.

This CR does the actual move: relocate those 14 files (plus their 14
`_test.go` files) into a new `internal/view` package. A fresh audit
(not reused from CR 79's, which predates CR 82/83) surfaced two
distinct blockers, not one:

**Blocker 1 — non-moving `internal/app` code still reaches into the
14 files' unexported state.** Even though the 14 files no longer
reach *out* into `*App` (CR 82/83 closed that direction), `app.go`,
`host.go`, `viewwiring.go`, and `codepipelinewatch.go` still reach
*in*:

- `app.go`'s `focusExemptInputs` slice literal reads 10 raw filter-
  input fields across 8 types (`a.queuesV.filterInput`,
  `a.messagesV.searchInput`, `a.ssmParamsV.filterInput`,
  `a.secretsV.filterInput`, `a.logsV.filterInput`,
  `a.logSearchV.patternInput`, `a.datadogLogsV.queryInput`,
  `a.datadogLogsV.serviceFilterDD`, `a.datadogLogsV.envFilterDD`,
  `a.codePipelineListV.filterInput`).
- `app.go`'s `AddPage` registration reads 8 raw root-primitive fields
  (`.flex`/`.textView`/`.table`) on the 8 types that aren't registered
  `ui.View`s (`messagesV`, `messageDetailV`, `secretDetailV`,
  `logSearchV`, `logDetailV`, `datadogLogDetailV`,
  `codePipelineDetailV`, `paramDetailV`) — they have no `Primitive()`
  today because that's currently a `ui.View`-only method.
- `app.go`'s initial construction/focus-wiring loop and `switchConnection`
  read 8 raw `.table` fields directly, and write
  `a.queuesV.backend = a.backend` directly.
- `viewwiring.go`'s 8 `Open*` trampolines reach `messagesV.{queueName,
  filter, quickSearch, searchInput, updateTitle, setHeader, load}`,
  `messageDetailV.{render, textView}`, `paramDetailV.{render,
  textView}`, `secretDetailV.{render, textView}`, `logSearchV.{open}`,
  `logDetailV.{render}`, `datadogLogDetailV.{render}`,
  `codePipelineDetailV.{open}`.
- `host.go`'s `ui.Host` implementation (`ReloadAfterSend`,
  `MessagesFilter`, `ApplyMessagesFilter`, `FocusMessages`) reaches
  `queuesV.{backend, load}` and `messagesV.{queueName, load, filter,
  updateTitle, table}`.
- `codepipelinewatch.go` (the background watcher) reaches
  `codePipelineDetailV.{pipelineName, render}` and
  `codePipelineListV.{all, repaint}`, **and** calls `statusLabel(...)`
  — an unexported top-level function defined in `codepipelinedetail.go`
  (one of the 14 files), not a method at all.

None of this is new coupling — it's the same reach-ins that were
always fine inside one package. It only becomes a problem once the 14
files are genuinely in a different package.

**Blocker 2 — every one of the 14 files' tests constructs the real
`*App`.** All 14 `_test.go` files call `New(config.Default())` — 161
call sites total (4–22 per file) — because that was always the
easiest way to get a fully-wired view with real dialogs/config
attached. Once `internal/app` imports `internal/view` (to hold
`*view.QueuesView` etc. as struct fields), `internal/view`'s test
files can no longer import `internal/app` back — a hard cycle,
regardless of test-file package naming (this repo's convention is
same-package tests per `tui/CLAUDE.md`, so there's no `_test`-suffix
escape hatch either).

Looking at what these tests actually need `New()` *for* clarifies the
fix: the large majority only use it to get a valid `ui.ViewHost` plus
some already-wired `*dialog.X` instances — they don't test cross-view
navigation outcomes. Since `*dialog.ConfirmDialog` etc. only need a
`ui.Host` (not `*App`) to construct (`dialog.NewConfirmDialog(host
ui.Host)`), and `ui.ViewHost`/`ui.Host` are already fully
`internal/app`-independent (confirmed: neither interface file imports
`internal/app` or `internal/dialog`), a small `ui.ViewHost`-
implementing test fake plus real `*dialog.X` instances built on top of
it covers nearly every one of the 161 sites with **no** `internal/app`
import at all.

The exception is the small number of tests that genuinely verify
cross-view *outcomes* driven by real `app.go` wiring — e.g.
`TestParamDetailViewEscReturnsToSSMParameters` currently drives Esc
and asserts `a.pages.GetFrontPage() == "ssm-parameters"`. Once `onBack`
is a plain injected closure (already true since CR 82/83), the view-
level version of this test only needs to assert "Esc calls the
injected `onBack` closure" — a spy, not the real app. What *is* still
worth testing against the real, fully-wired app is that `app.go`
itself supplies the *correct* closure to each constructor — that
belongs in `internal/app` as a lighter set of wiring-integration
tests, not duplicated per view.

**User confirmed** (via `AskUserQuestion`) tackling all of this — the
exports, the test-fake design, the 161 rewrites, the physical move,
and the wiring-test relocation — in one CR rather than splitting it,
since none of the pieces are independently mergeable (the move can't
land without the exports; the exports are only worth doing as part of
the move).

## Problem

`internal/view` cannot exist as a real, independently-compilable
package while (a) `internal/app` still reaches into its types'
unexported fields/methods, and (b) its own tests can only be written
by importing `internal/app`, which will itself depend on
`internal/view`.

## Solution

### Part A — export the ~15 blocked symbols

For each type, replace the raw field/method that non-moving code
touches with an exported equivalent, choosing the narrowest shape that
serves the actual call site (an accessor method, not a public field,
matching this codebase's existing convention — e.g. `ui.View`'s own
`Primitive()`):

| Type | Old (unexported) | New (exported) |
|---|---|---|
| `queuesView` | `.filterInput` (focus-exempt list) | `FilterInputs() []tview.Primitive` |
| `queuesView` | `.table` (chrome wiring, onBack targets) | `Table() *tview.Table` |
| `queuesView` | `.backend = x` (direct field write) | `SetBackend(b queue.Backend)` |
| `queuesView` | `.load()` | `Load()` |
| `messagesView` | `.searchInput`, `.table` | `FilterInputs() []tview.Primitive`, `Table() *tview.Table` |
| `messagesView` | `.flex` (AddPage) | `Primitive() tview.Primitive` |
| `messagesView` | `.queueName`, `.filter`, `.quickSearch`, `.searchInput.SetText("")`, `.updateTitle()`, `.setHeader()`, `.load()` (all of `OpenMessages`'s body) | `Open(queueName string)` — folds the entire reset-and-load sequence into one call |
| `messagesView` | `.queueName` (read, `ReloadAfterSend`) | `QueueName() string` |
| `messagesView` | `.filter` (read, `MessagesFilter`) | `Filter() queue.MessageFilter` |
| `messagesView` | `.filter =`, `.updateTitle()`, `.load()` (`ApplyMessagesFilter`'s body) | `ApplyFilter(f queue.MessageFilter)` |
| `messageDetailView` | `.textView` (AddPage) | `Primitive() tview.Primitive` |
| `messageDetailView` | `.render(...)` + `.textView.SetTitle(...)` | `Render(queueName string, msg queue.Message)` — folds the title-set in, since `Render` already receives `queueName` |
| `paramDetailView` | `.textView` (AddPage) | `Primitive() tview.Primitive` |
| `paramDetailView` | `.render(param)` + `.textView.SetTitle(...)` | `Render(param awsssm.Parameter)` — folds title-set in |
| `secretDetailView` | `.textView` (AddPage) | `Primitive() tview.Primitive` |
| `secretDetailView` | `.render(secret)` + `.textView.SetTitle(...)` | `Render(secret awssecrets.Secret)` — folds title-set in |
| `logSearchView` | `.filterInput`→`.patternInput`, `.table` | `FilterInputs() []tview.Primitive`, `Table() *tview.Table` |
| `logSearchView` | `.flex` (AddPage) | `Primitive() tview.Primitive` |
| `logSearchView` | `.open(logGroupName, pattern)` | `Open(logGroupName, pattern string)` (already sets its own title internally — pure rename) |
| `logDetailView` | `.textView` (AddPage) | `Primitive() tview.Primitive` |
| `logDetailView` | `.render(event)` | `Render(event awslogs.LogEvent)` (no title-fold needed — its title is static) |
| `datadogLogsView` | `.queryInput`, `.serviceFilterDD`, `.envFilterDD`, `.table` | `FilterInputs() []tview.Primitive`, `Table() *tview.Table` |
| `datadogLogDetailView` | `.textView` (AddPage) | `Primitive() tview.Primitive` |
| `datadogLogDetailView` | `.render(event)` | `Render(event datadoglogs.LogEvent)` (no title-fold needed) |
| `codePipelineListView` | `.filterInput`, `.table` | `FilterInputs() []tview.Primitive`, `Table() *tview.Table` |
| `codePipelineListView` | `.all`, `.repaint(.all)` (`codepipelinewatch.go`) | `Repaint()` — repaints from its own cached state, no field exposed |
| `codePipelineDetailView` | `.table` (AddPage) | already `ui.View`-registered? — **no**, confirmed not registered; add `Primitive() tview.Primitive` |
| `codePipelineDetailView` | `.pipelineName` (read) | `PipelineName() string` |
| `codePipelineDetailView` | `.render(stages)` | `Render(stages []awscodepipeline.StageStatus)` |
| `codePipelineDetailView` | `.open(pipelineName)` | `Open(pipelineName string)` (already sets its own title — pure rename) |
| `codepipelinedetail.go` | `statusLabel(status)` (top-level func) | `StatusLabel(status string) string` |

`ssmParamsView`/`secretsView`/`logsView` need only `FilterInputs()` +
`Table()` (both already registered `ui.View`s with `Primitive()`, so
no separate accessor needed there) — no `Open`/`Render` folding, since
nothing outside reaches into their render path (`OpenParamDetail`
etc. render a *different* type, the detail view).

`FilterInputs() []tview.Primitive` (plural) rather than a single-value
method: `datadogLogsView` has 3 focus-exempt inputs, not 1 — one
consistent method shape across all 8 types beats a singular method on
7 of them plus a special case on the 8th.

### Part B — break the test import cycle

Add a small `ui.ViewHost`-implementing fake to `internal/view`'s test
files — a struct holding a `config.Config`, a stub `queue.Backend`,
recorded-call fields (which page was switched to, what text was set
as status/context hint, which `Open*` was called with what args), and
injectable func fields for the 12 data-fetcher methods (so a test can
supply canned parameters/secrets/log events/etc. without a real AWS
call) — the same "inject a func field, override per-test" shape this
codebase already uses for `App.listParameters` and friends pre-CR-80.
Real `*dialog.X` instances get constructed on top of this fake host
where a view's constructor needs one (dialogs only need `ui.Host`, so
this is free).

Every one of the 161 `New(config.Default())` call sites that doesn't
depend on genuine cross-view behavior gets rewritten against this
fake. The small number that do (Esc-back landing on the right page
with the right table focused and the right context-panel content,
`Open*` trampolines wiring the right view) move to a new
`internal/app` integration-test file, driven against the real
`New(config.Default())`, that exists specifically to verify app.go's
wiring — one test per `Open*`/`onBack` pair rather than duplicated
per-view.

`fakeQueueBackend` (defined in `queues_test.go`, one of the 14) is
also used today by `connectionsecrets_test.go`, which is **not**
moving — this repo has no shared test-fixtures package, so the
pragmatic fix is a small, independent duplicate in whichever of the
two locations doesn't already have the original (it's ~10 trivial
stub methods, cheaper to duplicate than to introduce a new shared
package for).

### Part C — the physical move

1. `internal/view/` created; the 14 production files moved in,
   `package app` → `package view`.
2. Every one of the 14 types renamed to its exported form
   (`queuesView` → `QueuesView`, `paramDetailView` → `ParamDetailView`,
   etc.); every constructor `newXView` → `NewXView`. Field names
   inside each struct are unaffected (still unexported — same-package
   access among the 14 files is unchanged, per the audit's point 3:
   only `logEventPreview` is shared across two of the 14 files, and
   that stays as-is, package-private within `internal/view`).
3. The 14 `_test.go` files move alongside, rewritten per Part B.
4. `internal/app`'s struct fields (`queuesV *queuesView`, etc.)
   retyped to `*view.QueuesView` etc.; every `newXView(...)` call site
   in `app.go` becomes `view.NewXView(...)`.
5. `app.go`, `host.go`, `viewwiring.go`, `codepipelinewatch.go`
   rewritten to use the Part A exports instead of raw field access.
6. New `internal/app` integration-test file per Part B's tail end.

## Scope

### In scope

- The 14 production files + their exports (Part A).
- The `internal/view` test fake + all 161 test-call-site rewrites
  (Part B).
- The physical move and rename (Part C).
- `app.go`/`host.go`/`viewwiring.go`/`codepipelinewatch.go` updated to
  the new exported surface.
- New `internal/app`-level wiring-integration tests for the relocated
  cross-view assertions.
- `gofmt`/`go vet`/`go build`/`go test` clean across `tui/`.
- Live verification covering the full app (every moved view's normal
  operation, not just the parts this CR touches — this is the biggest
  structural change so far in phase 4).

### Out of scope

- `settings.go`, `log.go`, `codepipelinewatch.go` themselves (only its
  reach-ins into the 14 files change) — still `internal/app`, per
  CR 79's original scoping.
- Any further interface redesign of `ui.ViewHost`/`ui.Host` — Part A's
  additions are all on the concrete view types, not the interfaces.
- Any behavior change — every export in Part A wraps the exact
  existing logic (folding a title-set into `Render` changes *where*
  the line runs, not what it does); the test-fake rewrite changes
  *how* a view gets constructed in a test, not what's asserted, except
  where a test's assertion itself moves to `internal/app` (still the
  same assertion, different file).
- `queueColumns`'s package-level-var status (pre-existing, moves
  as-is — not this CR's job to also fix).

## Definition of done

1. `internal/view` exists, holds all 14 view types (exported) + their
   tests; `internal/app` no longer has any of the 14 files.
2. `go build ./...` and `go test ./...` pass in `tui/`; zero import
   cycle.
3. `gofmt -l` reports nothing; `go vet ./...` clean.
4. `app.go`/`host.go`/`viewwiring.go`/`codepipelinewatch.go` use only
   exported `view.X` methods — zero raw field/unexported-method access
   into any of the 14 types.
5. New `internal/app` wiring-integration tests cover every `Open*`
   trampoline and every `onBack` closure's actual navigation outcome.
6. Live verification: full pass through every moved view's normal
   operation (list/filter/detail/back for all 9 resource kinds, plus
   the dialog flows CR 83 already covers) confirms no behavior change.
7. No behavior change.
