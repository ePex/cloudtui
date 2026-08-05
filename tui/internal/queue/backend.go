package queue

import (
	"context"
	"time"
)

// Summary holds the key metrics for a single queue.
type Summary struct {
	Name          string
	PendingCount  int64
	ConsumerCount int64
	EnqueueCount  int64
	DequeueCount  int64
}

// Message represents a single message browsed from a queue.
type Message struct {
	ID            string
	JMSType       string // jMSType header, or inferred type ("text"/"bytes"/"other")
	CorrelationID string // jMSCorrelationID header
	Timestamp     time.Time
	Preview       string // first 80 chars of body text; "(binary)" for non-text messages
}

// Backend is the interface all queue data sources must implement.
type Backend interface {
	List(ctx context.Context) ([]Summary, error)
	BrowseMessages(ctx context.Context, queueName string) ([]Message, error)
}
