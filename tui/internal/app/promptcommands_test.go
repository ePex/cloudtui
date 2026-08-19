package app

import (
	"reflect"
	"testing"

	"github.com/ePex/cloudtui/tui/internal/config"
)

func TestPromptSuggestionsEmptyPrefixReturnsFullList(t *testing.T) {
	a := New(config.Default())

	got := a.promptSuggestions("")

	want := []string{
		"q", "quit", "h", "home", "s", "settings", "l", "log",
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
