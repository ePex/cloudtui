package jolokia

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"sort"
	"strings"

	"github.com/ePex/cloudtui/tui/internal/queue/httplog"
)

// WithRequestLog makes c log every HTTP request it sends, naming
// connection in each line (spec/06). Returns c.
func (c *Client) WithRequestLog(connection string) *Client {
	c.http.Transport = &httplog.Transport{
		Base:         c.http.Transport,
		Backend:      "jolokia",
		Connection:   connection,
		Describe:     describeRequest,
		AppStatus:    appStatus,
		AppStatusKey: "jolokiaStatus",
	}
	return c
}

// requestSummary is what describeRequest decodes of one Jolokia request:
// never its arguments (message IDs, bodies, headers).
type requestSummary struct {
	Type      string `json:"type"`
	MBean     string `json:"mbean"`
	Operation string `json:"operation"`
	Attribute string `json:"attribute"`
}

// describeRequest names a Jolokia request's operation for the log: a
// GET is the queue search; a POST holds one request ("exec
// removeMessage on orders") or a bulk array ("bulk: 8 × read
// QueueSize/ConsumerCount/EnqueueCount/DequeueCount"). "" if the body
// can't be read.
func describeRequest(req *http.Request) string {
	if req.Method == http.MethodGet {
		if strings.Contains(req.URL.Path, "/search/") {
			return "search"
		}
		return "get"
	}
	body := httplog.RequestBody(req)
	var one requestSummary
	if err := json.Unmarshal(body, &one); err == nil {
		return fmt.Sprintf("%s %s on %s", one.Type, one.name(), target(one.MBean))
	}
	var bulk []requestSummary
	if err := json.Unmarshal(body, &bulk); err != nil || len(bulk) == 0 {
		return ""
	}
	// Names per type, in order of first appearance: "read A/B, exec C".
	var types []string
	names := map[string][]string{}
	for _, r := range bulk {
		if _, ok := names[r.Type]; !ok {
			types = append(types, r.Type)
		}
		if n := r.name(); !slices.Contains(names[r.Type], n) {
			names[r.Type] = append(names[r.Type], n)
		}
	}
	parts := make([]string, len(types))
	for i, typ := range types {
		parts[i] = typ + " " + strings.Join(names[typ], "/")
	}
	return fmt.Sprintf("bulk: %d × %s", len(bulk), strings.Join(parts, ", "))
}

// name is the operation (without its signature) or the attribute.
func (r requestSummary) name() string {
	if r.Operation != "" {
		op, _, _ := strings.Cut(r.Operation, "(")
		return op
	}
	return r.Attribute
}

// target is the queue an MBean names (its destinationName), or the
// whole MBean if it names none.
func target(mbean string) string {
	for _, prop := range strings.Split(mbean, ",") {
		if name, ok := strings.CutPrefix(prop, "destinationName="); ok {
			return name
		}
	}
	return mbean
}

// statusOnly is all appStatus decodes of a Jolokia reply.
type statusOnly struct {
	Status int `json:"status"`
}

// appStatus reports the status Jolokia puts inside its reply, which is
// HTTP 200 even when the operation failed: "200" for one reply, the
// count per status for a bulk reply ("200×46, 404×2"). ok is false
// unless every status is 200. "" if the body isn't a Jolokia reply.
func appStatus(body []byte) (string, bool) {
	var one statusOnly
	if err := json.Unmarshal(body, &one); err == nil {
		if one.Status == 0 {
			return "", true
		}
		return fmt.Sprint(one.Status), one.Status == http.StatusOK
	}
	var bulk []statusOnly
	if err := json.Unmarshal(body, &bulk); err != nil || len(bulk) == 0 {
		return "", true
	}
	counts := map[int]int{}
	for _, r := range bulk {
		counts[r.Status]++
	}
	statuses := make([]int, 0, len(counts))
	for s := range counts {
		statuses = append(statuses, s)
	}
	sort.Ints(statuses)
	parts := make([]string, len(statuses))
	for i, s := range statuses {
		parts[i] = fmt.Sprintf("%d×%d", s, counts[s])
	}
	return strings.Join(parts, ", "), len(counts) == 1 && counts[http.StatusOK] > 0
}
