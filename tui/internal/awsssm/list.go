package awsssm

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

// List fetches every parameter under path (recursively) for profile,
// paginating through all results. Requesting WithDecryption: false is
// safe and sufficient here — String/StringList values come back as real
// plaintext regardless (decryption is meaningless for them), while
// SecureString values come back as ciphertext, never plaintext, without
// WithDecryption: true. See buildParameters for what happens to each.
func List(ctx context.Context, profile, path string) ([]Parameter, error) {
	client, err := newClient(ctx, profile)
	if err != nil {
		return nil, err
	}

	var raw []types.Parameter
	var nextToken *string
	for {
		out, err := client.GetParametersByPath(ctx, &ssm.GetParametersByPathInput{
			Path:           aws.String(path),
			Recursive:      aws.Bool(true),
			WithDecryption: aws.Bool(false),
			NextToken:      nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("listing parameters under %q: %w", path, err)
		}
		raw = append(raw, out.Parameters...)
		if out.NextToken == nil {
			break
		}
		nextToken = out.NextToken
	}
	return buildParameters(raw), nil
}

// buildParameters converts raw AWS parameters into Parameter, discarding
// SecureString ciphertext (never returned to callers unrevealed) and
// sorting by name for stable, predictable display. Split out from List so
// this — the part with actual logic to get wrong — is unit-testable
// without a real AWS call.
func buildParameters(raw []types.Parameter) []Parameter {
	out := make([]Parameter, 0, len(raw))
	for _, p := range raw {
		pt := ParameterType(p.Type)
		value := ""
		if pt != TypeSecureString {
			value = aws.ToString(p.Value)
		}
		var lastModified time.Time
		if p.LastModifiedDate != nil {
			lastModified = *p.LastModifiedDate
		}
		out = append(out, Parameter{
			Name:         aws.ToString(p.Name),
			Type:         pt,
			Value:        value,
			LastModified: lastModified,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
