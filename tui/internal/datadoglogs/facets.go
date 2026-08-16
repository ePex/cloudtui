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

// facetValueLimit caps how many distinct values a single facet-listing
// call returns, same convention as Search's page limit.
const facetValueLimit = 1000

type aggregateRequest struct {
	Filter struct {
		Query string `json:"query"`
		From  string `json:"from"`
		To    string `json:"to"`
	} `json:"filter"`
	Compute []aggregateCompute `json:"compute"`
	GroupBy []aggregateGroupBy `json:"group_by"`
}

type aggregateCompute struct {
	Aggregation string `json:"aggregation"`
}

// aggregateGroupBy's Sort is deliberately left unset: Datadog's sort
// defaults to alphabetical, which is fine here (the actual dropdown
// ordering comes from sortedKeys in internal/app, not this response) —
// found live that a sort-by-count request needs an explicit
// `"type":"measure"` alongside `"aggregation"`/`"order"`, or Datadog
// rejects `aggregation` outright ("Unrecognized parameter") since it
// only applies to type=measure, not the default alphabetical sort.
type aggregateGroupBy struct {
	Facet string `json:"facet"`
	Limit int    `json:"limit"`
}

type aggregateResponse struct {
	Data struct {
		Buckets []struct {
			By map[string]string `json:"by"`
		} `json:"buckets"`
	} `json:"data"`
}

// ListFacetValues asks Datadog's Logs Aggregate API for the distinct
// values of facet (e.g. "service", "env") seen across all logs within
// [from, to), sorted by count descending, capped at facetValueLimit —
// independent of whatever time range a search itself uses (see
// spec/52-fe-datadog-logs-facet-listing). Unlike Search, this queries
// everything ("*"), not a caller-supplied query, since the point is
// discovering every value that exists, not narrowing to a subset.
func ListFacetValues(ctx context.Context, cfg config.DatadogConfig, facet string, from, to time.Time) ([]string, error) {
	if cfg.AccessToken == "" {
		return nil, fmt.Errorf("Datadog access token not configured — set it in Settings or DD_ACCESS_TOKEN")
	}

	site := cfg.Site
	if site == "" {
		site = defaultSite
	}

	return listFacetValues(ctx, fmt.Sprintf("https://api.%s", site), cfg.AccessToken, facet, from, to)
}

// listFacetValues does the actual HTTP round-trip against baseURL + the
// aggregate path — split out from ListFacetValues for the same reason
// search is split out from Search (testing against an httptest.Server).
func listFacetValues(ctx context.Context, baseURL, accessToken, facet string, from, to time.Time) ([]string, error) {
	var reqBody aggregateRequest
	reqBody.Filter.Query = "*"
	reqBody.Filter.From = from.UTC().Format(time.RFC3339)
	reqBody.Filter.To = to.UTC().Format(time.RFC3339)
	reqBody.Compute = []aggregateCompute{{Aggregation: "count"}}
	reqBody.GroupBy = []aggregateGroupBy{{
		Facet: facet,
		Limit: facetValueLimit,
	}}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("encoding Datadog aggregate request: %w", err)
	}

	url := baseURL + "/api/v2/logs/analytics/aggregate"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("building Datadog aggregate request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: requestTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("listing Datadog facet values: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading Datadog aggregate response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("listing Datadog facet values: status %d: %s", resp.StatusCode, truncate(respBody, 500))
	}

	var parsed aggregateResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("parsing Datadog aggregate response: %w", err)
	}

	values := make([]string, 0, len(parsed.Data.Buckets))
	for _, b := range parsed.Data.Buckets {
		if v := b.By[facet]; v != "" {
			values = append(values, v)
		}
	}
	return values, nil
}
