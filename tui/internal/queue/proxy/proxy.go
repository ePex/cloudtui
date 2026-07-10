// Package proxy implements queue.Backend by talking to mq-proxy's REST
// API — the real, network-backed implementation, used both locally
// (against the embedded broker) and in AWS, per the project's
// architecture.
package proxy

import (
	"context"
	"fmt"
	"net/http"

	mqproxyclient "github.com/ePex/cloudtui/tui/internal/mqproxyclient/generated"
	"github.com/ePex/cloudtui/tui/internal/queue"
)

// Backend is a queue.Backend backed by an HTTP call to mq-proxy.
type Backend struct {
	client *mqproxyclient.ClientWithResponses
}

var _ queue.Backend = (*Backend)(nil)

// New builds a Backend for the mq-proxy at baseURL, authenticating every
// request with HTTP Basic Auth.
func New(baseURL, username, password string) (*Backend, error) {
	client, err := mqproxyclient.NewClientWithResponses(baseURL, mqproxyclient.WithRequestEditorFn(basicAuthEditor(username, password)))
	if err != nil {
		return nil, fmt.Errorf("creating mq-proxy client: %w", err)
	}
	return &Backend{client: client}, nil
}

func basicAuthEditor(username, password string) mqproxyclient.RequestEditorFn {
	return func(_ context.Context, req *http.Request) error {
		req.SetBasicAuth(username, password)
		return nil
	}
}

func (b *Backend) List(ctx context.Context) ([]queue.Summary, error) {
	resp, err := b.client.ListQueuesWithResponse(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing queues: %w", err)
	}
	if resp.JSON200 == nil {
		return nil, apiError("listing queues", resp.StatusCode(), resp.Body, resp.JSON401, resp.JSONDefault)
	}
	summaries := make([]queue.Summary, 0, len(*resp.JSON200))
	for _, s := range *resp.JSON200 {
		summaries = append(summaries, queue.Summary{
			Name:          s.Name,
			PendingCount:  s.PendingCount,
			ConsumerCount: s.ConsumerCount,
		})
	}
	return summaries, nil
}

func (b *Backend) Browse(ctx context.Context, queueName string, limit int) ([]queue.Message, error) {
	limit32 := int32(limit)
	resp, err := b.client.BrowseMessagesWithResponse(ctx, queueName, &mqproxyclient.BrowseMessagesParams{Limit: &limit32})
	if err != nil {
		return nil, fmt.Errorf("browsing queue %s: %w", queueName, err)
	}
	if resp.JSON200 == nil {
		return nil, apiError("browsing queue "+queueName, resp.StatusCode(), resp.Body, resp.JSON401, resp.JSON404, resp.JSONDefault)
	}
	messages := make([]queue.Message, 0, len(*resp.JSON200))
	for _, m := range *resp.JSON200 {
		var properties map[string]string
		if m.Properties != nil {
			properties = *m.Properties
		}
		messages = append(messages, queue.Message{ID: m.Id, Body: m.Body, Properties: properties})
	}
	return messages, nil
}

func (b *Backend) Send(ctx context.Context, queueName, body string, properties map[string]string) error {
	req := mqproxyclient.SendMessageJSONRequestBody{Body: body}
	if len(properties) > 0 {
		req.Properties = &properties
	}
	resp, err := b.client.SendMessageWithResponse(ctx, queueName, req)
	if err != nil {
		return fmt.Errorf("sending to queue %s: %w", queueName, err)
	}
	if resp.StatusCode() != http.StatusCreated {
		return apiError("sending to queue "+queueName, resp.StatusCode(), resp.Body, resp.JSON401, resp.JSON404, resp.JSONDefault)
	}
	return nil
}

func (b *Backend) Purge(ctx context.Context, queueName string) error {
	resp, err := b.client.PurgeQueueWithResponse(ctx, queueName)
	if err != nil {
		return fmt.Errorf("purging queue %s: %w", queueName, err)
	}
	if resp.StatusCode() != http.StatusNoContent {
		return apiError("purging queue "+queueName, resp.StatusCode(), resp.Body, resp.JSON401, resp.JSON404, resp.JSONDefault)
	}
	return nil
}

func (b *Backend) Move(ctx context.Context, sourceQueueName, targetQueueName string, maxMessages *int) error {
	req := mqproxyclient.MoveMessagesJSONRequestBody{TargetQueue: targetQueueName}
	if maxMessages != nil {
		v := int32(*maxMessages)
		req.MaxMessages = &v
	}
	resp, err := b.client.MoveMessagesWithResponse(ctx, sourceQueueName, req)
	if err != nil {
		return fmt.Errorf("moving messages from %s to %s: %w", sourceQueueName, targetQueueName, err)
	}
	if resp.StatusCode() != http.StatusNoContent {
		return apiError("moving messages from "+sourceQueueName, resp.StatusCode(), resp.Body, resp.JSON401, resp.JSON404, resp.JSONDefault)
	}
	return nil
}

// apiError builds an error from an unsuccessful mq-proxy response, using
// the first populated typed error body if any, else the raw response body.
func apiError(op string, statusCode int, body []byte, candidates ...*mqproxyclient.ErrorResponse) error {
	msg := ""
	for _, c := range candidates {
		if c != nil {
			msg = c.Message
			break
		}
	}
	if msg == "" {
		msg = string(body)
	}
	return fmt.Errorf("%s: mq-proxy returned %d: %s", op, statusCode, msg)
}
