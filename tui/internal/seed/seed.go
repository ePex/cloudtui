// Package seed generates and sends sample JSON messages to a queue, for
// populating a broker with test data during local development.
package seed

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"
)

// Sender is the subset of queue.Backend needed to seed a queue.
type Sender interface {
	SendMessage(ctx context.Context, queueName, body string) error
}

// Run sends count sample JSON messages to queueName via sender, in order,
// stopping at the first error. progress, if non-nil, is called after each
// successful send with the number sent so far and the total.
func Run(ctx context.Context, sender Sender, queueName string, count int, progress func(sent, total int)) error {
	for i := 1; i <= count; i++ {
		body, err := sampleMessage(i)
		if err != nil {
			return fmt.Errorf("encoding message %d: %w", i, err)
		}
		if err := sender.SendMessage(ctx, queueName, body); err != nil {
			return fmt.Errorf("sending message %d/%d: %w", i, count, err)
		}
		if progress != nil {
			progress(i, count)
		}
	}
	return nil
}

// sampleEvent is a small, generic order-event payload — realistic enough to
// exercise the TUI's JSON message browsing/detail views without implying any
// particular queue schema.
type sampleEvent struct {
	ID        int       `json:"id"`
	Event     string    `json:"event"`
	Timestamp time.Time `json:"timestamp"`
	Amount    float64   `json:"amount"`
	Customer  string    `json:"customer"`
}

var sampleEventNames = []string{"order.created", "order.updated", "order.shipped", "order.cancelled"}
var sampleCustomers = []string{"acme-corp", "globex", "initech", "umbrella", "soylent"}

func sampleMessage(id int) (string, error) {
	e := sampleEvent{
		ID:        id,
		Event:     sampleEventNames[rand.Intn(len(sampleEventNames))],
		Timestamp: time.Now(),
		Amount:    float64(rand.Intn(10000)) / 100,
		Customer:  sampleCustomers[rand.Intn(len(sampleCustomers))],
	}
	b, err := json.Marshal(e)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
