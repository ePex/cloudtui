package queue

import "context"

// Summary holds the key metrics for a single queue.
type Summary struct {
	Name          string
	PendingCount  int64
	ConsumerCount int64
}

// Backend is the interface all queue data sources must implement.
type Backend interface {
	List(ctx context.Context) ([]Summary, error)
}
