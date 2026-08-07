// Package proxy implements queue.Backend by calling the mq-proxy REST API.
package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/queue"
)

// Client calls the mq-proxy REST API and implements queue.Backend.
type Client struct {
	baseURL    string
	username   string
	password   string
	httpClient *http.Client
}

// NewClient returns a Client configured from cfg.
func NewClient(cfg config.ProxyConfig) *Client {
	return &Client{
		baseURL:    strings.TrimRight(cfg.URL, "/"),
		username:   cfg.Username,
		password:   cfg.Password,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// proxyQueue is the JSON shape returned by GET /api/queues.
type proxyQueue struct {
	Name          string `json:"name"`
	PendingCount  int64  `json:"pendingCount"`
	ConsumerCount int64  `json:"consumerCount"`
	EnqueueCount  int64  `json:"enqueueCount"`
	DequeueCount  int64  `json:"dequeueCount"`
}

// proxyMessage is the JSON shape returned by GET /api/queues/{name}/messages.
type proxyMessage struct {
	ID         string            `json:"id"`
	Timestamp  string            `json:"timestamp"`
	Body       *string           `json:"body"`
	Properties map[string]string `json:"properties"`
}

// List implements queue.Backend.
func (c *Client) List(ctx context.Context) ([]queue.Summary, error) {
	var resp []proxyQueue
	if err := c.getJSON(ctx, "/api/queues", &resp); err != nil {
		return nil, fmt.Errorf("list queues: %w", err)
	}
	out := make([]queue.Summary, len(resp))
	for i, q := range resp {
		out[i] = queue.Summary{
			Name:          q.Name,
			PendingCount:  q.PendingCount,
			ConsumerCount: q.ConsumerCount,
			EnqueueCount:  q.EnqueueCount,
			DequeueCount:  q.DequeueCount,
		}
	}
	return out, nil
}

// BrowseMessages implements queue.Backend.
func (c *Client) BrowseMessages(ctx context.Context, queueName string) ([]queue.Message, error) {
	var resp []proxyMessage
	path := "/api/queues/" + url.PathEscape(queueName) + "/messages"
	if err := c.getJSON(ctx, path, &resp); err != nil {
		return nil, fmt.Errorf("browse messages %q: %w", queueName, err)
	}
	out := make([]queue.Message, len(resp))
	for i, m := range resp {
		out[i] = toQueueMessage(m)
	}
	return out, nil
}

// PurgeQueue implements queue.Backend.
func (c *Client) PurgeQueue(ctx context.Context, queueName string) error {
	path := "/api/queues/" + url.PathEscape(queueName) + "/messages"
	if err := c.doRequest(ctx, http.MethodDelete, path, nil, nil); err != nil {
		return fmt.Errorf("purge queue %q: %w", queueName, err)
	}
	return nil
}

// RemoveMessage implements queue.Backend.
func (c *Client) RemoveMessage(ctx context.Context, queueName, messageID string) error {
	path := "/api/queues/" + url.PathEscape(queueName) + "/messages/" + url.PathEscape(messageID)
	if err := c.doRequest(ctx, http.MethodDelete, path, nil, nil); err != nil {
		return fmt.Errorf("remove message %q from %q: %w", messageID, queueName, err)
	}
	return nil
}

// MoveMessage implements queue.Backend.
func (c *Client) MoveMessage(ctx context.Context, sourceQueue, messageID, targetQueue string) error {
	path := "/api/queues/" + url.PathEscape(sourceQueue) +
		"/messages/" + url.PathEscape(messageID) +
		"/move?to=" + url.QueryEscape(targetQueue)
	if err := c.doRequest(ctx, http.MethodPost, path, nil, nil); err != nil {
		return fmt.Errorf("move message %q from %q to %q: %w", messageID, sourceQueue, targetQueue, err)
	}
	return nil
}

// MoveAllMessages implements queue.Backend.
func (c *Client) MoveAllMessages(ctx context.Context, sourceQueue, targetQueue string) (int, error) {
	var resp struct {
		Moved int `json:"moved"`
	}
	path := "/api/queues/" + url.PathEscape(sourceQueue) + "/move?to=" + url.QueryEscape(targetQueue)
	if err := c.doRequest(ctx, http.MethodPost, path, nil, &resp); err != nil {
		return 0, fmt.Errorf("move all from %q to %q: %w", sourceQueue, targetQueue, err)
	}
	return resp.Moved, nil
}

// SendMessage implements queue.Backend.
func (c *Client) SendMessage(ctx context.Context, queueName, body string) error {
	path := "/api/queues/" + url.PathEscape(queueName) + "/messages"
	if err := c.doRequest(ctx, http.MethodPost, path, strings.NewReader(body), nil); err != nil {
		return fmt.Errorf("send message to %q: %w", queueName, err)
	}
	return nil
}

// toQueueMessage converts a proxyMessage into a queue.Message.
func toQueueMessage(m proxyMessage) queue.Message {
	ts, _ := time.Parse(time.RFC3339, m.Timestamp)

	jmsType := "other"
	var bodyText string
	var preview string
	if m.Body != nil {
		jmsType = "text"
		bodyText = *m.Body
		preview = bodyText
		if len([]rune(preview)) > 80 {
			preview = string([]rune(preview)[:80])
		}
	}

	props := make(map[string]interface{}, len(m.Properties))
	for k, v := range m.Properties {
		props[k] = v
	}

	return queue.Message{
		ID:        m.ID,
		JMSType:   jmsType,
		Timestamp: ts,
		Preview:   preview,
		RawFields: map[string]interface{}{
			"text":       bodyText,
			"properties": props,
		},
	}
}

// getJSON performs a GET and JSON-decodes the response into out.
func (c *Client) getJSON(ctx context.Context, path string, out interface{}) error {
	return c.doRequest(ctx, http.MethodGet, path, nil, out)
}

// doRequest executes an HTTP request against the proxy.
// body may be nil for requests without a payload.
// out may be nil to discard the response body.
func (c *Client) doRequest(ctx context.Context, method, path string, body io.Reader, out interface{}) error {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.SetBasicAuth(c.username, c.password)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("proxy returned %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}

	if out != nil && resp.StatusCode != http.StatusNoContent {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}
	}
	return nil
}
