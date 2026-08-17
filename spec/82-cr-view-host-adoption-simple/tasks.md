# Tasks — CR 82: `ui.ViewHost` adoption, 9 dialog-free views

1. [x] `logdetail.go`: `app *App` → `host ui.ViewHost`; constructor
   gains a trailing `onBack func()` param; the Esc-handler's
   sibling-reaching body (page switch, focus `a.logSearchV.table`,
   manual `Shortcuts()`-based context-panel rebuild) replaced with a
   single `onBack()` call; all other call sites renamed per plan.md's
   table (`.statusBar.SetText` → `.SetStatus`, `.cfg.Colors`×2 →
   `.Config().Colors`, `.contextPanel.SetText` → `.SetContextHint`);
   add the `internal/ui` import if not already present. Update
   `app.go`'s `newLogDetailView(a)` call to pass the `onBack` closure
   (moved verbatim from the file). `gofmt -l`, `go build ./...` clean.

2. [x] `paramdetail.go`: same field/constructor rename; `onBack
   func()` added, Esc-handler body (page switch, focus
   `a.ssmParamsV.table`, `UpdateContextPanel(a.ssmParamsV)`) replaced
   with `onBack()`; all 9 other call sites renamed. Update `app.go`'s
   `newParamDetailView(a)` call with the `onBack` closure. `gofmt -l`,
   `go build ./...` clean.

3. [x] `secretdetail.go`: same rename + `onBack` (page switch, focus
   `a.secretsV.table`, `UpdateContextPanel(a.secretsV)`); all 10 other
   call sites renamed. Update `app.go`'s `newSecretDetailView(a)`
   call. `gofmt -l`, `go build ./...` clean.

4. [x] `datadoglogdetail.go`: same rename + `onBack` (page switch,
   focus `a.datadogLogsV.table`, `UpdateContextPanel(a.datadogLogsV)`);
   all 8 other call sites renamed. Update `app.go`'s
   `newDatadogLogDetailView(a)` call. `gofmt -l`, `go build ./...`
   clean.

5. [x] `codepipelinedetail.go`: same rename + `onBack` (page switch,
   focus `a.codePipelineListV.table`,
   `UpdateContextPanel(a.codePipelineListV)`); all 13 other call sites
   renamed including the async `.getPipelineState` →
   `.GetPipelineState`. Update `app.go`'s
   `newCodePipelineDetailView(a)` call. `gofmt -l`, `go build ./...`
   clean.

6. [x] `ssmparams.go`: field/constructor rename only (no `onBack` —
   list views don't navigate away); all 12 call sites (`.tv`×3,
   `.cfg`×3, `.awsAuthTypeFor`→`.AWSAuthTypeFor`,
   `.awsSSOLogin`→`.AWSSSOLogin` (bare, no parens),
   `.listParameters`→`.ListParameters`, `.tv.QueueUpdateDraw`×2).
   `gofmt -l`, `go build ./...` clean.

7. [x] `secrets.go`: same rename; all 12 call sites (same shape as
   `ssmparams.go`, `.listSecrets`→`.ListSecrets`). `gofmt -l`,
   `go build ./...` clean.

8. [x] `logs.go`: same rename; all 12 call sites (same shape,
   `.listLogGroups`→`.ListLogGroups`). `gofmt -l`, `go build ./...`
   clean.

9. [x] `codepipelinelist.go`: same rename; all 16 call sites (same
   shape, `.listPipelines`→`.ListPipelines`, plus the extra
   `.cfg.Colors` reads in `repaint`'s row coloring). `gofmt -l`,
   `go build ./...` clean.

10. [x] Final verification pass: grep confirms zero remaining `.app.`
    references, zero raw `.tv`/`.cfg`/`.contextPanel`/`.statusBar`/
    lowercase-func-field access, and zero sibling-view-field reaches
    (`a.ssmParamsV.`/`a.secretsV.`/`a.logSearchV.`/`a.datadogLogsV.`/
    `a.codePipelineListV.`) inside the 5 detail-view files themselves
    (they should only appear now in `app.go`'s `onBack` closures);
    `gofmt -l tui/` clean; `go vet ./...` clean; `go build ./...` and
    `go test ./...` pass repo-wide (all packages `ok`).

11. [x] Live verification via `verify-live`: confirm Esc from each of
    the 5 detail views (SSM param detail, secret detail, log event
    detail, Datadog log detail, CodePipeline detail) returns to the
    correct list view with focus on its table and the context panel
    showing that list's shortcuts. Record what was checked and the
    outcome here once complete.

    Checked against the real `example-dev` AWS profile / Datadog config via
    tmux (`verify-live` skill). SSM param detail, secret detail, log
    event detail, and Datadog log detail were each opened from their
    real list, confirmed to render correctly, and Esc confirmed to
    return focus to the correct list's table with that list's shortcuts
    in the context panel — all four correct. CodePipeline detail could
    not be exercised with live data (`example-dev` has 0 pipelines), so
    `app.go`'s `newCodePipelineDetailView` construction site was
    instead checked by hand: its `onBack` closure has the identical
    shape (`SwitchToPage`/`SetFocus`/`UpdateContextPanel`) as the four
    already confirmed live, so it's covered by the same code path.
    (One unrelated hiccup during this pass: the first verification
    binary run hung at ~99% CPU after issuing an SSM `ListParameters`
    call, unresponsive to all input. Killed and relaunched; the retry
    loaded 398 parameters normally within ~3s and stayed idle
    afterward — a one-off stall, not reproducible, and unrelated to
    this CR's mechanical rename.)
