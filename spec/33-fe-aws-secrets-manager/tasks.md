# Tasks — FE 33: AWS Secrets Manager integration

Plan: [plan.md](plan.md)

Each task needs separate approval before it's implemented — see
`CLAUDE.md`.

1. [x] Add `github.com/aws/aws-sdk-go-v2/service/secretsmanager` to
   `tui/go.mod` (`go get` + `go mod tidy`); confirm `go build ./...`
   still passes with no code using it yet. `go mod tidy` prunes the
   entry again since nothing imports it yet (same as FE 28/32's
   precedent) — it lands for real once task 2's code imports the
   package.
2. [x] `tui/internal/awssecrets/awssecrets.go`: `Secret` type, `newClient`
   (empty-profile guard, else `secretsmanager.NewFromConfig` +
   `config.WithSharedConfigProfile`), `List` (paginated `ListSecrets`),
   `Reveal` (`GetSecretValue`, `AWSCURRENT` only). One small deviation
   from plan.md: `extractValue` returns an `error` too (not just
   `value, isBinary`), for the edge case where a `GetSecretValueOutput`
   has neither `SecretString` nor `SecretBinary` populated — surfaces
   as an explicit error instead of silently succeeding with an empty
   non-binary value.
3. [x] `awssecrets` tests: `buildSecrets` (nil-date handling, `*bool`/
   `*string` unwrapping, sorting) and `extractValue`
   (`SecretString`-vs-`SecretBinary` branch, plus the "neither
   populated" error case) as pure functions, per plan.md's testing
   note — `List`/`Reveal` themselves aren't unit tested (same
   precedent as `awsssm`).
4. [x] `App.listSecrets`/`App.revealSecret` fields (defaulting to
   `awssecrets.List`/`awssecrets.Reveal`) — same dependency-injection
   shape as `listParameters`/`revealParameter`.
5. [x] New view `tui/internal/app/secrets.go`: table (NAME/ROTATION/LAST
   CHANGED) + filter input (mirrors `ssmParamsView`), registered as
   `ui.View` + `ui.Shortcuttable`, added to `a.views` and Home's "Apps"
   section. Errors clearly if `cfg.ActiveAWSProfile` is empty. Filtered
   title uses `"(text)"` from the start (not `"[text]"` — see FE 32's
   `queues.go` for why). View-layer tests (`secrets_test.go`) written
   alongside it, including a render-based filtered-title test from the
   start (task 9's coverage for this file is effectively done already;
   task 9 will still cover the detail view and `prettyPrintJSON`).
6. [x] `onGlobalKey` exemption for `a.secretsV.filterInput`, matching the
   existing `a.ssmParamsV.filterInput`/`a.queuesV.filterInput` entries
   (FE 32 found this is required per-view, not covered by the overlay
   `*Visible`-flag blanket exemption). Covered by
   `TestOnGlobalKeyPassesThroughWhenSecretsFilterFocused`.
7. [x] Detail view `tui/internal/app/secretdetail.go`: metadata display,
   "(encrypted — press `r` to reveal)" until revealed, `r` triggers
   async `Reveal`; `prettyPrintJSON` helper applied to the revealed
   value; binary secrets show "(binary secret — cannot display)"; `c`
   copies the displayed (pretty-printed, if applicable) value via the
   existing `App.copyToClipboard`, gated on a value being present.
   Wired into `App` (`secretDetailV` field, `openSecretDetail`,
   `secretsV.table.SetSelectedFunc`, `"secret-detail"` page). Dedicated
   tests land in task 9, per this task's scope (implementation only) —
   `go build`/`go vet`/`go test ./...` all pass with the existing suite
   in the meantime.
8. [x] `theme.go`: secrets-manager table/filter block and secret-detail
   textview block, mirroring FE 32's `ssm-parameters`/param-detail
   blocks. `ViewColor("secrets-manager")` falls back to `Border` (same
   as `"ssm-parameters"` — neither is explicitly listed in the theme
   YAML files), so no theme-file changes were needed.
9. [x] Tests for the view layer: construction, header, filter (incl. a
   `renderedScreenText`-based filtered-title render test, added in
   task 5), the no-active-profile error path (task 5), the detail
   view's reveal/binary/copy paths (`secretdetail_test.go`), and
   `prettyPrintJSON`'s table-driven cases — all via injected
   `listSecrets`/`revealSecret`, no real AWS calls.
10. [x] `go build ./...`, `go vet ./...`, `go test ./...` — all pass.
11. [x] Manual verification per `verify-live`, against this machine's
    real active AWS profile, via tmux:
    - Home shows the new `secrets-manager` entry; navigating in loaded
      118 real secrets (`Secrets Manager (118)` title).
    - Filtering (e.g. `/rds`) narrows rows and the parenthesized
      filtered title actually renders on screen (not just `GetTitle()`)
      — the FE 32 bracket-swallowing bug does not recur here.
    - Opening a secret shows metadata (Name/ARN/Rotation Enabled/Last
      Changed) with the value masked and only `<r> reveal`/`<Esc> back`
      in the context panel.
    - `r` on a plain-string secret revealed it as-is; `r` on an
      RDS-managed secret (always JSON) revealed it **pretty-printed**
      (indented `{ "username": ..., "password": ... }`), confirming
      `prettyPrintJSON` end-to-end against a real secret.
    - `c` after reveal updated the status bar to name the secret only
      (confirmed the actual displayed text never appeared in the status
      bar) — same clipboard-mechanism caveat as FE 32: the real OS-level
      paste-buffer write can't be confirmed from a detached tmux session
      (no attached terminal to interpret the OSC 52 sequence), so that
      link relies on the `SimulationScreen`-based unit tests.
    - One secret in the real list produced `ResourceNotFoundException`
      on reveal despite appearing in the same `ListSecrets` response
      used to populate the row. Confirmed via the log fix below: the
      actual cause is `"can't find the specified secret value for
      staging label: AWSCURRENT"` — this specific secret has no
      `AWSCURRENT`-staged version (e.g. an orphaned/mid-rotation
      secret), an AWS-side data state issue, not a cloudtui bug —
      `Reveal` passes through exactly the `Name` AWS itself returned,
      and the error renders correctly in the status bar (and now the
      log — see below) rather than crashing or hanging.
    - No binary (`SecretBinary`-only) secret was found in the real
      account to exercise end-to-end; that path is covered by
      `extractValue`'s unit tests and
      `TestSecretDetailViewRenderShowsBinaryMessageAfterReveal`.
    - No secret values were pasted into any commit message, spec file,
      or this summary — only confirming the mechanism works.

## Bugfix found during live verification: reveal errors weren't logged

The `ResourceNotFoundException` above surfaced in the status bar but
never reached `~/.cloudtui/cloudtui.log` / the Log view — every other
error path in `internal/app` calls `slog.Error(...)` alongside the
user-facing message (e.g. `queues.go`'s `load()`), but the four
AWS-reveal/list error paths never picked up that convention. Fixed by
adding `slog.Error(...)` immediately before the existing user-facing
error handling in:

- `secretdetail.go`'s `reveal()` (this feature — the one actually
  reported).
- `secrets.go`'s `load()` (this feature, same gap).
- `ssmparams.go`'s `load()` and `paramdetail.go`'s `reveal()` — FE 32,
  already shipped on `main`; same gap, same fix, included here since
  it's the identical bug in the sibling AWS feature this one was
  modeled on.

Live-verified: reproduced the same `ResourceNotFoundException` again
and confirmed `level=ERROR msg="secret detail: failed to reveal
secret"` (with `name` and full `error` fields) now appears in both
`~/.cloudtui/cloudtui.log` and the in-app Log view. Not covered by a
new unit test — none of the 15 existing `slog.Error` call sites in
`internal/app` have test coverage either (the error branch lives
inside a goroutine + `QueueUpdateDraw` closure that established
precedent already treats as verified live rather than in a unit test;
see `ssmParamsView`/`queuesView`'s tests, which only ever exercise
`repaint()`/`showError()` directly).

## Follow-up UX change: copy shouldn't require reveal first

User feedback: `c` only appeared once a value was already displayed —
meaning copying a secret always required first showing it on screen via
`r`, even though the point of `c` is often to avoid ever looking at the
value directly. Clarified via two quick questions: `c` should be
available the instant the detail view opens, and pressing it before
`r` should fetch-and-copy silently, never displaying the value — closer
to how a password manager's "copy" button works. Applied to both this
feature and FE 32's `paramdetail.go` (identical shape for a
`SecureString` parameter) — see that spec's `tasks.md` for the pointer.

Implementation: split `revealed` (rendered on screen) from `fetched`
(the value has been retrieved at all) on `secretDetailView`;
`Shortcuts()`/the input-capture gate now check the right one for each
key. Both `r` and `c` funnel through `fetchThen`, which skips the
network call entirely if already `fetched`. Extracted
`handleFetchResult` from the goroutine body specifically so this new
behavior — the actual "silent fetch, no display" path — could be
unit-tested directly (`TestHandleFetchResult`'s three subtests), rather
than only testing the parts reachable via the already-fetched fast path
(as `secretdetail_test.go` did before). `go build`/`go vet`/`go test
./...` all pass with the updated suite.

Live-verified via tmux: opening a secret shows both `<r> reveal` and
`<c> copy value` immediately; pressing `c` on an unfetched secret
copied it (status bar confirmed) while the screen stayed on "(encrypted
— press 'r' to reveal)"; a subsequent `r` displayed the value instantly
(no fetch delay), confirming the cached value was reused rather than
re-fetched, and the context panel correctly dropped `<r> reveal`
afterward. Also incidentally re-confirmed the earlier
`ResourceNotFoundException` secret's error path still works when
triggered via `c` instead of `r`.

## Follow-up bugfix: list didn't scroll to top on load

User feedback: opening `secrets-manager` (and separately,
`ssm-parameters`) landed the table scrolled to the *bottom* of the list
rather than the top, on an account with enough rows (118 secrets, 396
parameters) for it to be visible. This is the exact bug already fixed
for `queuesView` in `spec/11-bugfix-queues-scroll-to-top`: `tview.Table`'s
"track end" auto-scroll (meant for tailing logs) latches on during the
table's first, still-empty draw (before the async load completes) and
stays latched through the repaint that follows. `queues.go`'s `repaint`
already carries the fix (`Select(1, 0)` + `SetOffset(0, 0)`, the latter
needed to actually clear the latched flag) — `secrets.go`'s and
`ssmparams.go`'s `repaint` never picked it up when they were modeled on
an earlier version of `queuesView`. Fixed by adding the identical two
lines to both. Regression tests added:
`TestSecretsViewRepaintScrollsToTopWithManyRows` and
`TestSSMParamsViewRepaintScrollsToTopWithManyRows`, both mirroring
`TestQueuesViewRepaintScrollsToTopWithManyRows`'s
draw-empty-then-repaint-then-draw sequence against a `tcell.
SimulationScreen`. Live-verified via tmux against the real account:
both lists now open scrolled to the top (first alphabetical entry
visible) instead of the bottom.

Noted but not fixed (out of the scope the user asked for): `awsprofiles.go`'s
`repaintAWSProfiles` and `messages.go`'s message-list repaint have the
same missing two lines and would show the same symptom on a long enough
list. Flagged for a future pass rather than fixed here as a drive-by
change.
