# Spec — CR 55: Drop the connection `alias` field

Date: 2026-08-16

## Background

FE 22 ([spec/22-fe-connections](../22-fe-connections/spec.md)) introduced named
connections with both a `name` (human-readable identifier) and an `alias` (a
short 3–6 char label shown where space is tight: the top-left info panel and
the connection manager list).

## Problem

In practice `name` and `alias` end up redundant — every connection needs two
labels for the same thing, and the connection editor form has an extra
required field that doesn't earn its keep. `name` alone is enough to identify
a connection everywhere it's shown.

## Solution

Remove `Alias` entirely. `Name` becomes the only identifying label, used in
the info panel, the connection manager list, and as the uniqueness key (as
`ActiveConnection` already does today).

### Config shape

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
```

No `alias:` key. Existing config files that still have `alias:` on a
connection load fine — YAML unmarshalling silently ignores unknown fields,
same as it already does for other stale keys — and `Save()` never writes
`alias` back, so the field disappears from a file the first time it's
round-tripped. No explicit migration code needed.

## Scope

### In scope

- Remove `Connection.Alias` from `tui/internal/config/config.go`; update
  `Default()`.
- Info panel (`topbar.go`): `AMQ Connection: <alias>` → `AMQ Connection:
  <name>`.
- Settings list item (`settings.go`): `"AMQ Connection: %s"` uses `conn.Name`.
- Connection manager list (`connections.go`): row format `<name>  (<backend>)`
  instead of `<alias>  <name>  (<backend>)`.
- Connection editor form (`app.go`/`connections.go`): remove the "Alias"
  input field; reindex the remaining `GetFormItem()` accesses.
- Duplicate action: copy is named `"<original>-copy"` (no more alias-suffix
  logic like `alias+"2"`).
- `devtool` CLI (`cmd/devtool/main.go`, `internal/devtool/config.go`):
  `add-proxy-conn <name> <alias> <url> <username> <password>` drops the
  `<alias>` argument.
- `config.example.yaml` updated to drop `alias:` lines.
- Update `spec/22-fe-connections/spec.md` to reflect the new (alias-less)
  shape, since this CR changes behavior it documented.

### Out of scope

- Any other connection-editor field changes.
- The AWS-Secrets-Manager password work (tracked separately, see
  `spec/56-fe-amq-connection-aws-secret-password/spec.md`).

## Definition of done

1. `go build ./...` and `go test ./...` pass in `tui/`.
2. A config file containing a stale `alias:` key loads without error and the
   field is gone after the next `Save()`.
3. Info panel, settings list, and connection manager all show the connection
   `name`; there is no `alias` anywhere in the UI.
4. Connection editor has no Alias field; add/edit/duplicate/delete still work.
5. `devtool add-proxy-conn` works with the shortened argument list.
