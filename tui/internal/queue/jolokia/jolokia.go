package jolokia

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

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

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Origin", c.origin)
	if c.cfg.Username != "" {
		req.SetBasicAuth(c.cfg.Username, c.cfg.Password)
	}
}
