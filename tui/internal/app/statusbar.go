package app

import "github.com/rivo/tview"

// statusPlaceholderText is shown until real async operations (queue
// browse/send/purge/move, secret/parameter fetches) exist to report on.
const statusPlaceholderText = "cloudtui ready"

// newStatusBar builds the bottom row: a single-line, unbordered strip
// reserved for minimal transient status (loading indicators, progress).
func newStatusBar() *tview.TextView {
	return tview.NewTextView().SetText(statusPlaceholderText)
}
