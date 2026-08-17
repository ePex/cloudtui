package app

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// movePicker is the "Move to Queue" overlay: a searchable list of queues
// used to pick a target for a message move.
type movePicker struct {
	host      ui.Host
	flex      *tview.Flex
	list      *tview.List
	search    *tview.InputField
	queues    []string // full unfiltered list from the last load, sorted by sortPickerQueues
	preferred string   // the requeue-corresponding queue, shown with ⭐
	onSelect  func(string)
	onClose   func()
	visible   bool
}

// newMovePicker builds the move-picker overlay's widgets.
func newMovePicker(host ui.Host) *movePicker {
	mp := &movePicker{host: host}
	mp.list = tview.NewList().ShowSecondaryText(false)
	mp.search = tview.NewInputField().SetLabel(" / filter: ")
	mp.flex = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(mp.list, 0, 1, true).
		AddItem(mp.search, 1, 0, false)
	mp.flex.SetBorder(true).SetTitle(" Move to Queue ")

	// SetChangedFunc is registered in show (needs sourceQueue/msg closure).
	mp.search.SetDoneFunc(func(_ tcell.Key) {
		host.SetFocus(mp.list)
	})
	mp.search.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			mp.search.SetText("")
			host.SetFocus(mp.list)
			return nil
		}
		return event
	})
	return mp
}

// isSystemQueue reports whether name belongs to a built-in AMQ system
// destination (activemq.*, statistics.*).
func isSystemQueue(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasPrefix(lower, "activemq.") || strings.HasPrefix(lower, "statistics.")
}

// isDLQQueue reports whether name is a dead-letter queue (dlq.*).
func isDLQQueue(name string) bool {
	return strings.HasPrefix(strings.ToLower(name), "dlq.")
}

// isIMQQueue reports whether name is an in-memory / internal queue (imq.*).
func isIMQQueue(name string) bool {
	return strings.HasPrefix(strings.ToLower(name), "imq.")
}

// requeueQueueCandidate returns the non-prefixed counterpart of sourceQueue if
// it has a requeue prefix (dlq. or imq.), otherwise "".
// E.g. "dlq.foo.bar" → "foo.bar", "imq.foo.bar" → "foo.bar".
func requeueQueueCandidate(sourceQueue string) string {
	lower := strings.ToLower(sourceQueue)
	if strings.HasPrefix(lower, "dlq.") || strings.HasPrefix(lower, "imq.") {
		return sourceQueue[4:]
	}
	return ""
}

// sortPickerQueues orders names into four tiers:
//  1. Preferred — the non-prefixed counterpart when source is a dlq.* or imq.*
//     queue (e.g. "dlq.foo" → "foo", "imq.foo" → "foo"). Pinned first with ⭐.
//  2. Regular — all other queues, alphabetical.
//  3. Requeue — dlq.* and imq.* queues, alphabetical (➖).
//  4. System — activemq.* and statistics.* queues, alphabetical (❓).
func sortPickerQueues(sourceQueue string, names []string) []string {
	var preferred string
	var regular, requeue, system []string

	candidateLower := strings.ToLower(requeueQueueCandidate(sourceQueue))

	for _, name := range names {
		lower := strings.ToLower(name)
		switch {
		case candidateLower != "" && lower == candidateLower:
			preferred = name
		case isSystemQueue(name):
			system = append(system, name)
		case isDLQQueue(name) || isIMQQueue(name):
			requeue = append(requeue, name)
		default:
			regular = append(regular, name)
		}
	}

	sort.Strings(regular)
	sort.Strings(requeue)
	sort.Strings(system)

	result := make([]string, 0, len(names))
	if preferred != "" {
		result = append(result, preferred)
	}
	result = append(result, regular...)
	result = append(result, requeue...)
	result = append(result, system...)
	return result
}

// show opens the queue-picker overlay. onSelect is called (in a new
// goroutine) with the chosen target queue name when the user makes a
// selection. onClose is called (on the UI goroutine) when the picker is
// dismissed — the caller uses it to restore focus and the context panel.
func (mp *movePicker) show(sourceQueue string, onSelect func(string), onClose func()) {
	host := mp.host
	mp.onSelect = onSelect
	mp.onClose = onClose
	mp.list.Clear()
	mp.list.AddItem("Loading…", "", 0, nil)
	mp.search.SetText("")

	mp.list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case event.Rune() == 'j':
			return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		case event.Rune() == 'k':
			return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
		case event.Rune() == '/':
			host.SetFocus(mp.search)
			return nil
		case event.Key() == tcell.KeyEscape:
			mp.close()
			return nil
		}
		return event
	})

	mp.search.SetChangedFunc(func(text string) {
		mp.fillList(text)
	})

	host.ShowPage("move-picker")
	host.SetFocus(mp.list)
	mp.visible = true
	ac := host.Config().Colors.Accent
	host.SetContextHint(fmt.Sprintf("[%s]<Esc>[-] cancel  [%s]</>[-] search", ac, ac))

	go func() {
		summaries, err := host.Backend().List(context.Background())
		host.QueueUpdateDraw(func() {
			if err != nil {
				slog.Error("move-picker: failed to list queues", "error", err)
				mp.list.Clear()
				mp.list.AddItem("Error loading queues", "", 0, nil)
				return
			}
			names := make([]string, 0, len(summaries))
			for _, s := range summaries {
				if s.Name != sourceQueue {
					names = append(names, s.Name)
				}
			}
			mp.queues = sortPickerQueues(sourceQueue, names)
			// Determine the preferred (requeue-corresponding) queue for ⭐ display.
			mp.preferred = ""
			if candidate := strings.ToLower(requeueQueueCandidate(sourceQueue)); candidate != "" {
				for _, name := range names {
					if strings.ToLower(name) == candidate {
						mp.preferred = name
						break
					}
				}
			}
			mp.fillList("")
		})
	}()
}

// fillList repopulates list from queues, keeping only entries whose name
// contains filter (case-insensitive; empty = show all). Each item's
// selected func invokes onSelect in a new goroutine.
func (mp *movePicker) fillList(filter string) {
	mp.list.Clear()
	lower := strings.ToLower(filter)
	for _, name := range mp.queues {
		if lower != "" && !strings.Contains(strings.ToLower(name), lower) {
			continue
		}
		n := name
		var displayName string
		switch {
		case n == mp.preferred:
			displayName = "⭐ " + n
		case isDLQQueue(n) || isIMQQueue(n):
			displayName = "➖ " + n
		case isSystemQueue(n):
			displayName = "❓ " + n
		default:
			displayName = n
		}
		mp.list.AddItem(displayName, "", 0, func() {
			mp.close()
			go mp.onSelect(n)
		})
	}
}

// close hides the queue-picker overlay and calls onClose to let the
// caller restore focus and the context panel.
func (mp *movePicker) close() {
	mp.host.HidePage("move-picker")
	mp.visible = false
	if mp.onClose != nil {
		mp.onClose()
	}
}

// ApplyPalette recolors the move-picker overlay for a live theme switch.
func (mp *movePicker) ApplyPalette(p config.Palette) {
	bg := tcell.GetColor(p.Background)
	mp.flex.SetBackgroundColor(bg)
	mp.flex.SetBorderColor(tcell.GetColor(p.Border))
	mp.flex.SetTitleColor(tcell.GetColor(p.Border))
	ui.StyleList(mp.list, p)
	mp.list.SetBackgroundColor(bg)
	mp.search.SetBackgroundColor(bg)
	mp.search.SetLabelColor(tcell.GetColor(p.Label))
	mp.search.SetFieldBackgroundColor(tcell.GetColor(p.SelectionBg))
	mp.search.SetFieldTextColor(tcell.GetColor(p.SelectionText))
}

var _ ui.Themeable = (*movePicker)(nil)
