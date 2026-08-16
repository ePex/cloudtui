package datadoglogs

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ePex/cloudtui/tui/internal/config"
)

func TestListFacetValuesEmptyAccessTokenErrorsWithoutRequest(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer srv.Close()

	_, err := ListFacetValues(context.Background(), config.DatadogConfig{Site: strings.TrimPrefix(srv.URL, "http://"), AccessToken: ""}, "service", time.Now(), time.Now())
	if err == nil {
		t.Fatal("ListFacetValues() error = nil, want non-nil for empty access token")
	}
	if called {
		t.Error("ListFacetValues() made an HTTP request despite an empty access token")
	}
}

func TestListFacetValuesSendsExpectedRequestAndParsesResponse(t *testing.T) {
	var gotAuth, gotContentType, gotMethod, gotPath string
	var gotBody aggregateRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotContentType = r.Header.Get("Content-Type")
		gotMethod = r.Method
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decoding request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":{"buckets":[
			{"by":{"service":"bar-proxy"}},
			{"by":{"service":"foo-worker"}}
		]}}`))
	}))
	defer srv.Close()

	from := time.Date(2026, 7, 17, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC)

	got, err := listFacetValues(context.Background(), srv.URL, "tok-123", "service", from, to)
	if err != nil {
		t.Fatalf("listFacetValues() error = %v", err)
	}
	if len(got) != 2 || got[0] != "bar-proxy" || got[1] != "foo-worker" {
		t.Errorf("listFacetValues() = %v, want [bar-proxy foo-worker]", got)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/api/v2/logs/analytics/aggregate" {
		t.Errorf("path = %q, want /api/v2/logs/analytics/aggregate", gotPath)
	}
	if gotAuth != "Bearer tok-123" {
		t.Errorf("Authorization header = %q, want %q", gotAuth, "Bearer tok-123")
	}
	if gotContentType != "application/json" {
		t.Errorf("Content-Type header = %q, want application/json", gotContentType)
	}
	if gotBody.Filter.Query != "*" {
		t.Errorf("filter.query = %q, want %q", gotBody.Filter.Query, "*")
	}
	if gotBody.Filter.From != "2026-07-17T00:00:00Z" {
		t.Errorf("filter.from = %q, want %q", gotBody.Filter.From, "2026-07-17T00:00:00Z")
	}
	if gotBody.Filter.To != "2026-08-16T00:00:00Z" {
		t.Errorf("filter.to = %q, want %q", gotBody.Filter.To, "2026-08-16T00:00:00Z")
	}
	if len(gotBody.Compute) != 1 || gotBody.Compute[0].Aggregation != "count" {
		t.Errorf("compute = %+v, want a single count aggregation", gotBody.Compute)
	}
	if len(gotBody.GroupBy) != 1 {
		t.Fatalf("group_by = %+v, want exactly one entry", gotBody.GroupBy)
	}
	gb := gotBody.GroupBy[0]
	if gb.Facet != "service" {
		t.Errorf("group_by.facet = %q, want %q", gb.Facet, "service")
	}
	if gb.Limit != facetValueLimit {
		t.Errorf("group_by.limit = %d, want %d", gb.Limit, facetValueLimit)
	}
}

func TestListFacetValuesSkipsBucketsWithEmptyValue(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":{"buckets":[
			{"by":{"service":"bar-proxy"}},
			{"by":{"service":""}},
			{"by":{}}
		]}}`))
	}))
	defer srv.Close()

	got, err := listFacetValues(context.Background(), srv.URL, "tok", "service", time.Now(), time.Now())
	if err != nil {
		t.Fatalf("listFacetValues() error = %v", err)
	}
	if len(got) != 1 || got[0] != "bar-proxy" {
		t.Errorf("listFacetValues() = %v, want [bar-proxy] (empty/missing values skipped)", got)
	}
}

func TestListFacetValuesNonOKStatusReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"errors":["Forbidden"]}`))
	}))
	defer srv.Close()

	_, err := listFacetValues(context.Background(), srv.URL, "bad-token", "service", time.Now(), time.Now())
	if err == nil {
		t.Fatal("listFacetValues() error = nil, want non-nil for a 401 response")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("listFacetValues() error = %v, want it to mention status 401", err)
	}
}
