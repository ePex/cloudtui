# Tasks

1. [ ] Config schema: `AWSFavorites`, `FavoriteKind`, `IsFavorite`,
   `Toggle` in `tui/internal/config/config.go`, wired into `Config`; unit
   tests in `tui/internal/config/config_test.go` (toggle on/off,
   independent kinds, independent profiles, YAML round-trip).
2. [ ] `Host.ToggleFavorite` — interface method in `tui/internal/ui/host.go`,
   implementation in `tui/internal/app/host.go` (mutate + persist,
   mirroring `SetActiveAWSProfile`), stub methods in
   `tui/internal/app/host_test.go`, `tui/internal/dialog/hosttest_test.go`,
   and `tui/internal/view/testfake_test.go`.
3. [ ] `internal/view` favorite-sort helper (package-level, reused by all
   three views below) plus its own unit test.
4. [ ] SSM Parameters view: star column, `f` toggle, favorite-first sort,
   `Shortcuts()` entry, tests.
5. [ ] Secrets Manager view: same as task 4.
6. [ ] CloudWatch Logs view: same as task 4.
7. [ ] Manual verification: real AWS profile(s) — favorite an item in
   each of the three views, confirm star + top-sort, switch AWS profile
   and confirm favorites don't leak across profiles, switch back and
   confirm persistence, confirm `config.yaml` reflects each toggle
   (record what was checked here).
8. [ ] Merge-back: update `spec/15-aws-parameter-store/spec.md`,
   `spec/16-aws-secrets-manager/spec.md`, and
   `spec/17-aws-cloudwatch-logs/spec.md` to document the favorite/star
   feature; delete `spec-wip/fe-aws-resource-favorites/`.
