package app

import "github.com/rivo/tview"

// statusReadyText is shown when no async operation is in flight.
const statusReadyText = "cloudtui ready"

// newStatusBar builds the bottom row: a single-line, unbordered strip
// used for minimal transient status (loading indicators, errors).
func newStatusBar() *tview.TextView {
	return tview.NewTextView().SetText(statusReadyText)
}
