package awsssm

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

// Reveal fetches and decrypts a single SecureString parameter's value.
// Callers should only invoke this in direct response to an explicit user
// action (a "reveal" key press), never as part of routine listing.
func Reveal(ctx context.Context, profile, name string) (string, error) {
	client, err := newClient(ctx, profile)
	if err != nil {
		return "", err
	}
	out, err := client.GetParameter(ctx, &ssm.GetParameterInput{
		Name:           aws.String(name),
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		return "", fmt.Errorf("revealing parameter %q: %w", name, err)
	}
	if out.Parameter == nil {
		return "", fmt.Errorf("revealing parameter %q: empty response", name)
	}
	return aws.ToString(out.Parameter.Value), nil
}
