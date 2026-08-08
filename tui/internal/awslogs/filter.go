package awslogs

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
)

// FilterEvents runs a single FilterLogEvents call against logGroupName
// for the time window [start, end), optionally narrowed by pattern (AWS's
// own filter-pattern syntax, passed straight through — empty matches
// everything in range). Returns at most one page of results (Limit:
// 1000), newest first (StartFromHead: false). hasMore reports whether
// AWS indicated more results exist (NextToken present) — this function
// never auto-paginates; the caller decides what to do with hasMore (show
// it, don't silently fetch more).
func FilterEvents(ctx context.Context, profile, logGroupName string, start, end time.Time, pattern string) ([]LogEvent, bool, error) {
	client, err := newClient(ctx, profile)
	if err != nil {
		return nil, false, err
	}

	input := &cloudwatchlogs.FilterLogEventsInput{
		LogGroupName:  aws.String(logGroupName),
		StartTime:     aws.Int64(start.UnixMilli()),
		EndTime:       aws.Int64(end.UnixMilli()),
		Limit:         aws.Int32(1000),
		StartFromHead: aws.Bool(false),
	}
	if pattern != "" {
		input.FilterPattern = aws.String(pattern)
	}

	out, err := client.FilterLogEvents(ctx, input)
	if err != nil {
		return nil, false, fmt.Errorf("searching log group %q: %w", logGroupName, err)
	}
	return buildLogEvents(out.Events), out.NextToken != nil, nil
}

// buildLogEvents converts raw AWS filtered log events into LogEvent.
// Preserves AWS's own order (already newest-first via StartFromHead:
// false in FilterEvents, so no re-sort needed here). Split out so this —
// the part with actual logic to get wrong — is unit-testable without a
// real AWS call.
func buildLogEvents(raw []types.FilteredLogEvent) []LogEvent {
	out := make([]LogEvent, 0, len(raw))
	for _, e := range raw {
		var ts time.Time
		if e.Timestamp != nil {
			ts = time.UnixMilli(*e.Timestamp)
		}
		out = append(out, LogEvent{
			Timestamp: ts,
			LogStream: aws.ToString(e.LogStreamName),
			Message:   aws.ToString(e.Message),
		})
	}
	return out
}
