package datadoglogs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ePex/cloudtui/tui/internal/config"
)

const defaultSite = "datadoghq.com"

type searchRequest struct {
	Filter struct {
		Query string `json:"query"`
		From  string `json:"from"`
		To    string `json:"to"`
	} `json:"filter"`
	Sort string `json:"sort"`
	Page struct {
		Limit int `json:"limit"`
	} `json:"page"`
}

type searchResponse struct {
	Data []struct {
		ID         string `json:"id"`
		Attributes struct {
			Timestamp time.Time `json:"timestamp"`
			Message   string    `json:"message"`
			Status    string    `json:"status"`
			Service   string    `json:"service"`
			Host      string    `json:"host"`
			Tags      []string  `json:"tags"`
		} `json:"attributes"`
	} `json:"data"`
	Meta struct {
		Page struct {
			After string `json:"after"`
		} `json:"page"`
	} `json:"meta"`
}

// Search runs a single query against Datadog's Logs Search API over the
// time window [from, to), newest first. Returns at most one page of
// results (limit 1000); hasMore reports whether Datadog indicated more
// results exist (a pagination cursor is present) — this function never
// auto-paginates, matching internal/awslogs.FilterEvents's same
// "surface, don't auto-fetch" behavior.
func Search(ctx context.Context, cfg config.DatadogConfig, query string, from, to time.Time) ([]LogEvent, bool, error) {
	if cfg.AccessToken == "" {
		return nil, false, fmt.Errorf("Datadog access token not configured — set it in Settings or DD_ACCESS_TOKEN")
	}

	site := cfg.Site
	if site == "" {
		site = defaultSite
	}

	return search(ctx, fmt.Sprintf("https://api.%s", site), cfg.AccessToken, query, from, to)
}

// search does the actual HTTP round-trip against baseURL + the search
// path. Split out from Search so tests can point baseURL at an
// httptest.Server (plain HTTP, arbitrary host:port) without Search's
// hardcoded "https://api.<site>" construction getting in the way.
func search(ctx context.Context, baseURL, accessToken, query string, from, to time.Time) ([]LogEvent, bool, error) {
	var reqBody searchRequest
	reqBody.Filter.Query = query
	reqBody.Filter.From = from.UTC().Format(time.RFC3339)
	reqBody.Filter.To = to.UTC().Format(time.RFC3339)
	reqBody.Sort = "-timestamp"
	reqBody.Page.Limit = 1000

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, false, fmt.Errorf("encoding Datadog search request: %w", err)
	}

	url := baseURL + "/api/v2/logs/events/search"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, false, fmt.Errorf("building Datadog search request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: requestTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("searching Datadog logs: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false, fmt.Errorf("reading Datadog search response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, false, fmt.Errorf("searching Datadog logs: status %d: %s", resp.StatusCode, truncate(respBody, 500))
	}

	var parsed searchResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, false, fmt.Errorf("parsing Datadog search response: %w", err)
	}

	return buildLogEvents(parsed), parsed.Meta.Page.After != "", nil
}

// buildLogEvents converts a parsed searchResponse into LogEvents.
// Preserves Datadog's own order (already newest-first via sort:
// -timestamp in Search, so no re-sort needed here). Split out so this —
// the part with actual logic to get wrong — is unit-testable without a
// real HTTP call.
func buildLogEvents(resp searchResponse) []LogEvent {
	out := make([]LogEvent, 0, len(resp.Data))
	for _, d := range resp.Data {
		out = append(out, LogEvent{
			Timestamp: d.Attributes.Timestamp,
			Service:   d.Attributes.Service,
			Status:    d.Attributes.Status,
			Host:      d.Attributes.Host,
			Message:   d.Attributes.Message,
			Tags:      d.Attributes.Tags,
		})
	}
	return out
}

// truncate caps b at n bytes for inclusion in an error message, so a
// large HTML error page from a misconfigured host doesn't blow up log
// lines.
func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}
