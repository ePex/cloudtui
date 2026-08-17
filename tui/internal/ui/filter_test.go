package ui

import (
	"testing"
	"time"

	"github.com/ePex/cloudtui/tui/internal/queue"
)

func TestParseMessageFilterForm(t *testing.T) {
	tests := []struct {
		name                   string
		jmsType, from, to, max string
		want                   queue.MessageFilter
		wantErr                bool
	}{
		{
			name: "all empty",
			want: queue.MessageFilter{},
		},
		{
			name:    "jms type only",
			jmsType: "order-created",
			want:    queue.MessageFilter{JMSType: "order-created"},
		},
		{
			name: "RFC3339 dates",
			from: "2025-01-31T08:30:00Z", to: "2025-02-01T17:00:00Z",
			want: queue.MessageFilter{
				FromDate: time.Date(2025, 1, 31, 8, 30, 0, 0, time.UTC),
				ToDate:   time.Date(2025, 2, 1, 17, 0, 0, 0, time.UTC),
			},
		},
		{
			name: "date-only dates taken as UTC midnight",
			from: "2025-01-31", to: "2025-02-01",
			want: queue.MessageFilter{
				FromDate: time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC),
				ToDate:   time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC),
			},
		},
		{
			// Local, not UTC (unlike the date-only case above) — see
			// ParseFilterDate's doc comment. Expectations built via
			// time.Local, not a hardcoded UTC offset, so this passes
			// regardless of the machine's timezone.
			name: "date+time dates (minute precision, no timezone) taken as local time",
			from: "2025-01-31 08:30", to: "2025-02-01 17:00",
			want: queue.MessageFilter{
				FromDate: time.Date(2025, 1, 31, 8, 30, 0, 0, time.Local),
				ToDate:   time.Date(2025, 2, 1, 17, 0, 0, 0, time.Local),
			},
		},
		{
			name: "max count",
			max:  "100",
			want: queue.MessageFilter{MaxCount: 100},
		},
		{
			name: "max count zero",
			max:  "0",
			want: queue.MessageFilter{MaxCount: 0},
		},
		{
			name:    "combined",
			jmsType: "order-created", from: "2025-01-31", max: "10",
			want: queue.MessageFilter{
				JMSType:  "order-created",
				FromDate: time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC),
				MaxCount: 10,
			},
		},
		{
			name:    "invalid from date",
			from:    "not-a-date",
			wantErr: true,
		},
		{
			name:    "invalid to date",
			to:      "not-a-date",
			wantErr: true,
		},
		{
			name:    "invalid max count",
			max:     "abc",
			wantErr: true,
		},
		{
			name:    "negative max count",
			max:     "-1",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseMessageFilterForm(tt.jmsType, tt.from, tt.to, tt.max)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseMessageFilterForm() error = nil, want an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseMessageFilterForm() error = %v", err)
			}
			if !got.FromDate.Equal(tt.want.FromDate) || !got.ToDate.Equal(tt.want.ToDate) ||
				got.JMSType != tt.want.JMSType || got.MaxCount != tt.want.MaxCount {
				t.Errorf("ParseMessageFilterForm() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
