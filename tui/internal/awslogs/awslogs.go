// Package awslogs provides read-only access to AWS CloudWatch Logs:
// listing log groups' metadata, and searching a log group's events
// within a time window via FilterLogEvents.
//
// Like internal/awsssm and internal/awssecrets, this package makes real
// AWS API calls and needs credentials to actually resolve — via the
// given profile name, through the standard AWS SDK credential chain
// (SSO, credential_process, static keys, ...). A cached SSO token being
// expired can trigger a real browser-based login flow; that's AWS SDK
// behavior this package doesn't control.
package awslogs

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
)

// LogGroup is one discovered CloudWatch Logs log group's metadata.
type LogGroup struct {
	Name          string
	RetentionDays int32 // 0 = never expire
	CreatedAt     time.Time
}

// LogEvent is one matched log event from a FilterEvents search.
type LogEvent struct {
	Timestamp time.Time
	LogStream string
	Message   string
}

// newClient builds a cloudwatchlogs.Client authenticated as profile. An
// empty profile is a caller error — the view layer is responsible for
// checking a profile is actually selected before calling in.
func newClient(ctx context.Context, profile string) (*cloudwatchlogs.Client, error) {
	if profile == "" {
		return nil, fmt.Errorf("no AWS profile selected")
	}
	cfg, err := config.LoadDefaultConfig(ctx, config.WithSharedConfigProfile(profile))
	if err != nil {
		return nil, fmt.Errorf("loading AWS config for profile %q: %w", profile, err)
	}
	return cloudwatchlogs.NewFromConfig(cfg), nil
}
