package awsauth

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/ssocreds"

	"github.com/ePex/cloudtui/tui/internal/awsprofile"
)

func TestNeedsReauth(t *testing.T) {
	invalidToken := &ssocreds.InvalidTokenError{}
	wrappedInvalidToken := fmt.Errorf("get identity: get credentials: %w", invalidToken)
	ssoSessionExpired := errors.New("cached SSO token is expired, or not present, and cannot be refreshed")
	accessDenied := errors.New("operation error SSM: GetParametersByPath, AccessDeniedException")

	cases := []struct {
		name     string
		err      error
		authType awsprofile.AuthType
		want     bool
	}{
		{"nil error", nil, awsprofile.AuthSSO, false},
		{"invalid token + SSO", invalidToken, awsprofile.AuthSSO, true},
		{"wrapped invalid token + SSO", wrappedInvalidToken, awsprofile.AuthSSO, true},
		{"sso-session expired + SSO", ssoSessionExpired, awsprofile.AuthSSO, true},
		{"invalid token + assume-role", invalidToken, awsprofile.AuthAssumeRole, false},
		{"invalid token + static-keys", invalidToken, awsprofile.AuthStaticKeys, false},
		{"invalid token + credential-process", invalidToken, awsprofile.AuthCredentialProcess, false},
		{"invalid token + unknown", invalidToken, awsprofile.AuthUnknown, false},
		{"unrelated error + SSO", accessDenied, awsprofile.AuthSSO, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := NeedsReauth(c.err, c.authType); got != c.want {
				t.Errorf("NeedsReauth(%v, %v) = %v, want %v", c.err, c.authType, got, c.want)
			}
		})
	}
}

// missingSSOTokenError returns the real SDK's error for an sso-session
// profile that has never logged in: its token cache file doesn't exist.
// Built locally from a temp ~/.aws/config, no network.
func missingSSOTokenError(t *testing.T) error {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home) // the SDK's home directory on Windows
	configFile := filepath.Join(home, "config")
	err := os.WriteFile(configFile, []byte(`[profile example-sso]
sso_session = example
sso_account_id = 111111111111
sso_role_name = ExampleRole
region = eu-central-1

[sso-session example]
sso_start_url = https://example.invalid/start
sso_region = eu-central-1
`), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadDefaultConfig(t.Context(),
		config.WithSharedConfigProfile("example-sso"),
		config.WithSharedConfigFiles([]string{configFile}),
		config.WithSharedCredentialsFiles([]string{filepath.Join(home, "credentials")}),
	)
	if err != nil {
		t.Fatalf("loading the example profile: %v", err)
	}
	_, err = cfg.Credentials.Retrieve(t.Context())
	if err == nil {
		t.Fatal("retrieving credentials without a token cache succeeded, want the SDK's error")
	}
	return fmt.Errorf("revealing secret %q: %w", "example/secret", err)
}

// TestNeedsReauthMissingSSOTokenFile: a profile that never logged in (or
// whose token cache was cleared) gets the same login-and-retry as an
// expired token — but only for an SSO profile.
func TestNeedsReauthMissingSSOTokenFile(t *testing.T) {
	err := missingSSOTokenError(t)

	if !NeedsReauth(err, awsprofile.AuthSSO) {
		t.Errorf("NeedsReauth(%v, SSO) = false, want true", err)
	}
	if NeedsReauth(err, awsprofile.AuthStaticKeys) {
		t.Errorf("NeedsReauth(%v, static-keys) = true, want false", err)
	}
}

func TestLoginAWSCLINotOnPath(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	err := Login(t.Context(), "some-profile", nil)
	if err == nil {
		t.Fatal("Login() error = nil, want non-nil when aws CLI isn't on PATH")
	}
}

// realCLIDeviceCodeOutput is the exact text (confirmed against the
// installed AWS CLI's own source,
// awscli/customizations/sso/utils.py's OpenBrowserHandler) that `aws
// sso login` prints when opening the browser for the device-
// authorization flow.
const realCLIDeviceCodeOutput = `Attempting to automatically open the SSO authorization page in your default browser.
If the browser does not open or you wish to use a different device to authorize this request, open the following URL:

https://device.sso.us-east-1.amazonaws.com/

Then enter the code:

WDJB-MJHT
`

func TestScanForDeviceCodeParsesRealCLIOutputShape(t *testing.T) {
	var out bytes.Buffer
	var gotCode, gotURL string
	calls := 0
	scanForDeviceCode(strings.NewReader(realCLIDeviceCodeOutput), &out, func(code, url string) {
		calls++
		gotCode, gotURL = code, url
	})

	if calls != 1 {
		t.Fatalf("onCode called %d times, want 1", calls)
	}
	if gotCode != "WDJB-MJHT" {
		t.Errorf("code = %q, want %q", gotCode, "WDJB-MJHT")
	}
	if gotURL != "https://device.sso.us-east-1.amazonaws.com/" {
		t.Errorf("url = %q, want %q", gotURL, "https://device.sso.us-east-1.amazonaws.com/")
	}
	if out.String() != realCLIDeviceCodeOutput {
		t.Errorf("captured output = %q, want it to preserve the input verbatim", out.String())
	}
}

func TestScanForDeviceCodeNilCallbackSafe(t *testing.T) {
	var out bytes.Buffer
	// Must not panic with onCode == nil, even when the shape matches.
	scanForDeviceCode(strings.NewReader(realCLIDeviceCodeOutput), &out, nil)
}

func TestScanForDeviceCodeNoMatchNeverCallsOnCode(t *testing.T) {
	var out bytes.Buffer
	calls := 0
	scanForDeviceCode(strings.NewReader("some unrelated CLI output\nwith no matching anchors\n"), &out, func(string, string) { calls++ })
	if calls != 0 {
		t.Errorf("onCode called %d times, want 0", calls)
	}
}

// TestLoginStreamsDeviceCodeBeforeCompleting is a real end-to-end test
// of Login's subprocess-streaming behavior: a fake executable named
// "aws" (a POSIX shell script printing the exact confirmed CLI output)
// on a temp-dir PATH, standing in for the real AWS CLI. Skipped on
// Windows — the fake script needs a POSIX shebang; the
// platform-independent scanForDeviceCode tests above already cover the
// parsing logic itself everywhere.
func TestLoginStreamsDeviceCodeBeforeCompleting(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake aws script needs a POSIX shebang")
	}

	dir := t.TempDir()
	script := filepath.Join(dir, "aws")
	var script2 bytes.Buffer
	script2.WriteString("#!/bin/sh\n")
	for _, line := range strings.Split(strings.TrimRight(realCLIDeviceCodeOutput, "\n"), "\n") {
		script2.WriteString("echo '" + line + "'\n")
	}
	if err := os.WriteFile(script, script2.Bytes(), 0o755); err != nil {
		t.Fatalf("WriteFile(fake aws script) error = %v", err)
	}
	t.Setenv("PATH", dir)

	var gotCode, gotURL string
	err := Login(t.Context(), "some-profile", func(code, url string) {
		gotCode, gotURL = code, url
	})
	if err != nil {
		t.Fatalf("Login() error = %v, want nil", err)
	}
	if gotCode != "WDJB-MJHT" || gotURL != "https://device.sso.us-east-1.amazonaws.com/" {
		t.Errorf("onCode(%q, %q), want (%q, %q)", gotCode, gotURL, "WDJB-MJHT", "https://device.sso.us-east-1.amazonaws.com/")
	}
}

// TestLoginCancelReturnsDespiteAChildHoldingOutput: when Login's context
// is cancelled (quitting cloudtui), the CLI is killed, but a child it
// started may still hold the output pipe open. Login must return
// anyway, not wait for that child, or quitting hangs (found live with a
// fake CLI).
func TestLoginCancelReturnsDespiteAChildHoldingOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake aws script needs a POSIX shebang")
	}

	// The fake CLI starts a child that inherits its output, records the
	// child's PID once it exists, and waits for it.
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "child.pid")
	script := "#!/bin/sh\n/bin/sleep 8 &\necho $! > '" + pidFile + "'\nwait\n"
	if err := os.WriteFile(filepath.Join(dir, "aws"), []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile(fake aws script) error = %v", err)
	}
	t.Setenv("PATH", dir)

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- Login(ctx, "some-profile", nil) }()
	childPID := waitForPIDFile(t, pidFile)
	t.Cleanup(func() {
		if p, err := os.FindProcess(childPID); err == nil {
			_ = p.Kill() // the orphaned child; Login no longer waits for it
		}
	})
	cancel()

	select {
	case err := <-done:
		if err == nil {
			t.Error("Login() error = nil after cancelling, want an error")
		}
	case <-time.After(4 * time.Second):
		t.Fatal("Login didn't return after its context was cancelled (a child still holds its output)")
	}
}

// waitForPIDFile waits for path to hold a PID and returns it, failing
// the test if it doesn't appear.
func waitForPIDFile(t *testing.T, path string) int {
	t.Helper()
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		if b, err := os.ReadFile(path); err == nil {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(b))); err == nil {
				return pid
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("the fake aws script never wrote %s", path)
	return 0
}
