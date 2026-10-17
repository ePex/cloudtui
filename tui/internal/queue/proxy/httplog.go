package proxy

import (
	"net/http"

	"github.com/ePex/cloudtui/tui/internal/queue/httplog"
)

// WithRequestLog makes c log every HTTP request it sends, naming
// connection in each line (spec/06), including each warm-up retry.
// Returns c.
func (c *Client) WithRequestLog(connection string) *Client {
	c.httpClient.Transport = &httplog.Transport{
		Base:       c.httpClient.Transport,
		Backend:    "proxy",
		Connection: connection,
		Describe:   describeRequest,
	}
	return c
}

// describeRequest names an mq-proxy request's operation for the log: its
// URL path (e.g. /api/management/command/list-messages). The query
// string (queue name, filter) is in the logged URL.
func describeRequest(req *http.Request) string {
	return req.URL.Path
}
