# Tasks: log every queue backend HTTP request

Each task needs its own approval before it starts, and is pushed once
done. Live checks run with an isolated HOME and a separate tmux server
(`tmux -L <name>`), never against the real `~/.cloudtui`.

1. [ ] **`httplog.Transport`** (plan "Approach").
   `internal/queue/httplog/httplog.go` + `httplog_test.go`. Tests
   against an `httptest` server with a capturing `slog` handler: a 200
   gives one `INFO` line with every attribute and the client gets the
   body byte for byte; a 500 and an `AppStatus` failure give `WARN`; a
   refused connection gives `WARN` with the error; a cancelled request
   gives `INFO` "cancelled"; the `Authorization` value, a password in
   the URL and the bodies never appear in the log; the request body
   reaches the server intact after `Describe` read it.
   Mutation-check. Not used by any client yet.

2. [ ] **Jolokia** (plan "Backend specifics").
   `internal/queue/jolokia/httplog.go` + test: `describeRequest` and
   `appStatus` (table tests: single exec, single read, bulk, search
   GET, non-JSON), and `Client.WithRequestLog`. A test of the real
   `Client` with request logging against a fake Jolokia server: `List`
   logs the search and the bulk read, and a Jolokia error status inside
   a 200 response is a `WARN` line. Mutation-check.

3. [ ] **mq-proxy** (plan "Backend specifics").
   `internal/queue/proxy/httplog.go` + test: the operation is the URL
   path; `Client.WithRequestLog`. A test of the real `Client` against a
   fake mq-proxy: one line per request, including each warm-up retry.
   Mutation-check.

4. [ ] **Wire it in** (plan "Backend specifics").
   `internal/queue/secretbackend/secretbackend.go`: `buildBackend`
   turns on request logging with the connection's name, for both
   backends (test: a request through the built backend writes a line
   naming the connection).
   *Live check (isolated HOME, local broker, scratch queue):* browse,
   delete a message, purge with and without a JMS type, then read the
   `:log` view and the file: one line per request with the expected
   operations, `INFO`/`WARN` as expected, no password. The same against
   mq-proxy (`task dev:proxy:start`). Try Esc during a large browse for
   a cancelled line.

5. [ ] **Regression run.** `task test`, `task test:tui:race`, smoke
   test (isolated). Remove the scratch setup.
