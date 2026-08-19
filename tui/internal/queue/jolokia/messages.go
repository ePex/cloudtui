package jolokia

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/ePex/cloudtui/tui/internal/queue"
)

// previewMaxLen caps queue.Message.Preview's length. The full body is
// already in memory by the time this is applied — this is a display/
// memory bound, not a network optimization — so raising it costs
// nothing per request; it only needs to be generous enough that the
// message browser's wrap toggle (tui/internal/view, previewWrapWidth /
// maxWrapLines) rarely hits it before its own line cap does. Found via
// user feedback (CR 92) that 80 was too aggressive even before wrap
// existed to reveal more.
const previewMaxLen = 2000

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
		Status int              `json:"status"`
		Error  string           `json:"error"`
		Value  []map[string]any `json:"value"`
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
		} else if len(preview) > previewMaxLen {
			preview = preview[:previewMaxLen]
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
		if len(preview) > previewMaxLen {
			preview = preview[:previewMaxLen]
		}
		return queue.Message{
			JMSType:   "text",
			Preview:   preview,
			RawFields: map[string]any{"text": s},
		}
	}

	// Full message object (ActiveMQ 5.18+ browse() format).
	var obj map[string]any
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
	} else if len(preview) > previewMaxLen {
		preview = preview[:previewMaxLen]
	}

	// Merge all typed property maps into one map for the detail view.
	props := map[string]any{}
	for _, key := range []string{
		"StringProperties", "IntProperties", "LongProperties",
		"ByteProperties", "ShortProperties", "FloatProperties",
		"DoubleProperties", "BooleanProperties",
	} {
		if m, ok := obj[key].(map[string]any); ok {
			for k, v := range m {
				props[k] = v
			}
		}
	}

	// Build RawFields with the key names message_detail.go expects.
	rawFields := map[string]any{
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
	var obj map[string]any
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
func extractMessageID(raw any) string {
	if raw == nil {
		return ""
	}
	// Plain string (some brokers / Jolokia versions return this directly).
	if s, ok := raw.(string); ok {
		return s
	}
	// CompositeData object.
	m, ok := raw.(map[string]any)
	if !ok {
		return fmt.Sprintf("%v", raw)
	}
	var connID string
	var sessionID, producerVal int64
	if prod, ok := m["producerId"].(map[string]any); ok {
		// connectionId may be serialized as a nested {"value":"..."} map or
		// as a plain string (Jolokia simplifies single-field CompositeData).
		switch v := prod["connectionId"].(type) {
		case string:
			connID = v
		case map[string]any:
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
