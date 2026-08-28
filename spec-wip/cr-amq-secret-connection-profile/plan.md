# Implementation plan

## `tui/internal/config/config.go`

`QueueConfig`/`ProxyConfig` each gain the new field, next to
`PasswordSecret`:

```go
type QueueConfig struct {
	BrokerName string `yaml:"brokerName"`
	URL        string `yaml:"url"`
	Username   string `yaml:"username"`
	Password   string `yaml:"password"`
	PasswordSecret string `yaml:"passwordSecret,omitempty"`
	// PasswordSecretAWSProfile names the AWS profile used to resolve
	// PasswordSecret — required whenever PasswordSecret is set (see
	// spec/12-named-connections). Independent of cfg.ActiveAWSProfile,
	// the profile used for SSM Parameters/Secrets Manager/CloudWatch
	// Logs/CodePipeline browsing.
	PasswordSecretAWSProfile string `yaml:"passwordSecretAWSProfile,omitempty"`
}
```

(Same shape for `ProxyConfig`.) `omitempty` matches `PasswordSecret`'s
own tag — both are absent from a plain-password connection's YAML.

## `tui/internal/queue/secretbackend/secretbackend.go`

- `passwordSecretName(conn)` gets a sibling,
  `passwordSecretAWSProfile(conn) string`, same shape (branches on
  `conn.Backend == "proxy"`).
- `New`'s signature drops `profile string` entirely:

  ```go
  func New(resolver *SecretResolver, conn config.Connection) queue.Backend {
  	secretName := passwordSecretName(conn)
  	if secretName == "" {
  		return buildBackend(conn)
  	}
  	return &Backend{resolver: resolver, conn: conn, secretName: secretName, profile: passwordSecretAWSProfile(conn), build: buildBackend}
  }
  ```

  `Backend`'s own fields/methods (`profile`, `Profile()`, `current()`,
  `refresh()`) are otherwise unchanged — `profile` is still captured
  once at construction from `conn`, same discipline as before, just
  sourced from the connection's own field instead of a caller-supplied
  parameter.
- `SecretResolver.Resolve`'s empty-profile error message changes from
  "no AWS profile selected — pick one in Settings -> AWS Profiles" to
  something naming the connection field instead — e.g. "no AWS profile
  configured for this connection's password secret — set
  passwordSecretAWSProfile" — since an empty profile now means a
  hand-edited config bypassing the editor's required-field validation,
  not "no global profile picked."

## `tui/internal/app/app.go` + `tui/internal/app/host.go`

All 4 `secretbackend.New(...)` call sites drop the
`a.cfg.ActiveAWSProfile` argument:

```go
a.backend = secretbackend.New(a.secretResolver, cfg.ActiveConn())
```

`SetActiveAWSProfile` (`host.go`) simplifies — no longer rebuilds
`a.backend` or calls `a.queuesV.SetBackend`, since secret resolution no
longer depends on this value at all:

```go
func (a *App) SetActiveAWSProfile(name string) {
	a.cfg.ActiveAWSProfile = name
	a.infoPanel.SetText(ui.InfoPanelText(a.cfg))
	a.settingsV.Refresh()
	if err := config.SaveDefault(a.cfg); err != nil {
		slog.Error("SetActiveAWSProfile: save failed", "error", err)
	}
}
```

## `tui/internal/dialog/connections.go`

Three places already identified by reading the current form-building
code (initial construction, `Show()`, `setPasswordField`, `rebuildTail`,
`save()`):

1. **`setPasswordField(sourceIdx)`**: currently swaps exactly the one
   trailing item. Needs to remove/add *two* items when the AWS Secret
   side is involved, and must work regardless of which state it's
   currently in (checked via whether "AWS Profile" currently exists,
   not via a separately-tracked flag). Field order is "AWS Profile"
   above "Password Secret Name" (renamed from "Password Secret (AWS)")
   — the user asked for the profile field to lead, since it's the more
   fundamental choice ("which account") with the secret name scoped
   under it:

   ```go
   func (ce *ConnEditor) setPasswordField(sourceIdx int) {
   	f := ce.form
   	currentCount := 1
   	if _, ok := f.GetFormItemByLabel("AWS Profile").(*tview.InputField); ok {
   		currentCount = 2
   	}
   	for i := 0; i < currentCount; i++ {
   		f.RemoveFormItem(f.GetFormItemCount() - 1)
   	}
   	if sourceIdx == 1 {
   		f.AddInputField("AWS Profile", "", 20, nil, nil)
   		f.AddInputField("Password Secret Name", "", 30, nil, nil)
   	} else {
   		f.AddPasswordField("Password", "", 20, '*', nil)
   	}
   }
   ```

2. **`rebuildTail(backend)`**: capture the new field's current text
   before wiping the tail (same "don't lose what the user typed" rule
   already applied to `passwordOrSecret`), and re-add it when
   `sourceIdx == 1`:

   ```go
   var url, username, passwordOrSecret, secretProfile string
   // ...
   if item, ok := f.GetFormItemByLabel("AWS Profile").(*tview.InputField); ok {
   	secretProfile = item.GetText()
   }
   // ... (existing wipe loop unchanged)
   f.AddDropDown("Password Source", []string{"Plain", "AWS Secret"}, sourceIdx, nil)
   if sourceIdx == 1 {
   	f.AddInputField("AWS Profile", secretProfile, 20, nil, nil)
   	f.AddInputField("Password Secret Name", passwordOrSecret, 30, nil, nil)
   } else {
   	f.AddPasswordField("Password", passwordOrSecret, 20, '*', nil)
   }
   ```

3. **`Show(conn, isNew, origName)`**: read
   `conn.Queue.PasswordSecretAWSProfile` /
   `conn.Proxy.PasswordSecretAWSProfile` alongside `passwordSecret`, set
   it into the new field when `sourceIdx == 1`, same place `passwordSecret`
   itself is set.

4. **`save()`**: read the new field (trimmed) when `sourceIdx == 1`;
   validate it's non-empty (new check, right after the existing "Name is
   required" check):

   ```go
   if sourceIdx == 1 && passwordSecretProfile == "" {
   	ce.host.SetStatus("[red]AWS Profile is required when Password Source is AWS Secret[-]")
   	return
   }
   ```

   Then include it in both the `conn.Proxy = ...` and
   `conn.Queue = ...` construction lines.

## Test updates

- `tui/internal/queue/secretbackend/secretbackend_test.go`: `New(...)`
  call sites drop the `profile` argument (the connection's
  `PasswordSecretAWSProfile` field now supplies it — test fixtures set
  it directly on the `config.Connection` literal instead of passing it
  separately). `newTestBackend` helper updated to match.
- `tui/internal/app/host_test.go`: **delete**
  `TestSetActiveAWSProfileRebuildsSecretBackedBackend` entirely (per
  spec.md — the scenario it guards against can't happen anymore).
  `TestSetActiveAWSProfilePersistsAndUpdatesUI` stays (still valid —
  `SetActiveAWSProfile` still updates the info panel/settings list/config,
  just doesn't touch `a.backend` anymore).
- `tui/internal/dialog/connections_test.go`: new tests for the "AWS
  Profile" field mirroring the existing "Password Secret Name" field's
  own test coverage — appears/disappears with Password Source (and sits
  directly above "Password Secret Name"), survives a Backend toggle
  round-trip (`rebuildTail`), round-trips through `Show()`→`save()`,
  and the new required-field validation (empty profile + AWS Secret
  source → status message, no save).

## AWS Profile autocomplete (added per user feedback after the initial cut)

The "AWS Profile" field gains autocomplete against the same discovery
source Settings → AWS Profiles already uses (`ui.Host.ListAWSProfiles`,
backed by `awsprofile.List()`), following the existing JMS Type field's
pattern (`MessageFilter.jmsTypeSuggestions` / `messagefilter.go`,
`jmstypeprompt.go`) rather than inventing a new one:

- `ConnEditor` gains `awsProfileNames []string`, refreshed once per
  `Show()` call via a new `loadAWSProfileNames()` (mirrors
  `AWSProfilesPicker.populate()`'s own re-discovery on every open —
  no cross-open caching there either). A discovery error just leaves
  `awsProfileNames` nil — no error surfaced inline, the field just
  offers no suggestions and still works as plain freeform text.
- `awsProfileSuggestions(currentText string) []string` filters the
  cached names by prefix (`strings.HasPrefix`), same as
  `jmsTypeSuggestions`.
- A new `wireAWSProfileAutocomplete()` helper calls
  `ui.StyleInputFieldAutocomplete` *then* `SetAutocompleteFunc` (order
  matters — see the tview gotcha already documented at every other
  autocomplete call site in this codebase) on the "AWS Profile" field.
  Called right after `f.AddInputField("AWS Profile", ...)` in both
  `setPasswordField` and `rebuildTail` — the only two places that field
  gets (re)created.
- `Show()` calls `loadAWSProfileNames()` before the Password Source
  `SetCurrentOption` call that can immediately create the field (when
  editing an existing AWS-Secret connection) — `SetAutocompleteFunc`
  eagerly invokes the callback once at wiring time, so the cache must
  already be populated by then.

Tests: `awsProfileSuggestions` is unit-tested directly against
`testHost.listAWSProfiles` (same convention `TestMessageFilterJMSType*`
already uses for `jmsTypeSuggestions`) — prefix filtering, and graceful
empty-suggestions behavior on a discovery error.

## Info panel (added per user feedback after the initial cut)

`config.Connection` gains a `SecretAWSProfile() string` method — returns
the backend-appropriate `PasswordSecretAWSProfile` when the
backend-appropriate `PasswordSecret` is non-empty, else `""` (so a
hand-edited config with a stray profile but no secret doesn't falsely
report one in use). Placed on `Connection` rather than duplicating
`secretbackend.go`'s existing private `passwordSecretName`/
`passwordSecretAWSProfile` helpers, since `ui/topbar.go` doesn't import
`queue/secretbackend`.

`ui.InfoPanelText` (`topbar.go`) calls it on `cfg.ActiveConn()`: when
non-empty, the "AMQ Connection" line becomes
`<name> (AWS: <profile>)` instead of just `<name>`, in the label color
(a secondary annotation, not the primary value).

Tests: `config.SecretAWSProfile` unit-tested directly (jolokia backend,
proxy backend, plain password → empty, PasswordSecretAWSProfile set but
PasswordSecret unset → empty). `ui.InfoPanelText` tested for both the
secret case (contains the profile annotation) and the plain-password
case (no stray annotation).

## Bugfix: Tab out of "AWS Profile" could silently change it (found by user)

Editing an existing AWS-Secret connection, then tabbing straight out of
"AWS Profile" without typing, could silently replace the saved profile
with an unrelated one. Root cause: `wireAWSProfileAutocomplete`'s
`SetAutocompleteFunc` (called from `setPasswordField`, fired by
`Show()`'s `SetCurrentOption(1)`) eagerly builds tview's autocomplete
drop-down while the field is still empty — before `Show()`'s later
`SetText(passwordSecretProfile)` sets the real value. `SetText()` alone
doesn't refresh an already-open drop-down (same gotcha already
documented for `MessageFilter.jmsTypeItem`), so the drop-down stayed
built from `""` (all profiles, arbitrary pre-selection). tview's
`InputField.InputHandler()` treats Tab as "accept the drop-down's
current entry" rather than "move to the next field" whenever a
drop-down is open — so tabbing out replaced the field's real value with
that stale pre-selection.

Fix: in `Show()`, call `profileItem.Autocomplete()` immediately after
`SetText(passwordSecretProfile)` — rebuilds the drop-down filtered by
the real current text, so it either matches (harmless no-op selection)
or, with no matches, closes entirely. Exactly the fix
`MessageFilter.Show()` already applies to `jmsTypeItem` for the same
reason.

## Connection editor sections + renames (added per user feedback)

Investigated whether the Save/Cancel buttons' apparent indentation (user
report: "the save and cancel buttons are now indented as well") was
caused by the Password/AWS Profile/Secret Name label-indent commit.
Built the prior commit (`f37d131`) via a detached worktree and compared
column positions directly — Save/Cancel already rendered 2 columns right
of Name's own column *before* that commit too (`tview.Form`'s button-
width calculation reserves `label+4` cells per button, independent of
any item's `SetFormAttributes` label width — confirmed by reading
`form.go`'s button-positioning code, which never reads `maxLabelWidth`).
Not a regression from the label-indent change; not something the public
`tview.Button`/`tview.Form` API exposes a way to remove either. Reported
this finding rather than pretending to fix an unrelated, unreproducible
"bug."

## Full-width section headers (added per user feedback, second round)

Went deeper on the Save/Cancel question by reading `Button.Draw()`
directly: `printWithStyle(screen, b.text, x, y, 0, width, AlignCenter,
style, true)` — the button's *box* starts at the same `x` as every
field (confirmed: `Form.Draw()` never advances `x` for a vertical
form's items, so `positions[buttonIndex].x` inherits the same unchanged
`startX`), but its *text* is centered within a box `label+4` cells wide
(`buttonWidths[index] = TaggedStringWidth(button.GetLabel()) + 4`, also
hardcoded in `form.go`). Both the `+4` width and the `AlignCenter` are
compile-time constants inside `tview` with no exposed setter — the only
way to make the button's *text* start flush left would be to stop using
`tview.Form`'s built-in `AddButton`/`Button` entirely, which every
`AddButton`-using dialog in this codebase relies on identically. Not
attempted without the user's explicit go-ahead, given the blast radius
(app-wide, not scoped to this one dialog) and the payoff (a 2-character
cosmetic gap).

Also asked to make the section headers span the modal's full width.
`tview.Form.AddTextView` can't do both flush-left *and* full-width at
once: `TextView.SetFormAttributes` (called by every `Form.Draw()` pass)
unconditionally sets `t.labelWidth` to the form's shared
`maxLabelWidth`, and `TextView`'s own renderer (`textview.go`,
`printWithStyle` call around line 1025) always reserves that many cells
before drawing body text — so body text lands indented to the value
column, while label text is capped/truncated to that same column's
width. Resolved by writing a minimal custom `tview.FormItem`
(`sectionHeaderItem`, `sectionheader.go`) instead: `SetFormAttributes`
still receives the shared `labelWidth` but discards it, and `Draw()`
computes the header text length itself from `GetInnerRect()`'s actual
width at draw time (via `strings.Repeat("─", pad)` + the exported
`tview.Print`), so it always spans the item's true full width — flush
left and edge-to-edge simultaneously, and it adapts automatically if
the overlay is ever resized. `Focus()` replicates
`TextView.Focus()`'s exact non-scrollable trick (call the
`SetFinishedFunc` handler with a negative key instead of taking real
focus) so Tab-skipping behavior is unchanged.

Sections implemented via `tview.Form.AddTextView(label, "", 0, 1,
false, false)`: scrollable=false makes a Form-embedded `TextView`
non-focusable (`TextView.Focus()` special-cases `!scrollable` by
immediately replaying the last Tab/Backtab instead of stopping there —
confirmed by reading `textview.go`). Put the header text in the
TextView's own *label* slot, not its body text — `Form.Draw()` reserves
the same `maxLabelWidth` column for every item's body regardless of that
item's own label length, so body text would render indented to the
value column; label text, by contrast, always starts at the row's own
left edge, matching every other field's label.

This forced two structural changes since Backend/Name were previously
looked up by fixed index (`GetFormItem(0)`/`GetFormItem(1)`), which no
longer hold once headers occupy indices 0 and 2:
- Every fixed-index lookup in `connections.go` (`NewConnEditor`,
  `Show`, `save`, `ApplyPalette`) switched to `GetFormItemByLabel`. This
  incidentally fixes a long-standing dead-code bug in `ApplyPalette`
  (`GetFormItem(2)` had been silently targeting the wrong item since an
  earlier structural move, per its own preserved comment — now
  `GetFormItemByLabel("Backend")`, correctly restyling it on a live
  theme switch).
- `NewConnEditor` no longer duplicates the tail-field chain (Broker
  Name/URL/Auth header/Username/Authentication Mode/Password) — it
  builds only the static prefix (General header, Name, Destination
  header, Backend) directly, then calls `rebuildTail("jolokia")` once to
  build the rest, reusing that logic instead of maintaining two copies.
  `rebuildTail`'s own wipe-loop boundary changed from a hardcoded `2` to
  the new `staticPrefixItemCount = 4` constant.
- `Show()` now explicitly calls `ce.form.SetFocus(ce.form.GetFormItemIndex("Name"))`
  before showing the page. Needed because `tview.Form` remembers
  `focusedElement` across `Show()` calls and defaults to index `0` on a
  brand-new instance — now the non-focusable "General" header. Verified
  via `textview.go` that landing there isn't fatal (`TextView.Focus()`
  replays the last Tab/Backtab), but there is no "last" key yet on a
  fresh editor, so focus would otherwise appear stuck there on first
  open; forcing it onto Name sidesteps relying on that fallback.

Renames: "Password Source" → "Authentication Mode"; "Password Secret
Name" → "Secret Name" (`labelSecretName` constant, was
`labelPasswordSecretName`). The save-validation error message's wording
updated to match ("...when Authentication Mode is AWS Secret...").

`app.go`'s `connEditorOverlay` fixed height (`ui.Centered(...)`)
increased from 20 to 28 — discovered via live verification that the
taller sectioned form (11 items in the jolokia+AWS-Secret worst case, up
from 7) was clipped at the old height, hiding AWS Profile/Secret
Name/Save/Cancel entirely below the box's bottom border. Recomputed
using the same "border+padding (4) + items×2 + button row (1)" budget
already documented in that file's existing comment convention.

Tests added in `connections_test.go`: `TestConnEditorSectionHeadersPresentInOrder`
(each header sits directly above the field it groups),
`TestConnEditorSectionHeadersAreNotFocusable` (Tab from Name lands on
Backend, skipping the Destination header — simulated via
`Form.Focus(delegate)` + a direct `InputHandler()` Tab keypress, since
`Form.SetFocus` alone doesn't wire the `SetFinishedFunc` callback a real
Tab needs to propagate), and `TestConnEditorFormItemCount` (pins the
11-item worst case so a future field addition that needs another
overlay-height bump doesn't silently start clipping instead).

## `tui/config.example.yaml`

Update the `passwordSecret` comment block to document
`passwordSecretAWSProfile` as a required companion field, and update
both commented-out examples (`local` and `aws-staging`) to include it.

## Manual verification

This one's genuinely testable live without needing a real expired SSO
session (unlike the two prior AWS-auth changes) — per the `verify-live`
skill: create a connection with `passwordSecret`/`passwordSecretAWSProfile`
set to a real secret/profile, confirm it resolves correctly; switch the
*global* AWS profile (Settings → AWS Profiles) and confirm the
connection's queues still load fine (proving it's no longer affected);
try saving the connection editor with AWS Secret selected and the new
field blank, confirm the validation message appears and nothing saves.
