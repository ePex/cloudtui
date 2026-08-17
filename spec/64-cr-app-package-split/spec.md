# Spec — CR 64: split `internal/app` into k9s-inspired layer packages

Date: 2026-08-16

## Background

`internal/app` is 31 files (~13,700 lines) in a single flat package,
mixing four genuinely different kinds of code: the composition root
(`app.go`), generic UI chrome with no domain knowledge (`topbar.go`,
`statusbar.go`, `help.go`, `notify.go`), ~10 modal overlays (`confirm.go`,
`movepicker.go`, `sendmessage.go`, `connections.go`, `messagefilter.go`,
`timerangemodal.go`, `datadogsettings.go`, the theme/AWS-profiles
pickers), and ~16 resource-view files (`queues.go`, `messages.go`, their
detail-view counterparts, `settings.go`, `codepipelinewatch.go`, `log.go`).

[k9s](https://github.com/derailed/k9s) — the project this app's shell is
explicitly modeled on (`app.go`'s own package doc calls it "the k9s-style
shell") — splits exactly along these lines as real package boundaries,
not just file-naming convention: `internal/ui/` (generic widgets, no
resource knowledge), `internal/ui/dialog/` (generic modal dialogs —
confirm/prompt/selection, parameterized by message + callback, not by
what they're confirming), `internal/view/` (~50 files, one per resource
type, thin, built on shared `browser.go`/`table.go`), plus a
data-access/render layer (`dao`, `render`, `model`) that cloudtui already
has a rough equivalent of (`internal/queue`, `internal/awsprofile`,
`internal/awsssm`, etc.) — that part isn't the problem here.

## Problem

Unlike k9s's `ui`/`dialog` packages, cloudtui's overlays and views are
not decoupled from each other today — they reach directly into each
other's and `App`'s unexported fields, because everything sharing one
package makes that free. An audit of all 31 files
(spec/64-cr-app-package-split — see "coupling audit" below) found ~49
distinct cross-type unexported-field accesses. A straight file-move into
new packages would not compile; splitting `internal/app` for real means
first deciding, file by file, what each piece actually needs from the
others and exposing exactly that — not the whole struct.

### Coupling audit summary

| File(s) | Category | Coupling |
|---|---|---|
| `topbar.go`, `statusbar.go`, `help.go`, `notify.go` | generic chrome | **None** — no `*App` or cross-type references at all |
| `confirm.go` | modal overlay | None — fully self-contained already |
| `theme.go` | cross-cutting | Touches 19 other types by name to recolor them on theme switch — the single biggest coupling point, and it already has a gap (doesn't recolor `datadogLogsV`/`codePipelineListV`/their detail views) |
| 6 detail↔list view pairs (`message_detail.go`↔`messagesV`, etc.) | resource views | Each detail view reaches its parent list view's `.table`/`.filtered` directly (the CR 62 trampoline pattern) |
| `sendmessage.go`, `messagefilter.go`, `connections.go` | modal overlays | Reach back into the view that opened them after their action completes (`queuesV.load()`, `messagesV.table`/`.filter`/`.updateTitle()`, `queuesV.backend`) |
| `codepipelinewatch.go` | background service | A goroutine pushing updates directly into `codePipelineListV`/`codePipelineDetailV` |
| `connectionsecrets.go` | data-layer-ish | Reaches `app.secretCache`/`app.cfg` — doesn't fit "view" or "dialog" cleanly, arguably misplaced today regardless of this split |
| Remaining ~9 overlays, ~10 views | — | Only touch their own state + simple `App` leaf fields (`a.cfg`, `a.backend`, `a.tv`) or open another overlay by calling its (currently unexported) `show()` — mechanical once methods are exported |

## Solution

Four target packages, migrated in phases — each phase is its own future
CR, smallest/lowest-risk first, same approach as the CR 59–63 overlay
extraction series:

| Phase | Target | Contents | Coupling work required | Order |
|---|---|---|---|---|
| 1 | `internal/ui` (existing package — just gains files) | `topbar.go`, `statusbar.go`, `help.go` (incl. the `centered()` helper every overlay uses for sizing), `notify.go` | **None** — verified zero `*App`/cross-type references | **This CR** |
| 2 | `internal/app` (no package move) | `theme.go`: replace the 19-type hardcoded recolor with a `Themeable` interface (`ApplyPalette(config.Colors)`) each view/overlay implements, looped over via a registry slice built once in `New()` | One exported method per type (~19) — mechanical, and fixes the existing datadog/codepipeline recolor gap as a side effect. Mirrors the `focusExemptInputs`/`overlayVisible` pattern from the recent `onGlobalKey` cleanup (PR #14) — same shape of fix, same payoff. Doing this *before* phase 3 removes the worst blocker to moving overlays/views into separate packages. | Backlog |
| 3 | `internal/dialog` (new package) | `confirm.go`, `movepicker.go`, `sendmessage.go`, `connections.go`, `messagefilter.go`, `timerangemodal.go`, `datadogsettings.go`, theme picker (currently in `settings.go`), AWS profiles picker | **Corrected during CR 66's audit** (the original line below undersold this): every one of the 10 overlay types holds an `app *App` field and calls `a.rootPages`/`a.tv` directly, not just the 3 originally flagged — moving any of them to a new package needs `*App` satisfied through an interface (`internal/dialog` can't import `internal/app` back without a cycle). ~15-18 methods, task-shaped (`SwitchTheme`, `SaveConnection`, `DeleteConnection`, ...) rather than raw getters/setters. Original line, kept for history: "Export each overlay's `show`/`close` methods (mechanical, ~9 files). Three overlays (`sendmessage.go`, `messagefilter.go`, `connections.go`) need real redesign..." | **Done** — CR 66–78 (the `ui.Host` interface, `Themeable`, shared-helper promotion, accessor methods, the export pass, the `ui.TimeRange` promotion, the `testHost` double, and finally the physical move) |
| 4 | `internal/view` (new package) | `queues.go`, `messages.go`, `settings.go`, all 6 detail views, `ssmparams.go`, `secrets.go`, `logs.go`, `logsearch.go`, `datadoglogs.go`, `codepipelinelist.go`, `codepipelinewatch.go`, `log.go` | The 6 detail↔list trampolines and `codepipelinewatch.go`'s direct writes stay **intra-package** once everything lands together — no interface work needed for those. Each view calls `internal/dialog`'s now-exported `Show`/`Close` methods to open overlays — normal one-way import, no unexported access needed. | Backlog |
| 5 (optional, low priority) | TBD | `connectionsecrets.go`'s `secretBackend` | Doesn't fit view or dialog; likely belongs nearer `internal/queue` or stays in `internal/app` — a separate, smaller decision, not core to this split | Backlog |

`app.go` remains the composition root throughout (owns `App`, `New()`,
`Run()`, hotkey/prompt handling, `switchTo`/`switchTheme`/
`switchConnection`) — it's the one place allowed to import all four
packages and wire them together, same role k9s's `view/app.go` plays.

## Scope

### In scope (this CR — phase 1 only)

- Move `topbar.go`, `statusbar.go`, `help.go`, `notify.go` from
  `internal/app` to `internal/ui` (existing package).
- Export whatever `app.go` currently accesses across the new package
  boundary (constructor names, `topBar` struct fields, `centered`,
  `desktopNotify`, help-modal size constants) — full symbol list decided
  in plan.md.
- Update `app.go` and any other in-package caller to the new
  `ui.`-qualified names.
- Move the corresponding test files (`topbar_test.go`, `statusbar_test.go`,
  `help_test.go`, `notify_test.go` if they exist — confirmed in plan.md)
  alongside their source files, same package (`ui`), per `tui/CLAUDE.md`'s
  "one `_test.go` file per source file, same package" convention.

### Out of scope (this CR)

- Phases 2–5 in the table above — each becomes its own future CR, in the
  order listed, once the prior phase has landed. This spec exists so the
  next CR doesn't have to re-derive the coupling audit or re-litigate the
  target architecture.
- Any behavior change. Pure structural move — if it changes what the user
  sees or how anything responds, that's a bug in the refactor.
- `internal/dialog` / `internal/view` package creation — not needed until
  phase 3/4.

## Definition of done

1. `go build ./...` and `go test ./...` pass in `tui/`.
2. `topbar.go`, `statusbar.go`, `help.go`, `notify.go` (and their tests)
   live in `internal/ui`, removed from `internal/app`.
3. `internal/app` is four files smaller; nothing outside `internal/ui`
   and `internal/app` needed to change (these four files had no other
   callers).
4. No behavior change — the top bar, status bar, help overlay, and
   desktop notifications all still render/behave identically. Given zero
   logic changes (pure rename + export), existing unit tests are the
   verification; no live check needed beyond a quick sanity launch.
