package view

import (
	"reflect"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func TestWrapText(t *testing.T) {
	tests := []struct {
		name  string
		s     string
		width int
		want  []string
	}{
		{"empty string", "", 10, []string{""}},
		{"all whitespace", "   ", 10, []string{""}},
		{"fits on one line", "hello world", 20, []string{"hello world"}},
		{"exact width boundary", "hello world", 11, []string{"hello world"}},
		{"one char over wraps", "hello world", 10, []string{"hello", "world"}},
		{"multi-word wrap", "the quick brown fox jumps", 10, []string{"the quick", "brown fox", "jumps"}},
		{"single word longer than width hard-breaks", "supercalifragilisticexpialidocious", 10, []string{"supercalif", "ragilistic", "expialidoc", "ious"}},
		{"leading and trailing whitespace trimmed", "  hi there  ", 20, []string{"hi there"}},
		{"long word mid-stream flushes current line first", "hi supercalifragilisticexpialidocious", 10, []string{"hi", "supercalif", "ragilistic", "expialidoc", "ious"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wrapText(tt.s, tt.width)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("wrapText(%q, %d) = %#v, want %#v", tt.s, tt.width, got, tt.want)
			}
		})
	}
}

func TestWrapTextZeroWidthReturnsUnchanged(t *testing.T) {
	got := wrapText("hello world", 0)
	want := []string{"hello world"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("wrapText(_, 0) = %#v, want %#v", got, want)
	}
}

func TestSetContinuationRow(t *testing.T) {
	table := tview.NewTable()
	setContinuationRow(table, 3, 4, 2, "wrapped text", tcell.ColorWhite, 3)

	for col := 0; col < 4; col++ {
		if !table.GetCell(3, col).NotSelectable {
			t.Errorf("column %d: NotSelectable = false, want true", col)
		}
	}

	textCell := table.GetCell(3, 2)
	if got := textCell.Text; got != "wrapped text" {
		t.Errorf("text column text = %q, want %q", got, "wrapped text")
	}
	if got := textCell.Expansion; got != 3 {
		t.Errorf("text column expansion = %d, want 3", got)
	}

	for _, col := range []int{0, 1, 3} {
		if got := table.GetCell(3, col).Text; got != "" {
			t.Errorf("non-text column %d text = %q, want empty", col, got)
		}
	}
}
