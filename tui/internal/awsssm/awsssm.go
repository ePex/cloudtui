// Package awsssm provides read-only access to AWS Systems Manager
// Parameter Store: listing parameters under a path, and revealing a
// SecureString parameter's decrypted value on demand.
//
// Unlike internal/awsprofile (which only parses local config files), this
// package makes real AWS API calls and needs credentials to actually
// resolve — via the given profile name, through the standard AWS SDK
// credential chain (SSO, credential_process, static keys, ...). A cached
// SSO token being expired can trigger a real browser-based login flow;
// that's AWS SDK behavior this package doesn't control.
package awsssm

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

// ParameterType mirrors AWS's parameter type values.
type ParameterType string

const (
	TypeString       ParameterType = "String"
	TypeStringList   ParameterType = "StringList"
	TypeSecureString ParameterType = "SecureString"
)

// Parameter is one discovered Parameter Store entry. Value is populated
// for String/StringList (returned directly by the list call) and left
// empty for SecureString until Reveal is called for that name — the
// ciphertext GetParametersByPath returns for SecureString without
// decryption is deliberately discarded, never surfaced.
type Parameter struct {
	Name         string
	Type         ParameterType
	Value        string
	LastModified time.Time
}

// newClient builds an ssm.Client authenticated as profile. An empty
// profile is a caller error — the view layer is responsible for checking
// a profile is actually selected before calling in.
func newClient(ctx context.Context, profile string) (*ssm.Client, error) {
	if profile == "" {
		return nil, fmt.Errorf("no AWS profile selected")
	}
	cfg, err := config.LoadDefaultConfig(ctx, config.WithSharedConfigProfile(profile))
	if err != nil {
		return nil, fmt.Errorf("loading AWS config for profile %q: %w", profile, err)
	}
	return ssm.NewFromConfig(cfg), nil
}
