// Package awsauth recognizes when an AWS SDK call failed because the
// active profile's cached SSO token is missing or expired, and drives
// `aws sso login` to fix that in place — the browser-based
// re-authentication AWS's own SDK never performs automatically (only
// the AWS CLI's `aws sso login` command does the device-authorization
// flow and writes the token cache the SDK reads).
package awsauth

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/aws/aws-sdk-go-v2/credentials/ssocreds"

	"github.com/ePex/cloudtui/tui/internal/awsprofile"
)

// NeedsReauth reports whether err indicates the active profile's cached
// AWS SSO token is missing or expired — the specific case `aws sso
// login` fixes. Only ever true for SSO profiles: authType gates it so a
// coincidentally similar error from a non-SSO credential provider is
// never misread as this case.
func NeedsReauth(err error, authType awsprofile.AuthType) bool {
	if err == nil || authType != awsprofile.AuthSSO {
		return false
	}

	var invalidToken *ssocreds.InvalidTokenError
	if errors.As(err, &invalidToken) {
		return true
	}

	// sso-session-style profiles refresh through ssocreds'
	// SSOTokenProvider, which doesn't wrap a typed error the way the
	// legacy sso_start_url provider's InvalidTokenError does — see
	// ssocreds/sso_token_provider.go in aws-sdk-go-v2/credentials.
	return strings.Contains(err.Error(), "cached SSO token is expired, or not present")
}

// Login runs `aws sso login --profile <profile>`, which opens the
// user's default browser to complete the SSO device-authorization flow
// and writes the resulting token to ~/.aws/sso/cache — the same cache
// aws-sdk-go-v2's SSO credential provider reads. Blocks until the AWS
// CLI process exits or ctx is canceled.
//
// onCode, if non-nil, is called exactly once with the device
// verification code and URL the CLI prints as soon as the browser
// opens — parsed live from the process's output (not buffered until it
// exits, since the command blocks until the user finishes the browser
// flow, well after the code/URL are printed). Matching this code
// against what the browser page shows is the whole point of the
// device-authorization flow (RFC 8628) — it's how the user confirms the
// page that just opened is legitimately theirs to approve. Parses by
// anchoring on literal substrings confirmed against the real AWS CLI
// source (awscli/customizations/sso/utils.go's OpenBrowserHandler), not
// a code-format regex, so a future CLI wording tweak just means onCode
// doesn't fire rather than Login misbehaving.
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

// scanForDeviceCode copies r's lines into out (preserving Login's
// existing behavior of including the full CLI output in a failure's
// error message) while watching for the URL and code the AWS CLI prints
// when opening the browser (see Login's doc comment). Calls onCode
// exactly once, as soon as both have been seen — order matches the
// CLI's own output (the URL line, then further down, the code line).
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
