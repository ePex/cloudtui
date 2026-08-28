package awsauth

import (
	"context"
	"errors"
	"testing"

	"github.com/ePex/cloudtui/tui/internal/awsprofile"
)

func TestWithReauthSuccessFirstTry(t *testing.T) {
	loginCalls, onReauthCalls, callCalls := 0, 0, 0
	login := func(context.Context, string, func(string, string)) error {
		loginCalls++
		return nil
	}
	onReauth := func() { onReauthCalls++ }
	call := func(context.Context) (string, error) {
		callCalls++
		return "ok", nil
	}

	got, err := WithReauth(t.Context(), "p", awsprofile.AuthSSO, login, onReauth, nil, call)
	if err != nil {
		t.Fatalf("WithReauth() error = %v, want nil", err)
	}
	if got != "ok" {
		t.Errorf("WithReauth() = %q, want %q", got, "ok")
	}
	if loginCalls != 0 || onReauthCalls != 0 {
		t.Errorf("login/onReauth called (login=%d, onReauth=%d), want neither called", loginCalls, onReauthCalls)
	}
	if callCalls != 1 {
		t.Errorf("call invoked %d times, want 1", callCalls)
	}
}

func TestWithReauthRetriesAfterSuccessfulLogin(t *testing.T) {
	invalidToken := &sentinelReauthError{}
	loginCalls, onReauthCalls, callCalls := 0, 0, 0
	login := func(context.Context, string, func(string, string)) error {
		loginCalls++
		return nil
	}
	onReauth := func() { onReauthCalls++ }
	call := func(context.Context) (string, error) {
		callCalls++
		if callCalls == 1 {
			return "", invalidToken
		}
		return "ok-after-retry", nil
	}

	got, err := WithReauth(t.Context(), "p", awsprofile.AuthSSO, login, onReauth, nil, call)
	if err != nil {
		t.Fatalf("WithReauth() error = %v, want nil", err)
	}
	if got != "ok-after-retry" {
		t.Errorf("WithReauth() = %q, want %q", got, "ok-after-retry")
	}
	if loginCalls != 1 {
		t.Errorf("login invoked %d times, want 1", loginCalls)
	}
	if onReauthCalls != 1 {
		t.Errorf("onReauth invoked %d times, want 1", onReauthCalls)
	}
	if callCalls != 2 {
		t.Errorf("call invoked %d times, want 2", callCalls)
	}
}

func TestWithReauthLoginFails(t *testing.T) {
	invalidToken := &sentinelReauthError{}
	loginErr := errors.New("aws CLI not found")
	callCalls := 0
	login := func(context.Context, string, func(string, string)) error { return loginErr }
	call := func(context.Context) (string, error) {
		callCalls++
		return "", invalidToken
	}

	_, err := WithReauth(t.Context(), "p", awsprofile.AuthSSO, login, nil, nil, call)
	if err == nil {
		t.Fatal("WithReauth() error = nil, want non-nil when login fails")
	}
	if !errors.Is(err, invalidToken) {
		t.Errorf("WithReauth() error = %v, want it to wrap the original call error", err)
	}
	if callCalls != 1 {
		t.Errorf("call invoked %d times, want 1 (no retry when login fails)", callCalls)
	}
}

func TestWithReauthNotNeeded(t *testing.T) {
	originalErr := errors.New("no AWS profile selected")
	loginCalls, onReauthCalls, callCalls := 0, 0, 0
	login := func(context.Context, string, func(string, string)) error {
		loginCalls++
		return nil
	}
	onReauth := func() { onReauthCalls++ }
	call := func(context.Context) (string, error) {
		callCalls++
		return "", originalErr
	}

	// AuthStaticKeys: NeedsReauth is never true regardless of the error,
	// so this must fall straight through without touching login/onReauth.
	_, err := WithReauth(t.Context(), "p", awsprofile.AuthStaticKeys, login, onReauth, nil, call)
	if !errors.Is(err, originalErr) {
		t.Errorf("WithReauth() error = %v, want %v unchanged", err, originalErr)
	}
	if loginCalls != 0 || onReauthCalls != 0 {
		t.Errorf("login/onReauth called (login=%d, onReauth=%d), want neither called", loginCalls, onReauthCalls)
	}
	if callCalls != 1 {
		t.Errorf("call invoked %d times, want 1", callCalls)
	}
}

func TestWithReauthPassesOnCodeThroughToLogin(t *testing.T) {
	invalidToken := &sentinelReauthError{}
	callCalls := 0
	call := func(context.Context) (string, error) {
		callCalls++
		if callCalls == 1 {
			return "", invalidToken
		}
		return "ok", nil
	}

	var gotOnCode func(string, string)
	login := func(_ context.Context, _ string, onCode func(string, string)) error {
		gotOnCode = onCode
		return nil
	}

	onCodeCalled := false
	onCode := func(code, url string) { onCodeCalled = true }

	_, err := WithReauth(t.Context(), "p", awsprofile.AuthSSO, login, nil, onCode, call)
	if err != nil {
		t.Fatalf("WithReauth() error = %v, want nil", err)
	}
	if gotOnCode == nil {
		t.Fatal("login was not given an onCode callback")
	}
	gotOnCode("WDJB-MJHT", "https://device.sso.us-east-1.amazonaws.com/")
	if !onCodeCalled {
		t.Error("the onCode callback login received was not the one WithReauth was given")
	}
}

// sentinelReauthError is a plain error whose message matches the
// sso-session-style string NeedsReauth looks for (see awsauth_test.go
// for detection-logic coverage) — used here purely to make WithReauth's
// own retry/error-plumbing control flow exercisable independent of how
// NeedsReauth classifies errors.
type sentinelReauthError struct{}

func (e *sentinelReauthError) Error() string {
	return "cached SSO token is expired, or not present, and cannot be refreshed"
}
