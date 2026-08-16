# Spec — CR 63: Go standards cleanup

Date: 2026-08-16

## Background

Audited `tui/` for deviations from idiomatic Go / the conventions already
declared in `tui/CLAUDE.md`. The module is in good shape overall:
`gofmt -l` is clean, `go vet ./...` is clean, error wrapping is
consistent, there's no real package-level mutable state (the `var`s that
exist are compile-time interface assertions, `regexp.MustCompile`
patterns, or static lookup tables — none are mutated), and `go doc`-style
comment gaps only show up on trivial one-line interface-satisfying
methods (`Name()`, `Title()`, `Primitive()`) where a comment would be
noise, not signal. Two genuine, mechanical gaps remain:

1. **`interface{}` instead of `any`.** The `any` alias has been idiomatic
   Go since 1.18 (this module targets a current Go toolchain — `go
   version` here is 1.26). 26 non-comment occurrences of `interface{}`
   remain, concentrated in `internal/queue/jolokia/jolokia.go`,
   `internal/queue/proxy/proxy.go`, `internal/queue/backend.go`, and
   `internal/app/message_detail.go`.
2. **`internal/queue/jolokia/jolokia.go` is a 949-line single file**
   covering three distinct responsibilities: queue listing/metrics
   (`List`, `searchQueues`, `readMetrics`, ...), message
   browsing/parsing (`BrowseMessages`, `browseMessagesFull`,
   `parseBrowseItem`, `extractMessageID`, ...), and message mutation
   (`PurgeQueue`, `RemoveMessage`, `MoveMessage`, `MoveAllMessages`,
   `SendMessage`, ...). The sibling `internal/queue/proxy` package already
   splits this way (`proxy.go` + `filter.go`), and the app package's own
   CR 59–62 series established the precedent of splitting a large file by
   responsibility within the same package — this applies the same idea to
   `jolokia.go`.

## Problem

`interface{}` vs `any` is pure style noise that a reader has to
mentally normalize on every read. `jolokia.go` at 949 lines is the
largest file in the module by a wide margin (next largest non-test file
is 634 lines) and mixes three responsibilities that don't share much
beyond the `*Client` receiver, making it the hardest file in the module
to navigate or review a diff against.

## Solution

Two independent, mechanical changes — no behavior change in either:

1. Replace all 26 non-comment `interface{}` occurrences with `any`
   (`gofmt -r 'interface{} -> any'` handles this cleanly, verified
   against the current occurrences). Doc comments that literally say
   `interface{}` (e.g. `message_detail.go:134`) get reworded to `any` too
   for consistency.
2. Split `internal/queue/jolokia/jolokia.go` into three files by
   responsibility, same package, no signature changes:
   - `jolokia.go` — keeps `Client`, `NewClient`, `List`, `searchQueues`,
     `readMetrics`, `queueNameFromMBean`, `splitMBean`, `splitComma`,
     `setHeaders`, `execSimple` (the shared exec helper used by mutation
     methods, but small/generic enough to leave with the client core).
   - `messages.go` — `BrowseMessages`, `browseMessagesFull`,
     `browseBodies`, `browseMessagesFallback`, `parseBrowseItem`,
     `extractBrowseBody`, `extractMessageID`, plus the `searchResponse`
     JSON-shape struct they use.
   - `mutate.go` — `PurgeQueue`, `removeMatchingMessages`, `RemoveMessage`,
     `MoveMessage`, `MoveAllMessages`, `SendMessage`, plus `bulkItem` /
     `bulkResponseItem`.
   Exact grouping may shift slightly once the move is underway if a
   helper is only used by code landing in a different file than
   expected — the goal is three files organized by responsibility, not
   an exact match to this list.

## Scope

### In scope

- `any` migration: `internal/queue/jolokia/jolokia.go`,
  `internal/queue/proxy/proxy.go`, `internal/queue/backend.go`,
  `internal/app/message_detail.go` (all current `interface{}` sites).
- File split of `internal/queue/jolokia/jolokia.go` into `jolokia.go`,
  `messages.go`, `mutate.go` as described above. Pure code motion —
  same package, same exported API (`Client` and its methods), no
  functional change.

### Out of scope

- Splitting `internal/app` into multiple *packages*. It's a large
  package (31 files, ~9k lines) but tightly coupled through the shared
  unexported `App` struct and view types — splitting it would mean
  exporting a large surface area currently private, a much bigger and
  riskier change than this cleanup, and it's already being addressed
  incrementally at the file/struct level by the CR 59–62 series. Worth
  its own dedicated spec discussion later if wanted, not bundled here.
- Adding doc comments to exported methods on unexported view types
  (`Name()`, `Title()`, `Primitive()`, `Shortcuts()`, etc.) — these
  satisfy `ui.View`/`ui.Shortcuttable` and are self-explanatory
  one-liners; a comment would restate the code, not explain it.
- Any other file-size reduction (`messages.go` at 634 lines, `app.go` at
  602, `datadoglogs.go` at 462, `connections.go` at 453) — these weren't
  found to mix unrelated responsibilities the way `jolokia.go` does, and
  splitting them isn't a "Go standards" issue on its own. Flagging for a
  separate look if wanted, not part of this CR.
- Any behavior, API, or test-visible change. Both changes are pure
  syntax/organization.

## Definition of done

1. `gofmt -l .` and `go vet ./...` clean in `tui/`.
2. `go build ./...` and `go test ./...` pass, no test changes needed
   (pure rename + code motion, not behavior change).
3. Zero remaining non-comment `interface{}` occurrences in the files
   listed under "in scope".
4. `internal/queue/jolokia/jolokia.go` reduced to roughly a third of its
   current size, with `messages.go` and `mutate.go` present alongside it
   in the same package.
