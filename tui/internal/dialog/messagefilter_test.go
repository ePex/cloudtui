package dialog

import (
	"context"
	"strings"
	"testing"
)

func newTestMessageFilter(t *testing.T) (*MessageFilter, *testHost) {
	t.Helper()
	host := newTestHost()
	return NewMessageFilter(host), host
}

func TestJMSTypeSuggestionsIncludesLoadedTypesAndSentinel(t *testing.T) {
	mf, host := newTestMessageFilter(t)
	host.loadedJMSTypes = []string{"OrderCreated", "OrderCancelled"}

	got := mf.jmsTypeSuggestions("")

	want := []string{"OrderCreated", "OrderCancelled", jmsTypeScanSentinel}
	if len(got) != len(want) {
		t.Fatalf("jmsTypeSuggestions(\"\") = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("jmsTypeSuggestions(\"\")[%d] = %q, want %q (full: %v)", i, got[i], want[i], got)
		}
	}
}

func TestJMSTypeSuggestionsFiltersByPrefix(t *testing.T) {
	mf, host := newTestMessageFilter(t)
	host.loadedJMSTypes = []string{"OrderCreated", "OrderCancelled", "PaymentFailed"}

	got := mf.jmsTypeSuggestions("Order")

	want := []string{"OrderCreated", "OrderCancelled", jmsTypeScanSentinel}
	if len(got) != len(want) {
		t.Fatalf("jmsTypeSuggestions(\"Order\") = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("jmsTypeSuggestions(\"Order\")[%d] = %q, want %q (full: %v)", i, got[i], want[i], got)
		}
	}
}

func TestJMSTypeSuggestionsSentinelAlwaysPresentEvenWithoutMatches(t *testing.T) {
	mf, host := newTestMessageFilter(t)
	host.loadedJMSTypes = []string{"OrderCreated"}

	got := mf.jmsTypeSuggestions("zzz-no-match")

	if len(got) != 1 || got[0] != jmsTypeScanSentinel {
		t.Errorf("jmsTypeSuggestions(\"zzz-no-match\") = %v, want just the sentinel", got)
	}
}

func TestJMSTypeSuggestionsMergesScannedTypes(t *testing.T) {
	mf, host := newTestMessageFilter(t)
	host.loadedJMSTypes = []string{"OrderCreated"}
	mf.scanned = []string{"RareType"}

	got := mf.jmsTypeSuggestions("")

	want := []string{"OrderCreated", "RareType", jmsTypeScanSentinel}
	if len(got) != len(want) {
		t.Fatalf("jmsTypeSuggestions(\"\") = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("jmsTypeSuggestions(\"\")[%d] = %q, want %q (full: %v)", i, got[i], want[i], got)
		}
	}
}

func TestJMSTypeSuggestionsDedupesLoadedAndScanned(t *testing.T) {
	mf, host := newTestMessageFilter(t)
	host.loadedJMSTypes = []string{"OrderCreated"}
	mf.scanned = []string{"OrderCreated", "RareType"}

	got := mf.jmsTypeSuggestions("")

	want := []string{"OrderCreated", "RareType", jmsTypeScanSentinel}
	if len(got) != len(want) {
		t.Fatalf("jmsTypeSuggestions(\"\") = %v, want %v (deduped)", got, want)
	}
}

// TestOnJMSTypeChangedTriggersScan confirms selecting the sentinel (its
// text landing in the field via tview's own accept-and-set behavior)
// synchronously clears the field and marks a scan in flight. It
// deliberately does not wait for startScan's actual goroutine to
// complete — like this codebase's view load() methods, that path only
// completes with a running tview event loop (see
// TestHandleScanResultMergesIntoSuggestions for the completion half,
// tested directly instead).
func TestOnJMSTypeChangedTriggersScan(t *testing.T) {
	mf, host := newTestMessageFilter(t)
	host.scanJMSTypesFn = func(context.Context, int) ([]string, error) {
		select {} // never returns; this test only checks the synchronous part
	}

	mf.onJMSTypeChanged(jmsTypeScanSentinel)

	if got := mf.jmsTypeItem.GetText(); got != "" {
		t.Errorf("field text after selecting sentinel = %q, want empty", got)
	}
	if !mf.scanning {
		t.Error("scanning after selecting sentinel = false, want true")
	}
}

func TestOnJMSTypeChangedIgnoresNormalText(t *testing.T) {
	mf, _ := newTestMessageFilter(t)

	mf.onJMSTypeChanged("OrderCreated")

	if mf.scanning {
		t.Error("scanning = true after a normal (non-sentinel) text change")
	}
}

func TestStartScanIgnoresDuplicateWhileInFlight(t *testing.T) {
	mf, host := newTestMessageFilter(t)
	calls := 0
	entered := make(chan struct{})
	host.scanJMSTypesFn = func(context.Context, int) ([]string, error) {
		calls++
		close(entered) // happens-after calls++, so the test's read below is race-free
		select {}      // never returns
	}

	mf.startScan()
	<-entered      // wait for the first call's goroutine to actually reach the fake
	mf.startScan() // should no-op: mf.scanning is already true after the first call

	if calls != 1 {
		t.Errorf("ScanJMSTypes calls = %d, want 1 (second startScan should have no-opped)", calls)
	}
}

// TestHandleScanResultMergesIntoSuggestions covers startScan's
// completion path directly (see the doc comment on
// TestOnJMSTypeChangedTriggersScan for why it isn't exercised through
// the real goroutine).
func TestHandleScanResultMergesIntoSuggestions(t *testing.T) {
	mf, host := newTestMessageFilter(t)
	mf.scanning = true
	host.loadedJMSTypes = []string{"OrderCreated"}

	mf.handleScanResult([]string{"ScannedType"}, nil)

	if mf.scanning {
		t.Error("scanning after handleScanResult = true, want false")
	}
	got := mf.jmsTypeSuggestions("")
	want := []string{"OrderCreated", "ScannedType", jmsTypeScanSentinel}
	if len(got) != len(want) {
		t.Fatalf("jmsTypeSuggestions() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("jmsTypeSuggestions()[%d] = %q, want %q (full: %v)", i, got[i], want[i], got)
		}
	}
}

func TestHandleScanResultErrorSetsStatusAndClearsFlag(t *testing.T) {
	mf, host := newTestMessageFilter(t)
	mf.scanning = true

	mf.handleScanResult(nil, context.DeadlineExceeded)

	if mf.scanning {
		t.Error("scanning after handleScanResult error = true, want false")
	}
	if !strings.Contains(host.status, "deadline exceeded") {
		t.Errorf("status after scan error = %q, want it to mention the error", host.status)
	}
	if mf.scanned != nil {
		t.Errorf("scanned after a failed scan = %v, want nil (unchanged)", mf.scanned)
	}
}

func TestShowResetsScannedState(t *testing.T) {
	mf, _ := newTestMessageFilter(t)
	mf.scanned = []string{"Stale"}
	mf.scanning = true

	mf.Show()

	if mf.scanned != nil {
		t.Errorf("scanned after Show() = %v, want nil", mf.scanned)
	}
	if mf.scanning {
		t.Error("scanning after Show() = true, want false")
	}
}
