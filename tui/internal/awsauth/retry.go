package awsauth

import (
	"context"
	"fmt"

	"github.com/ePex/cloudtui/tui/internal/awsprofile"
)

// WithReauth calls call once. If it fails with an error NeedsReauth
// recognizes for profile's authType, it invokes onReauth (so the caller
// can show a status message before the browser opens), then login
// (passing onCode through so the caller can show the device
// verification code/URL once login's subprocess prints them), then
// retries call exactly once. Any other failure — including login itself
// failing — is returned as-is; login's failure is wrapped onto the
// original call error for context.
func WithReauth[T any](
	ctx context.Context,
	profile string,
	authType awsprofile.AuthType,
	login func(ctx context.Context, profile string, onCode func(code, url string)) error,
	onReauth func(),
	onCode func(code, url string),
	call func(ctx context.Context) (T, error),
) (T, error) {
	result, err := call(ctx)
	if err == nil || !NeedsReauth(err, authType) {
		return result, err
	}

	if onReauth != nil {
		onReauth()
	}

	if loginErr := login(ctx, profile, onCode); loginErr != nil {
		return result, fmt.Errorf("%w (re-auth attempt failed: %v)", err, loginErr)
	}

	return call(ctx)
}

// Do is WithReauth with AuthType resolution folded in — every current
// call site repeats "look up AuthType, then call WithReauth" by hand;
// Do does both in one call. authTypeFor's error is discarded (same as
// every existing call site's own `authType, _ := ...`) since WithReauth
// degrades gracefully to "never reauth" for an unrecognized AuthType.
func Do[T any](
	ctx context.Context,
	profile string,
	authTypeFor func(ctx context.Context, profile string) (awsprofile.AuthType, error),
	login func(ctx context.Context, profile string, onCode func(code, url string)) error,
	onReauth func(),
	onCode func(code, url string),
	call func(ctx context.Context) (T, error),
) (T, error) {
	authType, _ := authTypeFor(ctx, profile)
	return WithReauth(ctx, profile, authType, login, onReauth, onCode, call)
}
