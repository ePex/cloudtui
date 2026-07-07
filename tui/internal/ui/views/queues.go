package views

import "github.com/ePex/cloudtui/tui/internal/ui"

// NewQueues returns the placeholder view for Amazon MQ (ActiveMQ) queues:
// list queues, browse/send/purge/move messages via the QueueBackend interface.
func NewQueues() ui.View {
	return &placeholder{
		name:        "queues",
		title:       "Queues",
		description: "List queues; browse, send, purge, and move messages.",
	}
}
