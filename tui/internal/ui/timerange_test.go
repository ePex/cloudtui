package ui

import (
	"testing"
	"time"
)

func TestTimeRangeBounds(t *testing.T) {
	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)

	t.Run("relative resolves against now", func(t *testing.T) {
		tr := TimeRange{Mode: TimeRangeRelative, PresetIdx: 4} // "2d"
		start, end := tr.Bounds(now)
		if !end.Equal(now) {
			t.Errorf("end = %v, want %v", end, now)
		}
		if want := now.Add(-48 * time.Hour); !start.Equal(want) {
			t.Errorf("start = %v, want %v", start, want)
		}
	})

	t.Run("absolute ignores now", func(t *testing.T) {
		from := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
		to := time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)
		tr := TimeRange{Mode: TimeRangeAbsolute, From: from, To: to}
		start, end := tr.Bounds(now)
		if !start.Equal(from) || !end.Equal(to) {
			t.Errorf("bounds = %v, %v, want %v, %v", start, end, from, to)
		}
	})
}

func TestTimeRangeLabel(t *testing.T) {
	t.Run("relative uses preset label", func(t *testing.T) {
		tr := TimeRange{Mode: TimeRangeRelative, PresetIdx: 4}
		if got, want := tr.Label(), "2d"; got != want {
			t.Errorf("label = %q, want %q", got, want)
		}
	})

	t.Run("absolute renders both timestamps in local time", func(t *testing.T) {
		// Label renders via .Local() (matches the results table's own
		// timestamp display) — built via time.Local, not a hardcoded UTC
		// offset, so this passes regardless of the machine's timezone.
		from := time.Date(2026, 8, 1, 9, 30, 0, 0, time.Local)
		to := time.Date(2026, 8, 2, 17, 45, 0, 0, time.Local)
		tr := TimeRange{Mode: TimeRangeAbsolute, From: from, To: to}
		want := "2026-08-01 09:30 → 2026-08-02 17:45"
		if got := tr.Label(); got != want {
			t.Errorf("label = %q, want %q", got, want)
		}
	})
}
