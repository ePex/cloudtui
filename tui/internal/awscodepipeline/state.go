package awscodepipeline

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/codepipeline"
	"github.com/aws/aws-sdk-go-v2/service/codepipeline/types"
)

// GetPipelineState fetches pipelineName's current per-stage status —
// the "stage transition" data the caller diffs against a previous poll
// to detect a transition (see internal/app/codepipelinewatch.go).
func GetPipelineState(ctx context.Context, profile, pipelineName string) ([]StageStatus, error) {
	client, err := newClient(ctx, profile)
	if err != nil {
		return nil, err
	}

	out, err := client.GetPipelineState(ctx, &codepipeline.GetPipelineStateInput{
		Name: aws.String(pipelineName),
	})
	if err != nil {
		return nil, fmt.Errorf("getting pipeline state for %q: %w", pipelineName, err)
	}
	return buildStageStatuses(out.StageStates), nil
}

// buildStageStatuses converts raw AWS stage states into StageStatus, in
// the order AWS returns them (a pipeline's own stage order, not
// re-sorted — unlike ListPipelines' alphabetical listing, stage order
// here is meaningful: it's the pipeline's actual execution sequence).
// A stage with no LatestExecution (never run) gets an empty Status, not
// a zero-value struct field access. Split out from GetPipelineState so
// this — the part with actual logic to get wrong — is unit-testable
// without a real AWS call.
func buildStageStatuses(raw []types.StageState) []StageStatus {
	out := make([]StageStatus, 0, len(raw))
	for _, s := range raw {
		var status string
		if s.LatestExecution != nil {
			status = string(s.LatestExecution.Status)
		}
		out = append(out, StageStatus{
			Name:   aws.ToString(s.StageName),
			Status: status,
		})
	}
	return out
}
