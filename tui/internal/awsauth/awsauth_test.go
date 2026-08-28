package awsauth

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

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
