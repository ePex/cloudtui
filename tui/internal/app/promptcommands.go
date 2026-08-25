package app

import (
	"strings"

	"github.com/ePex/cloudtui/tui/internal/config"
)

// promptCommand is one entry in the ':' prompt's special-command table: a
// set of equivalent typed forms (e.g. "q"/"quit") and the action to run
// when one of them is entered.
type promptCommand struct {
	names []string
	run   func(a *App)
}

// promptCommandTable is the single source of truth for the ':' prompt's
// special commands, shared by onPromptDone (execution) and
// promptSuggestions (autocomplete) so the two can't drift out of sync —
// see spec-wip/90-fe-command-autocomplete/plan.md for why that matters.
func promptCommandTable() []promptCommand {
	return []promptCommand{
		{names: []string{"q", "quit"}, run: func(a *App) { a.tv.Stop() }},
		{names: []string{"h", "home"}, run: func(a *App) { a.SwitchTo("home") }},
		{names: []string{"s", "settings"}, run: func(a *App) { a.SwitchTo("settings") }},
		{names: []string{"l", "log"}, run: func(a *App) { a.SwitchTo("log") }},
		{names: []string{"aq", "connections"}, run: func(a *App) { a.connManager.Show() }},
		{names: []string{"ap", "awsprofiles"}, run: func(a *App) { a.awsProfiles.Show() }},
	}
}

// globalHotkeyAliases are promptCommand names that duplicate a global
// single-key hotkey handled by onGlobalKey (app.go, the switch around
// lines 456-483). They stay valid to type-and-execute in the prompt (see
// promptCommandTable) but are excluded from promptSuggestions: the global
// hotkey already covers that need, and suggesting them just clutters the
// list under their own full name. Keep this in sync with onGlobalKey's
// switch if a hotkey is added, removed, or reassigned.
var globalHotkeyAliases = map[string]bool{"h": true, "s": true, "l": true, "q": true}

// promptSuggestions returns the ':' prompt's autocomplete entries matching
// currentText: theme names once the text starts with "theme ", otherwise
// every special command name (except globalHotkeyAliases), "theme "
// itself, and every registered view's Name() that starts with
// currentText.
func (a *App) promptSuggestions(currentText string) []string {
	if strings.HasPrefix(currentText, "theme ") {
		prefix := strings.TrimPrefix(currentText, "theme ")
		var matches []string
		for _, name := range config.AvailableThemes() {
			if strings.HasPrefix(name, prefix) {
				matches = append(matches, "theme "+name)
			}
		}
		return matches
	}

	var matches []string
	seen := make(map[string]bool)
	add := func(s string) {
		if !seen[s] {
			seen[s] = true
			matches = append(matches, s)
		}
	}

	for _, pc := range promptCommandTable() {
		for _, n := range pc.names {
			if globalHotkeyAliases[n] {
				continue
			}
			if strings.HasPrefix(n, currentText) {
				add(n)
			}
		}
	}
	if strings.HasPrefix("theme ", currentText) {
		add("theme ")
	}
	for _, v := range a.views {
		// A view's Name() can coincide with one of its own command
		// aliases (e.g. "home", "settings", "log" are both a
		// promptCommand name and a view's Name()) — add dedupes rather
		// than listing the same entry twice.
		if name := v.Name(); strings.HasPrefix(name, currentText) {
			add(name)
		}
	}
	return matches
}
