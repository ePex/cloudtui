package awsauth

import (
	"errors"
	"fmt"
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

	err := Login(t.Context(), "some-profile")
	if err == nil {
		t.Fatal("Login() error = nil, want non-nil when aws CLI isn't on PATH")
	}
}
