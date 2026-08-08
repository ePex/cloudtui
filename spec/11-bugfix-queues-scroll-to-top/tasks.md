# Tasks — Bugfix 11: Queues list does not scroll to top on navigation

Plan: [plan.md](plan.md)

1. [x] **Reset selection in `repaint`** — add `qv.table.Select(1, 0)` at
   the end of `repaint` in `queues.go`, guarded by `GetRowCount() > 1`.
2. [x] **Reset scroll offset too** — add `qv.table.SetOffset(0, 0)` next to
   `Select(1, 0)` to clear `tview.Table`'s `trackEnd` latch, which otherwise
   scrolls a long initial list to the bottom. See plan.md addendum.
3. [x] **Regression test** — `TestQueuesViewRepaintScrollsToTopWithManyRows`
   in `queues_test.go`.

## Follow-up: same fix extended to every other table-based list view

This fix was never generalized past `queuesView` — each new list view
added since (`ssmParamsView`/FE 32, `secretsView`/FE 33) was modeled on
an earlier copy of `queuesView.repaint` that predated this bugfix, and
two long-standing views (`messagesView`/FE 07,
`repaintAWSProfiles`/FE 28) never picked it up either. All four showed
the identical symptom on a long enough list: opening/filtering scrolled
to the bottom instead of the top, because `tview.Table`'s "track end"
auto-scroll latches on during the table's first, still-empty draw and
stays latched through the repaint that follows.

Fixed by adding the same two lines (`Select(1, 0)` +
`SetOffset(0, 0)`, guarded by `GetRowCount() > 1`) to:

- `secrets.go`'s `repaint` and `ssmparams.go`'s `repaint` — found and
  fixed while building/using FE 33; see
  `spec/33-fe-aws-secrets-manager/tasks.md`'s "Follow-up bugfix: list
  didn't scroll to top on load".
- `awsprofiles.go`'s `repaintAWSProfiles` and `messages.go`'s
  `repaint` — fixed on request as a direct follow-up to the above.

Regression tests added mirroring
`TestQueuesViewRepaintScrollsToTopWithManyRows`'s
draw-empty-then-repaint-then-draw sequence against a
`tcell.SimulationScreen`: `TestSecretsViewRepaintScrollsToTopWithManyRows`,
`TestSSMParamsViewRepaintScrollsToTopWithManyRows`,
`TestRepaintAWSProfilesScrollsToTopWithManyRows`, and
`TestMessagesViewRepaintScrollsToTopWithManyRows`.

Live-verified `secrets-manager`/`ssm-parameters` against the real
account (118/396 rows). `awsProfiles`'s overlay was verified live too
(69 real profiles) — with one incident: a timing issue while driving
tmux caused an unintended `Enter` to land on the wrong row and change
the machine's real active AWS profile mid-test; caught immediately via
the top-bar profile name changing unexpectedly, corrected back to the
original profile via the same overlay, and confirmed restored in
`config.yaml`. `messagesView` was verified via its unit test only (not
re-driven live against a real broker), consistent with how
`queuesView`'s original fix was verified.
