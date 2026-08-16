# Spec — CR 54: Home screen sections, by backend instead of one flat "Apps" list

Date: 2026-08-16

## What

Replace the Home screen's single "Apps" section (6 entries) with three
backend-grouped sections, keeping "System" as-is:

1. **ActiveMQ**: `queues`
2. **AWS**: `ssm-parameters`, `secrets-manager`, `cloudwatch-logs`,
   `codepipeline`
3. **Datadog**: `datadog-logs`
4. **System** (unchanged): `settings`, `log`

Entry order within each section matches the order requested (same
relative order as today's "Apps" list for the AWS group). View names,
descriptions, hotkeys (`h`/`s`/`l`/`:`/`?`/`q`), and every view's own
behavior are unchanged — this only regroups how Home lists them.

## Why

Discussed live: the flat "Apps" list mixes three unrelated backends
(ActiveMQ, AWS, Datadog) under one label, which stops scaling as more
views get added to any one of them (e.g. AWS already has 4 of 6 "Apps"
entries). Grouping by backend makes Home's structure match how a user
actually thinks about the tool ("I want the AWS stuff" / "I want the
queue browser").

## Decisions

1. **Data-only change.** `views.SectionInfo`/`HomeView`
   (`internal/ui/views/home.go`) already render an arbitrary number of
   named sections generically — confirmed no code there special-cases
   "Apps" or a 2-section layout. The only edit is `app.go`'s
   `homeSections` literal (`New()`).
2. **No new view, no renamed view names/hotkeys.** Every `{Name:
   "queues", ...}`-style entry keeps its existing `Name`/`Description`
   exactly; only which section each lives under changes.
3. **Doc comments that reference "Home's 'Apps' section"** (in
   `ssmparams.go`, `secrets.go`, `codepipelinelist.go`, `logs.go`) get
   updated to name their new section, so they don't go stale.

## Scope

- `internal/app/app.go`: `homeSections` literal in `New()`.
- `internal/app/ssmparams.go`, `secrets.go`, `codepipelinelist.go`,
  `logs.go`: doc comments referencing the old "Apps" section name.

## Out of scope

- Any change to `internal/ui/views/home.go`'s rendering logic (already
  generic).
- Renaming/moving `datadog-logs` or `queues` themselves, or changing any
  view's own behavior.
- Persisting section collapse/expand state, custom user ordering, or any
  other Home UI feature beyond this regrouping.
