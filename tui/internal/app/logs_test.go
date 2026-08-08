package app

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/ePex/cloudtui/tui/internal/awslogs"
	"github.com/ePex/cloudtui/tui/internal/config"
)

func TestLogsViewNameAndTitle(t *testing.T) {
	a := New(config.Default())
	if got := a.logsV.Name(); got != "cloudwatch-logs" {
		t.Errorf("Name() = %q, want %q", got, "cloudwatch-logs")
	}
	if got := a.logsV.Title(); got != "CloudWatch Logs" {
		t.Errorf("Title() = %q, want %q", got, "CloudWatch Logs")
	}
}

func TestLogsViewHeaderLabels(t *testing.T) {
	a := New(config.Default())
	want := []string{"NAME", "RETENTION", "CREATED"}
	for col, label := range want {
		cell := a.logsV.table.GetCell(0, col)
		if cell == nil {
			t.Fatalf("header cell at column %d is nil", col)
		}
		if got := cell.Text; got != label {
			t.Errorf("header col %d = %q, want %q", col, got, label)
		}
	}
}

// TestLogsViewLoadErrorsWithoutActiveProfile exercises load()'s
// synchronous guard, which returns before spawning the fetch goroutine —
// safe to call directly in a test, unlike the goroutine+QueueUpdateDraw
// path itself (which needs a running tview event loop to ever complete;
// see ssmParamsView/secretsView's tests for the same reasoning).
func TestLogsViewLoadErrorsWithoutActiveProfile(t *testing.T) {
	a := New(config.Default())
	a.cfg.ActiveAWSProfile = ""
	calls := 0
	a.listLogGroups = func(context.Context, string) ([]awslogs.LogGroup, error) {
		calls++
		return nil, nil
	}

	a.logsV.load()

	if calls != 0 {
		t.Error("listLogGroups was called despite no active AWS profile")
	}
	if got := a.logsV.table.GetCell(1, 0).Text; !strings.Contains(got, "no AWS profile selected") {
		t.Errorf("error cell = %q, want it to mention no profile selected", got)
	}
}

func TestLogsViewRepaintPopulatesRows(t *testing.T) {
	a := New(config.Default())

	a.logsV.repaint([]awslogs.LogGroup{
		{Name: "/aws/lambda/one", RetentionDays: 14},
		{Name: "/aws/lambda/two", RetentionDays: 0},
	})

	if got := a.logsV.table.GetRowCount(); got != 3 { // header + 2
		t.Fatalf("row count = %d, want 3", got)
	}
	if got := a.logsV.table.GetCell(1, 0).Text; got != "/aws/lambda/one" {
		t.Errorf("row 1 name = %q, want %q", got, "/aws/lambda/one")
	}
	if got := a.logsV.table.GetCell(1, 1).Text; got != "14 days" {
		t.Errorf("row 1 retention = %q, want %q", got, "14 days")
	}
	if got := a.logsV.table.GetCell(2, 1).Text; got != "never" {
		t.Errorf("row 2 retention = %q, want %q", got, "never")
	}
	if got := a.logsV.table.GetTitle(); got != " CloudWatch Logs (2) " {
		t.Errorf("title = %q, want %q", got, " CloudWatch Logs (2) ")
	}
}

func TestLogsViewRepaintShowsDashForNoCreatedAt(t *testing.T) {
	a := New(config.Default())

	a.logsV.repaint([]awslogs.LogGroup{{Name: "/x"}})

	if got := a.logsV.table.GetCell(1, 2).Text; got != "-" {
		t.Errorf("created cell = %q, want %q", got, "-")
	}
}

func TestLogsViewFilterNarrowsRowsByName(t *testing.T) {
	a := New(config.Default())
	a.logsV.repaint([]awslogs.LogGroup{
		{Name: "/aws/lambda/db-writer"},
		{Name: "/aws/lambda/db-reader"},
		{Name: "/aws/ecs/other"},
	})

	a.logsV.applyFilter("db")

	if got := a.logsV.table.GetRowCount(); got != 3 { // header + 2 matches
		t.Fatalf("row count after filter = %d, want 3", got)
	}
	if got := a.logsV.table.GetTitle(); got != " CloudWatch Logs (db) " {
		t.Errorf("title after filter = %q, want %q", got, " CloudWatch Logs (db) ")
	}
}

// TestLogsViewFilteredTitleActuallyRenders is the render-based companion
// to the title-format fix: GetTitle() alone wouldn't have caught the
// bracket-swallowing bug FE 32 found (see queues_test.go's
// renderedScreenText doc comment).
func TestLogsViewFilteredTitleActuallyRenders(t *testing.T) {
	a := New(config.Default())
	a.logsV.repaint([]awslogs.LogGroup{
		{Name: "/aws/lambda/db-writer"},
	})
	a.logsV.applyFilter("db")

	rendered := renderedScreenText(t, a.logsV.table, 60, 10)
	if !strings.Contains(rendered, "db") {
		t.Errorf("rendered screen = %q, want it to contain the filter text %q", rendered, "db")
	}
}

func TestLogsViewFilterClearRestoresAll(t *testing.T) {
	a := New(config.Default())
	a.logsV.repaint([]awslogs.LogGroup{{Name: "/a"}, {Name: "/b"}})
	a.logsV.applyFilter("a")

	a.logsV.applyFilter("")

	if got := a.logsV.table.GetRowCount(); got != 3 {
		t.Errorf("row count after clearing filter = %d, want 3", got)
	}
}

func TestLogsViewSelectedFuncMapsThroughFilter(t *testing.T) {
	a := New(config.Default())
	a.logsV.repaint([]awslogs.LogGroup{
		{Name: "/aws/lambda/db-writer"},
		{Name: "/aws/lambda/other"},
	})
	a.logsV.applyFilter("other") // only "/aws/lambda/other" remains, at row 1

	if len(a.logsV.filtered) != 1 || a.logsV.filtered[0].Name != "/aws/lambda/other" {
		t.Fatalf("filtered = %+v, want exactly [/aws/lambda/other]", a.logsV.filtered)
	}
}

func TestLogsViewShowErrorRendersMessage(t *testing.T) {
	a := New(config.Default())

	a.logsV.showError(context.DeadlineExceeded)

	if got := a.logsV.table.GetCell(1, 0).Text; !strings.Contains(got, "deadline exceeded") {
		t.Errorf("error cell = %q, want it to contain the error", got)
	}
}

// TestLogsViewRepaintScrollsToTopWithManyRows guards against the bug
// fixed for queuesView (spec/11-bugfix-queues-scroll-to-top):
// tview.Table's "track end" auto-scroll latches on during the table's
// first, still-empty draw and stays latched through repaint, scrolling a
// long list to the bottom instead of the top.
func TestLogsViewRepaintScrollsToTopWithManyRows(t *testing.T) {
	a := New(config.Default())
	table := a.logsV.table
	table.SetRect(0, 0, 60, 15) // fewer visible rows than groups below

	screen := tcell.NewSimulationScreen("")
	if err := screen.Init(); err != nil {
		t.Fatalf("screen.Init: %v", err)
	}
	screen.SetSize(60, 15)

	// First draw while the table is still empty (header only), mirroring
	// the real sequence: switchTo("cloudwatch-logs") draws before the
	// async load returns.
	table.Draw(screen)

	groups := make([]awslogs.LogGroup, 50)
	for i := range groups {
		groups[i] = awslogs.LogGroup{Name: fmt.Sprintf("/aws/lambda/group-%02d", i)}
	}
	a.logsV.repaint(groups)

	// The redraw that follows repaint via QueueUpdateDraw.
	table.Draw(screen)

	if row, _ := table.GetOffset(); row != 0 {
		t.Errorf("table scrolled away from top: rowOffset = %d, want 0", row)
	}
}
