package view

import (
	"context"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/ePex/cloudtui/tui/internal/awscodepipeline"
)

func newTestCodePipelineDetailView(t *testing.T) (*fakeViewHost, *CodePipelineDetailView) {
	t.Helper()
	host := newFakeViewHost()
	return host, NewCodePipelineDetailView(host, func() {})
}

func TestCodePipelineDetailViewShortcuts(t *testing.T) {
	_, dv := newTestCodePipelineDetailView(t)
	want := []string{"r", "w", "W", "Esc"}
	got := dv.Shortcuts()
	if len(got) != len(want) {
		t.Fatalf("Shortcuts() = %+v, want %d entries", got, len(want))
	}
	for i, k := range want {
		if got[i].Key != k {
			t.Errorf("Shortcuts()[%d].Key = %q, want %q", i, got[i].Key, k)
		}
	}
}

func TestCodePipelineDetailViewRenderShowsStages(t *testing.T) {
	_, dv := newTestCodePipelineDetailView(t)

	dv.Render([]awscodepipeline.StageStatus{
		{Name: "Source", Status: "Succeeded"},
		{Name: "Build", Status: "InProgress"},
	})

	if got := dv.table.GetRowCount(); got != 3 { // header + 2
		t.Fatalf("row count = %d, want 3", got)
	}
	if got := dv.table.GetCell(1, 0).Text; got != "Source" {
		t.Errorf("row 1 name = %q, want %q", got, "Source")
	}
	if got := dv.table.GetCell(1, 1).Text; got != "Succeeded" {
		t.Errorf("row 1 status = %q, want %q", got, "Succeeded")
	}
	if got := dv.table.GetCell(2, 1).Text; got != "InProgress" {
		t.Errorf("row 2 status = %q, want %q", got, "InProgress")
	}
}

// TestCodePipelineDetailViewRenderShowsNeverRunForEmptyStatus covers
// StatusLabel's empty-string path — a stage that hasn't executed yet in
// this pipeline's current run.
func TestCodePipelineDetailViewRenderShowsNeverRunForEmptyStatus(t *testing.T) {
	_, dv := newTestCodePipelineDetailView(t)

	dv.Render([]awscodepipeline.StageStatus{{Name: "Deploy", Status: ""}})

	if got := dv.table.GetCell(1, 1).Text; got != "(never run)" {
		t.Errorf("status cell = %q, want %q", got, "(never run)")
	}
}

func TestCodePipelineDetailViewWrapTogglesAtBottomEdge(t *testing.T) {
	_, dv := newTestCodePipelineDetailView(t)
	dv.Render([]awscodepipeline.StageStatus{
		{Name: "Source", Status: "Succeeded"},
		{Name: "Build", Status: "Succeeded"},
		{Name: "Deploy", Status: "Succeeded"},
	})
	dv.table.Select(3, 0) // last stage row

	capture := dv.table.GetInputCapture()
	capture(tcell.NewEventKey(tcell.KeyRune, 'W', tcell.ModNone))
	capture(tcell.NewEventKey(tcell.KeyRune, 'j', tcell.ModNone))

	if row, _ := dv.table.GetSelection(); row != 1 {
		t.Errorf("selection after wrap = %d, want 1 (wrapped to first stage row)", row)
	}
}

// TestCodePipelineDetailViewToggleWatchUpdatesTitle covers the 'w'
// handler's effect: starts/stops the background watch and updates the
// table title's "▶ watching" suffix (see updateTitle).
func TestCodePipelineDetailViewToggleWatchUpdatesTitle(t *testing.T) {
	host, dv := newTestCodePipelineDetailView(t)
	dv.pipelineName = "my-pipeline"

	dv.toggleWatch()
	if !host.IsWatchingPipeline("my-pipeline") {
		t.Fatal("toggleWatch() did not start watching")
	}
	if got := dv.table.GetTitle(); !strings.Contains(got, "▶ watching") {
		t.Errorf("title after starting watch = %q, want it to contain %q", got, "▶ watching")
	}

	dv.toggleWatch()
	if host.IsWatchingPipeline("my-pipeline") {
		t.Error("toggleWatch() did not stop watching on second call")
	}
	if got := dv.table.GetTitle(); strings.Contains(got, "▶ watching") {
		t.Errorf("title after stopping watch = %q, want no watching indicator", got)
	}
}

func TestCodePipelineDetailViewShowErrorRendersMessage(t *testing.T) {
	_, dv := newTestCodePipelineDetailView(t)

	dv.showError(context.DeadlineExceeded)

	if got := dv.table.GetCell(1, 0).Text; !strings.Contains(got, "deadline exceeded") {
		t.Errorf("error cell = %q, want it to contain the error", got)
	}
}

// TestCodePipelineDetailViewShowStatusRendersMessage covers the
// in-progress status message load() shows while awsauth.WithReauth is
// running an SSO re-auth (spec/36-fe-aws-sso-reauth).
func TestCodePipelineDetailViewShowStatusRendersMessage(t *testing.T) {
	host, dv := newTestCodePipelineDetailView(t)

	dv.showStatus("AWS SSO session expired — opening browser to log in...")

	if got := dv.table.GetCell(1, 0).Text; !strings.Contains(got, "opening browser") {
		t.Errorf("status cell = %q, want it to contain the status message", got)
	}
	fg, _, _ := dv.table.GetCell(1, 0).Style.Decompose()
	if want := tcell.GetColor(host.cfg.Colors.Accent); fg != want {
		t.Errorf("status cell color = %v, want accent color %v", fg, want)
	}
}
