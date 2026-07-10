// Package queue defines the contract the tui uses for queue operations,
// independent of how they're actually carried out — AWS calls (here, via
// the mq-proxy REST API) live behind this interface, never in UI code.
package queue

import "context"

// Summary is a queue's identity plus its current statistics.
type Summary struct {
	Name          string
	PendingCount  int64
	ConsumerCount int64
}

// Message is a single queue message.
type Message struct {
	ID         string
	Body       string
	Properties map[string]string
}

// Backend is the set of queue operations the tui's queues view needs.
// The primary implementation talks to mq-proxy (see the proxy
// subpackage); it's used both locally and in AWS so there's one code
// path everywhere, per the project's architecture.
type Backend interface {
	List(ctx context.Context) ([]Summary, error)
	Browse(ctx context.Context, queueName string, limit int) ([]Message, error)
	Send(ctx context.Context, queueName, body string, properties map[string]string) error
	Purge(ctx context.Context, queueName string) error
	Move(ctx context.Context, sourceQueueName, targetQueueName string, maxMessages *int) error
}
