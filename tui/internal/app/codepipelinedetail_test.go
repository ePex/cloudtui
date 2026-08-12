package app

import (
	"context"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/ePex/cloudtui/tui/internal/awscodepipeline"
	"github.com/ePex/cloudtui/tui/internal/config"
)

func TestCodePipelineDetailViewShortcuts(t *testing.T) {
	a := New(config.Default())
	want := []string{"r", "w", "Esc"}
	got := a.codePipelineDetailV.Shortcuts()
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
	a := New(config.Default())

	a.codePipelineDetailV.render([]awscodepipeline.StageStatus{
		{Name: "Source", Status: "Succeeded"},
		{Name: "Build", Status: "InProgress"},
	})

	if got := a.codePipelineDetailV.table.GetRowCount(); got != 3 { // header + 2
		t.Fatalf("row count = %d, want 3", got)
	}
	if got := a.codePipelineDetailV.table.GetCell(1, 0).Text; got != "Source" {
		t.Errorf("row 1 name = %q, want %q", got, "Source")
	}
	if got := a.codePipelineDetailV.table.GetCell(1, 1).Text; got != "Succeeded" {
		t.Errorf("row 1 status = %q, want %q", got, "Succeeded")
	}
	if got := a.codePipelineDetailV.table.GetCell(2, 1).Text; got != "InProgress" {
		t.Errorf("row 2 status = %q, want %q", got, "InProgress")
	}
}

// TestCodePipelineDetailViewRenderShowsNeverRunForEmptyStatus covers
// statusLabel's empty-string path — a stage that hasn't executed yet in
// this pipeline's current run.
func TestCodePipelineDetailViewRenderShowsNeverRunForEmptyStatus(t *testing.T) {
	a := New(config.Default())

	a.codePipelineDetailV.render([]awscodepipeline.StageStatus{{Name: "Deploy", Status: ""}})

	if got := a.codePipelineDetailV.table.GetCell(1, 1).Text; got != "(never run)" {
		t.Errorf("status cell = %q, want %q", got, "(never run)")
	}
}

func TestOpenCodePipelineDetailSwitchesPage(t *testing.T) {
	a := New(config.Default())
	a.cfg.ActiveAWSProfile = "" // load() bails out synchronously, no goroutine spawned

	a.openCodePipelineDetail("my-pipeline")

	if name, _ := a.pages.GetFrontPage(); name != "codepipeline-detail" {
		t.Errorf("front page = %q, want %q", name, "codepipeline-detail")
	}
	if got := a.codePipelineDetailV.pipelineName; got != "my-pipeline" {
		t.Errorf("pipelineName = %q, want %q", got, "my-pipeline")
	}
}

// TestCodePipelineDetailViewToggleWatchUpdatesTitle covers the 'w'
// handler's effect: starts/stops the background watch and updates the
// table title's "▶ watching" suffix (see updateTitle).
func TestCodePipelineDetailViewToggleWatchUpdatesTitle(t *testing.T) {
	a := New(config.Default())
	a.codePipelineDetailV.pipelineName = "my-pipeline"

	a.codePipelineDetailV.toggleWatch()
	if !a.isWatchingPipeline("my-pipeline") {
		t.Fatal("toggleWatch() did not start watching")
	}
	if got := a.codePipelineDetailV.table.GetTitle(); !strings.Contains(got, "▶ watching") {
		t.Errorf("title after starting watch = %q, want it to contain %q", got, "▶ watching")
	}

	a.codePipelineDetailV.toggleWatch()
	if a.isWatchingPipeline("my-pipeline") {
		t.Error("toggleWatch() did not stop watching on second call")
	}
	if got := a.codePipelineDetailV.table.GetTitle(); strings.Contains(got, "▶ watching") {
		t.Errorf("title after stopping watch = %q, want no watching indicator", got)
	}
}

func TestCodePipelineDetailViewShowErrorRendersMessage(t *testing.T) {
	a := New(config.Default())

	a.codePipelineDetailV.showError(context.DeadlineExceeded)

	if got := a.codePipelineDetailV.table.GetCell(1, 0).Text; !strings.Contains(got, "deadline exceeded") {
		t.Errorf("error cell = %q, want it to contain the error", got)
	}
}

// TestCodePipelineDetailViewShowStatusRendersMessage covers the
// in-progress status message load() shows while awsauth.WithReauth is
// running an SSO re-auth (spec/36-fe-aws-sso-reauth).
func TestCodePipelineDetailViewShowStatusRendersMessage(t *testing.T) {
	a := New(config.Default())

	a.codePipelineDetailV.showStatus("AWS SSO session expired — opening browser to log in...")

	if got := a.codePipelineDetailV.table.GetCell(1, 0).Text; !strings.Contains(got, "opening browser") {
		t.Errorf("status cell = %q, want it to contain the status message", got)
	}
	fg, _, _ := a.codePipelineDetailV.table.GetCell(1, 0).Style.Decompose()
	if want := tcell.GetColor(a.cfg.Colors.Accent); fg != want {
		t.Errorf("status cell color = %v, want accent color %v", fg, want)
	}
}

func TestCodePipelineDetailViewEscReturnsToList(t *testing.T) {
	a := New(config.Default())
	a.cfg.ActiveAWSProfile = ""
	a.openCodePipelineDetail("my-pipeline")

	capture := a.codePipelineDetailV.table.GetInputCapture()
	if capture == nil {
		t.Fatal("codePipelineDetailV.table has no input capture set")
	}
	capture(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))

	if name, _ := a.pages.GetFrontPage(); name != "codepipeline" {
		t.Errorf("front page after Esc = %q, want %q", name, "codepipeline")
	}
}
