package dialog

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// press sends keys through p's real input handling (input capture
// included): a string types its runes, a tcell.Key presses that key.
func press(p tview.Primitive, keys ...any) {
	h := p.InputHandler()
	for _, k := range keys {
		switch k := k.(type) {
		case string:
			for _, r := range k {
				h(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone), func(tview.Primitive) {})
			}
		case tcell.Key:
			h(tcell.NewEventKey(k, 0, tcell.ModNone), func(tview.Primitive) {})
		}
	}
}

// navigateTo presses Down until the arrow keys have highlighted want,
// failing the test if it never comes up.
func navigateTo(t *testing.T, field *tview.InputField, ac *jmsTypeAutocomplete, want string) {
	t.Helper()
	for range 10 {
		press(field, tcell.KeyDown)
		if ac.highlighted == want {
			return
		}
	}
	t.Fatalf("never highlighted %q (last: %q)", want, ac.highlighted)
}

// newTestAutocomplete wires a bare field with TypeA/TypeC plus the scan
// entry, an open drop-down, and a done func recording keys.
func newTestAutocomplete(t *testing.T) (*tview.InputField, *jmsTypeAutocomplete, *int, *[]tcell.Key) {
	t.Helper()
	field := tview.NewInputField()
	scans := 0
	ac := newJMSTypeAutocomplete(field, func(string) []string {
		return []string{"TypeA", "TypeC", jmsTypeScanSentinel}
	}, func() { scans++ })
	var done []tcell.Key
	field.SetDoneFunc(func(k tcell.Key) { done = append(done, k) })
	field.Autocomplete()
	return field, ac, &scans, &done
}

func TestJMSTypeAutocompleteEnterAndTabKeepTypedTextWithoutAChoice(t *testing.T) {
	for _, key := range []tcell.Key{tcell.KeyEnter, tcell.KeyTab} {
		for _, typed := range []string{"", "TypeB", "Type"} {
			field, _, scans, done := newTestAutocomplete(t)

			press(field, typed, key)

			if got := field.GetText(); got != typed {
				t.Errorf("%s on %q: text = %q, want it kept", tcell.KeyNames[key], typed, got)
			}
			if *scans != 0 {
				t.Errorf("%s on %q: started a scan", tcell.KeyNames[key], typed)
			}
			if len(*done) != 1 || (*done)[0] != key {
				t.Errorf("%s on %q: done func got %v, want the key to reach it", tcell.KeyNames[key], typed, *done)
			}
		}
	}
}

func TestJMSTypeAutocompleteChosenTypeIsAccepted(t *testing.T) {
	for _, key := range []tcell.Key{tcell.KeyEnter, tcell.KeyTab} {
		field, ac, scans, done := newTestAutocomplete(t)

		navigateTo(t, field, ac, "TypeC")
		press(field, key)

		if got := field.GetText(); got != "TypeC" {
			t.Errorf("%s: text = %q, want the chosen TypeC", tcell.KeyNames[key], got)
		}
		if *scans != 0 || len(*done) != 0 {
			t.Errorf("%s: scans=%d done=%v, want no scan and the key used for the choice", tcell.KeyNames[key], *scans, *done)
		}
	}
}

func TestJMSTypeAutocompleteChosenScanEntryRunsTheScan(t *testing.T) {
	field, ac, scans, _ := newTestAutocomplete(t)
	press(field, "Ty")

	navigateTo(t, field, ac, jmsTypeScanSentinel)
	press(field, tcell.KeyEnter)

	if *scans != 1 {
		t.Errorf("scans = %d, want 1", *scans)
	}
	if got := field.GetText(); got != "Ty" {
		t.Errorf("text = %q, want the typed %q (the scan entry never goes into the field)", got, "Ty")
	}
}

// TestJMSTypeAutocompleteArrowOnASingleEntryChoosesIt: with no types to
// suggest, the scan entry is the drop-down's only entry. Down can't move
// the highlight in a one-entry list, so tview reports no navigation — yet
// it's exactly when the user needs the scan (found live).
func TestJMSTypeAutocompleteArrowOnASingleEntryChoosesIt(t *testing.T) {
	for _, key := range []tcell.Key{tcell.KeyDown, tcell.KeyUp} {
		field := tview.NewInputField()
		scans := 0
		newJMSTypeAutocomplete(field, func(string) []string {
			return []string{jmsTypeScanSentinel}
		}, func() { scans++ })
		field.Autocomplete()

		press(field, key, tcell.KeyEnter)

		if scans != 1 {
			t.Errorf("%s then Enter on the only entry: scans = %d, want 1", tcell.KeyNames[key], scans)
		}
	}
}

func TestJMSTypeAutocompleteNavigationKeepsTheDropDownAndText(t *testing.T) {
	field, ac, _, _ := newTestAutocomplete(t)

	seen := map[string]bool{}
	for range 3 {
		press(field, tcell.KeyDown)
		seen[ac.highlighted] = true
	}

	if len(seen) != 3 {
		t.Errorf("3 Downs highlighted %v, want all 3 entries (the drop-down collapsed?)", seen)
	}
	if got := field.GetText(); got != "" {
		t.Errorf("text after navigating = %q, want it untouched", got)
	}
}

func TestJMSTypeAutocompleteTypingAfterNavigatingDropsTheChoice(t *testing.T) {
	field, ac, scans, done := newTestAutocomplete(t)
	navigateTo(t, field, ac, jmsTypeScanSentinel)

	press(field, "TypeB", tcell.KeyEnter)

	if *scans != 0 {
		t.Error("Enter after typing started a scan from the earlier arrow-key choice")
	}
	if got := field.GetText(); got != "TypeB" || len(*done) != 1 {
		t.Errorf("text = %q, done = %v; want TypeB kept and Enter passed on", got, *done)
	}
}
