package dialog

import (
	"context"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/ui"
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
func blockForever(context.Context, string, int) (ui.JMSTypeScan, error) {
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

func TestJMSTypePromptStartScanIgnoresDuplicateWhileInFlight(t *testing.T) {
	jp, host := newTestJMSTypePrompt(t)
	jp.queueName = "orders"
	calls := 0
	entered := make(chan struct{})
	host.scanJMSTypesFn = func(context.Context, string, int) (ui.JMSTypeScan, error) {
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
	host.scanJMSTypesFn = func(_ context.Context, queueName string, maxCount int) (ui.JMSTypeScan, error) {
		gotQueue, gotMax = queueName, maxCount
		close(done)
		return ui.JMSTypeScan{}, nil
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
	host.scanJMSTypesFn = func(_ context.Context, queueName string, maxCount int) (ui.JMSTypeScan, error) {
		gotQueue, gotMax = queueName, maxCount
		close(done)
		return ui.JMSTypeScan{Types: []string{"OrderCreated"}}, nil
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

	jp.handleScanResult(ui.JMSTypeScan{Types: []string{"ScannedType"}}, nil)

	if jp.scanning {
		t.Error("scanning after handleScanResult = true, want false")
	}
	got := jp.jmsTypeSuggestions("")
	want := []string{"ScannedType", jmsTypeScanSentinel}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("jmsTypeSuggestions() = %v, want %v", got, want)
	}
}

func TestJMSTypePromptHandleScanResultReportsTheLimit(t *testing.T) {
	tests := []struct {
		name string
		scan ui.JMSTypeScan
		want string
	}{
		{"queue holds more", ui.JMSTypeScan{Types: []string{"TypeA"}, Scanned: 400, HasMore: true},
			"Scanned the first 400 messages — the queue holds more; types further back aren't listed"},
		{"whole queue scanned", ui.JMSTypeScan{Types: []string{"TypeA"}, Scanned: 10}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jp, host := newTestJMSTypePrompt(t)
			host.status = "Scanning up to 500 messages for JMS types..."

			jp.handleScanResult(tt.scan, nil)

			if host.status != tt.want {
				t.Errorf("status = %q, want %q", host.status, tt.want)
			}
		})
	}
}

func TestJMSTypePromptHandleScanResultErrorClearsFlag(t *testing.T) {
	jp, host := newTestJMSTypePrompt(t)
	jp.scanning = true
	jp.field.SetText("Typ") // typed while the scan ran

	jp.handleScanResult(ui.JMSTypeScan{}, context.DeadlineExceeded)

	if jp.scanning {
		t.Error("scanning after handleScanResult error = true, want false")
	}
	if got := jp.field.GetText(); got != "Typ" {
		t.Errorf("field text after a failed scan = %q, want the typed %q kept", got, "Typ")
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

// TestJMSTypePromptContinueNowProceedsWhileAutoScanInFlight: a scan in
// flight must not block continueNow — "leave blank + Enter proceeds
// immediately" has to hold for the whole automatic scan, which Show()
// starts synchronously.
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

// openSettledPrompt opens the prompt with its automatic scan blocked, then
// completes that scan with TypeA — the drop-down holds TypeA plus the
// scan entry, as a user sees it once the prompt has settled. continued
// receives onContinue's argument.
func openSettledPrompt(t *testing.T) (jp *JMSTypePrompt, host *testHost, continued **string) {
	t.Helper()
	jp, host = newTestJMSTypePrompt(t)
	host.scanJMSTypesFn = blockForever
	var got *string
	jp.Show("Purge", "orders", func(s string) { got = &s }, func() {})
	jp.handleScanResult(ui.JMSTypeScan{Types: []string{"TypeA"}}, nil)
	return jp, host, &got
}

// TestJMSTypePromptKeysKeepTypedText is the regression test for Enter/Tab
// picking the highlighted scan entry instead (spec-wip
// bugfix-jmstype-scan-input, defect 2): without an arrow-key choice,
// Enter continues with exactly what was typed and Tab keeps it.
func TestJMSTypePromptKeysKeepTypedText(t *testing.T) {
	for _, typed := range []string{"", "TypeB", "Type"} {
		for _, key := range []tcell.Key{tcell.KeyEnter, tcell.KeyTab} {
			jp, _, continued := openSettledPrompt(t)

			press(jp.field, typed, key)

			name := tcell.KeyNames[key] + " on " + typed
			if jp.scanning {
				t.Errorf("%s: started a scan", name)
			}
			if got := jp.field.GetText(); got != typed {
				t.Errorf("%s: field = %q, want the typed text kept", name, got)
			}
			switch key {
			case tcell.KeyEnter:
				if *continued == nil || **continued != typed {
					t.Errorf("%s: continued = %v, want %q", name, *continued, typed)
				}
			case tcell.KeyTab:
				if *continued != nil || !jp.visible {
					t.Errorf("%s: continued = %v, visible = %v; want the prompt still open", name, *continued, jp.visible)
				}
			}
		}
	}
}

func TestJMSTypePromptArrowChoiceThenEnterAcceptsThenContinues(t *testing.T) {
	jp, _, continued := openSettledPrompt(t)

	navigateTo(t, jp.field, jp.ac, "TypeA")
	press(jp.field, tcell.KeyEnter)
	if got := jp.field.GetText(); got != "TypeA" || *continued != nil {
		t.Fatalf("after choosing: field = %q, continued = %v; want TypeA in the field, prompt still open", got, *continued)
	}

	press(jp.field, tcell.KeyEnter)
	if *continued == nil || **continued != "TypeA" {
		t.Errorf("second Enter: continued = %v, want TypeA", *continued)
	}
}

func TestJMSTypePromptArrowChoiceOfScanEntryRunsTheWiderScan(t *testing.T) {
	jp, _, continued := openSettledPrompt(t)

	navigateTo(t, jp.field, jp.ac, jmsTypeScanSentinel)
	press(jp.field, tcell.KeyEnter)

	if !jp.scanning {
		t.Fatal("choosing the scan entry didn't start a scan")
	}
	if got := jp.field.GetText(); got != "" {
		t.Errorf("field = %q, want empty — the scan entry never goes into the field", got)
	}
	if *continued != nil {
		t.Error("choosing the scan entry continued the purge")
	}
}

// TestJMSTypePromptTextTypedDuringAutomaticScanSurvives is the regression
// test for the automatic scan wiping typed text (defect 1).
func TestJMSTypePromptTextTypedDuringAutomaticScanSurvives(t *testing.T) {
	jp, host := newTestJMSTypePrompt(t)
	host.scanJMSTypesFn = blockForever
	jp.Show("Purge", "orders", nil, nil)

	press(jp.field, "TypeB")
	jp.handleScanResult(ui.JMSTypeScan{Types: []string{"TypeA"}}, nil)

	if got := jp.field.GetText(); got != "TypeB" {
		t.Errorf("field after the automatic scan = %q, want the typed TypeB", got)
	}
	if got := jp.jmsTypeSuggestions("Type"); len(got) != 2 || got[0] != "TypeA" {
		t.Errorf("suggestions for Type = %v, want TypeA from the scan, then the scan entry", got)
	}
}
