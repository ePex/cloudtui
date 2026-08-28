# Tasks

1. [x] **Config schema and backend plumbing.** Added
   `PasswordSecretAWSProfile` to `QueueConfig`/`ProxyConfig`. Updated
   `secretbackend.New` (dropped `profile` param, added
   `passwordSecretAWSProfile(conn)` helper) and all 4 call sites in
   `app.go`/`host.go`. Simplified `SetActiveAWSProfile` (no more backend
   rebuild). Reworded `SecretResolver.Resolve`'s empty-profile error.
   Deleted `TestSetActiveAWSProfileRebuildsSecretBackedBackend` (and the
   now-unused `secretbackend` import in `host_test.go`).
   `secretbackend_test.go` needed no changes at all — `newTestBackend`
   already constructs `*Backend` via a struct literal, never through
   `New()`, so it was already decoupled from this signature change.
   `go build`/`go vet`/`go test ./...` clean.

2. [x] **Connection editor UI.** Updated `setPasswordField`,
   `rebuildTail`, `Show`, and `save` in `connections.go` per `plan.md`
   (the new "Secret AWS Profile" field, required-field validation).
   Added 4 tests in `connections_test.go`:
   `TestConnEditorSecretAWSProfileFieldTracksPasswordSource` (appears/
   disappears with Password Source), `TestConnEditorSecretAWSProfileSurvivesBackendToggle`
   (survives a jolokia->proxy->jolokia round trip), `TestConnEditorPasswordSecretAWSProfileRoundTrips`
   (Show()->edit->save() round-trip), and
   `TestConnEditorSaveRequiresSecretAWSProfileWhenAWSSecretSelected`
   (validation rejects an empty profile with AWS Secret selected).
   `gofmt -l .`, `go build ./...`, `go vet ./...`, `go test ./...` all
   clean.

3. [x] **Docs and manual verification.** Updated
   `tui/config.example.yaml`'s `passwordSecret` comment block and both
   commented examples to document the new required
   `passwordSecretAWSProfile` field. Manually verified live via tmux
   against the real binary and the user's real config (backed up and
   restored around the test): created a new jolokia connection
   (`zzz-cr-test`), switched Password Source to AWS Secret — "Secret
   AWS Profile" appeared alongside "Password Secret (AWS)"; attempted
   Save with the profile blank — blocked with status message "AWS
   Profile is required when Password Source is AWS Secret", editor
   stayed open; filled in the profile, saved successfully, confirmed
   `passwordSecret`/`passwordSecretAWSProfile` both persisted correctly
   to `config.yaml`; reopened the connection for editing — the field
   round-tripped correctly; switched the *global* AWS profile via
   Settings -> AWS Profiles and confirmed `zzz-cr-test`'s
   `passwordSecretAWSProfile` was untouched in `config.yaml`, proving
   the two are independent as designed. Test connection and config
   changes cleaned up afterward (config.yaml restored from backup).

4. [ ] **Merge-back.** Update `spec/12-named-connections/spec.md`'s
   "Password resolution" section: remove the "per-connection AWS
   profile" out-of-scope line (this CR ships exactly that), document
   the new required field and its interaction with the connection
   editor, and note `SetActiveAWSProfile` no longer rebuilds
   secret-backed connections. Delete
   `spec-wip/cr-amq-secret-connection-profile/`.
