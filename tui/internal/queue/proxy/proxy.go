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

// apiError is a single entry in a response envelope's errors.
type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// listEnvelope is the {data, errors} shape every list-returning
// mq-proxy endpoint responds with.
type listEnvelope[T any] struct {
	Data   []T        `json:"data"`
	Errors []apiError `json:"errors"`
}

// itemEnvelope is the {data, error} shape single-result mq-proxy
// endpoints (send-message) respond with.
type itemEnvelope[T any] struct {
	Data  *T        `json:"data"`
	Error *apiError `json:"error"`
}

// proxyQueue is the JSON shape of one entry in list-queues' data array.
type proxyQueue struct {
	Name          string `json:"name"`
	MessageCount  int64  `json:"messageCount"`
	ConsumerCount int64  `json:"consumerCount"`
	EnqueuedCount int64  `json:"enqueuedCount"`
	DequeuedCount int64  `json:"dequeuedCount"`
	ProducerCount int64  `json:"producerCount"`
}

// proxyMessage is the JSON shape of one entry in list-messages' data array.
// Headers is map[string]interface{}, not map[string]string: mq-proxy always
// stringifies header values, but a JMS-management API implementing this same
// contract may return a message's real property types (numbers, booleans,
// etc.) instead — the UI already renders arbitrary property value types
// (see message_detail.go's decodePropertyValue), so there's no reason to
// force a decode failure by assuming every value is a string.
type proxyMessage struct {
	SourceQueue string                 `json:"sourceQueue"`
	MessageID   string                 `json:"messageId"`
	JMSType     string                 `json:"jmsType"`
	Body        *string                `json:"body"`
	Timestamp   string                 `json:"timestamp"`
	Headers     map[string]interface{} `json:"headers"`
}

// queueMessageFilter mirrors mq-proxy's QueueMessageFilter DTO. Every
// field is optional; the zero value matches every message on the queue.
type queueMessageFilter struct {
	JMSType   string `json:"jmsType,omitempty"`
	FromDate  string `json:"fromDate,omitempty"`
	ToDate    string `json:"toDate,omitempty"`
	MessageID string `json:"messageId,omitempty"`
	MaxCount  *int   `json:"maxCount,omitempty"`
}

// toFilterDTO converts a queue.MessageFilter (Go-side, MaxCount 0 means
// "unlimited") into the wire shape mq-proxy expects (MaxCount omitted
// entirely means "unlimited" — a present 0 would mean "match nothing").
func toFilterDTO(f queue.MessageFilter) queueMessageFilter {
	dto := queueMessageFilter{JMSType: f.JMSType, MessageID: f.MessageID}
	if !f.FromDate.IsZero() {
		dto.FromDate = f.FromDate.UTC().Format(time.RFC3339)
	}
	if !f.ToDate.IsZero() {
		dto.ToDate = f.ToDate.UTC().Format(time.RFC3339)
	}
	if f.MaxCount > 0 {
		maxCount := f.MaxCount
		dto.MaxCount = &maxCount
	}
	return dto
}

type deleteMessagesRequest struct {
	SourceQueue string             `json:"sourceQueue"`
	Filter      queueMessageFilter `json:"filter"`
}

type moveMessagesRequest struct {
	SourceQueue string             `json:"sourceQueue"`
	TargetQueue string             `json:"targetQueue"`
	Filter      queueMessageFilter `json:"filter"`
}

type sendMessageRequest struct {
	TargetQueue   string            `json:"targetQueue"`
	JMSType       string            `json:"jmsType"`
	Headers       map[string]string `json:"headers,omitempty"`
	GroupID       string            `json:"groupId,omitempty"`
	Body          string            `json:"body"`
	CorrelationID string            `json:"correlationId,omitempty"`
}

type sendMessageResponse struct {
	MessageID string `json:"messageId"`
}

type deletedMessage struct {
	MessageID string `json:"messageId"`
}

type movedMessage struct {
	MessageID string `json:"messageId"`
}

// List implements queue.Backend.
func (c *Client) List(ctx context.Context) ([]queue.Summary, error) {
	var env listEnvelope[proxyQueue]
	if err := c.getJSON(ctx, "/api/management/command/list-queues", &env); err != nil {
		return nil, fmt.Errorf("list queues: %w", err)
	}
	if err := envelopeErr(env.Errors); err != nil {
		return nil, fmt.Errorf("list queues: %w", err)
	}
	out := make([]queue.Summary, len(env.Data))
	for i, q := range env.Data {
		out[i] = queue.Summary{
			Name:          q.Name,
			PendingCount:  q.MessageCount,
			ConsumerCount: q.ConsumerCount,
			EnqueueCount:  q.EnqueuedCount,
			DequeueCount:  q.DequeuedCount,
			ProducerCount: q.ProducerCount,
		}
	}
	return out, nil
}

// BrowseMessages implements queue.Backend.
func (c *Client) BrowseMessages(ctx context.Context, queueName string) ([]queue.Message, error) {
	path := "/api/management/command/list-messages?sourceQueue=" + url.QueryEscape(queueName)
	var env listEnvelope[proxyMessage]
	if err := c.getJSON(ctx, path, &env); err != nil {
		return nil, fmt.Errorf("browse messages %q: %w", queueName, err)
	}
	if err := envelopeErr(env.Errors); err != nil {
		return nil, fmt.Errorf("browse messages %q: %w", queueName, err)
	}
	out := make([]queue.Message, len(env.Data))
	for i, m := range env.Data {
		out[i] = toQueueMessage(m)
	}
	return out, nil
}

// PurgeQueue implements queue.Backend.
func (c *Client) PurgeQueue(ctx context.Context, queueName string) error {
	if _, err := c.DeleteMessages(ctx, queueName, queue.MessageFilter{}); err != nil {
		return fmt.Errorf("purge queue %q: %w", queueName, err)
	}
	return nil
}

// RemoveMessage implements queue.Backend.
func (c *Client) RemoveMessage(ctx context.Context, queueName, messageID string) error {
	n, err := c.DeleteMessages(ctx, queueName, queue.MessageFilter{MessageID: messageID, MaxCount: 1})
	if err != nil {
		return fmt.Errorf("remove message %q from %q: %w", messageID, queueName, err)
	}
	if n == 0 {
		return fmt.Errorf("remove message %q from %q: not found", messageID, queueName)
	}
	return nil
}

// MoveMessage implements queue.Backend.
func (c *Client) MoveMessage(ctx context.Context, sourceQueue, messageID, targetQueue string) error {
	n, err := c.MoveMessages(ctx, sourceQueue, targetQueue, queue.MessageFilter{MessageID: messageID, MaxCount: 1})
	if err != nil {
		return fmt.Errorf("move message %q from %q to %q: %w", messageID, sourceQueue, targetQueue, err)
	}
	if n == 0 {
		return fmt.Errorf("move message %q from %q to %q: not found", messageID, sourceQueue, targetQueue)
	}
	return nil
}

// MoveAllMessages implements queue.Backend.
func (c *Client) MoveAllMessages(ctx context.Context, sourceQueue, targetQueue string) (int, error) {
	n, err := c.MoveMessages(ctx, sourceQueue, targetQueue, queue.MessageFilter{})
	if err != nil {
		return 0, fmt.Errorf("move all from %q to %q: %w", sourceQueue, targetQueue, err)
	}
	return n, nil
}

// SendMessage implements queue.Backend.
func (c *Client) SendMessage(ctx context.Context, queueName, body string) error {
	req := sendMessageRequest{TargetQueue: queueName, JMSType: "text", Body: body}
	var env itemEnvelope[sendMessageResponse]
	if err := c.postJSON(ctx, "/api/management/command/send-message", req, &env); err != nil {
		return fmt.Errorf("send message to %q: %w", queueName, err)
	}
	if env.Error != nil {
		return fmt.Errorf("send message to %q: %s: %s", queueName, env.Error.Code, env.Error.Message)
	}
	return nil
}

// DeleteMessages implements queue.Backend.
func (c *Client) DeleteMessages(ctx context.Context, queueName string, filter queue.MessageFilter) (int, error) {
	reqs := []deleteMessagesRequest{{SourceQueue: queueName, Filter: toFilterDTO(filter)}}
	var env listEnvelope[deletedMessage]
	if err := c.postJSON(ctx, "/api/management/command/delete-messages", reqs, &env); err != nil {
		return 0, err
	}
	if err := envelopeErr(env.Errors); err != nil {
		return 0, err
	}
	return len(env.Data), nil
}

// MoveMessages implements queue.Backend.
func (c *Client) MoveMessages(ctx context.Context, sourceQueue, targetQueue string, filter queue.MessageFilter) (int, error) {
	reqs := []moveMessagesRequest{{SourceQueue: sourceQueue, TargetQueue: targetQueue, Filter: toFilterDTO(filter)}}
	var env listEnvelope[movedMessage]
	if err := c.postJSON(ctx, "/api/management/command/move-messages", reqs, &env); err != nil {
		return 0, err
	}
	if err := envelopeErr(env.Errors); err != nil {
		return 0, err
	}
	return len(env.Data), nil
}

// envelopeErr builds a Go error from the first entry of a response
// envelope's errors, or nil if there are none.
func envelopeErr(errs []apiError) error {
	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("%s: %s", errs[0].Code, errs[0].Message)
}

// toQueueMessage converts a proxyMessage into a queue.Message. Prefers
// the real jmsType mq-proxy reports (the JMS JMSType header) and only
// falls back to inferring "text"/"other" from body presence when it's
// empty — mirrors the Jolokia backend's jMSType-header-first pattern
// (internal/queue/jolokia/jolokia.go).
func toQueueMessage(m proxyMessage) queue.Message {
	ts, _ := time.Parse(time.RFC3339, m.Timestamp)

	jmsType := m.JMSType
	var bodyText string
	var preview string
	if m.Body != nil {
		bodyText = *m.Body
		preview = bodyText
		if len([]rune(preview)) > 80 {
			preview = string([]rune(preview)[:80])
		}
		if jmsType == "" {
			jmsType = "text"
		}
	} else if jmsType == "" {
		jmsType = "other"
	}

	return queue.Message{
		ID:        m.MessageID,
		JMSType:   jmsType,
		Timestamp: ts,
		Preview:   preview,
		RawFields: map[string]interface{}{
			"text":       bodyText,
			"properties": m.Headers,
		},
	}
}

// getJSON performs a GET and JSON-decodes the response into out.
func (c *Client) getJSON(ctx context.Context, path string, out interface{}) error {
	return c.doRequest(ctx, http.MethodGet, path, nil, out)
}

// postJSON JSON-encodes body, POSTs it, and JSON-decodes the response into out.
func (c *Client) postJSON(ctx context.Context, path string, body interface{}, out interface{}) error {
	encoded, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encoding request: %w", err)
	}
	return c.doRequest(ctx, http.MethodPost, path, strings.NewReader(string(encoded)), out)
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
