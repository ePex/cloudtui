# Tasks: AMQ Manager settings

1. [ ] Add the AMQ Manager settings type to configuration, persist it in
   `config.yaml`, and cover default-off plus save/load behavior with unit
   tests.
2. [ ] Add the AMQ Manager settings overlay, including save/cancel behavior,
   and unit tests for its interaction with the host.
3. [ ] Wire the overlay into the app's page, palette, visibility, and focus
   handling; add an AMQ Manager row to Settings and cover the wiring and row
   behavior with unit tests.
4. [ ] Apply the setting in the Queues view and test enabled/disabled
   behavior for zero, positive, and unknown pending counts, interaction with
   name filtering and sorting, and immediate repaint after saving.
5. [ ] Update the living shell/settings and ActiveMQ queue-list specs, run
   the full verification commands, and manually verify the settings flow
   and persisted queue filtering.
