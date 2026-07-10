package app

import "testing"

func TestNewStatusBar(t *testing.T) {
	tv := newStatusBar()

	if got := tv.GetText(true); got != statusReadyText {
		t.Errorf("status bar text = %q, want %q", got, statusReadyText)
	}
}
