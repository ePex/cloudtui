# Spec — CR 79: split view-wiring trampolines out of the paired view/detail files

Date: 2026-08-17

## Background

`spec/64`'s phase 4 (`internal/view`, moving all 12 resource views +
their detail-view companions out of `internal/app`) is next after
phase 3 closed with CR 78. Before designing a `ui.Host`-equivalent
interface for views (the phase 3 playbook started with CR 66/67's
interface design), audited all 17 files in scope — the same rigor CR
66's audit applied before phase 3's interface design.

The audit's headline finding: **file boundaries don't align with
package boundaries** for 8 of the 17 files. Each mixes two genuinely
different things:

1. A view type + its own methods — movable to `internal/view`.
2. One or two `(a *App) openX(...)`/`wireXOpensY()` methods — the
   "trampolines" that wire one view to open another (e.g. Enter on a
   queues-table row opens the messages view for that queue). These
   take `*App` as receiver and reach directly into the *other* view's
   unexported fields/methods — they can't move to `internal/view` at
   all, and they're exactly what determines what shape a `Host`-style
   interface for views would even need to cover.

Verified directly (not just from the audit) — every trampoline pair,
by file and exact signature:

| File (view type stays) | Trampoline methods (must move) | Reaches into |
|---|---|---|
| `messages.go` | `openMessages`, `wireQueuesOpensMessages` | `a.queuesV.table` |
| `message_detail.go` | `openMessageDetail`, `wireMessagesOpensDetail` | `a.messagesV.table`/`.msgs`/`.queueName`/`.Shortcuts()`/`.load()` |
| `paramdetail.go` | `openParamDetail`, `wireSSMParamsOpensDetail` | `a.ssmParamsV.table`/`.filtered` |
| `secretdetail.go` | `openSecretDetail`, `wireSecretsOpensDetail` | `a.secretsV.table`/`.filtered` |
| `logsearch.go` | `openLogSearch`, `wireLogsOpensSearch` | `a.logsV.table`/`.filtered` |
| `logdetail.go` | `openLogEventDetail`, `wireLogSearchOpensEventDetail` | `a.logSearchV.table`/`.results`/`.Shortcuts()` |
| `datadoglogdetail.go` | `openDatadogLogDetail`, `wireDatadogLogsOpensDetail` | `a.datadogLogsV.table`/`.results` |
| `codepipelinedetail.go` | `openCodePipelineDetail`, `wireCodePipelineListOpensDetail` | `a.codePipelineListV.table`/`.filtered` |

16 methods total, verified via `grep -n '^func (a \*App)'` against each
file directly.

## Problem

Nothing forces these 16 methods to live in the same file as the view
type they're paired with — that's just where they were originally
written. Splitting them out is unblocked, mechanical, same-package
work (Go doesn't care which file within a package declares a symbol),
and doing it now — before any interface design — means the interface
design CR starts from "here are the 8 files with just view types, and
here's the wiring layer that already exists as its own concern,"
rather than having to first untangle the same 8 files while also
designing the interface.

## Solution

Move all 16 methods into a new `internal/app/viewwiring.go`, grouped
by the pair they came from (same order as the table above), each pair
keeping its existing doc comments untouched. Nothing else changes —
same method bodies, same receiver, same file-of-origin's imports
follow only if still needed there (most of `strings`/`fmt`/etc. stay
needed in the origin file for the view type's own code; `viewwiring.go`
gets whatever *it* needs, worked out per-file during implementation).

Not in scope for this CR (re-confirmed during the audit, worth stating
explicitly since they could look similar at a glance):

- **`settings.go`'s `refreshSettingsList`** — not a cross-view
  trampoline (it doesn't wire one view to open another); it rebuilds
  `settingsView`'s own list from config. `settingsView` has a deeper,
  different pre-existing wrinkle instead — its real logic already
  lives almost entirely in `(a *App)`-scoped code
  (`newSettingsView`/`refreshSettingsList`), with the `settingsView`
  struct itself just a thin `Name()`/`Title()`/`Primitive()` facade,
  and `a.settingsList`/`settingsView.list` are the same `*tview.List`
  stored in two places. Worth its own look when phase 4 actually
  reaches Settings, not bundled into this mechanical split.
- **`codepipelinewatch.go`** — entirely `(a *App)` methods already, no
  view type of its own to split away from. It stays in `internal/app`
  as a whole file; its direct writes into `codePipelineDetailV`/
  `codePipelineListV`'s fields are a problem for the future
  accessor-methods CR (mirroring CR 73), not this one.

## Scope

### In scope

- New `internal/app/viewwiring.go`: the 16 trampoline methods, moved
  verbatim (bodies unchanged) from the 8 files listed above.
- The 8 origin files: the 16 methods removed; each file's own imports
  trimmed if anything became unused as a result.
- `gofmt`/`go vet`/`go build`/`go test` clean across `tui/`.

### Out of scope

- Any interface design, any `Open*`/`Show*` method signatures on a
  future `Host`-equivalent — this CR doesn't design or touch the
  target shape those trampolines will eventually take; it only
  relocates their current form.
- `settings.go`, `codepipelinewatch.go` — see Solution above.
- The remaining 16 files' own `*App`-coupling (the 12 view types'
  `app *App` fields, the ~35-40 candidate interface methods, the
  ~211 test functions across 17 files) — future CRs, once this split
  lands and the actual interface design starts.
- Any behavior change. Pure file reorganization within one package —
  `go test ./...` passing is sufficient, no live verification needed
  (nothing here changes what runs, only which file declares it).

## Definition of done

1. `go build ./...` and `go test ./...` pass in `tui/`.
2. All 16 trampoline methods live in `internal/app/viewwiring.go`; the
   8 origin files no longer declare any `(a *App)` method.
3. `gofmt -l` reports nothing; `go vet ./...` clean.
4. No behavior change — pure move within `internal/app`.
