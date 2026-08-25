package view

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/ePex/cloudtui/tui/internal/awslogs"
	"github.com/ePex/cloudtui/tui/internal/config"
)

func newTestLogsView(t *testing.T) (*fakeViewHost, *LogsView) {
	t.Helper()
	host := newFakeViewHost()
	return host, NewLogsView(host, func(string) {})
}

func TestLogsViewNameAndTitle(t *testing.T) {
	_, lv := newTestLogsView(t)

	if got := lv.Name(); got != "cloudwatch-logs" {
		t.Errorf("Name() = %q, want %q", got, "cloudwatch-logs")
	}
	if got := lv.Title(); got != "CloudWatch Logs" {
		t.Errorf("Title() = %q, want %q", got, "CloudWatch Logs")
	}
}

func TestLogsViewHeaderLabels(t *testing.T) {
	_, lv := newTestLogsView(t)

	// Column 0 is the star column (blank header, checked separately).
	want := []string{"NAME", "RETENTION", "CREATED"}
	for i, label := range want {
		col := i + 1
		cell := lv.table.GetCell(0, col)
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
	host, lv := newTestLogsView(t)
	host.cfg.ActiveAWSProfile = ""
	calls := 0
	host.listLogGroupsFn = func(context.Context, string) ([]awslogs.LogGroup, error) {
		calls++
		return nil, nil
	}

	lv.load()

	if calls != 0 {
		t.Error("listLogGroups was called despite no active AWS profile")
	}
	if got := lv.table.GetCell(1, 1).Text; !strings.Contains(got, "no AWS profile selected") {
		t.Errorf("error cell = %q, want it to mention no profile selected", got)
	}
}

func TestLogsViewRepaintPopulatesRows(t *testing.T) {
	_, lv := newTestLogsView(t)

	lv.repaint([]awslogs.LogGroup{
		{Name: "/aws/lambda/one", RetentionDays: 14},
		{Name: "/aws/lambda/two", RetentionDays: 0},
	})

	if got := lv.table.GetRowCount(); got != 3 { // header + 2
		t.Fatalf("row count = %d, want 3", got)
	}
	if got := lv.table.GetCell(1, 1).Text; got != "/aws/lambda/one" {
		t.Errorf("row 1 name = %q, want %q", got, "/aws/lambda/one")
	}
	if got := lv.table.GetCell(1, 2).Text; got != "14 days" {
		t.Errorf("row 1 retention = %q, want %q", got, "14 days")
	}
	if got := lv.table.GetCell(2, 2).Text; got != "never" {
		t.Errorf("row 2 retention = %q, want %q", got, "never")
	}
	if got := lv.table.GetTitle(); got != " CloudWatch Logs (2) " {
		t.Errorf("title = %q, want %q", got, " CloudWatch Logs (2) ")
	}
}

func TestLogsViewRepaintShowsDashForNoCreatedAt(t *testing.T) {
	_, lv := newTestLogsView(t)

	lv.repaint([]awslogs.LogGroup{{Name: "/x"}})

	if got := lv.table.GetCell(1, 3).Text; got != "-" {
		t.Errorf("created cell = %q, want %q", got, "-")
	}
}

func TestLogsViewFilterNarrowsRowsByName(t *testing.T) {
	_, lv := newTestLogsView(t)

	lv.repaint([]awslogs.LogGroup{
		{Name: "/aws/lambda/db-writer"},
		{Name: "/aws/lambda/db-reader"},
		{Name: "/aws/ecs/other"},
	})

	lv.applyFilter("db")

	if got := lv.table.GetRowCount(); got != 3 { // header + 2 matches
		t.Fatalf("row count after filter = %d, want 3", got)
	}
	if got := lv.table.GetTitle(); got != " CloudWatch Logs (db) " {
		t.Errorf("title after filter = %q, want %q", got, " CloudWatch Logs (db) ")
	}
}

// TestLogsViewFilteredTitleActuallyRenders is the render-based companion
// to the title-format fix: GetTitle() alone wouldn't have caught the
// bracket-swallowing bug FE 32 found (see queues_test.go's
// renderedScreenText doc comment).
func TestLogsViewFilteredTitleActuallyRenders(t *testing.T) {
	_, lv := newTestLogsView(t)

	lv.repaint([]awslogs.LogGroup{
		{Name: "/aws/lambda/db-writer"},
	})
	lv.applyFilter("db")

	rendered := renderedScreenText(t, lv.table, 60, 10)
	if !strings.Contains(rendered, "db") {
		t.Errorf("rendered screen = %q, want it to contain the filter text %q", rendered, "db")
	}
}

func TestLogsViewFilterClearRestoresAll(t *testing.T) {
	_, lv := newTestLogsView(t)

	lv.repaint([]awslogs.LogGroup{{Name: "/a"}, {Name: "/b"}})
	lv.applyFilter("a")

	lv.applyFilter("")

	if got := lv.table.GetRowCount(); got != 3 {
		t.Errorf("row count after clearing filter = %d, want 3", got)
	}
}

func TestLogsViewSelectedFuncMapsThroughFilter(t *testing.T) {
	_, lv := newTestLogsView(t)

	lv.repaint([]awslogs.LogGroup{
		{Name: "/aws/lambda/db-writer"},
		{Name: "/aws/lambda/other"},
	})
	lv.applyFilter("other") // only "/aws/lambda/other" remains, at row 1

	if len(lv.filtered) != 1 || lv.filtered[0].Name != "/aws/lambda/other" {
		t.Fatalf("filtered = %+v, want exactly [/aws/lambda/other]", lv.filtered)
	}
}

func TestLogsViewShowErrorRendersMessage(t *testing.T) {
	_, lv := newTestLogsView(t)

	lv.showError(context.DeadlineExceeded)

	if got := lv.table.GetCell(1, 1).Text; !strings.Contains(got, "deadline exceeded") {
		t.Errorf("error cell = %q, want it to contain the error", got)
	}
}

// TestLogsViewShowStatusRendersMessage covers the in-progress status
// message load() shows while awsauth.WithReauth is running an SSO
// re-auth (spec/36-fe-aws-sso-reauth) — see ssmparams_test.go's
// equivalent test for why load() itself isn't exercised here.
func TestLogsViewShowStatusRendersMessage(t *testing.T) {
	host, lv := newTestLogsView(t)

	lv.showStatus("AWS SSO session expired — opening browser to log in...")

	if got := lv.table.GetCell(1, 1).Text; !strings.Contains(got, "opening browser") {
		t.Errorf("status cell = %q, want it to contain the status message", got)
	}
	fg, _, _ := lv.table.GetCell(1, 1).Style.Decompose()
	if want := tcell.GetColor(host.cfg.Colors.Accent); fg != want {
		t.Errorf("status cell color = %v, want accent color %v", fg, want)
	}
}

func TestLogsViewFavoriteTogglePersistsAndShowsStar(t *testing.T) {
	host, lv := newTestLogsView(t)
	host.cfg.ActiveAWSProfile = "work"
	lv.repaint([]awslogs.LogGroup{{Name: "/aws/lambda/one"}, {Name: "/aws/lambda/two"}})
	lv.table.Select(1, 0) // "/aws/lambda/one"

	lv.table.GetInputCapture()(tcell.NewEventKey(tcell.KeyRune, 'f', tcell.ModNone))

	if !host.cfg.AWSFavorites.IsFavorite(config.FavoriteLogGroup, "work", "/aws/lambda/one") {
		t.Error("host.ToggleFavorite was not called for the selected row")
	}
	if got := lv.table.GetCell(1, 0).Text; got != favoriteStar {
		t.Errorf("star cell = %q, want %q", got, favoriteStar)
	}
}

func TestLogsViewFavoriteSortsToTop(t *testing.T) {
	host, lv := newTestLogsView(t)
	host.cfg.ActiveAWSProfile = "work"
	host.cfg.AWSFavorites = host.cfg.AWSFavorites.Toggle(config.FavoriteLogGroup, "work", "/aws/lambda/two")

	lv.repaint([]awslogs.LogGroup{{Name: "/aws/lambda/one"}, {Name: "/aws/lambda/two"}})

	if got := lv.table.GetCell(1, 1).Text; got != "/aws/lambda/two" {
		t.Errorf("row 1 (favorited) = %q, want %q", got, "/aws/lambda/two")
	}
	if got := lv.table.GetCell(2, 1).Text; got != "/aws/lambda/one" {
		t.Errorf("row 2 = %q, want %q", got, "/aws/lambda/one")
	}
}

func TestLogsViewFavoriteToggleTwiceRemovesStar(t *testing.T) {
	host, lv := newTestLogsView(t)
	host.cfg.ActiveAWSProfile = "work"
	lv.repaint([]awslogs.LogGroup{{Name: "/aws/lambda/one"}})
	lv.table.Select(1, 0)

	capture := lv.table.GetInputCapture()
	capture(tcell.NewEventKey(tcell.KeyRune, 'f', tcell.ModNone))
	capture(tcell.NewEventKey(tcell.KeyRune, 'f', tcell.ModNone))

	if host.cfg.AWSFavorites.IsFavorite(config.FavoriteLogGroup, "work", "/aws/lambda/one") {
		t.Error("favorite still set after toggling twice")
	}
	if got := lv.table.GetCell(1, 0).Text; got != "" {
		t.Errorf("star cell = %q, want empty", got)
	}
}

func TestLogsViewFavoritesDoNotLeakAcrossProfiles(t *testing.T) {
	host, lv := newTestLogsView(t)
	host.cfg.ActiveAWSProfile = "work"
	host.cfg.AWSFavorites = host.cfg.AWSFavorites.Toggle(config.FavoriteLogGroup, "work", "/aws/lambda/one")

	host.cfg.ActiveAWSProfile = "personal"
	lv.repaint([]awslogs.LogGroup{{Name: "/aws/lambda/one"}})

	if got := lv.table.GetCell(1, 0).Text; got != "" {
		t.Errorf("star cell under a different profile = %q, want empty (favorite shouldn't leak)", got)
	}
}

// TestLogsViewRepaintScrollsToTopWithManyRows guards against the bug
// fixed for queuesView (spec/11-bugfix-queues-scroll-to-top):
// tview.Table's "track end" auto-scroll latches on during the table's
// first, still-empty draw and stays latched through repaint, scrolling a
// long list to the bottom instead of the top.
func TestLogsViewRepaintScrollsToTopWithManyRows(t *testing.T) {
	_, lv := newTestLogsView(t)

	table := lv.table
	table.SetRect(0, 0, 60, 15) // fewer visible rows than groups below

	screen := tcell.NewSimulationScreen("")
	if err := screen.Init(); err != nil {
		t.Fatalf("screen.Init: %v", err)
	}
	screen.SetSize(60, 15)

	// First draw while the table is still empty (header only), mirroring
	// the real sequence: SwitchTo("cloudwatch-logs") draws before the
	// async load returns.
	table.Draw(screen)

	groups := make([]awslogs.LogGroup, 50)
	for i := range groups {
		groups[i] = awslogs.LogGroup{Name: fmt.Sprintf("/aws/lambda/group-%02d", i)}
	}
	lv.repaint(groups)

	// The redraw that follows repaint via QueueUpdateDraw.
	table.Draw(screen)

	if row, _ := table.GetOffset(); row != 0 {
		t.Errorf("table scrolled away from top: rowOffset = %d, want 0", row)
	}
}
