// Package datadoglogs provides read-only access to Datadog Logs: running
// a single search over a query and time window via the Logs Search API
// (POST /api/v2/logs/events/search).
//
// Unlike the AWS-backed packages (internal/awsssm, internal/awssecrets,
// internal/awslogs), there's no SDK/credential-chain here — Datadog
// authenticates a single request with one static credential, a Personal
// Access Token, sent as "Authorization: Bearer <token>" (not the classic
// API Key + Application Key pair — see spec/39-fe-datadog-logs for why).
// A hand-rolled net/http client is used instead of the official
// datadog-api-client-go, which is a large, fully code-genned client
// covering Datadog's entire API surface — not justified for one
// endpoint.
package datadoglogs

import (
	"time"
)

// LogEvent is one matched log entry from a Search call.
type LogEvent struct {
	Timestamp time.Time
	Service   string
	Status    string
	Host      string
	// Env is extracted from Tags (Datadog has no top-level "env"
	// attribute the way it does service/status/host — env is
	// conventionally an "env:<value>" tag) — see buildLogEvents.
	Env     string
	Message string
	Tags    []string
}

// requestTimeout bounds a single Search call's HTTP round-trip.
const requestTimeout = 30 * time.Second
