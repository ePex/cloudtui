package jolokia

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/ePex/cloudtui/tui/internal/queue"
)

// PurgeQueue removes all messages from queueName using a three-tier strategy:
//  1. purgeQueue() — removes directly from the store, no message iteration.
//  2. removeMatchingMessages("TRUE") — store-removal path via JMS selector;
//     avoids the getClientID() call that fails for JMX-originated messages.
//  3. browse-and-remove — falls back to iterating messages individually.
func (c *Client) PurgeQueue(ctx context.Context, queueName string) error {
	mbean := fmt.Sprintf(
		"org.apache.activemq:type=Broker,brokerName=%s,destinationType=Queue,destinationName=%s",
		c.cfg.BrokerName, queueName,
	)

	// Tier 1: purgeQueue() — simplest, not available on all deployments.
	if err := c.execSimple(ctx, mbean, "purgeQueue()"); err == nil {
		return nil
	}

	// Tier 2: removeMatchingMessages("TRUE") — store path, avoids browse failure.
	if err := c.removeMatchingMessages(ctx, mbean, "TRUE"); err == nil {
		return nil
	}

	// Tier 3: browse-and-remove.
	msgs, err := c.BrowseMessages(ctx, queueName, queue.MessageFilter{})
	if err != nil {
		return fmt.Errorf("purge queue %s: browse: %w", queueName, err)
	}

	for _, msg := range msgs {
		reqBody := map[string]any{
			"type":      "exec",
			"mbean":     mbean,
			"operation": "removeMessage(java.lang.String)",
			"arguments": []string{msg.ID},
		}
		body, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("purge queue %s: marshal removeMessage: %w", queueName, err)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.URL, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("purge queue %s: removeMessage request: %w", queueName, err)
		}
		req.Header.Set("Content-Type", "application/json")
		c.setHeaders(req)

		resp, err := c.http.Do(req)
		if err != nil {
			return fmt.Errorf("purge queue %s: removeMessage: %w", queueName, err)
		}
		var result struct {
			Status int    `json:"status"`
			Error  string `json:"error"`
		}
		err = json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()
		if err != nil {
			return fmt.Errorf("purge queue %s: removeMessage decode: %w", queueName, err)
		}
		if result.Status != 200 {
			return fmt.Errorf("purge queue %s: removeMessage error (status %d): %s", queueName, result.Status, result.Error)
		}
	}
	return nil
}

// execSimple calls a no-argument Jolokia exec operation and returns nil on
// Jolokia status 200. Used for purgeQueue().
func (c *Client) execSimple(ctx context.Context, mbean, operation string) error {
	reqBody := map[string]any{
		"type":      "exec",
		"mbean":     mbean,
		"operation": operation,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("%s marshal: %w", operation, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("%s request: %w", operation, err)
	}
	req.Header.Set("Content-Type", "application/json")
	c.setHeaders(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%s: %w", operation, err)
	}
	var result struct {
		Status int    `json:"status"`
		Error  string `json:"error"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	resp.Body.Close()
	if err != nil {
		return fmt.Errorf("%s decode: %w", operation, err)
	}
	if result.Status != 200 {
		return fmt.Errorf("%s error (status %d): %s", operation, result.Status, result.Error)
	}
	return nil
}

// removeMatchingMessages calls removeMatchingMessages(java.lang.String) with
// the given JMS selector. Returns nil on Jolokia status 200.
func (c *Client) removeMatchingMessages(ctx context.Context, mbean, selector string) error {
	reqBody := map[string]any{
		"type":      "exec",
		"mbean":     mbean,
		"operation": "removeMatchingMessages(java.lang.String)",
		"arguments": []string{selector},
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("removeMatchingMessages marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("removeMatchingMessages request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	c.setHeaders(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("removeMatchingMessages: %w", err)
	}
	var result struct {
		Status int    `json:"status"`
		Error  string `json:"error"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	resp.Body.Close()
	if err != nil {
		return fmt.Errorf("removeMatchingMessages decode: %w", err)
	}
	if result.Status != 200 {
		return fmt.Errorf("removeMatchingMessages error (status %d): %s", result.Status, result.Error)
	}
	return nil
}

// RemoveMessage removes a single message by ID from queueName via the
// removeMessage(java.lang.String) Jolokia exec operation.
func (c *Client) RemoveMessage(ctx context.Context, queueName, messageID string) error {
	mbean := fmt.Sprintf(
		"org.apache.activemq:type=Broker,brokerName=%s,destinationType=Queue,destinationName=%s",
		c.cfg.BrokerName, queueName,
	)
	reqBody := map[string]any{
		"type":      "exec",
		"mbean":     mbean,
		"operation": "removeMessage(java.lang.String)",
		"arguments": []string{messageID},
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("removeMessage marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("removeMessage request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	c.setHeaders(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("removeMessage: %w", err)
	}
	var result struct {
		Status int    `json:"status"`
		Error  string `json:"error"`
		Value  any    `json:"value"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	resp.Body.Close()
	if err != nil {
		return fmt.Errorf("removeMessage decode: %w", err)
	}
	if result.Status != 200 {
		return fmt.Errorf("removeMessage error (status %d): %s", result.Status, result.Error)
	}
	if removed, ok := result.Value.(bool); ok && !removed {
		return fmt.Errorf("removeMessage returned false: message %q not found in queue %q", messageID, queueName)
	}
	return nil
}

// MoveMessage moves a single message from sourceQueue to targetQueue via the
// moveMessageTo(java.lang.String,java.lang.String) Jolokia exec operation.
func (c *Client) MoveMessage(ctx context.Context, sourceQueue, messageID, targetQueue string) error {
	mbean := fmt.Sprintf(
		"org.apache.activemq:type=Broker,brokerName=%s,destinationType=Queue,destinationName=%s",
		c.cfg.BrokerName, sourceQueue,
	)
	reqBody := map[string]any{
		"type":      "exec",
		"mbean":     mbean,
		"operation": "moveMessageTo(java.lang.String,java.lang.String)",
		"arguments": []string{messageID, targetQueue},
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("moveMessage marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("moveMessage request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	c.setHeaders(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("moveMessage: %w", err)
	}
	var result struct {
		Status int    `json:"status"`
		Error  string `json:"error"`
		Value  any    `json:"value"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	resp.Body.Close()
	if err != nil {
		return fmt.Errorf("moveMessage decode: %w", err)
	}
	if result.Status != 200 {
		return fmt.Errorf("moveMessage error (status %d): %s", result.Status, result.Error)
	}
	if moved, ok := result.Value.(bool); ok && !moved {
		return fmt.Errorf("moveMessageTo returned false: message %q not found in queue %q", messageID, sourceQueue)
	}
	return nil
}

// MoveAllMessages moves all messages from sourceQueue to targetQueue via the
// moveMatchingMessagesTo(java.lang.String,java.lang.String) Jolokia exec
// operation with selector "TRUE". Returns the number of messages moved.
func (c *Client) MoveAllMessages(ctx context.Context, sourceQueue, targetQueue string) (int, error) {
	mbean := fmt.Sprintf(
		"org.apache.activemq:type=Broker,brokerName=%s,destinationType=Queue,destinationName=%s",
		c.cfg.BrokerName, sourceQueue,
	)
	reqBody := map[string]any{
		"type":      "exec",
		"mbean":     mbean,
		"operation": "moveMatchingMessagesTo(java.lang.String,java.lang.String)",
		"arguments": []string{"TRUE", targetQueue},
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return 0, fmt.Errorf("moveAllMessages marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.URL, bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("moveAllMessages request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	c.setHeaders(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("moveAllMessages: %w", err)
	}
	var result struct {
		Status int    `json:"status"`
		Error  string `json:"error"`
		Value  any    `json:"value"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	resp.Body.Close()
	if err != nil {
		return 0, fmt.Errorf("moveAllMessages decode: %w", err)
	}
	if result.Status != 200 {
		return 0, fmt.Errorf("moveAllMessages error (status %d): %s", result.Status, result.Error)
	}
	count := 0
	if f, ok := result.Value.(float64); ok {
		count = int(f)
	}
	return count, nil
}

// SendMessage sends body as a JMS TextMessage to queueName via the Jolokia
// JMX sendTextMessage operation.
//
// Side-effect: the JMX path creates a short-lived VM-transport connection that
// is stored as a closed reference inside the message's producer info. This
// causes browseMessages() to fail with "Error while extracting clientID" for
// the affected queue. BrowseMessages() handles this transparently by falling
// back to browse() which returns the message bodies without IDs, so the body
// is still readable and individual move/delete is replaced by the
// "limited info" mode indicated in the status bar.
func (c *Client) SendMessage(ctx context.Context, queueName, body string) error {
	mbean := fmt.Sprintf(
		"org.apache.activemq:type=Broker,brokerName=%s,destinationType=Queue,destinationName=%s",
		c.cfg.BrokerName, queueName,
	)
	reqBody := map[string]any{
		"type":      "exec",
		"mbean":     mbean,
		"operation": "sendTextMessage(java.util.Map,java.lang.String,java.lang.String,java.lang.String)",
		"arguments": []any{map[string]string{}, body, c.cfg.Username, c.cfg.Password},
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("sendTextMessage marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.URL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("sendTextMessage request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	c.setHeaders(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("sendTextMessage: %w", err)
	}
	var result struct {
		Status int    `json:"status"`
		Error  string `json:"error"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	resp.Body.Close()
	if err != nil {
		return fmt.Errorf("sendTextMessage decode: %w", err)
	}
	if result.Status != 200 {
		return fmt.Errorf("sendTextMessage error (status %d): %s", result.Status, result.Error)
	}
	return nil
}
