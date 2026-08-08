// Package devtool provides dev-only helpers for live-testing against a real
// broker: creating/removing disposable queues via JMX, and starting/
// stopping a local mq-proxy instance to test the proxy backend. It is used
// by cmd/devtool, not by the TUI application itself.
package devtool

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/ePex/cloudtui/tui/internal/config"
)

// AddQueue creates queueName on the broker via the JMX Broker MBean's
// addQueue operation, for setting up disposable test queues (ActiveMQ's
// sendTextMessage requires the destination MBean to already exist).
func AddQueue(ctx context.Context, cfg config.QueueConfig, queueName string) error {
	return execBrokerOp(ctx, cfg, "addQueue(java.lang.String)", queueName)
}

// RemoveQueue deletes queueName from the broker via JMX. Any messages still
// in the queue are discarded.
func RemoveQueue(ctx context.Context, cfg config.QueueConfig, queueName string) error {
	return execBrokerOp(ctx, cfg, "removeQueue(java.lang.String)", queueName)
}

func execBrokerOp(ctx context.Context, cfg config.QueueConfig, operation, arg string) error {
	mbean := fmt.Sprintf("org.apache.activemq:type=Broker,brokerName=%s", cfg.BrokerName)
	reqBody := map[string]any{
		"type":      "exec",
		"mbean":     mbean,
		"operation": operation,
		"arguments": []any{arg},
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("encode request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.URL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost") // required by this broker's Jolokia CORS check
	if cfg.Username != "" {
		req.SetBasicAuth(cfg.Username, cfg.Password)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("%s: %w", operation, err)
	}
	defer resp.Body.Close()
	var result struct {
		Status int    `json:"status"`
		Error  string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	if result.Status != 200 {
		return fmt.Errorf("%s failed (status %d): %s", operation, result.Status, result.Error)
	}
	return nil
}
