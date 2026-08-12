package app

import (
	"context"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/awscodepipeline"
	"github.com/ePex/cloudtui/tui/internal/config"
)

func TestCodePipelineListViewNameAndTitle(t *testing.T) {
	a := New(config.Default())
	if got := a.codePipelineListV.Name(); got != "codepipeline" {
		t.Errorf("Name() = %q, want %q", got, "codepipeline")
	}
	if got := a.codePipelineListV.Title(); got != "AWS CodePipeline" {
		t.Errorf("Title() = %q, want %q", got, "AWS CodePipeline")
	}
}

func TestCodePipelineListViewHeaderLabels(t *testing.T) {
	a := New(config.Default())
	want := []string{"NAME", "WATCHING", "UPDATED"}
	for col, label := range want {
		cell := a.codePipelineListV.table.GetCell(0, col)
		if cell == nil {
			t.Fatalf("header cell at column %d is nil", col)
		}
		if got := cell.Text; got != label {
			t.Errorf("header col %d = %q, want %q", col, got, label)
		}
	}
}

// TestCodePipelineListViewLoadErrorsWithoutActiveProfile exercises load()'s
// synchronous guard, which returns before spawning the fetch goroutine —
// safe to call directly in a test, unlike the goroutine+QueueUpdateDraw
// path itself (which needs a running tview event loop to ever complete;
// see logs_test.go's equivalent test for the same reasoning).
func TestCodePipelineListViewLoadErrorsWithoutActiveProfile(t *testing.T) {
	a := New(config.Default())
	a.cfg.ActiveAWSProfile = ""
	calls := 0
	a.listPipelines = func(context.Context, string) ([]awscodepipeline.Pipeline, error) {
		calls++
		return nil, nil
	}

	a.codePipelineListV.load()

	if calls != 0 {
		t.Error("listPipelines was called despite no active AWS profile")
	}
	if got := a.codePipelineListV.table.GetCell(1, 0).Text; !strings.Contains(got, "no AWS profile selected") {
		t.Errorf("error cell = %q, want it to mention no profile selected", got)
	}
}

func TestCodePipelineListViewRepaintPopulatesRows(t *testing.T) {
	a := New(config.Default())

	a.codePipelineListV.repaint([]awscodepipeline.Pipeline{
		{Name: "pipeline-one"},
		{Name: "pipeline-two"},
	})

	if got := a.codePipelineListV.table.GetRowCount(); got != 3 { // header + 2
		t.Fatalf("row count = %d, want 3", got)
	}
	if got := a.codePipelineListV.table.GetCell(1, 0).Text; got != "pipeline-one" {
		t.Errorf("row 1 name = %q, want %q", got, "pipeline-one")
	}
	if got := a.codePipelineListV.table.GetTitle(); got != " AWS CodePipeline (2) " {
		t.Errorf("title = %q, want %q", got, " AWS CodePipeline (2) ")
	}
}

func TestCodePipelineListViewRepaintShowsDashForNoUpdatedAt(t *testing.T) {
	a := New(config.Default())

	a.codePipelineListV.repaint([]awscodepipeline.Pipeline{{Name: "x"}})

	if got := a.codePipelineListV.table.GetCell(1, 2).Text; got != "-" {
		t.Errorf("updated cell = %q, want %q", got, "-")
	}
}

// TestCodePipelineListViewRepaintShowsWatchingIndicator covers the
// WATCHING column — populated from App.isWatchingPipeline, not from any
// field on awscodepipeline.Pipeline itself.
func TestCodePipelineListViewRepaintShowsWatchingIndicator(t *testing.T) {
	a := New(config.Default())
	a.watchedPipelines["pipeline-one"] = make(chan struct{})

	a.codePipelineListV.repaint([]awscodepipeline.Pipeline{
		{Name: "pipeline-one"},
		{Name: "pipeline-two"},
	})

	if got := a.codePipelineListV.table.GetCell(1, 1).Text; got != "▶ watching" {
		t.Errorf("watched row's WATCHING cell = %q, want %q", got, "▶ watching")
	}
	if got := a.codePipelineListV.table.GetCell(2, 1).Text; got != "" {
		t.Errorf("unwatched row's WATCHING cell = %q, want empty", got)
	}
}

func TestCodePipelineListViewFilterNarrowsRowsByName(t *testing.T) {
	a := New(config.Default())
	a.codePipelineListV.repaint([]awscodepipeline.Pipeline{
		{Name: "db-writer"},
		{Name: "db-reader"},
		{Name: "ecs-other"},
	})

	a.codePipelineListV.applyFilter("db")

	if got := a.codePipelineListV.table.GetRowCount(); got != 3 { // header + 2 matches
		t.Fatalf("row count after filter = %d, want 3", got)
	}
	if got := a.codePipelineListV.table.GetTitle(); got != " AWS CodePipeline (db) " {
		t.Errorf("title after filter = %q, want %q", got, " AWS CodePipeline (db) ")
	}
}

func TestCodePipelineListViewFilterClearRestoresAll(t *testing.T) {
	a := New(config.Default())
	a.codePipelineListV.repaint([]awscodepipeline.Pipeline{{Name: "a"}, {Name: "b"}})
	a.codePipelineListV.applyFilter("a")

	a.codePipelineListV.applyFilter("")

	if got := a.codePipelineListV.table.GetRowCount(); got != 3 {
		t.Errorf("row count after clearing filter = %d, want 3", got)
	}
}

// TestCodePipelineListViewToggleWatchSelectedStartsAndStops exercises
// toggleWatchSelected directly against the table's current selection,
// mirroring how the 'w' key handler invokes it.
func TestCodePipelineListViewToggleWatchSelectedStartsAndStops(t *testing.T) {
	a := New(config.Default())
	a.codePipelineListV.repaint([]awscodepipeline.Pipeline{{Name: "pipeline-one"}})
	a.codePipelineListV.table.Select(1, 0)

	a.codePipelineListV.toggleWatchSelected()
	if !a.isWatchingPipeline("pipeline-one") {
		t.Fatal("toggleWatchSelected() did not start watching")
	}
	if got := a.codePipelineListV.table.GetCell(1, 1).Text; got != "▶ watching" {
		t.Errorf("WATCHING cell after starting = %q, want %q", got, "▶ watching")
	}

	a.codePipelineListV.toggleWatchSelected()
	if a.isWatchingPipeline("pipeline-one") {
		t.Error("toggleWatchSelected() did not stop watching on second call")
	}
	if got := a.codePipelineListV.table.GetCell(1, 1).Text; got != "" {
		t.Errorf("WATCHING cell after stopping = %q, want empty", got)
	}
}

func TestCodePipelineListViewShowErrorRendersMessage(t *testing.T) {
	a := New(config.Default())

	a.codePipelineListV.showError(context.DeadlineExceeded)

	if got := a.codePipelineListV.table.GetCell(1, 0).Text; !strings.Contains(got, "deadline exceeded") {
		t.Errorf("error cell = %q, want it to contain the error", got)
	}
}

// TestCodePipelineListViewShowStatusRendersMessage covers the in-progress
// status message load() shows while awsauth.WithReauth is running an SSO
// re-auth (spec/36-fe-aws-sso-reauth) — see logs_test.go's equivalent test.
func TestCodePipelineListViewShowStatusRendersMessage(t *testing.T) {
	a := New(config.Default())

	a.codePipelineListV.showStatus("AWS SSO session expired — opening browser to log in...")

	if got := a.codePipelineListV.table.GetCell(1, 0).Text; !strings.Contains(got, "opening browser") {
		t.Errorf("status cell = %q, want it to contain the status message", got)
	}
	fg, _, _ := a.codePipelineListV.table.GetCell(1, 0).Style.Decompose()
	if want := tcell.GetColor(a.cfg.Colors.Accent); fg != want {
		t.Errorf("status cell color = %v, want accent color %v", fg, want)
	}
}

// TestCodePipelineListViewSelectedFuncOpensDetail covers the
// SetSelectedFunc wiring done in New() — Enter on a row opens the
// stage-status detail view for that pipeline (mapped through the current
// filter, not the raw index).
func TestCodePipelineListViewSelectedFuncOpensDetail(t *testing.T) {
	a := New(config.Default())
	a.codePipelineListV.repaint([]awscodepipeline.Pipeline{
		{Name: "pipeline-one"},
		{Name: "pipeline-two"},
	})
	a.codePipelineListV.applyFilter("two") // only "pipeline-two" remains, at row 1

	a.codePipelineListV.table.Select(1, 0)
	// Invoke the table's registered SetSelectedFunc handler directly.
	handler := a.codePipelineListV.table.InputHandler()
	handler(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(tview.Primitive) {})

	if name, _ := a.pages.GetFrontPage(); name != "codepipeline-detail" {
		t.Errorf("front page = %q, want %q", name, "codepipeline-detail")
	}
	if got := a.codePipelineDetailV.pipelineName; got != "pipeline-two" {
		t.Errorf("opened detail for %q, want %q", got, "pipeline-two")
	}
}
