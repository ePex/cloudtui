# spec

The living documentation of cloudtui: one folder per **feature area**,
each holding a single `spec.md` that describes the **current, end-state
behavior** of that area — detailed enough that if all code were deleted,
the application could be rebuilt from these 20 documents alone.

This is not an incremental log. A doc describes what's true *today*; when
a feature area's behavior changes, its `spec.md` is updated in place
rather than gaining a new dated entry. Bugfixes and change requests don't
get their own folder here — they're folded into the area they correct.

Numbering is a 1–21 reading order (foundational/infra first), not a
chronological counter — see the table below.

| Folder | Covers |
|---|---|
| [01-repo-and-tui-shell](01-repo-and-tui-shell/spec.md) | Repo foundations and TUI shell |
| [02-ci-and-release](02-ci-and-release/spec.md) | CI and release |
| [03-architecture-and-package-layout](03-architecture-and-package-layout/spec.md) | Package layout and internal architecture |
| [04-theming](04-theming/spec.md) | Theming |
| [05-home-navigation](05-home-navigation/spec.md) | Home screen and global navigation legend |
| [06-logging](06-logging/spec.md) | Debug logging |
| [07-activemq-queue-list](07-activemq-queue-list/spec.md) | ActiveMQ queue list |
| [08-message-browser-and-detail](08-message-browser-and-detail/spec.md) | Message browser and detail view |
| [09-queue-message-actions](09-queue-message-actions/spec.md) | Queue and message actions: purge, move, send |
| [10-mq-proxy-service](10-mq-proxy-service/spec.md) | mq-proxy: standalone ActiveMQ REST proxy service |
| [11-mq-proxy-backend-integration](11-mq-proxy-backend-integration/spec.md) | mq-proxy wire contract and the TUI's proxy backend |
| [12-named-connections](12-named-connections/spec.md) | Named connections |
| [13-dev-verification-tooling](13-dev-verification-tooling/spec.md) | Developer verification tooling |
| [14-aws-profiles](14-aws-profiles/spec.md) | AWS profile discovery, selection, and SSO re-authentication |
| [15-aws-parameter-store](15-aws-parameter-store/spec.md) | AWS Systems Manager Parameter Store browser |
| [16-aws-secrets-manager](16-aws-secrets-manager/spec.md) | AWS Secrets Manager browser |
| [17-aws-cloudwatch-logs](17-aws-cloudwatch-logs/spec.md) | AWS CloudWatch Logs search |
| [18-datadog-logs](18-datadog-logs/spec.md) | Datadog Logs search |
| [19-log-investigation-crosslinks](19-log-investigation-crosslinks/spec.md) | Log investigation cross-links: correlation jump + shared time-range modal |
| [20-aws-codepipeline-monitor](20-aws-codepipeline-monitor/spec.md) | AWS CodePipeline monitor with desktop notifications |
| [21-amq-web-console](21-amq-web-console/spec.md) | Static web console for browsing/managing ActiveMQ queues via mq-proxy |

## Relationship to `spec-wip/`

Active feature/bugfix/change-request work does not happen here directly —
it happens in [`spec-wip/`](../spec-wip/README.md), following the gated
spec → plan → tasks workflow described in the root `CLAUDE.md`. Once a
change is fully implemented, its content is merged back: the relevant
`spec/<area>/spec.md` above is updated to reflect the new end-state
behavior (or a new area folder is added, numbered 21 onward, for a
genuinely new capability), and the `spec-wip/` folder is deleted. Nothing
is lost by deleting it — the PR that shipped the change is the permanent
record of what was decided and why; this file only needs to reflect
what's true now.

Adding a new area folder: pick the next unused number, name it
`NN-<slug>/`, and give it a single `spec.md` in the same style as the
existing entries (Purpose / Behavior / Data & config / Implementation
notes / Notable gotchas, as applicable).
