# Tasks: AWS SSO login opens the browser twice

Each task needs its own approval before it starts, and is pushed once
done. Live checks run with an isolated HOME, a fake `aws` first on
`PATH`, and a separate tmux server (`tmux -L <name>`), never against the
real `~/.cloudtui` or `~/.aws`.

1. [x] **`awsauth.LoginCoordinator`** (plan "Approach").
   `internal/awsauth/coordinator.go` + `coordinator_test.go`:
   `NewLoginCoordinator(run)`, `Login(ctx, profile, onCode)` and
   `Close()`. Tests with a channel-controlled fake `run`: one run for
   concurrent logins of one profile; separate runs per profile; the
   code reaching every caller once, whether it joins before or after
   the code was printed; a cancelled caller returning
   `context.Canceled` while the run continues, a later login joining
   it, and a new run once it finished; a failed run's error for every
   waiting caller; `Close()` cancelling the run's context and returning
   only after the run returned. Mutation-check each. Not wired into
   the app yet.
   *Done (2026-10-17):* one addition to the plan: a caller that stops
   waiting is also dropped from the callers to get the code, so a
   cancelled queue load doesn't later get "Verify code …" written into
   its status. Six tests (`TestLoginCoordinator…`), with a fake run
   controlled through channels and every wait bounded at 2 s, pass
   with `-race` (5 runs). Mutation-checked, each failing at least one
   test: no shared run; the run on the caller's context; a cancelled
   caller keeping its `onCode`; no code for a late joiner; callers kept
   after the code (so a second report reaches them again); `Close`
   without waiting; `Close` without cancelling. A guard against the
   run reporting the code twice survived its mutation: clearing the
   callers after the first report already covers it, so the guard was
   removed. `go test ./...`, gofmt and vet pass.

2. [x] **Missing token file counts as expired** (plan "Missing token
   file"). `internal/awsauth/awsauth.go` + `awsauth_test.go`: the new
   string in `NeedsReauth`. Test with the real SDK's error from an
   `sso-session` profile in a temp `~/.aws/config` with no token cache
   (no network); the same error for a non-SSO auth type stays a plain
   error. Mutation-check.
   *Done (2026-10-17):* `TestNeedsReauthMissingSSOTokenFile` gets the
   error from the real SDK (`config.LoadDefaultConfig` on a temp
   config, then `Credentials.Retrieve`), wrapped like
   `awssecrets.Reveal` wraps it. Run before the fix, it failed on that
   error ("…failed to read cached SSO token file, open …/.aws/sso/
   cache/….json: no such file or directory, SSO) = false, want true");
   with the fix it passes, and a static-keys profile still gets a plain
   error. `go test ./...` (with `-race` for `awsauth`), gofmt and vet
   pass.

3. [x] **Wire it into the app** (plan "Approach", "Quitting").
   `internal/app/app.go`: create the coordinator around
   `awsauth.Login`, use its `Login` as `a.awsSSOLogin`, and `Close()`
   it in `Run` after `tv.Run()` returns.
   *Live check:* the bug's reproduction setup (fake `sso-session`
   profile with an expired token, connection with a Secrets Manager
   password, fake `aws` logging each call and waiting a few seconds).
   Open the queue list, Esc during the login, reload with `r`: the log
   shows one `aws sso login`, which runs to the end. Remove the token
   file and reload: a login is offered (one call) instead of a plain
   error. Quit during a login: the fake `aws` process is gone.
   *Done (2026-10-17):* `app.New` keeps the coordinator on `App`
   (`ssoLogins`) and uses its `Login` as `a.awsSSOLogin`; `Run` closes
   it after `tv.Run()` returns. Live (fake `aws` waiting 10 s, scratch
   HOME, own tmux server):
   - **Esc, then `r`:** Esc left "Loading cancelled — press r to
     retry"; `r` joined the running login and showed its code at once.
     The log has one `aws sso login`, which ran its full 10 s to the end
     (before the fix, Esc killed it and `r` started a second one).
   - **Token file removed, `r`:** the login was offered (one call, the
     code shown) instead of a plain error.
   - **Quit during a login (Esc, then `q`):** the fake `aws` process was
     killed, but cloudtui took **7.7 s** to exit (idle: 0.15 s). The
     fake's `sleep` child inherited the CLI's output, and `exec.Cmd.Run`
     reads output until EOF, so `Close()` waited for that child. The
     real CLI usually runs as one process, but a child holding its
     output would hang quitting the same way. Fixed beyond the plan in
     `awsauth.Login`: `cmd.WaitDelay = loginWaitDelay` (1 s) stops
     reading the output 1 s after the kill. New test
     `TestLoginCancelReturnsDespiteAChildHoldingOutput`: a fake CLI
     starts a background child, writes its PID once it exists (a fixed
     sleep first let the test cancel before the child existed, so it
     passed without the fix), and is cancelled; it hangs past 4 s
     without `WaitDelay` and passes with it. Live after the fix:
     cloudtui exits **1.1 s** after `q`, no fake `aws` left.
   `go test ./...` (with `-race` for `awsauth`), gofmt and vet pass.
   Merge-back: spec/14 should mention the bounded wait on quitting.

4. [ ] **Regression run.** `task test`, `task test:tui:race`, smoke
   test (isolated HOME and tmux server). Remove the scratch setup.
