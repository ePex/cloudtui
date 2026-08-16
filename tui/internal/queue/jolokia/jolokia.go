package jolokia

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
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
	Status int    `json:"status"`
	Value  int64  `json:"value"`
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
		queueSize := results[i*4]
		consumerCount := results[i*4+1]
		enqueueCount := results[i*4+2]
		dequeueCount := results[i*4+3]

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

// BrowseMessages fetches messages from queueName. It tries browseMessages()
// first (full CompositeData including IDs and timestamps). If that fails (e.g.
// due to JMX-originated messages whose internal connection ID cannot be
// serialized), it falls back to browse() which returns plain body strings.
// Fallback messages have empty IDs — individual move/delete will not work for
// them. If both operations fail, the original browseMessages error is returned.
//
// filter is applied client-side (via filterMessages, filter.go) after the
// full/fallback browse completes — JMX has no selector-based browse, unlike
// the proxy backend, which pushes the filter down to mq-proxy's list-messages
// endpoint.
func (c *Client) BrowseMessages(ctx context.Context, queueName string, filter queue.MessageFilter) ([]queue.Message, error) {
	msgs, err := c.browseMessagesFull(ctx, queueName)
	if err != nil {
		slog.Debug("browseMessages failed, trying browse() fallback", "queue", queueName, "err", err)
		fallback, fallbackErr := c.browseMessagesFallback(ctx, queueName)
		if fallbackErr != nil {
			slog.Debug("browse() fallback also failed", "queue", queueName, "err", fallbackErr)
			return nil, err
		}
		return filterMessages(fallback, filter), nil
	}
	return filterMessages(msgs, filter), nil
}

// browseMessagesFull calls the browseMessages() Jolokia exec operation and
// returns fully-populated queue.Message values including IDs and timestamps.
// For messages that carry no "text" field (e.g. BytesMessages created by the
// STOMP adapter), a secondary browse() call backfills the body text so that
// the preview and detail view always show the message content.
func (c *Client) browseMessagesFull(ctx context.Context, queueName string) ([]queue.Message, error) {
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

	// Backfill body text for messages that have no "text" field (BytesMessages
	// stored by the STOMP adapter). browse() returns bodies as strings — for
	// BytesMessages it decodes the bytes as UTF-8, giving us the original text.
	needsBackfill := false
	for _, m := range result.Value {
		if t, _ := m["text"].(string); t == "" {
			needsBackfill = true
			break
		}
	}
	if needsBackfill {
		if bodies, err := c.browseBodies(ctx, queueName); err == nil && len(bodies) == len(result.Value) {
			for i, bodyText := range bodies {
				if t, _ := result.Value[i]["text"].(string); t == "" && bodyText != "" {
					result.Value[i]["text"] = bodyText
				}
			}
		}
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
			RawFields:     m,
		})
	}
	return messages, nil
}

// browseBodies calls the browse() Jolokia exec operation and returns message
// bodies as strings. For TextMessages this is the text content; for
// BytesMessages ActiveMQ decodes the bytes as UTF-8 and returns the result.
func (c *Client) browseBodies(ctx context.Context, queueName string) ([]string, error) {
	mbean := fmt.Sprintf(
		"org.apache.activemq:type=Broker,brokerName=%s,destinationType=Queue,destinationName=%s",
		c.cfg.BrokerName, queueName,
	)
	reqBody := map[string]any{
		"type":      "exec",
		"mbean":     mbean,
		"operation": "browse()",
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("jolokia browse marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.URL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("jolokia browse request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	c.setHeaders(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jolokia browse: %w", err)
	}
	defer resp.Body.Close()
	var result struct {
		Status int               `json:"status"`
		Error  string            `json:"error"`
		Value  []json.RawMessage `json:"value"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("jolokia browse decode: %w", err)
	}
	if result.Status != 200 {
		return nil, fmt.Errorf("jolokia browse error (status %d): %s", result.Status, result.Error)
	}
	bodies := make([]string, 0, len(result.Value))
	for _, raw := range result.Value {
		bodies = append(bodies, extractBrowseBody(raw))
	}
	return bodies, nil
}

// browseMessagesFallback calls browse() and parses the full message objects
// that ActiveMQ 5.18+ returns. Each object contains JMSMessageID, Text, and
// all JMS headers — so the fallback produces fully-populated messages
// including IDs (enabling delete/move) and headers (visible in detail view).
func (c *Client) browseMessagesFallback(ctx context.Context, queueName string) ([]queue.Message, error) {
	mbean := fmt.Sprintf(
		"org.apache.activemq:type=Broker,brokerName=%s,destinationType=Queue,destinationName=%s",
		c.cfg.BrokerName, queueName,
	)
	reqBody := map[string]any{
		"type":      "exec",
		"mbean":     mbean,
		"operation": "browse()",
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("jolokia browse marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.URL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("jolokia browse request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	c.setHeaders(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jolokia browse: %w", err)
	}
	defer resp.Body.Close()
	var result struct {
		Status int               `json:"status"`
		Error  string            `json:"error"`
		Value  []json.RawMessage `json:"value"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("jolokia browse decode: %w", err)
	}
	if result.Status != 200 {
		return nil, fmt.Errorf("jolokia browse error (status %d): %s", result.Status, result.Error)
	}
	messages := make([]queue.Message, 0, len(result.Value))
	for _, raw := range result.Value {
		messages = append(messages, parseBrowseItem(raw))
	}
	return messages, nil
}

// parseBrowseItem converts one element from browse()'s value array into a
// queue.Message. ActiveMQ 5.18+ returns full message objects (JMSMessageID,
// Text, all headers); older versions return plain body strings.
func parseBrowseItem(raw json.RawMessage) queue.Message {
	// Try plain string (legacy browse behavior — body only).
	var s string
	if json.Unmarshal(raw, &s) == nil {
		preview := s
		if len(preview) > 80 {
			preview = preview[:80]
		}
		return queue.Message{
			JMSType:   "text",
			Preview:   preview,
			RawFields: map[string]interface{}{"text": s},
		}
	}

	// Full message object (ActiveMQ 5.18+ browse() format).
	var obj map[string]interface{}
	if json.Unmarshal(raw, &obj) != nil {
		return queue.Message{Preview: "(binary)"}
	}

	id, _ := obj["JMSMessageID"].(string)
	bodyText, _ := obj["Text"].(string)
	jmsType, _ := obj["JMSType"].(string)
	correlationID, _ := obj["JMSCorrelationID"].(string)

	if jmsType == "" {
		if bodyText != "" {
			jmsType = "text"
		} else {
			jmsType = "other"
		}
	}

	var ts time.Time
	if tsStr, ok := obj["JMSTimestamp"].(string); ok && tsStr != "" {
		ts, _ = time.Parse(time.RFC3339, tsStr)
	}

	preview := bodyText
	if preview == "" {
		preview = "(binary)"
	} else if len(preview) > 80 {
		preview = preview[:80]
	}

	// Merge all typed property maps into one map for the detail view.
	props := map[string]interface{}{}
	for _, key := range []string{
		"StringProperties", "IntProperties", "LongProperties",
		"ByteProperties", "ShortProperties", "FloatProperties",
		"DoubleProperties", "BooleanProperties",
	} {
		if m, ok := obj[key].(map[string]interface{}); ok {
			for k, v := range m {
				props[k] = v
			}
		}
	}

	// Build RawFields with the key names message_detail.go expects.
	rawFields := map[string]interface{}{
		"text":             bodyText,
		"jMSCorrelationID": obj["JMSCorrelationID"],
		"jMSDeliveryMode":  obj["JMSDeliveryMode"],
		"jMSDestination":   obj["JMSDestination"],
		"jMSExpiration":    obj["JMSExpiration"],
		"jMSRedelivered":   obj["JMSRedelivered"],
		"jMSReplyTo":       obj["JMSReplyTo"],
		"jMSPriority":      obj["JMSPriority"],
		"groupID":          obj["JMSXGroupID"],
		"groupSequence":    obj["JMSXGroupSeq"],
		"userID":           obj["JMSXUserID"],
		"properties":       props,
	}

	return queue.Message{
		ID:            id,
		JMSType:       jmsType,
		CorrelationID: correlationID,
		Timestamp:     ts,
		Preview:       preview,
		RawFields:     rawFields,
	}
}

// extractBrowseBody returns the body text from a single browse() value element.
// Used by browseBodies for the body-backfill path in browseMessagesFull.
func extractBrowseBody(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var obj map[string]interface{}
	if json.Unmarshal(raw, &obj) == nil {
		// ActiveMQ 5.18+ returns "Text" (capital T) for the message body.
		for _, key := range []string{"Text", "text"} {
			if v, ok := obj[key].(string); ok {
				return v
			}
		}
	}
	return ""
}

// extractMessageID converts whatever Jolokia returns for messageId into a
// string matching ActiveMQ's MessageId.toString() format, which is what
// removeMessage / moveMessageTo use to locate the message.
//
// The canonical JMS message ID format (from ActiveMQ's MessageId.toString()):
//
//	connectionId.value + ":" + sessionId + ":" + producerId.value + ":" + producerSequenceId
//
// Note: brokerSequenceId is an internal broker field and is NOT part of the
// JMS message ID string, even though it appears in the CompositeData object.
//
// connectionId.value is already the full "ID:<host>-<port>-<ts>-<n>" string.
// The Jolokia CompositeData structure therefore is:
//
//	messageId.producerId.connectionId.value  (string, includes "ID:" prefix)
//	messageId.producerId.sessionId           (float64)
//	messageId.producerId.value               (float64)
//	messageId.producerSequenceId             (float64)
func extractMessageID(raw interface{}) string {
	if raw == nil {
		return ""
	}
	// Plain string (some brokers / Jolokia versions return this directly).
	if s, ok := raw.(string); ok {
		return s
	}
	// CompositeData object.
	m, ok := raw.(map[string]interface{})
	if !ok {
		return fmt.Sprintf("%v", raw)
	}
	var connID string
	var sessionID, producerVal int64
	if prod, ok := m["producerId"].(map[string]interface{}); ok {
		// connectionId may be serialized as a nested {"value":"..."} map or
		// as a plain string (Jolokia simplifies single-field CompositeData).
		switch v := prod["connectionId"].(type) {
		case string:
			connID = v
		case map[string]interface{}:
			connID, _ = v["value"].(string)
		}
		if s, ok := prod["sessionId"].(float64); ok {
			sessionID = int64(s)
		}
		if v, ok := prod["value"].(float64); ok {
			producerVal = int64(v)
		}
	}
	prodSeq, _ := m["producerSequenceId"].(float64)
	if connID != "" {
		// connID already carries the "ID:" prefix.
		// Format matches ActiveMQ's MessageId.toString():
		//   connectionId.value + ":" + sessionId + ":" + producerId.value + ":" + producerSequenceId
		// Note: brokerSequenceId is an internal broker field and is NOT part of the JMS message ID.
		return fmt.Sprintf("%s:%d:%d:%d", connID, sessionID, producerVal, int64(prodSeq))
	}
	return fmt.Sprintf("ID:?:%d:%d", sessionID, int64(prodSeq))
}

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
		Status int         `json:"status"`
		Error  string      `json:"error"`
		Value  interface{} `json:"value"`
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
		Status int         `json:"status"`
		Error  string      `json:"error"`
		Value  interface{} `json:"value"`
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
		Status int         `json:"status"`
		Error  string      `json:"error"`
		Value  interface{} `json:"value"`
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

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Origin", c.origin)
	if c.cfg.Username != "" {
		req.SetBasicAuth(c.cfg.Username, c.cfg.Password)
	}
}
