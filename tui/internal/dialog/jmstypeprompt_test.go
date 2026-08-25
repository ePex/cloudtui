package dialog

import (
	"context"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func newTestJMSTypePrompt(t *testing.T) (*JMSTypePrompt, *testHost) {
	t.Helper()
	host := newTestHost()
	return NewJMSTypePrompt(host), host
}

// blockForever is a scanJMSTypesFn that never returns — used whenever a
// test calls Show() (which now starts an automatic scan synchronously,
// see jmsTypeAutoScanCount) but doesn't want that scan to actually
// complete during the test. Without this, the real goroutine Show()
// spawns races the test's own assertions on jp's fields, which -race
// would (correctly) flag even on runs where the race happens not to
// change the asserted values.
func blockForever(context.Context, string, int) ([]string, error) {
	select {}
}

// TestJMSTypePromptShowRefreshesStaleAutocomplete is a regression test, same root
// cause and fix as MessageFilter's own version of this test (see its
// doc comment) — SetAutocompleteFunc eagerly caches the drop-down once
// at construction time, and SetText (which Show() calls) doesn't itself
// refresh it. Without Show() calling Autocomplete() explicitly, opening
// the prompt fresh would render only the sentinel.
func TestJMSTypePromptShowRefreshesStaleAutocomplete(t *testing.T) {
	jp, host := newTestJMSTypePrompt(t)
	jp.scanned = []string{"OrderCreated"} // simulate a completed scan from a prior Show()... but Show() resets it
	host.scanJMSTypesFn = blockForever    // freeze state at what Show() sets synchronously

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
	host.scanJMSTypesFn = blockForever

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
		close(entered) // happens-after calls++, so the test's read below is race-free
		select {}
	}

	jp.startScan(jmsTypeScanCount)
	<-entered
	jp.startScan(jmsTypeScanCount)

	if calls != 1 {
		t.Errorf("ScanJMSTypes calls = %d, want 1 (second startScan should have no-opped)", calls)
	}
}

func TestJMSTypePromptStartScanUsesQueueNameAndMaxCount(t *testing.T) {
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

	jp.startScan(jmsTypeScanCount)
	<-done

	if gotQueue != "orders" {
		t.Errorf("queue passed to ScanJMSTypes = %q, want %q", gotQueue, "orders")
	}
	if gotMax != jmsTypeScanCount {
		t.Errorf("maxCount passed to ScanJMSTypes = %d, want %d", gotMax, jmsTypeScanCount)
	}
}

// TestJMSTypePromptShowStartsAutomaticScan confirms Show() itself kicks
// off a scan (capped at jmsTypeAutoScanCount, smaller than the sentinel's
// opt-in jmsTypeScanCount) without any user action — the fix for the
// live report that the prompt looked empty/pointless on open, since
// selecting the always-present sentinel wasn't obviously an action to a
// user who hadn't used it before.
func TestJMSTypePromptShowStartsAutomaticScan(t *testing.T) {
	jp, host := newTestJMSTypePrompt(t)
	done := make(chan struct{})
	var gotQueue string
	var gotMax int
	host.scanJMSTypesFn = func(_ context.Context, queueName string, maxCount int) ([]string, error) {
		gotQueue, gotMax = queueName, maxCount
		close(done)
		return []string{"OrderCreated"}, nil
	}

	jp.Show("Purge", "orders", nil, nil)
	<-done

	if gotQueue != "orders" {
		t.Errorf("queue passed to the automatic scan = %q, want %q", gotQueue, "orders")
	}
	if gotMax != jmsTypeAutoScanCount {
		t.Errorf("maxCount passed to the automatic scan = %d, want %d (jmsTypeAutoScanCount)", gotMax, jmsTypeAutoScanCount)
	}
}

func TestJMSTypePromptHandleScanResultMergesIntoSuggestions(t *testing.T) {
	jp, _ := newTestJMSTypePrompt(t)
	jp.scanning = true
	jp.field.SetText(jmsTypeScanSentinel) // left there by onJMSTypeChanged in the real flow

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

// TestJMSTypePromptShowResetsState also confirms Show() leaves scanning
// == true — it starts the automatic scan synchronously (jp.scanning is
// set before the scan's own goroutine is even spawned), unlike before
// this feature existed, when Show() left scanning alone.
func TestJMSTypePromptShowResetsState(t *testing.T) {
	jp, host := newTestJMSTypePrompt(t)
	jp.scanned = []string{"Stale"}
	host.scanJMSTypesFn = blockForever

	jp.Show("Purge", "orders", nil, nil)

	if jp.scanned != nil {
		t.Errorf("scanned after Show() = %v, want nil", jp.scanned)
	}
	if !jp.scanning {
		t.Error("scanning after Show() = false, want true (the automatic scan should already be in flight)")
	}
	if jp.queueName != "orders" {
		t.Errorf("queueName after Show() = %q, want %q", jp.queueName, "orders")
	}
}

// TestJMSTypePromptContinueNowRefusesWhileFieldHoldsSentinel guards
// against applying the JMS Type field's literal sentinel text as a
// filter value: while the sentinel's own opt-in scan is in flight, the
// field still holds that literal text (see onJMSTypeChanged's doc
// comment for why it can't be cleared synchronously).
//
// SetText on the sentinel's own literal text fires the field's real
// SetChangedFunc wiring — same as a live keystroke would — which starts
// a real scan via onJMSTypeChanged/startScan. blockForever keeps that
// scan from ever completing and touching jp.host concurrently with this
// test's own continueNow() call below (found via -race: both would call
// SetStatus without synchronization otherwise).
func TestJMSTypePromptContinueNowRefusesWhileFieldHoldsSentinel(t *testing.T) {
	jp, host := newTestJMSTypePrompt(t)
	jp.queueName = "orders"
	host.scanJMSTypesFn = blockForever
	jp.field.SetText(jmsTypeScanSentinel)
	continued := false
	jp.onContinue = func(string) { continued = true }

	jp.continueNow()

	if continued {
		t.Error("onContinue was called while the field held the scan-trigger sentinel")
	}
	if !strings.Contains(host.status, "scanning") {
		t.Errorf("status = %q, want it to mention the in-progress scan", host.status)
	}
}

// TestJMSTypePromptContinueNowProceedsWhileAutoScanInFlight is the
// counterpart to the test above: jp.scanning alone (the automatic scan
// from Show(), not the sentinel-triggered one) must NOT block
// continueNow — only the field literally holding the sentinel's text
// does. Without this distinction, "leave blank + Enter proceeds
// immediately" would be broken for the entire, often-non-trivial
// duration of the automatic scan, since Show() itself sets jp.scanning
// synchronously.
func TestJMSTypePromptContinueNowProceedsWhileAutoScanInFlight(t *testing.T) {
	jp, _ := newTestJMSTypePrompt(t)
	jp.scanning = true // simulating the automatic scan still in flight; field never touched
	var got string
	called := false
	jp.onContinue = func(jmsType string) { called, got = true, jmsType }

	jp.continueNow()

	if !called || got != "" {
		t.Errorf("onContinue called=%v jmsType=%q, want called=true jmsType=\"\" even while a scan is in flight", called, got)
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

// TestJMSTypePromptEnterOnBlankFieldContinuesEvenWhileAutoScanInFlight is
// a regression test found live (verify-live): with no free suggestion
// tier, the sentinel used to be the sole, already-highlighted
// autocomplete entry on an untouched field — tview.InputField's own
// Enter handling accepts whatever's highlighted in an open drop-down
// before ever reaching SetDoneFunc, so without the SetInputCapture guard
// this test exercises, pressing Enter on a blank, fresh-from-Show()
// field would accept the sentinel instead of continuing with no filter.
// Now that Show() also starts an automatic scan synchronously (jmsTypeAutoScanCount),
// this additionally confirms that scan being in flight doesn't block the
// blank-field Enter either — only the field literally holding the
// sentinel text does (see TestJMSTypePromptContinueNowRefusesWhileFieldHoldsSentinel).
func TestJMSTypePromptEnterOnBlankFieldContinuesEvenWhileAutoScanInFlight(t *testing.T) {
	jp, host := newTestJMSTypePrompt(t)
	host.scanJMSTypesFn = blockForever // keeps the automatic scan "in flight" for the whole test
	var got string
	called := false
	jp.Show("Purge", "orders", func(jmsType string) { called, got = true, jmsType }, nil)

	if !jp.scanning {
		t.Fatal("scanning after Show() = false, want true (the automatic scan should already be in flight)")
	}

	// Show() already opens the drop-down (its own eager Autocomplete()
	// call) with the sentinel as the sole entry (scanned is nil at this
	// point — the automatic scan is still in flight) — this is exactly
	// the state a real "just press Enter" user hits immediately after
	// pressing 'p'/'M'.
	jp.field.GetInputCapture()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	if !called {
		t.Fatal("onContinue was not called")
	}
	if got != "" {
		t.Errorf("onContinue jmsType = %q, want empty (no filter)", got)
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
