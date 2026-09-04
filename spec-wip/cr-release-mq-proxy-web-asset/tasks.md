# Tasks

1. [ ] `.github/workflows/release.yml`: add `go-task/setup-task@v2`
   between `setup-go` and `goreleaser-action`, per `plan.md` step 1.

2. [ ] `.goreleaser.yaml`: add the `before.hooks` entry that builds
   `mq-proxy-web` and copies its output to
   `mq-proxy-web/dist/mq-console.html`, plus the new top-level
   `release.extra_files` section publishing that file, per `plan.md`
   step 2. Verify locally: run `task build:mq-proxy-web && cp
   mq-proxy-web/dist/index.html mq-proxy-web/dist/mq-console.html` by
   hand and confirm the file exists and opens correctly in a browser;
   if `goreleaser` is available locally, also run `goreleaser release
   --snapshot --clean` to confirm the hook and `extra_files` config are
   both syntactically valid and produce the expected output without
   actually publishing anything.

3. [ ] Merge-back: document the release-asset publishing in
   `spec/21-amq-web-console/spec.md`'s "Using it" section (a
   non-technical user's actual path to the file is now "download
   `mq-console.html` from the latest GitHub release," not just "build
   it yourself") — plus a brief mention in
   `spec/03-architecture-and-package-layout/spec.md` isn't warranted
   here (this doesn't touch `internal/` package layout at all); confirm
   during this task whether any other spec references the old
   "build-it-yourself-only" distribution story and needs the same
   update. Note in `tasks.md`/the commit that the actual proof is the
   next real tagged release (can't be verified pre-merge without
   cutting one) — flag as a manual follow-up check once that happens,
   not a merge blocker. Delete
   `spec-wip/cr-release-mq-proxy-web-asset/`. Mark the PR ready for
   review.
