package queue

import "fmt"

// HeaderField maps a JMS header's display label to the key the backends
// store it under in Message.RawFields (Jolokia's browseMessages() names;
// the proxy backend fills "text" and "properties" only). The special key
// "properties" holds the message's property map.
type HeaderField struct {
	Label string
	Key   string
}

// HeaderFields lists the headers the message detail view and the message
// export show, in display order.
func HeaderFields() []HeaderField {
	return []HeaderField{
		{"JMSCorrelationID", "jMSCorrelationID"},
		{"JMSDeliveryMode", "jMSDeliveryMode"},
		{"JMSDestination", "jMSDestination"},
		{"JMSExpiration", "jMSExpiration"},
		{"JMSRedelivered", "jMSRedelivered"},
		{"JMSReplyTo", "jMSReplyTo"},
		{"JMSXGroupID", "groupID"},
		{"JMSXGroupSeq", "groupSequence"},
		{"JMSXUserID", "userID"},
		{"Priority", "jMSPriority"},
		{"PropertiesText", "properties"},
	}
}

// Body returns m's full text body (not the trimmed Preview), and false when
// it has none — e.g. a binary message.
func (m Message) Body() (string, bool) {
	body, _ := m.RawFields["text"].(string)
	return body, body != ""
}

// DecodePropertyValue converts a Jolokia property value to a human-readable
// string. ActiveMQ serialises string properties as ByteSequence objects:
// a map with a "data" key holding a []any of float64 byte values.
// Those are decoded to a UTF-8 string. All other values fall back to %v.
func DecodePropertyValue(v any) string {
	m, ok := v.(map[string]any)
	if !ok {
		return fmt.Sprintf("%v", v)
	}
	dataRaw, hasData := m["data"]
	if !hasData {
		return fmt.Sprintf("%v", v)
	}
	items, ok := dataRaw.([]any)
	if !ok {
		return fmt.Sprintf("%v", v)
	}
	bs := make([]byte, len(items))
	for i, item := range items {
		f, ok := item.(float64)
		if !ok {
			return fmt.Sprintf("%v", v)
		}
		bs[i] = byte(f)
	}
	return string(bs)
}
