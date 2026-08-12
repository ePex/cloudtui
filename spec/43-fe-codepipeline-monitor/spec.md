# Spec — FE 43: AWS CodePipeline monitoring with desktop notifications

Date: 2026-08-12

## Background

Long-running CodePipeline executions are hard to keep an eye on — the
user currently has to keep checking the AWS Console. AWS CodePipeline
already models exactly this as "stage transitions": `GetPipelineState`
returns each stage's `LatestExecution.Status` (InProgress / Succeeded /
Failed / Stopped / Stopping), which is precisely the "notification on
transition stages" the user asked for.

## Decisions (confirmed)

1. **CodePipeline, not CodeBuild.** Monitors pipeline-level stage
   transitions via `GetPipelineState`, not individual CodeBuild build
   phases.
2. **Opt-in per pipeline, not blanket-monitor-everything.** A new list
   view shows the user's pipelines (`ListPipelines`); picking one and
   pressing `w` starts watching it. Multiple pipelines can be watched
   at once (a simple name-keyed registry, no reason to artificially cap
   it at one). Bounded API usage — nothing is polled unless explicitly
   opted into.
3. **Desktop (OS-level) notifications, not just an in-app indicator** —
   confirmed explicitly: must work even when the user isn't looking at
   the CodePipeline view or the terminal isn't focused. Requires
   **cloudtui to still be running as a process** (it's a foreground TUI,
   not a daemon — this isn't "notify me after I quit cloudtui", it's
   "notify me while cloudtui is open but I'm doing something else").
4. **Notify on every stage transition**, not just the final outcome —
   confirmed explicitly. Each time a stage's status changes (e.g.
   "Build: Succeeded", "Deploy: InProgress"), a desktop notification
   fires. The very first poll after starting a watch establishes a
   baseline silently (nothing to "transition" from yet); only changes
   *after* that baseline notify.
5. **Watching auto-stops when the pipeline execution reaches a terminal
   state** (a stage other than the last one is skipped due to a
   dependent stage failing, or the last stage finishes) — with a final
   notification — or the user manually toggles it off with `w` again.
6. **New dependency: `github.com/gen2brain/beeep`** (cross-platform
   desktop notifications — macOS via `osascript`/`terminal-notifier`,
   Linux via D-Bus with `notify-send` fallback, Windows via WinRT COM
   with PowerShell fallback). Justified: Go's stdlib has no notification
   API, and hand-rolling three different OS integrations correctly
   (especially Windows toast notifications) is real, error-prone
   surface area a small, actively-maintained, no-cgo library already
   solves. A failed/unavailable notification (e.g. headless Linux with
   no D-Bus session) logs a warning and is otherwise silent — never
   fatal, never blocks polling.
7. **New dependency: `github.com/aws/aws-sdk-go-v2/service/codepipeline`**
   — same justification pattern as every other AWS service package
   already in this repo (ssm, secretsmanager, cloudwatchlogs): needed to
   actually call the API, no reasonable stdlib alternative.
8. **Reuses `cfg.ActiveAWSProfile`** — same credential concept already
   established for SSM/Secrets/CloudWatch Logs, including FE 36's SSO
   auto-reauth (`awsauth.WithReauth`). A background poll can therefore
   trigger a browser SSO login unexpectedly while the user is in a
   different view — consistent with how every other AWS-backed view in
   this app already behaves, not a new class of surprise.
9. **Poll interval: 20 seconds**, fixed (not user-configurable in this
   slice) — frequent enough to catch transitions promptly for a
   minutes-scale build, infrequent enough not to hammer the API.
10. **A watched pipeline's detail view live-refreshes** if it happens to
    be the currently open screen when a poll completes, in addition to
    firing the notification — so leaving it open shows real-time
    progress, and switching away still notifies.

## Scope

- `internal/awscodepipeline`: thin wrapper — `ListPipelines(ctx,
  profile) ([]Pipeline, error)`, `GetPipelineState(ctx, profile,
  pipelineName) ([]StageStatus, error)`. Credential resolution matches
  every other `awsXxx` package (`config.LoadDefaultConfig` +
  `WithSharedConfigProfile`).
- New view: pipeline list (table, filterable by name, same shape as the
  other AWS list views) → `Enter` opens a per-pipeline stage-status
  detail view. `w` toggles watching from either screen; a watched
  pipeline shows a "▶ watching" indicator in the list.
- Background watcher mechanism in `internal/app`: one goroutine per
  watched pipeline, polling on a ticker; all AWS calls, state
  diffing, and UI mutation happen inside `tv.QueueUpdateDraw` (the
  existing single-writer pattern every other view already uses for its
  background goroutine) — no new mutexes. The `profile` string is
  captured once when a watch starts and passed into the goroutine by
  value, never re-read from `a.cfg` inside it, to avoid a data race with
  the main goroutine's own reads/writes of `a.cfg` (matches how
  `datadogLogsView.search()` already captures `cfg.Datadog` once before
  spawning its own goroutine).
- Desktop notification helper, injectable the same way every AWS call
  in this app is (a func field on `App`, real implementation wired in
  `New()`, fakeable in tests) so tests never trigger a real OS
  notification.

## Out of scope

- CodeBuild-level detail (individual build phases/logs) — stage-level
  only.
- User-configurable poll interval.
- Persisting watch state across app restarts (watches are in-memory
  only, per session).
- Any pipeline mutation (retry a stage, approve a manual approval
  action, stop an execution) — read-only monitoring, matching this
  app's AWS views' existing read-only posture.
- Notification content beyond stage name + new status (no diff of what
  changed within the stage, no action-level detail).
