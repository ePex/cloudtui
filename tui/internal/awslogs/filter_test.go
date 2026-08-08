package awslogs

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
)

func TestBuildLogEventsPopulatesFields(t *testing.T) {
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	raw := []types.FilteredLogEvent{
		{
			Timestamp:     aws.Int64(ts.UnixMilli()),
			LogStreamName: aws.String("2026/01/02/[$LATEST]abc123"),
			Message:       aws.String("START RequestId: abc123"),
		},
	}

	got := buildLogEvents(raw)

	if len(got) != 1 {
		t.Fatalf("buildLogEvents() len = %d, want 1", len(got))
	}
	e := got[0]
	if !e.Timestamp.Equal(ts) {
		t.Errorf("Timestamp = %v, want %v", e.Timestamp, ts)
	}
	if e.LogStream != "2026/01/02/[$LATEST]abc123" {
		t.Errorf("LogStream = %q, want %q", e.LogStream, "2026/01/02/[$LATEST]abc123")
	}
	if e.Message != "START RequestId: abc123" {
		t.Errorf("Message = %q, want %q", e.Message, "START RequestId: abc123")
	}
}

func TestBuildLogEventsHandlesNilTimestamp(t *testing.T) {
	raw := []types.FilteredLogEvent{
		{Message: aws.String("x"), Timestamp: nil},
	}

	got := buildLogEvents(raw)

	if !got[0].Timestamp.IsZero() {
		t.Errorf("Timestamp = %v, want zero value when AWS returns nil", got[0].Timestamp)
	}
}

func TestBuildLogEventsPreservesOrder(t *testing.T) {
	raw := []types.FilteredLogEvent{
		{Message: aws.String("third")},
		{Message: aws.String("first")},
		{Message: aws.String("second")},
	}

	got := buildLogEvents(raw)

	want := []string{"third", "first", "second"} // AWS's own order, no re-sort
	for i, w := range want {
		if got[i].Message != w {
			t.Errorf("got[%d].Message = %q, want %q", i, got[i].Message, w)
		}
	}
}

func TestBuildLogEventsEmptyInput(t *testing.T) {
	got := buildLogEvents(nil)
	if len(got) != 0 {
		t.Errorf("buildLogEvents(nil) = %+v, want empty", got)
	}
}
