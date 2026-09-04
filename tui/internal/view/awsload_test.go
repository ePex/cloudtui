package view

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/ePex/cloudtui/tui/internal/awsprofile"
)

func TestRunAWSLoadErrorsWithoutActiveProfile(t *testing.T) {
	host := newFakeViewHost()
	var loadSeq int
	fetchCalls := 0
	var errs []error

	runAWSLoad(host, &loadSeq,
		func(string) { t.Error("showStatus called despite no active profile") },
		func(err error) { errs = append(errs, err) },
		"Loading things…",
		func(context.Context, string) (string, error) {
			fetchCalls++
			return "", nil
		},
		func(string) { t.Error("onSuccess called despite no active profile") },
	)

	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "no AWS profile selected") {
		t.Errorf("errs = %v, want one error mentioning no profile selected", errs)
	}
	if fetchCalls != 0 {
		t.Errorf("fetch invoked %d times, want 0", fetchCalls)
	}
	if loadSeq != 0 {
		t.Errorf("loadSeq = %d, want 0 (guard returns before bumping it)", loadSeq)
	}
}

func TestRunAWSLoadShowsLoadingStatusImmediately(t *testing.T) {
	host := newFakeViewHost()
	host.cfg.ActiveAWSProfile = "work"
	var loadSeq int
	var statuses []string
	unblock := make(chan struct{})

	runAWSLoad(host, &loadSeq,
		func(msg string) { statuses = append(statuses, msg) },
		func(error) {},
		"Loading things…",
		func(context.Context, string) (string, error) {
			<-unblock
			return "", nil
		},
		func(string) {},
	)

	if len(statuses) != 1 || statuses[0] != "Loading things…" {
		t.Errorf("statuses after runAWSLoad() = %v, want [%q]", statuses, "Loading things…")
	}
	close(unblock) // let the goroutine finish so it doesn't leak past the test
}

// newTestAWSLoadHostWithDrawSignal returns a fakeViewHost wrapped in
// drawSignalingHost (shared package-wide, defined in queues_test.go)
// so a test can block until runAWSLoad's QueueUpdateDraw dispatch has
// actually landed instead of guessing with a sleep.
func newTestAWSLoadHostWithDrawSignal(t *testing.T, bufSize int) *drawSignalingHost {
	t.Helper()
	base := newFakeViewHost()
	base.cfg.ActiveAWSProfile = "work"
	return &drawSignalingHost{fakeViewHost: base, drawn: make(chan struct{}, bufSize)}
}

// TestRunAWSLoadDiscardsStaleResponse is runAWSLoad's own direct
// coverage of the loadSeq guard — see e.g. queues_test.go's
// TestQueuesViewLoadDiscardsStaleResponse for the pattern this mirrors,
// now exercised once at the shared-helper level instead of once per view.
func TestRunAWSLoadDiscardsStaleResponse(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	firstCalled := make(chan struct{})
	releaseFirst := make(chan struct{})

	host := newTestAWSLoadHostWithDrawSignal(t, 2)
	var loadSeq int
	var results []string
	fetch := func(context.Context, string) (string, error) {
		mu.Lock()
		calls++
		n := calls
		mu.Unlock()
		if n == 1 {
			close(firstCalled)
			<-releaseFirst
			return "stale", nil
		}
		return "fresh", nil
	}
	onSuccess := func(r string) { results = append(results, r) }

	runAWSLoad(host, &loadSeq, func(string) {}, func(error) {}, "loading", fetch, onSuccess) // call 1 — will become "stale"; blocks inside fetch
	<-firstCalled                                                                            // call 1's fetch has started (and is now blocked on releaseFirst)

	runAWSLoad(host, &loadSeq, func(string) {}, func(error) {}, "loading", fetch, onSuccess) // call 2 — "fresh"; proceeds and draws immediately
	<-host.drawn                                                                             // call 2's draw has landed (guaranteed first: call 1 can't proceed yet)

	if len(results) != 1 || results[0] != "fresh" {
		t.Fatalf("results after call 2's draw = %v, want [%q]", results, "fresh")
	}

	close(releaseFirst) // let call 1 (stale) proceed to its now-discarded dispatch attempt
	<-host.drawn        // call 1's dispatch attempt has landed (and should have no-opped)

	if len(results) != 1 || results[0] != "fresh" {
		t.Errorf("results after stale call 1's draw = %v, want unchanged [%q]", results, "fresh")
	}
}

// TestRunAWSLoadShowsReauthWaitingThenCode exercises the full
// reauth-then-retry path through awsauth.Do, confirming runAWSLoad
// shows the loading placeholder, then the reauth-waiting message, then
// that same message with the device code/URL appended, in that order,
// before finally succeeding on the retried fetch — the sequence every
// one of the 5 views' now-removed inline onReauth/onCode closures used
// to build by hand.
func TestRunAWSLoadShowsReauthWaitingThenCode(t *testing.T) {
	host := newFakeViewHost()
	host.cfg.ActiveAWSProfile = "work"
	host.awsAuthTypeForFn = func(context.Context, string) (awsprofile.AuthType, error) {
		return awsprofile.AuthSSO, nil
	}
	host.awsSSOLoginFn = func(_ context.Context, _ string, onCode func(string, string)) error {
		onCode("WDJB-MJHT", "https://device.sso.us-east-1.amazonaws.com/")
		return nil
	}

	fetchCalls := 0
	fetch := func(context.Context, string) (string, error) {
		fetchCalls++
		if fetchCalls == 1 {
			return "", errors.New("cached SSO token is expired, or not present, and cannot be refreshed")
		}
		return "ok", nil
	}

	var loadSeq int
	var statuses []string
	var results []string
	done := make(chan struct{})

	runAWSLoad(host, &loadSeq,
		func(msg string) { statuses = append(statuses, msg) },
		func(error) {},
		"Loading things…",
		fetch,
		func(r string) { results = append(results, r); close(done) },
	)
	<-done

	want := []string{
		"Loading things…",
		"AWS SSO session expired — opening browser to log in...",
		"AWS SSO session expired — opening browser to log in... Verify code WDJB-MJHT at https://device.sso.us-east-1.amazonaws.com/",
	}
	if len(statuses) != len(want) {
		t.Fatalf("statuses = %v, want %v", statuses, want)
	}
	for i := range want {
		if statuses[i] != want[i] {
			t.Errorf("statuses[%d] = %q, want %q", i, statuses[i], want[i])
		}
	}
	if len(results) != 1 || results[0] != "ok" {
		t.Errorf("results = %v, want [%q]", results, "ok")
	}
	if fetchCalls != 2 {
		t.Errorf("fetch invoked %d times, want 2 (initial + retry after login)", fetchCalls)
	}
}
