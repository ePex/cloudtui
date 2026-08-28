# Named connections

_Condensed from spec/22, spec/27, spec/55, spec/56, spec/57 — see those folders for the incremental history. Per-connection AWS profile added by spec-wip/cr-amq-secret-connection-profile — see that PR for the incremental history and rationale._

## Purpose

Let the user define, switch between, and manage multiple broker connections
(e.g. a local dev ActiveMQ and an AWS Amazon MQ in staging) from inside the
app, without hand-editing `config.yaml` and restarting.

## Behavior / user flow

- One connection is active at a time (`config.yaml`'s `activeConnection`).
  The top-left info panel shows an `AMQ Connection: <name>` line — with
  `(AWS: <profile>)` appended when that connection authenticates via AWS
  Secret, naming the profile its password resolves through (see
  Password resolution below). This is separate from, and can differ
  from, the panel's own `AWS Profile: <name>` line just below it, which
  is the *globally* active profile used for SSM Parameters/Secrets
  Manager/CloudWatch Logs/CodePipeline browsing.
- **Settings view** (a `tview.List`) has an `AMQ Connection: <name>` item that
  opens the connection manager overlay. It also has a `Theme: <name>` item
  (theme picker) and an "AWS Profiles" item (spec/14).
- **Connection manager overlay** — reachable from Settings, or directly from
  anywhere via the command prompt (`:aq` or `:connections`):
  - Lists all connections as `<name>  (<backend>)`; the active one is marked
    with a star.
  - `Enter` activates the row under the cursor: hot-swaps the backend
    immediately, updates the info panel, closes any open Messages/Message
    Detail view (returning to the queues list), and reloads the queue list.
  - `n` new, `e` edit, `d` duplicate, `Del`/`x` delete (with confirmation;
    the last remaining connection cannot be deleted). Deleting the active
    connection activates the first remaining one.
  - Duplicate creates a copy named `"<original>-copy"` and opens the editor
    on it.
- **Connection editor overlay** (shared by Add and Edit) — a form grouped
  into three full-modal-width, non-interactive section headers ("──
  General ──", "── Destination ──", "── Auth ──" — Tab skips straight
  over them):
  - **General**: `Name` (required, must be unique).
  - **Destination**: `Backend` dropdown (`jolokia` or `proxy`), then
    `Broker Name` — **only shown when Backend = jolokia**. Toggling the
    Backend dropdown live shows/hides this field immediately (the rest of
    the form is preserved across the toggle, not cleared). It's
    irrelevant to the proxy backend, which never reads `QueueConfig`.
    Then `URL` (both backends).
  - **Auth**: `Username` (both backends), then `Authentication Mode`
    dropdown: `Plain` or `AWS Secret`. Below it, indented (2-space label
    prefix, reading as visually nested under Authentication Mode rather
    than a peer of Name/Backend/URL): a `Password` text input for Plain,
    or an `AWS Profile` + `Secret Name` pair for AWS Secret — only one
    of the two branches is ever visible or saved, mutually exclusive by
    construction. `AWS Profile` is **required** whenever `AWS Secret` is
    selected (validated on save, same pattern as "Name is required"),
    and offers autocomplete against the same discovered-profile source
    Settings → AWS Profiles uses, filtered by prefix.
  - `Save`/`Cancel`, last, outside every section. `Esc` cancels without
    saving (same effect as the Cancel button).
- Changes persist to `config.yaml` immediately on save.

## Data & config

```yaml
activeConnection: local
connections:
  - name: local
    backend: jolokia
    queue:
      brokerName: localhost
      url: http://localhost:8161/api/jolokia
      username: admin
      password: ""
  - name: aws-staging
    backend: proxy
    proxy:
      url: http://localhost:8080
      username: cloudtui
      passwordSecret: /cloudtui/aws-staging/mq-password    # resolved via Secrets Manager
      passwordSecretAWSProfile: work                        # required whenever passwordSecret is set
      # password: ""                                        # ignored when passwordSecret is set
```

- `Connection` struct: `Name`, `Backend`, `Queue` (`QueueConfig`), `Proxy`
  (`ProxyConfig`). No `Alias` field — `Name` is the only identifying label,
  used in the info panel, the manager list, and as the `activeConnection`
  key. `Connection.SecretAWSProfile()` returns the backend-appropriate
  `PasswordSecretAWSProfile` when the backend-appropriate `PasswordSecret`
  is non-empty, else `""` — used by the info panel's `(AWS: <profile>)`
  annotation.
- `QueueConfig`/`ProxyConfig` each carry a `Password string`, an optional
  `PasswordSecret string` (yaml `passwordSecret,omitempty`), and, required
  whenever `PasswordSecret` is set, `PasswordSecretAWSProfile string`
  (yaml `passwordSecretAWSProfile,omitempty`) — the AWS profile used to
  resolve that connection's own secret (see Password resolution below).
- If `connections` is absent from the file, `Load()` synthesizes a single
  connection named `"default"` from legacy top-level `backend`/`queue`/
  `proxy` fields. Those legacy fields exist on `Config` only for this
  migration and are never written back by `Save()` — a round-tripped file
  always uses the `connections` shape. A stale `alias:` key on an old config
  loads fine (unknown YAML fields are ignored) and disappears on next save.

### Password resolution (AWS-Secrets-Manager-backed passwords)

- When `passwordSecret` is set (non-empty), it takes precedence over both
  the plain `password` field and the `MQPROXY_CLIENT_PASSWORD` env-var
  fallback. The secret's value is used verbatim as the password — no JSON
  key extraction; a JSON-valued secret is used including its braces and
  will simply fail auth.
- Resolved via the connection's own `passwordSecretAWSProfile` — a
  required field, independent of the single global `cfg.ActiveAWSProfile`
  (Settings → AWS Profiles) used for SSM Parameters/Secrets Manager/
  CloudWatch Logs/CodePipeline browsing. Switching the global profile has
  *no effect* on an already-configured connection's password — this is
  the point: earlier, the two were conflated, so switching the global
  profile to browse a different account's parameters silently changed
  which account an unrelated connection's password came from too, with
  no way to tell from the UI which profile a connection actually
  depended on. `App.SetActiveAWSProfile` no longer rebuilds the active
  backend as a result — resolution simply no longer reads that value.
- Resolution is lazy: it happens inside whichever `queue.Backend` call
  first needs the password, on that call's existing async goroutine (no
  separate resolve step at activation time). An empty
  `passwordSecretAWSProfile` (only reachable via a hand-edited
  `config.yaml` that bypasses the editor's required-field validation)
  fails resolution immediately with "no AWS profile configured for this
  connection's password secret — set passwordSecretAWSProfile" instead
  of attempting a call.
- Resolved values are cached in memory only, keyed by `(profile,
  secretName)` — never written to `config.yaml`, never persisted across
  restarts.
- Failure/refetch behavior differs by call type:
  - **Read-only calls** (list queues, browse/detail a message) that fail on
    a cached secret: invalidate the cache, refetch, rebuild the backend
    client, and transparently retry the same call once. A still-failing
    retry surfaces the error normally.
  - **Mutating calls** (delete, move, send, purge) are never auto-retried
    (to avoid double-applying a delete/move that actually succeeded
    server-side but returned an error). The cache is still invalidated on
    failure so the *next* call fetches fresh, but the mutating call itself
    just fails and is reported as usual.
- **SSO re-auth**: if resolving the secret fails because the active AWS
  profile's SSO session is missing/expired, the same mechanism every
  other AWS-backed view already uses (`awsauth.WithReauth`,
  spec/14-aws-profiles) kicks in — a status message ("AWS SSO session
  expired — opening browser to log in...", updated in place with the
  device verification code/URL once `aws sso login` prints them — see
  spec/14 for why), then `aws sso login` opens the browser, then the
  secret resolution (and whichever `queue.Backend` call needed it)
  retries once. Wired in
  `secretbackend.SecretResolver.Resolve` — the one place that actually
  calls AWS Secrets Manager — so it covers every operation on a
  secret-backed connection (list, browse, send, delete, move, purge),
  not just one call site. Unlike the other `WithReauth` call sites
  (each owned by one view's own table, using an in-table status row),
  this one posts to the bottom status bar (`Host.SetStatus`) instead,
  since the resolver is shared across every view that might touch a
  secret-backed connection — but only as a **fallback**: if the
  currently active view implements `ui.ReauthStatusShower` (e.g.
  `QueuesView`, whose own loading placeholder switches from "Loading
  queues…" to the SSO-wait message and back once login completes), it
  handles the message itself and the status bar is left untouched
  entirely. Found live, in two rounds: first, that the status bar's
  message never got cleared after login completed; then, once the
  table-level display was added and fixed, that showing the *same*
  message in both places at once was redundant rather than helpful — so
  the two displays are mutually exclusive, not simultaneous. Only
  triggers for an SSO-authenticating profile — a
  static-keys/assume-role/credential-process profile's resolution
  failure surfaces as a normal error, unchanged.
- Out of scope by design: structured/JSON secrets, sourcing
  username/broker-name/URL from Secrets Manager, a manual "refresh
  secret" action, editing/rotating the secret's value from within
  cloudtui, a profile-*picker* dropdown restricted to known profiles for
  `passwordSecretAWSProfile` (the field stays freeform text with
  autocomplete — an unlisted or not-yet-configured profile name must
  still be typeable).

## Implementation notes

- `tui/internal/config/config.go` — `Connection`, `QueueConfig`,
  `ProxyConfig` structs; `Connections []Connection` / `ActiveConnection
  string` on `Config`; `Load()` migration; `Save()`.
  `Connection.SecretAWSProfile()` returns the backend-appropriate
  `PasswordSecretAWSProfile` when the backend-appropriate
  `PasswordSecret` is non-empty, else `""`.
- `tui/internal/dialog/connections.go` — `ConnManager` and `ConnEditor`
  overlays (moved out of `internal/app`; see spec/03,
  architecture-and-package-layout — `ConnEditor` takes its sibling
  `ConnManager` as a constructor parameter, one of the few cross-dialog
  references). Every field is looked up via `GetFormItemByLabel` rather
  than a fixed `GetFormItem(index)` — the three section headers occupy
  indices that shift depending on Backend/Authentication Mode, so no
  field's numeric position is stable. `NewConnEditor` builds only the
  static prefix (General header, Name, Destination header, Backend)
  directly and calls `rebuildTail("jolokia")` once to build the rest,
  rather than duplicating that field list in two places.
- `tui/internal/dialog/sectionheader.go` — `sectionHeaderItem`, a
  bespoke `tview.FormItem` for a non-interactive, full-modal-width
  section-divider row (`tview.Form`'s own `AddTextView` can't be both
  flush-left and full-width at once — see Notable gotchas below).
- `tui/internal/queue/secretbackend/` — the `queue.Backend` decorator that
  resolves a `passwordSecret` password lazily (in-memory cache,
  invalidate/retry wiring); moved here from `internal/app` (see
  spec/03). `New(resolver, conn)` derives the AWS profile from `conn`
  itself (via its own unexported `passwordSecretAWSProfile(conn)`
  helper) rather than taking it as a separate parameter.
- Command prompt: `onPromptDone` matches `"aq"`/`"connections"` →
  open the connection manager. It intentionally has no bare-key global
  hotkey (unlike `s` for Settings) — it's one level under Settings, and
  `:aq` is prompt-only by design. Opening an overlay from the prompt must
  skip the prompt's usual post-command `SetFocus(a.pages)` reset (the
  connection manager is a `rootPages` overlay, a sibling of `a.pages`, not
  a page inside it) — the focus-reset is guarded by the same overlay-
  visibility check `onGlobalKey` uses, or the overlay renders on top but
  keystrokes go to the hidden view underneath.
- `cmd/devtool` — `add-proxy-conn <name> <url> <username> <password>` (no
  alias argument).

## Notable gotchas worth preserving

- Broker Name is a jolokia-only concept: it builds the ActiveMQ JMX
  `ObjectName` (`org.apache.activemq:type=Broker,brokerName=<name>,...`)
  used by every jolokia call site. The proxy backend never reads
  `QueueConfig` at all, so the field must be structurally absent (not just
  ignored) from the editor when Backend = proxy — showing it invites typing
  a value that's silently discarded.
- The Authentication Mode and Backend dropdowns both dynamically rebuild
  part of the form (`tview.Form.RemoveFormItem` + re-`Add*`) driven by a
  `SetSelectedFunc`. Because Broker Name sits in the *middle* of the form
  (between Backend and URL) rather than at the end like the Password
  swap, toggling it requires rebuilding the whole form tail (URL,
  Username, Authentication Mode, Password/AWS Profile+Secret Name)
  rather than inserting/removing just one item — otherwise currently-
  typed values in later fields are lost or misaligned.
- A rotated Secrets-Manager secret is discovered either on the next
  read-only call, or immediately if a mutating call happens to fail for an
  unrelated reason first — never proactively.
- Editing an existing AWS-Secret connection and tabbing straight out of
  "AWS Profile" without typing anything could silently replace the saved
  profile with an unrelated one: `SetAutocompleteFunc`'s eager wiring
  call builds the drop-down while the field is still empty (fired by
  `Show()`'s `SetCurrentOption` before it sets the real saved value),
  and plain `SetText()` doesn't refresh an already-open drop-down —
  `tview.InputField` treats Tab as "accept the drop-down's current
  entry" whenever one is open, not "move to the next field". Fixed by
  calling `Autocomplete()` right after `SetText()` in `Show()` — the
  same fix already applied to `MessageFilter.jmsTypeItem`
  (spec/01-repo-and-tui-shell's own autocomplete gotcha).
- `tview.Form.AddTextView`'s `TextView` can't be both flush-left and
  span the form's full width at once: `SetFormAttributes` (called by
  every `Form.Draw()` pass) unconditionally reserves the form's shared
  label-width column before drawing anything — body text renders
  indented to that column, label text is truncated to it. A bespoke
  `tview.FormItem` (`sectionHeaderItem`) that discards the incoming
  label width and computes its own text at `Draw()` time from
  `GetInnerRect()`'s actual width sidesteps this; it stays non-focusable
  by replicating `TextView.Focus()`'s own "replay the last Tab/Backtab
  via the finished callback instead of taking real focus" trick for a
  non-scrollable, Form-embedded `TextView`.
- Save/Cancel render with an apparent ~2-column indent relative to every
  field's own left edge — this is `tview.Button.Draw()` centering its
  label within a box `label+4` cells wide (both hardcoded, no public
  override), not an offset of the button's own position (confirmed:
  its box starts at the same column as every field). Present on every
  `AddButton`-using dialog in this codebase; not fixable without
  replacing `tview.Form`'s built-in buttons everywhere, which was
  judged not worth it for a purely cosmetic gap.
