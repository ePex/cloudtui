package awscodepipeline

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/codepipeline/types"
)

func TestBuildPipelinesPopulatesFields(t *testing.T) {
	created := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	updated := time.Date(2026, 1, 3, 3, 4, 5, 0, time.UTC)
	raw := []types.PipelineSummary{
		{Name: aws.String("my-pipeline"), Created: &created, Updated: &updated},
	}

	got := buildPipelines(raw)

	if len(got) != 1 {
		t.Fatalf("buildPipelines() len = %d, want 1", len(got))
	}
	p := got[0]
	if p.Name != "my-pipeline" {
		t.Errorf("Name = %q, want %q", p.Name, "my-pipeline")
	}
	if !p.Created.Equal(created) {
		t.Errorf("Created = %v, want %v", p.Created, created)
	}
	if !p.Updated.Equal(updated) {
		t.Errorf("Updated = %v, want %v", p.Updated, updated)
	}
}

func TestBuildPipelinesHandlesNilTimes(t *testing.T) {
	raw := []types.PipelineSummary{
		{Name: aws.String("my-pipeline")},
	}

	got := buildPipelines(raw)

	if !got[0].Created.IsZero() {
		t.Errorf("Created = %v, want zero value when AWS returns nil", got[0].Created)
	}
	if !got[0].Updated.IsZero() {
		t.Errorf("Updated = %v, want zero value when AWS returns nil", got[0].Updated)
	}
}

func TestBuildPipelinesSortsByName(t *testing.T) {
	raw := []types.PipelineSummary{
		{Name: aws.String("z-pipeline")},
		{Name: aws.String("a-pipeline")},
		{Name: aws.String("m-pipeline")},
	}

	got := buildPipelines(raw)

	want := []string{"a-pipeline", "m-pipeline", "z-pipeline"}
	for i, w := range want {
		if got[i].Name != w {
			t.Errorf("got[%d].Name = %q, want %q", i, got[i].Name, w)
		}
	}
}

func TestBuildPipelinesEmptyInput(t *testing.T) {
	got := buildPipelines(nil)
	if len(got) != 0 {
		t.Errorf("buildPipelines(nil) = %+v, want empty", got)
	}
}
