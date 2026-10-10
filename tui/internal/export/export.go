// Package export writes queue messages to NDJSON files (spec-wip
// fe-message-export): one JSON object per message, the same format the AMQ
// web console exports. No UI dependency.
package export

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ePex/cloudtui/tui/internal/queue"
)

// MaxMessages caps one export, so a broker configured with a large browse
// limit can't make the TUI hold an unbounded export in memory.
const MaxMessages = 10000

// Record is one exported message — one NDJSON line.
type Record struct {
	Queue     string `json:"queue"`
	MessageID string `json:"messageId"`
	// JMSType is the real JMSType header only; an inferred type
	// ("text"/"bytes"/"other", queue.Message.JMSTypeInferred) is omitted.
	JMSType string `json:"jmsType,omitempty"`
	// CorrelationID is omitted when unset — always, via the proxy
	// backend, whose list-messages doesn't return it (spec/11).
	CorrelationID string `json:"correlationId,omitempty"`
	// Timestamp is RFC 3339 in UTC, omitted when unknown (Jolokia's
	// limited-info browse fallback, spec/08).
	Timestamp string `json:"timestamp,omitempty"`
	// Headers holds the JMS headers that are set (keyed by the detail
	// view's labels, values as the backend reported them) and every
	// property (decoded). Always an object, never null.
	Headers map[string]any `json:"headers"`
	// Body is the full text body, null for a message without one.
	Body *string `json:"body"`
}

// NewRecord builds the export record for m from queueName.
func NewRecord(queueName string, m queue.Message) Record {
	r := Record{
		Queue:         queueName,
		MessageID:     m.ID,
		CorrelationID: m.CorrelationID,
		Headers:       map[string]any{},
	}
	if !m.JMSTypeInferred {
		r.JMSType = m.JMSType
	}
	if !m.Timestamp.IsZero() {
		r.Timestamp = m.Timestamp.UTC().Format("2006-01-02T15:04:05.000Z07:00")
	}
	for _, f := range queue.HeaderFields() {
		raw, ok := m.RawFields[f.Key]
		if !ok || raw == nil || f.Label == "JMSCorrelationID" { // correlationId has its own field
			continue
		}
		if props, isProps := raw.(map[string]any); isProps && f.Key == "properties" {
			for k, v := range props {
				r.Headers[k] = queue.DecodePropertyValue(v)
			}
			continue
		}
		r.Headers[f.Label] = raw
	}
	if body, ok := m.Body(); ok {
		r.Body = &body
	}
	return r
}

// WriteFile writes msgs from queueName to path as NDJSON. It writes to a
// temporary file in the same directory and renames it into place, so a
// failed write never leaves a half-written file — nor damages a file it
// was going to replace. The file is created readable by its owner only:
// exports can hold message contents worth protecting.
func WriteFile(path, queueName string, msgs []queue.Message) (err error) {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".cloudtui-export-*")
	if err != nil {
		return fmt.Errorf("export: create temp file: %w", err)
	}
	defer func() {
		if err != nil {
			tmp.Close()
			os.Remove(tmp.Name())
		}
	}()

	w := bufio.NewWriter(tmp)
	enc := json.NewEncoder(w) // Encode ends every record with "\n"
	enc.SetEscapeHTML(false)  // keep bodies readable: < > & stay as they are
	for _, m := range msgs {
		if err = enc.Encode(NewRecord(queueName, m)); err != nil {
			return fmt.Errorf("export: write %s: %w", path, err)
		}
	}
	if err = w.Flush(); err != nil {
		return fmt.Errorf("export: write %s: %w", path, err)
	}
	if err = tmp.Close(); err != nil {
		return fmt.Errorf("export: write %s: %w", path, err)
	}
	if err = os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("export: move into place %s: %w", path, err)
	}
	return nil
}

// DefaultFileName is "<queue>-<UTC timestamp>.ndjson", e.g.
// "orders-20261010T153000Z.ndjson". Characters invalid in file names on
// any OS are replaced with "_".
func DefaultFileName(queueName string, now time.Time) string {
	safe := strings.NewReplacer(
		"/", "_", `\`, "_", ":", "_", "*", "_", "?", "_", `"`, "_", "<", "_", ">", "_", "|", "_",
	).Replace(queueName)
	return safe + "-" + now.UTC().Format("20060102T150405Z") + ".ndjson"
}

// DefaultDir is the user's Downloads directory when it exists, otherwise
// their home directory ("." if even that can't be determined).
func DefaultDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	downloads := filepath.Join(home, "Downloads")
	if info, err := os.Stat(downloads); err == nil && info.IsDir() {
		return downloads
	}
	return home
}
