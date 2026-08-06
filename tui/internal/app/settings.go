package app

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// settingsView is the Settings screen: an editable tview.Form where each
// config field is a row. Lives in internal/app (not internal/ui/views) because
// it needs live config read/write and runtime overlay control.
type settingsView struct {
	form *tview.Form
}

func (s *settingsView) Name() string               { return "settings" }
func (s *settingsView) Title() string              { return "Settings" }
func (s *settingsView) Primitive() tview.Primitive { return s.form }

// styleDropDown applies palette colors to the dropdown's popup list so
// unselected items are readable against the theme background.
func styleDropDown(dd *tview.DropDown, p config.Palette) {
	dd.SetListStyles(
		tcell.StyleDefault.
			Foreground(tcell.GetColor(p.Text)).
			Background(tcell.GetColor(p.Background)),
		tcell.StyleDefault.
			Foreground(tcell.GetColor(p.SelectionText)).
			Background(tcell.GetColor(p.SelectionBg)),
	)
}

// newSettingsView builds the Settings view. The Theme dropdown immediately
// applies and persists the selected palette at runtime via a.switchTheme.
func newSettingsView(a *App) ui.View {
	form := tview.NewForm()
	form.SetBorder(true).SetTitle(" Settings ")

	themes := config.AvailableThemes()
	currentIdx := 0
	for i, t := range themes {
		if t == a.cfg.Theme {
			currentIdx = i
			break
		}
	}

	form.AddDropDown("Theme", themes, currentIdx, func(option string, _ int) {
		if a.switchingTheme {
			return
		}
		a.switchTheme(option)
	})

	// Style the popup list so unselected items are readable against the theme
	// background. Without this, tview uses terminal defaults which may be
	// invisible against the palette background color.
	if dd, ok := form.GetFormItem(0).(*tview.DropDown); ok {
		styleDropDown(dd, a.cfg.Colors)
	}

	a.settingsForm = form
	return &settingsView{form: form}
}
