# Tasks — Retroactive workflow compliance for pre-existing specs

Plan: [2026-07-10_bugfix-retroactive-workflow-compliance-plan.md](2026-07-10_bugfix-retroactive-workflow-compliance-plan.md)

Each task below needs explicit manual approval before it is implemented.
Status is tracked here as each task completes.

1. **Annotate both existing specs** — append a one-line "Predates workflow
   update" note (pointing at the 2026-07-10 bugfix spec) to
   `2026-07-07_feat-cross-platform-taskfile.md` and
   `2026-07-07_feat-tui-app-scaffold.md`. No other content changes.
   Status: done.

2. **Retroactive plan + tasks for the Taskfile change** — write
   `2026-07-07_feat-cross-platform-taskfile-plan.md` and `-tasks.md`,
   labeled as written after the fact, documenting the approach taken and
   explicitly invoking the "genuinely untestable" carve-out (no tests to
   backfill for declarative Task config).
   Status: done.

3. **Retroactive plan + tasks for the tui scaffold change** — write
   `2026-07-07_feat-tui-app-scaffold-plan.md` and `-tasks.md`, labeled as
   written after the fact, documenting the actual design decisions as the
   plan; the tasks file marks the original implementation steps as
   done/historical and adds the not-yet-done test-backfill tasks (tasks 4
   and 5 below).
   Status: done.

4. **`tui/internal/ui/views/views_test.go`** — table-driven test that
   `NewSecrets`/`NewParams`/`NewQueues` return the expected `Name()`/
   `Title()`, plus a `placeholder.Primitive()` check (`*tview.TextView`
   with the expected `GetTitle()` and body text — `tview.Box` has no
   border getter, so title/text are checked instead of the border flag).
   Status: done.

5. **`tui/internal/app/app_test.go`** — `New()` registers all three views
   with `secrets` active by default; `switchTo` switches on a known name
   and no-ops on an unknown one; `onGlobalKey` focuses the prompt on `:`
   when unfocused, passes through other keys, and passes through
   everything when the prompt already has focus; `onPromptDone` stops the
   app on `q`/`quit`, switches view on a known name, leaves the current
   view unchanged on an unknown command, and only returns focus/clears
   text via the `defer` — matching current `app.go` logic exactly (no
   behavior changes). Note: `tview.Pages.Focus` delegates focus down to
   the front page's primitive rather than staying on `Pages` itself, so
   focus assertions compare against `a.pages.GetPage(<name>)`, not `a.pages`.
   Status: done.

6. **Verify** — `gofmt -l .`, `go vet ./...`, `go test ./...` (or
   `task test`) from `tui/`, all clean/passing.
   Status: done — `gofmt -l .` empty, `go vet ./...` clean, `go test ./...`
   passes (`internal/app`, `internal/ui/views`).
