package app

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/ePex/cloudtui/tui/internal/awsssm"
	"github.com/ePex/cloudtui/tui/internal/config"
)

func TestSSMParamsViewNameAndTitle(t *testing.T) {
	a := New(config.Default())
	if got := a.ssmParamsV.Name(); got != "ssm-parameters" {
		t.Errorf("Name() = %q, want %q", got, "ssm-parameters")
	}
	if got := a.ssmParamsV.Title(); got != "SSM Parameters" {
		t.Errorf("Title() = %q, want %q", got, "SSM Parameters")
	}
}

func TestSSMParamsViewHeaderLabels(t *testing.T) {
	a := New(config.Default())
	want := []string{"NAME", "TYPE", "LAST MODIFIED"}
	for col, label := range want {
		cell := a.ssmParamsV.table.GetCell(0, col)
		if cell == nil {
			t.Fatalf("header cell at column %d is nil", col)
		}
		if got := cell.Text; got != label {
			t.Errorf("header col %d = %q, want %q", col, got, label)
		}
	}
}

// TestSSMParamsViewLoadErrorsWithoutActiveProfile exercises load()'s
// synchronous guard, which returns before spawning the fetch goroutine —
// safe to call directly in a test, unlike the goroutine+QueueUpdateDraw
// path itself (which needs a running tview event loop to ever complete;
// see queuesView/messagesView's tests, which likewise only ever test
// repaint()/showError() directly rather than load()).
func TestSSMParamsViewLoadErrorsWithoutActiveProfile(t *testing.T) {
	a := New(config.Default())
	a.cfg.ActiveAWSProfile = ""
	calls := 0
	a.listParameters = func(context.Context, string, string) ([]awsssm.Parameter, error) {
		calls++
		return nil, nil
	}

	a.ssmParamsV.load()

	if calls != 0 {
		t.Error("listParameters was called despite no active AWS profile")
	}
	if got := a.ssmParamsV.table.GetCell(1, 0).Text; !strings.Contains(got, "no AWS profile selected") {
		t.Errorf("error cell = %q, want it to mention no profile selected", got)
	}
}

func TestSSMParamsViewRepaintPopulatesRows(t *testing.T) {
	a := New(config.Default())

	a.ssmParamsV.repaint([]awsssm.Parameter{
		{Name: "/app/one", Type: awsssm.TypeString, Value: "x"},
		{Name: "/app/two", Type: awsssm.TypeStringList, Value: "a,b"},
	})

	if got := a.ssmParamsV.table.GetRowCount(); got != 3 { // header + 2
		t.Fatalf("row count = %d, want 3", got)
	}
	if got := a.ssmParamsV.table.GetCell(1, 0).Text; got != "/app/one" {
		t.Errorf("row 1 name = %q, want %q", got, "/app/one")
	}
	if got := a.ssmParamsV.table.GetCell(1, 1).Text; got != string(awsssm.TypeString) {
		t.Errorf("row 1 type = %q, want %q", got, awsssm.TypeString)
	}
	if got := a.ssmParamsV.table.GetTitle(); got != " SSM Parameters (2) " {
		t.Errorf("title = %q, want %q", got, " SSM Parameters (2) ")
	}
}

func TestSSMParamsViewRepaintShowsDashForNoLastModified(t *testing.T) {
	a := New(config.Default())

	a.ssmParamsV.repaint([]awsssm.Parameter{{Name: "/x", Type: awsssm.TypeString}})

	if got := a.ssmParamsV.table.GetCell(1, 2).Text; got != "-" {
		t.Errorf("last-modified cell = %q, want %q", got, "-")
	}
}

func TestSSMParamsViewFilterNarrowsRowsByName(t *testing.T) {
	a := New(config.Default())
	a.ssmParamsV.repaint([]awsssm.Parameter{
		{Name: "/app/db-url", Type: awsssm.TypeString, Value: "x"},
		{Name: "/app/db-pass", Type: awsssm.TypeSecureString},
		{Name: "/app/other", Type: awsssm.TypeString, Value: "y"},
	})

	a.ssmParamsV.applyFilter("db")

	if got := a.ssmParamsV.table.GetRowCount(); got != 3 { // header + 2 matches
		t.Fatalf("row count after filter = %d, want 3", got)
	}
	if got := a.ssmParamsV.table.GetTitle(); got != " SSM Parameters (db) " {
		t.Errorf("title after filter = %q, want %q", got, " SSM Parameters (db) ")
	}
}

// TestSSMParamsViewFilteredTitleActuallyRenders is the render-based
// companion to the title-format fix: GetTitle() alone wouldn't have caught
// the bug (see queues_test.go's renderedScreenText doc comment).
func TestSSMParamsViewFilteredTitleActuallyRenders(t *testing.T) {
	a := New(config.Default())
	a.ssmParamsV.repaint([]awsssm.Parameter{
		{Name: "/app/db-url", Type: awsssm.TypeString, Value: "x"},
	})
	a.ssmParamsV.applyFilter("db")

	rendered := renderedScreenText(t, a.ssmParamsV.table, 60, 10)
	if !strings.Contains(rendered, "db") {
		t.Errorf("rendered screen = %q, want it to contain the filter text %q", rendered, "db")
	}
}

func TestSSMParamsViewFilterClearRestoresAll(t *testing.T) {
	a := New(config.Default())
	a.ssmParamsV.repaint([]awsssm.Parameter{{Name: "/a"}, {Name: "/b"}})
	a.ssmParamsV.applyFilter("a")

	a.ssmParamsV.applyFilter("")

	if got := a.ssmParamsV.table.GetRowCount(); got != 3 {
		t.Errorf("row count after clearing filter = %d, want 3", got)
	}
}

func TestSSMParamsViewSelectedFuncMapsThroughFilter(t *testing.T) {
	a := New(config.Default())
	a.ssmParamsV.repaint([]awsssm.Parameter{
		{Name: "/app/db-url", Type: awsssm.TypeString, Value: "x"},
		{Name: "/app/other", Type: awsssm.TypeString, Value: "y"},
	})
	a.ssmParamsV.applyFilter("other") // only "/app/other" remains, at row 1

	if len(a.ssmParamsV.filtered) != 1 || a.ssmParamsV.filtered[0].Name != "/app/other" {
		t.Fatalf("filtered = %+v, want exactly [/app/other]", a.ssmParamsV.filtered)
	}
}

func TestSSMParamsViewSecureStringValueNeverInTable(t *testing.T) {
	a := New(config.Default())
	a.ssmParamsV.repaint([]awsssm.Parameter{
		{Name: "/app/secret", Type: awsssm.TypeSecureString, Value: ""},
	})

	// The table only ever shows NAME/TYPE/LAST MODIFIED columns — this
	// locks in that a SecureString's value is structurally never rendered
	// in the list, only ever in the opt-in detail view after reveal.
	if got := a.ssmParamsV.table.GetColumnCount(); got != 3 {
		t.Errorf("column count = %d, want 3 (no value column)", got)
	}
}

func TestSSMParamsViewShowErrorRendersMessage(t *testing.T) {
	a := New(config.Default())

	a.ssmParamsV.showError(context.DeadlineExceeded)

	if got := a.ssmParamsV.table.GetCell(1, 0).Text; !strings.Contains(got, "deadline exceeded") {
		t.Errorf("error cell = %q, want it to contain the error", got)
	}
}

// TestSSMParamsViewShowStatusRendersMessage covers the in-progress
// status message load() shows while awsauth.WithReauth is running an SSO
// re-auth (spec/36-fe-aws-sso-reauth) — load() itself isn't tested here
// since its goroutine+QueueUpdateDraw path needs a running tview event
// loop to ever complete (see TestSSMParamsViewLoadErrorsWithoutActiveProfile's
// doc comment); the retry control flow is covered independently by
// internal/awsauth's own tests.
func TestSSMParamsViewShowStatusRendersMessage(t *testing.T) {
	a := New(config.Default())

	a.ssmParamsV.showStatus("AWS SSO session expired — opening browser to log in...")

	if got := a.ssmParamsV.table.GetCell(1, 0).Text; !strings.Contains(got, "opening browser") {
		t.Errorf("status cell = %q, want it to contain the status message", got)
	}
	fg, _, _ := a.ssmParamsV.table.GetCell(1, 0).Style.Decompose()
	if want := tcell.GetColor(a.cfg.Colors.Accent); fg != want {
		t.Errorf("status cell color = %v, want accent color %v", fg, want)
	}
}

// TestSSMParamsViewRepaintScrollsToTopWithManyRows guards against the same
// bug fixed for queuesView (spec/11-bugfix-queues-scroll-to-top):
// tview.Table's "track end" auto-scroll latches on during the table's
// first, still-empty draw and stays latched through repaint, scrolling a
// long list to the bottom instead of the top.
func TestSSMParamsViewRepaintScrollsToTopWithManyRows(t *testing.T) {
	a := New(config.Default())
	table := a.ssmParamsV.table
	table.SetRect(0, 0, 60, 15) // fewer visible rows than params below

	screen := tcell.NewSimulationScreen("")
	if err := screen.Init(); err != nil {
		t.Fatalf("screen.Init: %v", err)
	}
	screen.SetSize(60, 15)

	// First draw while the table is still empty (header only), mirroring
	// the real sequence: switchTo("ssm-parameters") draws before the async
	// load returns.
	table.Draw(screen)

	params := make([]awsssm.Parameter, 50)
	for i := range params {
		params[i] = awsssm.Parameter{Name: fmt.Sprintf("/app/param-%02d", i), Type: awsssm.TypeString, Value: "x"}
	}
	a.ssmParamsV.repaint(params)

	// The redraw that follows repaint via QueueUpdateDraw.
	table.Draw(screen)

	if row, _ := table.GetOffset(); row != 0 {
		t.Errorf("table scrolled away from top: rowOffset = %d, want 0", row)
	}
}
