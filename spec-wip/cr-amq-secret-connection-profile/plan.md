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
   currently in (checked via whether "Secret AWS Profile" currently
   exists, not via a separately-tracked flag):

   ```go
   func (ce *ConnEditor) setPasswordField(sourceIdx int) {
   	f := ce.form
   	currentCount := 1
   	if _, ok := f.GetFormItemByLabel("Secret AWS Profile").(*tview.InputField); ok {
   		currentCount = 2
   	}
   	for i := 0; i < currentCount; i++ {
   		f.RemoveFormItem(f.GetFormItemCount() - 1)
   	}
   	if sourceIdx == 1 {
   		f.AddInputField("Password Secret (AWS)", "", 30, nil, nil)
   		f.AddInputField("Secret AWS Profile", "", 20, nil, nil)
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
   if item, ok := f.GetFormItemByLabel("Secret AWS Profile").(*tview.InputField); ok {
   	secretProfile = item.GetText()
   }
   // ... (existing wipe loop unchanged)
   f.AddDropDown("Password Source", []string{"Plain", "AWS Secret"}, sourceIdx, nil)
   if sourceIdx == 1 {
   	f.AddInputField("Password Secret (AWS)", passwordOrSecret, 30, nil, nil)
   	f.AddInputField("Secret AWS Profile", secretProfile, 20, nil, nil)
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
- `tui/internal/dialog/connections_test.go`: new tests for the "Secret
  AWS Profile" field mirroring the existing "Password Secret (AWS)"
  field's own test coverage — appears/disappears with Password Source,
  survives a Backend toggle round-trip (`rebuildTail`), round-trips
  through `Show()`→`save()`, and the new required-field validation
  (empty profile + AWS Secret source → status message, no save).

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
