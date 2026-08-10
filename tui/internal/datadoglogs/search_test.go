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

func newTestLogEvent(ts time.Time) searchResponse {
	var resp searchResponse
	resp.Data = make([]struct {
		ID         string `json:"id"`
		Attributes struct {
			Timestamp time.Time `json:"timestamp"`
			Message   string    `json:"message"`
			Status    string    `json:"status"`
			Service   string    `json:"service"`
			Host      string    `json:"host"`
			Tags      []string  `json:"tags"`
		} `json:"attributes"`
	}, 1)
	resp.Data[0].ID = "abc123"
	resp.Data[0].Attributes.Timestamp = ts
	resp.Data[0].Attributes.Message = "something happened"
	resp.Data[0].Attributes.Status = "error"
	resp.Data[0].Attributes.Service = "fibuproxy"
	resp.Data[0].Attributes.Host = "host-1"
	resp.Data[0].Attributes.Tags = []string{"env:testt"}
	return resp
}

func TestBuildLogEventsPopulatesFields(t *testing.T) {
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	got := buildLogEvents(newTestLogEvent(ts))

	if len(got) != 1 {
		t.Fatalf("buildLogEvents() len = %d, want 1", len(got))
	}
	e := got[0]
	if !e.Timestamp.Equal(ts) {
		t.Errorf("Timestamp = %v, want %v", e.Timestamp, ts)
	}
	if e.Message != "something happened" {
		t.Errorf("Message = %q, want %q", e.Message, "something happened")
	}
	if e.Status != "error" {
		t.Errorf("Status = %q, want %q", e.Status, "error")
	}
	if e.Service != "fibuproxy" {
		t.Errorf("Service = %q, want %q", e.Service, "fibuproxy")
	}
	if e.Host != "host-1" {
		t.Errorf("Host = %q, want %q", e.Host, "host-1")
	}
	if len(e.Tags) != 1 || e.Tags[0] != "env:testt" {
		t.Errorf("Tags = %v, want [env:testt]", e.Tags)
	}
}

func TestBuildLogEventsEmptyInput(t *testing.T) {
	got := buildLogEvents(searchResponse{})
	if len(got) != 0 {
		t.Errorf("buildLogEvents(empty) = %+v, want empty", got)
	}
}

func TestSearchEmptyAccessTokenErrorsWithoutRequest(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer srv.Close()

	_, _, err := Search(context.Background(), config.DatadogConfig{Site: strings.TrimPrefix(srv.URL, "http://"), AccessToken: ""}, "q", time.Now(), time.Now())
	if err == nil {
		t.Fatal("Search() error = nil, want non-nil for empty access token")
	}
	if called {
		t.Error("Search() made an HTTP request despite an empty access token")
	}
}

func TestSearchSendsExpectedRequestAndParsesResponse(t *testing.T) {
	var gotAuth, gotContentType, gotMethod, gotPath string
	var gotBody searchRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotContentType = r.Header.Get("Content-Type")
		gotMethod = r.Method
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decoding request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		respBytes, _ := json.Marshal(newTestLogEvent(time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)))
		w.Write(respBytes)
	}))
	defer srv.Close()

	from := time.Date(2026, 8, 10, 10, 0, 0, 0, time.UTC)
	to := time.Date(2026, 8, 10, 14, 0, 0, 0, time.UTC)

	got, hasMore, err := search(context.Background(), srv.URL, "tok-123", "env:testt service:fibuproxy", from, to)
	if err != nil {
		t.Fatalf("search() error = %v", err)
	}
	if hasMore {
		t.Error("hasMore = true, want false (no meta.page.after in response)")
	}
	if len(got) != 1 || got[0].Message != "something happened" {
		t.Errorf("search() results = %+v, want the single stubbed event", got)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/api/v2/logs/events/search" {
		t.Errorf("path = %q, want /api/v2/logs/events/search", gotPath)
	}
	if gotAuth != "Bearer tok-123" {
		t.Errorf("Authorization header = %q, want %q", gotAuth, "Bearer tok-123")
	}
	if gotContentType != "application/json" {
		t.Errorf("Content-Type header = %q, want application/json", gotContentType)
	}
	if gotBody.Filter.Query != "env:testt service:fibuproxy" {
		t.Errorf("filter.query = %q, want %q", gotBody.Filter.Query, "env:testt service:fibuproxy")
	}
	if gotBody.Filter.From != "2026-08-10T10:00:00Z" {
		t.Errorf("filter.from = %q, want %q", gotBody.Filter.From, "2026-08-10T10:00:00Z")
	}
	if gotBody.Filter.To != "2026-08-10T14:00:00Z" {
		t.Errorf("filter.to = %q, want %q", gotBody.Filter.To, "2026-08-10T14:00:00Z")
	}
	if gotBody.Sort != "-timestamp" {
		t.Errorf("sort = %q, want %q", gotBody.Sort, "-timestamp")
	}
	if gotBody.Page.Limit != 1000 {
		t.Errorf("page.limit = %d, want 1000", gotBody.Page.Limit)
	}
}

func TestSearchHasMoreWhenCursorPresent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[],"meta":{"page":{"after":"cursor-xyz"}}}`))
	}))
	defer srv.Close()

	_, hasMore, err := search(context.Background(), srv.URL, "tok", "q", time.Now(), time.Now())
	if err != nil {
		t.Fatalf("search() error = %v", err)
	}
	if !hasMore {
		t.Error("hasMore = false, want true (meta.page.after present)")
	}
}

func TestSearchNonOKStatusReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"errors":["Forbidden"]}`))
	}))
	defer srv.Close()

	_, _, err := search(context.Background(), srv.URL, "bad-token", "q", time.Now(), time.Now())
	if err == nil {
		t.Fatal("search() error = nil, want non-nil for a 401 response")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("search() error = %v, want it to mention status 401", err)
	}
}
