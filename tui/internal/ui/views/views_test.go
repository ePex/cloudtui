package views

import (
	"strings"
	"testing"

	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/ui"
)

func TestViewConstructors(t *testing.T) {
	tests := []struct {
		name        string
		constructor func() ui.View
		wantName    string
		wantTitle   string
	}{
		{"home", NewHome, "home", "Home"},
		{"secrets", NewSecrets, "secrets", "Secrets Manager"},
		{"params", NewParams, "params", "Parameter Store"},
		{"queues", NewQueues, "queues", "Queues"},
		{"settings", NewSettings, "settings", "Settings"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := tt.constructor()
			if got := v.Name(); got != tt.wantName {
				t.Errorf("Name() = %q, want %q", got, tt.wantName)
			}
			if got := v.Title(); got != tt.wantTitle {
				t.Errorf("Title() = %q, want %q", got, tt.wantTitle)
			}
		})
	}
}

func TestPlaceholderPrimitive(t *testing.T) {
	v := NewSecrets()
	prim := v.Primitive()

	tv, ok := prim.(*tview.TextView)
	if !ok {
		t.Fatalf("Primitive() = %T, want *tview.TextView", prim)
	}

	if got, want := tv.GetTitle(), " Secrets Manager "; got != want {
		t.Errorf("GetTitle() = %q, want %q", got, want)
	}

	if text := tv.GetText(true); !strings.Contains(text, "not yet implemented") {
		t.Errorf("GetText(true) = %q, want it to contain %q", text, "not yet implemented")
	}
}
