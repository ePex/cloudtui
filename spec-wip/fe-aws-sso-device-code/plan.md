# Implementation plan

## `tui/internal/awsauth/awsauth.go`

`Login` rewritten to stream instead of buffer:

```go
func Login(ctx context.Context, profile string, onCode func(code, url string)) error {
	if _, err := exec.LookPath("aws"); err != nil {
		return fmt.Errorf("aws CLI not found on PATH — install it, or run `aws sso login --profile %s` after authenticating another way: %w", profile, err)
	}

	cmd := exec.CommandContext(ctx, "aws", "sso", "login", "--profile", profile)
	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw

	var output bytes.Buffer
	scanDone := make(chan struct{})
	go func() {
		defer close(scanDone)
		scanForDeviceCode(pr, &output, onCode)
	}()

	runErr := cmd.Run()
	pw.Close()
	<-scanDone

	if runErr != nil {
		return fmt.Errorf("aws sso login --profile %s: %w\n%s", profile, runErr, output.String())
	}
	return nil
}

func scanForDeviceCode(r io.Reader, out *bytes.Buffer, onCode func(code, url string)) {
	scanner := bufio.NewScanner(r)
	var url, code string
	var wantURL, wantCode, notified bool
	for scanner.Scan() {
		line := scanner.Text()
		out.WriteString(line)
		out.WriteByte('\n')

		trimmed := strings.TrimSpace(line)
		switch {
		case wantURL && trimmed != "":
			url = trimmed
			wantURL = false
		case wantCode && trimmed != "":
			code = trimmed
			wantCode = false
		case strings.Contains(line, "open the following URL:"):
			wantURL = true
		case strings.Contains(line, "Then enter the code:"):
			wantCode = true
		}

		if !notified && onCode != nil && url != "" && code != "" {
			onCode(code, url)
			notified = true
		}
	}
}
```

New imports: `bufio`, `bytes`, `io`. `pr`/`pw` (an `io.Pipe`) let the
scanning goroutine consume output *as it's written*, unlike
`CombinedOutput()` which only returns after the process exits — the
whole point, since `aws sso login` blocks until the user finishes the
browser flow, potentially long after the code/URL are already printed.
`out` still captures everything verbatim, so a failure's error message
is unchanged (full CLI output included). Anchoring on the literal
substrings confirmed against the real AWS CLI source (not a code-format
regex) means a future AWS CLI wording tweak degrades gracefully — `Login`
still works, `onCode` just doesn't fire.

## `tui/internal/awsauth/retry.go`

`WithReauth` gains an `onCode` parameter, positioned next to `onReauth`
(both are re-auth-lifecycle callbacks) and threaded straight to `login`:

```go
func WithReauth[T any](
	ctx context.Context,
	profile string,
	authType awsprofile.AuthType,
	login func(ctx context.Context, profile string, onCode func(code, url string)) error,
	onReauth func(),
	onCode func(code, url string),
	call func(ctx context.Context) (T, error),
) (T, error) {
	result, err := call(ctx)
	if err == nil || !NeedsReauth(err, authType) {
		return result, err
	}
	if onReauth != nil {
		onReauth()
	}
	if loginErr := login(ctx, profile, onCode); loginErr != nil {
		return result, fmt.Errorf("%w (re-auth attempt failed: %v)", err, loginErr)
	}
	return call(ctx)
}
```

## `tui/internal/ui/viewhost.go` + `tui/internal/app/viewhost.go`

`AWSSSOLogin`'s signature grows the same `onCode` parameter in both the
interface and `*App`'s implementation (a straight passthrough to
`a.awsSSOLogin`, whose field type in `app.go` updates to match —- no
other change there, since it's assigned `awsauth.Login` directly).

## `tui/internal/ui/reauth.go`

`ShowReauthWaiting` gains a `msg string` parameter — previously 0-arg
and implicitly always the same fixed text; now the caller supplies it,
so the *same* call can update an already-shown message once the code/URL
arrive, not just set it once at the start:

```go
type ReauthStatusShower interface {
	ShowReauthWaiting(msg string)
	ShowReauthDone()
}
```

`QueuesView.ShowReauthWaiting` becomes a thin passthrough to `showStatus`
(the hardcoded message text moves to the caller, `app.go`).

## `tui/internal/app/app.go`

`showReauthWaiting` gains the same `msg string` parameter, passed
through to the active view or the status-bar fallback unchanged:

```go
func (a *App) showReauthWaiting(msg string) {
	if av, ok := a.activeView().(ui.ReauthStatusShower); ok {
		av.ShowReauthWaiting(msg)
	} else {
		a.SetStatus(msg)
	}
}
```

The `NewSecretResolver` construction site gains a 6th argument, the new
`onCode` callback, which re-shows the *same* message with the code/url
appended:

```go
const awsSSOReauthWaitingMsg = "AWS SSO session expired — opening browser to log in..."

a.secretResolver = secretbackend.NewSecretResolver(a.revealSecret, a.AWSAuthTypeFor, a.AWSSSOLogin,
	func() { a.QueueUpdateDraw(func() { a.showReauthWaiting(awsSSOReauthWaitingMsg) }) },
	func() { a.QueueUpdateDraw(a.showReauthDone) },
	func(code, url string) {
		a.QueueUpdateDraw(func() {
			a.showReauthWaiting(fmt.Sprintf("%s Verify code %s at %s", awsSSOReauthWaitingMsg, code, url))
		})
	},
)
```

## `tui/internal/queue/secretbackend/secretbackend.go`

`SecretResolver` gains an `onCode func(code, url string)` field;
`NewSecretResolver` gains the matching parameter; `login`'s field type
updates to accept `onCode` and `Resolve`'s `loginThenNotifyDone` wrapper
passes it through to `r.login(ctx, profile, onCode)` before firing
`onReauthDone` as it already does. `Resolve`'s `awsauth.WithReauth` call
gains `r.onCode` in the new parameter slot.

## The five remaining `awsauth.WithReauth` call sites

`ssmparams.go`, `secrets.go`, `logs.go`, `codepipelinedetail.go`,
`codepipelinelist.go` each get the same two changes: extract their
existing inline "AWS SSO session expired — opening browser to log
in..." literal into a local `const` (per-file — not a new shared
package-level constant; each file already independently duplicates this
literal today, and consolidating that is a separate, unrelated cleanup
not bundled into this fix) so the "waiting" and "waiting-with-code"
messages can't drift out of sync within that file, then add the new
`onCode` argument:

```go
const reauthWaitingMsg = "AWS SSO session expired — opening browser to log in..."

// ...
params, err := awsauth.WithReauth(ctx, profile, authType, pv.host.AWSSSOLogin,
	func() {
		pv.host.QueueUpdateDraw(func() {
			pv.showStatus(reauthWaitingMsg)
		})
	},
	func(code, url string) {
		pv.host.QueueUpdateDraw(func() {
			pv.showStatus(fmt.Sprintf("%s Verify code %s at %s", reauthWaitingMsg, code, url))
		})
	},
	func(ctx context.Context) ([]awsssm.Parameter, error) {
		return pv.host.ListParameters(ctx, profile, path)
	},
)
```

(Exact receiver/type names vary per file; shape is identical across all
five.)

## `tui/internal/view/pipelinewatcher.go`

Its existing `awsauth.WithReauth` call already passes `onReauth: nil`
("no in-progress status message — this isn't a visible search view") —
gains `onCode: nil` in the new slot for the same reason, no other change.

## Test updates

- `tui/internal/awsauth/awsauth_test.go`: `TestLoginAWSCLINotOnPath`
  passes `nil` for the new parameter. New:
  `TestScanForDeviceCodeParsesRealCLIOutputShape` (feeds the exact
  confirmed AWS CLI output text, asserts `onCode` fires once with the
  right code/url, and that the captured buffer preserves the input
  verbatim), `TestScanForDeviceCodeNilCallbackSafe`,
  `TestScanForDeviceCodeNoMatchNeverCallsOnCode`. Also
  `TestLoginStreamsDeviceCodeBeforeCompleting`: a real end-to-end test of
  `Login` itself, using a fake executable named `aws` (a POSIX shell
  script printing the same confirmed output) placed on a temp-dir `PATH`
  — `t.Skip` on Windows (the fake script needs a POSIX shebang; the
  platform-independent `scanForDeviceCode` tests already cover the
  parsing logic itself everywhere).
- `tui/internal/awsauth/retry_test.go`: existing `login` stubs' signatures
  updated (extra unused `onCode` parameter); `WithReauth` calls updated
  with a `nil`/no-op `onCode` argument. New:
  `TestWithReauthPassesOnCodeThroughToLogin` — captures the `onCode`
  `login` receives and confirms invoking it calls through to the one
  `WithReauth` was given.
- `tui/internal/queue/secretbackend/secretbackend_test.go`:
  `newTestResolver` and the three SSO-specific tests updated for the new
  `login` signature and `NewSecretResolver`'s extra `onCode` parameter
  (`nil` throughout — this package's own new-behavior test coverage is
  about the `onReauth`/`onReauthDone` wiring already covered; `onCode`
  passthrough itself is covered once, generically, by
  `TestWithReauthPassesOnCodeThroughToLogin` above and the `app_test.go`
  test below, not re-proven per-package).
- `tui/internal/app/app_test.go`: `showReauthWaiting` calls updated to
  pass an explicit message string. New:
  `TestShowReauthWaitingIncludesDeviceCodeAndURLWhenProvided` — calls
  `a.showReauthWaiting("... Verify code X at Y")` with `queues` active,
  asserts the exact combined string lands in the table.
- `tui/internal/view/testfake_test.go`: `fakeViewHost.awsSSOLoginFn`'s
  signature and the `AWSSSOLogin` method both updated for the new
  `onCode` parameter (passed through when the fn is set, ignored
  otherwise — no behavior change for any test not specifically about
  this).
- Each of the five remaining view test files
  (`ssmparams_test.go`/`secrets_test.go`/`logs_test.go`/
  `codepipelinedetail_test.go`/`codepipelinelist_test.go`): one new test
  per file confirming that view's `onCode` callback updates its status
  message to include the code/url (mirroring the shape of each file's
  existing `TestXShowStatusRendersMessage` test, which continues to
  cover the unchanged "waiting" message).

## Manual verification

Same honest caveat as the secretbackend SSO re-auth bugfix: a genuinely
expired real SSO session isn't available in this environment to drive
end-to-end. `TestLoginStreamsDeviceCodeBeforeCompleting`'s fake-`aws`-
script approach is the closest thing to a live check achievable here —
it exercises the real subprocess-streaming code path, just not against
the real AWS CLI. If a real expired SSO session is available when this
lands, worth a real live check per `tasks.md`, but the unit tests above
are the evidence of record.
