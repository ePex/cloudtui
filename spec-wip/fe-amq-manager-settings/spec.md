# AMQ Manager settings

Date: 2026-09-23

## Purpose

Provide a place for settings that control the AMQ queue manager, starting
with an option to show only queues that have pending messages. Keep this
settings area extensible so additional AMQ manager options can be added
without adding unrelated rows to the global Settings screen.

## Behavior

- Global Settings gains an **AMQ Manager** row. Selecting it opens a
  centered settings overlay for the AMQ queue manager.
- The overlay initially contains **Show only queues with pending
  messages**, an on/off setting. It is off by default, preserving the
  current queue list behavior.
- When enabled, the Queues view omits queues whose pending count is known
  to be zero. Queues with a positive pending count remain visible. If the
  backend reports an unknown count (`-1`, as it can when the ActiveMQ
  Statistics Plugin is unavailable), the queue remains visible because
  it cannot be established that the count is zero.
- The setting affects the displayed queue rows after the normal backend
  list request. Both the setting and existing queue-name filter apply;
  sorting applies to the resulting rows. Reloading or re-entering the
  Queues view uses the same setting.
- The setting is global to the AMQ Manager, not specific to the selected
  broker connection, and persists across application restarts in
  `~/.cloudtui/config.yaml`.
- The AMQ Manager overlay is structured to accommodate additional
  manager settings later. This feature adds only the pending-message
  filter.

## Scope

- Add the AMQ Manager entry to global Settings and a settings overlay
  containing the pending-message visibility toggle.
- Persist the toggle and apply it consistently in the Queues view.
- Update the living queue-list and shell/settings specifications after
  implementation.

## Out of scope

- Filtering queues at the broker or backend request level; the current
  backend interface lists all queue summaries.
- Per-connection values, additional settings, or changes to how pending
  counts are fetched or displayed.
