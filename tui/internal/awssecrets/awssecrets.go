// Package awssecrets provides read-only access to AWS Secrets Manager:
// listing secrets' metadata, and revealing a secret's decrypted current
// (AWSCURRENT) value on demand.
//
// Unlike internal/awsssm's Parameter Store, ListSecrets never returns a
// value at all — not even ciphertext — so every secret is structurally
// masked until Reveal is called for it. Like awsssm, this package makes
// real AWS API calls and needs credentials to actually resolve — via the
// given profile name, through the standard AWS SDK credential chain (SSO,
// credential_process, static keys, ...). A cached SSO token being expired
// can trigger a real browser-based login flow; that's AWS SDK behavior
// this package doesn't control.
package awssecrets

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

// Secret is one discovered Secrets Manager entry's metadata. A value is
// never present here — GetSecretValue is a separate call, made only via
// Reveal in direct response to an explicit user action.
type Secret struct {
	Name            string
	ARN             string
	LastChanged     time.Time
	RotationEnabled bool
}

// newClient builds a secretsmanager.Client authenticated as profile. An
// empty profile is a caller error — the view layer is responsible for
// checking a profile is actually selected before calling in.
func newClient(ctx context.Context, profile string) (*secretsmanager.Client, error) {
	if profile == "" {
		return nil, fmt.Errorf("no AWS profile selected")
	}
	cfg, err := config.LoadDefaultConfig(ctx, config.WithSharedConfigProfile(profile))
	if err != nil {
		return nil, fmt.Errorf("loading AWS config for profile %q: %w", profile, err)
	}
	return secretsmanager.NewFromConfig(cfg), nil
}
