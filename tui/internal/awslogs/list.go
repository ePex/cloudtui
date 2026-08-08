package awslogs

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
)

// ListLogGroups fetches every log group's metadata for profile,
// paginating through all results. This is metadata, not a search —
// same cost shape as awsssm.List/awssecrets.List.
func ListLogGroups(ctx context.Context, profile string) ([]LogGroup, error) {
	client, err := newClient(ctx, profile)
	if err != nil {
		return nil, err
	}

	var raw []types.LogGroup
	var nextToken *string
	for {
		out, err := client.DescribeLogGroups(ctx, &cloudwatchlogs.DescribeLogGroupsInput{
			NextToken: nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("listing log groups: %w", err)
		}
		raw = append(raw, out.LogGroups...)
		if out.NextToken == nil {
			break
		}
		nextToken = out.NextToken
	}
	return buildLogGroups(raw), nil
}

// buildLogGroups converts raw AWS log groups into LogGroup, sorting by
// name for stable, predictable display. Split out from ListLogGroups so
// this — the part with actual logic to get wrong — is unit-testable
// without a real AWS call.
func buildLogGroups(raw []types.LogGroup) []LogGroup {
	out := make([]LogGroup, 0, len(raw))
	for _, g := range raw {
		var created time.Time
		if g.CreationTime != nil {
			created = time.UnixMilli(*g.CreationTime)
		}
		var retention int32
		if g.RetentionInDays != nil {
			retention = *g.RetentionInDays
		}
		out = append(out, LogGroup{
			Name:          aws.ToString(g.LogGroupName),
			RetentionDays: retention,
			CreatedAt:     created,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
