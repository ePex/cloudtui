// Package awsauth recognizes when an AWS SDK call failed because the
// active profile's cached SSO token is missing or expired, and drives
// `aws sso login` to fix that in place — the browser-based
// re-authentication AWS's own SDK never performs automatically (only
// the AWS CLI's `aws sso login` command does the device-authorization
// flow and writes the token cache the SDK reads).
package awsauth

import (
	"context"
	"errors"
	"fmt"
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
func Login(ctx context.Context, profile string) error {
	if _, err := exec.LookPath("aws"); err != nil {
		return fmt.Errorf("aws CLI not found on PATH — install it, or run `aws sso login --profile %s` after authenticating another way: %w", profile, err)
	}
	out, err := exec.CommandContext(ctx, "aws", "sso", "login", "--profile", profile).CombinedOutput()
	if err != nil {
		return fmt.Errorf("aws sso login --profile %s: %w\n%s", profile, err, out)
	}
	return nil
}
