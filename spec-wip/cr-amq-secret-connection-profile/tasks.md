# Tasks

1. [ ] **Config schema and backend plumbing.** Add
   `PasswordSecretAWSProfile` to `QueueConfig`/`ProxyConfig`. Update
   `secretbackend.New` (drop `profile` param, add
   `passwordSecretAWSProfile(conn)` helper) and all 4 call sites in
   `app.go`/`host.go`. Simplify `SetActiveAWSProfile` (no more backend
   rebuild). Reword `SecretResolver.Resolve`'s empty-profile error.
   Delete `TestSetActiveAWSProfileRebuildsSecretBackedBackend`; update
   `secretbackend_test.go`'s `New(...)` call sites and `newTestBackend`
   helper. `go build`/`go vet`/`go test ./...` clean.

2. [ ] **Connection editor UI.** Update `setPasswordField`,
   `rebuildTail`, `Show`, and `save` in `connections.go` per `plan.md`
   (the new "Secret AWS Profile" field, required-field validation). Add
   tests in `connections_test.go`: field appears/disappears with
   Password Source, survives a Backend toggle round-trip, round-trips
   through `Show()`→`save()`, and the new validation rejects an empty
   profile with AWS Secret selected. `go build`/`go vet`/`go test ./...`
   clean.

3. [ ] **Docs and manual verification.** Update
   `tui/config.example.yaml`'s `passwordSecret` comment block and both
   commented examples. Manually verify live (per `plan.md` — this one
   doesn't need a real expired SSO session): create/edit a connection
   with `passwordSecret`/`passwordSecretAWSProfile` set, confirm it
   resolves; switch the *global* AWS profile and confirm the
   connection's queues still load unaffected; try saving with AWS
   Secret selected and the new field blank, confirm the validation
   message blocks the save.

4. [ ] **Merge-back.** Update `spec/12-named-connections/spec.md`'s
   "Password resolution" section: remove the "per-connection AWS
   profile" out-of-scope line (this CR ships exactly that), document
   the new required field and its interaction with the connection
   editor, and note `SetActiveAWSProfile` no longer rebuilds
   secret-backed connections. Delete
   `spec-wip/cr-amq-secret-connection-profile/`.
