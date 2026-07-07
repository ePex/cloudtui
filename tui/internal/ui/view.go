// Package ui defines the shared contract between the app shell and resource views.
package ui

import "github.com/rivo/tview"

// View is a single resource screen (secrets, params, queues, ...) that the
// app shell can switch to from the command prompt.
type View interface {
	// Name is the command prompt token that activates this view, e.g. "secrets".
	Name() string
	// Title is the human-readable heading shown above the view.
	Title() string
	// Primitive returns the tview primitive to display for this view.
	Primitive() tview.Primitive
}
