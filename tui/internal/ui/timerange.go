package ui

import "time"

// TimeRangePreset is one relative time window offered by the time range
// modal's Relative tab (spec/34-fe-cloudwatch-logs decision 4 — relative
// presets, not free-form timestamps).
type TimeRangePreset struct {
	Label    string
	Duration time.Duration
}

var TimeRangePresets = []TimeRangePreset{
	{"15m", 15 * time.Minute},
	{"1h", time.Hour},
	{"4h", 4 * time.Hour},
	{"1d", 24 * time.Hour},
	{"2d", 48 * time.Hour},
	{"3d", 72 * time.Hour},
	{"7d", 7 * 24 * time.Hour},
	{"15d", 15 * 24 * time.Hour},
	{"1mo", 30 * 24 * time.Hour}, // approximate — time.Duration has no calendar-aware month
}

// DefaultPresetIdx is "1h" — a reasonable default investigation window.
const DefaultPresetIdx = 1

// TimeRangeMode selects which of TimeRange's two representations is
// active — a relative preset (index into TimeRangePresets) or an
// absolute [from, to) window (spec/53-fe-log-time-range-modal).
type TimeRangeMode int

const (
	TimeRangeRelative TimeRangeMode = iota
	TimeRangeAbsolute
)

// TimeRange is the shared time-window selection for logSearchView and
// datadogLogsView, driven by the shared Relative/Absolute modal
// (spec/53-fe-log-time-range-modal). Only one of PresetIdx or From/To is
// meaningful, selected by Mode.
type TimeRange struct {
	Mode      TimeRangeMode
	PresetIdx int
	From, To  time.Time
}

// Bounds returns the [start, end) window for tr, resolving a relative
// preset against now.
func (tr TimeRange) Bounds(now time.Time) (start, end time.Time) {
	if tr.Mode == TimeRangeAbsolute {
		return tr.From, tr.To
	}
	return now.Add(-TimeRangePresets[tr.PresetIdx].Duration), now
}

// Label renders tr for display in a view's title. Absolute From/To are
// stored as UTC (see ParseFilterDate); .Local() here matches the results
// table's own timestamp display (repaint's e.Timestamp.Local()) — showing
// the UTC instant directly would silently disagree with what the table
// rows and the Absolute tab's fields themselves show for the same range.
func (tr TimeRange) Label() string {
	if tr.Mode == TimeRangeAbsolute {
		return tr.From.Local().Format("2006-01-02 15:04") + " → " + tr.To.Local().Format("2006-01-02 15:04")
	}
	return TimeRangePresets[tr.PresetIdx].Label
}
