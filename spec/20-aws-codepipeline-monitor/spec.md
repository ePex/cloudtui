# AWS CodePipeline monitor with desktop notifications

_Condensed from spec/43 — see that folder for the incremental history._

## Purpose

Watch long-running AWS CodePipeline executions from inside cloudtui and
get desktop notifications on stage transitions, instead of repeatedly
checking the AWS Console.

## Behavior / user flow

- A top-level app (`codepipeline`), pipeline list view (table, filterable
  by name, same shape as the other AWS list views), using
  `cfg.ActiveAWSProfile` for credentials (including FE 36-style SSO
  auto-reauth — a background poll can trigger a browser SSO login
  unexpectedly while the user is in a different view, consistent with how
  every other AWS-backed view behaves).
- `Enter` opens a per-pipeline stage-status detail view.
- `w` (from either the list or the detail view) toggles watching that
  pipeline. A watched pipeline shows a "▶ watching" indicator in the
  list. Multiple pipelines can be watched at once (a simple name-keyed
  registry, no cap).
- **Monitors pipeline-level stage transitions** via `GetPipelineState`
  (not individual CodeBuild build phases) — polled every **20 seconds**,
  fixed, not user-configurable.
- The first poll after starting a watch establishes a silent baseline
  (nothing to "transition" from yet). Every subsequent poll that finds a
  stage's status changed (e.g. "Build: Succeeded", "Deploy: InProgress")
  fires a **desktop (OS-level) notification** — not just an in-app
  indicator, since it must reach the user even when they're not looking
  at the CodePipeline view or the terminal isn't focused. Requires
  cloudtui to still be running as a process (foreground TUI, not a
  daemon).
- Watching auto-stops (with a final notification) when the pipeline
  execution reaches a terminal state (last stage finishes, or a stage is
  skipped because a dependent stage failed) — or the user manually
  toggles `w` off again.
- If the watched pipeline's detail view happens to be the currently open
  screen when a poll completes, it live-refreshes in addition to firing
  the notification — so leaving it open shows real-time progress, and
  switching away still notifies.
- Read-only: no pipeline mutation (retry a stage, approve a manual
  approval, stop an execution) — matches this app's AWS views' read-only
  posture. No CodeBuild-level detail (individual build phases/logs).

## Data & config

- `internal/awscodepipeline/`: `ListPipelines(ctx, profile)
  ([]Pipeline, error)`, `GetPipelineState(ctx, profile, pipelineName)
  ([]StageStatus, error)`. Credential resolution matches every other
  `awsXxx` package (`config.LoadDefaultConfig` +
  `WithSharedConfigProfile`).
- Watch state is in-memory only, per session — not persisted across app
  restarts.
- Notification content is stage name + new status only — no diff of what
  changed within the stage, no action-level detail.

## Implementation notes

- One goroutine per watched pipeline, polling on a ticker. All AWS calls,
  state diffing, and UI mutation happen inside `tv.QueueUpdateDraw` (the
  single-writer pattern every other view's background goroutine already
  uses) — no new mutexes.
- The AWS profile string is captured once when a watch starts and passed
  into the goroutine by value, never re-read from `a.cfg` inside it — the
  same pattern `datadogLogsView.search()` uses, avoiding a data race with
  the main goroutine's own reads/writes of `a.cfg`.
- Desktop notifications go through `github.com/gen2brain/beeep`
  (cross-platform: macOS via `osascript`/`terminal-notifier`, Linux via
  D-Bus with `notify-send` fallback, Windows via WinRT COM with
  PowerShell fallback) — justified because Go's stdlib has no
  notification API and hand-rolling three OS integrations (especially
  Windows toasts) is significant, error-prone surface area.
- The notification call is injectable — a func field on `App`, real
  implementation wired in `New()`, fakeable in tests — so tests never
  trigger a real OS notification.

## Notable gotchas worth preserving

- A failed/unavailable notification (e.g. headless Linux with no D-Bus
  session) logs a warning and is otherwise silent — never fatal, never
  blocks polling.
- Background-goroutine AWS-profile capture-by-value (not re-reading
  `a.cfg` live) is a pattern worth repeating for any future background
  poller — the alternative risks a data race against the UI goroutine's
  own config reads/writes.
