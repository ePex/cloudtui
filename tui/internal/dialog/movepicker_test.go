package dialog

import "testing"

func TestSortPickerQueues(t *testing.T) {
	cases := []struct {
		name        string
		sourceQueue string
		input       []string
		want        []string
	}{
		{
			name:        "DLQ source with matching non-DLQ queue pins it first",
			sourceQueue: "dlq.foo.bar",
			input:       []string{"alpha", "foo.bar", "zebra"},
			want:        []string{"foo.bar", "alpha", "zebra"},
		},
		{
			name:        "DLQ source case-insensitive prefix detection",
			sourceQueue: "DLQ.foo.bar",
			input:       []string{"foo.bar", "alpha"},
			want:        []string{"foo.bar", "alpha"},
		},
		{
			name:        "DLQ source candidate match is case-insensitive",
			sourceQueue: "dlq.Foo.Bar",
			input:       []string{"foo.bar", "alpha"},
			want:        []string{"foo.bar", "alpha"},
		},
		{
			name:        "DLQ source without matching candidate returns alphabetical",
			sourceQueue: "dlq.missing.queue",
			input:       []string{"zebra", "alpha", "mango"},
			want:        []string{"alpha", "mango", "zebra"},
		},
		{
			name:        "non-DLQ source returns alphabetical",
			sourceQueue: "foo.bar",
			input:       []string{"zebra", "alpha", "mango"},
			want:        []string{"alpha", "mango", "zebra"},
		},
		{
			name:        "empty list",
			sourceQueue: "dlq.foo",
			input:       []string{},
			want:        []string{},
		},
		{
			name:        "DLQ queues are de-prioritized after regular queues",
			sourceQueue: "foo.bar",
			input:       []string{"dlq.other", "regular", "another"},
			want:        []string{"another", "regular", "dlq.other"},
		},
		{
			name:        "system queues are de-prioritized last",
			sourceQueue: "foo.bar",
			input:       []string{"activemq.advisory", "regular", "statistics.foo"},
			want:        []string{"regular", "activemq.advisory", "statistics.foo"},
		},
		{
			name:        "full tier ordering: preferred, regular, DLQ, system",
			sourceQueue: "dlq.my.queue",
			input:       []string{"activemq.advisory", "dlq.other", "my.queue", "regular.a", "statistics.foo"},
			want:        []string{"my.queue", "regular.a", "dlq.other", "activemq.advisory", "statistics.foo"},
		},
		{
			name:        "IMQ source pins corresponding queue first",
			sourceQueue: "imq.foo.bar",
			input:       []string{"foo.bar", "alpha"},
			want:        []string{"foo.bar", "alpha"},
		},
		{
			name:        "IMQ queues are de-prioritized alongside DLQ queues",
			sourceQueue: "regular.queue",
			input:       []string{"imq.other", "regular.a", "dlq.other"},
			want:        []string{"regular.a", "dlq.other", "imq.other"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := sortPickerQueues(tc.sourceQueue, tc.input)
			if len(got) != len(tc.want) {
				t.Fatalf("sortPickerQueues() len = %d, want %d; got %v", len(got), len(tc.want), got)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("sortPickerQueues()[%d] = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}
