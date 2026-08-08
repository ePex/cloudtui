package awslogs

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
)

func TestBuildLogGroupsPopulatesFields(t *testing.T) {
	created := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	raw := []types.LogGroup{
		{
			LogGroupName:    aws.String("/aws/lambda/foo"),
			CreationTime:    aws.Int64(created.UnixMilli()),
			RetentionInDays: aws.Int32(14),
		},
	}

	got := buildLogGroups(raw)

	if len(got) != 1 {
		t.Fatalf("buildLogGroups() len = %d, want 1", len(got))
	}
	g := got[0]
	if g.Name != "/aws/lambda/foo" {
		t.Errorf("Name = %q, want %q", g.Name, "/aws/lambda/foo")
	}
	if g.RetentionDays != 14 {
		t.Errorf("RetentionDays = %d, want 14", g.RetentionDays)
	}
	if !g.CreatedAt.Equal(created) {
		t.Errorf("CreatedAt = %v, want %v", g.CreatedAt, created)
	}
}

func TestBuildLogGroupsHandlesNilCreationTime(t *testing.T) {
	raw := []types.LogGroup{
		{LogGroupName: aws.String("/aws/lambda/x"), CreationTime: nil},
	}

	got := buildLogGroups(raw)

	if !got[0].CreatedAt.IsZero() {
		t.Errorf("CreatedAt = %v, want zero value when AWS returns nil", got[0].CreatedAt)
	}
}

func TestBuildLogGroupsHandlesNilRetention(t *testing.T) {
	raw := []types.LogGroup{
		{LogGroupName: aws.String("/aws/lambda/x"), RetentionInDays: nil},
	}

	got := buildLogGroups(raw)

	if got[0].RetentionDays != 0 {
		t.Errorf("RetentionDays = %d, want 0 (never expire) when AWS returns nil", got[0].RetentionDays)
	}
}

func TestBuildLogGroupsSortsByName(t *testing.T) {
	raw := []types.LogGroup{
		{LogGroupName: aws.String("/z")},
		{LogGroupName: aws.String("/a")},
		{LogGroupName: aws.String("/m")},
	}

	got := buildLogGroups(raw)

	want := []string{"/a", "/m", "/z"}
	for i, w := range want {
		if got[i].Name != w {
			t.Errorf("got[%d].Name = %q, want %q", i, got[i].Name, w)
		}
	}
}

func TestBuildLogGroupsEmptyInput(t *testing.T) {
	got := buildLogGroups(nil)
	if len(got) != 0 {
		t.Errorf("buildLogGroups(nil) = %+v, want empty", got)
	}
}
