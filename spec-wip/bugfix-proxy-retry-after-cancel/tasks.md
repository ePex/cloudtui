# Tasks: mq-proxy retries a GET the user cancelled

Each task needs its own approval before it starts, and is pushed once
done. Live checks run with an isolated HOME and a separate tmux server
(`tmux -L <name>`), never against the real `~/.cloudtui`, and confirm
on the captured screen that the target is a scratch queue before every
destructive key.

1. [x] **No retry once the context is done** (plan "Approach").
   `internal/queue/proxy/proxy.go` + `proxy_test.go`: the
   `ctx.Err() == nil` condition and its comment. Tests: a cancelled
   request and a caller deadline each reach the server once and return
   the original error (`context.Canceled`, `context.DeadlineExceeded`);
   a short `httpClient.Timeout` with a slow first response is still
   retried and succeeds; the existing warm-up test keeps passing.
   Mutation-check.
   *Live check (isolated HOME, local broker, `task dev:proxy:start`):*
   Esc during a queue load over mq-proxy; the log has one cancelled
   line for that load, not two.
   *Done (2026-10-17):* `doRequest` retries a GET only while
   `ctx.Err() == nil`, with a comment on why (and why the client's
   Timeout is still retried). One test detail beyond the plan: a
   request on an already done context fails inside `http.Client.Do`
   before reaching the server (the live log's extra line took 0 s), so
   counting at the server would pass even without the fix; the tests
   count attempts with a `countingTransport` on the client instead.
   `TestGetIsNotRetriedOnceTheContextIsDone` (cancelled once the
   request arrived; a 100 ms caller deadline: 1 attempt each, the error
   wrapping `context.Canceled` / `context.DeadlineExceeded`) and
   `TestGetIsStillRetriedAfterTheClientTimeout` (`httpClient.Timeout`
   200 ms, the first response 2 s late: retried, 2 server hits,
   success). The existing retry tests pass. All pass with `-race`.
   Mutation-checked: retrying despite a done context fails both cancel
   sub-tests; removing the retry entirely fails the client-timeout test
   (and the existing retry tests), so it really checks the retry.
   Live (scratch HOME, own tmux server, `task dev:proxy:start`): Esc
   0.4 s into a queue load over mq-proxy → "Loading cancelled" and one
   log line, `list-queues … duration=490.3ms result=cancelled` (before:
   a second, 0 s cancelled line). `r` then loaded normally (`INFO`,
   status 200). mq-proxy stopped. `go test ./...`, gofmt and vet pass.

2. [ ] **Regression run.** `task test`, `task test:tui:race`, smoke
   test (isolated). Stop mq-proxy, remove the scratch setup.
