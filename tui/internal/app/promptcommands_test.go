package app

import (
	"reflect"
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/ePex/cloudtui/tui/internal/config"
)

func TestPromptSuggestionsEmptyPrefixReturnsFullList(t *testing.T) {
	a := New(config.Default())

	got := a.promptSuggestions("")

	want := []string{
		"quit", "home", "settings", "log",
		"aq", "connections", "ap", "awsprofiles",
		"theme ",
		"queues", "ssm-parameters", "secrets-manager",
		"cloudwatch-logs", "datadog-logs", "codepipeline",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("promptSuggestions(\"\") =\n%v\nwant\n%v", got, want)
	}
}

func TestPromptSuggestionsFiltersByPrefix(t *testing.T) {
	a := New(config.Default())

	got := a.promptSuggestions("th")

	want := []string{"theme "}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("promptSuggestions(%q) = %v, want %v", "th", got, want)
	}
}

func TestPromptSuggestionsDedupesAliasAndViewName(t *testing.T) {
	a := New(config.Default())

	got := a.promptSuggestions("home")

	want := []string{"home"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("promptSuggestions(%q) = %v, want %v (deduped)", "home", got, want)
	}
}

func TestPromptSuggestionsThemePrefixListsAllThemeNames(t *testing.T) {
	a := New(config.Default())

	got := a.promptSuggestions("theme ")

	var want []string
	for _, name := range config.AvailableThemes() {
		want = append(want, "theme "+name)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("promptSuggestions(%q) = %v, want %v", "theme ", got, want)
	}
}

func TestPromptSuggestionsThemePrefixFiltersThemeNames(t *testing.T) {
	a := New(config.Default())

	got := a.promptSuggestions("theme d")

	want := []string{"theme dark"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("promptSuggestions(%q) = %v, want %v", "theme d", got, want)
	}
}

func TestPromptSuggestionsNoMatchReturnsEmpty(t *testing.T) {
	a := New(config.Default())

	got := a.promptSuggestions("zzz")

	if len(got) != 0 {
		t.Errorf("promptSuggestions(%q) = %v, want empty", "zzz", got)
	}
}

// TestPromptSuggestionsOmitsGlobalHotkeyAliases guards against re-adding
// clutter to the ':' prompt's suggestion list: q/h/s/l are already reachable
// as global hotkeys without opening the prompt at all (see onGlobalKey), so
// promptSuggestions should never offer the bare letter, even though it
// remains a valid, executable promptCommand name (see
// TestPromptShortAliasStillExecutes).
func TestPromptSuggestionsOmitsGlobalHotkeyAliases(t *testing.T) {
	a := New(config.Default())

	tests := []struct {
		text        string
		wantMissing string
		wantPresent string
	}{
		{text: "q", wantMissing: "q", wantPresent: "quit"},
		{text: "h", wantMissing: "h", wantPresent: "home"},
		{text: "s", wantMissing: "s", wantPresent: "settings"},
		{text: "l", wantMissing: "l", wantPresent: "log"},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			matches := a.promptSuggestions(tt.text)
			for _, m := range matches {
				if m == tt.wantMissing {
					t.Errorf("promptSuggestions(%q) = %v, want it to omit %q", tt.text, matches, tt.wantMissing)
				}
			}
			found := false
			for _, m := range matches {
				if m == tt.wantPresent {
					found = true
				}
			}
			if !found {
				t.Errorf("promptSuggestions(%q) = %v, want it to include %q", tt.text, matches, tt.wantPresent)
			}
		})
	}
}

// TestPromptSuggestionsKeepsPromptOnlyAliases confirms aq/ap still suggest:
// unlike q/h/s/l, they have no global-hotkey equivalent (a global hotkey is
// necessarily a single rune, and these are two characters), so the prompt is
// the only discoverable way to reach them short of typing the full name.
func TestPromptSuggestionsKeepsPromptOnlyAliases(t *testing.T) {
	a := New(config.Default())

	matches := a.promptSuggestions("a")
	for _, want := range []string{"aq", "ap"} {
		found := false
		for _, m := range matches {
			if m == want {
				found = true
			}
		}
		if !found {
			t.Errorf(`promptSuggestions("a") = %v, want it to include %q`, matches, want)
		}
	}
}

// TestPromptShortAliasStillExecutes confirms that filtering q/h/s/l out of
// the suggestion list (TestPromptSuggestionsOmitsGlobalHotkeyAliases) didn't
// also remove them from onPromptDone's execution path — typing ":s" and
// pressing Enter must still switch to Settings via the promptCommand,
// independent of the suggestion list.
func TestPromptShortAliasStillExecutes(t *testing.T) {
	a := New(config.Default())
	a.prompt.SetText("s")

	a.onPromptDone(tcell.KeyEnter)

	if name, _ := a.pages.GetFrontPage(); name != "settings" {
		t.Errorf("front page after ':s' = %q, want %q", name, "settings")
	}
}
