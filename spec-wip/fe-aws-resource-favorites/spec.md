# Feature: favorite/star items in SSM Parameters, Secrets Manager, and CloudWatch Logs

Date: 2026-08-25

## Problem

SSM Parameters, Secrets Manager, and CloudWatch Logs are all flat,
filterable lists of AWS resources (parameters, secrets, log groups
respectively) scoped to the currently active AWS profile
(`config.Config.ActiveAWSProfile`). On an account/profile with many
items, the ones a user cares about repeatedly (a handful of parameters,
one or two secrets, a couple of log groups) get lost in the list every
time, with only the `/` text filter to narrow things down — no way to
mark an item as "one I come back to" and have it stay easy to find.

## Feature

Add a favorite/star toggle to all three views (SSM Parameters, Secrets
Manager, CloudWatch Logs), with identical behavior across all three:

- **Toggle key: `f`**, on the currently selected row (table must have
  focus, same as the existing `r`/`/`/`j`/`k` bindings). Not currently
  bound in any of the three views. Added to each view's `Shortcuts()`
  list (`{Key: "f", Description: "favorite"}`) alongside the existing
  entries.
- **Display: a dedicated star column**, leftmost in the table, showing a
  filled star for a favorited row and blank for others.
- **Sort: favorited rows always sort above non-favorited ones**, as a
  layer on top of whatever column-sort is currently active in that view
  (name, for these three views, since none currently expose a
  multi-column sort like Queues does) — i.e., favorite status is the
  primary sort key, the existing sort is the secondary key within each
  group.
- **Persisted immediately** to `config.yaml` on every toggle, same as
  Settings' fields and `switchTheme`.

## Scoped per AWS profile, not globally

Favorites are stored **per AWS profile**, keyed by profile name — a
parameter/secret/log-group name is only meaningful within the account a
profile points at, and the same name can exist (as an unrelated resource)
under a different profile, or not exist at all. Region is *not* a
separate scoping axis: each AWS CLI profile already pins its own region
(`~/.aws/config`), and cloudtui has no independent region selector, so
"profile" alone is sufficient to disambiguate.

Each of the three item kinds (parameters, secrets, log groups) is its own
separate namespace — favoriting a parameter named `db-password` under
profile `prod` has no bearing on a secret or log group that happens to
share that name. Switching the active AWS profile (Settings → AWS
Profile, or `:ap`) changes which set of favorites each view shows, same
as it already changes which items load at all.

## Out of scope

- A dedicated "favorites only" filter/view — the star column + top-sort
  is the whole mechanism; the existing `/` text filter still searches
  across all items (favorited or not), not just favorites.
- Favoriting across other views (Queues, SSM/Secrets/Log detail views,
  Datadog Logs, CodePipeline) — this is scoped to the three list views
  named above only, for now.
- Any cross-profile favorites view or export/import of favorites.
- Cleaning up "orphaned" favorites (a favorited name that no longer
  exists in AWS, e.g. deleted or renamed) — a stale favorite just won't
  match anything in the loaded list and has no visible effect; no special
  handling needed.

## Manual verification

Since this touches real AWS API-backed views, verify manually against a
real profile (or the `verify-live`-adjacent approach used for queue
features, adapted for AWS — exact steps decided in `plan.md`/`tasks.md`):
favorite an item in each of the three views, confirm it sorts to the top
with a star, switch to a different AWS profile and confirm the star
doesn't follow (that profile's own favorites, if any, show instead),
switch back and confirm the original favorite is still there, and confirm
`config.yaml` reflects the change after each toggle.
