package jolokia

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/queue"
)

// Client implements queue.Backend against an ActiveMQ Jolokia endpoint.
type Client struct {
	cfg    config.QueueConfig
	http   *http.Client
	origin string
}

// NewClient returns a Jolokia-backed queue client.
func NewClient(cfg config.QueueConfig) *Client {
	return &Client{
		cfg:    cfg,
		http:   &http.Client{},
		origin: "http://localhost",
	}
}

// searchResponse is the top-level Jolokia search reply.
type searchResponse struct {
	Status int      `json:"status"`
	Value  []string `json:"value"`
	Error  string   `json:"error"`
}

// bulkItem is one entry in a Jolokia bulk read request/response.
type bulkItem struct {
	Type      string `json:"type"`
	MBean     string `json:"mbean"`
	Attribute string `json:"attribute"`
}

type bulkResponseItem struct {
	Status int   `json:"status"`
	Value  int64 `json:"value"`
	Error  string `json:"error"`
}

// List fetches all queues from ActiveMQ via Jolokia and returns their summaries.
func (c *Client) List(ctx context.Context) ([]queue.Summary, error) {
	// Step 1: search for all queue MBeans.
	mbeans, err := c.searchQueues(ctx)
	if err != nil {
		return nil, err
	}
	if len(mbeans) == 0 {
		return nil, nil
	}

	// Step 2: bulk read QueueSize and ConsumerCount for all queues.
	return c.readMetrics(ctx, mbeans)
}

func (c *Client) searchQueues(ctx context.Context) ([]string, error) {
	pattern := fmt.Sprintf("org.apache.activemq:type=Broker,brokerName=%s,destinationType=Queue,destinationName=*", c.cfg.BrokerName)
	url := fmt.Sprintf("%s/search/%s", c.cfg.URL, pattern)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("jolokia search request: %w", err)
	}
	c.setHeaders(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jolokia search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jolokia search: HTTP %d", resp.StatusCode)
	}

	var sr searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, fmt.Errorf("jolokia search decode: %w", err)
	}
	if sr.Status != 200 {
		return nil, fmt.Errorf("jolokia search error (status %d): %s", sr.Status, sr.Error)
	}

	return sr.Value, nil
}

func (c *Client) readMetrics(ctx context.Context, mbeans []string) ([]queue.Summary, error) {
	// Build a bulk request: four entries per queue.
	bulk := make([]bulkItem, 0, len(mbeans)*4)
	for _, mb := range mbeans {
		bulk = append(bulk,
			bulkItem{Type: "read", MBean: mb, Attribute: "QueueSize"},
			bulkItem{Type: "read", MBean: mb, Attribute: "ConsumerCount"},
			bulkItem{Type: "read", MBean: mb, Attribute: "EnqueueCount"},
			bulkItem{Type: "read", MBean: mb, Attribute: "DequeueCount"},
		)
	}

	body, err := json.Marshal(bulk)
	if err != nil {
		return nil, fmt.Errorf("jolokia bulk marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.URL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("jolokia bulk request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	c.setHeaders(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jolokia bulk read: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jolokia bulk read: HTTP %d", resp.StatusCode)
	}

	var results []bulkResponseItem
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, fmt.Errorf("jolokia bulk decode: %w", err)
	}

	// Each queue produced four consecutive entries:
	// [QueueSize, ConsumerCount, EnqueueCount, DequeueCount].
	summaries := make([]queue.Summary, 0, len(mbeans))
	for i, mb := range mbeans {
		queueSize    := results[i*4]
		consumerCount := results[i*4+1]
		enqueueCount  := results[i*4+2]
		dequeueCount  := results[i*4+3]

		for _, r := range []struct {
			item bulkResponseItem
			name string
		}{
			{queueSize, "QueueSize"},
			{consumerCount, "ConsumerCount"},
			{enqueueCount, "EnqueueCount"},
			{dequeueCount, "DequeueCount"},
		} {
			if r.item.Status != 200 {
				return nil, fmt.Errorf("jolokia read %s error (status %d): %s", r.name, r.item.Status, r.item.Error)
			}
		}

		summaries = append(summaries, queue.Summary{
			Name:          queueNameFromMBean(mb),
			PendingCount:  queueSize.Value,
			ConsumerCount: consumerCount.Value,
			EnqueueCount:  enqueueCount.Value,
			DequeueCount:  dequeueCount.Value,
		})
	}

	return summaries, nil
}

// queueNameFromMBean extracts the destinationName value from a Jolokia MBean path.
// Example: "org.apache.activemq:...,destinationName=myQueue" → "myQueue"
func queueNameFromMBean(mbean string) string {
	const prefix = "destinationName="
	for _, part := range splitMBean(mbean) {
		if len(part) > len(prefix) && part[:len(prefix)] == prefix {
			return part[len(prefix):]
		}
	}
	return mbean
}

func splitMBean(mbean string) []string {
	// MBean format: "domain:key=val,key=val,..."
	// Split on ',' after the first ':'.
	for i := 0; i < len(mbean); i++ {
		if mbean[i] == ':' {
			return splitComma(mbean[i+1:])
		}
	}
	return []string{mbean}
}

func splitComma(s string) []string {
	var parts []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	return parts
}

// BrowseMessages fetches the messages currently in queueName via the Jolokia
// browseMessages() exec operation.
func (c *Client) BrowseMessages(ctx context.Context, queueName string) ([]queue.Message, error) {
	mbean := fmt.Sprintf(
		"org.apache.activemq:type=Broker,brokerName=%s,destinationType=Queue,destinationName=%s",
		c.cfg.BrokerName, queueName,
	)
	reqBody := map[string]any{
		"type":      "exec",
		"mbean":     mbean,
		"operation": "browseMessages()",
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("jolokia browseMessages marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.URL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("jolokia browseMessages request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	c.setHeaders(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jolokia browseMessages: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jolokia browseMessages: HTTP %d", resp.StatusCode)
	}

	// Decode value as []map so any field can be a string, number, or object
	// (ActiveMQ returns messageId as a JMX CompositeData object, not a plain string).
	var result struct {
		Status int                      `json:"status"`
		Error  string                   `json:"error"`
		Value  []map[string]interface{} `json:"value"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("jolokia browseMessages decode: %w", err)
	}
	if result.Status != 200 {
		return nil, fmt.Errorf("jolokia browseMessages error (status %d): %s", result.Status, result.Error)
	}

	messages := make([]queue.Message, 0, len(result.Value))
	for _, m := range result.Value {
		id := extractMessageID(m["messageId"])

		var ts int64
		if f, ok := m["timestamp"].(float64); ok {
			ts = int64(f)
		}

		// JMS type: use the jMSType header if set, otherwise infer from body fields.
		jmsType, _ := m["jMSType"].(string)
		if jmsType == "" {
			textVal := m["text"]
			if textVal != nil && textVal != "" {
				jmsType = "text"
			} else if _, hasLen := m["bodyLength"].(float64); hasLen {
				jmsType = "bytes"
			} else {
				jmsType = "other"
			}
		}

		correlationID, _ := m["jMSCorrelationID"].(string)

		preview, _ := m["text"].(string)
		if preview == "" {
			preview = "(binary)"
		} else if len(preview) > 80 {
			preview = preview[:80]
		}

		messages = append(messages, queue.Message{
			ID:            id,
			JMSType:       jmsType,
			CorrelationID: correlationID,
			Timestamp:     time.UnixMilli(ts),
			Preview:       preview,
		})
	}
	return messages, nil
}

// extractMessageID converts whatever Jolokia returns for messageId into a
// display string. ActiveMQ returns a CompositeData object whose fields can be
// used to reconstruct the canonical "ID:..." string; if the field is already a
// plain string it is returned as-is.
func extractMessageID(raw interface{}) string {
	if raw == nil {
		return ""
	}
	// Plain string (some brokers / Jolokia versions return this directly).
	if s, ok := raw.(string); ok {
		return s
	}
	// CompositeData object: try to reconstruct the JMS message ID string from
	// the known ActiveMQ MessageId fields.
	m, ok := raw.(map[string]interface{})
	if !ok {
		return fmt.Sprintf("%v", raw)
	}
	// ActiveMQ MessageId toString: "ID:<connectionId>:<producerSequenceId>:<brokerSequenceId>"
	var connID string
	if prod, ok := m["producerId"].(map[string]interface{}); ok {
		if sess, ok := prod["producerSessionId"].(map[string]interface{}); ok {
			if conn, ok := sess["connectionId"].(map[string]interface{}); ok {
				connID, _ = conn["value"].(string)
			}
		}
	}
	prodSeq, _ := m["producerSequenceId"].(float64)
	brokerSeq, _ := m["brokerSequenceId"].(float64)
	if connID != "" {
		return fmt.Sprintf("ID:%s:%d:%d", connID, int64(prodSeq), int64(brokerSeq))
	}
	// Fallback: just show the producer sequence numbers.
	return fmt.Sprintf("ID:?:%d:%d", int64(prodSeq), int64(brokerSeq))
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Origin", c.origin)
	if c.cfg.Username != "" {
		req.SetBasicAuth(c.cfg.Username, c.cfg.Password)
	}
}
