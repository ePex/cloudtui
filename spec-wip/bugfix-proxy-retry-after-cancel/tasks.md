# Tasks: mq-proxy retries a GET the user cancelled

Each task needs its own approval before it starts, and is pushed once
done. Live checks run with an isolated HOME and a separate tmux server
(`tmux -L <name>`), never against the real `~/.cloudtui`, and confirm
on the captured screen that the target is a scratch queue before every
destructive key.

1. [ ] **No retry once the context is done** (plan "Approach").
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

2. [ ] **Regression run.** `task test`, `task test:tui:race`, smoke
   test (isolated). Stop mq-proxy, remove the scratch setup.
