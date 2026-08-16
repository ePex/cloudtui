package jolokia

import (
	"context"
	"fmt"

	"github.com/ePex/cloudtui/tui/internal/queue"
)

// filterMessages returns the subset of msgs matching filter, truncated to
// filter.MaxCount (0 = unlimited). Jolokia/JMX has no selector-based browse,
// so — unlike mq-proxy, which pushes the filter down to a JMS selector —
// this browses everything and filters client-side.
func filterMessages(msgs []queue.Message, filter queue.MessageFilter) []queue.Message {
	matches := make([]queue.Message, 0, len(msgs))
	for _, m := range msgs {
		if filter.JMSType != "" && m.JMSType != filter.JMSType {
			continue
		}
		if filter.MessageID != "" && m.ID != filter.MessageID {
			continue
		}
		if !filter.FromDate.IsZero() && m.Timestamp.Before(filter.FromDate) {
			continue
		}
		if !filter.ToDate.IsZero() && m.Timestamp.After(filter.ToDate) {
			continue
		}
		matches = append(matches, m)
		if filter.MaxCount > 0 && len(matches) >= filter.MaxCount {
			break
		}
	}
	return matches
}

// DeleteMessages implements queue.Backend: browses queueName filtered by
// filter, and removes each match via RemoveMessage.
func (c *Client) DeleteMessages(ctx context.Context, queueName string, filter queue.MessageFilter) (int, error) {
	matches, err := c.BrowseMessages(ctx, queueName, filter)
	if err != nil {
		return 0, fmt.Errorf("delete messages from %q: %w", queueName, err)
	}
	for _, m := range matches {
		if err := c.RemoveMessage(ctx, queueName, m.ID); err != nil {
			return 0, fmt.Errorf("delete messages from %q: %w", queueName, err)
		}
	}
	return len(matches), nil
}

// MoveMessages implements queue.Backend: browses sourceQueue filtered by
// filter, and moves each match via MoveMessage.
func (c *Client) MoveMessages(ctx context.Context, sourceQueue, targetQueue string, filter queue.MessageFilter) (int, error) {
	matches, err := c.BrowseMessages(ctx, sourceQueue, filter)
	if err != nil {
		return 0, fmt.Errorf("move messages from %q to %q: %w", sourceQueue, targetQueue, err)
	}
	for _, m := range matches {
		if err := c.MoveMessage(ctx, sourceQueue, m.ID, targetQueue); err != nil {
			return 0, fmt.Errorf("move messages from %q to %q: %w", sourceQueue, targetQueue, err)
		}
	}
	return len(matches), nil
}
