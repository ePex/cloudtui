package awssecrets

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

// Reveal fetches and decrypts a single secret's current (AWSCURRENT)
// value. Callers should only invoke this in direct response to an
// explicit user action (a "reveal" key press), never as part of routine
// listing. isBinary is true when the secret has no SecretString (a
// SecretBinary-only secret), in which case value is empty.
func Reveal(ctx context.Context, profile, name string) (value string, isBinary bool, err error) {
	client, err := newClient(ctx, profile)
	if err != nil {
		return "", false, err
	}
	out, err := client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(name),
	})
	if err != nil {
		return "", false, fmt.Errorf("revealing secret %q: %w", name, err)
	}
	return extractValue(out)
}

// extractValue picks out the SecretString-vs-SecretBinary branch. Split
// out from Reveal so this — the part with actual logic to get wrong — is
// unit-testable against a hand-built GetSecretValueOutput, no real AWS
// call needed.
func extractValue(out *secretsmanager.GetSecretValueOutput) (value string, isBinary bool, err error) {
	if out.SecretString != nil {
		return *out.SecretString, false, nil
	}
	if out.SecretBinary != nil {
		return "", true, nil
	}
	return "", false, fmt.Errorf("secret has neither a string nor a binary value")
}
