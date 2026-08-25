# Named connections

_Condensed from spec/22, spec/27, spec/55, spec/56, spec/57 — see those folders for the incremental history._

## Purpose

Let the user define, switch between, and manage multiple broker connections
(e.g. a local dev ActiveMQ and an AWS Amazon MQ in staging) from inside the
app, without hand-editing `config.yaml` and restarting.

## Behavior / user flow

- One connection is active at a time (`config.yaml`'s `activeConnection`).
  The top-left info panel shows an `AMQ Connection: <name>` line.
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
- **Connection editor overlay** (shared by Add and Edit) — a form with:
  - `Name` (required, must be unique).
  - `Backend` dropdown: `jolokia` or `proxy`.
  - `Broker Name` — **only shown when Backend = jolokia**. Toggling the
    Backend dropdown live shows/hides this field immediately (the rest of
    the form — URL, Username, Password Source, Password/Password Secret —
    is preserved across the toggle, not cleared). It's irrelevant to the
    proxy backend, which never reads `QueueConfig`.
  - `URL`, `Username` (both backends).
  - `Password Source` dropdown: `Plain` or `AWS Secret`. Swaps a single
    field below it between a `Password` text input and a `Password Secret`
    text input — only one of the two is ever visible or saved; they're
    mutually exclusive by construction.
  - `Esc` cancels without saving (same effect as the Cancel button).
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
      passwordSecret: /cloudtui/aws-staging/mq-password   # resolved via Secrets Manager
      # password: ""                                       # ignored when passwordSecret is set
```

- `Connection` struct: `Name`, `Backend`, `Queue` (`QueueConfig`), `Proxy`
  (`ProxyConfig`). No `Alias` field — `Name` is the only identifying label,
  used in the info panel, the manager list, and as the `activeConnection`
  key.
- `QueueConfig`/`ProxyConfig` each carry a `Password string` and an optional
  `PasswordSecret string` (yaml `passwordSecret,omitempty`).
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
- Resolved via the single global `cfg.ActiveAWSProfile` (Settings → AWS
  Profiles) — there is no per-connection AWS profile.
- Resolution is lazy: it happens inside whichever `queue.Backend` call
  first needs the password, on that call's existing async goroutine (no
  separate resolve step at activation time). If no AWS profile is
  selected, resolution fails immediately with "no AWS profile selected —
  pick one in Settings → AWS Profiles" instead of attempting a call.
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
- Out of scope by design: per-connection AWS profile, structured/JSON
  secrets, sourcing username/broker-name/URL from Secrets Manager, a manual
  "refresh secret" action, editing/rotating the secret's value from within
  cloudtui.

## Implementation notes

- `tui/internal/config/config.go` — `Connection`, `QueueConfig`,
  `ProxyConfig` structs; `Connections []Connection` / `ActiveConnection
  string` on `Config`; `Load()` migration; `Save()`.
- `tui/internal/dialog/connections.go` — `ConnManager` and `ConnEditor`
  overlays (moved out of `internal/app`; see spec/03,
  architecture-and-package-layout — `ConnEditor` takes its sibling
  `ConnManager` as a constructor parameter, one of the few cross-dialog
  references).
- `tui/internal/queue/secretbackend/` — the `queue.Backend` decorator that
  resolves a `passwordSecret` password lazily (in-memory cache,
  invalidate/retry wiring); moved here from `internal/app` (see
  spec/03).
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
- The Password Source and Backend dropdowns both dynamically rebuild part
  of the form (`tview.Form.RemoveFormItem` + re-`Add*`) driven by a
  `SetSelectedFunc`. Because Broker Name sits in the *middle* of the form
  (between Backend and URL) rather than at the end like the Password swap,
  toggling it requires rebuilding the whole form tail (URL, Username,
  Password Source, Password/Password Secret) rather than inserting/removing
  just one item — otherwise currently-typed values in later fields are lost
  or misaligned.
- A rotated Secrets-Manager secret is discovered either on the next
  read-only call, or immediately if a mutating call happens to fail for an
  unrelated reason first — never proactively.
