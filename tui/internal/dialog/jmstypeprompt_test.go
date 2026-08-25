package dialog

import (
	"context"
	"strings"
	"testing"

	"github.com/rivo/tview"
)

func newTestJMSTypePrompt(t *testing.T) (*JMSTypePrompt, *testHost) {
	t.Helper()
	host := newTestHost()
	return NewJMSTypePrompt(host), host
}

// TestJMSTypePromptShowRefreshesStaleAutocomplete is a regression test, same root
// cause and fix as MessageFilter's own version of this test (see its
// doc comment) — SetAutocompleteFunc eagerly caches the drop-down once
// at construction time, and SetText (which Show() calls) doesn't itself
// refresh it. Without Show() calling Autocomplete() explicitly, opening
// the prompt fresh would render only the sentinel.
func TestJMSTypePromptShowRefreshesStaleAutocomplete(t *testing.T) {
	jp, _ := newTestJMSTypePrompt(t)
	jp.scanned = []string{"OrderCreated"} // simulate a completed scan from a prior Show()... but Show() resets it

	jp.Show("Purge", "orders", nil, nil)
	jp.field.Focus(func(tview.Primitive) {})

	rendered := renderedScreenText(t, jp.field, 60, 6)
	// scanned is reset by Show(), so only the sentinel should render —
	// this asserts the drop-down actually reflects that reset (not a
	// stale cache from before Show() ran).
	if strings.Contains(rendered, "OrderCreated") {
		t.Errorf("rendered autocomplete drop-down = %q, want it to NOT contain the pre-Show scanned type", rendered)
	}
	if !strings.Contains(rendered, "Scan up to") {
		t.Errorf("rendered autocomplete drop-down = %q, want it to contain the scan sentinel", rendered)
	}
}

func TestJMSTypePromptSuggestionsOnlySentinelBeforeAnyScan(t *testing.T) {
	jp, _ := newTestJMSTypePrompt(t)

	got := jp.jmsTypeSuggestions("")

	if len(got) != 1 || got[0] != jmsTypeScanSentinel {
		t.Errorf("jmsTypeSuggestions(\"\") = %v, want just the sentinel (no free tier)", got)
	}
}

func TestJMSTypePromptSuggestionsIncludesScannedTypesAndSentinel(t *testing.T) {
	jp, _ := newTestJMSTypePrompt(t)
	jp.scanned = []string{"OrderCreated", "OrderCancelled"}

	got := jp.jmsTypeSuggestions("")

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

func TestJMSTypePromptSuggestionsFiltersByPrefix(t *testing.T) {
	jp, _ := newTestJMSTypePrompt(t)
	jp.scanned = []string{"OrderCreated", "OrderCancelled", "PaymentFailed"}

	got := jp.jmsTypeSuggestions("Order")

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

func TestJMSTypePromptOnJMSTypeChangedTriggersScan(t *testing.T) {
	jp, host := newTestJMSTypePrompt(t)
	jp.queueName = "orders"
	host.scanJMSTypesFn = func(_ context.Context, _ string, _ int) ([]string, error) {
		select {} // never returns; this test only checks the synchronous part
	}

	jp.onJMSTypeChanged(jmsTypeScanSentinel)

	if !jp.scanning {
		t.Error("scanning after selecting sentinel = false, want true")
	}
}

func TestJMSTypePromptOnJMSTypeChangedIgnoresNormalText(t *testing.T) {
	jp, _ := newTestJMSTypePrompt(t)

	jp.onJMSTypeChanged("OrderCreated")

	if jp.scanning {
		t.Error("scanning = true after a normal (non-sentinel) text change")
	}
}

func TestJMSTypePromptStartScanIgnoresDuplicateWhileInFlight(t *testing.T) {
	jp, host := newTestJMSTypePrompt(t)
	jp.queueName = "orders"
	calls := 0
	entered := make(chan struct{})
	host.scanJMSTypesFn = func(context.Context, string, int) ([]string, error) {
		calls++
		close(entered)
		select {}
	}

	jp.startScan()
	<-entered
	jp.startScan()

	if calls != 1 {
		t.Errorf("ScanJMSTypes calls = %d, want 1 (second startScan should have no-opped)", calls)
	}
}

func TestJMSTypePromptStartScanUsesQueueName(t *testing.T) {
	jp, host := newTestJMSTypePrompt(t)
	jp.queueName = "orders"
	done := make(chan struct{})
	var gotQueue string
	var gotMax int
	host.scanJMSTypesFn = func(_ context.Context, queueName string, maxCount int) ([]string, error) {
		gotQueue, gotMax = queueName, maxCount
		close(done)
		return nil, nil
	}

	jp.startScan()
	<-done

	if gotQueue != "orders" {
		t.Errorf("queue passed to ScanJMSTypes = %q, want %q", gotQueue, "orders")
	}
	if gotMax != jmsTypeScanCount {
		t.Errorf("maxCount passed to ScanJMSTypes = %d, want %d", gotMax, jmsTypeScanCount)
	}
}

func TestJMSTypePromptHandleScanResultMergesIntoSuggestions(t *testing.T) {
	jp, _ := newTestJMSTypePrompt(t)
	jp.scanning = true
	jp.field.SetText(jmsTypeScanSentinel)

	jp.handleScanResult([]string{"ScannedType"}, nil)

	if jp.scanning {
		t.Error("scanning after handleScanResult = true, want false")
	}
	if got := jp.field.GetText(); got != "" {
		t.Errorf("field text after handleScanResult = %q, want empty", got)
	}
	got := jp.jmsTypeSuggestions("")
	want := []string{"ScannedType", jmsTypeScanSentinel}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("jmsTypeSuggestions() = %v, want %v", got, want)
	}
}

func TestJMSTypePromptHandleScanResultErrorClearsFlag(t *testing.T) {
	jp, host := newTestJMSTypePrompt(t)
	jp.scanning = true
	jp.field.SetText(jmsTypeScanSentinel)

	jp.handleScanResult(nil, context.DeadlineExceeded)

	if jp.scanning {
		t.Error("scanning after handleScanResult error = true, want false")
	}
	if got := jp.field.GetText(); got != "" {
		t.Errorf("field text after a failed scan = %q, want empty", got)
	}
	if !strings.Contains(host.status, "deadline exceeded") {
		t.Errorf("status after scan error = %q, want it to mention the error", host.status)
	}
}

func TestJMSTypePromptShowResetsState(t *testing.T) {
	jp, _ := newTestJMSTypePrompt(t)
	jp.scanned = []string{"Stale"}
	jp.scanning = true

	jp.Show("Purge", "orders", nil, nil)

	if jp.scanned != nil {
		t.Errorf("scanned after Show() = %v, want nil", jp.scanned)
	}
	if jp.scanning {
		t.Error("scanning after Show() = true, want false")
	}
	if jp.queueName != "orders" {
		t.Errorf("queueName after Show() = %q, want %q", jp.queueName, "orders")
	}
}

func TestJMSTypePromptContinueNowRefusesWhileScanning(t *testing.T) {
	jp, host := newTestJMSTypePrompt(t)
	jp.scanning = true
	continued := false
	jp.onContinue = func(string) { continued = true }

	jp.continueNow()

	if continued {
		t.Error("onContinue was called while scanning")
	}
	if !strings.Contains(host.status, "scanning") {
		t.Errorf("status = %q, want it to mention the in-progress scan", host.status)
	}
}

func TestJMSTypePromptContinueNowCallsOnContinueWithFieldText(t *testing.T) {
	jp, _ := newTestJMSTypePrompt(t)
	jp.field.SetText("OrderCreated")
	var got string
	called := false
	jp.onContinue = func(jmsType string) { called, got = true, jmsType }

	jp.continueNow()

	if !called {
		t.Fatal("onContinue was not called")
	}
	if got != "OrderCreated" {
		t.Errorf("onContinue jmsType = %q, want %q", got, "OrderCreated")
	}
}

func TestJMSTypePromptContinueNowEmptyFieldMeansNoFilter(t *testing.T) {
	jp, _ := newTestJMSTypePrompt(t)
	var got string
	called := false
	jp.onContinue = func(jmsType string) { called, got = true, jmsType }

	jp.continueNow()

	if !called || got != "" {
		t.Errorf("onContinue called=%v jmsType=%q, want called=true jmsType=\"\"", called, got)
	}
}

func TestJMSTypePromptCloseCallsOnClose(t *testing.T) {
	jp, _ := newTestJMSTypePrompt(t)
	closed := false
	jp.onClose = func() { closed = true }

	jp.close()

	if !closed {
		t.Error("onClose was not called")
	}
	if jp.visible {
		t.Error("visible after close() = true, want false")
	}
}
