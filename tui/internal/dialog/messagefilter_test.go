package dialog

import (
	"context"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/ui"
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

func TestStartScanIgnoresDuplicateWhileInFlight(t *testing.T) {
	mf, host := newTestMessageFilter(t)
	calls := 0
	entered := make(chan struct{})
	host.scanJMSTypesFn = func(context.Context, string, int) (ui.JMSTypeScan, error) {
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
// completion path directly — like this codebase's view load() methods,
// the goroutine path only completes with a running tview event loop.
func TestHandleScanResultMergesIntoSuggestions(t *testing.T) {
	mf, host := newTestMessageFilter(t)
	mf.scanning = true
	mf.jmsTypeItem.SetText("Typ") // typed while the scan ran
	host.loadedJMSTypes = []string{"OrderCreated"}

	mf.handleScanResult(ui.JMSTypeScan{Types: []string{"ScannedType"}}, nil)

	if mf.scanning {
		t.Error("scanning after handleScanResult = true, want false")
	}
	if got := mf.jmsTypeItem.GetText(); got != "Typ" {
		t.Errorf("field text after handleScanResult = %q, want the typed %q kept", got, "Typ")
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

func TestHandleScanResultReportsTheLimit(t *testing.T) {
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
			mf, host := newTestMessageFilter(t)
			host.status = "Scanning up to 5000 messages for JMS types..."

			mf.handleScanResult(tt.scan, nil)

			if host.status != tt.want {
				t.Errorf("status = %q, want %q", host.status, tt.want)
			}
		})
	}
}

func TestHandleScanResultErrorSetsStatusAndClearsFlag(t *testing.T) {
	mf, host := newTestMessageFilter(t)
	mf.scanning = true
	mf.jmsTypeItem.SetText("Typ")

	mf.handleScanResult(ui.JMSTypeScan{}, context.DeadlineExceeded)

	if mf.scanning {
		t.Error("scanning after handleScanResult error = true, want false")
	}
	if got := mf.jmsTypeItem.GetText(); got != "Typ" {
		t.Errorf("field text after a failed scan = %q, want the typed %q kept", got, "Typ")
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

// TestApplyWhileScanningUsesTheTypedText: the field only ever holds what
// the user typed (the scan entry's text never goes in), so applying while
// a scan runs is safe and filters by that text.
func TestApplyWhileScanningUsesTheTypedText(t *testing.T) {
	mf, host := newTestMessageFilter(t)
	mf.scanning = true
	mf.jmsTypeItem.SetText("TypeB")

	mf.apply()

	if host.appliedFilter == nil || host.appliedFilter.JMSType != "TypeB" {
		t.Errorf("applied filter = %+v, want JMSType TypeB", host.appliedFilter)
	}
}

// openSettledFilter opens the filter with TypeA among the loaded types and
// the form's own focus handling installed (Form.Focus sets the handler that
// moves to the next field on Tab/Enter). focused receives whatever the
// form hands focus to — without a running app nothing really gets focus,
// so Form.GetFocusedItemIndex can't tell.
func openSettledFilter(t *testing.T) (mf *MessageFilter, focused *tview.Primitive) {
	t.Helper()
	mf, host := newTestMessageFilter(t)
	host.loadedJMSTypes = []string{"TypeA"}
	host.scanJMSTypesFn = blockForever
	mf.Show()
	var got tview.Primitive
	mf.form.Focus(func(p tview.Primitive) { got = p })
	return mf, &got
}

// TestMessageFilterKeysKeepTypedText is the regression test for Enter/Tab
// picking the highlighted scan entry (spec-wip bugfix-jmstype-scan-input,
// defect 2): without an arrow-key choice, both keep the typed text and
// move on to the next form field.
func TestMessageFilterKeysKeepTypedText(t *testing.T) {
	for _, typed := range []string{"", "TypeB", "Type"} {
		for _, key := range []tcell.Key{tcell.KeyEnter, tcell.KeyTab} {
			mf, focused := openSettledFilter(t)

			press(mf.jmsTypeItem, typed, key)

			name := tcell.KeyNames[key] + " on " + typed
			if mf.scanning {
				t.Errorf("%s: started a scan", name)
			}
			if got := mf.jmsTypeItem.GetText(); got != typed {
				t.Errorf("%s: field = %q, want the typed text kept", name, got)
			}
			if *focused != mf.form.GetFormItem(1) {
				t.Errorf("%s: focus didn't move on to the From field", name)
			}
		}
	}
}

func TestMessageFilterArrowChoiceAcceptsTheType(t *testing.T) {
	mf, focused := openSettledFilter(t)

	navigateTo(t, mf.jmsTypeItem, mf.ac, "TypeA")
	press(mf.jmsTypeItem, tcell.KeyEnter)

	if got := mf.jmsTypeItem.GetText(); got != "TypeA" {
		t.Errorf("field = %q, want the chosen TypeA", got)
	}
	if *focused != mf.form.GetFormItem(0) {
		t.Error("focus moved on — the key should only accept the choice")
	}
}

func TestMessageFilterArrowChoiceOfScanEntryRunsTheScan(t *testing.T) {
	mf, _ := openSettledFilter(t)
	press(mf.jmsTypeItem, "Ty")

	navigateTo(t, mf.jmsTypeItem, mf.ac, jmsTypeScanSentinel)
	press(mf.jmsTypeItem, tcell.KeyEnter)

	if !mf.scanning {
		t.Fatal("choosing the scan entry didn't start a scan")
	}
	if got := mf.jmsTypeItem.GetText(); got != "Ty" {
		t.Errorf("field = %q, want the typed %q — the scan entry never goes into the field", got, "Ty")
	}
}
