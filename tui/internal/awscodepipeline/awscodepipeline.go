// Package awscodepipeline provides read-only access to AWS CodePipeline:
// listing pipelines, and getting a pipeline's current per-stage status.
//
// Like internal/awsssm and internal/awslogs, this package makes real AWS
// API calls and needs credentials to actually resolve — via the given
// profile name, through the standard AWS SDK credential chain (SSO,
// credential_process, static keys, ...).
package awscodepipeline

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/codepipeline"
)

// Pipeline is one discovered CodePipeline pipeline's metadata.
type Pipeline struct {
	Name    string
	Created time.Time
	Updated time.Time
}

// StageStatus is one stage's current status within a pipeline, from its
// latest execution. Status is one of CodePipeline's
// types.StageExecutionStatus values (Cancelled, InProgress, Failed,
// Stopped, Stopping, Succeeded, Skipped), or "" if the stage has never
// run. PipelineExecutionID identifies which pipeline execution Status
// belongs to — GetPipelineState reports each stage's *last* execution
// independently, so a stage the current execution hasn't reached yet
// still shows a status (possibly Succeeded or Failed) left over from an
// earlier execution. Callers that need to know whether the pipeline is
// still actively running must compare this against the other stages'
// IDs, not just read Status at face value (see
// internal/app/codepipelinewatch.go's pipelineFinished).
type StageStatus struct {
	Name                string
	Status              string
	PipelineExecutionID string
}

// newClient builds a codepipeline.Client authenticated as profile. An
// empty profile is a caller error — the view layer is responsible for
// checking a profile is actually selected before calling in.
func newClient(ctx context.Context, profile string) (*codepipeline.Client, error) {
	if profile == "" {
		return nil, fmt.Errorf("no AWS profile selected")
	}
	cfg, err := config.LoadDefaultConfig(ctx, config.WithSharedConfigProfile(profile))
	if err != nil {
		return nil, fmt.Errorf("loading AWS config for profile %q: %w", profile, err)
	}
	return codepipeline.NewFromConfig(cfg), nil
}
