package awssecrets

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
)

// List fetches every secret's metadata for profile, paginating through
// all results. ListSecrets never returns a value — Secrets Manager has no
// list-adjacent call that does, unlike ssm.GetParametersByPath — so
// Secret never carries one; see Reveal for the only way to get a value.
func List(ctx context.Context, profile string) ([]Secret, error) {
	client, err := newClient(ctx, profile)
	if err != nil {
		return nil, err
	}

	var raw []types.SecretListEntry
	var nextToken *string
	for {
		out, err := client.ListSecrets(ctx, &secretsmanager.ListSecretsInput{
			NextToken: nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("listing secrets: %w", err)
		}
		raw = append(raw, out.SecretList...)
		if out.NextToken == nil {
			break
		}
		nextToken = out.NextToken
	}
	return buildSecrets(raw), nil
}

// buildSecrets converts raw AWS secret entries into Secret, sorting by
// name for stable, predictable display. Split out from List so this — the
// part with actual logic to get wrong — is unit-testable without a real
// AWS call.
func buildSecrets(raw []types.SecretListEntry) []Secret {
	out := make([]Secret, 0, len(raw))
	for _, s := range raw {
		var lastChanged time.Time
		if s.LastChangedDate != nil {
			lastChanged = *s.LastChangedDate
		}
		out = append(out, Secret{
			Name:            aws.ToString(s.Name),
			ARN:             aws.ToString(s.ARN),
			LastChanged:     lastChanged,
			RotationEnabled: aws.ToBool(s.RotationEnabled),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
