package view

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
)

// favoriteStar is the glyph shown in the star column for a favorited row.
const favoriteStar = "★"

// sortFavoritesFirst stably reorders items so the ones isFavorite reports
// true for come first, preserving each group's relative order — a
// favorite-status partition, not a full sort, since none of SSM
// Parameters/Secrets Manager/CloudWatch Logs expose a column-sort toggle
// (unlike Queues' o/O) for favorite status to layer on top of.
func sortFavoritesFirst[T any](items []T, isFavorite func(T) bool) []T {
	out := make([]T, 0, len(items))
	var rest []T
	for _, item := range items {
		if isFavorite(item) {
			out = append(out, item)
		} else {
			rest = append(rest, item)
		}
	}
	return append(out, rest...)
}

// favoriteCell builds the star column's cell for one row: the star glyph
// in the palette's accent color when favorited, blank otherwise.
func favoriteCell(favorited bool, p config.Palette) *tview.TableCell {
	text := ""
	if favorited {
		text = favoriteStar
	}
	return tview.NewTableCell(text).
		SetTextColor(tcell.GetColor(p.Accent)).
		SetAlign(tview.AlignCenter)
}
