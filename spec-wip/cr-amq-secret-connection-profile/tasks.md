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
   (the new field, required-field validation). Added 4 tests in
   `connections_test.go`: `TestConnEditorAWSProfileFieldTracksPasswordSource`
   (appears/disappears with Password Source, and sits directly above
   the secret-name field), `TestConnEditorAWSProfileSurvivesBackendToggle`
   (survives a jolokia->proxy->jolokia round trip), `TestConnEditorPasswordSecretAWSProfileRoundTrips`
   (Show()->edit->save() round-trip), and
   `TestConnEditorSaveRequiresAWSProfileWhenAWSSecretSelected`
   (validation rejects an empty profile with AWS Secret selected).
   `gofmt -l .`, `go build ./...`, `go vet ./...`, `go test ./...` all
   clean.

   **Follow-up per user feedback**: the field was initially labeled
   "Secret AWS Profile" and placed *below* "Password Secret (AWS)".
   The user asked for it to lead instead — renamed to "AWS Profile",
   moved above the secret-name field (also renamed, to "Password
   Secret Name" for symmetry), in `setPasswordField`/`rebuildTail`/
   `Show`/`save` and all 4 tests (which now also assert the field
   order via `GetFormItemIndex`). Re-verified live via tmux: field
   order/labels render correctly, and the full create->save->persist
   round trip still works with the new names.

3. [x] **Docs and manual verification.** Updated
   `tui/config.example.yaml`'s `passwordSecret` comment block and both
   commented examples to document the new required
   `passwordSecretAWSProfile` field. Manually verified live via tmux
   against the real binary and the user's real config (backed up and
   restored around the test): created a new jolokia connection
   (`zzz-cr-test`), switched Password Source to AWS Secret — the AWS
   profile field appeared alongside the secret-name field; attempted
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

4. [x] **AWS Profile autocomplete** (added per user feedback after the
   initial cut). `ConnEditor` gained `awsProfileNames []string`,
   `loadAWSProfileNames()` (called once per `Show()`, before the
   Password Source `SetCurrentOption` that can immediately recreate the
   field), `awsProfileSuggestions(currentText string) []string`
   (prefix-filters the cached names, same convention as
   `MessageFilter.jmsTypeSuggestions`), and `wireAWSProfileAutocomplete()`
   (styles then wires `SetAutocompleteFunc` on the "AWS Profile" field —
   called from both `setPasswordField` and `rebuildTail`, the only two
   places that field is (re)created). Added 2 tests in
   `connections_test.go`: `TestConnEditorAWSProfileSuggestionsFiltersByPrefix`
   and `TestConnEditorAWSProfileSuggestionsEmptyOnDiscoveryError`
   (degrades to no suggestions, doesn't break `Show()`, on a discovery
   error). `gofmt -l .`, `go build ./...`, `go vet ./...`,
   `go test ./...` all clean. Manually verified live via tmux against
   the real binary and the user's real `~/.aws/config` (backed up and
   restored around the test): typing "redacted-profile" into "AWS Profile" popped a
   real, filtered autocomplete list of the user's actual `redacted-profile-*`
   profiles; selecting an entry populated the field correctly.

5. [x] **Info panel shows the connection's AWS profile** (added per
   user feedback after the initial cut). Added `Connection.SecretAWSProfile()`
   to `config.go` (backend-appropriate `PasswordSecretAWSProfile` when
   the backend-appropriate `PasswordSecret` is non-empty, else `""`).
   `ui.InfoPanelText` (`topbar.go`) appends `(AWS: <profile>)` to the
   "AMQ Connection" line when non-empty. Added 4 tests:
   `TestSecretAWSProfileJolokia`/`Proxy`/`EmptyWhenPlainPassword`/
   `EmptyWhenPasswordSecretUnset` in `config_test.go`, and
   `TestInfoPanelTextShowsSecretAWSProfile`/
   `TestInfoPanelTextOmitsSecretAWSProfileForPlainPassword` in
   `topbar_test.go`. `gofmt -l .`, `go build ./...`, `go vet ./...`,
   `go test ./...` all clean. Manually verified live via tmux against
   the real binary/config: the real `redacted-aws-secret-connection`
   connection's info panel line read
   `AMQ Connection: redacted-aws-secret-connection (AWS: redacted-profile)` while the
   separate "AWS Profile:" line showed the differing globally-active
   profile, demonstrating the two are independent; switching to a
   plain-password connection showed no annotation. (Made and then
   corrected a stray real-config change mid-verification — a settings
   list cursor persisted from a prior navigation and a keystroke landed
   on the AWS Profile picker instead of the connection manager;
   activeConnection/activeAWSProfile were restored to their original
   values and confirmed via `config.yaml` before moving on.)

6. [x] **Bugfix: tabbing out of "AWS Profile" could silently change it**
   (found by the user while editing an existing AWS-Secret connection).
   Root cause: `wireAWSProfileAutocomplete`'s `SetAutocompleteFunc` call
   in `setPasswordField` eagerly builds tview's autocomplete drop-down
   while the field is still empty (fired by `Show()`'s
   `SetCurrentOption(1)`, before `Show()` sets the real saved profile
   text) — `SetText()` doesn't refresh an already-open drop-down, so it
   stayed unfiltered/stale; tview's `InputField` treats Tab as "accept
   the drop-down's current entry" rather than "move to the next field"
   whenever a drop-down is open, so tabbing straight out silently
   replaced the just-set profile with the stale list's pre-selected
   entry (e.g. alphabetically-first). Fixed in `Show()` by calling
   `profileItem.Autocomplete()` right after `SetText()` — the exact
   same fix already used for `MessageFilter.jmsTypeItem` in
   `messagefilter.go`'s own `Show()`. Added
   `TestConnEditorAWSProfileFieldSurvivesTabWhenEditingExisting`
   (simulates the Tab keypress via `InputField.InputHandler()` directly
   — confirmed it fails without the fix, via a `git stash` of just the
   fix and re-running it, before restoring). `gofmt -l .`,
   `go build ./...`, `go vet ./...`, `go test ./...` all clean.
   Manually re-verified live via tmux against the real binary/config:
   editing the real `redacted-aws-secret-connection` connection, tabbing
   into "AWS Profile" now shows a drop-down correctly pre-filtered to
   that profile's own prefix (not the full unfiltered list), and
   tabbing straight out leaves the value unchanged and correctly
   advances focus.

7. [x] **Indent the fields below "Password Source"** (added per user
   feedback). "Password" / "AWS Profile" / "Password Secret Name" now
   carry a 2-space indent (`labelPassword`/`labelAWSProfile`/
   `labelPasswordSecretName` constants in `connections.go`, since
   `GetFormItemByLabel` matches the label string exactly, indent
   included — used everywhere these fields are created or looked up).
   Updated all call sites in `connections.go` and all
   `GetFormItemByLabel`/`GetFormItemIndex` lookups in
   `connections_test.go` (comment/error-message prose left as plain
   "AWS Profile" etc., unaffected since those are just text). `gofmt -l .`,
   `go build ./...`, `go vet ./...`, `go test ./...` all clean. Manually
   verified live via tmux: both the Plain "Password" field and the AWS
   Secret "AWS Profile"/"Password Secret Name" pair render indented
   under "Password Source".

8. [x] **Connection editor sections + renames** (added per user
   feedback). Investigated the reported "Save/Cancel are indented too"
   as a possible regression: built the prior commit in a detached
   worktree and confirmed the same 2-column offset already existed
   there — it's `tview.Button`'s own inherent width padding
   (`label+4`), unrelated to and unaffected by the label-indent commit;
   reported this finding rather than "fixing" something that wasn't
   actually broken by this CR (and isn't reachable via the public
   `tview` API regardless). Restructured `connections.go` into three
   sections via `tview.Form.AddTextView(header, "", 0, 1, false,
   false)` (non-focusable — Tab skips it) with the header text in the
   TextView's label slot (renders flush left, not indented to the value
   column): "── General ──" (Name), "── Destination ──" (Backend, Broker
   Name/URL), "── Auth ──" (Username, Authentication Mode, and the
   indented field(s) below it); Save/Cancel stay last, unindented,
   outside every section. Renamed "Password Source" → "Authentication
   Mode" and "Password Secret Name" → "Secret Name" throughout
   (`labelAuthenticationMode`/`labelSecretName` constants, save's
   validation message wording). Switched every fixed-index
   `GetFormItem(N)` lookup to `GetFormItemByLabel` (Name/Backend no
   longer sit at fixed indices 0/1 once headers exist) — incidentally
   fixed a pre-existing dead-code bug in `ApplyPalette` (a stale
   `GetFormItem(2)` that had silently targeted the wrong item since an
   earlier structural move). `NewConnEditor` now builds only the static
   prefix directly and calls `rebuildTail("jolokia")` once instead of
   duplicating the tail-field chain. `Show()` now explicitly focuses
   Name (`SetFocus`+`GetFormItemIndex`), since Form defaults a fresh
   instance's focus to index 0 — now the header — and there's no prior
   Tab/Backtab yet for `TextView.Focus()`'s "replay the last one"
   fallback to use. Bumped `app.go`'s `connEditorOverlay` fixed height
   from 20 to 28 rows after live verification showed the taller
   11-item worst case (jolokia + AWS Secret) clipping AWS Profile/
   Secret Name/Save/Cancel entirely below the box. Added 3 tests:
   `TestConnEditorSectionHeadersPresentInOrder`,
   `TestConnEditorSectionHeadersAreNotFocusable` (Tab from Name reaches
   Backend, skipping the header — via `Form.Focus(delegate)` +
   `InputHandler()`, since `Form.SetFocus` alone doesn't wire the
   `SetFinishedFunc` a real Tab needs), and `TestConnEditorFormItemCount`
   (pins the 11-item worst case so a future field addition needing
   another height bump doesn't silently start clipping instead) — plus
   updated existing tests'/comments' stale "Password Source"/"Password
   Secret Name" references. `gofmt -l .`, `go build ./...`,
   `go vet ./...`, `go test ./...`, `go test ./... -race` (dialog + app
   packages) all clean. Manually verified live via tmux against the
   real binary/config (backed up and restored around the test): all
   three sections render correctly for both backends and both
   Authentication Modes; the taller box no longer clips; Tab from Name
   correctly skips the Destination header (typed text landed in the
   right field each time); a full create→fill AWS Profile+Secret
   Name→save→persist round trip against a throwaway connection worked
   end-to-end with the new field names.

9. [ ] **Merge-back.** Update `spec/12-named-connections/spec.md`'s
   "Password resolution" section: remove the "per-connection AWS
   profile" out-of-scope line (this CR ships exactly that), document
   the new required field (including its autocomplete and its info
   panel display) and its interaction with the connection editor, and
   note `SetActiveAWSProfile` no longer rebuilds secret-backed
   connections. Delete `spec-wip/cr-amq-secret-connection-profile/`.
