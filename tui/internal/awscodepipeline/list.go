package awscodepipeline

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/codepipeline"
	"github.com/aws/aws-sdk-go-v2/service/codepipeline/types"
)

// ListPipelines fetches every pipeline's metadata for profile,
// paginating through all results. This is metadata, not a search — same
// cost shape as awslogs.ListLogGroups.
func ListPipelines(ctx context.Context, profile string) ([]Pipeline, error) {
	client, err := newClient(ctx, profile)
	if err != nil {
		return nil, err
	}

	var raw []types.PipelineSummary
	paginator := codepipeline.NewListPipelinesPaginator(client, &codepipeline.ListPipelinesInput{})
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing pipelines: %w", err)
		}
		raw = append(raw, out.Pipelines...)
	}
	return buildPipelines(raw), nil
}

// buildPipelines converts raw AWS pipeline summaries into Pipeline,
// sorting by name for stable, predictable display. Split out from
// ListPipelines so this — the part with actual logic to get wrong — is
// unit-testable without a real AWS call.
func buildPipelines(raw []types.PipelineSummary) []Pipeline {
	out := make([]Pipeline, 0, len(raw))
	for _, p := range raw {
		var created, updated time.Time
		if p.Created != nil {
			created = *p.Created
		}
		if p.Updated != nil {
			updated = *p.Updated
		}
		out = append(out, Pipeline{
			Name:    aws.ToString(p.Name),
			Created: created,
			Updated: updated,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
