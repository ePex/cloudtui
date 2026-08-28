package dialog

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// sectionHeaderItem is a non-interactive, full-width divider row used to
// group a tview.Form's fields into visual sections (e.g. the connection
// editor's General/Destination/Auth). It's a bespoke tview.FormItem
// rather than a tview.Form.AddTextView row: TextView's own
// SetFormAttributes unconditionally reserves the form's shared
// label-width column (whatever the longest field label needs) before
// drawing anything — text placed in TextView's body renders indented to
// that column, and text placed in its label is truncated to that same
// column's width. Neither lets a header both start flush at the row's
// left edge and span the row's full width. This type ignores the
// shared label width entirely and always draws across its full
// allotted row.
type sectionHeaderItem struct {
	*tview.Box
	title    string
	color    tcell.Color
	finished func(key tcell.Key)
}

// newSectionHeader builds a section header row titled title (rendered
// as "── title ──" padded with dashes to fill the row).
func newSectionHeader(title string) *sectionHeaderItem {
	return &sectionHeaderItem{Box: tview.NewBox(), title: title}
}

// Draw draws the header text across the item's full inner width,
// padding with "─" so it spans the row regardless of the modal's size.
func (s *sectionHeaderItem) Draw(screen tcell.Screen) {
	s.Box.DrawForSubclass(screen, s)
	x, y, width, height := s.GetInnerRect()
	if width <= 0 || height <= 0 {
		return
	}
	prefix := "── " + s.title + " "
	text := prefix
	if pad := width - len([]rune(prefix)); pad > 0 {
		text = prefix + strings.Repeat("─", pad)
	}
	tview.Print(screen, text, x, y, width, tview.AlignLeft, s.color)
}

// GetLabel returns "" — a section header has no field label; its title
// is drawn directly by Draw, not through the form's shared label
// column (see the type doc comment).
func (s *sectionHeaderItem) GetLabel() string { return "" }

// SetFormAttributes stores the label color (used for the header text,
// matching every other field's label color) and background color.
// labelWidth is intentionally ignored — see the type doc comment.
func (s *sectionHeaderItem) SetFormAttributes(labelWidth int, labelColor, bgColor, fieldTextColor, fieldBgColor tcell.Color) tview.FormItem {
	s.color = labelColor
	s.SetBackgroundColor(bgColor)
	return s
}

// GetFieldWidth returns 0 (flexible) — irrelevant in practice, since a
// vertical tview.Form always gives every item the form's full width
// regardless of this value.
func (s *sectionHeaderItem) GetFieldWidth() int { return 0 }

// GetFieldHeight returns 1 — a header is always a single row.
func (s *sectionHeaderItem) GetFieldHeight() int { return 1 }

// SetFinishedFunc stores handler, called by Focus (with a negative key,
// meaning "repeat whatever the last real key press was") to skip a
// header transparently when Tab/Backtab would otherwise land on it —
// the same mechanism tview.TextView uses for a non-scrollable,
// Form-embedded instance (see TextView.Focus).
func (s *sectionHeaderItem) SetFinishedFunc(handler func(key tcell.Key)) tview.FormItem {
	s.finished = handler
	return s
}

// SetDisabled is a no-op — a section header is never interactive
// regardless, so there's no separate disabled state to track.
func (s *sectionHeaderItem) SetDisabled(disabled bool) tview.FormItem { return s }

// Focus immediately replays the last Tab/Backtab via the finished
// callback instead of actually taking focus, so tabbing through the
// form skips straight over this header. Falls back to the normal
// Box.Focus when finished hasn't been wired yet (matching
// TextView.Focus's own fallback), which only matters before this item
// has ever been part of a form that received focus at least once.
func (s *sectionHeaderItem) Focus(delegate func(p tview.Primitive)) {
	if s.finished != nil {
		s.finished(-1)
		return
	}
	s.Box.Focus(delegate)
}

// InputHandler returns nil: a section header never has focus (see
// Focus), so it never needs to handle a key event itself.
func (s *sectionHeaderItem) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return nil
}

var _ tview.FormItem = (*sectionHeaderItem)(nil)
