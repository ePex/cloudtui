package dialog

import (
	"context"
	"strings"
	"testing"

	"github.com/rivo/tview"
)

func newTestMessageFilter(t *testing.T) (*MessageFilter, *testHost) {
	t.Helper()
	host := newTestHost()
	return NewMessageFilter(host), host
}

// TestShowRefreshesStaleAutocomplete is a regression test: NewMessageFilter
// wires SetAutocompleteFunc, which eagerly builds and caches the
// drop-down's rendered contents once immediately at construction time —
// before messagesV exists (LoadedJMSTypes returns nothing then) and
// before any real messages are loaded. SetText (which Show() calls to
// prefill the field) does not itself refresh that cache — only a live
// keystroke or an explicit Autocomplete() call does (same gotcha as the
// ':' prompt, see spec/01). Without Show() calling Autocomplete()
// explicitly, opening the dialog fresh would render only the sentinel,
// never the types actually available by the time a user opens it.
func TestShowRefreshesStaleAutocomplete(t *testing.T) {
	mf, host := newTestMessageFilter(t)
	// Simulate messagesV/loaded messages becoming available only after
	// construction, same as in the real app (NewMessageFilter runs before
	// a.messagesV exists).
	host.loadedJMSTypes = []string{"OrderCreated"}

	mf.Show()
	mf.jmsTypeItem.Focus(func(tview.Primitive) {})

	rendered := renderedScreenText(t, mf.jmsTypeItem, 40, 6)
	if !strings.Contains(rendered, "OrderCreated") {
		t.Errorf("rendered autocomplete drop-down = %q, want it to contain the loaded type %q", rendered, "OrderCreated")
	}
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

// TestOnJMSTypeChangedTriggersScan confirms selecting the sentinel
// synchronously marks a scan in flight. It deliberately does not wait for
// startScan's actual goroutine to complete — like this codebase's view
// load() methods, that path only completes with a running tview event
// loop (see TestHandleScanResultMergesIntoSuggestions for the completion
// half, tested directly instead, including where the field's text
// actually gets cleared — see onJMSTypeChanged's doc comment for why not
// here).
func TestOnJMSTypeChangedTriggersScan(t *testing.T) {
	mf, host := newTestMessageFilter(t)
	host.scanJMSTypesFn = func(context.Context, string, int) ([]string, error) {
		select {} // never returns; this test only checks the synchronous part
	}

	mf.onJMSTypeChanged(jmsTypeScanSentinel)

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
	host.scanJMSTypesFn = func(context.Context, string, int) ([]string, error) {
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
	mf.jmsTypeItem.SetText(jmsTypeScanSentinel) // left there by onJMSTypeChanged in the real flow
	host.loadedJMSTypes = []string{"OrderCreated"}

	mf.handleScanResult([]string{"ScannedType"}, nil)

	if mf.scanning {
		t.Error("scanning after handleScanResult = true, want false")
	}
	if got := mf.jmsTypeItem.GetText(); got != "" {
		t.Errorf("field text after handleScanResult = %q, want empty", got)
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
	mf.jmsTypeItem.SetText(jmsTypeScanSentinel)

	mf.handleScanResult(nil, context.DeadlineExceeded)

	if mf.scanning {
		t.Error("scanning after handleScanResult error = true, want false")
	}
	if got := mf.jmsTypeItem.GetText(); got != "" {
		t.Errorf("field text after a failed scan = %q, want empty (cleared even on error)", got)
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

// TestApplyRefusesWhileScanning guards against applying the JMS Type
// field's literal sentinel text as a filter value: while a scan is in
// flight, the field still holds that text (see onJMSTypeChanged's doc
// comment for why it can't be cleared synchronously).
func TestApplyRefusesWhileScanning(t *testing.T) {
	mf, host := newTestMessageFilter(t)
	mf.scanning = true

	mf.apply()

	if host.appliedFilter != nil {
		t.Errorf("ApplyMessagesFilter was called while scanning: %+v", *host.appliedFilter)
	}
	if !strings.Contains(host.status, "scanning") {
		t.Errorf("status = %q, want it to mention the in-progress scan", host.status)
	}
}
