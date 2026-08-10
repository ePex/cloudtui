package awsauth

import (
	"context"
	"fmt"

	"github.com/ePex/cloudtui/tui/internal/awsprofile"
)

// WithReauth calls call once. If it fails with an error NeedsReauth
// recognizes for profile's authType, it invokes onReauth (so the caller
// can show a status message before the browser opens), then login, then
// retries call exactly once. Any other failure — including login itself
// failing — is returned as-is; login's failure is wrapped onto the
// original call error for context.
func WithReauth[T any](
	ctx context.Context,
	profile string,
	authType awsprofile.AuthType,
	login func(ctx context.Context, profile string) error,
	onReauth func(),
	call func(ctx context.Context) (T, error),
) (T, error) {
	result, err := call(ctx)
	if err == nil || !NeedsReauth(err, authType) {
		return result, err
	}

	if onReauth != nil {
		onReauth()
	}

	if loginErr := login(ctx, profile); loginErr != nil {
		return result, fmt.Errorf("%w (re-auth attempt failed: %v)", err, loginErr)
	}

	return call(ctx)
}
