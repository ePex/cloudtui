# Spec — Bugfix 57: hide Broker Name when Backend is proxy

Date: 2026-08-16

## Background

The connection editor (FE 22, refined in FE 56) always shows a "Broker
Name" field regardless of which Backend is selected. `BrokerName` is used
to build the ActiveMQ JMX `ObjectName` for every jolokia operation
(`org.apache.activemq:type=Broker,brokerName=<name>,...` — see
`internal/queue/jolokia/jolokia.go`, 8 call sites, and
`internal/devtool/queue.go`), so it's genuinely required for the jolokia
backend.

## Problem

`internal/queue/proxy/proxy.go` never reads `QueueConfig` at all — for a
proxy-backend connection, Broker Name is completely unused. Worse,
`saveConnEditor` already silently discards whatever's typed into it when
Backend is proxy (`conn.Queue` is simply never populated in that branch).
The field is visible and editable but has no effect — a misleading form
input, not just an unused one.

## Solution

Hide the Broker Name field from the connection editor whenever Backend is
set to proxy, using the same dynamic swap mechanism FE 56 already
introduced for the Password Source dropdown (`tview.Form.RemoveFormItem` +
re-`Add*`, driven by a dropdown's `SetSelectedFunc`) — this time driven by
the existing Backend dropdown instead of a new one.

Because Broker Name sits in the *middle* of the form (between Backend and
URL), not at the end like the Password/Password Secret swap, toggling it
shifts every item after it. The chosen approach rebuilds the whole form
tail after Backend (URL, Username, Password Source, and Password/Password
Secret) rather than trying to insert/remove just one middle item — see
`plan.md` for why, and how currently-typed values survive the rebuild.

## Scope

### In scope

- Connection editor: Broker Name field only present when Backend is
  jolokia. Toggling Backend between jolokia and proxy during editing
  shows/hides it live, without losing whatever the user already typed into
  URL, Username, the Password Source selection, or Password/Password
  Secret.
- `saveConnEditor`: unaffected in outcome (Broker Name was already only
  read for jolokia), but now reads it defensively (empty when hidden)
  rather than relying on the Backend branch to discard it.
- Prefilling an existing connection for edit: Broker Name is shown (and
  filled in) only if that connection is jolokia.

### Out of scope

- Any change to which fields are backend-specific for URL/Username/
  Password — those stay shown for both backends, as they are today (both
  backends genuinely use URL/Username/Password-or-secret).
- `devtool` CLI (`add-proxy-conn`) — already jolokia-agnostic, doesn't
  touch Broker Name.
- Validation (e.g. requiring Broker Name non-empty for jolokia) — not
  currently validated today either; not introducing new validation here.

## Definition of done

1. `go build ./...` and `go test ./...` pass in `tui/`.
2. Opening the editor for a jolokia connection shows Broker Name, prefilled;
   for a proxy connection, Broker Name is absent.
3. Toggling Backend live in the editor shows/hides Broker Name immediately,
   without clearing URL, Username, Password Source, or Password/Password
   Secret.
4. Saving a proxy connection never has Broker Name in the resulting
   `config.Connection` (matches today's behavior, now because it's
   structurally impossible to type it in, not just discarded).
