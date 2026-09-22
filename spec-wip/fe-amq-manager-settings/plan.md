# Implementation plan: AMQ Manager settings

## Approach

Add one typed AMQ Manager settings group to the application's configuration,
persisted alongside other global settings in `config.yaml`. Give the group
an explicit `showOnlyQueuesWithPendingMessages` field whose zero value is
false, preserving existing behavior for users whose config has no such
field. Add a save method to the app host contract so the dialog updates the
in-memory config and persists it through the existing `config.SaveDefault`
path.

Create a focused dialog under `tui/internal/dialog` following the existing
Datadog settings overlay conventions. The overlay exposes the single
boolean option, saves on explicit confirmation, and closes without saving
on cancellation. Add it to app construction, page layering, palette/theming,
and modal visibility/focus handling where applicable. Extend the Settings
view with an AMQ Manager row that opens this overlay.

Pass the setting into the Queues view through its existing host/config
access rather than introducing backend-specific behavior. During repaint,
derive visible summaries from the loaded list, applying the pending-count
predicate and existing name filter before sorting and rendering. Keep
unknown negative pending counts visible; only a count of exactly zero is
excluded. Repainting after a setting save must immediately reflect the
change without requiring a network reload.

## Files and modules

- `tui/internal/config/config.go`: add the AMQ Manager settings type and
  field to both `Config` and the `settingsFile` persisted shape.
- `tui/internal/config/config_test.go`: verify default-off behavior and
  config save/load round-tripping.
- `tui/internal/ui/host.go` and `tui/internal/app/host.go`: add and implement
  the settings persistence operation, updating the queue view after save.
- `tui/internal/dialog/amqmanagersettings.go` (new): implement the focused
  settings overlay and its save/cancel behavior.
- `tui/internal/dialog/hosttest_test.go` and new dialog tests: support and
  verify settings editing behavior.
- `tui/internal/app/app.go` and relevant app tests: construct and register
  the overlay for pages, theming, and modal visibility.
- `tui/internal/view/settings.go` and its tests: add the AMQ Manager row.
- `tui/internal/view/queues.go` and `queues_test.go`: filter visible
  summaries and verify zero, positive, and unknown counts as well as
  immediate repaint after changing the setting.
- `spec/01-repo-and-tui-shell/spec.md` and
  `spec/07-activemq-queue-list/spec.md`: update end-state behavior during
  merge-back.

## Key decisions and trade-offs

- Store the setting globally in `config.yaml`, independently of named
  broker connections, as approved in the feature spec.
- Keep backend requests unchanged because `queue.Backend.List` currently
  has no filtering parameter; filter the summaries already returned.
- Treat only `PendingCount == 0` as known empty. This preserves queues with
  unavailable statistics (`-1`) rather than incorrectly implying they have
  no pending messages.
- Use a dedicated settings type so subsequent AMQ Manager options can be
  added without changing the persistence and UI shape again.
- Keep sorting, queue-name filtering, backend loading, and the current
  default display behavior intact when the new setting is off.

## Verification

- Run focused config, dialog, settings-view, and queue-view unit tests as
  they are added, then run the full Go test suite and project formatting /
  lint commands recorded in the repository's task configuration.
- Manually open Settings → AMQ Manager, toggle the option, confirm it is
  saved across restart, and verify the queue list hides zero-count rows
  while retaining positive and unknown-count rows.
