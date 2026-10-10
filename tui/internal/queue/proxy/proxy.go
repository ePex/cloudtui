// Package proxy implements queue.Backend by calling the mq-proxy REST API.
package proxy

import (
	"context"
	"encoding/json"
	"errors"
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

// listEnvelope is the {data, errors, hasMore, truncated} shape every
// list-returning mq-proxy endpoint responds with. Only list-messages sets
// hasMore (another page can be fetched) and truncated (the broker cut the
// browse short) — see spec/11.
type listEnvelope[T any] struct {
	Data      []T        `json:"data"`
	Errors    []apiError `json:"errors"`
	HasMore   bool       `json:"hasMore"`
	Truncated bool       `json:"truncated"`
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
// Headers is map[string]any, not map[string]string: mq-proxy always
// stringifies header values, but a JMS-management API implementing this same
// contract may return a message's real property types (numbers, booleans,
// etc.) instead — the UI already renders arbitrary property value types
// (see message_detail.go's decodePropertyValue), so there's no reason to
// force a decode failure by assuming every value is a string.
type proxyMessage struct {
	SourceQueue string         `json:"sourceQueue"`
	MessageID   string         `json:"messageId"`
	JMSType     string         `json:"jmsType"`
	Body        *string        `json:"body"`
	Timestamp   string         `json:"timestamp"`
	Headers     map[string]any `json:"headers"`
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

// browseQuery builds the list-messages query string for queueName and
// filter, reusing toFilterDTO's field mapping. returnBody is always sent
// explicitly true: the message browser needs the body for its preview
// column, regardless of mq-proxy's own default.
//
// Filter fields nest under "filter." (filter.jmsType, filter.maxCount,
// etc.) — mq-proxy binds list-messages' query the same nested way its
// delete-messages/move-messages JSON bodies already did, matching the
// reference API's convention too (spec/49-cr-mq-proxy-nested-filter-query).
// sourceQueue/returnBody stay top-level on both.
func browseQuery(queueName string, filter queue.MessageFilter) string {
	dto := toFilterDTO(filter)
	q := url.Values{}
	q.Set("sourceQueue", queueName)
	q.Set("returnBody", "true")
	if dto.JMSType != "" {
		q.Set("filter.jmsType", dto.JMSType)
	}
	if dto.MessageID != "" {
		q.Set("filter.messageId", dto.MessageID)
	}
	if dto.FromDate != "" {
		q.Set("filter.fromDate", dto.FromDate)
	}
	if dto.ToDate != "" {
		q.Set("filter.toDate", dto.ToDate)
	}
	if dto.MaxCount != nil {
		q.Set("filter.maxCount", fmt.Sprintf("%d", *dto.MaxCount))
	}
	return q.Encode()
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

// BrowseMessages implements queue.Backend. Its bool is "incomplete": either
// of list-messages' hasMore or truncated — the TUI only needs to know the
// queue holds more than it was shown, not which.
func (c *Client) BrowseMessages(ctx context.Context, queueName string, filter queue.MessageFilter) ([]queue.Message, bool, error) {
	path := "/api/management/command/list-messages?" + browseQuery(queueName, filter)
	var env listEnvelope[proxyMessage]
	if err := c.getJSON(ctx, path, &env); err != nil {
		return nil, false, fmt.Errorf("browse messages %q: %w", queueName, err)
	}
	if err := envelopeErr(env.Errors); err != nil {
		return nil, false, fmt.Errorf("browse messages %q: %w", queueName, err)
	}
	out := make([]queue.Message, len(env.Data))
	for i, m := range env.Data {
		out[i] = toQueueMessage(m)
	}
	return out, env.HasMore || env.Truncated, nil
}

// PurgeQueue implements queue.Backend.
func (c *Client) PurgeQueue(ctx context.Context, queueName string) (int, error) {
	n, err := c.DeleteMessages(ctx, queueName, queue.MessageFilter{})
	if err != nil {
		return n, fmt.Errorf("purge queue %q: %w", queueName, err)
	}
	return n, nil
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
		return n, fmt.Errorf("move all from %q to %q: %w", sourceQueue, targetQueue, err)
	}
	return n, nil
}

// SendMessage implements queue.Backend.
func (c *Client) SendMessage(ctx context.Context, queueName string, sendReq queue.SendMessageRequest) error {
	req := sendMessageRequest{
		TargetQueue:   queueName,
		JMSType:       sendReq.JMSType,
		Headers:       sendReq.Headers,
		GroupID:       sendReq.GroupID,
		Body:          sendReq.Body,
		CorrelationID: sendReq.CorrelationID,
	}
	var env itemEnvelope[sendMessageResponse]
	if err := c.postJSON(ctx, "/api/management/command/send-message", req, &env); err != nil {
		return fmt.Errorf("send message to %q: %w", queueName, err)
	}
	if env.Error != nil {
		return fmt.Errorf("send message to %q: %s: %s", queueName, env.Error.Code, env.Error.Message)
	}
	return nil
}

// maxBulkRounds caps how many requests one bulk delete/move makes (see
// inRounds). At the ~400 messages per request a browse-capped server
// manages, that's 400,000 messages.
const maxBulkRounds = 1000

// errBulkIncomplete is returned (wrapped) when a bulk delete/move hits
// maxBulkRounds while the server still reports progress.
var errBulkIncomplete = errors.New("incomplete")

// errBulkNoProgress is returned (wrapped) when a round reports messages
// but none that an earlier round hadn't already reported.
var errBulkNoProgress = errors.New("incomplete")

// inRounds repeats round until the server reports nothing left, and
// returns how many messages were removed. Nothing in the wire contract
// (spec/11) promises that one delete-messages/move-messages call acts on
// every matching message - a server that browses before consuming stops at
// the broker's browse limit - so a bulk operation keeps asking.
//
// round returns the message IDs it reports (entries without an ID dropped)
// and how many entries the response had. Only IDs not seen in an earlier
// round count: a really removed message can't be reported again, so a
// round that reports entries but nothing new made no progress - typically
// a server reporting messages it browsed but couldn't consume - and ends
// the repetition with errBulkNoProgress instead of repeating them until
// maxBulkRounds. maxBulkRounds stays as the last resort; the context is
// checked between rounds. On error, the total so far comes back alongside
// it.
func inRounds(ctx context.Context, round func() (ids []string, reported int, err error)) (int, error) {
	seen := make(map[string]bool)
	total := 0
	for range maxBulkRounds {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		ids, reported, err := round()
		fresh := 0
		for _, id := range ids {
			if !seen[id] {
				seen[id] = true
				fresh++
			}
		}
		total += fresh
		if err != nil {
			return total, err
		}
		if reported == 0 {
			return total, nil
		}
		if fresh == 0 {
			// Everything this round reported was counted in an earlier
			// round, yet it's still there: those weren't removed after all.
			total -= len(ids)
			return total, fmt.Errorf("%w: the server kept reporting %d message(s) it didn't remove (%d removed)",
				errBulkNoProgress, reported, total)
		}
	}
	return total, fmt.Errorf("%w after %d requests (%d messages so far); the server kept reporting progress",
		errBulkIncomplete, maxBulkRounds, total)
}

// DeleteMessages implements queue.Backend. Without a MaxCount ("all
// matching") it repeats until nothing is left (inRounds); with one, it's a
// single request.
func (c *Client) DeleteMessages(ctx context.Context, queueName string, filter queue.MessageFilter) (int, error) {
	if filter.MaxCount > 0 {
		ids, _, err := c.deleteOnce(ctx, queueName, filter)
		return len(ids), err
	}
	return inRounds(ctx, func() ([]string, int, error) { return c.deleteOnce(ctx, queueName, filter) })
}

// deleteOnce makes a single delete-messages request and returns the IDs
// of the messages the server reports as deleted, plus how many entries the
// response had (inRounds tells "nothing left" from "no progress" by it).
// An entry without a
// message ID doesn't count: nothing says that message was really removed
// — a server that browses before consuming can report a browsed message
// whose consume came back empty (spec/11).
func (c *Client) deleteOnce(ctx context.Context, queueName string, filter queue.MessageFilter) ([]string, int, error) {
	reqs := []deleteMessagesRequest{{SourceQueue: queueName, Filter: toFilterDTO(filter)}}
	var env listEnvelope[deletedMessage]
	if err := c.postJSON(ctx, "/api/management/command/delete-messages", reqs, &env); err != nil {
		return nil, 0, err
	}
	if err := envelopeErr(env.Errors); err != nil {
		return nil, 0, err
	}
	ids := make([]string, 0, len(env.Data))
	for _, m := range env.Data {
		if m.MessageID != "" {
			ids = append(ids, m.MessageID)
		}
	}
	return ids, len(env.Data), nil
}

// MoveMessages implements queue.Backend, repeating like DeleteMessages
// when filter has no MaxCount.
func (c *Client) MoveMessages(ctx context.Context, sourceQueue, targetQueue string, filter queue.MessageFilter) (int, error) {
	if filter.MaxCount > 0 {
		ids, _, err := c.moveOnce(ctx, sourceQueue, targetQueue, filter)
		return len(ids), err
	}
	return inRounds(ctx, func() ([]string, int, error) { return c.moveOnce(ctx, sourceQueue, targetQueue, filter) })
}

// moveOnce makes a single move-messages request and returns the IDs of
// the messages the server reports as moved, plus the response's entry
// count — like deleteOnce, entries without a message ID don't count.
func (c *Client) moveOnce(ctx context.Context, sourceQueue, targetQueue string, filter queue.MessageFilter) ([]string, int, error) {
	reqs := []moveMessagesRequest{{SourceQueue: sourceQueue, TargetQueue: targetQueue, Filter: toFilterDTO(filter)}}
	var env listEnvelope[movedMessage]
	if err := c.postJSON(ctx, "/api/management/command/move-messages", reqs, &env); err != nil {
		return nil, 0, err
	}
	if err := envelopeErr(env.Errors); err != nil {
		return nil, 0, err
	}
	ids := make([]string, 0, len(env.Data))
	for _, m := range env.Data {
		if m.MessageID != "" {
			ids = append(ids, m.MessageID)
		}
	}
	return ids, len(env.Data), nil
}

// envelopeErr builds a Go error from the first entry of a response
// envelope's errors, or nil if there are none.
func envelopeErr(errs []apiError) error {
	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("%s: %s", errs[0].Code, errs[0].Message)
}

// previewMaxLen caps queue.Message.Preview's length — see the Jolokia
// backend's identical constant (internal/queue/jolokia/messages.go) for
// why: the full body is already in memory by the time this is applied,
// so raising it costs nothing per request, and it only needs to be
// generous enough that the message browser's wrap toggle rarely hits it
// before its own line cap does.
const previewMaxLen = 2000

// toQueueMessage converts a proxyMessage into a queue.Message. Prefers
// the real jmsType mq-proxy reports (the JMS JMSType header) and only
// falls back to inferring "text"/"other" from body presence when it's
// empty — mirrors the Jolokia backend's jMSType-header-first pattern
// (internal/queue/jolokia/jolokia.go).
func toQueueMessage(m proxyMessage) queue.Message {
	ts, _ := time.Parse(time.RFC3339, m.Timestamp)

	jmsType := m.JMSType
	inferred := jmsType == ""
	var bodyText string
	var preview string
	if m.Body != nil {
		bodyText = *m.Body
		preview = bodyText
		if len([]rune(preview)) > previewMaxLen {
			preview = string([]rune(preview)[:previewMaxLen])
		}
		if inferred {
			jmsType = "text"
		}
	} else if inferred {
		jmsType = "other"
	}

	return queue.Message{
		ID:        m.MessageID,
		JMSType:   jmsType,
		Timestamp: ts,
		Preview:   preview,
		RawFields: map[string]any{
			"text":       bodyText,
			"properties": m.Headers,
		},
		JMSTypeInferred: inferred,
	}
}

// getJSON performs a GET and JSON-decodes the response into out.
func (c *Client) getJSON(ctx context.Context, path string, out any) error {
	return c.doRequest(ctx, http.MethodGet, path, nil, out)
}

// postJSON JSON-encodes body, POSTs it, and JSON-decodes the response into out.
// Every POST changes the broker (reads are GETs), so it ignores ctx's
// cancellation: a stop (Esc in the progress modal, spec/24)
// never abandons a request that may already have been
// applied, losing its result. Bulk operations stop between requests
// instead (inRounds).
func (c *Client) postJSON(ctx context.Context, path string, body any, out any) error {
	encoded, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encoding request: %w", err)
	}
	return c.doRequest(context.WithoutCancel(ctx), http.MethodPost, path, strings.NewReader(string(encoded)), out)
}

// doRequest executes an HTTP request against the proxy.
// body may be nil for requests without a payload.
// out may be nil to discard the response body.
func (c *Client) doRequest(ctx context.Context, method, path string, body io.Reader, out any) error {
	newReq := func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
		if err != nil {
			return nil, err
		}
		req.SetBasicAuth(c.username, c.password)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		return req, nil
	}

	req, err := newReq()
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil && method == http.MethodGet {
		// The proxy can time out waiting for headers on its first request
		// after a period of inactivity (JVM/DB connection warmup on
		// AWS-hosted deployments) even though it's otherwise healthy — a
		// second identical request typically succeeds immediately.
		// Retried only for GET: a body-less, side-effect-free request is
		// always safe to repeat; a POST that timed out waiting for a
		// response may already have been applied server-side, so retrying
		// it risks applying it twice. body is always nil for GET (only
		// postJSON passes a non-nil body), so newReq() needs no rewind.
		if retryReq, rerr := newReq(); rerr == nil {
			resp, err = c.httpClient.Do(retryReq)
		}
	}
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
