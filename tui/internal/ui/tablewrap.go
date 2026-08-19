package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// TableWrap tracks a per-table wrap-around toggle and applies it to
// up/down navigation. The zero value has wrap disabled, matching every
// list view's default (clamped) navigation.
type TableWrap struct {
	enabled bool
}

// Enabled reports whether wrap-around is currently on.
func (w *TableWrap) Enabled() bool { return w.enabled }

// Toggle flips wrap-around on/off.
func (w *TableWrap) Toggle() { w.enabled = !w.enabled }

// HandleNav intercepts an up/down navigation key (KeyUp/KeyDown, or the
// 'j'/'k' vim aliases every list view supports) for table. headerRows is
// the number of non-selectable fixed header rows at the top (today
// always 1 across every list view).
//
// It returns the event to forward to the table's own InputHandler: nil
// if it fully handled the navigation itself (because it wrapped),
// otherwise a normalized KeyUp/KeyDown event so 'j'/'k' keep working as
// arrow-key aliases exactly as they do today. Callers should only invoke
// this for events already known to be nav keys — it does not itself
// filter out non-nav events.
func (w *TableWrap) HandleNav(table *tview.Table, headerRows int, event *tcell.EventKey) *tcell.EventKey {
	down := event.Rune() == 'j' || event.Key() == tcell.KeyDown
	normalized := tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
	if down {
		normalized = tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
	}

	if !w.enabled {
		return normalized
	}

	lastRow := table.GetRowCount() - 1
	if lastRow < headerRows {
		return normalized // no selectable data rows to wrap between
	}

	row, col := table.GetSelection()
	switch {
	case down && row >= lastRow:
		table.Select(headerRows, col)
		return nil
	case !down && row <= headerRows:
		table.Select(lastRow, col)
		return nil
	}
	return normalized
}
