// Package httplog logs each HTTP exchange of a queue backend client
// (Jolokia, mq-proxy) as one log line: what was asked, how it went and
// how long it took. Headers and bodies are never logged, so the log
// stays safe to attach to a ticket (spec/06).
package httplog

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// Message is the log message of every line Transport writes.
const Message = "queue backend request"

// Transport is an http.RoundTripper that sends each request through
// Base and logs it once the response body has been read (or the request
// failed): INFO normally, WARN for a failure, INFO "cancelled" when the
// request's context was cancelled (Esc, spec/24).
type Transport struct {
	// Base sends the requests; nil means http.DefaultTransport.
	Base http.RoundTripper
	// Backend and Connection name the client in each line.
	Backend, Connection string
	// Describe, if set, names the request's operation. It must not read
	// req.Body; RequestBody gives it a copy.
	Describe func(req *http.Request) string
	// AppStatus, if set, extracts a status the backend reports inside a
	// response body (Jolokia answers HTTP 200 for a failed operation),
	// logged under AppStatusKey. ok false makes the line a WARN. An empty
	// status is left out.
	AppStatus    func(body []byte) (status string, ok bool)
	AppStatusKey string
	// Logger is where lines go; nil means slog.Default() at the time of
	// each request.
	Logger *slog.Logger
}

// RoundTrip implements http.RoundTripper.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()
	op := ""
	if t.Describe != nil {
		op = t.Describe(req)
	}
	attrs := []any{
		"backend", t.Backend,
		"connection", t.Connection,
		"method", req.Method,
		"url", redactedURL(req),
		"op", op,
	}

	resp, err := t.base().RoundTrip(req)
	if err != nil {
		t.logFailure(req, append(attrs, "duration", since(start)), err)
		return nil, err
	}

	body, readErr := io.ReadAll(resp.Body)
	resp.Body.Close()
	// The client reads exactly what the server sent, and the same read
	// error after it, if any.
	var replay io.Reader = bytes.NewReader(body)
	if readErr != nil {
		replay = io.MultiReader(replay, errReader{readErr})
	}
	resp.Body = io.NopCloser(replay)

	attrs = append(attrs, "status", resp.StatusCode)
	ok := resp.StatusCode >= 200 && resp.StatusCode < 300
	if t.AppStatus != nil && readErr == nil {
		if status, appOK := t.AppStatus(body); status != "" {
			attrs = append(attrs, t.appStatusKey(), status)
			ok = ok && appOK
		}
	}
	attrs = append(attrs, "duration", since(start), "bytes", len(body))

	switch {
	case readErr != nil:
		t.logFailure(req, attrs, readErr)
	case ok:
		t.logger().Info(Message, attrs...)
	default:
		t.logger().Warn(Message, attrs...)
	}
	return resp, nil
}

// logFailure logs a request that got no (complete) response: INFO
// "cancelled" if its context was cancelled, otherwise WARN with err.
func (t *Transport) logFailure(req *http.Request, attrs []any, err error) {
	if errors.Is(req.Context().Err(), context.Canceled) {
		t.logger().Info(Message, append(attrs, "result", "cancelled")...)
		return
	}
	t.logger().Warn(Message, append(attrs, "error", err.Error())...)
}

func (t *Transport) base() http.RoundTripper {
	if t.Base != nil {
		return t.Base
	}
	return http.DefaultTransport
}

func (t *Transport) logger() *slog.Logger {
	if t.Logger != nil {
		return t.Logger
	}
	return slog.Default()
}

func (t *Transport) appStatusKey() string {
	if t.AppStatusKey != "" {
		return t.AppStatusKey
	}
	return "appStatus"
}

// RequestBody returns a copy of req's body for Describe, read through
// req.GetBody (which http.NewRequestWithContext sets for in-memory
// bodies), so the body that's sent stays intact. nil if there is none.
func RequestBody(req *http.Request) []byte {
	if req.GetBody == nil {
		return nil
	}
	rc, err := req.GetBody()
	if err != nil {
		return nil
	}
	defer rc.Close()
	b, err := io.ReadAll(rc)
	if err != nil {
		return nil
	}
	return b
}

// redactedURL is req's URL without any user name or password.
func redactedURL(req *http.Request) string {
	u := *req.URL
	u.User = nil
	return u.String()
}

// since is the time since start, rounded for a readable log line.
func since(start time.Time) time.Duration {
	return time.Since(start).Round(100 * time.Microsecond)
}

// errReader returns err on every read.
type errReader struct{ err error }

func (r errReader) Read([]byte) (int, error) { return 0, r.err }
