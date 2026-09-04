# Backlog

Ideas and follow-ups that are worth doing eventually but aren't queued up
as active work. Not a substitute for `spec-wip/` — when one of these
gets picked up, it still goes through the normal spec → plan → tasks
gate in `CLAUDE.md`.

- **In-queue message search/filter.** The message browser (spec/08)
  lists every message on a queue with no way to narrow by body/header
  content; a grep-style filter would help on queues with thousands of
  messages, similar to the search CloudWatch/Datadog Logs already offer.

- **Bulk message actions.** Purge/move/send (spec/09) act on a single
  message or the whole queue; there's no multi-select for "requeue
  these 5 poison messages" without operating on the entire queue.

- **Connection health at a glance.** Home shows queues per active
  connection, but switching to a named connection (spec/12) that's
  unreachable is a fail-and-wait; a quick reachability indicator next
  to each connection would surface that up front.

- **Bookmarks/pins for AWS resources.** SSM Parameters, Secrets
  Manager, and CloudWatch/Datadog saved queries all start from a flat
  list each session — pinning frequently-checked entries would save
  re-finding them.

- **Export search results to a file.** CloudWatch/Datadog Logs results
  and the message browser have no "save to file" action, which would
  help when pasting results into an incident doc.

## From the 2026-09-04 architectural review

Findings from a review of recent AWS-view work against Go/tview best
practices (k9s's package layout, tview's concurrency wiki, a GitLab
tview→Bubbletea migration rationale) and the independent `cloudtui-go`
reimplementation. Items being actively picked up are removed from this
list rather than checked off — see `spec-wip/` for the in-progress ones.

- **Dedup `showError`/`showStatus`/table-clear boilerplate.** The `for
  table.GetRowCount() > 1 { RemoveRow(...) }` clear-loop plus the
  single-row error/status rendering is repeated 3× per file across
  `ssmparams.go`, `secrets.go`, `logs.go`, `codepipelinelist.go`, and
  `codepipelinedetail.go` — same shape of duplication as the load/reauth
  helper being extracted now, smaller win, worth a follow-up pass.

- **Track the pre-existing `-race` findings in `datadoglogs_test.go`.**
  `go test -race ./internal/view/...` reports real races in the
  dropdown-refresh path — traced to `fakeViewHost.QueueUpdateDraw`
  (`testfake_test.go:70`) running its callback inline instead of
  serializing onto one goroutine the way the real tview
  `Application.QueueUpdateDraw` does, so a leftover background goroutine
  from one test can race the next test's direct widget access. Likely a
  test-fixture gap, not a production bug, but worth confirming and
  either fixing the fake or documenting why it's safe to leave.
