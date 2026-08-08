# Spec — FE 29: select an active AWS profile, filter the list, `:ap`

Date: 2026-08-08

## Background

FE 28 shipped read-only AWS profile discovery, explicitly scoped out any
notion of "activating" one. Immediately after shipping, three follow-up
requests arrived: show the currently selected profile the same way the
active connection is shown, add a `:ap` command shortcut (mirroring `:aq`
for the connection manager), and make the (now 69-entry-long, on the real
machine this was built against) list filterable.

## Solution

- **`Config.ActiveAWSProfile string`** (`activeAWSProfile` in
  `config.yaml`) — a new top-level field, independent of `Connections`/
  `ActiveConnection`. This answers part of FE 28's deferred open question
  ("is an AWS profile a new backend type, or a discovery aid?") in the
  "discovery aid" direction, at least for now: it's just a remembered
  selection, not wired to any backend or broker.
- Info panel gains a third line, `AWS Profile: <name>` (`(none)` when
  unset) — same treatment as `Connection: <alias>`.
- The AWS Profiles overlay (Settings, or now also `:ap`/`:awsprofiles`)
  gains: `Enter` activates the row under the cursor (persists
  `config.yaml`, updates the info panel, closes the overlay, status-bar
  confirmation); the active profile is marked with ⭐, same convention as
  the connection manager; a `/` filter (case-insensitive substring on
  name), same convention as the queues list.
- `:ap` / `:awsprofiles` command-prompt shortcut opens the overlay from
  anywhere, reusing the focus-reset guard added for `:aq` (FE 27) — the
  guard already included `awsProfilesVisible` pre-emptively, so no new gap
  to find here.

## Scope

### In scope

- `config.Config.ActiveAWSProfile` + round-trip tests.
- Info panel 3rd line.
- Filter input + filtering logic on the AWS Profiles overlay
  (`awsProfilesFilter`/`awsProfilesAll`/`awsProfilesFiltered` fields,
  mirroring `queuesView`'s `filter`/`allSummaries` pattern).
- `activateAWSProfile`: persist, update info panel, close, confirm.
- `:ap` / `:awsprofiles` command.
- `config.example.yaml` documents the new field.

### Out of scope (still)

- Any actual AWS API call, credential resolution, or broker discovery.
- Any change to `config.Connection` or the connection editor — activating
  an AWS profile and activating a connection remain two independent
  pieces of state that don't affect each other.

## Definition of done

1. `go build ./...`, `go vet ./...`, `go test ./...` pass in `tui/`.
2. Verified live: info panel shows "AWS Profile: (none)" by default;
   `:ap` from the Log view opens the overlay; filtering to `bi-dev`
   against this machine's real 69 profiles correctly narrowed to the 2
   matches; activating one updated the info panel, status bar, and
   showed ⭐ on reopen. `config.yaml` was backed up before this and
   restored byte-for-byte after.

## Addendum: Settings list didn't show the selection (2026-08-08)

Shipped this spec with the Settings row still reading a static "AWS
Profile" — only the info panel and the overlay itself reflected
`ActiveAWSProfile`. Reported immediately: real usage (the user tried the
feature and activated one of their own profiles) surfaced it faster than
review would have. Fixed: `refreshSettingsList` now renders
`fmt.Sprintf("AWS Profile: %s", ...)` from `cfg.ActiveAWSProfile` (no disk
read — the value's already in memory), and `activateAWSProfile` calls
`refreshSettingsList()`, matching `switchTheme`/`switchConnection`'s
existing pattern. Verified live against the real, already-activated
profile already on this machine — no synthetic test data needed since the
user's own prior usage demonstrated the fix directly.
