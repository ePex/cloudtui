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
