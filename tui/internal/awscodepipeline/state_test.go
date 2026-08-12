package awscodepipeline

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/codepipeline/types"
)

func TestBuildStageStatusesPopulatesFields(t *testing.T) {
	raw := []types.StageState{
		{
			StageName: aws.String("Build"),
			LatestExecution: &types.StageExecution{
				PipelineExecutionId: aws.String("exec-1"),
				Status:              types.StageExecutionStatusInProgress,
			},
		},
	}

	got := buildStageStatuses(raw)

	if len(got) != 1 {
		t.Fatalf("buildStageStatuses() len = %d, want 1", len(got))
	}
	s := got[0]
	if s.Name != "Build" {
		t.Errorf("Name = %q, want %q", s.Name, "Build")
	}
	if s.Status != "InProgress" {
		t.Errorf("Status = %q, want %q", s.Status, "InProgress")
	}
}

func TestBuildStageStatusesHandlesNilLatestExecution(t *testing.T) {
	raw := []types.StageState{
		{StageName: aws.String("Deploy"), LatestExecution: nil},
	}

	got := buildStageStatuses(raw)

	if got[0].Status != "" {
		t.Errorf("Status = %q, want empty when the stage has never run", got[0].Status)
	}
}

func TestBuildStageStatusesPreservesOrder(t *testing.T) {
	raw := []types.StageState{
		{StageName: aws.String("Source")},
		{StageName: aws.String("Build")},
		{StageName: aws.String("Deploy")},
	}

	got := buildStageStatuses(raw)

	want := []string{"Source", "Build", "Deploy"} // AWS's own order, no re-sort
	for i, w := range want {
		if got[i].Name != w {
			t.Errorf("got[%d].Name = %q, want %q", i, got[i].Name, w)
		}
	}
}

func TestBuildStageStatusesEmptyInput(t *testing.T) {
	got := buildStageStatuses(nil)
	if len(got) != 0 {
		t.Errorf("buildStageStatuses(nil) = %+v, want empty", got)
	}
}
