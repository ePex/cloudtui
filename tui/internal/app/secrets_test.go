package app

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/ePex/cloudtui/tui/internal/awssecrets"
	"github.com/ePex/cloudtui/tui/internal/config"
)

func TestSecretsViewNameAndTitle(t *testing.T) {
	a := New(config.Default())
	if got := a.secretsV.Name(); got != "secrets-manager" {
		t.Errorf("Name() = %q, want %q", got, "secrets-manager")
	}
	if got := a.secretsV.Title(); got != "Secrets Manager" {
		t.Errorf("Title() = %q, want %q", got, "Secrets Manager")
	}
}

func TestSecretsViewHeaderLabels(t *testing.T) {
	a := New(config.Default())
	want := []string{"NAME", "ROTATION", "LAST CHANGED"}
	for col, label := range want {
		cell := a.secretsV.table.GetCell(0, col)
		if cell == nil {
			t.Fatalf("header cell at column %d is nil", col)
		}
		if got := cell.Text; got != label {
			t.Errorf("header col %d = %q, want %q", col, got, label)
		}
	}
}

// TestSecretsViewLoadErrorsWithoutActiveProfile exercises load()'s
// synchronous guard, which returns before spawning the fetch goroutine —
// safe to call directly in a test, unlike the goroutine+QueueUpdateDraw
// path itself (which needs a running tview event loop to ever complete;
// see ssmParamsView's tests for the same reasoning).
func TestSecretsViewLoadErrorsWithoutActiveProfile(t *testing.T) {
	a := New(config.Default())
	a.cfg.ActiveAWSProfile = ""
	calls := 0
	a.listSecrets = func(context.Context, string) ([]awssecrets.Secret, error) {
		calls++
		return nil, nil
	}

	a.secretsV.load()

	if calls != 0 {
		t.Error("listSecrets was called despite no active AWS profile")
	}
	if got := a.secretsV.table.GetCell(1, 0).Text; !strings.Contains(got, "no AWS profile selected") {
		t.Errorf("error cell = %q, want it to mention no profile selected", got)
	}
}

func TestSecretsViewRepaintPopulatesRows(t *testing.T) {
	a := New(config.Default())

	a.secretsV.repaint([]awssecrets.Secret{
		{Name: "/app/one", RotationEnabled: true},
		{Name: "/app/two", RotationEnabled: false},
	})

	if got := a.secretsV.table.GetRowCount(); got != 3 { // header + 2
		t.Fatalf("row count = %d, want 3", got)
	}
	if got := a.secretsV.table.GetCell(1, 0).Text; got != "/app/one" {
		t.Errorf("row 1 name = %q, want %q", got, "/app/one")
	}
	if got := a.secretsV.table.GetCell(1, 1).Text; got != "yes" {
		t.Errorf("row 1 rotation = %q, want %q", got, "yes")
	}
	if got := a.secretsV.table.GetCell(2, 1).Text; got != "no" {
		t.Errorf("row 2 rotation = %q, want %q", got, "no")
	}
	if got := a.secretsV.table.GetTitle(); got != " Secrets Manager (2) " {
		t.Errorf("title = %q, want %q", got, " Secrets Manager (2) ")
	}
}

func TestSecretsViewRepaintShowsDashForNoLastChanged(t *testing.T) {
	a := New(config.Default())

	a.secretsV.repaint([]awssecrets.Secret{{Name: "/x"}})

	if got := a.secretsV.table.GetCell(1, 2).Text; got != "-" {
		t.Errorf("last-changed cell = %q, want %q", got, "-")
	}
}

func TestSecretsViewFilterNarrowsRowsByName(t *testing.T) {
	a := New(config.Default())
	a.secretsV.repaint([]awssecrets.Secret{
		{Name: "/app/db-url"},
		{Name: "/app/db-pass"},
		{Name: "/app/other"},
	})

	a.secretsV.applyFilter("db")

	if got := a.secretsV.table.GetRowCount(); got != 3 { // header + 2 matches
		t.Fatalf("row count after filter = %d, want 3", got)
	}
	if got := a.secretsV.table.GetTitle(); got != " Secrets Manager (db) " {
		t.Errorf("title after filter = %q, want %q", got, " Secrets Manager (db) ")
	}
}

// TestSecretsViewFilteredTitleActuallyRenders is the render-based
// companion to the title-format fix: GetTitle() alone wouldn't have
// caught the bracket-swallowing bug FE 32 found (see queues_test.go's
// renderedScreenText doc comment).
func TestSecretsViewFilteredTitleActuallyRenders(t *testing.T) {
	a := New(config.Default())
	a.secretsV.repaint([]awssecrets.Secret{
		{Name: "/app/db-url"},
	})
	a.secretsV.applyFilter("db")

	rendered := renderedScreenText(t, a.secretsV.table, 60, 10)
	if !strings.Contains(rendered, "db") {
		t.Errorf("rendered screen = %q, want it to contain the filter text %q", rendered, "db")
	}
}

func TestSecretsViewFilterClearRestoresAll(t *testing.T) {
	a := New(config.Default())
	a.secretsV.repaint([]awssecrets.Secret{{Name: "/a"}, {Name: "/b"}})
	a.secretsV.applyFilter("a")

	a.secretsV.applyFilter("")

	if got := a.secretsV.table.GetRowCount(); got != 3 {
		t.Errorf("row count after clearing filter = %d, want 3", got)
	}
}

func TestSecretsViewSelectedFuncMapsThroughFilter(t *testing.T) {
	a := New(config.Default())
	a.secretsV.repaint([]awssecrets.Secret{
		{Name: "/app/db-url"},
		{Name: "/app/other"},
	})
	a.secretsV.applyFilter("other") // only "/app/other" remains, at row 1

	if len(a.secretsV.filtered) != 1 || a.secretsV.filtered[0].Name != "/app/other" {
		t.Fatalf("filtered = %+v, want exactly [/app/other]", a.secretsV.filtered)
	}
}

func TestSecretsViewShowErrorRendersMessage(t *testing.T) {
	a := New(config.Default())

	a.secretsV.showError(context.DeadlineExceeded)

	if got := a.secretsV.table.GetCell(1, 0).Text; !strings.Contains(got, "deadline exceeded") {
		t.Errorf("error cell = %q, want it to contain the error", got)
	}
}

// TestSecretsViewShowStatusRendersMessage covers the in-progress status
// message load() shows while awsauth.WithReauth is running an SSO
// re-auth (spec/36-fe-aws-sso-reauth) — see ssmparams_test.go's
// equivalent test for why load() itself isn't exercised here.
func TestSecretsViewShowStatusRendersMessage(t *testing.T) {
	a := New(config.Default())

	a.secretsV.showStatus("AWS SSO session expired — opening browser to log in...")

	if got := a.secretsV.table.GetCell(1, 0).Text; !strings.Contains(got, "opening browser") {
		t.Errorf("status cell = %q, want it to contain the status message", got)
	}
	fg, _, _ := a.secretsV.table.GetCell(1, 0).Style.Decompose()
	if want := tcell.GetColor(a.cfg.Colors.Accent); fg != want {
		t.Errorf("status cell color = %v, want accent color %v", fg, want)
	}
}

// TestSecretsViewRepaintScrollsToTopWithManyRows guards against the same
// bug fixed for queuesView (spec/11-bugfix-queues-scroll-to-top):
// tview.Table's "track end" auto-scroll latches on during the table's
// first, still-empty draw and stays latched through repaint, scrolling a
// long list to the bottom instead of the top.
func TestSecretsViewRepaintScrollsToTopWithManyRows(t *testing.T) {
	a := New(config.Default())
	table := a.secretsV.table
	table.SetRect(0, 0, 60, 15) // fewer visible rows than secrets below

	screen := tcell.NewSimulationScreen("")
	if err := screen.Init(); err != nil {
		t.Fatalf("screen.Init: %v", err)
	}
	screen.SetSize(60, 15)

	// First draw while the table is still empty (header only), mirroring
	// the real sequence: switchTo("secrets-manager") draws before the
	// async load returns.
	table.Draw(screen)

	secrets := make([]awssecrets.Secret, 50)
	for i := range secrets {
		secrets[i] = awssecrets.Secret{Name: fmt.Sprintf("secret-%02d", i)}
	}
	a.secretsV.repaint(secrets)

	// The redraw that follows repaint via QueueUpdateDraw.
	table.Draw(screen)

	if row, _ := table.GetOffset(); row != 0 {
		t.Errorf("table scrolled away from top: rowOffset = %d, want 0", row)
	}
}
