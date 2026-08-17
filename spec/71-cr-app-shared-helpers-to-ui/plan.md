# Plan — CR 71: promote 4 shared helpers to `internal/ui`

## Approach

Two new files in `internal/ui`, each holding a related pair, plus
their moved tests; then update every caller.

### `internal/ui/style.go` (new)

```go
package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
)

// StyleList applies p's selection colors to l. tview.List's own computed
// default selection style inverts body text (background/text swapped), which
// doesn't produce the palette's highlight look — so selection is wired
// explicitly here rather than riding on applyTheme.
//
// Note: tview.List exposes no getter for its selected-item style, so the
// result cannot be unit-tested directly. Verified manually instead.
func StyleList(l *tview.List, p config.Palette) *tview.List {
	return l.
		SetSelectedBackgroundColor(tcell.GetColor(p.SelectionBg)).
		SetSelectedTextColor(tcell.GetColor(p.SelectionText))
}

// StyleDropDown applies palette colors to the dropdown's popup list so
// unselected items are readable against the theme background.
func StyleDropDown(dd *tview.DropDown, p config.Palette) {
	dd.SetListStyles(
		tcell.StyleDefault.
			Foreground(tcell.GetColor(p.Text)).
			Background(tcell.GetColor(p.Background)),
		tcell.StyleDefault.
			Foreground(tcell.GetColor(p.SelectionText)).
			Background(tcell.GetColor(p.SelectionBg)),
	)
}
```

### `internal/ui/filter.go` (new)

```go
package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ePex/cloudtui/tui/internal/queue"
)

// filterDateLayout is the bare "YYYY-MM-DD" (date-only, no time-of-day)
// format ParseFilterDate accepts in addition to RFC3339 — kept as its own
// named constant (rather than folded into the local-time branch below)
// because it's interpreted as UTC midnight, not local time, matching the
// message filter's original, already-shipped convention (proxy backend's
// toFilterDTO normalizes filter dates to UTC the same way).
const filterDateLayout = "2006-01-02"

// filterDateTimeLayout is the bare "YYYY-MM-DD HH:MM" (minute precision)
// format ParseFilterDate accepts in addition to RFC3339 and
// filterDateLayout — interpreted in the *local* timezone, not UTC (see
// ParseFilterDate's doc comment for why). Added for the time range
// modal's Absolute tab (spec/53-fe-log-time-range-modal decision 4).
const filterDateTimeLayout = "2006-01-02 15:04"

// ParseFilterDate parses s as RFC3339 (explicit zone honored as written),
// filterDateTimeLayout (local timezone), or filterDateLayout (UTC
// midnight) — in that order. The local-vs-UTC split between the two bare
// layouts is deliberate, not an oversight: the results table already
// displays event timestamps via .Local() (see logsearch.go/datadoglogs.go's
// repaint), so parsing a typed time as UTC would silently filter a
// different window than what the table then displays, offset by the local
// UTC offset — reported live as "I filtered 15:00-15:30 and got a message
// timestamped 17:29". filterDateLayout (date-only, no time-of-day) is left
// UTC-midnight, unchanged from before this fix — it's the already-shipped
// message filter's convention, and changing it wasn't asked for.
//
// An empty (post-trim) s returns the zero time with no error — "unset".
// label is used only to name the field in the returned error. Shared by
// ParseMessageFilterForm and the time range modal's Absolute tab.
func ParseFilterDate(label, s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.ParseInLocation(filterDateTimeLayout, s, time.Local); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse(filterDateLayout, s); err == nil {
		return t.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("invalid %s %q: want RFC3339, YYYY-MM-DD HH:MM, or YYYY-MM-DD", label, s)
}

// ParseMessageFilterForm parses the message filter form's four field values
// into a queue.MessageFilter. from/to accept any format ParseFilterDate
// does. maxCount must be a non-negative integer; empty fields
// are left unset (zero value).
func ParseMessageFilterForm(jmsType, from, to, maxCount string) (queue.MessageFilter, error) {
	f := queue.MessageFilter{JMSType: strings.TrimSpace(jmsType)}

	var err error
	if f.FromDate, err = ParseFilterDate("from", from); err != nil {
		return queue.MessageFilter{}, err
	}
	if f.ToDate, err = ParseFilterDate("to", to); err != nil {
		return queue.MessageFilter{}, err
	}

	maxCount = strings.TrimSpace(maxCount)
	if maxCount != "" {
		n, err := strconv.Atoi(maxCount)
		if err != nil || n < 0 {
			return queue.MessageFilter{}, fmt.Errorf("invalid max count %q: want a non-negative integer", maxCount)
		}
		f.MaxCount = n
	}

	return f, nil
}
```

### Call-site updates (all mechanical: bare name → `ui.`-qualified)

| File | Change |
|---|---|
| `theme.go` | remove `styleList`; `reapplyTheme`'s `styleList(a.settingsList, p)` → `ui.StyleList(...)` |
| `settings.go` | remove `styleDropDown` |
| `messages.go` | remove `parseFilterDate`, `parseMessageFilterForm`, `filterDateLayout`, `filterDateTimeLayout` |
| `confirm.go`, `movepicker.go`, `sendmessage.go`, `timerangemodal.go`, `connections.go` | `styleList(...)` → `ui.StyleList(...)` (all already import `internal/ui` for `ui.Host`/`ui.Themeable`, so no new import) |
| `connections.go` | `styleDropDown(...)` → `ui.StyleDropDown(...)` (×3 call sites) |
| `datadoglogs.go` | `styleDropDown(...)` → `ui.StyleDropDown(...)` (×2 call sites — `serviceFilterDD`/`envFilterDD`) |
| `timerangemodal.go` | `parseFilterDate(...)` → `ui.ParseFilterDate(...)` (×2, in `applyAbsolute`) |
| `messagefilter.go` | `parseMessageFilterForm(...)` → `ui.ParseMessageFilterForm(...)` |

### Test moves

- `theme_test.go`'s `TestStyleListAppliesSelectionColors` → new
  `internal/ui/style_test.go`, `styleList(...)` call updated to
  `StyleList(...)` (same-package test, unqualified export name).
- `messages_test.go`'s `TestParseMessageFilterForm` → new
  `internal/ui/filter_test.go`, `parseMessageFilterForm(...)` →
  `ParseMessageFilterForm(...)`.

## Files touched

New: `internal/ui/style.go`, `internal/ui/style_test.go`,
`internal/ui/filter.go`, `internal/ui/filter_test.go`.

Modified: `theme.go`, `settings.go`, `messages.go`, `confirm.go`,
`movepicker.go`, `sendmessage.go`, `timerangemodal.go`,
`connections.go`, `datadoglogs.go`, `messagefilter.go`,
`theme_test.go`, `messages_test.go`.

## Key decisions

- **Two files, grouped by what they do** (widget styling vs.
  filter-string parsing), not one big `helpers.go` — mirrors how
  `internal/ui` already separates concerns (`topbar.go`,
  `statusbar.go`, `help.go`, `theme.go`, `host.go` are each one
  concept, not a grab-bag).
- **Doc comments updated for the new exported names** (`styleList` →
  `StyleList` etc. in the comment text itself, not just the code) —
  the comments are user-facing documentation now (`go doc` reads
  them), not just inline notes.
- **No new tests beyond the two moved ones** — pure relocation of
  already-covered logic.
- **No new dependencies** — `queue` and `time` are both already
  dependencies of `internal/app`, and neither imports `internal/ui`,
  so `internal/ui` importing `internal/queue` doesn't create a cycle.

## Definition of done

Unchanged from spec.md — `go build`/`go test` pass, all four helpers
(+ constants) live in `internal/ui` exported, none of
`theme.go`/`settings.go`/`messages.go` defines them anymore, moved
tests pass unchanged.
