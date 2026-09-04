# Tasks

1. [x] `.github/workflows/release.yml`: add `go-task/setup-task@v2`
   between `setup-go` and `goreleaser-action`, per `plan.md` step 1.

2. [x] `.goreleaser.yaml`: added the `before.hooks` entry that builds
   `mq-proxy-web` and copies its output to
   `mq-proxy-web/dist/mq-console.html`, plus the new top-level
   `release.extra_files` section publishing that file, per `plan.md`
   step 2.

   Verified locally with `goreleaser release --snapshot --clean`
   (`goreleaser`/`task` both available on this machine): the new
   `before.hooks` entry ran successfully (`task build:mq-proxy-web`
   completed, `mq-console.html` was produced) and byte-for-byte
   matches the built `index.html` — confirmed via `diff`. Goreleaser's
   own upfront config validation also passed (it got past config
   parsing into hook execution and Go binary builds without any
   complaint about the new `release.extra_files`/`before.hooks`
   syntax, which it validates before running anything). The snapshot
   run's *later* failure (Go cross-compilation running out of disk
   space: "no space left on device") is an unrelated local-machine
   disk-space constraint, not a config problem — the GitHub Actions
   runner that performs real releases has ample disk space, and this
   failure happens well after (and independent of) the hook this task
   added. Real end-to-end proof is still the next actual tagged
   release, as `plan.md` already flags.

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
